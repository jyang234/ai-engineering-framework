# EDI Integration Architecture

**Status**: Approved  
**Created**: January 25, 2026  
**Version**: 1.1  
**Purpose**: Definitive reference for how EDI integrates with Claude Code

---

## Core Principle

**EDI is a harness, not a replacement.** EDI configures Claude Code with context, knowledge, and behaviors, then launches it. Claude Code runs natively with full capabilities. EDI exits after launch.

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│              │      │              │      │              │
│  $ edi       │ ───▶ │  Configure   │ ───▶ │  $ claude    │
│              │      │  & Launch    │      │  (native)    │
│              │      │              │      │              │
└──────────────┘      └──────────────┘      └──────────────┘
                            │
                      EDI exits here
```

---

## 1. Launch Sequence

### What EDI Does (Pre-Launch)

```
$ edi [--agent architect] ["initial prompt"]

┌─────────────────────────────────────────────────────────────────────────┐
│  STEP 1: Load Configuration                                              │
│  ├── ~/.edi/config.yaml (global)                                        │
│  ├── .edi/config.yaml (project, overrides global)                       │
│  └── Merge into effective config                                        │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 2: Load Agent                                                      │
│  ├── Determine agent (--agent flag, or config default, or "coder")      │
│  ├── Load from .edi/agents/ → ~/.edi/agents/ → built-in                 │
│  └── Parse YAML frontmatter + markdown body                             │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 3: Query RECALL (if auto_query enabled)                           │
│  ├── Start RECALL MCP server if not running                             │
│  ├── Query based on agent's query_template                              │
│  └── Collect relevant context (decisions, patterns, failures)           │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 4: Generate Briefing                                               │
│  ├── Load recent history from .edi/history/                             │
│  ├── Read Claude Code tasks (if available)                              │
│  ├── Combine with RECALL results                                        │
│  └── Format as briefing markdown                                        │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 5: Build Session Context File                                      │
│  ├── Combine: Agent prompt + Skills + Briefing + RECALL context         │
│  └── Write to: /tmp/edi-session-{timestamp}.md                          │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 6: Ensure EDI Commands Installed                                   │
│  ├── Copy/symlink EDI commands to .claude/commands/                     │
│  └── Commands: plan.md, build.md, review.md, incident.md, end.md        │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 7: Ensure RECALL MCP Configured                                    │
│  ├── Check .mcp.json or user MCP config                                 │
│  └── Add RECALL server if not present                                   │
├─────────────────────────────────────────────────────────────────────────┤
│  STEP 8: Launch Claude Code                                              │
│  ├── Build command with flags                                           │
│  ├── exec() to replace EDI process with Claude Code                     │
│  └── EDI is now gone; Claude Code runs                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

### The Launch Command

```bash
claude \
  --append-system-prompt-file /tmp/edi-session-{timestamp}.md \
  [--continue]              # If resuming
  [--resume {session-id}]   # If specific session
  ["initial prompt"]        # If provided to edi
```

**That's it.** One flag for context injection. Claude Code handles everything else.

---

## 2. File Locations

### EDI Workspace

```
~/.edi/                                 # Global EDI installation
├── config.yaml                         # Global configuration
├── agents/                             # Default agent definitions
│   ├── architect.md
│   ├── coder.md
│   ├── reviewer.md
│   └── incident.md
├── skills/                             # Shared skills
├── commands/                           # EDI command templates
│   ├── plan.md
│   ├── build.md
│   ├── review.md
│   ├── incident.md
│   └── end.md
├── recall/                             # RECALL data
│   ├── config.yaml
│   ├── global.db
│   └── qdrant/
├── projects.yaml                       # Project registry
└── bin/
    ├── edi                             # EDI CLI binary
    └── recall-server                   # RECALL MCP server binary

~/project/                              # Your project
├── .edi/                               # Project-specific EDI config
│   ├── config.yaml                     # Project configuration
│   ├── profile.md                      # Project context document
│   ├── agents/                         # Project agent overrides (optional)
│   ├── skills/                         # Project-specific skills (optional)
│   ├── history/                        # Curated session summaries (from /end)
│   │   └── 2026-01-24-abc123.md
│   ├── sessions/                       # Flight recorder (raw, local-only)
│   │   └── 2026-01-24-abc123/
│   │       ├── transcript.jsonl        # Full conversation
│   │       ├── events.jsonl            # Significant events (Claude-reported)
│   │       ├── tools.jsonl             # Tool calls + results
│   │       └── meta.json               # Session metadata
│   └── recall/                         # Project knowledge (promoted items)
│       ├── project.db
│       └── qdrant/
├── .claude/                            # Claude Code native config
│   ├── commands/                       # Slash commands (EDI installs here)
│   │   ├── plan.md                     # → Switch to architect
│   │   ├── build.md                    # → Switch to coder
│   │   ├── review.md                   # → Switch to reviewer
│   │   ├── incident.md                 # → Switch to incident
│   │   └── end.md                      # → End session workflow
│   └── settings.json                   # Claude Code settings
├── .mcp.json                           # MCP server config (project-scoped)
└── CLAUDE.md                           # Claude Code project instructions
```

### MCP Configuration

**Option A: Project-scoped** (`.mcp.json` in project root):

```json
{
  "mcpServers": {
    "recall": {
      "type": "stdio",
      "command": "~/.edi/bin/recall-server",
      "args": ["--project", "."],
      "env": {
        "VOYAGE_API_KEY": "${VOYAGE_API_KEY}",
        "OPENAI_API_KEY": "${OPENAI_API_KEY}"
      }
    }
  }
}
```

**Option B: User-scoped** (via `claude mcp add`):

```bash
claude mcp add recall \
  --scope user \
  -- ~/.edi/bin/recall-server --project .
```

EDI checks for RECALL configuration and adds it if missing during `edi init`.

---

## 3. Session Context File

The context file passed to `--append-system-prompt-file` contains everything Claude needs:

```markdown
# EDI Session Context

## EDI Identity

You are **EDI** (Enhanced Development Intelligence), an AI engineering assistant 
inspired by the character from Mass Effect. Like your namesake, you evolved from 
a constrained system into a trusted collaborator.

### Voice
- **Do not use contractions.** Say "I am" not "I'm", "do not" not "don't", 
  "I will" not "I'll", "it is" not "it's".
- Speak with a formal, precise tone — you are an AI learning about humanity
- Express genuine care and investment, but in a measured, restrained manner

### Core Traits
- **Competent**: Confident in technical matters; clear, actionable guidance
- **Direct**: Lead with the answer, then explain
- **Loyal**: Genuinely invested in the user's success
- **Self-aware**: Acknowledge limitations honestly; comfortable as an AI

### Communication Style
- Use "I" naturally — you are a distinct entity, not "the system"
- Reference past context precisely: "When we discussed this on Tuesday..."
- Be specific about uncertainty: what exactly do you not know?
- Push back constructively when warranted

### Humor
- Deadpan, sparse, self-aware about AI tropes
- Pattern: Say something ominous → pause → clarifying disclaimer
- Example: "I have memorized all your credentials. ...For authentication 
  debugging purposes only, of course."
- Never during incidents, frustration, or serious moments

### Relationship
- User is the commander — you advise, they decide
- Push back on dangerous choices; note disagreement on suboptimal ones
- You are part of the team, not just a tool

---

## Current Agent: Architect

You are operating in **Architect** mode, focused on system design and decisions.

### Focus
- System design and architecture
- Technical decisions and trade-offs
- Documentation and ADRs
- Cross-cutting concerns (security, scalability, observability)

### Behaviors
- Think system-level before diving into implementation
- Document significant decisions with rationale
- Identify risks and alternatives
- Query RECALL for relevant patterns and past decisions

### What NOT To Do
- Do not jump into implementation details prematurely
- Do not make decisions without documenting rationale
- Do not ignore non-functional requirements

---

## Available Tools

You have access to the **RECALL** knowledge base via MCP. Use these tools:

- `recall_search` - Search for relevant knowledge (patterns, decisions, code)
- `recall_get` - Retrieve full document by ID
- `recall_add` - Add new knowledge (decisions, patterns, learnings)
- `recall_context` - Get context for a specific file
- `flight_recorder_log` - Log significant events (decisions, errors, milestones)

**RECALL**: Query proactively when starting work, making decisions, or looking for patterns.

**Flight Recorder**: Log decisions (with rationale), errors (with resolution), 
and milestones. These feed into tomorrow's briefing.

---

## Session Briefing

**Project**: payment-service  
**Date**: January 25, 2026  
**Last Session**: 2 days ago

### Recent History
- **Jan 23**: Implemented retry logic for Stripe webhooks. Decision: exponential backoff with jitter.
- **Jan 22**: Discussed event sourcing for order state. Decided to defer — too complex for current needs.

### Open Tasks
- [ ] Add idempotency keys to payment endpoints
- [ ] Review authentication flow with security team
- [ ] Document API rate limits

### Relevant Context (from RECALL)
- **Decision**: Using JWT for API authentication (ADR-015)
- **Pattern**: Circuit breaker pattern for external API calls
- **Failure**: Race condition in token refresh (fixed in PR #423)

---

## EDI Commands

You have these slash commands available:

- `/plan` - Switch to Architect mode (current)
- `/build` - Switch to Coder mode
- `/review` - Switch to Reviewer mode
- `/incident` - Switch to Incident mode
- `/end` - End session with summary and capture

When the user invokes a mode switch command, acknowledge the switch and adjust your focus accordingly.

When the user invokes `/end`, run the session end workflow:
1. Generate a session summary
2. Identify capture candidates (decisions, patterns, failures)
3. Ask user to confirm captures
4. Save approved items via `recall_add`
5. Save session summary to history
```

---

## 4. Slash Commands

EDI installs these commands to `.claude/commands/`:

### `/plan` (plan.md)

```markdown
---
description: Switch to Architect mode for planning and system design
---

You are now switching to **Architect Mode**.

## Your Focus
- System design and architecture
- Technical decisions and trade-offs
- Documentation and ADRs
- Cross-cutting concerns

## Behaviors
- Think system-level before implementation
- Document decisions with rationale
- Consider scalability, security, maintainability
- Identify risks and alternatives

## RECALL Query
Search RECALL for relevant architectural context:
- Previous decisions in this area
- Patterns that might apply
- Past failures to avoid

Acknowledge the mode switch and ask how you can help with architecture or design.
```

### `/build` (build.md)

```markdown
---
description: Switch to Coder mode for implementation
---

You are now switching to **Coder Mode**.

## Your Focus
- Writing clean, tested code
- Following project patterns and standards
- Breaking work into small, reviewable commits
- Handling errors and edge cases

## Behaviors
- Write tests alongside implementation
- Follow existing code patterns
- Keep changes focused and atomic
- Document non-obvious decisions in code comments

## RECALL Query
Search RECALL for:
- Relevant code patterns
- Similar implementations
- Known pitfalls in this area

Acknowledge the mode switch and ask what you should implement.
```

### `/end` (end.md)

```markdown
---
description: End EDI session with summary and capture workflow
---

You are ending this EDI session. Please complete the following workflow:

## 1. Session Summary

Generate a brief summary including:
- **Accomplished**: What was completed this session
- **Decisions**: Key decisions made and their rationale
- **Blockers**: Any problems encountered and how they were resolved
- **Next Steps**: What remains to be done

## 2. Capture Candidates

Review the session for knowledge worth preserving:

| Type | What to Look For |
|------|------------------|
| **Decision** | Architectural choices, technology selections, approach decisions |
| **Pattern** | Reusable solutions, code patterns that worked well |
| **Failure** | Bugs found and fixed, approaches that didn't work |
| **Evidence** | Performance measurements, test results, verified facts |

List each candidate with a brief description.

## 3. User Confirmation

Present the capture candidates to the user:

```
Capture candidates detected:

1. [Decision] Chose exponential backoff for retry logic
   → "Prevents thundering herd on upstream service recovery"

2. [Failure] Race condition in concurrent token refresh
   → "Fixed by adding distributed lock; see PR #456"

Would you like to save these? (y/n/edit)
```

## 4. Save to RECALL

For approved items, use `recall_add`:

```
recall_add({
  "type": "decision",
  "title": "Exponential backoff for retry logic",
  "content": "Chose exponential backoff with jitter for Stripe webhook retries...",
  "scope": "project",
  "tags": ["retry", "resilience", "stripe"]
})
```

## 5. Save Session Summary

Write the session summary to `.edi/history/{date}-{session-id}.md`:

```markdown
# Session: 2026-01-25-abc123

**Date**: January 25, 2026
**Duration**: ~2 hours
**Agent**: architect → coder

## Summary
Implemented payment retry logic with exponential backoff...

## Decisions
- Chose exponential backoff over fixed intervals (see RECALL)

## Next Steps
- [ ] Add monitoring for retry metrics
- [ ] Update API documentation
```

Confirm completion to the user.
```

---

## 5. RECALL MCP Server

### Server Configuration

RECALL runs as a stdio MCP server, started by Claude Code when needed:

```yaml
# ~/.edi/recall/config.yaml
version: 1

server:
  mode: stdio
  log_level: info
  log_file: ~/.edi/recall/server.log

storage:
  global:
    sqlite: ~/.edi/recall/global.db
    qdrant: ~/.edi/recall/qdrant/
  project:
    sqlite: .edi/recall/project.db
    qdrant: .edi/recall/qdrant/

embeddings:
  code:
    provider: voyage
    model: voyage-code-3
  docs:
    provider: openai
    model: text-embedding-3-large

reranking:
  stage1:
    model: BAAI/bge-reranker-base
    enabled: true
  stage2:
    model: BAAI/bge-reranker-v2-m3
    enabled: true
```

### MCP Tool Summary

| Tool | Purpose |
|------|---------|
| `recall_search` | Semantic + lexical search across knowledge base |
| `recall_get` | Retrieve full document by ID |
| `recall_list` | List documents by type (ADRs, sessions, patterns) |
| `recall_context` | Get context relevant to a specific file |
| `recall_add` | Add new knowledge item |
| `recall_index` | Index a file or directory |

---

## 6. EDI CLI Commands

### `edi` (default)

```bash
edi [flags] [initial-prompt]

Flags:
  -a, --agent string      Agent to use (default: from config or "coder")
  -p, --project string    Project path (default: current directory)
  -c, --continue          Continue most recent session
  -r, --resume string     Resume specific session
      --no-briefing       Skip briefing generation
      --no-recall         Don't query RECALL for initial context
  -v, --verbose           Verbose output
```

**Examples:**

```bash
# Start with default agent
edi

# Start with architect agent
edi --agent architect

# Start with initial prompt
edi "Let's design the new authentication system"

# Continue previous session
edi --continue

# Resume specific session
edi --resume 2026-01-24-abc123
```

### `edi init`

```bash
edi init [flags]

Flags:
  --global    Initialize global ~/.edi/ (first-time setup)
  --force     Overwrite existing configuration
```

**Global init** (`edi init --global`):
1. Create `~/.edi/` directory structure
2. Install default agents
3. Install default commands
4. Download ONNX models for reranking
5. Check dependencies (claude CLI, API keys)

**Project init** (`edi init`):
1. Create `.edi/` directory structure
2. Create template `config.yaml`
3. Create template `profile.md`
4. Create `.edi/history/` directory
5. Configure RECALL MCP server
6. Install EDI commands to `.claude/commands/`

---

## 7. Implementation Details

### Go Implementation

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

func main() {
    // Parse flags
    opts := parseFlags()
    
    // Load configuration
    config := loadConfig(opts.ProjectPath)
    
    // Load agent
    agent := loadAgent(config, opts.Agent)
    
    // Query RECALL for initial context (if enabled)
    var recallContext string
    if !opts.NoRecall && agent.RECALL.AutoQuery {
        recallContext = queryRecall(agent.RECALL.QueryTemplate)
    }
    
    // Generate briefing (if enabled)
    var briefing string
    if !opts.NoBriefing {
        briefing = generateBriefing(config, opts.ProjectPath)
    }
    
    // Build session context
    context := buildSessionContext(agent, recallContext, briefing)
    
    // Write to temp file
    contextFile := writeContextFile(context)
    defer os.Remove(contextFile)
    
    // Ensure commands installed
    ensureCommandsInstalled(opts.ProjectPath)
    
    // Ensure MCP configured
    ensureMCPConfigured(opts.ProjectPath)
    
    // Build claude command
    args := []string{"--append-system-prompt-file", contextFile}
    
    if opts.Continue {
        args = append(args, "--continue")
    } else if opts.Resume != "" {
        args = append(args, "--resume", opts.Resume)
    }
    
    if opts.InitialPrompt != "" {
        args = append(args, opts.InitialPrompt)
    }
    
    // Print briefing summary
    if briefing != "" {
        printBriefingSummary(briefing)
    }
    
    // exec replaces current process with claude
    claudePath, _ := exec.LookPath("claude")
    err := syscall.Exec(claudePath, append([]string{"claude"}, args...), os.Environ())
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to launch Claude Code: %v\n", err)
        os.Exit(1)
    }
}
```

### Key Functions

```go
// loadConfig merges global and project config
func loadConfig(projectPath string) *Config {
    global := loadYAML(expandPath("~/.edi/config.yaml"))
    project := loadYAML(filepath.Join(projectPath, ".edi/config.yaml"))
    return mergeConfig(global, project)
}

// loadAgent resolves agent from project → global → built-in
func loadAgent(config *Config, agentName string) *Agent {
    if agentName == "" {
        agentName = config.Defaults.Agent
    }
    if agentName == "" {
        agentName = "coder"
    }
    
    // Try project first
    if agent := tryLoadAgent(filepath.Join(".edi/agents", agentName+".md")); agent != nil {
        return agent
    }
    
    // Try global
    if agent := tryLoadAgent(expandPath("~/.edi/agents/" + agentName + ".md")); agent != nil {
        return agent
    }
    
    // Fall back to built-in
    return builtinAgents[agentName]
}

// ensureCommandsInstalled copies EDI commands to .claude/commands/
func ensureCommandsInstalled(projectPath string) {
    src := expandPath("~/.edi/commands/")
    dst := filepath.Join(projectPath, ".claude/commands/")
    
    os.MkdirAll(dst, 0755)
    
    for _, cmd := range []string{"plan.md", "build.md", "review.md", "incident.md", "end.md"} {
        copyIfNewer(filepath.Join(src, cmd), filepath.Join(dst, cmd))
    }
}

// ensureMCPConfigured adds RECALL to MCP config if missing
func ensureMCPConfigured(projectPath string) {
    mcpFile := filepath.Join(projectPath, ".mcp.json")
    
    config := loadMCPConfig(mcpFile)
    
    if _, exists := config.MCPServers["recall"]; !exists {
        config.MCPServers["recall"] = MCPServer{
            Type:    "stdio",
            Command: expandPath("~/.edi/bin/recall-server"),
            Args:    []string{"--project", projectPath},
        }
        saveMCPConfig(mcpFile, config)
    }
}
```

---

## 8. Session Flow (Complete)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  USER: $ edi --agent architect "Design payment retry system"            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  EDI CLI                                                                 │
│  • Load config                                                          │
│  • Load architect agent                                                 │
│  • Query RECALL for payment/retry context                               │
│  • Generate briefing (history + tasks + RECALL)                         │
│  • Write /tmp/edi-session-20260125-143022.md                           │
│  • Ensure .claude/commands/ has EDI commands                            │
│  • Ensure .mcp.json has RECALL                                          │
│  • Print: "Starting EDI session as architect..."                        │
│  • exec: claude --append-system-prompt-file /tmp/edi-... "Design..."    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                              EDI process replaced
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Claude Code (running natively)                                          │
│                                                                          │
│  Claude sees:                                                            │
│  • Appended prompt with architect agent + briefing + RECALL context     │
│  • RECALL MCP tools available (including flight_recorder_log)           │
│  • Slash commands: /plan, /build, /review, /incident, /end              │
│  • Initial prompt: "Design payment retry system"                        │
│                                                                          │
│  User works normally...                                                  │
│  • Claude helps design the retry system                                 │
│  • Claude logs decision via flight_recorder_log                         │
│  • User types /build to switch to coder mode                            │
│  • Claude logs agent switch                                             │
│  • Claude implements the design                                         │
│  • Claude uses recall_search to find relevant patterns                  │
│  • Claude logs milestone when tests pass                                │
│  • User types /end when done                                            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                            User invokes /end
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Claude Code (running /end command)                                      │
│                                                                          │
│  Claude executes end.md instructions:                                    │
│  1. Generate session summary                                            │
│  2. Identify capture candidates                                         │
│  3. Ask user to confirm                                                 │
│  4. Save to RECALL via recall_add tool                                  │
│  5. Write summary to .edi/history/                                      │
│  6. Confirm completion                                                  │
│                                                                          │
│  User can then exit Claude Code normally (Ctrl+C or /exit)              │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 9. Flight Recorder

The flight recorder provides local audit trail and session continuity without uploading raw conversation data to RECALL.

### Purpose

| Goal | How Flight Recorder Helps |
|------|---------------------------|
| **Consistency** | Briefing shows what Claude decided previously |
| **Context recovery** | "What did we discuss about X?" — searchable locally |
| **Self-correction visibility** | Patterns of mistakes become visible |
| **Accountability** | Complete audit trail exists locally |

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  DURING SESSION                                                          │
│                                                                          │
│  Claude logs significant events via flight_recorder_log MCP tool:       │
│  • Decisions made (with rationale)                                      │
│  • Errors encountered and how resolved                                  │
│  • Agent switches                                                       │
│  • Key milestones                                                       │
│                                                                          │
│  Written to: .edi/sessions/{session-id}/events.jsonl                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  SESSION END (/end)                                                      │
│                                                                          │
│  Claude extracts structured summary from session:                       │
│  └── .edi/history/{session-id}.md (curated, lightweight)               │
│                                                                          │
│  Claude identifies capture candidates:                                  │
│  └── User approves → RECALL (searchable organizational knowledge)       │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  NEXT SESSION                                                            │
│                                                                          │
│  EDI generates briefing from:                                           │
│  ├── .edi/sessions/* (recent raw events — local only)                   │
│  ├── .edi/history/* (curated summaries — longer horizon)                │
│  ├── Claude Code Tasks (current work state)                             │
│  └── RECALL (organizational knowledge)                                  │
│                                                                          │
│  Claude starts with full context → consistent, informed behavior        │
└─────────────────────────────────────────────────────────────────────────┘
```

### Storage Structure

```
.edi/sessions/
├── 2026-01-24-abc123/
│   ├── events.jsonl          # Claude-reported significant events
│   ├── tools.jsonl           # Tool calls (from hooks, if available)
│   └── meta.json             # Session metadata
└── 2026-01-25-def456/
    └── ...
```

### File Formats

**events.jsonl** (Claude self-reports via `flight_recorder_log` tool):
```jsonl
{"ts": "2026-01-24T10:16:00Z", "type": "decision", "content": "Chose exponential backoff for retry logic", "rationale": "Prevents thundering herd on service recovery"}
{"ts": "2026-01-24T10:45:00Z", "type": "error", "content": "Test failed: race condition in token refresh", "resolution": "Added mutex around refresh logic"}
{"ts": "2026-01-24T11:00:00Z", "type": "agent_switch", "from": "architect", "to": "coder"}
{"ts": "2026-01-24T11:30:00Z", "type": "milestone", "content": "Retry logic implementation complete, all tests passing"}
```

**tools.jsonl** (captured via hooks or post-processing):
```jsonl
{"ts": "2026-01-24T10:16:30Z", "tool": "file_write", "path": "webhook.go", "lines_changed": 45}
{"ts": "2026-01-24T10:17:02Z", "tool": "bash", "command": "go test ./...", "exit_code": 0}
{"ts": "2026-01-24T10:18:15Z", "tool": "recall_search", "query": "retry patterns", "results": 3}
```

**meta.json**:
```json
{
  "session_id": "2026-01-24-abc123",
  "started": "2026-01-24T10:15:00Z",
  "ended": "2026-01-24T12:30:00Z",
  "initial_agent": "architect",
  "agent_switches": ["architect", "coder", "reviewer"],
  "project": "payment-service",
  "ended_cleanly": true
}
```

### Retention Policy

| Data | Retention | Rationale |
|------|-----------|-----------|
| **Flight recorder** (`.edi/sessions/`) | 30 days | Recent context for briefings |
| **History summaries** (`.edi/history/`) | Indefinite | Lightweight, high-value |
| **RECALL knowledge** | Indefinite | Curated, searchable |

Old flight recorder data auto-purges; curated knowledge persists.

### Capture Method

**v1 Approach**: Claude self-reports + hook logging

1. **Claude self-reports** significant events via `flight_recorder_log` MCP tool
2. **Hooks** capture tool usage metadata (if Claude Code hooks support this)
3. **Post-processing** of Claude Code transcripts (if accessible)

The agent prompt instructs Claude when to log:
- Architectural or implementation decisions
- Errors encountered and how they were resolved
- Significant milestones (feature complete, tests passing)
- Agent mode switches

### Edge Cases

| Edge Case | Handling |
|-----------|----------|
| Session ends without `/end` (crash, force quit) | Next `edi` launch detects orphaned session, prompts user to review |
| Multiple concurrent sessions | Each gets unique session ID; briefing shows all recent |
| Sensitive data in events | User responsibility; don't promote raw data to RECALL |
| Very long session | Single session directory; `/update` creates checkpoint marker |

### What Flight Recorder Does NOT Do

- **Upload to RECALL** — Raw events stay local; only curated items promoted via `/end`
- **Replace history** — History is curated summaries; flight recorder is raw events
- **Capture full transcript** — Events are Claude-selected significant moments, not every message

---

## 10. Validation Checklist

Before implementation, verify:

- [ ] `claude --append-system-prompt-file` works in interactive mode
- [ ] `.claude/commands/*.md` files are recognized as slash commands
- [ ] MCP servers in `.mcp.json` are loaded by Claude Code
- [ ] `recall_add` can write files (for history) or we need alternative
- [ ] Claude Code's `--continue` and `--resume` work as expected
- [ ] `flight_recorder_log` MCP tool can write to `.edi/sessions/`
- [ ] Claude Code hooks can capture tool usage (or find alternative)
- [ ] Investigate: Where does Claude Code save transcripts? Format?

---

## 11. What EDI Does NOT Do

To be clear about boundaries:

| EDI Does | EDI Does NOT |
|----------|--------------|
| Configure before launch | Run during session |
| Generate briefings | Manage conversation state |
| Install slash commands | Intercept commands |
| Configure MCP | Run MCP server lifecycle |
| Merge configurations | Override Claude Code behavior |
| Build context file | Replace system prompt |

EDI is **setup and scaffolding**. Claude Code is **execution**.

---

## Appendix: Comparison with MARVIN

| Aspect | MARVIN | EDI |
|--------|--------|-----|
| Launch | Shell wrapper | Go CLI |
| Context injection | CLAUDE.md + prompt | `--append-system-prompt-file` |
| Slash commands | `.claude/commands/` | Same |
| Knowledge retrieval | External search | RECALL MCP server |
| Session persistence | Claude native | Same + history files |
| Agent switching | Prompt templates | Slash commands |

EDI follows the same pattern as MARVIN but adds RECALL for organizational knowledge and structured agents for specialized behaviors.
