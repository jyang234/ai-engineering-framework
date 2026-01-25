# AI Engineering Framework (AEF)

## Overview

AEF is a comprehensive framework for AI-assisted software engineering. It provides tools, patterns, and infrastructure for building AI-powered development workflows.

The primary component is **EDI (Enhanced Development Intelligence)** - a Go CLI harness that configures Claude Code with context, knowledge, and specialized behaviors.

## Architecture

### Components

1. **EDI CLI** (`edi/`) - Go binary that launches Claude Code with:
   - Agent configurations (coder, architect, reviewer, incident)
   - RECALL MCP server for knowledge retrieval
   - Session briefings with history and context
   - Slash commands for mode switching

2. **RECALL** - Knowledge retrieval system:
   - SQLite FTS5 for v0 (implemented)
   - Future: Vector embeddings, hybrid search, Qdrant

3. **Documentation** (`docs/`) - Specifications and architecture docs

### Key Patterns

- EDI configures then exits; Claude Code runs natively
- Knowledge capture is prompted, not automatic
- Agents are markdown files with YAML frontmatter
- Tasks integrate with Claude Code's native task system

## Tech Stack

- **Go 1.22+** - EDI CLI and RECALL server
- **SQLite with FTS5** - Knowledge storage (v0)
- **Cobra/Viper** - CLI framework and configuration
- **MCP Protocol** - Claude Code integration via stdio JSON-RPC

## Conventions

### Go Code
- Standard Go project layout
- Packages in `internal/` for private, `pkg/` for public
- Embedded assets via `//go:embed`
- Build tags for SQLite FTS5: `-tags "fts5"`

### Agent/Command Files
- Markdown with YAML frontmatter
- Located in `~/.edi/agents/` (global) or `.edi/agents/` (project)
- Slash commands in `~/.edi/commands/`

### Configuration
- YAML format
- Global: `~/.edi/config.yaml`
- Project: `.edi/config.yaml`
- Project overrides global (arrays replace, not merge)

## Key Decisions

1. **Go over Python** - Single binary, official MCP SDK, CGO for SQLite FTS5
2. **syscall.Exec for launch** - EDI replaces itself with Claude Code
3. **SQLite FTS for v0** - Simple, no external dependencies
4. **Prompted capture** - Human curation keeps knowledge clean
5. **Agents as markdown** - Easy to customize and version control

## Current Status

### Implemented (v0)
- EDI CLI with init, launch, config, recall, history, agent commands
- RECALL MCP server with FTS5 search
- 4 core agents + 7 subagents
- 6 slash commands
- Briefing generation from profile/history
- Task annotations system

### Next Steps
- Test end-to-end with real Claude Code sessions
- Add more comprehensive tests
- Implement v1 features (vector search, web UI)
- Build additional AEF components

## Getting Started

```bash
# Build and install EDI
cd edi && make build && make install

# Initialize globally (once)
edi init --global

# Initialize in a project
cd your-project && edi init

# Edit profile
$EDITOR .edi/profile.md

# Start session
edi
```

## Reference Documents

- `docs/implementation/edi-implementation-plan.md` - Detailed implementation guide
- `docs/architecture/recall-mcp-server-spec.md` - RECALL specification
- `docs/architecture/edi-session-lifecycle-spec.md` - Session management
- `docs/architecture/edi-cli-commands-spec.md` - CLI commands
