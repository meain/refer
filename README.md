# refer

> Unlock Meaningful Insights: Effortless Semantic Search Across Your Local Files

`refer` is a command-line tool for semantic search across your local files using embeddings. It allows you to find relevant files based on meaning rather than just keyword matching.

https://github.com/user-attachments/assets/efc8c7fe-9fa3-43d4-9372-5af346591829

_View the video on [Youtube](https://youtu.be/K5LfqEMUwL0) if you are having trouble viewing it here._

## Features

- Semantic search using text embeddings
- Support for recursive directory scanning
- Support for indexing web pages
- Multiple output formats (file names or full content)
- SQLite-based vector storage for fast similarity search
- Document management (add, remove, reindex)

## Configuration

`refer` can be configured via a JSON file located at `~/.config/refer/config.json`.
The following settings are available:

```json
{
    "embedding_base_url": "http://localhost:11434/api/embeddings",
    "embedding_model": "nomic-embed-text",
    "api_key": "" // Optional API key
}
```

- `embedding_base_url`: The URL of embedding API endpoint
- `embedding_model`: The embedding model to use
- `api_key`: Optional API key for authorization. **It is recommended to pass this via the `REFER_API_KEY` environment variable for better security.**

If no config file is present, these default values will be used.
You can also use any provider that supports the OpenAI format for embedding API.

_If both `REFER_API_KEY` environment variable and `api_key` config value is set, the env variable takes precedence._

### Embedding API

The embedding API can be any server that provides an interface compliant with the [OpenAI embeddings specification](https://platform.openai.com/docs/api-reference/embeddings), such as Ollama or OpenAI.

By default, `refer` is configured to use Ollama, which is recommended since most machines can efficiently run an embedding model without any cost, rate limits, or privacy concerns. For setup instructions, please visit [Ollama](https://ollama.com).

If you'd like to use the OpenAI API instead, configure it with the following settings:

```json
{
    "embedding_base_url": "https://api.openai.com/v1/embeddings",
    "embedding_model": "text-embedding-v1",
    "api_key": "<your openai api key>"
}
```

For other providers, please consult their respective documentation.

## Authorization

You can optionally set the `REFER_API_KEY` environment variable to provide an authorization token for the API. This token will be included in the request header as `Authorization: Bearer $REFER_API_KEY`. If you are using Ollama, you can keep this variable empty.

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

Add files while respecting gitignore patterns:
```bash
refer add path/to/directory --ignore
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

Search on input (returns file names and similarity scores):
```bash
refer search "your search query"
```

Search based on stdin
```bash
cat file-name | refer search
echo "output from other command" | refer search
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

Max distance threshold:

``` bash
refer search "your search query" --threshold=20
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

Inspired by [inkeep search
widget](https://inkeep.com/showcase?example=pinecone&tab=aiForCustomers)
and [jkitchin/litdb](https://github.com/jkitchin/litdb).
