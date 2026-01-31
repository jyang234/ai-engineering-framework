# EDI Implementation Gaps Analysis

> **Implementation Status (January 31, 2026):** Gap resolutions mostly followed. Exception: MCP SDK decision was later reversed (hand-rolled JSON-RPC used instead of official SDK).

**Purpose**: Identify and resolve all gaps before generating detailed implementation plan for Claude Code.

**Status**: In Progress

---

## Gap Categories

1. **Critical** — Blocks implementation, must resolve now
2. **Important** — Would cause confusion, should resolve now
3. **Minor** — Can defer, note for implementer

---

## 1. Task Integration Hooks

### Gap 1.1: How does EDI intercept Task creation?

**Status**: ✅ RESOLVED

**Research findings**:
- Tasks stored in `~/.claude/tasks/{session-uuid}/{task-number}.json`
- JSON format with `blocks`/`blockedBy` for dependencies
- `CLAUDECODETASKLISTID` env var enables cross-session sharing
- No documented tool name for task creation (Claude creates them internally)

**Decision for v0**: Use **guidance in edi-core skill** (Option D)

Claude follows skill instructions to:
1. After creating tasks, call `recall_search` for each task
2. Store annotations using `flight_recorder_log` with metadata
3. EDI reads flight recorder to populate `.edi/tasks/` annotations

This is simplest for v0. Future versions can add file watching or hooks.

**Implementation**:
```markdown
# In edi-core skill

## After Creating Tasks

When you create tasks for a project, for each task:
1. Query RECALL: `recall_search({query: "[task description]", types: ["pattern", "failure", "decision"]})`
2. Log the annotation: `flight_recorder_log({type: "task_annotation", task_id: "[id]", recall_items: [...]})`

This stores RECALL context with each task for future reference.
```

---

### Gap 1.2: How does EDI intercept Task completion?

**Status**: ✅ RESOLVED

**Decision for v0**: Use **guidance in edi-core skill** (Option B)

Claude follows skill instructions to:
1. On task completion, log decisions with `propagate: true` flag
2. Call `flight_recorder_log` with task completion event
3. EDI's `/end` command reads flight recorder and prompts for capture

**Implementation**:
```markdown
# In edi-core skill

## On Task Completion

When completing a task:
1. Log key decisions: `flight_recorder_log({type: "decision", task_id: "[id]", propagate: true, ...})`
2. Log completion: `flight_recorder_log({type: "task_complete", task_id: "[id]", decisions: [...]})`
3. If task has dependents, note which decisions should propagate
```

---

### Gap 1.3: Task annotation file location

**Status**: ✅ RESOLVED

**Decision**: `.edi/tasks/{task-id}.yaml`

**Task ID source**: Use Claude Code's task ID from `~/.claude/tasks/` filesystem structure.
Format: `{session-uuid}-{task-number}` (e.g., `abc123-1`, `abc123-2`)

---

### Gap 1.4: How does Claude know about inherited context?

**Status**: ✅ RESOLVED (NEW)

**Problem**: When Claude picks up a task, how does it know about decisions from parent tasks?

**Decision for v0**: EDI's `/task` command loads annotations and presents to Claude

When user runs `/task task-004`:
1. Slash command reads `.edi/tasks/task-004.yaml`
2. Finds inherited_context from completed dependencies  
3. Presents to Claude as context in the command response

This keeps Claude's skill simple while EDI handles the orchestration.

---

## 2. MCP Server Protocol Details

### Gap 2.1: MCP SDK for Go — does it exist and is it stable?

**Status**: ✅ RESOLVED

**Finding**: The official Go MCP SDK exists at `github.com/modelcontextprotocol/go-sdk/mcp`:
- Maintained by Model Context Protocol team in collaboration with Google
- Current version supports MCP spec 2025-11-25
- Provides `mcp.NewServer()`, `mcp.AddTool()`, and stdio transport
- Published Dec 2024, actively maintained with releases through Jan 2026
- 395+ projects importing it

**Example from docs**:
```go
server := mcp.NewServer(&mcp.Implementation{Name: "recall", Version: "v1.0.0"}, nil)
mcp.AddTool(server, &mcp.Tool{Name: "recall_search", Description: "..."}, RecallSearchHandler)
if err := server.Run(ctx, mcp.NewStdioTransport()); err != nil {
    log.Fatal(err)
}
```

**Alternative**: `github.com/mark3labs/mcp-go` (community, also popular)

**DECISION**: Use official SDK `github.com/modelcontextprotocol/go-sdk/mcp`

---

### Gap 2.2: MCP Server stdio transport implementation

**Status**: ✅ RESOLVED

The official SDK handles all protocol details. Just call `server.Run(ctx, mcp.NewStdioTransport())`.

**DECISION**: SDK handles it, no manual protocol implementation needed

---

## 3. RECALL v0 vs v1 Scope

### Gap 3.1: What's in v0 (MVP) vs v1?

**Status**: ✅ RESOLVED

**v0 scope** (for initial implementation):

| Feature | v0 | v1 |
|---------|----|----|
| SQLite FTS search | ✅ | ✅ |
| recall_search tool | ✅ | ✅ |
| recall_add tool | ✅ | ✅ |
| recall_feedback tool | ✅ | ✅ |
| flight_recorder_log tool | ✅ | ✅ |
| recall_get (by ID) | ✅ | ✅ |
| Vector embeddings (Voyage) | ❌ | ✅ |
| Hybrid search | ❌ | ✅ |
| Multi-stage reranking | ❌ | ✅ |
| AST-aware chunking | ❌ | ✅ |
| Contextual retrieval | ❌ | ✅ |
| Web UI | ❌ | ✅ |
| Qdrant integration | ❌ | ✅ |

**DECISION**: v0 = SQLite FTS only, no external API calls, single binary

---

### Gap 3.2: RECALL storage location

**Status**: ✅ RESOLVED

- Global: `~/.edi/recall/global.db`
- Project: `.edi/recall/project.db`

---

## 4. CLI Implementation Details

### Gap 4.1: How does `edi` launch Claude Code?

**Status**: ✅ RESOLVED

**DECISION**: Use `syscall.Exec` — EDI replaces itself with Claude Code process

```go
// EDI exits cleanly, Claude Code takes over the terminal
syscall.Exec(claudePath, args, env)
```

---

### Gap 4.2: Where is the context file written?

**Status**: ✅ RESOLVED

**DECISION**: `~/.edi/cache/session-{timestamp}.md`

Cleaned up on next `edi` launch (not immediately, allows debugging).

---

### Gap 4.3: How do slash commands get installed?

**Status**: ✅ RESOLVED

**Source**: `~/.edi/commands/*.md` → `.claude/commands/*.md`

**DECISION**: Copy if file is missing OR content hash differs (checksum comparison using SHA256).

---

## 5. Configuration Schema

### Gap 5.1: Required vs optional fields

**Status**: ✅ RESOLVED

**DECISION**: Only `version` field required; everything else has sensible defaults

```yaml
# Minimum valid config
version: "1"

# All defaults:
# agent: "coder"
# recall.enabled: true
# briefing.include_history: true
# briefing.history_entries: 3
# capture.friction_budget: 3
# tasks.lazy_loading: true
```

---

### Gap 5.2: Config merge precedence

**Status**: ✅ RESOLVED

**DECISION**: Arrays are **replaced entirely** (no merge)

Example: If global config has `briefing.sources: [history, tasks]` and project config has `briefing.sources: [recall]`, result is `[recall]` only.

---

## 6. Error Handling

### Gap 6.1: What if RECALL MCP server is unavailable?

**Status**: ✅ RESOLVED

**DECISION**: **B — Graceful degradation**

- Launch Claude Code even if RECALL fails to start
- Warn user: "RECALL unavailable, starting without knowledge retrieval"
- Claude Code's MCP error handling manages tool failures

---

### Gap 6.2: What if Claude Code isn't installed?

**Status**: ✅ RESOLVED

Check `which claude` and provide helpful error:
```
Error: Claude Code not found in PATH.
Install from: https://claude.ai/code
```

---

### Gap 6.3: What if MCP server crashes mid-session?

**Status**: ✅ RESOLVED

**DECISION**: **A — Rely on native MCP error handling**

No watchdog in v0. Claude Code handles MCP tool failures gracefully.

---

## 7. Testing Strategy

### Gap 7.1: How to test MCP server?

**Status**: ✅ RESOLVED

**DECISION**: Both unit and integration tests

- **Unit tests**: Mock MCP protocol messages, test handlers directly
- **Integration tests**: Start actual MCP server, connect with test client
- Use Go's `testing` package + `github.com/stretchr/testify`

---

### Gap 7.2: How to test CLI?

**Status**: ✅ RESOLVED

Standard Go testing with mock filesystem (`testing/fstest`) and mock exec.

---

## 8. Build and Distribution

### Gap 8.1: How is EDI distributed?

**Status**: ✅ RESOLVED

**DECISION**: **Binary releases only for v0**

- GitHub releases with pre-built binaries
- User downloads and adds to PATH
- Future: Homebrew formula, installation script

---

### Gap 8.2: Cross-platform support?

**Status**: ✅ RESOLVED

| Platform | v0 Support |
|----------|------------|
| macOS (Apple Silicon) | ✅ Required |
| macOS (Intel) | ✅ Required |
| Linux (x64) | ✅ Required |
| Linux (ARM) | ⚠️ Nice to have |
| Windows | ❌ Not in v0 |

**DECISION**: Build for darwin/amd64, darwin/arm64, linux/amd64

---

## 9. File Format Details

### Gap 9.1: Task annotation YAML schema

**Status**: ✅ RESOLVED (specified in session lifecycle spec)

---

### Gap 9.2: Flight recorder JSONL schema

**Status**: ✅ RESOLVED

**Location**: `.edi/history/{session-id}-flight.jsonl` (per-session, alongside history)

**Schema**:
```jsonl
{"timestamp": "2026-01-25T14:30:00Z", "type": "decision", "content": "Using Stripe for payments", "rationale": "Better webhook reliability", "metadata": {"task_id": "abc123-4", "propagate": true}}
{"timestamp": "2026-01-25T14:35:00Z", "type": "task_complete", "content": "Task abc123-4 completed", "metadata": {"task_id": "abc123-4", "decisions": ["Using Stripe for payments"]}}
{"timestamp": "2026-01-25T14:36:00Z", "type": "observation", "content": "Stripe returns 429 with Retry-After", "metadata": {"tag": "parallel-discovery", "applies_to": ["payment", "refund"]}}
```

---

### Gap 9.3: History entry format

**Status**: ✅ RESOLVED (specified in session lifecycle spec)

---

## 10. Agent/Skill Loading

### Gap 10.1: How are agent definitions loaded?

**Status**: ✅ RESOLVED

**DECISION**: Agents installed to `~/.edi/agents/` during `edi init`

Load order:
1. `.edi/agents/{name}.md` (project override)
2. `~/.edi/agents/{name}.md` (global/default)

Built-in agents are copied to `~/.edi/agents/` during init, making them easy to customize.

---

### Gap 10.2: How is edi-core skill loaded by subagents?

**Status**: ✅ RESOLVED

**DECISION**: EDI installs `~/.claude/skills/edi-core/SKILL.md` during `edi init`

Subagents declare `skills: edi-core` in frontmatter, Claude Code loads automatically.

---

---

## Summary: All Decisions Resolved ✅

| # | Gap | Decision |
|---|-----|----------|
| 1.1 | Task creation hook | Guidance in edi-core skill (Claude calls recall_search) |
| 1.2 | Task completion hook | Guidance in edi-core skill (Claude logs to flight recorder) |
| 1.3 | Task annotation ID | Use Claude Code's ID: `{session-uuid}-{task-number}` |
| 1.4 | Inherited context loading | `/task` command loads and presents to Claude |
| 2.1 | Go MCP SDK | Official: `github.com/modelcontextprotocol/go-sdk/mcp` |
| 2.2 | MCP stdio transport | SDK handles it with `mcp.NewStdioTransport()` |
| 3.1 | v0 scope | SQLite FTS only, no external APIs |
| 4.1 | CLI launch mechanism | `syscall.Exec` (EDI replaces itself) |
| 4.2 | Context file location | `~/.edi/cache/session-{timestamp}.md` |
| 4.3 | Slash command install | Copy if missing or hash differs |
| 5.1 | Required config fields | Only `version` required |
| 5.2 | Array merge behavior | Replace entirely (no merge) |
| 6.1 | RECALL unavailable | Graceful degradation with warning |
| 6.3 | MCP crash handling | Rely on native MCP error handling |
| 7.1 | MCP testing | Unit + integration tests |
| 8.1 | Distribution | Binary releases only for v0 |
| 8.2 | Platforms | darwin/amd64, darwin/arm64, linux/amd64 |
| 9.2 | Flight recorder location | `.edi/history/{session-id}-flight.jsonl` |
| 10.1 | Agent loading | Installed to `~/.edi/agents/` during init |
| 10.2 | edi-core skill | Installed to `~/.claude/skills/edi-core/` during init |

---

## Implementation Ready: YES ✅

All gaps have been resolved. The specifications are complete enough for Claude Code to implement.

### Key Implementation Notes for Claude Code

1. **Go MCP SDK**: Use `github.com/modelcontextprotocol/go-sdk/mcp` — it handles all protocol details
2. **Task integration**: Relies on Claude following edi-core skill instructions, not hooks
3. **v0 simplicity**: SQLite FTS only, no external API dependencies
4. **Launch pattern**: `syscall.Exec` replaces EDI process with Claude Code
5. **Testing**: Both unit tests (mock MCP) and integration tests (real server)

### Files to Reference During Implementation

| Document | Purpose |
|----------|---------|
| `edi-specification-index.md` | Overview, architecture, quick reference |
| `edi-workspace-config-spec.md` | Directory structure, config schema |
| `recall-mcp-server-spec.md` | MCP tools, storage, v0 FTS implementation |
| `edi-session-lifecycle-spec.md` | Briefing, history, capture, Tasks integration |
| `edi-cli-commands-spec.md` | CLI commands, slash commands |
| `edi-agent-system-spec.md` | Agent schema, core agents |
| `edi-subagent-specification.md` | Subagent definitions, edi-core skill |
| `edi-persona-spec.md` | EDI's voice, humor, personality |

### Next Step

Generate detailed implementation plan with phases, milestones, and specific tasks for Claude Code.

