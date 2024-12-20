package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/meain/refer/internal/config"
	"github.com/meain/refer/internal/db"
	"github.com/meain/refer/internal/embedding"
	"github.com/meain/refer/internal/fileutil"
	"github.com/meain/refer/internal/webutil"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
)

type CLI struct {
	Database string   `help:"Database file path" default:".referdb"`
	Add      Add      `cmd:"" help:"Add a file or directory to the database"`
	Search   Search   `cmd:"" help:"Search for documents"`
	Show     Show     `cmd:"" help:"List documents in the database"`
	Stats    StatsCmd `cmd:"" help:"Show database statistics"`
	Reindex  Reindex  `cmd:"" help:"Reindex all documents"`
	Remove   Remove   `cmd:"" help:"Remove a document from the database"`
}

type Add struct {
	FilePath []string `kong:"arg,required"`
}

type Search struct {
	Query  string `arg:"" optional:""`
	Format string `kong:"default='names'"`
	Limit  int    `kong:"default=5"`
}

type Reindex struct{}

type Show struct {
	ID *int `arg:"" optional:"" help:"Optional document ID to show details for a specific document"`
}

type StatsCmd struct{}

type Remove struct {
	ID int `arg:"" help:"Document ID to remove"`
}

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Warning: using default config: %v", err)
	}
	embedding.BaseURL = cfg.EmbeddingBaseURL
	embedding.Model = cfg.EmbeddingModel
	embedding.APIKey = cfg.APIKey

	// Parse command-line arguments
	var cli CLI
	kctx := kong.Parse(&cli)

	// Setup database
	database, new, err := db.CreateDB(cli.Database)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	defer database.Close()

	if new {
		// Test embedding model as well as get the embedding size
		sampleEmbedding, err := embedding.CreateEmbedding(ctx, "refer")
		if err != nil {
			log.Fatalf("Failed to create embedding: %v", err)
		}

		err = db.InitDatabase(database, len(sampleEmbedding))
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}

		err = db.SaveConfig(
			database,
			map[string]string{
				"embedding_model": embedding.Model,
				"embedding_size":  fmt.Sprintf("%d", len(sampleEmbedding)),
			})
		if err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
	}

	if !new {
		if kctx.Command() == "add <file-path>" || kctx.Command() == "search" {
			// Check that the embedding model in the database matches the
			// one in the config only if the command is add or
			// search. This is necessary as the models must match for the
			// results to be usable.
			config, err := db.GetConfig(database)
			if err != nil {
				log.Fatalf("Failed to get config: %v", err)
			}

			if config["embedding_model"] != cfg.EmbeddingModel {
				fmt.Fprintf(
					os.Stderr,
					"Database embedding model does not match config: %s != %s\n"+
						"Please reindex the documents or update the model\n",
					config["embedding_model"],
					cfg.EmbeddingModel)

				os.Exit(1)
			}
		}
	}

	// Handle commands
	switch kctx.Command() {
	case "add <file-path>":
		for _, f := range cli.Add.FilePath {
			if webutil.IsURL(f) {
				if err := fileutil.AddDocument(ctx, database, f); err != nil {
					log.Printf("Failed to add document %q: %v", f, err)
				}
			} else {
				err = filepath.WalkDir(f, func(path string, dirEntry fs.DirEntry, err error) error {
					if err != nil {
						log.Printf("Failed to walk directory %q: %v", f, err)
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
					log.Printf("Failed to walk directory %q: %v", f, err)
				}
			}
		}
	case "search":
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Failed to read from stdin: %v", err)
		}

		if len(input) == 0 {
			log.Fatalf("No input provided")
		}

		cli.Search.Query = string(input)

		fallthrough
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
	case "reindex":
		sampleEmbedding, err := embedding.CreateEmbedding(ctx, "refer")
		if err != nil {
			log.Fatalf("Failed to create embedding: %v", err)
		}

		embeddingSize := len(sampleEmbedding)

		docs, err := db.RecreateDatabase(database, embeddingSize)
		if err != nil {
			log.Fatalf("Failed to reindex database: %v", err)
		}

		err = db.SaveConfig(
			database,
			map[string]string{
				"embedding_model": embedding.Model,
				"embedding_size":  fmt.Sprintf("%d", embeddingSize),
			})
		if err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}

		for _, doc := range docs {
			if err := fileutil.AddDocument(ctx, database, doc); err != nil {
				log.Printf("Failed to reindex document %q: %v", doc, err)
			}
		}

		fmt.Printf("Successfully reindexed %d documents\n", len(docs))
	case "show":
		// List all documents
		docs, err := db.GetAllDocuments(database)
		if err != nil {
			log.Fatalf("Failed to get documents: %v", err)
		}
		if len(docs) == 0 {
			log.Println("No documents found in database")
			return
		}
		for _, doc := range docs {
			fmt.Printf("[%d] %s\n", doc.ID, doc.Path)
		}
	case "show <id>":
		// Show specific document
		doc, err := db.GetDocumentByID(database, *cli.Show.ID)
		if err != nil {
			log.Fatalf("Failed to get document with ID %d: %v", *cli.Show.ID, err)
		}
		if doc == nil {
			log.Fatalf("No document found with ID %d", *cli.Show.ID)
		}
		fmt.Printf("%s\n%s\n", doc.Path, doc.Content)
	case "stats":
		stats, err := db.GetDatabaseStats(database)
		if err != nil {
			log.Fatalf("Failed to get database stats: %v", err)
		}

		fmt.Printf("Documents: %d\n", stats["documents"])
		fmt.Printf("Total Content Size: %s\n", formatBytes(stats["total_content_bytes"]))
	case "remove <id>":
		if err := db.RemoveDocument(database, cli.Remove.ID); err != nil {
			log.Fatalf("Failed to remove document: %v", err)
		}
		fmt.Printf("Document %d removed successfully\n", cli.Remove.ID)
	default:
		panic("Unexpected command: " + kctx.Command())
	}
}

func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
