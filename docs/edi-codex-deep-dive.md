# EDI + Codex Technical Deep-Dive

A comprehensive guide for engineers who will operate and maintain the EDI session harness and Codex knowledge retrieval system.

**EDI module:** `github.com/anthropics/aef/edi` — Go CLI harness
**Codex module:** `github.com/anthropics/aef/codex` — Knowledge engine
**Language:** Go 1.22+ (CGO required for SQLite)

---

## Table of Contents

### EDI System
0. [The Full Picture](#0-the-full-picture)
1. [EDI Session Lifecycle](#1-edi-session-lifecycle)
2. [Agent System](#2-agent-system)
3. [RECALL Backend Selection](#3-recall-backend-selection)
4. [Configuration](#4-configuration)
5. [Briefing Generation](#5-briefing-generation)
5A. [Ralph Loop Execution Mode](#5a-ralph-loop-execution-mode)

### Codex Internals
6. [Codex: System Overview](#6-codex-system-overview)
7. [Core Layer](#7-core-layer)
8. [Storage Layer](#8-storage-layer)
9. [Embedding Layer](#9-embedding-layer)
10. [Chunking Layer](#10-chunking-layer)
11. [Reranking Layer](#11-reranking-layer)
12. [MCP Protocol](#12-mcp-protocol)
13. [Web UI](#13-web-ui)
14. [CLI](#14-cli)
15. [Entry Points](#15-entry-points)
16. [Evaluation](#16-evaluation)
17. [Data Flow Diagrams](#17-data-flow-diagrams)
18. [Operational Guide](#18-operational-guide)

---

## 0. The Full Picture

EDI (Enhanced Development Intelligence) is the CLI harness. Codex is the knowledge engine. Together they provide AI-assisted software engineering with persistent organizational memory.

```
User → EDI CLI → Claude Code ↔ RECALL MCP (Codex or v0)
```

### How It Works

1. **EDI configures, then replaces itself.** The `edi` binary loads config, selects an agent, generates a briefing, starts a RECALL MCP server, and then calls `syscall.Exec` to replace its own process with Claude Code. From that point on, Claude Code runs natively.

2. **Claude Code runs with context.** EDI injects a system prompt file (`--append-system-prompt-file`) containing the agent personality, session briefing, RECALL instructions, and slash command definitions.

3. **RECALL provides knowledge retrieval.** Claude Code communicates with a RECALL MCP server (either v0 SQLite FTS or Codex hybrid vector) over stdin/stdout JSON-RPC. Five tools are available: `recall_search`, `recall_get`, `recall_add`, `recall_feedback`, `flight_recorder_log`.

4. **Codex is one of two RECALL backends.** The v0 backend uses simple SQLite FTS5 keyword search. Codex adds vector embeddings (nomic-embed-text via Ollama), brute-force KNN, and RRF fusion for hybrid retrieval. Both expose the identical 5-tool MCP interface.

---

## 1. EDI Session Lifecycle

**Key files:** `edi/internal/launch/context.go`, `edi/internal/launch/mcp.go`, `edi/internal/briefing/generator.go`

### Session Start

When the user runs `edi`, the following happens in order:

1. **Detect project** — find `.edi/` directory, determine project name
2. **Load config** — merge global (`~/.edi/config.yaml`) + project (`.edi/config.yaml`); project overrides global (arrays replace, not merge)
3. **Generate session ID** — UUID for this session, passed to RECALL as `EDI_SESSION_ID`
4. **Load agent** — read markdown + YAML frontmatter from `~/.edi/agents/{name}.md` or `.edi/agents/{name}.md` (project overrides global)
5. **Start RECALL MCP server** — write `.mcp.json` with the RECALL server config (v0 or Codex); Claude Code will launch the subprocess
6. **Generate briefing** — assemble profile (`.edi/profile.md`), recent session history, and task status into a markdown document
7. **Build context file** — combine agent system prompt + briefing + RECALL instructions + slash commands into a single markdown file written to `~/.edi/cache/session-{timestamp}.md`
8. **Launch Claude Code** — `syscall.Exec` replaces the EDI process with `claude --append-system-prompt-file {context-path}`

### During Session

- Claude Code runs natively with full terminal access
- RECALL tools available via MCP (Claude Code manages the subprocess lifecycle)
- Agent personality guides behavior (e.g., coder focuses on implementation, architect on design)
- Slash commands (`/plan`, `/build`, `/review`, `/incident`, `/task`, `/end`) switch modes

### Session End (`/end`)

1. Generate a session summary
2. Identify capture candidates (patterns, decisions worth saving)
3. Present candidates with structured content templates:
   - **Decisions**: Context, Decision, Alternatives Considered, Consequences, Files
   - **Patterns**: Pattern description, When to Use, Implementation, Files
   - **Failures**: Symptom, Root Cause, Fix, Prevention, Files
4. Prompt user to save approved items to RECALL via `recall_add`
5. Update `.edi/status.md` and save session history to `.edi/history/`

### Stale Session Recovery

**Key file:** `edi/internal/launch/recovery.go`

On startup, EDI detects stale sessions — previous sessions that exited without running `/end` (e.g., terminal closed, Ctrl+C). Detection works by checking if `active.yaml` has a `last_session_id` with no corresponding history file in `.edi/history/`.

When a stale session is detected:
1. EDI warns the user and offers to launch the `/end-recovery` command
2. `/end-recovery` gathers context from `git log`, `git diff`, and `.edi/status.md`
3. The user provides what they remember working on
4. A recovery summary is generated and saved to `.edi/history/`

---

## 2. Agent System

**Key file:** `edi/internal/agents/loader.go`

### Agent Format

Agents are markdown files with YAML frontmatter:

```markdown
---
name: coder
description: Default coding mode for implementation work
tools:
  - recall_search
  - recall_add
skills:
  - code-review
  - debugging
---

# Coder Agent

You are EDI operating in **Coder** mode, focused on implementation.
...
```

### Loading

`agents.Load(name)` checks two locations in order:
1. **Project:** `.edi/agents/{name}.md` (current working directory)
2. **Global:** `~/.edi/agents/{name}.md`

Project agents override global agents of the same name.

### Core Agents

| Agent | Purpose |
|-------|---------|
| `coder` | Implementation — clean, tested code following project conventions |
| `architect` | System design — ADRs, trade-offs, long-term implications |
| `reviewer` | Code review — quality, security, performance issues |
| `incident` | Troubleshooting — rapid diagnosis, runbooks, mitigation |

### Slash Commands

Each agent's system prompt includes instructions for these commands:

| Command | Aliases | Action |
|---------|---------|--------|
| `/plan` | `/architect`, `/design` | Switch to architect mode |
| `/build` | `/code`, `/implement` | Switch to coder mode |
| `/review` | `/check` | Switch to reviewer mode |
| `/incident` | `/debug`, `/fix` | Switch to incident mode |
| `/task` | — | Manage tasks with RECALL enrichment |
| `/end` | — | End session, save history |
| `/end-recovery` | — | Recover from unclean session exit |

### RECALL Usage in Agents

Every agent's system prompt includes patterns for RECALL integration:
- `recall_search` before implementing patterns you've seen before
- `flight_recorder_log` for significant decisions with rationale
- `recall_add` to capture new patterns, failures, or decisions

---

## 3. RECALL Backend Selection

**Key file:** `edi/internal/launch/mcp.go`

### Configuration

```yaml
recall:
  enabled: true
  backend: v0    # "v0" (default) or "codex"
```

### Two Backends, One Interface

Both backends expose the identical 5-tool MCP interface. Claude Code and agents are unaware of which backend is running.

| | v0 (SQLite FTS) | Codex (Hybrid Vector) |
|---|---|---|
| **Binary** | `edi recall-server` | `recall-mcp` |
| **Search** | FTS5 BM25 keyword only | Vector KNN + FTS5 BM25 + RRF fusion |
| **Embeddings** | None | nomic-embed-text via Ollama (768-dim) |
| **Storage** | `~/.edi/recall/global.db` | `~/.edi/codex.db` |
| **Dependencies** | None | Ollama running locally |
| **Tools** | 5 (identical) | 5 (identical) |

### MCP Config Generation

`GetRecallMCPConfig()` returns a `MCPServerConfig` struct that gets written to `.mcp.json` in the project directory. Claude Code reads this file to know how to launch the MCP subprocess.

**v0 config:**
```json
{
  "mcpServers": {
    "recall": {
      "type": "stdio",
      "command": "~/.edi/bin/edi",
      "args": ["recall-server", "--session-id", "{id}", "--global-db", "~/.edi/recall/global.db"]
    }
  }
}
```

**Codex config:**
```json
{
  "mcpServers": {
    "recall": {
      "type": "stdio",
      "command": "~/.edi/bin/recall-mcp",
      "env": {
        "EDI_SESSION_ID": "{id}",
        "EDI_AGENT_MODE": "{agent}",
        "EDI_GIT_BRANCH": "{branch}",
        "EDI_GIT_SHA": "{sha}",
        "EDI_PROJECT_PATH": "{cwd}",
        "EDI_PROJECT_NAME": "{project}",
        "CODEX_METADATA_DB": "~/.edi/codex.db"
      }
    }
  }
}
```

The additional environment variables are populated by `gitInfo()` in `mcp.go`, which calls `git rev-parse` for branch and SHA, and uses `os.Getwd()` / `filepath.Base()` for project path and name. These are passed to the Codex MCP server so that `recall_add` can auto-inject them as metadata on every knowledge item.

---

## 4. Configuration

**Key file:** `edi/internal/config/schema.go`

### File Locations

- **Global:** `~/.edi/config.yaml`
- **Project:** `.edi/config.yaml`
- Project overrides global. Arrays replace (not merge).

### Schema

```go
type Config struct {
    Version  string          // Config schema version
    Agent    string          // Current agent mode (e.g., "coder")
    Recall   RecallConfig    // RECALL backend settings
    Codex    CodexConfig     // Codex-specific settings (when backend = "codex")
    Briefing BriefingConfig  // Briefing generation settings
    Capture  CaptureConfig   // Knowledge capture settings
    Tasks    TasksConfig     // Task integration settings
    Project  ProjectConfig   // Project-specific settings
}

type RecallConfig struct {
    Enabled bool    // Enable RECALL (default: true)
    Backend string  // "v0" or "codex" (default: "v0")
}

type CodexConfig struct {
    ModelsPath string  // Path to ONNX reranker models
    MetadataDB string  // Path to SQLite DB (default: ~/.edi/codex.db)
    BinaryPath string  // Path to recall-mcp binary (default: ~/.edi/bin/recall-mcp)
}

type BriefingConfig struct {
    IncludeHistory bool  // Include recent session history
    HistoryEntries int   // Number of history entries to include
    IncludeTasks   bool  // Include current task status
    IncludeProfile bool  // Include project profile
}
```

---

## 5. Briefing Generation

**Key file:** `edi/internal/briefing/generator.go`

### Sources

The briefing assembles context from three sources:

1. **Profile** (`.edi/profile.md`) — project overview, architecture, conventions, tech stack. Written by the user. Included verbatim.

2. **Recent session history** (`.edi/history/`) — summaries of recent sessions with dates and accomplishments. Configurable via `briefing.history_entries`.

3. **Task status** — counts of completed/in-progress/pending tasks, with lists of in-progress and ready-to-start items.

### Injection

The briefing is rendered to markdown by `Briefing.Render(projectName)` and included in the context file built by `BuildContext()`. The context file is passed to Claude Code via:

```
claude --append-system-prompt-file ~/.edi/cache/session-{timestamp}.md
```

### Context File Structure

The full context file that Claude Code receives contains:

1. EDI identity and session ID
2. Current agent mode and system prompt
3. Session briefing (profile + history + tasks)
4. Briefing display instructions
5. RECALL knowledge base instructions (if enabled)
6. Slash command definitions

---

## 5A. Ralph Loop Execution Mode

**Key files:** `edi/internal/assets/ralph/ralph.sh`, `edi/internal/assets/ralph/PROMPT.md`, `edi/internal/assets/ralph/example-PRD.json`

Ralph is an autonomous execution mode that runs well-defined coding tasks in a loop. Each iteration starts with a fresh context window — no accumulated state in the LLM, all progress tracked in files and git.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ralph.sh (bash)                          │
│                                                                  │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │
│   │ Select   │───▶│ Build    │───▶│ Run      │───▶│ Analyze  │ │
│   │ Task     │    │ Prompt   │    │ claude -p│    │ Output   │ │
│   │ (jq)     │    │          │    │          │    │ (grep)   │ │
│   └──────────┘    └──────────┘    └──────────┘    └──────────┘ │
│        ▲                                               │        │
│        │              ┌────────────────┐               │        │
│        │              ▼                │               ▼        │
│        │       ┌────────────┐   ┌───────────┐   ┌──────────┐  │
│        │       │ Escalate   │   │ Update    │   │ Next     │  │
│        │       │ to Human   │   │ PRD.json  │   │ Task     │  │
│        │       └────────────┘   │ + git     │   └──────────┘  │
│        │              │         └───────────┘        │         │
│        └──────────────┴──────────────────────────────┘         │
│                                                                  │
│ External State:                                                  │
│   PRD.json        Task backlog with status                      │
│   PROMPT.md       Execution instructions for Claude             │
│   .ralph/         Working directory (outputs, prompts, input)   │
│   Git             Code changes committed per task               │
└─────────────────────────────────────────────────────────────────┘
```

### State Management

Ralph persists all state externally:

| State | Storage | Purpose |
|-------|---------|---------|
| Task backlog | `PRD.json` | User stories with `passes`, `skipped`, `depends_on` fields |
| Execution instructions | `PROMPT.md` | Injected into every iteration prompt |
| Human guidance | `.ralph/human-input.txt` | Temporary — consumed on next iteration |
| Claude output | `.ralph/output_N.txt` | One file per iteration, kept for debugging |
| Built prompt | `.ralph/prompt.md` | Combined task details + guidance + PROMPT.md |
| Code changes | Git | Committed after each completed task |

No state lives in the LLM. If the loop is killed mid-iteration, restart it and it picks up from the last committed task.

### Loop Mechanics

**Task selection** uses `jq` to filter `PRD.json`:
1. Filter tasks where `passes == false` and `skipped != true`
2. Filter tasks where all `depends_on` entries have `passes == true`
3. Select first remaining task

**Prompt building** concatenates:
1. Task details (title, description, acceptance criteria) from `PRD.json`
2. Human guidance from previous escalation (if any, from `.ralph/human-input.txt`)
3. `PROMPT.md` instructions (escalation protocol, completion format)

**Claude invocation:** `cat .ralph/prompt.md | claude -p 2>&1 | tee .ralph/output_N.txt`

**Output analysis** checks (in order):
1. `<promise>DONE</promise>` — all tasks complete, exit loop
2. `<escalate ...>` — escalation detected, prompt human
3. Task completion patterns (regex: `task US-001 complete`, `US-001 done`, `all acceptance criteria met`) — mark task done, commit, continue
4. Error extraction — track consecutive identical errors for auto-escalation

### Escalation Protocol

Two escalation types:

**STUCK** — Cannot make progress (same error 3+ times, blocked by external factors, doesn't know how to proceed)

**DEVIATION** — Can proceed but shouldn't without approval (spec wrong, scope larger than expected, out-of-scope changes needed, security concern)

Claude outputs an `<escalate>` XML block and stops. The loop script detects this, displays the escalation to the human, and offers options:

| Option | Effect |
|--------|--------|
| `[1-9]` | Guidance injected: "Proceed with option N" |
| `[c]` | Custom free-form text injected into next prompt |
| `[s]` | Task marked skipped, loop continues |
| `[r]` | Re-run iteration without additional guidance |
| `[a]` | Exit loop entirely |

**Auto-escalation:** When `extract_error` detects the same error string on `STUCK_THRESHOLD` (default 3) consecutive iterations, the script generates a synthetic `<escalate type="stuck">` block and prompts the human.

### Claude Invocation Mode

Ralph uses `claude -p` (pipe mode), not Claude Code (interactive mode). This is deliberate:

- **Pipe mode** reads stdin, produces output, exits. No MCP tools, no file system access, no interactive capabilities. This matches Ralph's design: focused execution of one task with all context in the prompt.
- **Claude Code** would provide file editing, terminal access, and MCP tools. This is unnecessary overhead for Ralph — the spec should contain everything Claude needs. If it doesn't, the spec isn't ready.
- **Interactive mode** would require human presence. Ralph is designed for unattended execution with escalation as the exception.

### Why No RECALL in Ralph

RECALL is deliberately excluded from the Ralph execution loop:

1. **Technical:** `claude -p` doesn't support MCP tools. Claude would see RECALL instructions but couldn't execute them.
2. **Conceptual:** If the spec needs RECALL queries, the spec isn't complete. Fix it in planning.
3. **Practical:** Pre-baked context in the spec is more reliable than runtime retrieval.

The correct flow is: **Plan** (interactive session, use RECALL to inform the PRD) → **Execute** (Ralph, no RECALL) → **Capture** (post-execution, save new patterns/failures to RECALL).

### File Structure

After `edi init --global`, Ralph files are installed to `~/.edi/ralph/`:

```
~/.edi/ralph/
├── ralph.sh           Loop script (copy or symlink to project)
├── PROMPT.md          Default execution instructions
└── example-PRD.json   Template PRD with sample user stories
```

Per-project working directory (gitignored):

```
.ralph/
├── prompt.md          Built prompt for current iteration
├── human-input.txt    Temporary human guidance (consumed)
└── output_N.txt       Claude output per iteration
```

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_ITERATIONS` | 50 | Maximum loop iterations before forced exit |
| `STUCK_THRESHOLD` | 3 | Consecutive identical errors before auto-escalation |

### Integration with EDI

Ralph is independent of EDI. It does not use EDI's agents, briefings, RECALL, or session management. The only connection is that Ralph files are distributed via `edi init --global`.

Future: `edi ralph PRD.json` is a planned invocation shortcut, not yet implemented.

### Preflight Checks

Before starting, `ralph.sh` verifies:
- `PRD.json` exists in the current directory
- `PROMPT.md` exists in the current directory
- `claude` CLI is in PATH
- `jq` is in PATH

If any check fails, the script exits immediately with an error message.

### Completion Detection

Task completion is detected via regex matching on Claude's output:
- `task {ID} complete` (case-insensitive)
- `{ID} done|complete|finished`
- `completed task {ID}`
- `all acceptance criteria met`

All-tasks-done detection: `<promise>DONE</promise>` anywhere in output.

If Claude's output doesn't match any pattern, the task is **not** marked complete and the same task is retried on the next iteration.

### Git Commits

Ralph commits after each state change:
- Task complete: `git add -A && git commit -m "Ralph: complete {task_id}"`
- Progress (no completion): `git add -A && git commit -m "Ralph: progress on {task_id}"`
- All done: `git add -A && git commit -m "Ralph: all tasks complete"`

Commits are non-fatal — if `git commit` fails (no changes, not a repo), the loop continues.

---

## 6. Codex: System Overview

### Architecture Diagram

```
                        +------------------+
                        |   Entry Points   |
                        +------------------+
                        |  recall-mcp      |  JSON-RPC stdio (MCP protocol)
                        |  codex-web       |  Gin HTTP server (:8080)
                        |  codex-cli       |  Cobra CLI (admin)
                        |  codex-testgen   |  Eval test data server (:8088)
                        +--------+---------+
                                 |
                        +--------v---------+
                        |   SearchEngine   |  internal/core/engine.go
                        |   (orchestrator) |
                        +--------+---------+
                                 |
              +------------------+------------------+
              |                  |                  |
    +---------v------+  +-------v--------+  +------v-------+
    |   Embedder     |  | VectorStorage  |  |MetadataStore |
    | (Ollama local) |  | (SQLite BLOB + |  |(SQLite FTS5) |
    | nomic-embed-   |  |  brute-force   |  |  + metadata  |
    | text, 768-dim) |  |     KNN)       |  |   tables     |
    +----------------+  +----------------+  +--------------+
              |                  |                  |
              |         +-------v------------------v-------+
              |         |         Single SQLite DB          |
              |         |  ~/.edi/codex.db (WAL mode)       |
              |         +----------------------------------+
              |
    +---------v------+   +----------------+   +----------------+
    |   Chunking     |   |   Reranking    |   |   RRF Fusion   |
    | AST (Tree-     |   | (STUB - not    |   | 2-way, k=60    |
    |  sitter) +     |   |  functional)   |   |                |
    |  Markdown      |   +----------------+   +----------------+
    +----------------+
```

### Component Map

| Package | Path | Purpose |
|---------|------|---------|
| `core` | `internal/core/` | Types, interfaces, SearchEngine, Indexer, RRF fusion, migration |
| `storage` | `internal/storage/` | SQLite metadata (MetadataStore), vector BLOBs (VecStore), FTS5 keyword search |
| `embedding` | `internal/embedding/` | Ollama client for nomic-embed-text embeddings |
| `chunking` | `internal/chunking/` | AST chunking (Tree-sitter), markdown chunking, contextual chunking (stub) |
| `reranking` | `internal/reranking/` | Reranker stub (not functional) |
| `mcp` | `internal/mcp/` | JSON-RPC stdio MCP server, 5 tools |
| `web` | `internal/web/` | Gin HTTP server, web UI + REST API |
| `eval` | `eval/` | Evaluation harness, metrics, LLM judge, PayFlow test data |
| `codex-cli` | `cmd/codex-cli/` | Admin CLI |
| `recall-mcp` | `cmd/recall-mcp/` | MCP server entry point |
| `codex-web` | `cmd/codex-web/` | Web server entry point |
| `codex-testgen` | `cmd/codex-testgen/` | Test data generation server |

### Key Design Decisions

- **Single SQLite database** for metadata, vectors, FTS5, feedback, and flight recorder. No external dependencies beyond Ollama.
- **Brute-force KNN** instead of ANN. At less than 10K documents this gives exact results in sub-millisecond time. The tradeoff is linear scan cost; if the corpus grows beyond ~50K items, this will need revisiting.
- **nomic-embed-text via Ollama** as the sole embedding model. 768-dimensional vectors. Asymmetric prefixes for search vs. document.
- **2-way RRF fusion** (vector + FTS5) rather than a single retrieval path. This handles both semantic and keyword queries well.

---

## 7. Core Layer

**Path:** `codex/internal/core/`

### Types (`types.go`)

```go
// Item types (constants)
TypePattern  = "pattern"    // Code patterns
TypeFailure  = "failure"    // Failure patterns
TypeDecision = "decision"   // Architecture decisions
TypeContext  = "context"    // General context
TypeCode     = "code"       // Code chunks
TypeDoc      = "doc"        // Documentation chunks
TypeRunbook  = "runbook"    // Runbooks
TypeManual   = "manual"     // Manually added

// Core data types
Item               // id, type, title, content, tags, scope, source, metadata, timestamps
SearchRequest      // query, types filter, scope filter, limit, use_hybrid flag
SearchResult       // embeds Item + score + highlights
IndexRequest       // content, type, file_path, language, tags, scope
IndexResult        // item_id, chunks_count
FlightRecorderEntry // id, session_id, timestamp, type, content, rationale, metadata
Feedback           // item_id, session_id, useful (bool), context, timestamp

// Config
Config {
    AnthropicAPIKey     string   // Optional, for contextual chunking (not yet functional)
    ModelsPath          string   // Optional, for reranking models (not yet functional)
    MetadataDBPath      string   // SQLite DB path, default ~/.edi/codex.db
    LocalEmbeddingURL   string   // Ollama URL, default http://localhost:11434/api/embed
    LocalEmbeddingModel string   // Model name, default nomic-embed-text
    ScoreThreshold      float64  // Min score ratio vs top result. 0 = disabled. Typical: 0.5
}
```

### Interfaces (`interfaces.go`)

All core dependencies are expressed as interfaces for testability:

| Interface | Implementation | Purpose |
|-----------|---------------|---------|
| `Embedder` | `embedding.LocalClient` | `EmbedDocument(ctx, text)` and `EmbedQuery(ctx, query)` |
| `VectorStorage` | `storage.VecStore` | `Upsert`, `Search` (KNN), `Delete` |
| `KeywordSearcher` | `storage.MetadataStore` | `KeywordSearch(query, limit)` via FTS5 BM25 |
| `MetadataStorage` | `storage.MetadataStore` | CRUD for items, feedback, flight recorder |
| `Reranker` | `reranking.Reranker` | `Rerank(query, docs, topK)` -- currently a stub |
| `CodeChunker` | `chunking.ASTChunker` | `ChunkFile(content, lang, filePath)` |
| `DocChunker` | `chunking.ContextualChunker` | `ChunkDocument(ctx, content, filePath)` -- currently a stub |

### SearchEngine (`engine.go`)

The `SearchEngine` is the central orchestrator. Created via:

```go
engine, err := core.NewSearchEngine(ctx, config)
// or for testing:
engine := core.NewSearchEngineWithDeps(deps)
```

Construction initializes in order:
1. `MetadataStore` (opens/creates SQLite DB, runs migrations)
2. `VecStore` (creates vectors table, loads all vectors into memory)
3. `LocalClient` (embedding client, points to Ollama)
4. `Reranker` (optional, logs warning if models not found)

### Search Pipeline (9 steps)

The `Search` method executes these steps in order:

```
Step 1: Embed query
    queryVec = embedder.EmbedQuery(ctx, req.Query)
    Applies "search_query: " prefix for asymmetric search.

Step 2: Vector KNN search
    vectorResults = vecStore.Search(ctx, queryVec, candidateLimit)
    Brute-force cosine similarity over all in-memory vectors.
    candidateLimit = 50 (with reranker) or min(limit*3, 20) (without).

Step 3: FTS5 BM25 keyword search
    keywordResults = keywords.KeywordSearch(req.Query, candidateLimit)
    Query is wrapped in double quotes for literal phrase matching.
    Failure is non-fatal -- vector results still returned.

Step 4: 2-way RRF fusion (k=60)
    results = reciprocalRankFusion(vectorResults, keywordResults, 60)
    Merges both lists. Score = sum(1/(60 + rank)) across lists.

Step 5: Hydrate metadata
    For vector-only results (missing Title/Content), fetch from MetadataStore.

Step 6: Apply type/scope filters
    Filter by req.Types and req.Scope if specified.

Step 7: Rerank (if reranker available)
    Currently never executes -- reranker.available is always false.

Step 8: Score threshold cutoff
    If config.ScoreThreshold > 0, drop results below topScore * threshold.

Step 9: Limit
    Truncate to req.Limit (default 10).
```

### Indexer (`index.go`)

The `Indexer` routes content through three pipelines based on type:

- **Code** (`indexCode`): AST chunking via Tree-sitter, each chunk gets its own embedding and metadata record. Chunk IDs: `{parentID}-chunk-{i}`.
- **Doc** (`indexDoc`): Contextual chunker if available (it is not -- see section 10), otherwise falls back to `ChunkMarkdown` with 2000-char max chunks.
- **Manual** (`indexManual`): Single item, no chunking. Used for patterns, failures, decisions added via MCP.

Directory indexing (`IndexDirectory`) walks the filesystem, skips hidden files, and indexes files with recognized extensions (.go, .py, .ts, .js, .rs, .md, .txt, etc.).

### RRF Fusion (`fusion.go`)

```go
func reciprocalRankFusion(vectorResults []storage.ScoredResult,
    keywordResults []SearchResult, k float64) []SearchResult
```

Standard Reciprocal Rank Fusion with k=60. For each document appearing in any result list, the fused score is:

```
score(d) = sum over lists L where d appears: 1 / (k + rank_L(d))
```

Where `rank_L(d)` is the 1-indexed position in list L. Results are sorted by descending fused score. Vector-only results have empty metadata and must be hydrated by the caller.

### Migration (`migrate.go`)

Migrates items from a RECALL v0 SQLite FTS database to Codex v1:

- Opens the v0 database, reads all items
- Processes in batches of 50
- Routes through the Indexer (re-embeds with nomic-embed-text, chunks code/docs)
- Tracks stats: total, migrated, failed, chunks created
- Type mapping: v0 types map to v1 types (pattern -> pattern, decision/adr -> decision, etc.)

---

## 8. Storage Layer

**Path:** `codex/internal/storage/`

### SQLite Configuration

The database is opened with these pragmas:

```
?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON
```

WAL mode enables concurrent reads during writes. Busy timeout of 5 seconds prevents immediate lock failures.

### Schema (`metadata.go`)

**`items` table** -- primary metadata store:

```sql
CREATE TABLE items (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,           -- pattern, failure, decision, context, code, doc
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,                    -- JSON array as string
    scope TEXT NOT NULL DEFAULT 'project',
    source TEXT,                  -- file path or "manual"
    metadata TEXT,               -- JSON object as string
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX idx_items_type ON items(type);
CREATE INDEX idx_items_scope ON items(scope);
```

**`items_fts` virtual table** -- FTS5 full-text search:

```sql
CREATE VIRTUAL TABLE items_fts USING fts5(
    title, content, tags,
    content=items, content_rowid=rowid,
    tokenize='porter unicode61'
);
```

Kept in sync with `items` via three triggers (`items_ai`, `items_ad`, `items_au`) that fire on INSERT, DELETE, and UPDATE. Uses Porter stemming and Unicode tokenization.

On startup, if `items_fts` is empty but `items` has data, a full rebuild is triggered:
```sql
INSERT INTO items_fts(items_fts) VALUES('rebuild')
```

**`feedback` table**:

```sql
CREATE TABLE feedback (
    id TEXT PRIMARY KEY,
    item_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    useful INTEGER NOT NULL,      -- boolean as int
    context TEXT,
    timestamp DATETIME NOT NULL,
    FOREIGN KEY (item_id) REFERENCES items(id)
);
CREATE INDEX idx_feedback_item ON feedback(item_id);
```

**`flight_recorder` table**:

```sql
CREATE TABLE flight_recorder (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,            -- decision, error, milestone, observation, retrieval_query, retrieval_judgment
    content TEXT NOT NULL,
    rationale TEXT,
    metadata TEXT,                 -- JSON object as string
    timestamp DATETIME NOT NULL
);
CREATE INDEX idx_flight_session ON flight_recorder(session_id);
CREATE INDEX idx_flight_type ON flight_recorder(type);
```

**`schema_version` table**:

```sql
CREATE TABLE schema_version (version INTEGER NOT NULL);
```

Current version: 1. On startup, if the database has a newer version than the binary supports, it returns an error telling the user to upgrade Codex.

### Keyword Search

`KeywordSearch` sanitizes the user query by wrapping it in double quotes to prevent FTS5 syntax injection. This means queries are treated as literal phrase matches -- FTS5 operators like AND, OR, NEAR, and wildcards are intentionally disabled.

Results are ranked by BM25 (using SQLite's built-in `rank` column, negated so higher = better).

### Vector Store (`vecstore.go`)

**`vectors` table**:

```sql
CREATE TABLE vectors (
    item_id    TEXT PRIMARY KEY,
    embedding  BLOB NOT NULL,     -- float32 array as little-endian bytes
    dimensions INTEGER NOT NULL   -- 768 for nomic-embed-text
);
```

**In-memory cache:** On initialization, `loadAll()` reads every vector from the database into a `map[string][]float32`. All KNN searches happen against this in-memory map.

**Vector serialization:** Each `float32` is stored as 4 little-endian bytes. A 768-dim vector = 3072 bytes per row.

**Upsert:** Normalizes the vector (L2 norm), writes BLOB to SQLite, updates in-memory map. Uses `ON CONFLICT` for upsert semantics.

**Search (brute-force KNN):**

```
1. Normalize query vector
2. Acquire read lock on vector map
3. For each stored vector:
   a. Skip if dimension mismatch
   b. Compute dot product (= cosine similarity since vectors are normalized)
   c. Maintain min-heap of size K for top-K tracking
4. Pop heap in reverse order for descending score
```

This is O(N) where N is the number of stored vectors. The comment in the source states this is sub-millisecond for less than 10K documents.

**Thread safety:** Protected by `sync.RWMutex`. Reads (Search) take RLock; writes (Upsert, Delete) take full Lock.

---

## 9. Embedding Layer

**Path:** `codex/internal/embedding/local.go`

### Ollama Client

The sole embedding implementation. Talks to an Ollama-compatible HTTP endpoint.

| Setting | Default | Env Var |
|---------|---------|---------|
| Base URL | `http://localhost:11434/api/embed` | `LOCAL_EMBEDDING_URL` |
| Model | `nomic-embed-text` | `LOCAL_EMBEDDING_MODEL` |
| HTTP timeout | 30 seconds | -- |
| Max retries | 5 | -- |
| Initial backoff | 1 second | -- |

### Asymmetric Prefixes

nomic-embed-text uses asymmetric search/document prefixes:

- **Documents** (indexing): `"search_document: " + text`
- **Queries** (search): `"search_query: " + query`

This is critical for retrieval quality. The model was trained with these prefixes and performs significantly worse without them.

### Retry Behavior

Exponential backoff: delay = 2^attempt * 1 second. Retries on:
- Network errors (all attempts)
- HTTP 5xx (server errors)

Does NOT retry on HTTP 4xx (client errors) -- returns immediately.

### Output

Returns `[]float32` with 768 dimensions. The Ollama API returns `{"embeddings": [[...]]}` and the client takes `embeddings[0]`.

---

## 10. Chunking Layer

**Path:** `codex/internal/chunking/`

### Types (`types.go`)

```go
CodeChunk       // content, type (function/class/method/type), name, start/end line, file path, signature, language
DocChunk        // original_content, context (from Haiku), enriched_content, file path, section, start/end line
MarkdownSection // title, content, level, start/end line
```

### AST Chunking (`ast.go`)

Uses [go-tree-sitter](https://github.com/smacker/go-tree-sitter) for syntax-aware code splitting.

**Supported languages:**
- Go: functions, methods, type declarations
- Python: functions, classes
- TypeScript/JavaScript/TSX/JSX: functions, methods, classes, interfaces

**Extraction logic:** Walks the AST tree recursively. For each semantic node (function, class, type), extracts:
- Full source content
- Name (from `identifier` or `name` child node)
- Signature (first line up to opening brace or colon)
- Line range

**Fallback:** For unsupported languages or parse failures, falls back to line-based chunking: 100 lines per chunk with 10-line overlap.

### Markdown Chunking (`markdown.go`)

`ChunkMarkdown(content, maxChunkSize)`:

1. Split content at heading boundaries (`# `, `## `, etc.)
2. Sections that exceed `maxChunkSize` (default 2000 chars) are further split at paragraph boundaries (`\n\n`)
3. Content before the first heading becomes an "(Introduction)" section

### Contextual Chunking (`contextual.go`)

**STATUS: NOT FUNCTIONAL (STUB)**

`ContextualChunker` is designed to enrich document chunks with contextual descriptions using Claude Haiku. Currently:

- `NewContextualChunker` requires an `ANTHROPIC_API_KEY` and returns a struct, but...
- `EnrichChunk` always returns `fmt.Errorf("contextual enrichment not implemented")`
- `ChunkDocument` calls `EnrichChunk`, catches the error, and falls back to empty context strings
- The intended flow: send each chunk + document context to Claude Haiku, get back a 1-2 sentence situating description, prepend it to the chunk before embedding

When this is implemented, the Anthropic SDK will be used to call `claude-3-haiku-20240307` with `max_tokens=100`.

---

## 11. Reranking Layer

**Path:** `codex/internal/reranking/`

**STATUS: NOT FUNCTIONAL (STUB)**

The `Reranker` struct has `available: false` always. No model inference actually occurs.

### Current Behavior

- `NewReranker(modelsPath)` checks for ONNX model files at:
  - `{modelsPath}/bge-reranker-base/model.onnx` (stage 1)
  - `{modelsPath}/bge-reranker-v2-m3/model.onnx` (stage 2)
- Even if the files exist, `available` remains `false` because the Hugot library integration is not implemented
- `Rerank()` returns documents in original order with placeholder scores: `1.0 - i*0.01`
- The engine checks `if e.reranker != nil` before calling Rerank, and the reranker constructor only returns non-nil when `config.ModelsPath != ""`. Even then, the fallback scores do not change result ordering.

### Intended Design (When Implemented)

Two-stage reranking pipeline:
1. **Stage 1:** BGE-reranker-base (50 candidates down to 20)
2. **Stage 2:** BGE-reranker-v2-m3 (20 candidates down to final limit)

Both stages would format inputs as `"query [SEP] document"` pairs and run through ONNX models via the [Hugot](https://github.com/knights-analytics/hugot) Go library.

---

## 12. MCP Protocol

**Path:** `codex/internal/mcp/`

### Server (`server.go`)

JSON-RPC 2.0 over stdio (newline-delimited JSON). Implements the Model Context Protocol (MCP).

**Protocol version:** `2024-11-05`
**Server name:** `codex` version `1.0.0`

**Supported methods:**
- `initialize` -- returns protocol version, server info, capabilities
- `notifications/initialized` -- no-op (notification)
- `tools/list` -- returns 5 tool definitions
- `tools/call` -- dispatches to tool handler

### Tools (`tools.go`)

| Tool | Required Params | Optional Params | Purpose |
|------|----------------|-----------------|---------|
| `recall_search` | `query` (string) | `types` (string[]), `scope` (string), `limit` (int, default 10) | Hybrid search. Auto-logs a `retrieval_query` flight recorder entry. |
| `recall_get` | `id` (string) | -- | Fetch item by ID |
| `recall_add` | `type`, `title`, `content` (strings) | `tags` (string[]), `scope` (string, default "project") | Add knowledge item. ID format: `{prefix}-{uuid8}`. Auto-injects session/git metadata. |
| `recall_feedback` | `item_id` (string), `useful` (bool) | `context` (string) | Record feedback on an item |
| `flight_recorder_log` | `type`, `content` (strings) | `rationale` (string), `metadata` (object) | Log decision/error/milestone/observation |

**ID prefixes:** P=pattern, F=failure, D=decision, C=context, X=code, O=doc, R=runbook, I=other.

**Input limits:**
- Query: 10KB max
- Content: 1MB max

**Audit trail:** Every `recall_search` call automatically logs a `retrieval_query` entry to the flight recorder with the query, filters, and scored result list.

**Auto-injected metadata on `recall_add`:** The Codex MCP server automatically injects 6 environment variables as metadata fields on every item added via `recall_add`:

| Env Var | Metadata Key | Source |
|---------|-------------|--------|
| `EDI_SESSION_ID` | `edi_session_id` | Session UUID from EDI launch |
| `EDI_AGENT_MODE` | `edi_agent_mode` | Current agent (coder, architect, etc.) |
| `EDI_GIT_BRANCH` | `edi_git_branch` | `git rev-parse --abbrev-ref HEAD` |
| `EDI_GIT_SHA` | `edi_git_sha` | `git rev-parse --short HEAD` |
| `EDI_PROJECT_PATH` | `edi_project_path` | Working directory path |
| `EDI_PROJECT_NAME` | `edi_project_name` | Base name of working directory |

This enriches every RECALL item with provenance, enabling queries like "what decisions were made on branch X" or "patterns captured in project Y".

---

## 13. Web UI

**Path:** `codex/internal/web/`

### Server (`server.go`)

Built with [Gin](https://github.com/gin-gonic/gin). Templates loaded from `web/templates/*`, static files from `web/static/`.

**Authentication:** If `CODEX_API_KEY` env var is set, all routes require `Authorization: Bearer {key}` header. Returns 401 on mismatch.

### Routes

**Web (HTML):**

| Route | Handler | Purpose |
|-------|---------|---------|
| `GET /` | `handleIndex` | Landing page |
| `GET /search?q=...&type=...` | `handleSearch` | Search UI (limit 20) |
| `GET /item/:id` | `handleItem` | Item detail view |
| `GET /browse?type=...&scope=...&page=...` | `handleBrowse` | Browse items (limit 50, paginated) |

**API (JSON):**

| Route | Handler | Purpose |
|-------|---------|---------|
| `GET /api/search?q=...&type=...` | `handleAPISearch` | Search (limit 20, query max 10KB) |
| `GET /api/item/:id` | `handleAPIItem` | Get item |
| `POST /api/item` | `handleAPICreate` | Create item (content max 1MB) |
| `PUT /api/item/:id` | `handleAPIUpdate` | Update item |
| `DELETE /api/item/:id` | `handleAPIDelete` | Delete item |

API responses follow the pattern `{"success": bool, "data": ..., "error": "..."}`.

---

## 14. CLI

**Path:** `codex/cmd/codex-cli/`

Built with [Cobra](https://github.com/spf13/cobra). Binary name: `codex-cli`.

### Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `index [path]` | Index files or directories | `--scope`, `--type`, `--language` |
| `search [query]` | Search the knowledge base | `--limit`, `--type`, `--scope`, `--json` |
| `migrate` | Migrate from RECALL v0 to Codex v1 | `--v0-db` (path to v0 SQLite) |
| `status` | Show system stats (item counts by type) | `--json` |
| `serve` | Start MCP or web server | `--mcp`, `--web`, `--addr` |

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ANTHROPIC_API_KEY` | (none) | Contextual chunking (not yet functional) |
| `LOCAL_EMBEDDING_URL` | `http://localhost:11434/api/embed` | Ollama endpoint |
| `LOCAL_EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model |
| `CODEX_MODELS_PATH` | `./models` | Reranking model directory (not yet functional) |
| `CODEX_METADATA_DB` | `~/.edi/codex.db` | SQLite database path |
| `CODEX_API_KEY` | (none) | Web server Bearer token auth |
| `CODEX_WEB_ADDR` | `:8080` | Web server listen address |
| `EDI_SESSION_ID` | `unknown` | MCP session ID (set by EDI) |

---

## 15. Entry Points

### `recall-mcp` (`cmd/recall-mcp/main.go`)

The MCP server process. Intended to be launched by an AI agent framework (EDI) and communicate over stdin/stdout.

```
Startup:
1. Parse env vars for config
2. core.NewSearchEngine(ctx, config)
3. mcp.NewServer(engine, sessionID)
4. server.Run(ctx)  -- blocks, reads stdin, writes stdout
5. Shutdown on SIGINT/SIGTERM
```

Default DB path: `~/.edi/codex.db`

### `codex-web` (`cmd/codex-web/main.go`)

The web UI server. Standalone HTTP server.

```
Startup:
1. Parse env vars for config
2. core.NewSearchEngine(ctx, config)
3. web.NewServer(engine, opts...)  -- optional API key auth
4. server.Run(addr)  -- blocks on HTTP listener
5. Shutdown on SIGINT/SIGTERM
```

### `codex-cli` (`cmd/codex-cli/main.go`)

Admin CLI for indexing, searching, migration, and server management. Each subcommand creates its own `SearchEngine` instance, executes, and exits.

### `codex-testgen` (`cmd/codex-testgen/main.go`)

Serves the PayFlow evaluation test collection as a REST API on `:8088`. Endpoints: `/`, `/documents`, `/documents/:id`, `/queries`, `/queries/:id`, `/export`.

---

## 16. Evaluation

**Path:** `codex/eval/`

### Harness (`harness.go`)

`EvalHarness` runs end-to-end evaluation against a real SearchEngine with a temp SQLite database. The full pipeline has 8 phases:

1. **Boot** -- create MCP client, perform initialize handshake
2. **Verify Protocol** -- list tools, confirm all 5 present
3. **Index Collection** -- add all PayFlow test documents via `recall_add`
4. **Verify Indexed** -- retrieve each document via `recall_get`
5. **Run Retrieval** -- search all queries, compute metrics
6. **Test Feedback** -- verify `recall_feedback` works
7. **Test Flight Recorder** -- verify `flight_recorder_log` works
8. **Test Audit Trail** -- verify `recall_search` auto-logs retrieval_query entries

### Metrics (`metrics.go`)

All metrics operate on ordered lists of document IDs:

| Metric | Function | Description |
|--------|----------|-------------|
| Precision@K | `PrecisionAtK(retrieved, relevant, k)` | Fraction of top-K results that are relevant |
| Recall@K | `RecallAtK(retrieved, relevant, k)` | Fraction of relevant items found in top-K |
| MRR | `MRR(retrieved, relevant)` | 1/rank of first relevant result |
| NDCG@K | `NDCG(retrieved, relevant, k)` | Normalized DCG. Relevance scores assigned by position in ground truth list (first = highest) |

### LLM Judge (`judge.go`)

`JudgeHarness` wraps the eval harness and adds LLM-as-judge evaluation using Claude Sonnet (`claude-sonnet-4-20250514`) via the Anthropic Messages API.

For each query:
1. Search via MCP
2. Build a numbered result list prompt
3. Send to Claude with a retrieval-judge skill prompt
4. Parse JSON response: `{"relevant_results": [1, 3], "reasoning": "..."}`
5. Compute judge precision, recall, F1, filtering rate

### Retrieval-Judge Skill (In-Session Quality Layer)

**Key file:** `edi/internal/assets/skills/retrieval-judge/SKILL.md`

The retrieval-judge skill is loaded into every agent's context as a mandatory post-search behavior. It forms the **production layer** of a two-layer quality pipeline:

**Layer 1 — Production (in-session):**

After every `recall_search`, the agent must:

1. **Evaluate each result** — check title match, content relevance, and applicability to the current task
2. **Log a judgment** — call `flight_recorder_log` with type `retrieval_judgment`, including:
   - `kept`: list of result IDs that passed evaluation
   - `dropped`: list of result IDs that were filtered out
   - Per-result reasoning
3. **Show a summary line** — `RECALL: X/Y results kept for '{query}'`

The skill also specifies query construction best practices (be specific with context, avoid single-word queries) and anti-patterns (don't trust rank order blindly, don't use results just because they appeared).

**Layer 2 — Evaluation (offline):**

The `JudgeHarness` in `codex/eval/judge.go` uses Claude Sonnet via the Anthropic Messages API to judge retrieval quality offline:

- For each test query, it builds a **numbered result list** with: title, type, score, and a 300-character content snippet
- The judge prompt asks which results are relevant, returning JSON: `{"relevant_results": [1, 3], "reasoning": "..."}`
- Metrics computed: **judge precision** (fraction of results judge keeps that are truly relevant), **recall** (fraction of relevant items the judge kept), **F1**, **filtering rate** (fraction of results dropped), **improvement** over raw precision@5

### `_judge_reminder` Field

The Codex MCP search handler (`codex/internal/mcp/tools.go`) injects a `_judge_reminder` field into every `recall_search` response. This reminder tells the agent to apply the retrieval-judge skill before using results. It acts as a nudge ensuring agents don't skip the evaluation step.

### Audit Trail

The retrieval quality pipeline produces two flight recorder entry types:

| Entry Type | Logged By | Contents |
|-----------|-----------|----------|
| `retrieval_query` | MCP server (automatic) | Query, filters, scored result list |
| `retrieval_judgment` | Agent (skill responsibility) | Kept/dropped lists, per-result reasoning, summary |

Together these entries form a complete audit trail from query to judgment, enabling offline analysis of retrieval quality in production sessions.

### Test Data (`testdata_payflow.go`)

The PayFlow scenario: a realistic set of knowledge items about a payment processing system. Includes documents covering API design, error handling, idempotency, webhook patterns, and more. Queries are categorized as `semantic`, `keyword`, or `hybrid-advantage` to test different retrieval strengths.

### Report (`report.go`)

`FullEvalReport` captures:
- MCP protocol verification
- Document index/verify counts
- Feedback and flight recorder status
- Audit trail results
- Per-pipeline retrieval quality metrics
- Per-category NDCG breakdown

---

## 17. Data Flow Diagrams

### Search Flow

```
User Query: "idempotency key for payment creation"
  |
  v
SearchEngine.Search(ctx, SearchRequest{Query: "...", Limit: 10})
  |
  +-- [1] embedder.EmbedQuery(ctx, "idempotency key for payment creation")
  |       -> Ollama POST /api/embed
  |       -> "search_query: idempotency key for payment creation"
  |       -> []float32 (768 dims)
  |
  +-- [2] vecStore.Search(ctx, queryVec, 20)
  |       -> brute-force dot product over in-memory vectors
  |       -> top-20 ScoredResult{ID, Score}
  |
  +-- [3] keywords.KeywordSearch("idempotency key for payment creation", 20)
  |       -> FTS5 MATCH '"idempotency key for payment creation"'
  |       -> top-20 KeywordResult{ID, Score (BM25)}
  |
  +-- [4] reciprocalRankFusion(vectorResults, keywordResults, 60)
  |       -> merged & ranked by RRF score
  |
  +-- [5] Hydrate metadata (GetItem for vector-only results)
  +-- [6] Filter by type/scope (if requested)
  +-- [7] Rerank (skipped -- reranker not available)
  +-- [8] Score threshold cutoff
  +-- [9] Limit to 10
  |
  v
[]SearchResult (up to 10 results with scores)
```

### Indexing Flow

```
IndexRequest{Content: "func CreatePayment(...) {...}", Type: "code", FilePath: "api/handler.go"}
  |
  v
Indexer.IndexFile(ctx, req)
  |
  +-- detectContentType -> TypeCode
  |
  +-- indexCode(ctx, req)
       |
       +-- DetectLanguage("api/handler.go") -> "go"
       |
       +-- astChunker.ChunkFile(content, "go", "api/handler.go")
       |       -> Tree-sitter parse
       |       -> extract functions, methods, types
       |       -> []CodeChunk
       |
       +-- For each chunk:
            |
            +-- embedder.EmbedDocument(ctx, chunk.Content)
            |       -> "search_document: func CreatePayment..."
            |       -> []float32 (768 dims)
            |
            +-- metaStore.SaveItem(item)
            |       -> INSERT INTO items (triggers FTS5 sync)
            |
            +-- vectorStore.Upsert(ctx, itemID, vec)
                    -> normalize -> BLOB -> INSERT INTO vectors
                    -> update in-memory map
```

### End-to-End Flow (EDI + Codex)

```
User types query in Claude Code session
  |
  v
Claude Code decides to search knowledge base
  |
  | (MCP tool call via stdin to recall-mcp subprocess)
  v
recall-mcp process receives JSON-RPC: tools/call recall_search
  |
  v
Codex SearchEngine.Search()
  |
  +-- Embed query via Ollama (nomic-embed-text, 768-dim)
  +-- Vector KNN over in-memory SQLite BLOBs
  +-- FTS5 BM25 keyword search
  +-- RRF fusion (k=60)
  +-- Hydrate + filter + limit
  |
  v
Results returned via MCP stdout JSON-RPC
  |
  v
Claude Code incorporates knowledge into response
```

### MCP Interaction Flow

```
AI Agent (e.g., Claude via EDI)
  |
  | stdin: {"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
  v
recall-mcp process
  |
  | stdout: {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05",...}}
  v
Agent sends:
  | {"jsonrpc":"2.0","id":2,"method":"tools/call",
  |  "params":{"name":"recall_search","arguments":{"query":"payment error handling"}}}
  v
Server:
  1. Parse JSON-RPC request
  2. Dispatch to ToolHandler.handleSearch
  3. engine.Search(ctx, SearchRequest{Query: "payment error handling", Limit: 10})
  4. Auto-log flight recorder entry (retrieval_query)
  5. Return results as JSON in text content
  |
  | stdout: {"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{...}"}]}}
  v
Agent receives search results, uses them in response
```

---

## 18. Operational Guide

### Prerequisites

- **Go 1.22+** with CGO enabled (required for `mattn/go-sqlite3`)
- **Ollama** running locally with `nomic-embed-text` model pulled:
  ```bash
  ollama pull nomic-embed-text
  ollama serve   # if not already running
  ```

### Building

```bash
cd codex
CGO_ENABLED=1 go build ./cmd/codex-cli
CGO_ENABLED=1 go build ./cmd/recall-mcp
CGO_ENABLED=1 go build ./cmd/codex-web
```

### Common Tasks

**Index a codebase:**
```bash
codex-cli index /path/to/project --scope project
```

**Search from CLI:**
```bash
codex-cli search "authentication pattern" --type pattern --limit 5
codex-cli search "error handling" --json
```

**Check system status:**
```bash
codex-cli status
```

**Run the MCP server (for AI agent integration):**
```bash
recall-mcp
# or with custom config:
CODEX_METADATA_DB=/path/to/db.sqlite LOCAL_EMBEDDING_URL=http://gpu-host:11434/api/embed recall-mcp
```

**Run the web UI:**
```bash
CODEX_API_KEY=my-secret-key codex-web
# Browse at http://localhost:8080
```

**Migrate from RECALL v0:**
```bash
codex-cli migrate --v0-db ~/.recall/recall.db
```

### Backup

The entire system state lives in a single SQLite file (default `~/.edi/codex.db`). To back up:

```bash
# Safe backup while running (uses SQLite backup API via .backup command)
sqlite3 ~/.edi/codex.db ".backup /path/to/backup.db"

# Or simply copy when no writes are happening
cp ~/.edi/codex.db /path/to/backup.db
```

The WAL file (`codex.db-wal`) and shared memory file (`codex.db-shm`) are transient and not needed for backup if you use the `.backup` command.

### Troubleshooting

**"failed to embed query" / connection refused to localhost:11434**
- Ollama is not running. Start it with `ollama serve`.
- Or the model is not pulled: `ollama pull nomic-embed-text`.
- If Ollama is on a different host, set `LOCAL_EMBEDDING_URL`.

**"database schema version X is newer than this binary supports"**
- The database was created by a newer version of Codex. Upgrade the binary.

**"vecstore load" errors on startup**
- Corrupted vector data in SQLite. The error message will identify the item ID.
- Recovery: delete the corrupted row from the `vectors` table, then re-index the affected content.

**FTS5 search returns no results but vector search works**
- The FTS5 index may be out of sync. Force a rebuild:
  ```sql
  sqlite3 ~/.edi/codex.db "INSERT INTO items_fts(items_fts) VALUES('rebuild')"
  ```

**Slow vector search (>100ms)**
- Check vector count: `sqlite3 ~/.edi/codex.db "SELECT COUNT(*) FROM vectors"`.
- If above ~50K, brute-force KNN will become noticeable. Consider implementing ANN or sharding.

**"contextual enrichment not implemented"**
- This is expected. The contextual chunker is a stub. It falls back to basic markdown chunking automatically. No action needed.

**"reranker not available"**
- This is expected. The reranker is a stub. Results are returned without reranking, which is the normal operating mode.

**Web server returns 401 Unauthorized**
- `CODEX_API_KEY` is set. Include `Authorization: Bearer {your-key}` header.

**CGO build errors / "sqlite3 requires cgo"**
- Ensure `CGO_ENABLED=1` is set.
- On macOS, Xcode Command Line Tools must be installed: `xcode-select --install`.
- On Linux, install `gcc` and `libc6-dev`.

### Memory Usage

All vectors are loaded into memory on startup. Each 768-dim float32 vector = 3KB. At 10K items, this is ~30MB. At 100K items, ~300MB. Plan capacity accordingly.

### Performance Characteristics

| Operation | Scale | Expected Latency |
|-----------|-------|-----------------|
| Vector search (KNN) | 10K items | < 1ms |
| Vector search (KNN) | 50K items | ~5-10ms |
| FTS5 keyword search | 10K items | < 1ms |
| Embedding (Ollama local) | single text | 10-50ms |
| Indexing a file | 10 chunks | ~500ms (dominated by embedding) |
| Full directory index | 100 files | ~1-5 min |
