package fileutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"lit/internal/embedding"
)

func IsTextFile(filePath string) bool {
	// Try to read the first 512 bytes of the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	if len(data) == 0 {
		return true
	}
	limit := 512
	if len(data) > limit {
		data = data[:limit]
	}
	isBinary := false
	for _, b := range data {
		if b == 0 {
			isBinary = true
			break
		}
		if b > 127 && !isPrintable(b) {
			isBinary = true
			break
		}
	}
	return !isBinary
}

func isPrintable(b byte) bool {
	return (b >= 32 && b <= 126) || (b >= 192 && b <= 255)
}

func AddDocument(ctx context.Context, db *sql.DB, filePath string) error {
	// Check if file is a text file
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %q: %v", filePath, err)
	}
	if fileInfo.IsDir() {
		return nil
	}
	if !IsTextFile(filePath) {
		return nil
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Generate embedding
	embedding, err := embedding.CreateEmbedding(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to create embedding: %v", err)
	}

	// Serialize the embedding
	serializedEmbedding, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %v", err)
	}

	// Check if document already exists, delete it if so
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM documents WHERE filepath = ?)", filePath).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if document exists: %v", err)
	}
	if exists {
		_, err = db.Exec("DELETE FROM documents WHERE filepath = ?", filePath)
		if err != nil {
			return fmt.Errorf("failed to delete existing document: %v", err)
		}
	}

	// Insert document with vector embedding
	_, err = db.Exec(
		"INSERT INTO documents(filepath, content, embedding) VALUES (?, ?, ?)",
		filePath, string(content), serializedEmbedding)
	if err != nil {
		return fmt.Errorf("failed to insert document: %v", err)
	}

	fmt.Printf("Document added: %s\n", filePath)
	return nil
}
