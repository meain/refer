# refer

> Unlock Meaningful Insights: Effortless Semantic Search Across Your Local Files

`refer` is a command-line tool for semantic search across your local files using embeddings. It allows you to find relevant files based on meaning rather than just keyword matching.

https://github.com/user-attachments/assets/efc8c7fe-9fa3-43d4-9372-5af346591829

_View the video on [Youtube](https://youtu.be/K5LfqEMUwL0) if you are having trouble viewing it here._

## Features

- Semantic search using text embeddings (powered by Ollama's nomic-embed-text model)
- Support for recursive directory scanning
- Support for indexing web pages
- Multiple output formats (file names or full content)
- SQLite-based vector storage for fast similarity search
- Document management (add, remove, reindex)

## Prerequisites

- Go 1.23 or later
- [Ollama](https://ollama.ai) running locally with the `nomic-embed-text` model

## Configuration

`refer` can be configured via a JSON file located at `~/.config/refer/config.json`. The following settings are available:

```json
{
    "embedding_base_url": "http://localhost:11434/api/embeddings",
    "embedding_model": "nomic-embed-text"
}
```

- `embedding_base_url`: The URL of your Ollama API endpoint
- `embedding_model`: The embedding model to use

If no config file is present, these default values will be used.

## Installation

```bash
go install github.com/meain/refer@latest
```

## Usage

### Adding Content

Add a single file:
```bash
refer add path/to/file.txt
```

Add files recursively from a directory:
```bash
refer add path/to/directory
```

Add a web page:
```bash
refer add https://example.com/page.html
```

### Managing Documents

Show all indexed documents:
```bash
refer show
```

Show specific document details:
```bash
refer show <id>
```

Remove a document:
```bash
refer remove <id>
```

Reindex all documents:
```bash
refer reindex
```

View database statistics:
```bash
refer stats
```

### Searching

Search files (returns file names and similarity scores):
```bash
refer search "your search query"
```

Use a different database file:
```bash
refer --database=/path/to/referdb search "query"
```

Get full content matches:
```bash
refer search "your search query" --format=llm
```

Limit results:
```bash
refer search "your search query" --limit=10
```

## How it Works

1. When adding files, `refer`:
   - Checks if they are text files
   - Generates embeddings using the nomic-embed-text model
   - Stores the file path, content, and embedding in SQLite

2. When searching:
   - Generates an embedding for your search query
   - Uses SQLite's vector similarity search to find matches
   - Returns results sorted by relevance

---

Inspired by [jkitchin/litdb](https://github.com/jkitchin/litdb).
