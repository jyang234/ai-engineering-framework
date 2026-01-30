# EDI - Enhanced Development Intelligence

EDI wraps Claude Code with agents, knowledge, briefings, and session continuity. One command to start a session that knows your project.

## What EDI Does

### Agents

Switch modes mid-session. `/plan` for architecture, `/build` for code, `/review` for quality, `/incident` for debugging. Each agent has its own system prompt, priorities, and tools.

### RECALL

Knowledge tools available in every session. Search patterns, log decisions, capture failures. Two backends: v0 (keyword-only via FTS5) or [Codex](../codex/README.md) (hybrid semantic + keyword).

| Tool | What it does |
|------|-------------|
| `recall_search` | Find knowledge by query |
| `recall_get` | Retrieve a specific item |
| `recall_add` | Capture a pattern, decision, or failure |
| `recall_feedback` | Mark results as useful or not |
| `flight_recorder_log` | Log session events |

### Briefings

Every session starts with context: project profile, recent history, open tasks. No cold starts. The briefing is generated from `.edi/profile.md`, session history, and task state.

### History

Sessions save summaries on `/end`. Next session picks up context from previous sessions. History lives in `.edi/history/`.

## Getting Started

### Requirements

- Go 1.22+
- Claude Code CLI installed and in PATH

### Install

```bash
cd edi
make build
make install  # Installs to ~/.local/bin/
```

### Initialize and Run

```bash
# Global init (once per machine)
edi init --global

# Project init
cd your-project && edi init

# Edit project profile
$EDITOR .edi/profile.md

# Start session
edi
```

## Slash Commands

| Command | Description |
|---------|-------------|
| `/plan` | Switch to architect mode |
| `/build` | Switch to coder mode |
| `/review` | Switch to reviewer mode |
| `/incident` | Switch to incident mode |
| `/task` | Manage tasks with RECALL context |
| `/end` | End session and save history |

## Configuration

Global config at `~/.edi/config.yaml`, project config at `.edi/config.yaml`. Project overrides global (arrays replace, not merge).

```yaml
version: "1"
agent: coder

recall:
  enabled: true
  backend: codex  # or "v0" for keyword-only

briefing:
  include_history: true
  history_entries: 3
  include_tasks: true
  include_profile: true
```

## Directory Structure

```
~/.edi/                    .edi/ (project)
├── agents/                ├── config.yaml
├── commands/              ├── profile.md
├── skills/                ├── history/
├── recall/                ├── tasks/
├── cache/                 └── recall/
└── config.yaml
```

## Links

- [AEF Overview](../README.md) — the big picture
- [Codex](../codex/README.md) — the knowledge engine
- [EDI + Codex Deep-Dive](../docs/edi-codex-deep-dive.md) — full system architecture

## License

MIT
