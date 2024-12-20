package internal

import (
	"database/sql"
	"fmt"
	"os"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

// Document represents a stored document
type Document struct {
	ID       int64
	Path     string
	Content  string
	Title    string
	IsRemote bool
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

		filepaths = append(filepaths, filepath)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating filepaths: %v", err)
	}

	return filepaths, nil
}

// CreateDB creates or opens a SQLite database at the given path.
// Returns the database connection, a boolean indicating if it's a new database,
// and any error that occurred.
func CreateDB(dbPath string) (*sql.DB, bool, error) {
	sqlite_vec.Auto() // Ensure sqlite-vec is loaded

	isNew := !fileExists(dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("open database %s: %w", dbPath, err)
	}

	return db, isNew, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// InitDatabase initializes the database schema with the required tables
func InitDatabase(db *sql.DB, embeddingSize int) error {
	query := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS documents USING vec0(
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			filepath TEXT UNIQUE,
			content TEXT,
			title TEXT,
			embedding float[%d]
		)
	`, embeddingSize)

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("create documents table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT
		)`); err != nil {
		return fmt.Errorf("create config table: %w", err)
	}

	return nil
}

// SaveConfig saves configuration key-value pairs to the database
func SaveConfig(db *sql.DB, config map[string]string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for key, value := range config {
		if _, err := stmt.Exec(key, value); err != nil {
			return fmt.Errorf("insert config %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
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

func SearchDocuments(db *sql.DB, queryEmbedding []float32, limit int, format string) error {
	serializedQuery, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return fmt.Errorf("serialize query: %w", err)
	}

	query := `
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
	`

	rows, err := db.Query(query, serializedQuery, limit)
	if err != nil {
		return fmt.Errorf("execute search: %w", err)
	}
	defer rows.Close()

	switch format {
	case "names":
		return printNameResults(rows)
	case "llm":
		return printLLMResults(rows)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

func printNameResults(rows *sql.Rows) error {
	for rows.Next() {
		var rowid int
		var filepath string
		var content, title string
		var distance float64

		if err := rows.Scan(&rowid, &filepath, &content, &title, &distance); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		fmt.Printf("%d: %s (%.4f)\n", rowid, filepath, distance)
	}
	return rows.Err()
}

func printLLMResults(rows *sql.Rows) error {
	var results []struct {
		Filepath string
		Title    string
		Contents string
	}

	for rows.Next() {
		var rowid int
		var filepath string
		var content, title string
		var distance float64

		if err := rows.Scan(&rowid, &filepath, &content, &title, &distance); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		results = append(results, struct {
			Filepath string
			Title    string
			Contents string
		}{
			Filepath: filepath,
			Title:    title,
			Contents: content,
		})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	// Print results in LLM format
	for _, r := range results {
		fmt.Printf("File: %s\nTitle: %s\n\n%s\n---\n", r.Filepath, r.Title, r.Contents)
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
