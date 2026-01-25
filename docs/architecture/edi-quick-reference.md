# EDI Quick Reference

**Purpose**: Quick context loading for EDI specification sessions.  
**Full Details**: See `edi-specification-plan.md`

---

## What is EDI?

**Enhanced Development Intelligence** — An AI engineering assistant providing continuity, knowledge, and specialized behaviors on top of Claude Code.

**One-liner**: "Your AI chief of staff for engineering."

---

## Core Components

| Component | Purpose | Status |
|-----------|---------|--------|
| **RECALL** | MCP server for knowledge retrieval | To spec |
| **Agents** | Specialized modes (architect, coder, reviewer, incident) | To spec |
| **History** | Session summaries, decision memory | To spec |
| **Briefings** | Proactive context loading at session start | To spec |
| **Capture** | Prompted knowledge preservation at session end | To spec |
| **CLI** | Launches Claude Code with EDI config | To spec |

---

## Key Design Decisions

1. **Build ON Claude Code, not around it**
   - Use native Tasks (don't rebuild task tracking)
   - Use native Subagents
   - Use native Sessions

2. **RECALL as MCP server**
   - Claude decides when to query
   - No CAL orchestration layer needed

3. **History ≠ State**
   - Tasks handles state (what's in progress)
   - History captures reasoning (why decisions were made)

4. **Prompted capture, not auto-ingest**
   - Human curation keeps knowledge clean

5. **Valuable at any scale**
   - Solo devs benefit from continuity
   - Teams benefit from alignment
   - Enterprise benefits from governance

---

## Architecture (ASCII)

```
┌─────────────────────────────────────────────┐
│              Claude Code                     │
│   (Tasks, Subagents, Sessions = native)     │
└─────────────────────┬───────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│                   EDI                        │
│  RECALL │ Agents │ History │ Briefings      │
└─────────────────────────────────────────────┘
```

---

## Workspace Structure

```
~/.edi/                    # Global (framework)
├── agents/                # Agent definitions
├── skills/                # Skill library
├── commands/              # Slash commands
├── recall/                # MCP server + global knowledge
└── config.yaml

~/project/.edi/            # Project-specific
├── history/               # Session summaries
├── profile.md             # Project context
└── config.yaml
```

---

## Session Lifecycle

```
START → Briefing (Tasks + History + RECALL)
      ↓
WORK  → Claude Code + RECALL queries + Agent behaviors
      ↓
END   → Summary → Capture prompt → Save history
```

---

## Commands

| Command | Purpose |
|---------|---------|
| `edi` | Start session with briefing |
| `/plan` | Architect agent |
| `/build` | Coder agent |
| `/review` | Reviewer agent |
| `/incident` | Incident agent |
| `/end` | End session, capture |

---

## Specification Phases

| Phase | Focus | Status |
|-------|-------|--------|
| **1** | Workspace, Config, RECALL MCP | ⬜ Not started |
| **2** | History, Briefings, Capture | ⬜ Not started |
| **3** | Agent schema, Core agents | ⬜ Not started |
| **4** | CLI, Commands | ⬜ Not started |
| **5** | Multi-project, Integrations | ⬜ Not started |

---

## Don't Forget

- EDI delegates task tracking to Claude Code Tasks
- No `state/current.md` — Tasks handles this
- RECALL inherits Codex hybrid search design
- Agents are markdown with YAML frontmatter
- History captures *decisions*, not *status*

---

## Key Files

| File | Purpose |
|------|---------|
| `edi-specification-plan.md` | Full planning document |
| `aef-architecture-specification-v0.5.md` | Parent framework |
| `codex-architecture-deep-dive.md` | RECALL retrieval design |
