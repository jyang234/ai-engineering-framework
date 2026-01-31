# AEF Install Model

> **Implementation Status (January 31, 2026):** Broadly accurate. Installation paths and scoping model match implementation.

## Workspace-Level (Global) — `~/.edi/`

Installed once per machine via `edi init --global`. Shared across all projects.

| Component | Path | Description |
|-----------|------|-------------|
| Binaries | `~/.edi/bin/` | `edi`, `recall-mcp` |
| Agents | `~/.edi/agents/` | Agent markdown definitions |
| Commands | `~/.edi/commands/` | Slash command definitions |
| Skills | `~/.edi/skills/` | Skill definitions |
| Config | `~/.edi/config.yaml` | Global configuration (backend, defaults) |
| Knowledge DB | `~/.edi/codex.db` or `~/.edi/recall/global.db` | RECALL knowledge base |
| Models | `~/.edi/models/` | ONNX embedding models (Codex) |
| Cache | `~/.edi/cache/` | Cached data |
| Logs | `~/.edi/logs/` | EDI logs |

## Project-Level — `.edi/`

Initialized per project via `edi init`. Committed to version control (except history).

| Component | Path | Description |
|-----------|------|-------------|
| Profile | `.edi/profile.md` | Project description, architecture, conventions |
| Config | `.edi/config.yaml` | Project-specific overrides |
| History | `.edi/history/` | Session history (gitignored) |
| Tasks | `.edi/tasks/` | Task annotations |
| RECALL | `.edi/recall/` | Project-scoped recall data |

## RECALL Scoping Model

RECALL uses a single global database (`~/.edi/codex.db`) with project attribution via metadata:

- Every item added via `recall_add` is automatically tagged with `project_name` and `project_path` from the current session environment.
- `scope: "project"` items are tagged with project metadata; `scope: "global"` items are not.
- Search with `scope: "project"` filters to items matching the current project.
- Search with `scope: "global"` returns items without project restrictions.
- Search with `scope: "all"` (default) returns everything, ranked by relevance.

## Install Flow

```
edi init --global
  │
  ├─► Create ~/.edi/ directory structure
  ├─► Install agents, commands, skills
  ├─► Write default config
  ├─► If backend=codex:
  │     ├─► Check for recall-mcp binary
  │     ├─► Detect codex source directory
  │     ├─► Build and install binary
  │     └─► Check Ollama + embedding model
  └─► Done

edi init  (in project)
  │
  ├─► Create .edi/ directory structure
  ├─► Write project config + profile template
  └─► Done
```

## Diagnostics

Run `edi doctor` to verify installation health. It checks:

- Global directory structure
- Config files
- Backend binary availability
- Database files
- Ollama and embedding model
- Claude Code CLI
- Project-level initialization
