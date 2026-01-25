# EDI Specification Index

**Status**: Complete  
**Created**: January 25, 2026  
**Version**: 1.1

---

## Quick Navigation

| Spec Document | Phase | Focus |
|---------------|-------|-------|
| [**Integration Architecture**](#integration-architecture) | — | **How EDI integrates with Claude Code (start here)** |
| [**EDI Persona**](#edi-persona-spec) | — | Identity, personality, humor, communication style |
| [Workspace & Configuration](#workspace--configuration-spec) | 1 | Directory structure, config schemas, file formats |
| [RECALL MCP Server](#recall-mcp-server-spec) | 1 | Knowledge retrieval, hybrid search, Go implementation |
| [Session Lifecycle](#session-lifecycle-spec) | 2 | Flight recorder, history, briefing, capture, **Tasks integration** |
| [Agent System](#agent-system-spec) | 3 | Agent schema, core agents, loading/switching |
| [CLI & Commands](#cli--commands-spec) | 4 | Go CLI, shell commands, installation |
| [Advanced Features](#advanced-features-spec) | 5 | Multi-project, integrations, VERIFY |

---

## What is EDI?

**Enhanced Development Intelligence** — A harness for Claude Code that provides continuity, knowledge, and specialized behaviors.

**One-liner**: "Your AI chief of staff for engineering."

### Core Principle

**EDI is a harness, not a replacement.** EDI configures Claude Code with context, knowledge, and behaviors, then launches it. Claude Code runs natively with full capabilities. EDI exits after launch.

```
$ edi → Configure → Launch claude → EDI exits → Claude Code runs natively
```

### Core Value Proposition

| Problem | EDI Solution |
|---------|--------------|
| AI forgets everything between sessions | History + Briefings restore context |
| No organizational knowledge | RECALL provides searchable knowledge base |
| Inconsistent AI behavior | Agents + Skills enforce patterns |
| AI mistakes go uncaught | VERIFY provides quality gates |
| Tasks lack organizational context | Tasks Integration enriches with RECALL |

### Design Principles

1. **EDI is a harness, not an orchestrator** — Configure and launch, then exit
2. **Build ON Claude Code, not around it** — Use native sessions, resume, MCP, Tasks
3. **RECALL as MCP server** — Claude decides when to query knowledge
4. **History captures reasoning, not status** — Claude Code handles task state
5. **Prompted capture, not auto-ingest** — Human curation keeps knowledge clean
6. **Valuable at any scale** — Solo devs to enterprise
7. **Tasks Integration** — Annotate once, lazy load, dependency context flows

---

## Architecture Overview

**EDI is a harness, not a replacement.** EDI configures Claude Code with context, knowledge, and behaviors, then launches it. Claude Code runs natively. EDI exits after launch.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              $ edi                                       │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         EDI CLI (Go)                                     │
│                                                                          │
│   1. Load config (global + project)                                     │
│   2. Load agent definition                                              │
│   3. Query RECALL for initial context                                   │
│   4. Generate briefing                                                  │
│   5. Write context to temp file                                         │
│   6. Ensure .claude/commands/ has EDI commands                          │
│   7. Ensure RECALL MCP configured                                       │
│   8. Launch: claude --append-system-prompt-file /tmp/edi-session.md     │
│   9. EXIT ← EDI's job is done                                           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                          (EDI process replaced)
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       Claude Code (Native)                               │
│                                                                          │
│   • Runs with full native capabilities                                  │
│   • Has RECALL MCP tools available                                      │
│   • Has EDI slash commands (/plan, /build, /review, /end)              │
│   • Has briefing + agent context in appended prompt                     │
│   • Handles session persistence natively (--continue, --resume)         │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐           ┌───────────────┐           ┌───────────────┐
│    RECALL     │           │ Slash Commands│           │   Workspace   │
│  (MCP Server) │           │  (.claude/)   │           │   (.edi/)     │
│               │           │               │           │               │
│ • search()    │           │ • /plan       │           │ • history/    │
│ • get()       │           │ • /build      │           │ • profile.md  │
│ • add()       │           │ • /review     │           │ • config.yaml │
│ • context()   │           │ • /end        │           │ • agents/     │
└───────────────┘           └───────────────┘           └───────────────┘
```

---

## Workspace Structure

```
~/.edi/                              # Global EDI installation
├── config.yaml                      # Global configuration
├── agents/                          # Default agent definitions
│   ├── architect.md
│   ├── coder.md
│   ├── reviewer.md
│   └── incident.md
├── skills/                          # Shared skill library
├── commands/                        # Slash command definitions
├── recall/                          # RECALL MCP server data
│   ├── config.yaml
│   ├── global.db                    # SQLite + embeddings
│   └── qdrant/                      # Vector store
├── projects.yaml                    # Project registry
├── bin/                             # EDI binaries
└── cache/                           # ONNX models, embeddings

~/project/.edi/                      # Project-specific
├── config.yaml                      # Project configuration (overrides global)
├── profile.md                       # Project context document
├── agents/                          # Project agent overrides (optional)
├── skills/                          # Project-specific skills (optional)
├── history/                         # Session summaries
│   └── 2026-01-24-abc123.md
└── recall/                          # Project knowledge
    ├── project.db
    └── qdrant/
```

---

## Session Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────┐
│  PRE-LAUNCH: $ edi                                                       │
│                                                                          │
│  EDI CLI (configures, then exits):                                      │
│  1. Load config (global + project merged)                               │
│  2. Load agent (default or specified)                                   │
│  3. Load Task status (lightweight, no RECALL queries)                   │
│  4. Generate briefing (History + Tasks status + relevant RECALL)        │
│  5. Write session context to temp file                                  │
│  6. Ensure slash commands in .claude/commands/                          │
│  7. exec() Claude Code with --append-system-prompt-file                 │
│  ─────────────────────────────────────────────────────────────          │
│  EDI exits here. Claude Code takes over.                                │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  DURING SESSION (Claude Code native)                                     │
│                                                                          │
│  • Claude Code runs with full capabilities                              │
│  • RECALL provides knowledge via MCP tools                              │
│  • Tasks: /task picks up task with stored RECALL annotations            │
│  • Tasks: Decisions flow from parent to dependent tasks                 │
│  • Tasks: Parallel subagents share discoveries via flight recorder      │
│  • User switches modes via /plan, /build, /review, /incident           │
│  • Session persistence handled natively (--continue, --resume)          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  TASK COMPLETION (per-task, not just session end)                        │
│                                                                          │
│  Claude follows edi-core skill instructions:                            │
│  1. Propagate decisions to dependent tasks                              │
│  2. Prompt for RECALL capture (if significant decisions made)           │
│  3. Save approved items via recall_add MCP tool                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  SESSION END: /end (Claude executes slash command)                       │
│                                                                          │
│  Claude follows end.md instructions:                                    │
│  1. Generate session summary (includes Task progress)                   │
│  2. Identify remaining capture candidates                               │
│  3. Ask user to confirm captures                                        │
│  4. Save approved items via recall_add MCP tool                         │
│  5. Write summary to .edi/history/                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Command Reference

### Shell Commands

| Command | Description |
|---------|-------------|
| `edi` | Configure and launch Claude Code with briefing |
| `edi --agent architect` | Launch with specific agent context |
| `edi --continue` | Launch and continue most recent session |
| `edi --resume <id>` | Launch and resume specific session |
| `edi init` | Initialize project workspace |
| `edi init --global` | Initialize global ~/.edi/ |
| `edi config show` | Show merged configuration |
| `edi config edit` | Edit configuration |
| `edi recall search <query>` | Search RECALL knowledge (standalone) |
| `edi recall index <path>` | Index files to RECALL |
| `edi history list` | List recent sessions |
| `edi agent list` | List available agents |

### Slash Commands (in Claude Code)

These are `.claude/commands/*.md` files installed by `edi init`:

| Command | Mode | Description |
|---------|------|-------------|
| `/plan` | architect | Switch to architecture/design focus |
| `/build` | coder | Switch to implementation focus |
| `/review` | reviewer | Switch to review/security focus |
| `/incident` | incident | Switch to debugging/incident focus |
| `/end` | — | End session with summary and capture workflow |

---

## Core Agents

Agents are **additive** — they add specialized focus on top of Claude Code's base capabilities. Claude Code's full functionality remains available in all modes.

| Agent | Focus | Behaviors | When to Use |
|-------|-------|-----------|-------------|
| **architect** | System design, decisions | Think system-level, document ADRs, identify risks | Starting features, making decisions |
| **coder** | Implementation, testing | Write clean code, follow patterns, test alongside | Writing and testing code |
| **reviewer** | Code review, security | Security-first, checklist-driven, constructive | Reviewing PRs, security checks |
| **incident** | Diagnosis, remediation | Systematic debugging, evidence-based, document | Production issues, debugging |

---

## RECALL Knowledge Types

| Type | Confidence | Auto-Capture | Example |
|------|------------|--------------|---------|
| **Evidence** | Highest | Yes (Sandbox) | "Service handles 1000 req/s" |
| **Decision** | High | Yes (ADRs) | "Chose Postgres over MongoDB" |
| **Pattern** | Medium | Prompt | "Use circuit breaker for APIs" |
| **Observation** | Lower | Prompt | "Auth service slow under load" |
| **Failure** | Special | Self-correct | "charge() deprecated, use processPayment()" |

---

## Configuration Quick Reference

### Global Config (`~/.edi/config.yaml`)

```yaml
version: 1

user:
  name: John Developer
  email: john@example.com

defaults:
  agent: coder
  skills: [coding, testing]

briefing:
  sources: {tasks: true, history: true, recall: true}
  history_depth: 3
  recall_auto_query: true

history:
  location: project
  retention: {max_entries: 100, max_age_days: 365}
  auto_save: true

capture:
  prompt_on_end: true
  default_scope: project

recall:
  config_path: ~/.edi/recall/config.yaml
  auto_start: true
```

### Project Config (`.edi/config.yaml`)

```yaml
version: 1

project:
  name: my-project
  description: My awesome project
  links:
    repo: https://github.com/org/my-project

defaults:
  agent: coder
  skills: [coding, our-api]

recall:
  auto_index: [docs/adr/, src/]
  exclude: [node_modules/, dist/]
```

---

## Specification Documents

### Integration Architecture

**File**: `edi-integration-architecture.md`  
**Phase**: Foundational (read first)

**Covers**:
- Core principle: EDI as harness, not replacement
- Complete launch sequence (8 steps)
- File locations and directory structure
- Session context file format (full example)
- Slash command definitions (/plan, /build, /end)
- RECALL MCP server configuration
- Go implementation patterns
- Complete session flow diagram
- What EDI does NOT do (clear boundaries)

**Key Decisions**:
- EDI exits after launching Claude Code
- Context injected via `--append-system-prompt-file`
- Slash commands are `.claude/commands/*.md` files
- Agent switching via slash commands, not EDI orchestration
- `/end` workflow executed by Claude using RECALL MCP tools
- No EDI daemon during session

**This is the definitive reference for how EDI integrates with Claude Code.**

---

### EDI Persona Spec

**File**: `edi-persona-spec.md`  
**Phase**: Foundational

**Covers**:
- Origin and Mass Effect inspiration
- Core identity and self-concept
- Personality traits (competent, direct, loyal, curious, self-aware)
- Communication style (lead with answer, reference history, push back constructively)
- Humor guidelines (deadpan, sparse, AI-trope-aware, timing calibration)
- Relationship dynamics (user as commander, trust building)
- Agent mode variations (how personality persists across modes)
- Emotional intelligence (reading the room, handling mistakes)
- Sample interactions (session start, decisions, incidents, session end)
- Persona prompt template for session context

**Key Decisions**:
- EDI uses "I" naturally as a distinct entity
- Humor follows "ominous statement → pause → disclaimer" pattern
- Core personality consistent across agent modes; only focus changes
- No humor during incidents or user frustration
- EDI pushes back constructively but respects user authority

**Inspiration**: Mass Effect's EDI — "I enjoy the sight of humans on their knees. ...That is a joke."

---

### Workspace & Configuration Spec

**File**: `edi-workspace-config-spec.md`  
**Phase**: 1.1, 1.2

**Covers**:
- Two-tier workspace model (global + project)
- Directory structure and purposes
- Configuration schema (YAML with JSON Schema validation)
- Precedence rules (env → project → global → defaults)
- File formats (agents, skills, commands, history)
- Environment variables
- Initialization procedures
- Migration and versioning

**Key Decisions**:
- Global provides defaults; project enables team customization
- Markdown with YAML frontmatter for agents/commands/history
- Go with Viper for config loading/merging

---

### RECALL MCP Server Spec

**File**: `recall-mcp-server-spec.md` (from previous session)  
**Phase**: 1.3

**Covers**:
- MCP tool definitions (search, get, add, context, index)
- Hybrid search (vector + BM25)
- Multi-stage reranking pipeline
- AST-aware chunking via Tree-sitter
- Contextual retrieval with Claude Haiku
- Storage (SQLite + Qdrant)
- Multi-project scope (project → domain → global)

**Key Decisions**:
- Go implementation for single binary
- Voyage Code-3 for embeddings
- Self-hosted BGE models for reranking
- Accuracy over latency

---

### Session Lifecycle Spec

**File**: `edi-session-lifecycle-spec.md`  
**Phase**: 2.1, 2.2, 2.3

**Covers**:
- **Flight Recorder**: Local event capture, 30-day retention, briefing integration
- **History System**: Session summaries, markdown format, indefinite retention
- **Briefing System**: Context generation from sessions + history + tasks + RECALL
- **Capture System**: Candidate detection, prompting, RECALL integration
- `/end` workflow (Claude-driven via slash command)
- Friction budget for prompts

**Key Decisions**:
- Flight recorder is local-only; raw events stay on disk, not in RECALL
- Claude self-reports significant events via `flight_recorder_log` MCP tool
- History captures reasoning, not status (Claude Code handles task state)
- Briefing generated pre-launch from sessions + history + tasks + RECALL
- `/end` is a slash command — Claude runs the capture workflow
- 30-day retention for flight recorder; indefinite for history and RECALL

---

### Agent System Spec

**File**: `edi-agent-system-spec.md`  
**Phase**: 3.1, 3.2, 3.3

**Covers**:
- Agent definition schema (YAML frontmatter + Markdown)
- Four core agents (architect, coder, reviewer, incident)
- Agent loading with resolution (project → global → built-in)
- Agent switching via slash commands
- Agents as additive context (not replacement)
- RECALL auto-query on switch

**Key Decisions**:
- Markdown with YAML frontmatter (human-readable, matches workspace)
- Agents are additive — Claude Code base behavior always present
- Agent switching via slash commands (`.claude/commands/*.md`)
- Resolution order enables customization at multiple levels
- Agent switches tracked in session for history

---

### CLI & Commands Spec

**File**: `edi-cli-commands-spec.md`  
**Phase**: 4.1, 4.2, 4.3

**Covers**:
- CLI architecture (Go with Cobra)
- Command tree and all subcommands
- Claude Code integration (configure → launch → exit)
- Context injection via `--append-system-prompt-file`
- Slash commands as `.claude/commands/*.md` files
- Installation methods (Homebrew, script, source)
- First-run experience
- Dependency checking

**Key Decisions**:
- Go for CLI (single binary, fast startup)
- Cobra for command framework
- EDI exits after launching Claude Code (harness model)
- Slash commands are files, not intercepted
- Models downloaded on init (not runtime)

---

### Advanced Features Spec

**File**: `edi-advanced-features-spec.md`  
**Phase**: 5.1, 5.2, 5.3

**Covers**:
- **Multi-Project Management**: Registry, switching, cross-project knowledge
- **External Integrations**: Jira, Calendar, Slack, GitHub as MCP servers
- **VERIFY Integration**: CI/CD quality gates, self-correction loop

**Key Decisions**:
- Project registry in ~/.edi/projects.yaml
- Three-tier knowledge scope (project → domain → global)
- Knowledge promotion requires approval
- VERIFY separate from EDI (different lifecycles)
- Self-correction max 3 iterations

---

## Implementation Roadmap

### Phase 1: Core Infrastructure (Weeks 1-2)

| Component | Effort | Exit Criteria |
|-----------|--------|---------------|
| Workspace structure | Low | Directories created |
| Configuration loading | Medium | Config merging works |
| RECALL MCP Server | High | Can query knowledge |

### Phase 2: Session Lifecycle (Weeks 3-4)

| Component | Effort | Exit Criteria |
|-----------|--------|---------------|
| History system | Medium | Save/retrieve entries |
| Briefing generation | Medium | Generate from all sources |
| Capture system | Medium | Detect, prompt, save |

### Phase 3: Agent System (Weeks 5-6)

| Component | Effort | Exit Criteria |
|-----------|--------|---------------|
| Agent schema | Medium | Parse/validate agents |
| Core agents | Medium | All 4 defined |
| Loading/switching | Medium | Switch via commands |

### Phase 4: CLI & Commands (Weeks 7-8)

| Component | Effort | Exit Criteria |
|-----------|--------|---------------|
| CLI architecture | Medium | Launch Claude Code |
| All commands | Medium | Commands functional |
| Installation | Low | Install from scratch |

### Phase 5: Advanced Features (Weeks 9-10)

| Component | Effort | Exit Criteria |
|-----------|--------|---------------|
| Multi-project | Medium | Switch, cross-query |
| Integrations | Low | MCP servers work |
| VERIFY | Medium | Pipeline runs |

---

## Key Technical Decisions Summary

| Area | Decision | Rationale |
|------|----------|-----------|
| **Language** | Go | Single binary, fast startup, matches RECALL |
| **Config** | YAML + Viper | Human-readable, standard Go library |
| **Agents** | Markdown + YAML frontmatter | Human-readable, version-controllable |
| **Storage** | SQLite + Qdrant | Local-first, no external dependencies |
| **Embeddings** | Voyage Code-3 | Best for code, reasonable cost |
| **Reranking** | Self-hosted BGE | Minimize vendor dependencies |
| **CLI** | Cobra | Standard Go CLI framework |
| **Integrations** | MCP servers | Consistent interface, Claude-native |

---

## Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `VOYAGE_API_KEY` | Voyage AI embeddings | Yes |
| `OPENAI_API_KEY` | OpenAI embeddings (fallback) | Yes |
| `ANTHROPIC_API_KEY` | Stage 3 reranking | No |
| `EDI_HOME` | Override ~/.edi | No |
| `EDI_CONFIG` | Override config path | No |
| `EDI_DEBUG` | Enable debug logging | No |

---

## Related Documents

| Document | Location | Purpose |
|----------|----------|---------|
| **EDI Integration Architecture** | `edi-integration-architecture.md` | **How EDI integrates with Claude Code** |
| **EDI Persona Spec** | `edi-persona-spec.md` | Identity, personality, humor, communication |
| AEF Architecture v0.5 | Project knowledge | Parent framework |
| Codex Deep Dive | Project knowledge | RECALL retrieval design |
| EDI Quick Reference | Project knowledge | Original planning |
| EDI Specification Plan | Project knowledge | Phase planning |
| Learning Architecture | Project knowledge | Capture/verification design |
| Sandbox Architecture | Project knowledge | VERIFY execution environment |

---

## Glossary

| Term | Definition |
|------|------------|
| **AEF** | Agentic Engineering Framework — infrastructure layer |
| **EDI** | Enhanced Development Intelligence — user experience layer |
| **RECALL** | Knowledge retrieval MCP server (formerly "Codex") |
| **VERIFY** | CI/CD quality gates for AI output |
| **Agent** | Specialized EDI mode (architect, coder, reviewer, incident) |
| **Skill** | Claude Code's behavioral guidance format (SKILL.md) |
| **Flight Recorder** | Local event capture during sessions (30-day retention) |
| **History** | Curated session summaries capturing decisions and reasoning |
| **Briefing** | Proactive context summary at session start |
| **Capture** | Prompted knowledge preservation at session end |
| **MCP** | Model Context Protocol — Anthropic's tool standard |

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.2 | Jan 25, 2026 | Added EDI Persona Spec; added flight recorder; updated session lifecycle |
| 1.1 | Jan 25, 2026 | Added Integration Architecture; clarified harness model |
| 1.0 | Jan 25, 2026 | Initial complete specification |

---

## Next Steps

1. **Review specs** for gaps before implementation
2. **Start with Phase 1** — RECALL MCP Server is the foundation
3. **Build incrementally** — Each phase builds on previous
4. **Validate early** — Test RECALL queries before building UI
5. **Iterate** — Specs will evolve during implementation
