Last updated: 2026-01-29

## Current Milestone
EDI v0 complete, iterating on developer experience and knowledge capture quality.

## Completed
- EDI CLI with init, launch, config, recall, history, agent commands
- RECALL MCP server with FTS5 search
- 4 core agents + 7 subagents
- 7 slash commands (added /end-recovery)
- Briefing generation from profile/history
- Task annotations system
- Codex v1: project structure and core components
- Stale session detection + /end-recovery command
- Enriched RECALL items: auto-injected session/git metadata + structured content templates

## Next Steps
- Run `edi init --global` to install new end-recovery command
- Test end-to-end stale session detection (start session, Ctrl+C, restart)
- Test enriched RECALL items: verify metadata injection and structured templates via /end
- Add more comprehensive tests
- Implement v1 features (vector search, web UI)
- Build additional AEF components
