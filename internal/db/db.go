package db

import (
	"database/sql"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

const (
	DBPath       = ".referdb"
	EmbeddingDim = 768 // Typical dimension for nomic-embed-text
)

// Document represents a stored document
type Document struct {
	ID      int64
	Path    string
	Content string
}

// GetAllDocuments retrieves all documents from the database
func GetAllDocuments(db *sql.DB) ([]Document, error) {
	rows, err := db.Query("SELECT rowid, filepath FROM documents")
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %v", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Path); err != nil {
			return nil, fmt.Errorf("failed to scan document: %v", err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %v", err)
	}
	return docs, nil
}

func InitDatabase() (*sql.DB, error) {
	// Ensure sqlite-vec is loaded
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, err
	}

	// Create virtual table for vector embeddings
	_, err = db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS documents USING vec0(
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			filepath TEXT,
			content TEXT,
			embedding float[%d]
		)
	`, EmbeddingDim))
	if err != nil {
		return nil, fmt.Errorf("failed to create vec table: %v", err)
	}

	return db, nil
}

func SearchDocuments(db *sql.DB, queryEmbedding []float32, limit int, format string) error {
	// Serialize the query embedding
	serializedQuery, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return fmt.Errorf("failed to serialize query embedding: %v", err)
	}

	// Perform vector similarity search
	rows, err := db.Query(`
        SELECT 
            rowid, 
            filepath, 
            content,
            distance 
        FROM documents 
        WHERE embedding match ?
        ORDER BY distance 
        LIMIT ?
    `, serializedQuery, limit)
	if err != nil {
		return fmt.Errorf("search query failed: %v", err)
	}
	defer rows.Close()
	var count int

	// Print results
	if format == "names" {
		for rows.Next() {
			var rowid int
			var filepath string
			var content string
			var distance float64

			if err := rows.Scan(&rowid, &filepath, &content, &distance); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			count++
			fmt.Printf("%d: %s (%.4f)\n", rowid, filepath, distance)
		}
	} else if format == "llm" {
		var llmQuery []struct {
			Filepath string
			Contents string
		}
		for rows.Next() {
			var rowid int
			var filepath string
			var content string
			var distance float64

			if err := rows.Scan(&rowid, &filepath, &content, &distance); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			count++
			llmQuery = append(llmQuery, struct {
				Filepath string
				Contents string
			}{
				Filepath: filepath,
				Contents: content,
			})
		}

		for _, item := range llmQuery {
			fmt.Printf("Filepath: %s\n", item.Filepath)
			fmt.Printf("Contents: \n%s\n", item.Contents)
			fmt.Println("------------------------------------------------------")
		}
	} else {
		return fmt.Errorf("unknown format: %s", format)
	}

	if rows.Err() != nil {
		return rows.Err()
	}
	if count == 0 {
		fmt.Println("No results found.")
	}

	return nil
}

// GetDocumentByID retrieves a single document by its ID
func GetDocumentByID(db *sql.DB, id int) (*Document, error) {
	var doc Document
	err := db.QueryRow(`
		SELECT rowid, filepath, content
		FROM documents 
		WHERE rowid = ?`, id).Scan(&doc.ID, &doc.Path, &doc.Content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query document: %w", err)
	}
	return &doc, nil
}

// RemoveDocument removes a document by its ID
func RemoveDocument(db *sql.DB, id int) error {
	result, err := db.Exec("DELETE FROM documents WHERE rowid = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove document: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}

	if rows == 0 {
		return fmt.Errorf("no document found with ID %d", id)
	}

	return nil
}

func GetDatabaseStats(db *sql.DB) (map[string]int, error) {
	stats := make(map[string]int)

	// Get total number of documents
	var docCount int
	err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count documents: %v", err)
	}
	stats["documents"] = docCount

	// Get total size of all documents
	var totalSize int
	err = db.QueryRow("SELECT COALESCE(SUM(LENGTH(content)), 0) FROM documents").Scan(&totalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total content size: %v", err)
	}
	stats["total_content_bytes"] = totalSize

	return stats, nil
}
