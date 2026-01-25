# Codex v1

Production-grade knowledge retrieval system for EDI. Upgrades RECALL from SQLite FTS to hybrid vector + BM25 search with multi-stage reranking.

## Features

- **Hybrid Search**: Qdrant vector search + BM25 sparse search with RRF fusion
- **Code Embeddings**: Voyage Code-3 for optimized code retrieval
- **Doc Embeddings**: OpenAI text-embedding-3-large for documentation
- **Multi-stage Reranking**: BGE rerankers (base → v2-m3) via ONNX/Hugot
- **AST Chunking**: Tree-sitter for semantic code extraction
- **Contextual Retrieval**: Claude Haiku for document chunk enrichment
- **Web UI**: Browse and search the knowledge base
- **MCP Server**: Drop-in replacement for RECALL v0

## Quick Start

```bash
# Start Qdrant
make qdrant-up

# Build binaries
make build

# Run MCP server (for EDI)
./bin/recall-mcp

# Run web UI
./bin/codex-web
```

## Environment Variables

```bash
# Required
VOYAGE_API_KEY=voy-xxx
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx

# Optional
QDRANT_ADDR=localhost:6334
CODEX_COLLECTION=recall
CODEX_WEB_ADDR=:8080
CODEX_MODELS_PATH=./models
CODEX_METADATA_DB=~/.edi/codex.db
EDI_SESSION_ID=xxx  # Set by EDI launcher
```

## Project Structure

```
codex/
├── cmd/
│   ├── recall-mcp/     # MCP server binary
│   ├── codex-web/      # Web UI binary
│   └── codex-cli/      # Admin CLI
├── internal/
│   ├── core/           # Search/index orchestration
│   ├── storage/        # Qdrant + SQLite metadata
│   ├── chunking/       # AST + contextual chunking
│   ├── embedding/      # Voyage + OpenAI clients
│   ├── reranking/      # Hugot BGE models
│   ├── mcp/            # MCP protocol handlers
│   └── web/            # Gin web handlers
├── web/
│   ├── templates/      # HTML templates
│   └── static/         # CSS/JS assets
└── models/             # ONNX models (gitignored)
```

## Migration from RECALL v0

```bash
# Migrate existing SQLite FTS data to Qdrant
./bin/codex-cli migrate
```

## Development

```bash
# Run tests
make test

# Lint
make lint

# Format
make fmt
```

## License

Part of the AI Engineering Framework (AEF).
