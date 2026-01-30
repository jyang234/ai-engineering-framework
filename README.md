# AI Engineering Framework (AEF)

AEF gives Claude Code persistent memory, specialized agents, and session continuity. Your AI assistant remembers what it learned.

## What You Get

- **Sessions that pick up where you left off** — Every session starts with a briefing: project profile, recent history, open tasks. No cold starts.
- **Knowledge that accumulates** — Patterns, decisions, and failures are stored and retrieved automatically. Ask "how did we handle auth?" and get the answer, not a blank stare.
- **Specialized modes for different work** — Switch between coder, architect, reviewer, and incident modes mid-session with slash commands.
- **Hybrid search** — Combines semantic understanding with keyword matching. Handles both vague queries ("something about retry logic") and precise lookups ("idempotency key").
- **Everything local** — Single SQLite file, local embeddings via Ollama, no API keys for core features.

## How It Works

```
edi
 │
 ├─► Loads agents, briefing, and config
 ├─► Starts RECALL/Codex MCP server
 └─► Launches Claude Code with everything wired up
      │
      └─► Claude uses MCP tools to search, store, and log knowledge
```

EDI configures the session and then gets out of the way. Claude Code runs natively with full access to knowledge tools.

## Quick Start

```bash
# Build and install EDI
cd edi && make build && make install

# Initialize (once per machine)
edi init --global

# Initialize in your project
cd your-project && edi init

# Edit your project profile
$EDITOR .edi/profile.md

# Start a session
edi
```

For hybrid search (semantic + keyword), use the Codex backend:

```bash
# Install Ollama and pull the embedding model
ollama pull nomic-embed-text

# Initialize with Codex backend (auto-builds if source is available)
edi init --global --backend=codex

# Or install manually
cd codex && make build && cp bin/recall-mcp ~/.edi/bin/

# Check everything works
edi doctor
```

## What's Global vs. Project-Level

| Level | Location | Contains |
|-------|----------|----------|
| **Global** (per machine) | `~/.edi/` | Binaries, agents, commands, knowledge DB, config |
| **Project** (per repo) | `.edi/` | Profile, history, tasks, config overrides |

Codex (the knowledge engine) is a **workspace-level** component — one binary and one database shared across all projects. Items are tagged with project metadata automatically.

Run `edi doctor` to verify your installation.

See [AEF Install Model](docs/architecture/aef-install-model.md) for details.

## Components

| Component | What it does | README |
|-----------|-------------|--------|
| **EDI** | Claude Code harness — agents, briefings, history, session management | [edi/README.md](edi/README.md) |
| **Codex** | Knowledge engine — hybrid search, local embeddings, single-file storage | [codex/README.md](codex/README.md) |

## Further Reading

- [EDI + Codex Technical Deep-Dive](docs/edi-codex-deep-dive.md) — full system architecture, data flows, and operational guide
- [AEF Components Overview](docs/aef-components.md)
- [RECALL MCP Server Spec](docs/architecture/recall-mcp-server-spec.md)
- [EDI Session Lifecycle](docs/architecture/edi-session-lifecycle-spec.md)
- [EDI CLI Commands](docs/architecture/edi-cli-commands-spec.md)

## License

MIT
