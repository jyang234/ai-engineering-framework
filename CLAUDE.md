# AI Engineering Framework (AEF)

## Architecture

AEF is a Go monorepo with two main modules:

- **`edi/`** — CLI harness that configures and launches Claude Code with agents, RECALL, and session briefings
- **`codex/`** — Backend library: hybrid search engine (vector + FTS5 + RRF), MCP server, eval infrastructure

EDI configures the session then replaces itself (`syscall.Exec`) with Claude Code. Claude Code runs natively with MCP tools for knowledge retrieval.

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| EDI CLI | `edi/cmd/edi/` | Entry point, Cobra commands |
| Agents | `edi/internal/assets/agents/` | Mode-specific system prompts (coder, architect, reviewer, incident) |
| Skills | `edi/internal/assets/skills/` | Behavioral instructions injected into system prompt (8 skills) |
| Commands | `edi/internal/assets/commands/` | Slash commands (`/end`, `/task`, `/plan`, `/build`, `/review`, `/incident`, `/ralph`, `/review-plan`, `/end-recovery`) |
| Subagents | `edi/internal/assets/subagents/` | Specialized sub-agent prompts (7 subagents) |
| Tasks | `edi/internal/tasks/` | Task management, manifest, annotations, git sync |
| RECALL MCP | `codex/internal/mcp/` | JSON-RPC server: recall_search, recall_add, recall_get, recall_feedback, flight_recorder_log |
| Search Engine | `codex/internal/core/` | Hybrid search: vector cosine similarity + FTS5 BM25 + RRF fusion |
| Eval | `codex/eval/` | Evaluation harnesses, judges, metrics |

### Data Flow

```
Session start:
  edi → loads profile, briefing, agents, skills
      → boots RECALL MCP server (codex)
      → exec's Claude Code with --mcp, --system-prompt, --allowedTools

During session:
  Claude Code → recall_search → codex search engine → SQLite (FTS5 + vectors)
             → flight_recorder_log → codex SQLite
             → /memories/ → session working memory (survives compaction)

At /end:
  Claude Code → recall_add → codex SQLite (curated items only)
             → status.md → human-readable project state
             → .edi/history/ → session log
```

## Conventions

### Go
- Go 1.22+, standard project layout
- `internal/` for private packages, embedded assets via `//go:embed`
- Build tag required for SQLite FTS5: `-tags "fts5"`
- Format with `gofumpt`, lint with `golangci-lint`
- Tests: `go test -tags "fts5" -v ./...`

### Agent/Skill Files
- Markdown with YAML frontmatter (`name`, `description`)
- Skills in `edi/internal/assets/skills/<name>/SKILL.md`
- Agents in `edi/internal/assets/agents/<name>.md`

### Configuration
- YAML format, project-level in `.edi/config.yaml`
- Project overrides global (`~/.edi/config.yaml`); arrays replace, not merge

## Build

```bash
cd edi && make build      # Build EDI binary + task-sync-hook
cd edi && make install    # Build + install to ~/.local/bin
cd edi && make sync       # Rebuild + sync assets (no config overwrite)
cd edi && make reinstall  # Clean rebuild + full reinitialize

cd codex && make build    # Build codex binaries (recall-mcp, codex-cli, codex-web)
```

## Key Decisions

1. **Go over Python** — Single binary, CGO for SQLite FTS5, official MCP SDK
2. **syscall.Exec for launch** — EDI replaces itself with Claude Code (not a subprocess)
3. **SQLite FTS5 for v0** — No external dependencies; vector search via Ollama embeddings
4. **Prompted capture** — Human curation at `/end` keeps knowledge clean
5. **MEMORY.md retired** — Replaced by CLAUDE.md (instructions), `/memories/` (session cache), codex DB (knowledge), status.md (project state)
6. **Align with Claude Code features** — Use memory tool, compaction, context editing rather than building parallel mechanisms

## Reference Documents

- `docs/edi-codex-deep-dive.md` — Full system architecture, data flows, and operational guide
- `docs/aef-components.md` — Component registry with status
- `docs/architecture/aef-evaluation-framework.md` — Eval framework spec (comprehensive)
- `docs/architecture/recall-mcp-server-spec.md` — RECALL specification
- `docs/architecture/edi-session-lifecycle-spec.md` — Session management
- `docs/architecture/edi-cli-commands-spec.md` — CLI commands
- `docs/implementation/edi-implementation-plan.md` — Implementation guide
