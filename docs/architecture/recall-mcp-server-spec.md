# RECALL MCP Server Specification

**Status**: Draft  
**Created**: January 24, 2026  
**Version**: 0.3  
**Implementation Language**: Go

---

## Table of Contents

1. [Overview](#1-overview)
2. [Design Decisions](#2-design-decisions)
3. [MCP Tool Definitions](#3-mcp-tool-definitions)
4. [Knowledge Scopes](#4-knowledge-scopes)
5. [Storage Architecture](#5-storage-architecture)
6. [Retrieval Pipeline](#6-retrieval-pipeline)
7. [Index Management](#7-index-management)
8. [Configuration](#8-configuration)
9. [Error Handling](#9-error-handling)
10. [Dependencies](#10-dependencies)
11. [Implementation Plan](#11-implementation-plan)

---

## 1. Overview

### What is RECALL?

RECALL is EDI's knowledge retrieval layer, implemented as an MCP server in Go. It provides Claude Code with tools to search and retrieve organizational knowledge — code, documentation, architecture decisions, and session history.

### Core Principle

**Claude decides when to query.** RECALL exposes MCP tools; Claude Code determines when retrieval would help the current task. This is native MCP behavior — no orchestration layer needed.

### Architecture: Codex Core with Multiple Interfaces

RECALL is built on **Codex Core**, a central Go library that provides the retrieval engine. Multiple interfaces consume this core:

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERFACES                               │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   MCP Server    │     Web UI      │      CLI (optional)         │
│   (for EDI)     │   (wiki view)   │     (admin/debug)           │
└────────┬────────┴────────┬────────┴──────────────┬──────────────┘
         │                 │                       │
         ▼                 ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                      CODEX CORE (Go)                             │
│                                                                  │
│  • Search(query, scope, filters) → Results                      │
│  • Get(id) → Document                                           │
│  • Index(path, options) → IndexResult                           │
│  • Add(content, metadata) → Document                            │
│  • GraphQuery(entity, relationship) → Entities                  │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                        STORAGE LAYER                             │
│  SQLite (metadata, graph) + Qdrant (vectors, BM25) + Files      │
└─────────────────────────────────────────────────────────────────┘
```

### Inherited Design

RECALL inherits the retrieval architecture from Codex (see `codex-architecture-deep-dive.md`):
- Hybrid search (vector + BM25)
- Multi-stage reranking (ONNX models via hugot)
- AST-aware code chunking (go-tree-sitter)
- Contextual retrieval for documentation

### What RECALL Adds

- **MCP interface** — Tools Claude Code can call directly (official Go SDK)
- **Web UI** — Browse knowledge base like a wiki
- **Multi-scope knowledge** — Global patterns + project-specific + optional domain tier
- **Session integration** — Access to EDI history and decisions
- **Single binary deployment** — Go enables simple distribution

---

## 2. Design Decisions

### Established (from EDI Planning)

| Decision | Rationale |
|----------|-----------|
| MCP server (not embedded) | Native Claude Code integration; reusable without EDI |
| Prompted capture (not auto-ingest) | Human curation keeps knowledge clean |
| Hybrid search from Codex | Proven +15-30% recall improvement |

### New Decisions for This Spec

| Decision | Options Considered | Choice | Rationale |
|----------|-------------------|--------|-----------|
| **Implementation language** | Python, Go, Hybrid | Go | Official MCP SDK available; single binary deployment; hugot provides ONNX reranking |
| **Scope model** | Single index, Multi-index | Multi-index | Clean separation; different update cadences |
| **Storage backend** | SQLite + Qdrant, SQLite-only | SQLite + Qdrant | Qdrant's native hybrid search; proven in Codex |
| **Reranking** | Python sidecar, ONNX in Go | ONNX via hugot | Single process; hugot is production-tested |
| **Federation** | Query all scopes, Query specified | Query specified | Predictable behavior; explicit scope selection |
| **Index location** | Centralized, Distributed | Distributed | Global in `~/.edi/recall/`, project in `.edi/recall/` |
| **Entity graph** | Full GraphRAG, Minimal, Defer | Minimal in v1 | Include imports + dependencies; defer complex graph queries |

### Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` | Official SDK, maintained with Google |
| Reranker | `hugot` + ONNX Runtime | Production-tested; supports cross-encoder models |
| AST Parsing | `go-tree-sitter` | Mature Go bindings; multi-language support |
| Vector DB | Qdrant (embedded or server) | Native hybrid search; Go client available |
| Metadata DB | SQLite via `go-sqlite3` | Proven; portable |
| Web Framework | Gin or Echo | High performance; familiar patterns |
| CLI Framework | Cobra | Standard for Go CLIs |

### Open Questions

| Question | Context | Default Position |
|----------|---------|-----------------|
| Domain tier in v1? | Middle scope between project and global | Optional; add if needed |
| Web UI auth? | Multi-user scenarios | Skip for v1 (local-first) |
| Realtime index updates? | Complexity vs. value | Batch updates; re-index on demand |

---

## 3. MCP Tool Definitions

### 3.1 Tool Overview

| Tool | Purpose | Scope |
|------|---------|-------|
| `recall_search` | Semantic + lexical search | Any |
| `recall_get` | Retrieve full document by ID | Any |
| `recall_list` | List documents by type/filter | Any |
| `recall_context` | Get context for a file path | Project |
| `recall_add` | Add content to knowledge base | Project |
| `recall_index` | Index a file or directory | Project |
| `recall_feedback` | Provide feedback on retrieved item usefulness | Any |
| `flight_recorder_log` | Log event to local flight recorder | Local (not RECALL) |

### 3.2 Tool: `recall_search`

Primary retrieval tool. Combines vector similarity and BM25 lexical matching.

```json
{
  "name": "recall_search",
  "description": "Search organizational knowledge including code, documentation, architecture decisions, and session history. Returns ranked results with relevance scores.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Natural language search query"
      },
      "scope": {
        "type": "string",
        "enum": ["project", "global", "all"],
        "default": "all",
        "description": "Knowledge scope to search. 'project' = current project only, 'global' = cross-project patterns, 'all' = both"
      },
      "types": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["code", "documentation", "adr", "playbook", "session", "decision"]
        },
        "description": "Filter by content types. Omit to search all types."
      },
      "limit": {
        "type": "integer",
        "default": 10,
        "minimum": 1,
        "maximum": 50,
        "description": "Maximum number of results to return"
      },
      "rerank": {
        "type": "boolean",
        "default": true,
        "description": "Apply multi-stage reranking for higher accuracy (adds ~150ms latency)"
      }
    },
    "required": ["query"]
  },
  "outputSchema": {
    "type": "object",
    "properties": {
      "results": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "id": { "type": "string" },
            "type": { "type": "string" },
            "title": { "type": "string" },
            "content": { "type": "string" },
            "score": { "type": "number" },
            "metadata": {
              "type": "object",
              "properties": {
                "source": { "type": "string" },
                "file_path": { "type": "string" },
                "created_at": { "type": "string" },
                "updated_at": { "type": "string" }
              }
            }
          }
        }
      },
      "query_metadata": {
        "type": "object",
        "properties": {
          "scope_searched": { "type": "string" },
          "total_candidates": { "type": "integer" },
          "reranked": { "type": "boolean" },
          "latency_ms": { "type": "integer" }
        }
      }
    }
  }
}
```

**Example invocation:**

```json
{
  "query": "authentication token refresh pattern",
  "scope": "project",
  "types": ["code", "adr"],
  "limit": 5
}
```

**Example response:**

```json
{
  "results": [
    {
      "id": "adr-023",
      "type": "adr",
      "title": "ADR-023: JWT Token Refresh Strategy",
      "content": "## Context\n\nWe need to handle token refresh for long-running sessions...",
      "score": 0.92,
      "metadata": {
        "source": "docs/adr/023-jwt-refresh.md",
        "file_path": "/project/docs/adr/023-jwt-refresh.md",
        "created_at": "2025-11-15T10:30:00Z",
        "updated_at": "2025-12-01T14:20:00Z"
      }
    },
    {
      "id": "code-auth-service-42",
      "type": "code",
      "title": "AuthService.refreshToken",
      "content": "async refreshToken(token: string): Promise<TokenPair> {\n  // Validate current token...",
      "score": 0.87,
      "metadata": {
        "source": "src/services/auth.ts",
        "file_path": "/project/src/services/auth.ts",
        "created_at": "2025-10-20T08:15:00Z",
        "updated_at": "2026-01-10T16:45:00Z"
      }
    }
  ],
  "query_metadata": {
    "scope_searched": "project",
    "total_candidates": 156,
    "reranked": true,
    "latency_ms": 245
  }
}
```

### 3.3 Tool: `recall_get`

Retrieve full document content by ID.

```json
{
  "name": "recall_get",
  "description": "Retrieve the full content of a specific document by its ID. Use after recall_search to get complete context.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {
        "type": "string",
        "description": "Document ID from search results"
      },
      "include_parent": {
        "type": "boolean",
        "default": false,
        "description": "For code chunks, include the parent file content"
      }
    },
    "required": ["id"]
  },
  "outputSchema": {
    "type": "object",
    "properties": {
      "id": { "type": "string" },
      "type": { "type": "string" },
      "title": { "type": "string" },
      "content": { "type": "string" },
      "parent_content": { "type": "string" },
      "metadata": { "type": "object" }
    }
  }
}
```

### 3.4 Tool: `recall_list`

List documents by type or filter criteria.

```json
{
  "name": "recall_list",
  "description": "List documents in the knowledge base by type or filter. Useful for browsing ADRs, recent sessions, or available playbooks.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "type": {
        "type": "string",
        "enum": ["code", "documentation", "adr", "playbook", "session", "decision"],
        "description": "Document type to list"
      },
      "scope": {
        "type": "string",
        "enum": ["project", "global"],
        "default": "project"
      },
      "limit": {
        "type": "integer",
        "default": 20,
        "maximum": 100
      },
      "sort": {
        "type": "string",
        "enum": ["updated_at", "created_at", "title"],
        "default": "updated_at"
      }
    },
    "required": ["type"]
  }
}
```

### 3.5 Tool: `recall_context`

Get relevant knowledge for a specific file being worked on.

```json
{
  "name": "recall_context",
  "description": "Get relevant context for a specific file path. Returns related code, documentation, and decisions that might be helpful when working on this file.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "file_path": {
        "type": "string",
        "description": "Path to the file being worked on"
      },
      "context_types": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["related_code", "documentation", "adrs", "recent_changes", "ownership"]
        },
        "default": ["related_code", "documentation", "adrs"]
      }
    },
    "required": ["file_path"]
  }
}
```

### 3.6 Tool: `recall_add`

Add content to the knowledge base (used by capture workflow).

```json
{
  "name": "recall_add",
  "description": "Add new content to the knowledge base. Typically used through EDI's capture workflow for decisions, learnings, or significant discoveries.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "type": {
        "type": "string",
        "enum": ["decision", "pattern", "lesson", "note"],
        "description": "Type of knowledge being captured"
      },
      "title": {
        "type": "string",
        "description": "Brief title for the knowledge item"
      },
      "content": {
        "type": "string",
        "description": "Full content of the knowledge item"
      },
      "scope": {
        "type": "string",
        "enum": ["project", "global"],
        "default": "project",
        "description": "Where to store: project-specific or global (cross-project)"
      },
      "tags": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Optional tags for categorization"
      },
      "related_files": {
        "type": "array",
        "items": { "type": "string" },
        "description": "File paths this knowledge relates to"
      }
    },
    "required": ["type", "title", "content"]
  }
}
```

### 3.7 Tool: `recall_index`

Index a file or directory into the knowledge base.

```json
{
  "name": "recall_index",
  "description": "Index a file or directory into the knowledge base. Use sparingly — designed for explicit indexing of important content, not bulk ingestion.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "Path to file or directory to index"
      },
      "recursive": {
        "type": "boolean",
        "default": false,
        "description": "For directories, index recursively"
      },
      "type_hint": {
        "type": "string",
        "enum": ["code", "documentation", "adr", "playbook"],
        "description": "Hint for content type (auto-detected if omitted)"
      }
    },
    "required": ["path"]
  }
}
```

### 3.8 Tool: `recall_feedback`

Provide feedback on the usefulness of a retrieved knowledge item. This builds signal for retrieval quality improvement.

```json
{
  "name": "recall_feedback",
  "description": "Provide feedback on whether a retrieved knowledge item was useful for the current task. Use after recall_search when you can determine if results helped. This signal improves future retrieval quality.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "item_id": {
        "type": "string",
        "description": "ID of the knowledge item (from recall_search results)"
      },
      "feedback": {
        "type": "string",
        "enum": ["useful", "not_useful", "outdated", "duplicate"],
        "description": "Feedback on the item: useful (helped with task), not_useful (retrieved but not helpful), outdated (information is stale), duplicate (same as another item)"
      },
      "context": {
        "type": "string",
        "description": "Optional context about why the item was/wasn't useful"
      },
      "duplicate_of": {
        "type": "string",
        "description": "If feedback is 'duplicate', the ID of the item this duplicates"
      }
    },
    "required": ["item_id", "feedback"]
  },
  "outputSchema": {
    "type": "object",
    "properties": {
      "acknowledged": { "type": "boolean" },
      "item_stats": {
        "type": "object",
        "properties": {
          "retrieval_count": { "type": "integer" },
          "useful_count": { "type": "integer" },
          "not_useful_count": { "type": "integer" },
          "usefulness_rate": { "type": "number" }
        }
      }
    }
  }
}
```

**Example invocations:**

```json
// Item was helpful
{
  "item_id": "adr-023",
  "feedback": "useful",
  "context": "JWT refresh pattern directly applicable to current implementation"
}

// Item retrieved but not relevant
{
  "item_id": "code-auth-42",
  "feedback": "not_useful",
  "context": "About OAuth, but task is API key authentication"
}

// Item has stale information
{
  "item_id": "pattern-015",
  "feedback": "outdated",
  "context": "References deprecated PaymentService.charge() API"
}

// Item duplicates another
{
  "item_id": "decision-089",
  "feedback": "duplicate",
  "duplicate_of": "adr-023"
}
```

**Storage:** Feedback is stored in the knowledge item's metadata:

```yaml
knowledge_item:
  id: "adr-023"
  # ... other fields ...
  
  # Quality signals (updated by recall_feedback)
  retrieval_count: 15
  useful_count: 12
  not_useful_count: 2
  outdated_count: 1
  usefulness_rate: 0.80  # useful_count / retrieval_count
  
  # Flags for review
  flagged_outdated: true  # At least one "outdated" feedback
  flagged_duplicate_of: null  # If marked duplicate
```

**v0 vs v1 behavior:**

| Version | Behavior |
|---------|----------|
| v0 | Store feedback, update counts, no automatic action |
| v1 | Flag items for review when usefulness_rate < 0.2 after 10+ retrievals |

### 3.9 Tool: `flight_recorder_log`

Log a significant event to the local flight recorder. Used by Claude to capture decisions, errors, milestones, and other notable moments during a session. Data stays local (not indexed in RECALL) and is used for briefing generation.

```json
{
  "name": "flight_recorder_log",
  "description": "Log a significant event to the local flight recorder for session continuity and briefing generation. Use for: decisions (with rationale), errors (with resolution), milestones, and agent switches. Events are stored locally and NOT uploaded to RECALL.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "type": {
        "type": "string",
        "enum": ["decision", "error", "milestone", "agent_switch", "observation"],
        "description": "Type of event being logged"
      },
      "content": {
        "type": "string",
        "description": "Brief description of the event"
      },
      "rationale": {
        "type": "string",
        "description": "For decisions: why this choice was made"
      },
      "resolution": {
        "type": "string",
        "description": "For errors: how the error was resolved"
      },
      "from_agent": {
        "type": "string",
        "description": "For agent_switch: previous agent mode"
      },
      "to_agent": {
        "type": "string",
        "description": "For agent_switch: new agent mode"
      },
      "related_files": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Files related to this event"
      }
    },
    "required": ["type", "content"]
  },
  "outputSchema": {
    "type": "object",
    "properties": {
      "logged": { "type": "boolean" },
      "event_id": { "type": "string" },
      "timestamp": { "type": "string" }
    }
  }
}
```

**Example invocations:**

```json
// Decision
{
  "type": "decision",
  "content": "Chose exponential backoff for retry logic",
  "rationale": "Prevents thundering herd on service recovery; industry standard pattern",
  "related_files": ["webhook.go"]
}

// Error resolved
{
  "type": "error",
  "content": "Race condition in token refresh caused intermittent auth failures",
  "resolution": "Added mutex around refresh logic in auth/token.go",
  "related_files": ["auth/token.go", "auth/token_test.go"]
}

// Milestone
{
  "type": "milestone",
  "content": "Retry logic implementation complete, all tests passing"
}

// Agent switch
{
  "type": "agent_switch",
  "content": "Switching to implementation mode",
  "from_agent": "architect",
  "to_agent": "coder"
}
```

**Storage location:** `.edi/sessions/{session-id}/events.jsonl`

**Note:** This tool writes to the local flight recorder only. It does NOT index content in RECALL. Knowledge worth preserving should be captured via `/end` workflow and `recall_add`.

---

## 4. Knowledge Scopes

### 4.1 Scope Hierarchy

```
┌─────────────────────────────────────────────────────────────┐
│                         GLOBAL                               │
│  ~/.edi/recall/                                             │
│  • Cross-project patterns                                    │
│  • Reusable architectural decisions                          │
│  • Personal coding conventions                               │
│  • Technology-specific knowledge                             │
└─────────────────────────────────────────────────────────────┘
                              │
                    Queried when scope = "all"
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        PROJECT                               │
│  ~/project/.edi/recall/                                     │
│  • Project-specific code index                               │
│  • Project documentation                                     │
│  • Project ADRs                                              │
│  • Session history (decisions, learnings)                    │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 Scope Characteristics

| Aspect | Global | Project |
|--------|--------|---------|
| Location | `~/.edi/recall/` | `.edi/recall/` |
| Updates | Infrequent (promotion from projects) | Frequent (active development) |
| Content | Patterns, conventions, reusable decisions | Code, docs, project-specific decisions |
| Index size | Small (curated) | Medium-large (comprehensive) |
| Ownership | User (personal) | Project (shared if team uses EDI) |

### 4.3 Scope Selection Logic

When a search specifies `scope: "all"`:

1. Query both global and project indexes
2. Merge results using Reciprocal Rank Fusion (RRF)
3. Apply unified reranking
4. Return combined results with scope annotation

```go
// SearchAllScopes queries both global and project indexes and merges results
func (c *CodexCore) SearchAllScopes(ctx context.Context, query string, limit int) ([]Result, error) {
    // Query both scopes concurrently
    var wg sync.WaitGroup
    var globalResults, projectResults []Result
    var globalErr, projectErr error

    wg.Add(2)
    go func() {
        defer wg.Done()
        globalResults, globalErr = c.searchIndex(ctx, query, ScopeGlobal, limit)
    }()
    go func() {
        defer wg.Done()
        projectResults, projectErr = c.searchIndex(ctx, query, ScopeProject, limit)
    }()
    wg.Wait()

    if globalErr != nil && projectErr != nil {
        return nil, fmt.Errorf("both searches failed: global=%v, project=%v", globalErr, projectErr)
    }

    // RRF merge
    merged := reciprocalRankFusion([][]Result{globalResults, projectResults})

    // Unified reranking
    reranked, err := c.reranker.Rerank(ctx, query, merged[:min(len(merged), limit*2)])
    if err != nil {
        return nil, fmt.Errorf("reranking failed: %w", err)
    }

    return reranked[:min(len(reranked), limit)], nil
}
```

### 4.4 Promotion Flow

Knowledge can be promoted from project → global scope:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Capture    │ ──▶ │   Project    │ ──▶ │   Global     │
│  (session)   │     │   (default)  │     │  (promoted)  │
└──────────────┘     └──────────────┘     └──────────────┘
                            │
                     "This pattern was
                      useful across
                      3 projects"
```

Promotion is manual (user decides) via `/promote` command or EDI suggestion.

---

## 5. Storage Architecture

### 5.1 Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      RECALL Storage Layer                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────┐         ┌──────────────────────────────┐  │
│  │    SQLite        │         │         Qdrant               │  │
│  │   (Metadata)     │◀───────▶│     (Vectors + BM25)         │  │
│  │                  │         │                              │  │
│  │ • Document IDs   │         │ • Code collection            │  │
│  │ • Chunk mapping  │         │   (Voyage Code-3 embeddings) │  │
│  │ • File paths     │         │                              │  │
│  │ • Timestamps     │         │ • Docs collection            │  │
│  │ • Relationships  │         │   (text-embedding-3-large)   │  │
│  │ • Freshness info │         │                              │  │
│  └──────────────────┘         │ • BM25 sparse vectors        │  │
│                               │   (Qdrant native)            │  │
│                               └──────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 SQLite Schema

```sql
-- Documents table (source files, ADRs, etc.)
CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- 'code', 'documentation', 'adr', 'session', etc.
    title TEXT NOT NULL,
    source_path TEXT,    -- Original file path
    scope TEXT NOT NULL, -- 'global' or 'project'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    content_hash TEXT,   -- For change detection
    metadata JSON        -- Type-specific metadata
);

-- Chunks table (indexed units)
CREATE TABLE chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT REFERENCES documents(id),
    chunk_index INTEGER,  -- Position in document
    content TEXT NOT NULL,
    contextualized_content TEXT,  -- With prepended context
    chunk_type TEXT,      -- 'function', 'class', 'section', etc.
    start_line INTEGER,
    end_line INTEGER,
    qdrant_id TEXT,       -- Reference to vector in Qdrant
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Session history (EDI sessions)
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    started_at TIMESTAMP,
    ended_at TIMESTAMP,
    summary TEXT,
    decisions JSON,       -- Array of decision objects
    agent TEXT,           -- Which agent was active
    task_ids JSON         -- Related Claude Code task IDs
);

-- Captured knowledge (from sessions)
CREATE TABLE knowledge (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,   -- 'decision', 'pattern', 'lesson'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    scope TEXT NOT NULL,
    session_id TEXT REFERENCES sessions(id),
    tags JSON,
    related_files JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    qdrant_id TEXT
);

-- Freshness tracking
CREATE TABLE freshness (
    document_id TEXT REFERENCES documents(id),
    last_verified TIMESTAMP,
    verification_status TEXT,  -- 'current', 'stale', 'deprecated'
    staleness_score REAL DEFAULT 0
);

-- Indexes
CREATE INDEX idx_documents_type ON documents(type);
CREATE INDEX idx_documents_scope ON documents(scope);
CREATE INDEX idx_chunks_document ON chunks(document_id);
CREATE INDEX idx_knowledge_scope ON knowledge(scope);
CREATE INDEX idx_knowledge_type ON knowledge(type);
```

### 5.3 Qdrant Collections

**Code Collection** (per scope):

```go
// CreateCodeCollection creates the vector collection for code chunks
func (s *Storage) CreateCodeCollection(ctx context.Context, scope Scope) error {
    collectionName := fmt.Sprintf("recall_code_%s", scope)
    
    return s.qdrant.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: collectionName,
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     1024, // Voyage Code-3 dimension
            Distance: qdrant.Distance_Cosine,
        }),
        SparseVectorsConfig: map[string]*qdrant.SparseVectorParams{
            "bm25": {
                Modifier: qdrant.Modifier_Idf, // For BM25 scoring
            },
        },
    })
}
```

**Documentation Collection** (per scope):

```go
// CreateDocsCollection creates the vector collection for documentation chunks
func (s *Storage) CreateDocsCollection(ctx context.Context, scope Scope) error {
    collectionName := fmt.Sprintf("recall_docs_%s", scope)
    
    return s.qdrant.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: collectionName,
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     3072, // text-embedding-3-large dimension
            Distance: qdrant.Distance_Cosine,
        }),
        SparseVectorsConfig: map[string]*qdrant.SparseVectorParams{
            "bm25": {
                Modifier: qdrant.Modifier_Idf,
            },
        },
    })
}
```

### 5.4 File Layout

```
~/.edi/
├── recall/
│   ├── global.db           # SQLite for global scope
│   ├── qdrant/             # Qdrant data directory
│   │   ├── collections/
│   │   │   ├── recall_code_global/
│   │   │   └── recall_docs_global/
│   │   └── storage/
│   └── config.yaml         # RECALL configuration

~/project/.edi/
├── recall/
│   ├── project.db          # SQLite for project scope
│   ├── qdrant/             # Project-specific Qdrant (or shared)
│   │   ├── collections/
│   │   │   ├── recall_code_project/
│   │   │   └── recall_docs_project/
│   │   └── storage/
│   └── indexed_paths.txt   # Tracked paths for incremental updates
├── history/                # Session summaries (separate from RECALL)
└── config.yaml
```

---

## 6. Retrieval Pipeline

### 6.1 Query Flow

```
┌────────────────────────────────────────────────────────────────┐
│                        RECALL Query Flow                        │
└────────────────────────────────────────────────────────────────┘

   User Query
       │
       ▼
┌──────────────┐
│ Query Router │  Classify: CODE | DOCS | RELATIONSHIP | HYBRID
└──────┬───────┘
       │
       ▼
┌──────────────┐     ┌──────────────┐
│ Embed Query  │     │ Generate BM25│
│ (Voyage/OAI) │     │   Keywords   │
└──────┬───────┘     └──────┬───────┘
       │                    │
       └────────┬───────────┘
                │
                ▼
┌───────────────────────────┐
│     Hybrid Search         │
│  (Vector + BM25 + RRF)    │
│     100 candidates        │
└───────────┬───────────────┘
            │
            ▼
┌───────────────────────────┐
│  Stage 1: BGE-base        │
│     100 → 30              │
│     ~50ms                 │
└───────────┬───────────────┘
            │
            ▼
┌───────────────────────────┐
│  Stage 2: BGE-v2-m3       │
│     30 → 10               │
│     ~100ms                │
└───────────┬───────────────┘
            │
            ▼ (if complex query)
┌───────────────────────────┐
│  Stage 3: Claude (opt)    │
│     10 → 5                │
│     ~3000ms               │
└───────────┬───────────────┘
            │
            ▼
┌───────────────────────────┐
│  Parent Expansion         │
│  (retrieve full context)  │
└───────────┬───────────────┘
            │
            ▼
       Results
```

### 6.2 Query Router (Simplified for EDI)

```go
package retrieval

import (
    "regexp"
    "strings"
)

// QueryType represents the type of query for routing
type QueryType int

const (
    QueryTypeCode QueryType = iota
    QueryTypeDocs
    QueryTypeHybrid
)

// QueryRouter routes queries to appropriate retrieval paths
type QueryRouter struct {
    codeSignals     []*regexp.Regexp
    docSignals      []*regexp.Regexp
    decisionSignals []*regexp.Regexp
}

// NewQueryRouter creates a router with default patterns
func NewQueryRouter() *QueryRouter {
    return &QueryRouter{
        codeSignals: compilePatterns([]string{
            `\b(function|class|method|def|async|import)\b`,
            `\.(py|ts|js|go|rs|java)$`,
            `how to implement`,
            `code example`,
        }),
        docSignals: compilePatterns([]string{
            `documentation`,
            `readme`,
            `guide`,
            `playbook`,
            `runbook`,
        }),
        decisionSignals: compilePatterns([]string{
            `\bADR\b`,
            `decision`,
            `why did we`,
            `rationale`,
        }),
    }
}

// Route determines the query type for optimal retrieval
func (r *QueryRouter) Route(query string) QueryType {
    queryLower := strings.ToLower(query)

    if r.matches(queryLower, r.decisionSignals) {
        return QueryTypeDocs // ADRs stored in docs collection
    }

    if r.matches(queryLower, r.codeSignals) {
        return QueryTypeCode
    }

    if r.matches(queryLower, r.docSignals) {
        return QueryTypeDocs
    }

    return QueryTypeHybrid // Search both
}

func (r *QueryRouter) matches(query string, patterns []*regexp.Regexp) bool {
    for _, p := range patterns {
        if p.MatchString(query) {
            return true
        }
    }
    return false
}

func compilePatterns(patterns []string) []*regexp.Regexp {
    compiled := make([]*regexp.Regexp, len(patterns))
    for i, p := range patterns {
        compiled[i] = regexp.MustCompile(p)
    }
    return compiled
}
```

### 6.3 Reranking Configuration

| Query Type | Stage 1 | Stage 2 | Stage 3 |
|------------|---------|---------|---------|
| Simple code lookup | ✅ | ✅ | ❌ |
| Documentation search | ✅ | ✅ | ❌ |
| Architecture question | ✅ | ✅ | ✅ (auto) |
| Cross-file relationship | ✅ | ✅ | ✅ (auto) |
| User requests deep analysis | ✅ | ✅ | ✅ (explicit) |

Stage 3 auto-triggers when:
- Query contains "architecture", "relationship", "depends on"
- Stage 2 scores are ambiguous (top 3 within 0.1 of each other)

---

## 7. Index Management

### 7.1 Indexing Triggers

| Trigger | Scope | Method |
|---------|-------|--------|
| `recall_index` tool call | Project | Explicit |
| `/index` EDI command | Project | Explicit |
| Session end capture | Project | Prompted |
| Promotion from project | Global | Explicit |
| First EDI run in project | Project | Initial scan (prompted) |

### 7.2 Incremental Updates

```go
package indexing

import (
    "context"
    "os"
    "path/filepath"
)

// IndexManager manages incremental index updates
type IndexManager struct {
    db      *Storage
    chunker *Chunker
}

// IndexResult contains the results of an indexing operation
type IndexResult struct {
    Added   int
    Removed int
    Updated int
}

// UpdateChangedFiles re-indexes only changed files
func (m *IndexManager) UpdateChangedFiles(ctx context.Context, projectPath string) (*IndexResult, error) {
    // Get tracked files and their hashes
    tracked, err := m.db.GetTrackedFiles(ctx, projectPath)
    if err != nil {
        return nil, fmt.Errorf("failed to get tracked files: %w", err)
    }

    // Scan current files
    current, err := m.scanDirectory(projectPath)
    if err != nil {
        return nil, fmt.Errorf("failed to scan directory: %w", err)
    }

    // Determine changes
    toAdd := difference(current, tracked)
    toRemove := difference(tracked, current)
    toUpdate := m.findModified(tracked, current)

    result := &IndexResult{}

    // Remove deleted files from index
    for path := range toRemove {
        if err := m.removeFromIndex(ctx, path); err != nil {
            return nil, fmt.Errorf("failed to remove %s: %w", path, err)
        }
        result.Removed++
    }

    // Add new and updated files
    for path := range union(toAdd, toUpdate) {
        if err := m.indexFile(ctx, path); err != nil {
            return nil, fmt.Errorf("failed to index %s: %w", path, err)
        }
        if toAdd[path] {
            result.Added++
        } else {
            result.Updated++
        }
    }

    return result, nil
}

func (m *IndexManager) scanDirectory(root string) (map[string]string, error) {
    files := make(map[string]string)
    
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            // Skip excluded directories
            if m.shouldSkipDir(info.Name()) {
                return filepath.SkipDir
            }
            return nil
        }
        if m.shouldIndex(path) {
            hash, err := hashFile(path)
            if err != nil {
                return err
            }
            files[path] = hash
        }
        return nil
    })
    
    return files, err
}

func (m *IndexManager) shouldSkipDir(name string) bool {
    excluded := map[string]bool{
        "node_modules": true,
        "vendor":       true,
        ".git":         true,
        "dist":         true,
        "build":        true,
    }
    return excluded[name]
}
```

### 7.3 What Gets Indexed

**Automatically (on initial setup):**
- `docs/adr/*.md` → ADRs
- `README.md`, `docs/*.md` → Documentation
- `*.md` in root → Documentation

**On explicit request:**
- Source code files → Code chunks
- Playbooks/runbooks → Documentation

**Never automatically:**
- `node_modules/`, `vendor/`, `.git/`
- Binary files
- Files in `.gitignore`

### 7.4 Freshness Management

```go
package freshness

import (
    "context"
    "time"
)

// FreshnessTracker tracks knowledge freshness
type FreshnessTracker struct {
    db         *Storage
    thresholds map[DocType]time.Duration
}

// NewFreshnessTracker creates a tracker with default thresholds
func NewFreshnessTracker(db *Storage) *FreshnessTracker {
    return &FreshnessTracker{
        db: db,
        thresholds: map[DocType]time.Duration{
            DocTypeCode:    90 * 24 * time.Hour,  // Code changes frequently
            DocTypeADR:     365 * 24 * time.Hour, // ADRs are more stable
            DocTypeSession: 30 * 24 * time.Hour,  // Recent sessions most relevant
            DocTypePattern: 180 * 24 * time.Hour, // Patterns need periodic review
        },
    }
}

// CalculateStaleness returns staleness score 0.0 (fresh) to 1.0 (stale)
func (t *FreshnessTracker) CalculateStaleness(doc *Document) float64 {
    age := time.Since(doc.UpdatedAt)
    
    threshold, ok := t.thresholds[doc.Type]
    if !ok {
        threshold = 180 * 24 * time.Hour // Default: 6 months
    }

    staleness := float64(age) / float64(threshold)
    if staleness > 1.0 {
        return 1.0
    }
    return staleness
}

// MarkVerified marks document as recently verified (still accurate)
func (t *FreshnessTracker) MarkVerified(ctx context.Context, documentID string) error {
    return t.db.UpdateFreshness(ctx, documentID, &FreshnessUpdate{
        LastVerified:       time.Now(),
        VerificationStatus: StatusCurrent,
    })
}

// GetStaleDocuments returns documents exceeding their staleness threshold
func (t *FreshnessTracker) GetStaleDocuments(ctx context.Context, scope Scope) ([]*Document, error) {
    docs, err := t.db.ListDocuments(ctx, scope)
    if err != nil {
        return nil, err
    }

    var stale []*Document
    for _, doc := range docs {
        if t.CalculateStaleness(doc) >= 1.0 {
            stale = append(stale, doc)
        }
    }
    return stale, nil
}
```

---

## 8. Configuration

### 8.1 Global Configuration

`~/.edi/recall/config.yaml`:

```yaml
# RECALL Global Configuration
version: 1

# Embedding providers
embeddings:
  code:
    provider: voyage
    model: voyage-code-3
    # API key from VOYAGE_API_KEY env var
  
  docs:
    provider: openai
    model: text-embedding-3-large
    # API key from OPENAI_API_KEY env var

# Reranking configuration  
reranking:
  stage1:
    model: BAAI/bge-reranker-base
    enabled: true
  
  stage2:
    model: BAAI/bge-reranker-v2-m3
    enabled: true
  
  stage3:
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    enabled: true
    auto_trigger: true  # Trigger on complex queries

# Qdrant configuration
qdrant:
  mode: embedded  # 'embedded' or 'server'
  # For server mode:
  # host: localhost
  # port: 6333

# Search defaults
search:
  default_limit: 10
  max_limit: 50
  rerank_by_default: true

# Freshness settings
freshness:
  check_on_search: false  # Add staleness warnings to results
  auto_deprecate_after_days: 365
```

### 8.2 Project Configuration

`~/project/.edi/recall/config.yaml`:

```yaml
# RECALL Project Configuration
version: 1

# Project-specific overrides
project:
  name: my-project
  
  # Paths to auto-index on setup
  auto_index:
    - docs/adr/
    - README.md
    - CONTRIBUTING.md
  
  # Paths to exclude
  exclude:
    - node_modules/
    - dist/
    - "*.test.ts"
    - "*.spec.ts"

# Override global settings if needed
search:
  default_scope: project  # Don't search global by default
```

---

## 9. Error Handling

### 9.1 MCP Error Responses

```go
package errors

import "fmt"

// ErrorCode represents MCP error codes
type ErrorCode string

const (
    ErrNotFound       ErrorCode = "NOT_FOUND"
    ErrIndexFailed    ErrorCode = "INDEX_FAILED"
    ErrEmbeddingFailed ErrorCode = "EMBEDDING_FAILED"
    ErrSearchFailed   ErrorCode = "SEARCH_FAILED"
    ErrInvalidScope   ErrorCode = "INVALID_SCOPE"
)

// RecallError represents a structured error for MCP responses
type RecallError struct {
    Code    ErrorCode `json:"code"`
    Message string    `json:"message"`
}

func (e *RecallError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(id string) *RecallError {
    return &RecallError{
        Code:    ErrNotFound,
        Message: fmt.Sprintf("Document not found: %s", id),
    }
}

// NewIndexFailedError creates an index failed error
func NewIndexFailedError(path, reason string) *RecallError {
    return &RecallError{
        Code:    ErrIndexFailed,
        Message: fmt.Sprintf("Failed to index %s: %s", path, reason),
    }
}

// NewEmbeddingFailedError creates an embedding failed error
func NewEmbeddingFailedError(provider string) *RecallError {
    return &RecallError{
        Code:    ErrEmbeddingFailed,
        Message: fmt.Sprintf("Embedding provider %s unavailable. Check API key.", provider),
    }
}

// ToMCPError converts to MCP tool result error format
func (e *RecallError) ToMCPError() map[string]interface{} {
    return map[string]interface{}{
        "error": map[string]interface{}{
            "code":    e.Code,
            "message": e.Message,
        },
    }
}
```

### 9.2 Graceful Degradation

| Failure | Degraded Behavior |
|---------|-------------------|
| Embedding API unavailable | Fall back to BM25-only search |
| Qdrant unavailable | Return error; suggest `recall index --rebuild` |
| Reranker model not loaded | Skip reranking; return raw search results |
| Global scope missing | Search project only; warn user |

---

## 10. Dependencies

### 10.1 Go Libraries

| Dependency | Purpose | Import Path |
|------------|---------|-------------|
| MCP SDK | MCP server implementation | `github.com/modelcontextprotocol/go-sdk/mcp` |
| Hugot | ONNX reranker inference | `github.com/knights-analytics/hugot` |
| go-tree-sitter | AST parsing for code | `github.com/smacker/go-tree-sitter` |
| Qdrant client | Vector database | `github.com/qdrant/go-client` |
| go-sqlite3 | Metadata storage | `github.com/mattn/go-sqlite3` |
| Gin | Web UI framework | `github.com/gin-gonic/gin` |
| Cobra | CLI framework | `github.com/spf13/cobra` |
| Viper | Configuration | `github.com/spf13/viper` |

### 10.2 External Services (API)

| Service | Purpose | When Called |
|---------|---------|-------------|
| Voyage AI | Code embeddings (voyage-code-3) | Indexing |
| OpenAI | Doc embeddings (text-embedding-3-large) | Indexing |
| Anthropic | Contextual chunk generation (Haiku) | Indexing |
| Anthropic | Stage 3 reranking (Sonnet) | Complex queries only |

### 10.3 System Dependencies

| Dependency | Purpose | Installation |
|------------|---------|--------------|
| ONNX Runtime | Model inference | Download `onnxruntime.so` to `/usr/lib/` |
| libtokenizers | Tokenization | Static link from hugot release |

### 10.4 ONNX Models (Pre-converted)

| Model | Source | Purpose |
|-------|--------|---------|
| `bge-reranker-base-onnx-o4` | HuggingFace | Stage 1 reranking |
| `bge-reranker-v2-m3-onnx-o4` | HuggingFace | Stage 2 reranking |

### 10.5 Project Structure

```
codex/
├── cmd/
│   ├── recall-mcp/          # MCP server binary
│   ├── recall-web/          # Web UI binary
│   └── recall-cli/          # CLI binary
├── internal/
│   ├── core/                # Codex Core library
│   │   ├── search.go
│   │   ├── index.go
│   │   ├── rerank.go
│   │   └── graph.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── qdrant.go
│   ├── chunking/
│   │   ├── ast.go           # go-tree-sitter
│   │   └── contextual.go    # Claude Haiku
│   ├── embedding/
│   │   ├── voyage.go
│   │   └── openai.go
│   └── reranking/
│       └── hugot.go         # ONNX via hugot
├── pkg/
│   └── api/                 # Shared API types
├── web/                     # Web UI assets
├── go.mod
├── go.sum
└── Makefile
```

---

## 11. Implementation Plan

### 11.1 Phase 1: Codex Core Foundation (Week 1-2)

**Goal**: Working retrieval engine with basic search.

- [ ] Project structure and Go module setup
- [ ] SQLite schema implementation
- [ ] Qdrant embedded setup and collection creation
- [ ] Basic `Search()` function (vector-only, no reranking)
- [ ] Basic `Index()` function (code files only, go-tree-sitter)
- [ ] Configuration loading (Viper)
- [ ] Unit tests for core components

**Exit criteria**: Can index a Go project and search it programmatically.

### 11.2 Phase 2: Full Retrieval Pipeline (Week 3-4)

**Goal**: Production-quality retrieval with reranking.

- [ ] Hybrid search (vector + BM25 via Qdrant)
- [ ] Hugot integration for ONNX reranking
- [ ] Stage 1 + 2 reranking pipeline
- [ ] Query router implementation
- [ ] Contextual chunking for docs (Claude Haiku API)
- [ ] AST-aware chunking for multiple languages
- [ ] `Get()`, `List()`, `GraphQuery()` functions
- [ ] Integration tests

**Exit criteria**: Retrieval accuracy matches Codex design targets (>75%).

### 11.3 Phase 3: MCP Server (Week 5)

**Goal**: Working MCP server for Claude Code integration.

- [ ] MCP server using official Go SDK
- [ ] `recall_search` tool implementation
- [ ] `recall_get`, `recall_list`, `recall_context` tools
- [ ] `recall_add`, `recall_index` tools
- [ ] Multi-scope support (global + project)
- [ ] MCP error handling
- [ ] Integration test with Claude Code

**Exit criteria**: Can use RECALL from Claude Code via MCP.

### 11.4 Phase 4: Web UI (Week 6)

**Goal**: Browse knowledge base like a wiki.

- [ ] Gin-based web server
- [ ] Document listing and viewing
- [ ] Search interface
- [ ] ADR browser
- [ ] Session history viewer
- [ ] Basic styling (Tailwind or similar)

**Exit criteria**: Can browse and search knowledge via web browser.

### 11.5 Phase 5: EDI Integration (Week 7-8)

**Goal**: Full EDI workflow support.

- [ ] Session history integration
- [ ] Capture workflow (add from sessions)
- [ ] Scope promotion (project → global)
- [ ] Freshness tracking and staleness warnings
- [ ] Stage 3 reranking (Claude Sonnet, conditional)
- [ ] CLI for admin tasks
- [ ] Documentation

**Exit criteria**: Full EDI session lifecycle with RECALL.

### 11.6 Validation Criteria

| Metric | Target | Measurement |
|--------|--------|-------------|
| Search latency (p50) | < 300ms | Typical queries |
| Search latency (p95) | < 1s | Complex queries without Stage 3 |
| Retrieval accuracy | > 75% | Manual evaluation on test queries |
| Index time | < 5s/100 files | Code indexing |
| Binary size | < 50MB | Single binary without models |
| Memory usage | < 500MB | Idle with loaded models |

## 12. Retrieval Quality: LLM Judge Integration

### Overview

RECALL integrates an LLM judge pipeline to evaluate retrieval quality both in production sessions and offline evaluation. This ensures agents don't blindly trust search results and provides metrics for continuous improvement.

### `_judge_reminder` in Search Responses

The Codex MCP server injects a `_judge_reminder` field into every `recall_search` response:

```json
{
  "results": [...],
  "_judge_reminder": "Apply the retrieval-judge skill: evaluate each result for relevance before using. Log a retrieval_judgment entry via flight_recorder_log."
}
```

This field is not part of the result data — it's a behavioral nudge for the consuming agent.

### Expected Agent Behavior (Retrieval-Judge Skill)

Every EDI agent's context includes the retrieval-judge skill (`edi/internal/assets/skills/retrieval-judge/SKILL.md`), which mandates:

1. **Evaluate results** — For each `recall_search` result, assess title relevance, content applicability, and fit for the current task
2. **Log judgment** — Call `flight_recorder_log` with:
   - `type`: `"retrieval_judgment"`
   - `metadata`: `{"kept": ["id1", "id3"], "dropped": ["id2"], "reasoning": {...}}`
3. **Show summary** — Output `RECALL: X/Y results kept for '{query}'`

### Flight Recorder Audit Trail

Two entry types form the retrieval audit trail:

| Entry Type | Source | Auto/Manual | Contents |
|-----------|--------|-------------|----------|
| `retrieval_query` | MCP search handler | Automatic | Query text, filters applied, scored result list |
| `retrieval_judgment` | Agent via skill | Manual (agent responsibility) | Kept/dropped result IDs, per-result reasoning |

The `retrieval_query` entry is logged automatically by the MCP server on every `recall_search` call. The `retrieval_judgment` entry is the agent's responsibility, guided by the skill.

### Offline Evaluation

The `JudgeHarness` (`codex/eval/judge.go`) uses Claude Sonnet (`claude-sonnet-4-20250514`) to evaluate retrieval quality offline against the PayFlow test collection. It computes precision, recall, F1, and filtering rate metrics. See the deep-dive document Section 16 for full details.

---

## Appendix A: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 24, 2026 | Go as implementation language | Official MCP SDK; single binary; hugot for ONNX |
| Jan 24, 2026 | Multi-index scope model | Clean separation; different update patterns |
| Jan 24, 2026 | SQLite + Qdrant storage | Qdrant proven in Codex; native hybrid search |
| Jan 24, 2026 | Hugot for reranking | Production-tested; supports cross-encoder |
| Jan 24, 2026 | Minimal entity graph in v1 | Include imports/deps; defer complex queries |
| Jan 24, 2026 | Codex Core + interfaces | Shared library for MCP, Web UI, CLI |
| Jan 24, 2026 | Embedded Qdrant default | Simplest deployment for single-user |

---

## Appendix B: Related Documents

- `codex-architecture-deep-dive.md` — Retrieval engine design
- `edi-specification-plan.md` — EDI planning document
- `edi-quick-reference.md` — EDI overview
- `aef-architecture-specification-v0.5.md` — Parent framework

---

## Appendix C: Codex Core API Reference

```go
package core

import "context"

// CodexCore is the central knowledge retrieval engine
type CodexCore struct {
    storage  *Storage
    router   *QueryRouter
    reranker *Reranker
    chunker  *ChunkerFactory
}

// NewCodexCore creates a new Codex Core instance
func NewCodexCore(config *Config) (*CodexCore, error)

// Search performs semantic + lexical search across knowledge
func (c *CodexCore) Search(ctx context.Context, opts *SearchOptions) (*SearchResult, error)

// Get retrieves a document by ID
func (c *CodexCore) Get(ctx context.Context, id string, includeParent bool) (*Document, error)

// List returns documents matching filters
func (c *CodexCore) List(ctx context.Context, opts *ListOptions) ([]*DocumentSummary, error)

// Index adds a file or directory to the knowledge base
func (c *CodexCore) Index(ctx context.Context, opts *IndexOptions) (*IndexResult, error)

// Add creates a new knowledge item (from capture workflow)
func (c *CodexCore) Add(ctx context.Context, opts *AddOptions) (*Document, error)

// GraphQuery queries the entity relationship graph
func (c *CodexCore) GraphQuery(ctx context.Context, opts *GraphQueryOptions) ([]*Entity, error)

// Reindex rebuilds the index for a scope
func (c *CodexCore) Reindex(ctx context.Context, scope Scope) (*IndexResult, error)

// Stats returns statistics about the knowledge base
func (c *CodexCore) Stats(ctx context.Context) (*Stats, error)

// Close releases resources
func (c *CodexCore) Close() error
```

---

## Appendix D: MCP Server Implementation

```go
package main

import (
    "context"
    "log"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/yourorg/codex/internal/core"
)

func main() {
    // Initialize Codex Core
    codex, err := core.NewCodexCore(loadConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer codex.Close()

    // Create MCP server
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "recall",
        Version: "1.0.0",
    }, nil)

    // Register tools
    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_search",
        Description: "Search organizational knowledge",
    }, makeSearchHandler(codex))

    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_get",
        Description: "Retrieve document by ID",
    }, makeGetHandler(codex))

    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_index",
        Description: "Index a file or directory",
    }, makeIndexHandler(codex))

    // Run on stdio
    if err := mcp.ServeStdio(context.Background(), server); err != nil {
        log.Fatal(err)
    }
}

func makeSearchHandler(codex *core.CodexCore) mcp.ToolHandler {
    return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Extract parameters
        query := req.Params.Arguments["query"].(string)
        scope := req.Params.Arguments["scope"].(string)
        
        // Execute search
        result, err := codex.Search(ctx, &core.SearchOptions{
            Query: query,
            Scope: core.ParseScope(scope),
        })
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        
        // Return results
        return mcp.NewToolResultJSON(result), nil
    }
}
```
