# ADR: Replace Qdrant with SQLite BLOBs + Brute-Force KNN

> **Implementation Status (January 31, 2026):** Implemented as described. SQLite BLOB + brute-force KNN is the current storage architecture.

| Field   | Value                                                        |
|---------|--------------------------------------------------------------|
| Status  | Accepted                                                     |
| Date    | 2026-01-27                                                   |
| Authors | AI Engineering Framework Team                                |
| Scope   | Codex vector storage layer (`codex/internal/core/`)          |

## Context

The Codex component requires a vector storage backend for embedding-based
semantic search over project documentation, code snippets, and knowledge
fragments. The current implementation stubs out Qdrant as the vector store but
contains no working integration.

Key facts about the current state:

- **Qdrant integration is entirely stubbed.** No client code, no connection
  management, no schema definitions exist. The cost of switching is zero.
- **The project already depends on `mattn/go-sqlite3` with CGO enabled** and
  the `-tags "fts5"` build constraint for full-text search. SQLite is already a
  first-class runtime dependency.
- **Expected document scale is under 10,000 vectors.** Codex indexes a single
  project's artifacts: source files, documentation pages, and knowledge
  fragments. Even large monorepos rarely exceed a few thousand indexable units.
- **Retrieval quality is determined by the embedding model and optional
  reranking, not by the nearest-neighbor algorithm.** At exact KNN, the vector
  store is a pure storage and distance-computation layer.

## Options Considered

### Option A: Qdrant (External Service)

Qdrant is a purpose-built vector database offering HNSW-based approximate
nearest neighbor (ANN) search, filtering, payload storage, and a gRPC/REST API.

**Advantages:**
- Mature ANN indexing (HNSW) for large-scale workloads (100K+ vectors).
- Built-in filtering and payload management.
- Active open-source community.

**Disadvantages:**
- Requires a Docker container or managed service at runtime, adding significant
  deployment complexity for a developer tool.
- gRPC client generation adds build complexity and dependency weight.
- HNSW indexing provides no measurable benefit at the expected scale (<10K
  vectors). The index construction overhead may actually slow small workloads.
- Introduces a network hop for every query, adding latency and a failure mode.

### Option B: sqlite-vec Extension

The `sqlite-vec` extension (by Alex Garcia) adds virtual-table-based vector
search to SQLite, supporting both exact KNN and experimental ANN indexes.

**Advantages:**
- Stays within the SQLite ecosystem.
- SQL-native query interface via virtual tables.

**Disadvantages:**
- **Pre-v1 software** with explicit warnings about breaking changes to the
  storage format and API. Production use is discouraged by the author.
- No support for the `modernc.org/sqlite` pure-Go driver; requires CGO with
  manual extension loading.
- At the scales relevant to Codex (<10K vectors), `sqlite-vec` performs
  brute-force KNN internally -- the same approach as Option C, but with
  additional abstraction overhead and stability risk.
- Extension loading introduces platform-specific shared library management
  (.so/.dylib/.dll) that complicates cross-compilation.

### Option C: SQLite BLOBs + Brute-Force Go Cosine Similarity (Chosen)

Store embedding vectors as BLOB columns in a standard SQLite table. Perform
exact KNN in Go application code using cosine similarity computed over
`float32` slices.

**Advantages:**
- Zero additional dependencies beyond the existing `mattn/go-sqlite3`.
- Sub-millisecond query latency at expected scale (see Performance Analysis).
- Exact results with no approximation error.
- Full control over the similarity computation, enabling straightforward
  profiling and optimization.
- No platform-specific extension loading.
- Testable with in-memory SQLite databases.

**Disadvantages:**
- Brute-force scan becomes impractical above approximately 100K vectors (see
  Performance Analysis). A migration path is required if scale increases.
- No built-in vector-aware filtering; application code must handle
  post-retrieval filtering or pre-filtering via SQL before similarity
  computation.

## Decision

**Option C: SQLite BLOBs + brute-force Go cosine similarity.**

The vector storage layer will:

1. Store embeddings as `BLOB` columns containing little-endian `float32`
   arrays in a standard SQLite table alongside metadata columns.
2. On query, load candidate vectors into Go memory and compute cosine
   similarity using a straightforward `float32` dot-product loop.
3. Return the top-K results sorted by descending similarity score.

## Rationale

### 1. Brute-force KNN is sub-millisecond at expected scale

The computational cost of brute-force cosine similarity is O(N * D) where N is
the number of vectors and D is the dimensionality. For typical embedding
models:

| Vectors (N) | Dimensions (D) | Memory (float32) | Distance Computations | Expected Latency |
|-------------|-----------------|-------------------|-----------------------|------------------|
| 1,000       | 768             | 3 MB              | 768,000 FLOPs         | <0.5 ms          |
| 10,000      | 768             | 30 MB             | 7.68M FLOPs           | 2-5 ms           |
| 100,000     | 768             | 300 MB            | 76.8M FLOPs           | 20-50 ms         |

At 1K vectors (the common case), the entire dataset fits in L2 cache on modern
CPUs. The 768K floating-point multiply-accumulate operations complete in under
0.5 ms on any recent x86-64 or ARM64 processor. Even at 10K vectors, latency
remains well within acceptable bounds for an interactive developer tool.

At 100K vectors, brute-force latency rises to tens of milliseconds and memory
consumption reaches 300 MB. This is the practical upper bound for this approach.
Beyond 100K vectors, an ANN index becomes justified.

### 2. sqlite-vec does brute-force anyway

At the scales relevant to Codex, `sqlite-vec` performs an exhaustive scan
internally. Its `knn` virtual table iterates over all stored vectors and
computes distances, which is algorithmically identical to Option C. The ANN
index support in `sqlite-vec` is experimental and not recommended for
production. Adopting `sqlite-vec` would add a pre-v1 dependency to perform the
same computation that 20 lines of Go accomplish directly.

### 3. HNSW provides no value below 100K vectors

Hierarchical Navigable Small World (HNSW) graphs trade index construction time
and memory overhead for sub-linear query time. The crossover point where HNSW
outperforms brute-force depends on dataset size:

- **Below 10K vectors:** HNSW index construction and graph traversal overhead
  often exceeds brute-force scan time. The graph structure provides no latency
  benefit.
- **10K-100K vectors:** HNSW begins to show marginal latency improvements, but
  brute-force remains acceptable (single-digit milliseconds).
- **Above 100K vectors:** HNSW provides significant (10-100x) speedup. This is
  its intended operating range.

Qdrant's value proposition is its HNSW implementation. At Codex's scale, that
value is unrealized, while the operational cost (Docker, networking, gRPC) is
fully incurred.

### 4. Zero retrieval quality loss

Retrieval quality in a semantic search pipeline is determined by:

1. **Embedding model quality** -- the representation learned by the encoder.
2. **Reranking** -- cross-encoder or LLM-based reranking of candidate results.
3. **Chunking strategy** -- how source documents are segmented.

The nearest-neighbor algorithm is a pure distance computation. Exact KNN
(brute-force) returns the mathematically optimal result set. ANN algorithms
like HNSW introduce controlled recall loss (typically 95-99% recall at
optimized settings) in exchange for speed. By using exact KNN, Option C
produces strictly superior retrieval results compared to any ANN approach.

### 5. Eliminates a runtime dependency

Qdrant requires either a Docker container or a managed cloud service. For a
developer tool that runs locally, requiring Docker significantly increases
friction. Option C embeds all storage in a single SQLite file that requires no
external processes, no network configuration, and no container orchestration.

## Performance Analysis

### Brute-Force Cosine Similarity Cost Model

Cosine similarity between two vectors of dimension D requires:
- D multiplications + (D-1) additions for dot product (x2 for norms if not
  pre-computed, but norms can be cached)
- With pre-computed norms: D multiplications + D additions per comparison

For N candidate vectors: N * (2D) floating-point operations.

**Benchmark projections** (single-threaded, ARM64 Apple Silicon, Go 1.22):

| N       | D    | FLOPs   | Projected Latency | Memory   |
|---------|------|---------|-------------------|----------|
| 1,000   | 384  | 768K    | ~0.2 ms           | 1.5 MB   |
| 1,000   | 768  | 1.5M    | ~0.4 ms           | 3.0 MB   |
| 1,000   | 1536 | 3.0M    | ~0.8 ms           | 6.0 MB   |
| 10,000  | 384  | 7.7M    | ~2.0 ms           | 15 MB    |
| 10,000  | 768  | 15.4M   | ~4.0 ms           | 30 MB    |
| 10,000  | 1536 | 30.7M   | ~8.0 ms           | 60 MB    |
| 100,000 | 768  | 153.6M  | ~40 ms            | 300 MB   |

These projections assume single-threaded execution with no SIMD intrinsics.
Performance can be improved 2-4x with goroutine parallelism over vector
batches, and further with SIMD-optimized dot product functions if needed.

### Storage Cost Model

SQLite BLOB storage overhead is minimal. Each embedding row consists of:

- BLOB column: `D * 4` bytes (float32 encoding)
- Metadata columns: ~200 bytes (document ID, chunk ID, timestamps)
- SQLite row overhead: ~50 bytes

Total per row at 768 dimensions: approximately 3,300 bytes.

| Vectors | Storage (768-dim) |
|---------|-------------------|
| 1,000   | ~3.3 MB           |
| 10,000  | ~33 MB            |
| 100,000 | ~330 MB           |

## Consequences

### What stays unchanged

- **Embedding pipeline:** Document chunking, embedding model selection, and
  embedding generation are unaffected. The storage layer receives pre-computed
  vectors.
- **Query pipeline:** Query embedding, top-K retrieval, optional reranking, and
  result formatting remain the same. The storage layer is called via an
  interface that any backend can implement.
- **FTS5 full-text search:** The existing SQLite FTS5 integration for keyword
  search is independent and unaffected.
- **Storage interface contract:** The `VectorStore` interface (Store, Search,
  Delete) remains unchanged. Only the implementation behind it changes.

### What changes

- **Storage backend:** The `VectorStore` implementation moves from a Qdrant
  stub to a concrete SQLite + Go implementation.
- **Deployment:** No Docker container or external service is required for
  vector storage. The entire Codex data layer is a single SQLite file.
- **Test infrastructure:** Vector storage tests use in-memory SQLite databases,
  eliminating the need for test containers or service mocks.

### Migration path if scale increases

If a future use case requires indexing beyond 100K vectors:

1. **First option (10K-100K):** Add goroutine parallelism and optional SIMD
   optimization to the brute-force implementation. This extends the practical
   ceiling to approximately 500K vectors at acceptable latency.
2. **Second option (100K-1M):** Integrate `sqlite-vec` once it reaches v1
   stability, gaining ANN indexing while remaining in-process.
3. **Third option (1M+):** Introduce an external vector database (Qdrant,
   Milvus, pgvector) behind the existing `VectorStore` interface. The
   interface abstraction ensures this is a localized change.

The `VectorStore` interface is designed to make this migration transparent to
all upstream code.

## References

- Malkov, Y. A., & Yashunin, D. A. (2018). "Efficient and robust approximate
  nearest neighbor search using Hierarchical Navigable Small World graphs."
  IEEE TPAMI. -- Establishes HNSW complexity and the scale at which
  approximate methods outperform exact search.
- Thakur, N., et al. (2021). "BEIR: A Heterogeneous Benchmark for Zero-shot
  Evaluation of Information Retrieval Models." NeurIPS Datasets and Benchmarks.
  -- Demonstrates that retrieval quality is dominated by embedding model
  choice, not the retrieval algorithm.
- sqlite-vec documentation (https://github.com/asg017/sqlite-vec) -- Confirms
  pre-v1 status and brute-force scan behavior for small datasets.
- Qdrant documentation on HNSW tuning
  (https://qdrant.tech/documentation/concepts/indexing/) -- Documents the
  threshold configuration where HNSW indexing activates, defaulting to 20K
  vectors.
