package db

import (
	"database/sql"
	"fmt"
	"os"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// Document represents a stored document
type Document struct {
	ID      int64
	Path    string
	Content string
	Title   string
}

// GetAllDocuments retrieves all documents from the database
func GetAllDocuments(db *sql.DB) ([]Document, error) {
	rows, err := db.Query("SELECT rowid, filepath, content, title FROM documents")
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %v", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Path, &doc.Content, &doc.Title); err != nil {
			return nil, fmt.Errorf("failed to scan document: %v", err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %v", err)
	}
	return docs, nil
}

func GetAllFilePaths(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT filepath FROM documents")
	if err != nil {
		return nil, fmt.Errorf("failed to query filepaths: %v", err)
	}
	defer rows.Close()

	var filepaths []string
	for rows.Next() {
		var filepath string
		if err := rows.Scan(&filepath); err != nil {
			return nil, fmt.Errorf("failed to scan filepath: %v", err)
		}

		fmt.Println(filepath)
		filepaths = append(filepaths, filepath)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating filepaths: %v", err)
	}

	return filepaths, nil
}

func CreateDB(dbPath string) (*sql.DB, bool, error) {
	// Ensure sqlite-vec is loaded
	sqlite_vec.Auto()

	_, ferr := os.Stat(dbPath) // check if already exists

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open database: %v", err)
	}

	return db, os.IsNotExist(ferr), nil
}

// SaveConfig saves the configuration into a database, we just have to
// store the embedding model to make sure that we will be using the same
// model for the search
func SaveConfig(db *sql.DB, config map[string]string) error {
	// Ensure sqlite-vec is loaded
	sqlite_vec.Auto()

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS config (key TEXT, value TEXT)")
	if err != nil {
		return fmt.Errorf("failed to create config table: %v", err)
	}

	for key, value := range config {
		_, err := db.Exec("INSERT INTO config (key, value) VALUES (?, ?)", key, value)
		if err != nil {
			return fmt.Errorf("failed to insert config value: %v", err)
		}
	}

	return nil
}

func GetConfig(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM config")
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %v", err)
	}
	defer rows.Close()

	config := make(map[string]string)

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan config: %v", err)
		}
		config[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating config: %v", err)
	}

	return config, nil
}

func InitDatabase(db *sql.DB, embeddingSize int) error {
	// Create virtual table for vector embeddings
	_, err := db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS documents USING vec0(
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			filepath TEXT,
			content TEXT,
			title TEXT,
			embedding float[%d]
		)
	`, embeddingSize))
	if err != nil {
		return fmt.Errorf("failed to create vec table: %v", err)
	}

	return nil
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
			title,
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
			var title string
			var distance float64

			if err := rows.Scan(&rowid, &filepath, &content, &title, &distance); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			count++
			fmt.Printf("%d: %s (%.4f)\n", rowid, filepath, distance)
		}
	} else if format == "llm" {
		var llmQuery []struct {
			Filepath string
			Title    string
			Contents string
		}
		for rows.Next() {
			var rowid int
			var filepath string
			var content string
			var title string
			var distance float64

			if err := rows.Scan(&rowid, &filepath, &content, &title, &distance); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			count++
			llmQuery = append(llmQuery, struct {
				Filepath string
				Title    string
				Contents string
			}{
				Filepath: filepath,
				Title:    title,
				Contents: content,
			})
		}

		for _, item := range llmQuery {
			fmt.Printf("File: %s\n", item.Filepath)
			fmt.Printf("Title: %s\n", item.Title)
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
		SELECT rowid, filepath, content, title
		FROM documents
		WHERE rowid = ?`, id).Scan(&doc.ID, &doc.Path, &doc.Content, &doc.Title)
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

// RecreateDatabase recreates the database from scratch with the current schema
func RecreateDatabase(db *sql.DB, embeddingSize int) ([]string, error) {
	// Get all existing documents before dropping the table
	docs, err := GetAllFilePaths(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing documents: %v", err)
	}

	// Drop the existing table
	_, err = db.Exec("DROP TABLE IF EXISTS documents")
	if err != nil {
		return nil, fmt.Errorf("failed to drop existing table: %v", err)
	}

	// Drop the config table
	_, err = db.Exec("DROP TABLE IF EXISTS config")
	if err != nil {
		return nil, fmt.Errorf("failed to drop config table: %v", err)
	}

	// Initialize new database with current schema
	err = InitDatabase(db, embeddingSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize new database: %v", err)
	}

	return docs, nil
}
