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
- SQLite with [sqlite-vec](https://github.com/asg017/sqlite-vec) extension

## Installation

```bash
go install github.com/yourusername/lit@latest
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

## License

MIT License - see LICENSE file for details
