# AEF Component Registry

**Purpose**: Quick reference for what exists, what is planned, and where to find details.
**Updated**: January 25, 2026

---

## Components Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     AI ENGINEERING FRAMEWORK (AEF)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐                │
│   │     EDI      │   │    Codex     │   │   Learning   │                │
│   │   (v0 ✅)    │──►│    (v1 📋)   │──►│    (📋)      │                │
│   │ CLI harness  │   │ Hybrid search│   │ Knowledge QA │                │
│   └──────────────┘   └──────────────┘   └──────────────┘                │
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
- RECALL MCP server with SQLite FTS5 search
- 4 core agents (coder, architect, reviewer, incident)
- 7 subagents for specialized tasks
- 6 slash commands (/plan, /build, /review, /incident, /task, /end)
- Session briefings with history integration

**Known Gaps:**
- Persona spec not fully integrated into agent/skill files
- See `docs/implementation/edi-implementation-gaps-analysis.md`

---

### Codex v1 (RECALL Upgrade)

| Attribute | Value |
|-----------|-------|
| **Status** | 📋 Planned |
| **Prerequisites** | EDI Phase 2 complete |
| **Purpose** | Upgrade RECALL from FTS to production-grade hybrid retrieval |
| **Implementation Plan** | `docs/implementation/codex-v1-implementation-plan.md` |

**Planned Capabilities:**
- Qdrant for vector + BM25 search
- Voyage Code-3 embeddings for code
- text-embedding-3-large for docs
- Multi-stage reranking (BGE models)
- AST-aware chunking (Tree-sitter)
- Contextual retrieval (Claude Haiku)
- Web UI for browsing knowledge

**Expected Improvement:** Top-10 recall from ~60% to ~85%

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
EDI v0 (✅)
    │
    ├──► Codex v1 (📋)
    │        │
    │        └──► Learning Architecture (📋)
    │
    └──► Sandbox (📋) [independent]
```

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

---

## Quick Start

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
