package db

import (
	"database/sql"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

const (
	DBPath       = ".litdb"
	EmbeddingDim = 768 // Typical dimension for nomic-embed-text
)

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
		fmt.Println("Search Results:")
		for rows.Next() {
			var rowid int
			var filepath string
			var content string
			var distance float64

			if err := rows.Scan(&rowid, &filepath, &content, &distance); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			count++
			fmt.Printf("%d. %s (%.4f)\n", count, filepath, distance)
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
