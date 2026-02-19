# AEF Component Registry

> **Implementation Status (February 19, 2026):** Updated with Auto Memory alignment (merge-based co-ownership model).

**Purpose**: Quick reference for what exists, what is planned, and where to find details.
**Updated**: February 19, 2026

---

## Components Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     AI ENGINEERING FRAMEWORK (AEF)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                │
│   │     EDI      │   │    Codex     │   │   Learning   │                │
│   │   (v0 ✅)    │◄─►│   (v1 ✅)    │──►│    (📋)      │                │
│   │ CLI harness  │   │ Hybrid search│   │ Knowledge QA │                │
│   └──────┬───────┘   └──────────────┘   └──────────────┘                │
│          │                                                               │
│          ├──► Auto Memory (✅) ── Claude Code MEMORY.md integration     │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────┐                                                      │
│   │   Sandbox    │                                                      │
│   │    (📋)      │                                                      │
│   │ Disposable   │                                                      │
│   │ environments │                                                      │
│   └──────────────┘                                                      │
│                                                                          │
│   Legend: ✅ Implemented  📋 Planned  🚧 In Progress                    │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### EDI (Enhanced Development Intelligence)

| Attribute | Value |
|-----------|-------|
| **Status** | ✅ v0 Implemented |
| **Location** | `edi/` |
| **Purpose** | CLI harness for Claude Code with context, knowledge, and behaviors |
| **Implementation Plan** | `docs/implementation/edi-implementation-plan.md` |

**Capabilities:**
- CLI commands: init, launch (default), config, recall, history, agent
- RECALL backend selection: v0 (SQLite FTS) or Codex v1 (hybrid vector search)
- MCP server auto-configuration based on backend
- 4 core agents (coder, architect, reviewer, incident)
- 7 subagents for specialized tasks
- 6 slash commands (/plan, /build, /review, /incident, /task, /end)
- Session briefings with history integration

**Configuration:**
- `recall.backend: v0` - SQLite FTS5 (default, zero dependencies)
- `recall.backend: codex` - Hybrid vector search (requires Ollama)

**Known Gaps:**
- Persona spec not fully integrated into agent/skill files
- See `docs/implementation/edi-implementation-gaps-analysis.md`

---

### Codex v1 (RECALL Upgrade)

| Attribute | Value |
|-----------|-------|
| **Status** | ✅ v1 Implemented |
| **Location** | `codex/` |
| **Purpose** | Production-grade hybrid retrieval for RECALL |
| **Implementation Plan** | `docs/implementation/codex-v1-implementation-plan.md` |

**Capabilities (Implemented):**
- SQLite BLOB + brute-force KNN vector search with 2-way RRF fusion (vector + FTS5 keywords)
- nomic-embed-text embeddings via local Ollama (768-dim, all content types)
- AST-aware chunking (Tree-sitter)
- Web UI for browsing knowledge (with optional API key auth)
- MCP server (drop-in replacement for RECALL v0)
- Input size limits and schema versioning

**Planned (stubs present, not yet functional):**
- Multi-stage reranking (BGE models via ONNX/Hugot) — stub falls back to original ordering
- Contextual retrieval (Claude Haiku enrichment) — stub returns error, not implemented

**Requirements:**
- Ollama running locally with `nomic-embed-text` model (`ollama pull nomic-embed-text`)
- No API keys required for core functionality
- Optional: `ANTHROPIC_API_KEY` for contextual document enrichment

**Architecture Decisions:**
- `docs/architecture/codex-storage-architecture-decision.md` — SQLite BLOBs over Qdrant
- `docs/architecture/codex-embedding-model-decision.md` — Single local model over dual API models

**EDI Integration:**
- Set `recall.backend: codex` in `~/.edi/config.yaml`
- EDI automatically generates MCP config for Codex

---

### Auto Memory Integration

| Attribute | Value |
|-----------|-------|
| **Status** | ✅ Implemented |
| **Location** | `edi/internal/memory/` |
| **Purpose** | Bridge between EDI's RECALL system and Claude Code's built-in auto memory (MEMORY.md) |
| **Specification** | `docs/architecture/auto-memory-alignment-spec.md` |

**Capabilities:**
- Co-manages MEMORY.md via merge-based approach — EDI manages structured sections, Claude's autonomous writes and user additions are preserved
- Detects auto memory directory using git repository root (matching Claude Code's path convention)
- Seeds MEMORY.md from project profile during `edi init`
- Updates MEMORY.md on session launch with promoted RECALL items
- Captures session insights to MEMORY.md during `/end`
- Fixed slot budget (10 per type) prevents unbounded growth
- 195-line hard limit enforced (truncates from bottom, preserving EDI-managed content)
- EDI Observations section for in-session notes
- Preamble preservation — freeform content Claude writes before the first section header is kept

**Architecture:**
- L1 (MEMORY.md): Always-loaded cache of best RECALL content
- L2 (EDI Context): Session-specific agent/task/command instructions
- L3 (RECALL): Full searchable knowledge base via MCP

**Configuration:**
```yaml
memory:
  enabled: true
  update_on_launch: true
  update_on_end: true     # Reserved: /end always includes MEMORY.md step (prompt-driven)
```

---

### Learning Architecture

| Attribute | Value |
|-----------|-------|
| **Status** | 📋 Planned |
| **Prerequisites** | EDI Phase 2 + Codex v1 |
| **Purpose** | Knowledge capture, attribution, and quality controls |
| **Implementation Plan** | `docs/implementation/aef-learning-architecture-implementation-plan.md` |

**Planned Capabilities:**
- Typed knowledge (evidence, decision, pattern, observation, failure)
- Confidence tiers with decay
- Friction-budgeted capture (max 3 prompts/session)
- LLM judge for failure attribution
- Freshness scoring and re-verification

---

### Sandbox

| Attribute | Value |
|-----------|-------|
| **Status** | 📋 Planned |
| **Prerequisites** | None (can be developed in parallel) |
| **Purpose** | Disposable Docker environments for controlled experimentation |
| **Implementation Plan** | `docs/implementation/aef-sandbox-implementation-plan.md` |

**Planned Capabilities:**
- Experiment execution in disposable containers
- Fault injection (network, connection, application)
- Full telemetry via OpenTelemetry
- Assertion engine for verification
- Artifact lifecycle management

---

## Dependency Graph

```
EDI v0 (✅) ◄───► Codex v1 (✅)
    │                  │
    ├──► Auto Memory (✅) ◄── Promoted items from RECALL
    │                  │
    │                  └──► Learning Architecture (📋)
    │
    └──► Sandbox (📋) [independent]
```

EDI can use either RECALL v0 (built-in) or Codex v1 (external) as its knowledge backend. Auto Memory bridges RECALL with Claude Code's built-in MEMORY.md feature.

---

## Architecture Documents

| Document | Purpose |
|----------|---------|
| `docs/architecture/edi-specification-index.md` | EDI overview and navigation |
| `docs/architecture/edi-workspace-config-spec.md` | Directory structure, config schemas |
| `docs/architecture/recall-mcp-server-spec.md` | RECALL tools and storage |
| `docs/architecture/edi-session-lifecycle-spec.md` | Briefing, history, capture, tasks |
| `docs/architecture/edi-cli-commands-spec.md` | CLI and slash commands |
| `docs/architecture/edi-agent-system-spec.md` | Agent schema and core agents |
| `docs/architecture/edi-subagent-specification.md` | Subagent definitions |
| `docs/architecture/edi-persona-spec.md` | EDI identity and communication style |
| `docs/architecture/auto-memory-alignment-spec.md` | Auto memory alignment with Claude Code MEMORY.md |

---

## Quick Start

### EDI with RECALL v0 (default, zero dependencies)

```bash
# Build and install EDI
cd edi && make build && make install

# Initialize globally (once)
edi init --global

# Initialize in a project
cd your-project && edi init

# Start session
edi
```

### EDI with Codex v1 (hybrid vector search)

```bash
# Pull embedding model
ollama pull nomic-embed-text

# Build Codex
cd codex && make build
cp bin/recall-mcp ~/.edi/bin/

# Initialize with Codex backend
edi init --global --backend codex

# Start session (uses Codex automatically)
cd your-project && edi init && edi
```
