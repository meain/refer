package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	_ "github.com/mattn/go-sqlite3"
	"github.com/meain/refer/internal"
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
	FilePath []string `arg:"" required:"" help:"File, directory or URL to add to the database"`
}

type Search struct {
	Query     string   `arg:"" optional:"" help:"Search query to be executed"`
	Format    string   `default:"names" help:"Format of the search results"`
	Limit     int      `default:"5" help:"Maximum number of search results to return"`
	Threshold *float64 `help:"Maximum distance threshold for search results (20 is a good value)"`
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
	cfg, err := internal.LoadConfig()
	if err != nil {
		log.Printf("Warning: using default config: %v", err)
	}

	// Parse command-line arguments
	var cli CLI
	kctx := kong.Parse(&cli)

	// Setup database
	database, new, err := internal.CreateDB(cli.Database)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	defer database.Close()

	if new {
		// Test embedding model as well as get the embedding size
		sampleEmbedding, err := internal.CreateEmbedding(ctx, "refer")
		if err != nil {
			log.Fatalf("Failed to create embedding: %v", err)
		}

		err = internal.InitDatabase(database, len(sampleEmbedding))
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}

		err = internal.SaveConfig(
			database,
			map[string]string{
				"embedding_model": internal.Model,
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
			config, err := internal.GetConfig(database)
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
		var allPaths []string
		for _, f := range cli.Add.FilePath {
			if internal.IsRemoteURL(f) {
				allPaths = append(allPaths, f)
			} else {
				err = filepath.WalkDir(f, func(path string, dirEntry fs.DirEntry, err error) error {
					if err != nil {
						log.Printf("Failed to walk directory %q: %v", f, err)
						return err
					}
					if !dirEntry.IsDir() {
						allPaths = append(allPaths, path)
					}
					return nil
				})
				if err != nil {
					log.Printf("Failed to walk directory %q: %v", f, err)
				}
			}
		}

		// Process documents in parallel
		if errors := internal.AddDocuments(ctx, database, allPaths, 5); len(errors) > 0 {
			for _, err := range errors {
				log.Printf("Error: %v", err)
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
		queryEmbedding, err := internal.CreateEmbedding(ctx, cli.Search.Query)
		if err != nil {
			log.Fatalf("Failed to create query embedding: %v", err)
		}

		// Perform search
		docs, err := internal.SearchDocuments(
			database,
			queryEmbedding,
			cli.Search.Limit,
			cli.Search.Threshold)
		if err != nil {
			log.Fatalf("Search failed: %v", err)
		}

		switch cli.Search.Format {
		case "names":
			PrintNameResults(docs)
		case "llm":
			PrintLLMResults(docs)
		default:
			log.Fatalf("Unknown format: %s", cli.Search.Format)
		}
	case "reindex":
		sampleEmbedding, err := internal.CreateEmbedding(ctx, "refer")
		if err != nil {
			log.Fatalf("Failed to create embedding: %v", err)
		}

		embeddingSize := len(sampleEmbedding)

		docs, err := internal.RecreateDatabase(database, embeddingSize)
		if err != nil {
			log.Fatalf("Failed to reindex database: %v", err)
		}

		err = internal.SaveConfig(
			database,
			map[string]string{
				"embedding_model": internal.Model,
				"embedding_size":  fmt.Sprintf("%d", embeddingSize),
			})
		if err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}

		// Process documents in parallel
		if errors := internal.AddDocuments(ctx, database, docs, 5); len(errors) > 0 {
			for _, err := range errors {
				log.Printf("Error during reindex: %v", err)
			}
		}

		fmt.Printf("Successfully reindexed %d documents\n", len(docs))
	case "show":
		// List all documents
		docs, err := internal.GetAllDocuments(database)
		if err != nil {
			log.Fatalf("Failed to get documents: %v", err)
		}
		if len(docs) == 0 {
			log.Println("No documents found in database")
			return
		}
		for _, doc := range docs {
			fmt.Printf("[%d] %s\n", doc.ID, doc.Title)
		}
	case "show <id>":
		// Show specific document
		doc, err := internal.GetDocumentByID(database, *cli.Show.ID)
		if err != nil {
			log.Fatalf("Failed to get document with ID %d: %v", *cli.Show.ID, err)
		}
		if doc == nil {
			log.Fatalf("No document found with ID %d", *cli.Show.ID)
		}
		fmt.Printf("%s\n%s\n", doc.Path, doc.Content)
	case "stats":
		stats, err := internal.GetDatabaseStats(database)
		if err != nil {
			log.Fatalf("Failed to get database stats: %v", err)
		}

		fmt.Printf("Documents: %d\n", stats["documents"])
		fmt.Printf("Total Content Size: %s\n", formatBytes(stats["total_content_bytes"]))
	case "remove <id>":
		if err := internal.RemoveDocument(database, cli.Remove.ID); err != nil {
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

func PrintNameResults(docs []internal.Document) {
	for _, doc := range docs {
		fmt.Printf("%d: %s (%.4f)\n", doc.ID, doc.Path, doc.Distance)
	}
}

func PrintLLMResults(docs []internal.Document) {
	// Print results in LLM format
	for _, doc := range docs {
		fmt.Printf("File: %s\nTitle: %s\n\n%s\n---\n", doc.Path, doc.Title, doc.Content)
	}
}
