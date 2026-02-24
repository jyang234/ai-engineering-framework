# RECALL Knowledge System: Technical Analysis

> Analysis date: 2026-02-24
> Scope: RECALL implementation, specification, skills, session integration, research validation

---

## Executive Summary

RECALL is EDI's persistent knowledge retrieval layer — a scope-aware, type-structured knowledge management system that stores and retrieves patterns, decisions, and failures across sessions. It operates as an MCP (Model Context Protocol) server in Go, exposing five tools to Claude Code.

**Key findings**:

1. RECALL is **not a RAG system**. It is a persistent, session-aware knowledge base with mandatory validation (retrieval-judge skill), typed items, dual-scope architecture, and a flight recorder audit trail.

2. RECALL's **compound value** comes from integration with EDI's other systems — task management, plan review, auto memory, session lifecycle, and governance. No component in isolation delivers the same benefit.

3. Two backends share **one MCP interface**. v0 (SQLite FTS5, keyword-only) is fully implemented. Codex (hybrid vector + BM25, multi-stage reranking) is specified but not yet built. Agents are unaware which backend is running.

4. The **retrieval-judge skill** is mandatory for every agent. It prevents blind trust in search results — the validation layer that keeps RECALL from becoming a hallucination amplifier.

5. Research validates the approach. Anthropic's verification hierarchy ranks deterministic tools highest; RECALL's flight recorder and retrieval-judge implement the audit trail that makes knowledge retrieval trustworthy.

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Architecture](#2-architecture)
3. [MCP Tool Interface](#3-mcp-tool-interface)
4. [Knowledge Types and Scopes](#4-knowledge-types-and-scopes)
5. [Storage and Retrieval](#5-storage-and-retrieval)
6. [Retrieval-Judge Validation Layer](#6-retrieval-judge-validation-layer)
7. [Integration with EDI Skills](#7-integration-with-edi-skills)
8. [Session Lifecycle Integration](#8-session-lifecycle-integration)
9. [v0 vs Codex Backend](#9-v0-vs-codex-backend)
10. [How RECALL Differs from Simple RAG](#10-how-recall-differs-from-simple-rag)
11. [Compound Value Analysis](#11-compound-value-analysis)
12. [Research Validation](#12-research-validation)
13. [File Reference](#13-file-reference)

---

## 1. System Overview

RECALL provides Claude Code with organizational memory that persists across sessions. Claude decides when to query — RECALL exposes MCP tools, Claude determines when retrieval would help.

```
User → EDI CLI → Claude Code ↔ RECALL MCP Server (v0 or Codex)
                                    ↓
                           SQLite FTS5 / Qdrant + Embeddings
                                    ↓
                           Knowledge Items (patterns, failures, decisions, context)
```

### Core Principle

**Claude decides when to query.** RECALL is not a pre-fetching layer or automatic context injection system. It exposes tools; the agent's judgment determines when retrieval would help. This is native MCP behavior — no orchestration layer needed.

### Five MCP Tools

| Tool | Purpose |
|---|---|
| `recall_search` | Semantic/lexical search for knowledge items |
| `recall_get` | Retrieve full document by ID |
| `recall_add` | Capture new knowledge |
| `recall_feedback` | Score usefulness of results |
| `flight_recorder_log` | Log session events for later capture |

---

## 2. Architecture

### Codex Core with Multiple Interfaces

RECALL is built on Codex Core, a central Go library providing the retrieval engine. Multiple interfaces consume this core:

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERFACES                              │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   MCP Server    │     Web UI      │      CLI (optional)         │
│   (for EDI)     │   (wiki view)   │     (admin/debug)           │
└────────┬────────┴────────┬────────┴──────────────┬──────────────┘
         │                 │                       │
         ▼                 ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                      CODEX CORE (Go)                            │
│                                                                 │
│  Search(query, scope, filters) → Results                        │
│  Get(id) → Document                                             │
│  Index(path, options) → IndexResult                             │
│  Add(content, metadata) → Document                              │
│  GraphQuery(entity, relationship) → Entities                    │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                        STORAGE LAYER                            │
│  SQLite (metadata, FTS5) + Qdrant (vectors, BM25) + Files      │
└─────────────────────────────────────────────────────────────────┘
```

### Key Implementation Files

| File | Lines | Purpose |
|---|---|---|
| `edi/internal/recall/storage.go` | 399 | SQLite FTS5 operations (Search, Add, FindByTitle, RecordFeedback, LogFlightRecorder) |
| `edi/internal/recall/server.go` | 541 | Hand-rolled JSON-RPC 2.0 MCP server, five tool handlers, stdio transport |
| `edi/internal/recall/schema.sql` | 75 | Database schema (items, items_fts, feedback, flight_recorder tables) |
| `edi/internal/recall/backend.go` | 108 | Backend interface + CodexBackend adapter |
| `edi/pkg/types/recall.go` | 34 | Go type definitions (Item, SearchResult, QueryMetadata) |

### Server Operation

The server reads stdin line-by-line, dispatches JSON-RPC to handlers. It runs via `edi recall-server --session-id {id} --global-db ~/.edi/recall/global.db` (v0) or as the `recall-mcp` binary (Codex).

---

## 3. MCP Tool Interface

```
┌─────────────────────────────────────────────────────────────────┐
│                    RECALL MCP TOOLS                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  recall_search(query, scope, types, limit, rerank)              │
│  ├─ Returns: [items with scores, query_metadata]                │
│  └─ Post-processing: Apply retrieval-judge skill                │
│                                                                 │
│  recall_get(id, include_parent)                                 │
│  └─ Returns: Full document + optional parent context            │
│                                                                 │
│  recall_add(type, title, content, scope, tags)                  │
│  └─ Returns: id, message                                        │
│                                                                 │
│  recall_feedback(item_id, useful, context)                      │
│  └─ Updates: usefulness_score, use_count                        │
│                                                                 │
│  flight_recorder_log(type, content, rationale, metadata)        │
│  └─ Logs: decision/error/milestone/observation/judgment         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Tool Details

**`recall_search`** — The primary entry point. Parameters: `query` (text), `scope` (project/global/all), `types` (pattern/failure/decision/context), `limit`, `rerank` (boolean). Returns scored results with QueryMetadata (scope_searched, total_candidates, reranked, latency_ms).

**`recall_get`** — Retrieve by ID with optional parent context expansion. Useful for code chunks that need surrounding context.

**`recall_add`** — Capture new knowledge items. ID generation uses type-specific prefixes: `P-` (pattern), `F-` (failure), `D-` (decision), `C-` (context), `X-` (other). Supports `--if-not-exists` for duplicate detection via `FindByTitle()`.

**`recall_feedback`** — Track usefulness via boolean feedback + context. Updates `usefulness_score` and `use_count` in the items table. Transaction-based for integrity.

**`flight_recorder_log`** — Persist session events with type, content, rationale, and JSON metadata. Types: decision, error, milestone, agent_switch, observation, retrieval_query, retrieval_judgment.

---

## 4. Knowledge Types and Scopes

### Item Types

| Type | Purpose | Example |
|---|---|---|
| `pattern` | Reusable solutions | "exponential backoff + jitter for retries" |
| `failure` | Known issues + fixes | "nil pointer in token refresh after 401" |
| `decision` | ADRs, technology choices | "chose Stripe for payments due to webhook reliability" |
| `context` | Background information | "PaymentService architecture and data flow" |

Each item carries structured metadata: id, type, title, content, tags (JSON), scope, project_path, timestamps, usefulness_score, use_count.

### Scope Hierarchy

```
GLOBAL (~/.edi/recall/)
├─ Cross-project patterns
├─ Reusable architectural decisions
├─ Personal coding conventions
└─ Technology-specific knowledge

PROJECT (.edi/recall/)
├─ Project-specific code index
├─ Project documentation
├─ Project ADRs
└─ Session history (decisions, learnings)
```

### Promotion Flow

```
Capture (session end) → Project (default scope) → Global (promoted)
                          "This pattern was useful across 3 projects"
```

Knowledge starts in project scope. When an item proves useful across multiple projects, it is promoted to global scope. This prevents premature generalization — project-specific context stays project-specific until validated.

### Scope Selection Logic

When `scope: "all"`, RECALL searches both global and project databases, merges results via Reciprocal Rank Fusion (RRF), and applies unified reranking.

---

## 5. Storage and Retrieval

### Database Schema

```sql
-- Core items table
items (id TEXT PK, type, title, content, tags JSON, scope, project_path,
       created_at, updated_at, usefulness_score, use_count)

-- Full-text search (v0 backend)
items_fts VIRTUAL TABLE (FTS5 on title, content, tags)
  + INSERT/DELETE/UPDATE triggers to keep in sync

-- Feedback tracking
feedback (item_id, session_id, useful BOOLEAN, context, timestamp)

-- Session event log
flight_recorder (session_id, timestamp, type, content, rationale, metadata JSON)
```

Indexes on: type, scope, project_path, feedback.item_id, feedback.session_id.

### v0 Search Mechanism (Current)

1. **Query sanitization**: Split on whitespace/hyphens, quote each term, join with OR
2. **FTS5 MATCH**: Execute against `items_fts` virtual table
3. **Ranking**: FTS5 built-in BM25 ranking
4. **Filtering**: By type and scope parameters

No embeddings, no vector search, no reranking. Simple but functional for small knowledge bases.

### Codex Search Pipeline (Future)

1. **Query router**: Classifies query as CODE/DOCS/HYBRID based on patterns
2. **Hybrid search**: Vector KNN (nomic-embed-text, 768-dim via Ollama) + BM25 sparse vectors + RRF fusion → 100 candidates
3. **Stage 1 reranking**: BGE-base (100→30, ~50ms) via ONNX/hugot
4. **Stage 2 reranking**: BGE-v2-m3 (30→10, ~100ms)
5. **Stage 3 (optional)**: Claude Sonnet (10→5, ~3000ms) for complex queries
6. **Parent expansion**: Code chunks expanded to include surrounding context

### File Layout

```
~/.edi/
├─ recall/
│  ├─ global.db           # v0: SQLite FTS5 for global-scoped items
│  └─ qdrant/             # Codex: Qdrant vector collections (future)
├─ codex.db               # Codex: SQLite metadata (future)

.edi/
├─ recall/
│  ├─ project.db          # v0: SQLite FTS5 for project-scoped items
│  └─ indexed_paths.txt   # Tracked files for incremental updates
```

---

## 6. Retrieval-Judge Validation Layer

**File**: `edi/internal/assets/skills/retrieval-judge/SKILL.md`

The retrieval-judge skill is loaded into **every agent's** system prompt. It is not optional — it is the validation layer that prevents RECALL from becoming a hallucination amplifier.

### Mandatory Workflow

After every `recall_search`, the agent must:

1. **Evaluate each result** against four criteria:
   - Title/type match — Does this relate to the query?
   - Content relevance — Does it address the specific question?
   - Applicability — Does it apply to current tech/codebase/domain?
   - Keep only directly relevant results

2. **Log judgment** to flight recorder:
   ```
   flight_recorder_log({
     type: "retrieval_judgment",
     content: "3/7 results relevant for 'payment retry logic after timeout'",
     metadata: { kept: [...], dropped: [...], per_result_reasoning: {...} }
   })
   ```

3. **Show summary** to user: `RECALL: 3/7 results kept for "..."`

### Anti-Patterns to Avoid

- Don't trust rank order blindly (RRF produces narrow score distributions)
- Don't use results just because they appeared
- Don't ignore low-ranked results (position 8 may beat position 2)
- Don't skip evaluation — mental check required after every search

### When Results Are Poor

Say so. Don't force-fit noise. Try rephrased queries. Proceed without RECALL rather than using bad context. An empty answer is better than a confident wrong one.

### Query Construction

The skill also specifies query construction standards:
- Good: `"payment retry logic after provider timeout"`
- Bad: `"retry"`

Queries should include domain, technology, and specific concern context.

---

## 7. Integration with EDI Skills

RECALL does not operate in isolation. Its value compounds through integration with EDI's skill ecosystem.

### edi-core (Orchestration Hub)

**File**: `edi/internal/assets/skills/edi-core/SKILL.md` (353 lines)

edi-core is the central skill that orchestrates RECALL with task management and governance:

| Integration Point | What Happens |
|---|---|
| Before significant work | Query RECALL: `recall_search({query: "[tech+domain+concern]", types: ["pattern", "failure", "decision"]})` |
| After important decisions | Log to flight recorder: `flight_recorder_log({type: "decision", content: "...", rationale: "..."})` |
| Task creation | Query RECALL for context, store in task annotations |
| Task pickup | Pre-load RECALL context from task annotations |
| Auto memory sync | MEMORY.md EDI-managed sections display promoted RECALL items |

Decision propagation rules: Technology choices, API design, and architecture patterns propagate to dependent tasks. Implementation details and bug fixes do not.

### plan-review (Architecture Validation)

**File**: `edi/internal/assets/skills/plan-review/SKILL.md` (117 lines)

Uses RECALL to validate architectural plans:

1. **Load context**: Search for failures and patterns in affected domains
2. **Cross-reference**: Compare plan changes against past failures and decisions
3. **Assess risk**: Regression risk, complexity, over-engineering
4. **Log findings**: Record risk counts to flight recorder

```
recall_search({query: "[affected domains]", types: ["failure", "pattern"]})
recall_search({query: "[affected domains]", types: ["decision", "context"]})
```

### Other Skills

All skills follow the edi-core pattern: query RECALL before work, apply retrieval-judge, log decisions after. This includes testing, scaffolding-tests, refactoring-planning, and agent-specific skills.

---

## 8. Session Lifecycle Integration

### Session Startup

```
User runs: edi
     ↓
1. Detect project (.edi/)
2. Load config (merge global + project)
3. Generate session ID (UUID)
4. Load agent (coder/architect/reviewer/incident)
5. Start RECALL MCP server
   ├─ v0: edi recall-server --session-id {id} --global-db ~/.edi/recall/global.db
   └─ Codex: recall-mcp with env vars (EDI_SESSION_ID, EDI_AGENT_MODE, etc.)
6. Generate briefing (profile + history + task status)
7. Build context file (agent prompt + briefing + RECALL instructions)
8. Launch Claude Code: syscall.Exec(...--append-system-prompt-file path)
```

### Retrieval Triggers During Session

| Trigger | When | Query Pattern |
|---|---|---|
| Significant work | Before implementation | `recall_search({query: "[tech+domain+concern]", types: ["pattern", "failure", "decision"]})` |
| Failures/errors | When stuck | `recall_search({query: "[error message]", types: ["failure"]})` |
| Plan review | Validating architecture | Per-domain queries for failures, patterns, decisions |
| Task creation | Breaking work into tasks | Context stored in task annotation |
| Task pickup | Starting work on task | Pre-loaded context from annotation |

### Session End (`/end` Command)

1. Generate session summary
2. Identify capture candidates (patterns, decisions, failures discovered during session)
3. Present candidates with structured templates:
   - **Decisions**: Context, Decision, Alternatives, Consequences, Files
   - **Patterns**: Description, When to Use, Implementation, Files
   - **Failures**: Symptom, Root Cause, Fix, Prevention, Files
4. Prompt user to save via `recall_add`
5. Update MEMORY.md with promoted items
6. Update `.edi/status.md`, save to `.edi/history/`

### Ralph Loop Exclusion

RECALL is deliberately **excluded** from Ralph (autonomous execution mode). The design principle: plan interactively (with RECALL) → execute autonomously (Ralph, no RECALL) → capture learnings (save to RECALL). Specs should be pre-baked before autonomous execution.

---

## 9. v0 vs Codex Backend

| Dimension | v0 (SQLite FTS5) | Codex (Hybrid Vector) |
|---|---|---|
| **Binary** | `edi recall-server` | `recall-mcp` |
| **Search** | FTS5 BM25 keyword | Vector KNN + BM25 + RRF |
| **Embeddings** | None | nomic-embed-text via Ollama (768-dim) |
| **Reranking** | None | Multi-stage: BGE-base → BGE-v2-m3 → optional Claude Sonnet |
| **Storage** | `~/.edi/recall/global.db` | `~/.edi/codex.db` + Qdrant |
| **Dependencies** | None (SQLite only) | Ollama running locally |
| **Tool interface** | 5 tools | 5 tools (identical) |
| **Status** | Implemented | Planned (Phases 2-5 of spec) |
| **Accuracy** | Lower (keyword only) | Higher (hybrid + reranking) |

The backend is selected via config:

```yaml
# ~/.edi/config.yaml
recall:
  backend: "v0"    # or "codex"
```

Both backends expose the identical 5-tool MCP interface. `backend.go` defines the `Backend` interface that both implement, with `CodexBackend` adapter translating between EDI v0 Item type and `codex/pkg/recall.Item`.

---

## 10. How RECALL Differs from Simple RAG

| Aspect | Simple RAG | RECALL |
|---|---|---|
| **Retrieval** | Keyword or vector only | Hybrid (keyword + vector + multi-stage reranking) + retrieval-judge validation |
| **Knowledge types** | Unstructured documents | Typed items (pattern, failure, decision, context) with structured metadata |
| **Scope** | Single global index | Dual scopes (global + project) with promotion workflow |
| **Session integration** | Stateless | Session-aware; flight recorder audit trail with session IDs |
| **Capture workflow** | Auto-ingest or manual | Explicitly prompted at session end with structured templates |
| **Feedback loop** | No quality signal | usefulness_score + use_count tracked per item |
| **Validation** | Trusts results implicitly | Mandatory retrieval-judge skill evaluates every result |
| **Persistence** | Ephemeral context window | Durable knowledge base accessible across sessions |
| **Evolution** | Fixed index | Freshness tracking; items can be promoted, deprecated, or scored |
| **Audit trail** | None | Flight recorder: retrieval_query (automatic) + retrieval_judgment (agent) |

---

## 11. Compound Value Analysis

RECALL's value is not self-contained — it compounds through integration with every other EDI subsystem. This section examines what each integration point adds.

### RECALL + Task Management

When RECALL is combined with task management:
- Tasks carry RECALL context as annotations, so knowledge persists across task boundaries
- Decision propagation rules ensure architecture choices flow to dependent tasks
- Task pickup pre-loads relevant RECALL context, reducing cold-start time
- Without this integration: knowledge would need to be re-queried for every task

### RECALL + Plan Review

When RECALL is combined with plan review:
- Past failures are automatically cross-referenced against proposed changes
- Architectural decisions from prior sessions inform current risk assessment
- The plan-review skill queries RECALL for each affected domain, not just globally
- Without this integration: plan review would rely solely on the agent's training data, missing project-specific failure history

### RECALL + Auto Memory (MEMORY.md)

When RECALL is combined with auto memory:
- MEMORY.md serves as the "projection" of RECALL's most important items
- EDI-managed sections display promoted patterns, decisions, pitfalls, and the topic index
- MEMORY.md is always present in context; RECALL is queried on demand
- Without this integration: promoted knowledge would have no lightweight access path

### RECALL + Session Lifecycle

When RECALL is combined with session lifecycle:
- Session IDs tag all flight recorder entries, creating a durable audit trail
- Session end triggers structured capture (patterns, decisions, failures)
- Stale session recovery can reconstruct context from RECALL + git history
- Without this integration: learnings from each session would be lost

### RECALL + Governance

When RECALL is combined with implementation governance:
- Severity calibration is informed by past failure patterns
- "Never skip steps" is backed by evidence of what happened when steps were skipped
- Decision logging creates the accountability trail that governance depends on
- Without this integration: governance rules would be abstract principles without project-specific evidence

### RECALL + Retrieval-Judge

When RECALL is combined with retrieval-judge:
- Every search result is validated before use — no blind trust
- Flight recorder logs both what was retrieved and what was used
- Poor results are discarded rather than force-fit
- Without this integration: RECALL results could introduce hallucination-amplifying noise

### The Compound Effect

No single integration is transformative. The compound effect is:

```
RECALL knowledge
  + Task annotations    → knowledge persists across task boundaries
  + Plan review         → past failures inform future designs
  + Auto memory         → critical knowledge always in context
  + Session lifecycle   → knowledge accumulates across sessions
  + Governance          → decisions have evidence backing
  + Retrieval-judge     → only valid knowledge is used
  = Organizational memory that improves with use
```

This is why extracting RECALL as a standalone tool would lose most of its value. The MCP interface is the mechanism; the integrations are the product.

---

## 12. Research Validation

The coding-standards-review (Part 8) provides research evidence that validates RECALL's design approach.

### Relevant Findings

| Finding | Source | Relevance to RECALL |
|---|---|---|
| AI-generated code has 1.7x more issues | Qodo 2025 | Persistent failure/pattern knowledge reduces repeat defects |
| Verification loops improve pass@1 by 9.2% | ICSME 2025 | Retrieval-judge is a verification loop on knowledge quality |
| Hooks increased skill activation ~20% to ~84% | alexop.dev | RECALL verification Stop hook could ensure protocol adherence |
| 94% of LLM compilation errors are type-related | ETH Zurich / UC Berkeley | RECALL's failure type captures the specific error classes that recur |
| Scaffold choice swings SWE-bench by 25.6 points | Vals.ai | The knowledge harness matters as much as the model |
| METR RCT: AI struggles with "deep implicit context" | METR 2025 | RECALL provides exactly the implicit context that makes AI effective in mature codebases |

### Anthropic's Verification Hierarchy

Anthropic ranks verification approaches:

1. **Deterministic tools** (highest): linters, type checkers, test suites
2. **Test execution**: running existing tests
3. **CLAUDE.md instructions**: system prompt guidance
4. **LLM-as-judge** (weakest): agent-type hooks

RECALL sits at level 3 (prompt-driven) with retrieval-judge providing a level 4 quality gate. The flight recorder creates the audit trail that makes this trustworthy — not because the LLM judge is infallible, but because every judgment is logged and reviewable.

### Coding Standards Review Classification

The coding-standards-review classified RECALL integration instructions as 90% agent prompt, 10% hook:

- **Agent prompt** (17/19): Query decisions, log decisions, apply retrieval-judge, recognize failure context — all require judgment
- **Hook** (2/19): Verify RECALL was queried on mode switch, verify retrieval judgment was logged — both are mechanically verifiable

This classification reinforces that RECALL is primarily a judgment system, not a mechanical one.

---

## 13. File Reference

### Core Implementation

| File | Purpose |
|---|---|
| `edi/internal/recall/storage.go` | SQLite FTS5 storage operations |
| `edi/internal/recall/server.go` | JSON-RPC MCP server |
| `edi/internal/recall/schema.sql` | Database schema |
| `edi/internal/recall/backend.go` | Backend interface + Codex adapter |
| `edi/internal/recall/doc.go` | Package documentation |
| `edi/internal/recall/integration_test.go` | MCP protocol lifecycle tests |
| `edi/pkg/types/recall.go` | Type definitions |

### CLI

| File | Purpose |
|---|---|
| `edi/internal/cli/recall.go` | CLI commands (search, add, status) |
| `edi/internal/cli/recall_server.go` | MCP server launcher |

### Skills

| File | Purpose |
|---|---|
| `edi/internal/assets/skills/retrieval-judge/SKILL.md` | Mandatory validation layer |
| `edi/internal/assets/skills/edi-core/SKILL.md` | Core RECALL integration orchestration |
| `edi/internal/assets/skills/plan-review/SKILL.md` | Architecture validation via RECALL |

### Specification and Architecture

| File | Purpose |
|---|---|
| `docs/architecture/recall-mcp-server-spec.md` | Complete specification (1814 lines) |
| `docs/edi-codex-deep-dive.md` | System architecture and lifecycle |
| `docs/coding-standards-review.md` | Research validation and gap analysis |
