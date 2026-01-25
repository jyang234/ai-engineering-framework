# EDI Project Structure & Implementation Plan

**Status**: Planning  
**Created**: January 25, 2026  
**Version**: 1.0  
**Purpose**: Define project structure, bootstrap strategy, and implementation order

---

## Overview

We are building EDI to build Codex, then upgrading EDI with Codex.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  PHASE 0: EDI v0                                                         │
│  Minimal EDI with SQLite FTS for RECALL                                 │
│  Goal: Useful session continuity while building Codex                   │
│  Duration: 2-3 weeks                                                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  PHASE 1: Codex                                                          │
│  Full retrieval system, built using EDI v0                              │
│  Goal: Production-quality semantic search                               │
│  Duration: 4-6 weeks                                                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  PHASE 2: EDI v1                                                         │
│  Upgrade RECALL to use Codex                                            │
│  Goal: Full semantic search in EDI                                      │
│  Duration: 1-2 weeks                                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Repository Structure

```
edi/
├── cmd/
│   └── edi/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── cli/                        # CLI commands (Cobra)
│   │   ├── root.go
│   │   ├── init.go
│   │   └── launch.go
│   ├── config/                     # Configuration loading (Viper)
│   │   ├── config.go
│   │   ├── schema.go
│   │   └── merge.go
│   ├── briefing/                   # Briefing generation
│   │   ├── generator.go
│   │   ├── sources.go
│   │   └── formatter.go
│   ├── history/                    # History management
│   │   ├── store.go
│   │   ├── parser.go
│   │   └── retention.go
│   ├── recorder/                   # Flight recorder
│   │   ├── writer.go
│   │   ├── reader.go
│   │   └── cleanup.go
│   ├── agent/                      # Agent loading
│   │   ├── loader.go
│   │   ├── parser.go
│   │   └── resolver.go
│   ├── capture/                    # Capture and noise control
│   │   ├── detector.go             # Detect capture candidates
│   │   ├── dedup.go                # Deduplication (exact match v0)
│   │   ├── capacity.go             # Capacity management
│   │   ├── saver.go                # Save to RECALL with checks
│   │   └── staging.go              # Staging queue (v1)
│   └── launch/                     # Claude Code launch
│       ├── context.go              # Build session context file
│       ├── launcher.go             # exec() Claude Code
│       └── mcp.go                  # Ensure MCP configured
├── recall/                         # RECALL MCP server (separate binary)
│   ├── cmd/
│   │   └── recall-server/
│   │       └── main.go
│   ├── internal/
│   │   ├── server/                 # MCP server implementation
│   │   │   ├── server.go
│   │   │   └── handlers.go
│   │   ├── search/                 # Search implementation
│   │   │   ├── fts.go              # v0: SQLite FTS
│   │   │   ├── hybrid.go           # v1: Codex integration
│   │   │   └── interface.go
│   │   ├── storage/                # Storage layer
│   │   │   ├── sqlite.go
│   │   │   └── schema.sql
│   │   ├── recorder/               # Flight recorder tool
│   │   │   └── handler.go
│   │   └── feedback/               # Usefulness tracking
│   │       └── handler.go
│   └── go.mod
├── agents/                         # Built-in agent definitions
│   ├── architect.md
│   ├── coder.md
│   ├── reviewer.md
│   └── incident.md
├── subagents/                      # EDI-aware subagent definitions
│   ├── edi-researcher.md
│   ├── edi-web-researcher.md
│   ├── edi-implementer.md
│   ├── edi-test-writer.md
│   ├── edi-doc-writer.md
│   ├── edi-reviewer.md
│   └── edi-debugger.md
├── skills/                         # Skills for Claude Code
│   └── edi-core.md                 # Core EDI behavior for subagents
├── commands/                       # Slash command templates
│   ├── plan.md
│   ├── build.md
│   ├── review.md
│   ├── incident.md
│   └── end.md
├── docs/
│   ├── specs/                      # All specification documents
│   │   ├── integration-architecture.md
│   │   ├── persona-spec.md
│   │   ├── workspace-config-spec.md
│   │   ├── session-lifecycle-spec.md
│   │   ├── agent-system-spec.md
│   │   ├── subagent-system-spec.md
│   │   ├── cli-commands-spec.md
│   │   ├── advanced-features-spec.md
│   │   ├── recall-mcp-server-spec.md
│   │   └── learning-architecture-addendum.md
│   └── faq.md
├── scripts/
│   ├── install.sh                  # Installation script
│   └── test-recall.sh              # RECALL smoke tests
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Phase 0: EDI v0 (Stub RECALL)

### Goal

Build minimal viable EDI that provides useful session continuity while we build Codex.

### Scope

| Component | Status | Description |
|-----------|--------|-------------|
| **CLI** | Build | `edi`, `edi init`, basic flags |
| **Config** | Build | YAML loading with Viper |
| **Workspace** | Build | `.edi/` structure creation |
| **Agents** | Build | All 4 agent prompts |
| **Commands** | Build | All 5 slash commands |
| **Briefing** | Build | Generate from flat files |
| **History** | Build | Read/write session summaries |
| **Flight Recorder** | Build | JSONL append via MCP |
| **RECALL stub** | Build | SQLite FTS (no embeddings) |
| **Launch** | Build | Context injection + exec |

### Noise Control (v0)

| Feature | v0 Implementation |
|---------|-------------------|
| **Human approval** | ✅ Most captures require user confirmation |
| **Confidence thresholds** | ✅ Only surface candidates above threshold |
| **Friction budget** | ✅ Limit prompts per session |
| **Capacity limits** | ✅ Warn at threshold, block at limit |
| **Deduplication** | ✅ Exact match on normalized summary |
| **Usefulness tracking** | ✅ `recall_feedback` tool stores signal, no auto-action |
| **Staging queue** | ❌ Deferred to v1 |
| **LLM Judge** | ❌ Log corrections only, no attribution |
| **Auto-archival** | ❌ Manual via `edi recall cleanup` |

### Implementation Order

```
Week 1: Foundation
├── Day 1-2: Project setup, go.mod, Makefile
├── Day 3-4: Config loading (Viper)
├── Day 5: Workspace structure + init command
└── Day 6-7: Agent loading + resolver

Week 2: Core Features
├── Day 1-2: RECALL stub (SQLite FTS + MCP server)
├── Day 3-4: Flight recorder MCP tool
├── Day 5: History read/write
└── Day 6-7: Briefing generation

Week 3: Integration
├── Day 1-2: Session context builder
├── Day 3: Claude Code launcher
├── Day 4-5: Slash commands
└── Day 6-7: Testing + polish
```

### RECALL Stub Design

SQLite FTS5 provides decent full-text search without embeddings:

```sql
-- Schema for RECALL stub
CREATE VIRTUAL TABLE knowledge USING fts5(
    id,
    type,           -- decision, pattern, failure, evidence
    title,
    content,
    tags,
    created_at,
    updated_at,
    tokenize='porter unicode61'
);

CREATE TABLE metadata (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT,
    scope TEXT DEFAULT 'project',  -- project, global
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

**MCP tools (same interface as v1):**
- `recall_search` — FTS5 search, ranked by BM25
- `recall_get` — Retrieve by ID
- `recall_add` — Insert + index
- `recall_list` — List by type/filter
- `flight_recorder_log` — Append to events.jsonl

When Codex is ready, we swap `fts.go` for `hybrid.go` without changing the MCP interface.

### Exit Criteria

- [ ] `edi init` creates workspace structure
- [ ] `edi` launches Claude Code with briefing
- [ ] Agents load and resolve correctly
- [ ] `/end` captures knowledge to RECALL stub
- [ ] Flight recorder logs events
- [ ] Briefing includes recent history
- [ ] RECALL stub returns relevant results for simple queries

---

## Phase 1: Codex

### Goal

Build production-quality retrieval system using EDI v0 for continuity.

### Scope

| Component | Description |
|-----------|-------------|
| **Codex Core** | Central Go library for retrieval |
| **Embedding Pipeline** | Voyage Code-3 integration |
| **Chunking** | AST-aware via Tree-sitter |
| **Contextual Retrieval** | Claude Haiku summaries |
| **Hybrid Search** | BM25 + vector via Qdrant |
| **Reranking** | Multi-stage with BGE models |
| **Evaluation** | Benchmark suite |

### Repository

Codex is a separate repository that EDI depends on:

```
codex/
├── cmd/
│   └── codex/
│       └── main.go                 # CLI for testing/admin
├── pkg/
│   └── codex/                      # Public API
│       ├── codex.go                # Main interface
│       ├── search.go
│       ├── index.go
│       └── types.go
├── internal/
│   ├── embedding/
│   │   ├── voyage.go
│   │   └── cache.go
│   ├── chunking/
│   │   ├── ast.go
│   │   ├── treesitter.go
│   │   └── contextual.go
│   ├── search/
│   │   ├── hybrid.go
│   │   ├── bm25.go
│   │   └── vector.go
│   ├── rerank/
│   │   ├── pipeline.go
│   │   ├── bge.go
│   │   └── onnx.go
│   └── storage/
│       ├── qdrant.go
│       └── sqlite.go
├── eval/
│   ├── benchmarks/
│   ├── datasets/
│   └── runner.go
└── go.mod
```

### Implementation Order

```
Week 1-2: Foundation
├── Storage layer (Qdrant + SQLite)
├── Basic embedding pipeline (Voyage)
├── Simple chunking (fixed-size)
└── Vector search (baseline)

Week 3-4: Chunking & Indexing
├── Tree-sitter integration
├── AST-aware chunking
├── Language detection
└── Contextual retrieval (Haiku)

Week 5-6: Search & Reranking
├── BM25 integration
├── Hybrid search (RRF fusion)
├── BGE reranker (ONNX)
├── Multi-stage pipeline
└── Evaluation benchmarks

Week 7: Polish
├── Performance optimization
├── Error handling
├── Documentation
└── Final benchmarks
```

### Key Decisions (From Codex Deep Dive)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Embeddings | Voyage Code-3 | Best for code |
| Vector DB | Qdrant (embedded) | Native hybrid, Go client |
| Chunking | Tree-sitter AST | Language-aware boundaries |
| Reranking | BGE via hugot/ONNX | Self-hosted, no API dependency |
| Accuracy target | 80-88% | Up from 50-60% baseline |

### Exit Criteria

- [ ] Codex indexes a test codebase
- [ ] Hybrid search works (BM25 + vector)
- [ ] Reranking improves results measurably
- [ ] Evaluation benchmark passes accuracy threshold
- [ ] Go package API is clean and documented

---

## Phase 2: EDI v1 (Full RECALL)

### Goal

Upgrade RECALL to use Codex for semantic search.

### Scope

| Change | Description |
|--------|-------------|
| **RECALL search** | Swap FTS for Codex hybrid search |
| **RECALL index** | Use Codex indexing pipeline |
| **Migration** | Move FTS data to Codex |
| **Config** | Add Codex-specific options |

### Implementation

```go
// recall/internal/search/hybrid.go

import "github.com/your-org/codex/pkg/codex"

type HybridSearcher struct {
    codex *codex.Codex
}

func (s *HybridSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
    results, err := s.codex.Search(ctx, codex.SearchRequest{
        Query:   query,
        Limit:   opts.Limit,
        Rerank:  opts.Rerank,
        Filters: opts.Filters,
    })
    if err != nil {
        return nil, err
    }
    
    return convertResults(results), nil
}
```

### Migration Path

1. Add Codex as dependency
2. Implement `HybridSearcher` 
3. Add feature flag for search backend
4. Migrate existing FTS data to Codex index
5. Switch default to Codex
6. Remove FTS code (or keep as fallback)

### Noise Control (v1)

| Feature | v1 Implementation |
|---------|-------------------|
| **Human approval** | ✅ Same as v0 |
| **Confidence thresholds** | ✅ Same as v0 |
| **Friction budget** | ✅ Same as v0 |
| **Capacity limits** | ✅ With automated review workflow |
| **Deduplication** | ✅ Semantic similarity via Codex embeddings |
| **Usefulness tracking** | ✅ Auto-flag items with low usefulness rate |
| **Staging queue** | ✅ Tier 2 items staged before Codex commit |
| **LLM Judge** | ✅ Full attribution with routing to capture/tuning |
| **Auto-archival** | ✅ Triggered by usefulness + staleness signals |

### Exit Criteria

- [ ] RECALL uses Codex for search
- [ ] Existing knowledge is migrated
- [ ] Search quality is measurably better
- [ ] No regression in response time (<500ms)
- [ ] Staging queue operational for Tier 2 captures
- [ ] LLM Judge classifying corrections
- [ ] Archival workflow triggers on low-usefulness items

---

## Development Workflow

### Using EDI to Build EDI/Codex

From Phase 0 completion onward, we use EDI for development:

```bash
# Start work on Codex
cd ~/codex
edi --agent architect

# EDI provides briefing:
# "Good morning. Yesterday we implemented the chunking pipeline.
#  Open question: how to handle very large files.
#  RECALL found: similar chunking decision in the payments service."

# Work session...
# EDI logs decisions to flight recorder
# At end of session:

/end

# EDI captures:
# - Decision: Use streaming for files > 1MB
# - Pattern: Chunking pipeline structure
# - Failure: Memory issue with 10MB files, fixed with streaming
```

This builds the knowledge base as we build the system.

### Branch Strategy

```
main
├── feature/phase-0-foundation
├── feature/phase-0-recall-stub
├── feature/phase-0-briefing
└── ...

# After Phase 0:
main (EDI v0 released)
├── feature/codex-embedding
├── feature/codex-chunking
└── ...

# After Phase 1:
main (Codex released)
├── feature/edi-v1-codex-integration
└── ...
```

---

## Dependencies

### EDI v0

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config loading |
| `github.com/mattn/go-sqlite3` | SQLite + FTS5 |
| `github.com/modelcontextprotocol/go-sdk` | MCP server |
| `gopkg.in/yaml.v3` | YAML parsing |

### Codex

| Dependency | Purpose |
|------------|---------|
| `github.com/qdrant/go-client` | Qdrant vector DB |
| `github.com/smacker/go-tree-sitter` | AST parsing |
| `github.com/knights-analytics/hugot` | ONNX reranking |
| `github.com/mattn/go-sqlite3` | Metadata storage |
| Voyage API | Embeddings (external) |
| Claude API | Contextual retrieval (external) |

---

## Testing Strategy

### EDI v0

| Test Type | Scope |
|-----------|-------|
| Unit | Config parsing, history parsing, agent loading |
| Integration | RECALL stub queries, briefing generation |
| E2E | `edi init` + `edi` launch + `/end` workflow |

### Codex

| Test Type | Scope |
|-----------|-------|
| Unit | Chunking, embedding cache, search logic |
| Integration | Full retrieval pipeline |
| Benchmark | Accuracy on evaluation dataset |

### Evaluation Dataset

Build a test corpus with known-good queries:

```yaml
# eval/datasets/test_queries.yaml
- query: "retry logic with exponential backoff"
  expected_ids: ["adr-031", "payment-service-retry"]
  min_recall: 0.8

- query: "authentication token refresh"
  expected_ids: ["adr-015", "auth-service-token"]
  min_recall: 0.8
```

---

## Success Metrics

### Phase 0 (EDI v0)

| Metric | Target |
|--------|--------|
| Time to `edi init` | < 5 seconds |
| Time to launch | < 2 seconds |
| Briefing generation | < 1 second |
| RECALL stub query | < 100ms |

### Phase 1 (Codex)

| Metric | Target |
|--------|--------|
| Retrieval accuracy | 80-88% (up from 50-60%) |
| Query latency | < 500ms |
| Index latency | < 1s per document |
| Memory usage | < 500MB for 10K documents |

### Phase 2 (EDI v1)

| Metric | Target |
|--------|--------|
| Search quality | Same as Codex benchmarks |
| Migration success | 100% of existing knowledge |
| No regression | Response time ≤ v0 |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Codex takes longer than expected | EDI v0 is usable; stub RECALL provides value |
| Embedding API costs | Cache aggressively; batch requests |
| Reranking latency | Skip for simple queries; async option |
| Claude compliance with flight recorder | Add to agent prompts; validate in testing |
| Scope creep | Strict phase boundaries; defer features |

---

## Next Steps

1. **Initialize repository** with structure above
2. **Start Phase 0** with config loading
3. **Use this document** as the roadmap
4. **Update specs** as implementation reveals issues
5. **Build Codex** once EDI v0 is functional

The specs are done. Time to build.
