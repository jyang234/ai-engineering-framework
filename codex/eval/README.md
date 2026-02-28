# Codex Evaluation Framework

Evaluation infrastructure for the Codex search engine and RECALL MCP tools. Tests whether the tools are built correctly and work as advertised.

## What This Tests

The eval framework answers one question: **do the tools work?**

- Does RECALL index documents and retrieve relevant results?
- Does hybrid search (vector + FTS5 + RRF) produce good retrieval metrics?
- Does the LLM judge improve precision over raw retrieval without discarding relevant results?
- Does the MCP protocol work end-to-end (add items, search, feedback, flight recorder)?

It does **not** test whether Claude writes better code with RECALL (that's a prompting effectiveness question, not a tooling question). An L3 infrastructure for that purpose was built and intentionally removed — see [Implementation History](#implementation-history).

## Quick Start

```bash
# Level 1: Retrieval quality (requires Ollama with nomic-embed-text)
cd codex && make test-eval

# Level 1 + Judge quality (also requires ANTHROPIC_API_KEY)
cd codex && make test-judge
```

### Prerequisites

- **Go 1.22+** with CGO enabled
- **Ollama** with `nomic-embed-text` running locally
- **ANTHROPIC_API_KEY** (for judge tests only)

```bash
ollama pull nomic-embed-text
```

## Components

| File | Purpose |
|---|---|
| `harness.go` | 8-phase evaluation pipeline: MCP protocol, indexing, retrieval quality, feedback, flight recorder, audit trail |
| `judge.go` | LLM-as-judge via Anthropic Messages API (Claude Sonnet, single-turn with retry) |
| `metrics.go` | IR metrics: Recall@K, Precision@K, nDCG, MRR |
| `judge_metrics.go` | Judge quality metrics: precision, recall, F1, filtering rate, false filtering rate |
| `report.go` | Text and JSON report output (`EvalSummary`, `FullEvalReport`) |
| `mcpclient.go` | JSON-RPC client for RECALL MCP server via `io.Pipe` (in-process, no subprocess) |
| `testdata.go` | Type definitions: `TestDocument`, `TestQuery`, `SearchResultFromMCP` |
| `testdata_payflow.go` | PayFlow domain corpus: 30 documents, 20 ground-truth queries across 3 categories |
| `groundtruth.go` | Ground truth annotation types for eval queries |

## Tests

| Test | What It Proves | Build Tags | Dependencies |
|---|---|---|---|
| `TestE2E` | 8-phase pipeline passes — retrieval quality meets thresholds | `fts5`, `evalintegration` | Ollama |
| `TestJudge` | LLM judge improves precision; audit trail logged | `fts5`, `evalintegration` | Ollama, Anthropic API |
| `TestRoundTrip` | Add via MCP, search via MCP, feedback, scope isolation | `fts5`, `evalintegration` | Ollama |
| `TestMetrics` | IR metric math (Recall@K, Precision@K, nDCG, MRR) | `fts5` | None |
| `TestJudgeMetrics` | Judge precision/recall/F1/filtering math (18 sub-tests) | `fts5` | None |

### Running Tests

```bash
# Unit tests only (no external dependencies)
cd codex && go test -tags "fts5" ./eval/...

# Full E2E retrieval eval (Ollama required, ~15 min)
cd codex && go test -tags "fts5 evalintegration" -v -timeout 15m -run TestE2E ./eval/

# MCP round-trip test (Ollama required, ~10 min)
cd codex && go test -tags "fts5 evalintegration" -v -timeout 10m -run TestRoundTrip ./eval/

# Judge eval (Ollama + Anthropic API required, ~45 min)
cd codex && go test -tags "fts5 evalintegration" -v -timeout 45m ./eval/

# Via Makefile
cd codex && make test-eval    # TestE2E -short
cd codex && make test-judge   # Full suite
```

## PayFlow Corpus

The test corpus is a 30-document, 20-query collection from a fictional payment processing domain ("PayFlow"). Documents include architecture decisions, failure post-mortems, coding patterns, and operational procedures.

Queries are categorized into three groups:

| Category | Description | Example Query |
|---|---|---|
| `semantic` | Meaning-based; low keyword overlap with relevant docs | "how to handle cascading failures" |
| `keyword` | Direct keyword matches | "idempotency key implementation" |
| `hybrid-advantage` | Queries where combining both retrieval paths outperforms either alone | "retry thundering herd prevention" |

Each query has ground-truth relevant document IDs for computing IR metrics.

## Benchmark Results

### nomic-embed-text (local, via Ollama)

| Metric | Run 1 | Run 2 |
|---|---|---|
| Recall@5 | 0.829 | 0.830 |
| Recall@10 | 0.908 | 0.910 |
| Precision@5 | 0.360 | 0.360 |
| nDCG@10 | 0.784 | 0.762 |
| MRR | 0.842 | 0.782 |

Per-category nDCG@10 (Run 1): semantic 0.761, keyword 0.742, hybrid-advantage 0.860.

### Voyage Code-3 (API)

| Metric | Run 1 | Run 2 |
|---|---|---|
| Recall@5 | 0.863 | 0.850 |
| Recall@10 | 0.908 | 0.910 |
| Precision@5 | 0.380 | 0.370 |
| nDCG@10 | 0.776 | 0.800 |
| MRR | 0.792 | 0.850 |

Per-category nDCG@10 (Run 1): semantic 0.822, keyword 0.729, hybrid-advantage 0.778.

See `benchmarks_local_nomic.txt` and `benchmarks_voyage.txt` for full details.

## Architecture

```
TestE2E / TestJudge / TestRoundTrip
    │
    ├── EvalHarness (harness.go)
    │   ├── Phase 1: MCP protocol check (initialize, tools/list)
    │   ├── Phase 2: Index PayFlow corpus via recall_add
    │   ├── Phase 3: Retrieval quality (20 queries → IR metrics)
    │   ├── Phase 4: Feedback recording (recall_feedback)
    │   ├── Phase 5: Flight recorder verification
    │   ├── Phase 6: Audit trail check
    │   ├── Phase 7: Judge evaluation (LLM-as-judge via Anthropic)
    │   └── Phase 8: Report generation (text + JSON)
    │
    ├── MCPClient (mcpclient.go)
    │   └── JSON-RPC over io.Pipe → RECALL MCP server (in-process)
    │
    ├── AnthropicClient (judge.go)
    │   └── Messages API → Claude Sonnet (relevance judgments)
    │
    └── Metrics (metrics.go, judge_metrics.go)
        ├── IR: Recall@K, Precision@K, nDCG, MRR
        └── Judge: precision, recall, F1, filtering rate, false filtering rate
```

The MCP client communicates with the RECALL server via `io.Pipe` — no subprocess, no network. The search engine uses real SQLite with FTS5 and real Ollama embeddings. Nothing is mocked in the E2E tests.

## Known Gaps

The following areas lack test coverage (see `docs/architecture/eval-test-spec.md` for detailed specs):

| Gap | Location | Impact |
|---|---|---|
| MCP server protocol handlers | `codex/internal/mcp/` | 5 tool handlers untested at the protocol level |
| Embedding client | `codex/internal/embedding/` | Ollama connectivity, retry behavior, dimension handling |
| Chunking | `codex/internal/chunking/` | AST and markdown chunking behavior |
| Reranking | `codex/internal/reranking/` | Cross-encoder functionality |

## Implementation History

The eval framework went through three phases:

1. **Level 1 built** (2026-01): EvalHarness, JudgeHarness, IR metrics, PayFlow corpus, MCP client. All runnable and passing.

2. **Level 3 built** (2026-02): 111 files, ~16,740 lines. Synthetic agent runner, pipe-mode runner, scoring pipeline, stats engine (Mann-Whitney U, Bootstrap CI, Wilcoxon, Spearman), results DB, 15-task corpus, CLI.

3. **Level 3 removed** (2026-02-28, commit `32e6b58`): The L3 infrastructure tested "does Claude write better code with pitfall knowledge in its prompt?" — a prompting question answerable by a 200-line bash script. The stated goal is "do the tools work?" — answered by Level 1. The infrastructure was removed; the Level 1 evaluation remains.

## Related Documentation

- [Eval Framework Spec](../../docs/architecture/aef-evaluation-framework.md) — full evaluation design, experiment specifications, and implementation history
- [Eval System Audit](../../docs/architecture/eval-audit-2026-02-27.md) — coverage audit identifying tested vs untested components
- [Test Gap Spec](../../docs/architecture/eval-test-spec.md) — detailed specs for filling 5 zero-coverage gaps
