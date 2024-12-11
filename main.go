package main

import (
	"context"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
	"lit/cmd"
	"lit/internal/config"
	"lit/internal/db"
	"lit/internal/embedding"
	"lit/internal/fileutil"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Warning: using default config: %v", err)
	}
	embedding.BaseURL = cfg.EmbeddingBaseURL
	embedding.Model = cfg.EmbeddingModel

	// Parse command-line arguments
	var cli cmd.CLI
	kctx := kong.Parse(&cli)

	// Initialize database
	database, err := db.InitDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Handle commands
	switch kctx.Command() {
	case "add <file-path>":
		err = filepath.WalkDir(cli.Add.FilePath, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Failed to walk directory %q: %v", cli.Add.FilePath, err)
				return err
			}
			if !dirEntry.IsDir() {
				if err := fileutil.AddDocument(ctx, database, path); err != nil {
					log.Printf("Failed to add document %q: %v", path, err)
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("Failed to walk directory %q: %v", cli.Add.FilePath, err)
		}
	case "search <query>":
		// Generate embedding for search query
		queryEmbedding, err := embedding.CreateEmbedding(ctx, cli.Search.Query)
		if err != nil {
			log.Fatalf("Failed to create query embedding: %v", err)
		}

		// Perform search
		if err := db.SearchDocuments(database, queryEmbedding, cli.Search.Limit, cli.Search.Format); err != nil {
			log.Fatalf("Search failed: %v", err)
		}
	default:
		panic("Unexpected command: " + kctx.Command())
	}
}
