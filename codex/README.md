# Codex

Hybrid search engine for a local knowledge base. Combines semantic vector search with FTS5 keyword search, fused via Reciprocal Rank Fusion, all in a single SQLite file.

## How It Works

Every search runs two retrieval paths in parallel and fuses the results:

```
Query → "how did we handle auth?"
  │
  ├─► Vector search (semantic similarity via nomic-embed-text embeddings)
  ├─► FTS5 keyword search (BM25 ranking)
  │
  └─► Reciprocal Rank Fusion → merged, ranked results
```

This handles both vague semantic queries ("something about retry logic") and precise keyword lookups ("idempotency key") well.

## Why It Matters

- **Nothing leaves your machine.** Embeddings are generated locally via Ollama. Your code patterns, architecture decisions, and failure logs stay on your disk.
- **Zero external dependencies.** No API keys needed for core search. Install Ollama, pull the model, and it works.
- **Everything in one file.** Metadata, vector embeddings, FTS5 index, feedback, and flight recorder — all in `~/.edi/codex.db`.
- **Drop-in upgrade from RECALL v0.** Same 5-tool MCP interface. One config change (`backend: codex`) switches from keyword-only to hybrid search.

## Getting Started

### Prerequisites

- **Go 1.22+** with CGO enabled
- **Ollama** with `nomic-embed-text`

```bash
# Install Ollama: https://ollama.com
ollama pull nomic-embed-text
```

### Build and Run

```bash
make build   # Builds all binaries with -tags "fts5"
cp bin/recall-mcp ~/.edi/bin/
```

Enable in your EDI config:

```yaml
# ~/.edi/config.yaml or .edi/config.yaml
recall:
  enabled: true
  backend: codex
```

### Standalone Usage

Codex also works outside EDI for indexing and admin tasks:

```bash
./bin/codex-cli index ./path/to/project
./bin/codex-cli search "error handling pattern" --type pattern
./bin/codex-cli status
CODEX_API_KEY=my-secret ./bin/codex-web  # → http://localhost:8080
```

## Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|---|---|---|
| `CODEX_METADATA_DB` | `~/.edi/codex.db` | SQLite database path |
| `LOCAL_EMBEDDING_URL` | `http://localhost:11434` | Ollama API base URL |
| `LOCAL_EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model name |
| `CODEX_API_KEY` | _(none)_ | Bearer token auth for web UI and MCP |
| `CODEX_WEB_ADDR` | `:8080` | Web server listen address |
| `CODEX_MODELS_PATH` | `./models` | Directory for local model files |

## Project Structure

```
codex/
├── cmd/
│   ├── recall-mcp/        # MCP server (what EDI launches)
│   ├── codex-cli/         # Admin CLI (index, search, migrate, status)
│   ├── codex-web/         # Web UI
│   └── codex-testgen/     # Evaluation test data server
├── internal/
│   ├── core/              # SearchEngine, Indexer, RRF fusion
│   ├── storage/           # SQLite metadata + vector BLOBs + FTS5
│   ├── embedding/         # Ollama client (nomic-embed-text, 768-dim)
│   ├── chunking/          # AST (Tree-sitter) + markdown chunking
│   ├── reranking/         # Reranker interfaces (planned)
│   ├── mcp/               # JSON-RPC stdio MCP server
│   └── web/               # Gin HTTP server + REST API
├── eval/                  # Evaluation harness, metrics, LLM judge
└── web/                   # HTML templates + static assets
```

## Development

```bash
make build    # Build all binaries
make test     # Run tests
make lint     # Run linter
make fmt      # Format code
```

## Further Reading

- [AEF Overview](../README.md) — the big picture, quick start, and component map
- [EDI + Codex Technical Deep-Dive](../docs/edi-codex-deep-dive.md) — full system architecture and data flows

## License

Part of the AI Engineering Framework (AEF).
