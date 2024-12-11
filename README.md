# lit - Local Intelligence Tool

`lit` is a command-line tool for semantic search across your local files using embeddings. It allows you to find relevant files based on meaning rather than just keyword matching.

## Features

- Semantic search using text embeddings (powered by Ollama's nomic-embed-text model)
- Support for recursive directory scanning
- Multiple output formats (file names or full content)
- SQLite-based vector storage for fast similarity search

## Prerequisites

- Go 1.23 or later
- [Ollama](https://ollama.ai) running locally with the `nomic-embed-text` model

## Configuration

`lit` can be configured via a JSON file located at `~/.config/lit/config.json`. The following settings are available:

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
go install github.com/meain/lit@latest
```

## Usage

### Adding Files

Add a single file:
```bash
lit add path/to/file.txt
```

Add files recursively from a directory:
```bash
lit add path/to/directory --recursive
```

### Searching

Search files (returns file names and similarity scores):
```bash
lit search "your search query"
```

Get full content matches:
```bash
lit search "your search query" --format=llm
```

Limit results:
```bash
lit search "your search query" --limit=10
```

## How it Works

1. When adding files, `lit`:
   - Checks if they are text files
   - Generates embeddings using the nomic-embed-text model
   - Stores the file path, content, and embedding in SQLite

2. When searching:
   - Generates an embedding for your search query
   - Uses SQLite's vector similarity search to find matches
   - Returns results sorted by relevance
