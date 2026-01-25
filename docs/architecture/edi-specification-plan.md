# EDI (Enhanced Development Intelligence) — Specification Plan

**Status**: Planning  
**Created**: January 24, 2026  
**Last Updated**: January 24, 2026  
**Version**: 0.1

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Context & Background](#2-context--background)
3. [Design Decisions (Established)](#3-design-decisions-established)
4. [Architecture Overview](#4-architecture-overview)
5. [Component Inventory](#5-component-inventory)
6. [Specification Status](#6-specification-status)
7. [Specification Plan](#7-specification-plan)
8. [Open Questions](#8-open-questions)
9. [Glossary](#9-glossary)
10. [References](#10-references)

---

## 1. Executive Summary

### What is EDI?

**EDI (Enhanced Development Intelligence)** is an AI engineering assistant that provides continuity, knowledge, and specialized behaviors on top of Claude Code's native capabilities. 

Named after the AI character from Mass Effect who evolves from a ship's VI into a trusted crew member, EDI serves as an engineer's "personal chief of staff" — maintaining context across sessions, surfacing relevant organizational knowledge, and adapting its behavior to different engineering tasks.

### Core Value Proposition

| Problem | EDI Solution |
|---------|--------------|
| Claude Code starts fresh every session | **History** — Session continuity and decision memory |
| No organizational/personal knowledge base | **RECALL** — MCP server for knowledge retrieval |
| Generic behavior for all tasks | **Agents** — Specialized modes (architect, coder, reviewer) |
| Context rebuilding is manual | **Briefings** — Proactive context loading |
| Knowledge decays without capture | **Capture** — Prompted knowledge preservation |

### Target Users

| Scale | Primary Value |
|-------|---------------|
| **Solo developer** | Personal continuity, cross-project knowledge, decision memory |
| **Small team (2-10)** | Above + team alignment, shared knowledge |
| **Enterprise (10+)** | Above + governance, compliance, audit trails |

### Relationship to Claude Code

EDI is a **value layer on top of Claude Code**, not a replacement:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Claude Code                                    │
│   Native: Tasks (dependencies, cross-session), Subagents, Sessions      │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│                              EDI                                         │
│   Adds: RECALL (knowledge), Agents (specialization), History,           │
│         Briefings, Capture                                               │
└─────────────────────────────────────────────────────────────────────────┘
```

**Principle**: Use Claude Code's native primitives (Tasks, Subagents, Sessions). Don't rebuild what Claude Code provides.

---

## 2. Context & Background

### Origin

EDI emerged from the AEF (Agentic Engineering Framework) project after Claude Code's Tasks feature announcement (January 22, 2026). The Tasks feature provides native support for:

- Task dependencies
- Cross-session collaboration via `CLAUDE_CODE_TASK_LIST_ID`
- Broadcast updates to all sessions on the same Task List
- File-based persistence in `~/.claude/tasks/`

This significantly overlapped with AEF's planned "Claude Harness" component, prompting a redesign. The result:

- **AEF** narrowed to: RECALL (knowledge retrieval), VERIFY (CI/CD quality gates), Org DNA (as Skills)
- **EDI** emerged as: The user-facing assistant layer that integrates RECALL with continuity features

### Relationship to AEF Components

| AEF Component | EDI Relationship |
|---------------|------------------|
| **RECALL (Codex)** | EDI uses RECALL as its knowledge layer (MCP server) |
| **VERIFY** | Separate; CI/CD integration, not part of EDI core |
| **Org DNA** | Implemented as Skills that EDI loads |
| **CAL** | Absorbed — Claude decides when to query RECALL |
| **Claude Harness** | Replaced by EDI + Claude Code native primitives |

### Inspirations

| Project | Influence on EDI |
|---------|------------------|
| **MARVIN** (SterlingChin) | Session persistence, workspace separation, slash commands, daily briefings |
| **bb-chiefofstaff** (Barbara Bermes) | Agents as markdown, MCP integration, specialized workflows |
| **Claude Code Skills** | Behavioral customization via SKILL.md format |
| **Claude Code Tasks** | Native task tracking (EDI defers to this) |

---

## 3. Design Decisions (Established)

### Architectural Decisions

| Decision | Rationale | Date |
|----------|-----------|------|
| **Build on Claude Code, don't replace** | Tasks, Subagents, Sessions are native; don't rebuild | Jan 2026 |
| **RECALL as MCP server** | Native integration, Claude decides when to query | Jan 2026 |
| **Agents as markdown definitions** | Declarative, easy to customize, version-controllable | Jan 2026 |
| **History captures decisions, not state** | Tasks handles state; History captures reasoning | Jan 2026 |
| **Workspace separation** | Global EDI (`~/.edi/`) vs project (`.edi/`); updatable framework | Jan 2026 |
| **Prompted capture, not auto-ingest** | Human curation keeps knowledge base clean | Jan 2026 |
| **EDI delegates task tracking to Tasks** | No custom `state/current.md`; use `CLAUDE_CODE_TASK_LIST_ID` | Jan 2026 |

### Value Decisions

| Decision | Rationale | Date |
|----------|-----------|------|
| **Continuity is valuable at any scale** | Solo devs lose context too; not just enterprise value | Jan 2026 |
| **Cross-project knowledge matters** | Solo devs have multiple projects; patterns should transfer | Jan 2026 |
| **Prompted discipline beats manual docs** | Capture prompts create moments of friction that encourage documentation | Jan 2026 |

### Naming

| Name | Meaning | Source |
|------|---------|--------|
| **EDI** | Enhanced Development Intelligence | Mass Effect AI character |
| **RECALL** | Knowledge retrieval layer | Descriptive (was "Codex" in AEF) |

---

## 4. Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              EDI CLI                                     │
│   Commands: edi, /plan, /build, /review, /incident, /end                │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           Claude Code                                    │
│   Native: Tasks, Subagents, Sessions                                    │
│   Loaded: Skills (via --skills), MCP Servers (RECALL)                   │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐           ┌───────────────┐           ┌───────────────┐
│    RECALL     │           │    Agents     │           │   Workspace   │
│   (MCP Server)│           │  (Markdown)   │           │   (Files)     │
│               │           │               │           │               │
│ • search()    │           │ • architect   │           │ • history/    │
│ • get_doc()   │           │ • coder       │           │ • profile.md  │
│ • add_doc()   │           │ • reviewer    │           │ • config      │
│ • list_adrs() │           │ • incident    │           │               │
└───────────────┘           └───────────────┘           └───────────────┘
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐           ┌───────────────┐           ┌───────────────┐
│  Knowledge    │           │    Skills     │           │   History     │
│    Store      │           │   Library     │           │   Store       │
│               │           │               │           │               │
│ SQLite +      │           │ org-standards │           │ Session       │
│ Embeddings    │           │ coding        │           │ summaries     │
│               │           │ security      │           │ per project   │
└───────────────┘           └───────────────┘           └───────────────┘
```

### Workspace Structure

```
~/.edi/                              # Global EDI installation
├── agents/                          # Agent definitions
│   ├── architect.md
│   ├── coder.md
│   ├── reviewer.md
│   └── incident.md
├── skills/                          # Skill library
│   ├── org-standards/
│   ├── coding/
│   └── security/
├── commands/                        # Slash command definitions
│   ├── edi.md
│   ├── plan.md
│   ├── build.md
│   └── end.md
├── recall/                          # RECALL MCP server
│   ├── server.py
│   └── knowledge/                   # Global knowledge store
└── config.yaml                      # Global configuration

~/projects/my-project/
├── .edi/                            # Project-specific EDI data
│   ├── history/                     # Session summaries
│   │   ├── 2026-01-22.md
│   │   ├── 2026-01-23.md
│   │   └── 2026-01-24.md
│   ├── agents/                      # Project agent overrides (optional)
│   ├── profile.md                   # Project context
│   └── config.yaml                  # Project configuration
├── .claude/
│   └── tasks/                       # Claude Code native (don't touch)
└── ... (project files)
```

### Session Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         1. SESSION START                                 │
│                                                                          │
│   $ cd ~/projects/my-project && edi                                     │
│                                                                          │
│   EDI:                                                                   │
│   1. Sets CLAUDE_CODE_TASK_LIST_ID=my-project                           │
│   2. Reads Claude Code Tasks for current state                          │
│   3. Reads .edi/history/ for recent decisions                           │
│   4. Queries RECALL for relevant context                                │
│   5. Generates briefing                                                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         2. DURING SESSION                                │
│                                                                          │
│   • Claude Code handles task state (native Tasks)                       │
│   • EDI provides knowledge (RECALL queries)                             │
│   • Agent behaviors guide Claude (Skills)                               │
│   • User can switch agents (/plan, /build, /review)                     │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         3. SESSION END                                   │
│                                                                          │
│   $ /end                                                                 │
│                                                                          │
│   EDI:                                                                   │
│   1. Summarizes session (what happened, decisions made)                 │
│   2. Identifies significant items for capture                           │
│   3. Prompts: "Save to RECALL? [Yes] [Edit] [Skip]"                     │
│   4. Saves session summary to .edi/history/                             │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Component Inventory

### Core Components

| Component | Type | Purpose |
|-----------|------|---------|
| **RECALL** | MCP Server | Knowledge retrieval (search, get, add) |
| **Agents** | Markdown files | Specialized system prompts + skill loadouts |
| **History** | File storage | Session summaries, decision memory |
| **Briefings** | Generated output | Proactive context loading |
| **Capture** | Workflow | Prompted knowledge preservation |
| **CLI** | Shell launcher | Invokes Claude Code with EDI configuration |
| **Commands** | Slash commands | Quick actions (/edi, /plan, /end, etc.) |
| **Config** | YAML files | Global and project settings |

### RECALL MCP Server

| Tool | Purpose |
|------|---------|
| `recall.search(query)` | Semantic search across knowledge base |
| `recall.get_document(id)` | Retrieve full document by ID |
| `recall.list_adrs()` | List architecture decision records |
| `recall.get_context(file_path)` | Get context relevant to a specific file |
| `recall.add_document(content, metadata)` | Add new content to knowledge base |
| `recall.index(path)` | Index a file or directory |

### Agent Definitions

| Agent | Purpose | Primary Skills |
|-------|---------|----------------|
| **architect** | System design, cross-cutting decisions | system-design, adrs, cross-team |
| **coder** | Implementation, testing | coding-standards, testing, patterns |
| **reviewer** | Critical evaluation, finding issues | security, review-checklist |
| **incident** | Diagnosis, remediation, urgency | incident-response, runbooks |

### Commands

| Command | Purpose |
|---------|---------|
| `edi` | Start EDI session with briefing |
| `/plan` | Switch to architect agent |
| `/build` | Switch to coder agent |
| `/review` | Switch to reviewer agent |
| `/incident` | Switch to incident agent |
| `/end` | End session, save history, prompt capture |

---

## 6. Specification Status

### Fully Specified

| Component | Status | Notes |
|-----------|--------|-------|
| (none yet) | — | — |

### Partially Specified (Need Details)

| Component | What's Established | What's Missing |
|-----------|-------------------|----------------|
| **Workspace Structure** | Directory layout | File formats, exact paths |
| **Session Lifecycle** | High-level flow | Exact triggers, data flows |
| **Agent Definitions** | Purpose, skills per agent | Full schema, loading mechanism |
| **Commands** | Names, purposes | Arguments, implementation |

### Underspecified (Need Deep Dive)

| Component | Key Questions |
|-----------|---------------|
| **RECALL MCP Server** | Tool schemas, multi-project indexing, global vs project scope, index management |
| **History System** | Session summary schema, capture triggers, context loading, retention policy |
| **Briefing System** | Data sources, Tasks integration, output format, customization |
| **Capture System** | Triggers, significance detection, approval flow, capture types |
| **CLI Architecture** | How it invokes Claude Code, context injection, installation |
| **Configuration** | Global schema, project schema, profile format |
| **Multi-Project** | Project registry, cross-project knowledge, switching |

---

## 7. Specification Plan

### Phase 1: Core Infrastructure (Sessions 1-2)

| Spec | Description | Effort |
|------|-------------|--------|
| **1.1 Workspace Structure** | File layout, directory purposes, file formats | Low |
| **1.2 Configuration Schema** | Global config, project config, profile format | Medium |
| **1.3 RECALL MCP Server** | Tool definitions, storage, indexing, multi-project | High |

**Exit Criteria**: Can run RECALL MCP server and query knowledge.

### Phase 2: Session Lifecycle (Sessions 3-4)

| Spec | Description | Effort |
|------|-------------|--------|
| **2.1 History System** | Session summary schema, storage, loading, retention | Medium |
| **2.2 Briefing System** | Data sources, generation, Tasks integration | Medium |
| **2.3 Capture System** | Triggers, significance, approval, destination | Medium |

**Exit Criteria**: Can start session, get briefing, end session, capture decisions.

### Phase 3: Agent System (Sessions 5-6)

| Spec | Description | Effort |
|------|-------------|--------|
| **3.1 Agent Definition Schema** | Full YAML/markdown schema for agents | Medium |
| **3.2 Core Agent Specs** | architect, coder, reviewer, incident definitions | Medium |
| **3.3 Agent Loading & Switching** | How agents load, switch, persist | Medium |

**Exit Criteria**: Can switch between agents within a session.

### Phase 4: CLI & Commands (Session 7)

| Spec | Description | Effort |
|------|-------------|--------|
| **4.1 CLI Architecture** | How EDI launches Claude Code | Medium |
| **4.2 Command Specifications** | Full spec for each command | Medium |
| **4.3 Installation & Setup** | First-run experience, dependencies | Low |

**Exit Criteria**: Can install EDI and run all commands.

### Phase 5: Advanced Features (Sessions 8-9)

| Spec | Description | Effort |
|------|-------------|--------|
| **5.1 Multi-Project Management** | Project registry, switching, cross-project knowledge | Medium |
| **5.2 External Integrations** | Calendar, Jira, etc. (optional) | Low |
| **5.3 VERIFY Integration** | CI/CD hooks (may remain separate from EDI) | Medium |

**Exit Criteria**: Full EDI specification complete.

### Validation Approach

After Phase 2, spec a thin vertical slice:

**`/edi` Command End-to-End**:
- Spec fully: workspace, config, RECALL query, history read, briefing generation
- Validates design before speccing remaining agents/commands

---

## 8. Open Questions

### Architecture

| Question | Options | Notes |
|----------|---------|-------|
| Should RECALL be embedded in EDI or standalone? | Embedded, Standalone | Standalone allows use without EDI |
| How does EDI read Claude Code Tasks? | CLI parsing, File reading, API | Need to investigate Claude Code internals |
| Should agents be Skills or separate? | Skills, Custom format | Skills = portable; Custom = more control |

### Scope

| Question | Options | Notes |
|----------|---------|-------|
| Is VERIFY part of EDI or separate? | Part of EDI, Separate tool | Leaning separate; different lifecycle |
| How much of Codex design carries over to RECALL? | All, Most, Some | Hybrid search yes; federation later |
| Should EDI work without RECALL? | Yes (degraded), No (required) | Probably required for core value |

### Implementation

| Question | Options | Notes |
|----------|---------|-------|
| What language for RECALL MCP server? | Python, TypeScript | Python has better embedding libs |
| What language for CLI? | Shell, Python, Go | Shell simplest; Python for complexity |
| Local-first or cloud option? | Local only, Cloud optional | Local first; cloud later maybe |

---

## 9. Glossary

| Term | Definition |
|------|------------|
| **Agent** | Specialized configuration (system prompt + skills + RECALL strategy) for a type of work |
| **Briefing** | Proactive summary generated at session start |
| **Capture** | Process of preserving session learnings to RECALL |
| **EDI** | Enhanced Development Intelligence — the assistant layer |
| **History** | Stored session summaries capturing decisions and reasoning |
| **MCP** | Model Context Protocol — Anthropic's tool integration standard |
| **RECALL** | EDI's knowledge retrieval layer (MCP server) |
| **Session** | One interaction period with EDI/Claude Code |
| **Skill** | Claude Code's modular behavioral component (SKILL.md) |
| **Tasks** | Claude Code's native task tracking with dependencies |
| **Workspace** | File structure for EDI state (global or project) |

---

## 10. References

### Related Documents

| Document | Purpose |
|----------|---------|
| `aef-architecture-specification-v0.5.md` | Parent framework architecture |
| `codex-architecture-deep-dive.md` | RECALL retrieval engine design (from Codex) |
| `claude-harness-deep-dive.md` | Historical reference (v0.4 design, now absorbed) |

### External References

| Reference | URL | Relevance |
|-----------|-----|-----------|
| Claude Code Tasks announcement | (X/Twitter, Jan 22, 2026) | Native task tracking |
| Claude Code Skills | docs.anthropic.com | Skill format |
| MARVIN template | github.com/SterlingChin/marvin-template | Session persistence pattern |
| bb-chiefofstaff | github.com/bbinto/bb-chiefofstaff | Agent orchestration pattern |
| MCP Specification | modelcontextprotocol.io | MCP server implementation |

### Project Files

| File | Location |
|------|----------|
| This plan | `edi-specification-plan.md` |
| AEF Architecture | `aef-architecture-specification-v0.5.md` |
| Codex Deep Dive | `codex-architecture-deep-dive.md` |

---

## Appendix A: Session Context Template

When starting a new specification session, load this context:

```markdown
## EDI Specification Session

**Session Goal**: [Specify which component(s)]

**Previously Completed**:
- [List completed specs]

**Current Focus**:
- [Spec being worked on]

**Key Constraints**:
- EDI builds on Claude Code (Tasks, Subagents, Sessions)
- RECALL is MCP server, not embedded
- History captures decisions, not state (Tasks handles state)
- Prompted capture, not auto-ingest

**References**:
- edi-specification-plan.md (this document)
- [Relevant deep-dive docs]
```

---

## Appendix B: Decision Log

| Date | Decision | Rationale | Session |
|------|----------|-----------|---------|
| Jan 24, 2026 | EDI name adopted | Mass Effect reference; "personal chief of staff" framing | Initial planning |
| Jan 24, 2026 | Delegate task tracking to Claude Code Tasks | Native feature, don't rebuild | Initial planning |
| Jan 24, 2026 | History captures decisions, not state | Tasks handles state; History adds reasoning | Initial planning |
| Jan 24, 2026 | RECALL as MCP server | Native integration, Claude decides when to query | Initial planning |
| Jan 24, 2026 | Workspace separation (global + project) | Updatable framework, project-specific data | Initial planning |
| Jan 24, 2026 | EDI valuable at all scales | Continuity is scale-agnostic; solo devs benefit too | Initial planning |

---

## Appendix C: Change Log

| Version | Date | Changes |
|---------|------|---------|
| 0.1 | Jan 24, 2026 | Initial planning document |
