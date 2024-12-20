package internal

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	md "github.com/JohannesKaufmann/html-to-markdown"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

const maxParallelEmbeddingRequests = 10

// FetchDocument retrieves content from either a local file or remote URL
func FetchDocument(path string) (*Document, error) {
	if IsRemoteURL(path) {
		return fetchRemoteDocument(path)
	}
	return fetchLocalDocument(path)
}

// IsRemoteURL checks if the given path is a remote URL
func IsRemoteURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// fetchRemoteDocument fetches and processes a remote document
func fetchRemoteDocument(url string) (*Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	converter := md.NewConverter("", true, nil)
	content, err := converter.ConvertString(string(body))
	if err != nil {
		return nil, fmt.Errorf("convert HTML to markdown: %w", err)
	}

	doc := &Document{
		Path:     url,
		Content:  strings.TrimSpace(content),
		Title:    extractTitle(string(body)),
		IsRemote: true,
	}

	if doc.Title == "" {
		doc.Title = url
	}

	return doc, nil
}

// fetchLocalDocument reads and processes a local document
func fetchLocalDocument(path string) (*Document, error) {
	if err := validateLocalFile(path); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	return &Document{
		Path:     path,
		Content:  string(content),
		Title:    path,
		IsRemote: false,
	}, nil
}

// validateLocalFile checks if a local file is valid for processing
func validateLocalFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}
	if !isTextFile(path) {
		return fmt.Errorf("not a text file: %s", path)
	}
	return nil
}

// isTextFile checks if a file is a text file by examining its contents
func isTextFile(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	if len(data) == 0 {
		return true
	}

	// Check only first 512 bytes for performance
	if len(data) > 512 {
		data = data[:512]
	}

	for _, b := range data {
		if b == 0 || (b > 127 && !isPrintable(b)) {
			return false
		}
	}
	return true
}

func isPrintable(b byte) bool {
	return (b >= 32 && b <= 126) || (b >= 192 && b <= 255)
}

func extractTitle(html string) string {
	titleStart := strings.Index(html, "<title>")
	titleEnd := strings.Index(html, "</title>")
	if titleStart != -1 && titleEnd != -1 {
		title := html[titleStart+7 : titleEnd]
		return strings.TrimSpace(title)
	}
	return ""
}

// AddDocument adds a single document to the database
func AddDocument(ctx context.Context, db *sql.DB, path string) error {
	doc, err := FetchDocument(path)
	if err != nil {
		return fmt.Errorf("fetch document %s: %w", path, err)
	}

	// Generate and serialize embedding
	embedding, err := createAndSerializeEmbedding(ctx, doc.Content)
	if err != nil {
		return err
	}

	// Update database
	if err := updateDocument(db, doc, embedding); err != nil {
		return err
	}

	fmt.Printf("Added document: %s\n", doc.Path)
	return nil
}

func createAndSerializeEmbedding(ctx context.Context, content string) ([]byte, error) {
	embedding, err := CreateEmbedding(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}

	serialized, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return nil, fmt.Errorf("serialize embedding: %w", err)
	}

	return serialized, nil
}

func updateDocument(db *sql.DB, doc *Document, embedding []byte) error {
	// Delete existing document if it exists
	_, err := db.Exec("DELETE FROM documents WHERE filepath = ?", doc.Path)
	if err != nil {
		return fmt.Errorf("delete existing document: %w", err)
	}

	// Insert new document
	_, err = db.Exec(
		"INSERT INTO documents(filepath, content, title, embedding) VALUES (?, ?, ?, ?)",
		doc.Path, doc.Content, doc.Title, embedding)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}

	return nil
}

// AddDocuments processes multiple documents in parallel
func AddDocuments(ctx context.Context, db *sql.DB, paths []string, maxWorkers int) []error {
	if maxWorkers <= 0 {
		maxWorkers = maxParallelEmbeddingRequests
	}

	// Create buffered channels for paths and errors
	pathChan := make(chan string, len(paths))
	errChan := make(chan error, len(paths))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				if err := AddDocument(ctx, db, path); err != nil {
					errChan <- fmt.Errorf("%s: %w", path, err)
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

	// Wait for workers and close error channel
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect non-nil errors
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
