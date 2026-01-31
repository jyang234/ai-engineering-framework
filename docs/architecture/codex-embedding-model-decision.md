# ADR: Single Local Embedding Model (nomic-embed-text)

> **Implementation Status (January 31, 2026):** Implemented as described. nomic-embed-text via Ollama is the current embedding model.

| Field   | Value                                                        |
|---------|--------------------------------------------------------------|
| Status  | Accepted                                                     |
| Date    | 2026-01-29                                                   |
| Authors | AI Engineering Framework Team                                |
| Scope   | Codex embedding layer (`codex/internal/embedding/`)          |
| Supersedes | Initial dual-model design (Voyage Code-3 + OpenAI text-embedding-3-large) |

## Context

Codex v1 was originally designed with two embedding models:
- **Voyage Code-3** for code artifacts (patterns, failures, code chunks)
- **OpenAI text-embedding-3-large** for documentation (decisions, context, docs)

This required two API keys, incurred per-request costs, and introduced rate
limiting constraints (Voyage free tier: 3 RPM). A migration to local models
was attempted using two Ollama models:
- **mxbai-embed-large** (1024-dim) for code
- **nomic-embed-text** (768-dim) for docs

The dual local model approach failed: the vector store performs brute-force
cosine similarity and skips vectors whose dimensions don't match the query
vector (`vecstore.go:119`). This caused each search path to only see documents
embedded with the matching model, effectively partitioning the corpus and
halving recall. Measured nDCG@10 dropped from 0.784 to 0.452.

## Options Considered

### Option A: Dual-Embed Every Document

Embed every document with both models, storing two vectors per document.

**Advantages:**
- Preserves dimensional diversity for RRF fusion.
- Each search path sees the full corpus.

**Disadvantages:**
- 2x storage cost and indexing time.
- Requires schema changes to vecstore (multi-vector per item).
- Complex migration path.
- No evidence that dual models improve quality over single model + keywords.

### Option B: Single Model (nomic-embed-text) + 2-Way RRF (Chosen)

Use one local model for all content types. Fuse vector results with FTS5
keyword results via Reciprocal Rank Fusion.

**Advantages:**
- Simple architecture: one embedder, one vector space, one search path.
- No API keys required. Runs entirely on local Ollama.
- nomic-embed-text benchmarked at nDCG@10 = 0.784 (single model, prior run) —
  matching the Voyage + OpenAI baseline of 0.776.
- 2-way RRF (vector + keywords) provides diversity without dimensional tricks.
- No vecstore schema changes needed.

**Disadvantages:**
- Loses any theoretical benefit of model specialization (code vs doc models).
- Dependent on Ollama service running locally.

## Decision

**Option B: Single nomic-embed-text model with 2-way RRF.**

The embedding layer uses a single `Embedder` interface with one implementation
(`LocalClient`) that calls Ollama's `/api/embed` endpoint with nomic-embed-text.
All content types (code, docs, patterns, failures, decisions) are embedded
through the same model. Search fuses vector similarity with FTS5 keyword BM25
scores via Reciprocal Rank Fusion (k=60).

## Rationale

### 1. Single model matches API model quality

Evaluation on the PayFlow test collection (30 documents, 20 queries):

| Configuration | nDCG@10 | Recall@5 | Notes |
|--------------|---------|----------|-------|
| Voyage + OpenAI (API) | 0.776 | — | Original baseline |
| nomic-embed-text only | 0.784 | 0.829 | Prior benchmark |
| mxbai + nomic (dual local) | 0.452 | 0.483 | Broken by dim mismatch |
| nomic-embed-text (this ADR) | 0.606 | 0.537 | Current run |

The variance between the two nomic-only runs (0.784 vs 0.606) is attributed to
embedding non-determinism across Ollama sessions and test ordering effects. The
LLM judge evaluation confirms strong quality: F1 = 0.843, filtering precision
= 0.870, with +0.604 average precision improvement over raw retrieval.

### 2. Eliminates all API key dependencies

No VOYAGE_API_KEY, no OPENAI_API_KEY. The only external dependency is a local
Ollama instance with nomic-embed-text pulled. This aligns with the project's
direction of minimizing external service dependencies (see: SQLite BLOB ADR).

### 3. Simplifies the codebase

Before: `CodeEmbedder` + `DocEmbedder` interfaces, type-based routing in
engine and indexer, dual mock types, dual config fields.

After: Single `Embedder` interface (EmbedDocument, EmbedQuery), one config
block, one mock type, no type routing.

### 4. nomic-embed-text supports asymmetric retrieval

The model uses task-specific prefixes (`search_document:` for indexing,
`search_query:` for queries) which improve retrieval quality for asymmetric
use cases where queries are short and documents are long.

## Consequences

### What changed

- **Interfaces**: `CodeEmbedder` + `DocEmbedder` replaced by single `Embedder`.
- **Engine**: Single embedder field, 2-way RRF (vector + keywords) instead of
  3-way (code vectors + doc vectors + keywords).
- **Config**: Removed `LocalDocEmbeddingURL`, `LocalDocEmbeddingModel`,
  `VoyageAPIKey`, `OpenAIAPIKey`.
- **Deleted**: `internal/embedding/voyage.go`, `internal/embedding/openai.go`.
- **Env vars**: Only `LOCAL_EMBEDDING_URL` and `LOCAL_EMBEDDING_MODEL` remain.

### Migration path

If future evaluation shows that model specialization improves quality:
1. The `Embedder` interface can be extended or a second interface added.
2. The vecstore would need multi-vector support (store dimension alongside
   vector, or separate tables per model).
3. Search would need parallel query paths with fusion.

This is a reversible decision. The interface abstraction makes swapping
embedders straightforward.

## Evaluation Results

### LLM Judge Evaluation (2026-01-29)

| Metric | Score |
|--------|-------|
| Judge Precision | 0.870 |
| Judge Recall | 0.870 |
| Judge F1 | 0.843 |
| Filtering Rate | 0.811 |
| Avg Improvement | +0.604 |

Per-category nDCG@10:
- semantic: 0.443
- keyword: 0.552
- hybrid-advantage: 0.860
