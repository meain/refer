package fileutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/meain/refer/internal/embedding"
	"github.com/meain/refer/internal/webutil"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

const maxParallelEmbeddingRequests = 10

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

func AddDocument(ctx context.Context, db *sql.DB, path string) error {
	var content string
	var title string
	var err error

	// TODO: Check if the document(file/url) exists in DB, if it does, check if
	// the contents match and only reindex if they are not the same.
	// Check if path is a URL
	if webutil.IsURL(path) {
		content, title, err = webutil.FetchWebContent(path)
		if err != nil {
			return fmt.Errorf("failed to fetch URL %q: %v", path, err)
		}

		if title == "" {
			title = path
		}
	} else {
		// Handle regular file
		fileInfo, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat file %q: %v", path, err)
		}
		if fileInfo.IsDir() {
			return nil
		}
		if !IsTextFile(path) {
			return nil
		}

		// Read file content
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %v", err)
		}
		content = string(contentBytes)
		title = path
	}

	// Generate embedding
	embedding, err := embedding.CreateEmbedding(ctx, content)
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
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM documents WHERE filepath = ?)", path).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if document exists: %v", err)
	}
	if exists {
		_, err = db.Exec("DELETE FROM documents WHERE filepath = ?", path)
		if err != nil {
			return fmt.Errorf("failed to delete existing document: %v", err)
		}
	}

	// Insert document with vector embedding
	_, err = db.Exec(
		"INSERT INTO documents(filepath, content, title, embedding) VALUES (?, ?, ?, ?)",
		path, content, title, serializedEmbedding)
	if err != nil {
		return fmt.Errorf("failed to insert document: %v", err)
	}

	fmt.Printf("Document added: %s\n", path)
	return nil
}

// AddDocuments processes multiple documents in parallel with a maximum number of concurrent operations
func AddDocuments(ctx context.Context, db *sql.DB, paths []string) []error {
	// Create a channel for paths and errors
	pathChan := make(chan string, len(paths))
	errChan := make(chan error, len(paths))

	// Create a wait group to track workers
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxParallelEmbeddingRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				if err := AddDocument(ctx, db, path); err != nil {
					errChan <- fmt.Errorf("error processing %q: %v", path, err)
				} else {
					errChan <- nil
				}
			}
		}()
	}

	// Send paths to workers
	for _, path := range paths {
		pathChan <- path
	}
	close(pathChan)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
