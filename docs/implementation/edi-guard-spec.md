# edi-guard: Command Hook Spec

**Date**: 2026-03-01
**Status**: Ready for implementation
**Depends on**: Claude Code hooks API (command type), existing `task-sync-hook` pattern
**Exploration**: `docs/architecture/webhook-hooks-exploration.md`

## Overview

`edi-guard` is a single Go binary that handles four Claude Code hook events. It enforces build tags, blocks destructive commands, detects failure loops, and snapshots session state before compaction.

```
PreToolUse (Bash)        → deny-list check, build tag injection, failure counter read
PostToolUse (Bash)       → failure counter reset
PostToolUseFailure (Bash) → failure counter increment
PreCompact (.*)          → write /memories/compaction-state.md
```

## Hook Input Schema

All events receive common fields on stdin as JSON:

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/project",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse"
}
```

### PreToolUse (Bash) — additional fields

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "go test ./pkg/...",
    "description": "Run tests",
    "timeout": 120000
  },
  "tool_use_id": "toolu_01ABC123"
}
```

### PostToolUse (Bash) — additional fields

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "go test -tags fts5 ./...",
    "description": "Run tests"
  },
  "tool_use_id": "toolu_01ABC123",
  "tool_response": "ok  \tgithub.com/anthropics/aef/edi/internal/config\t0.003s"
}
```

### PostToolUseFailure (Bash) — additional fields

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "go test ./...",
    "description": "Run tests"
  },
  "tool_use_id": "toolu_01ABC123",
  "error": "Command exited with non-zero status code 1",
  "is_interrupt": false
}
```

### PreCompact — additional fields

```json
{
  "trigger": "auto",
  "custom_instructions": ""
}
```

`trigger` is `"manual"` (from `/compact`) or `"auto"` (context window full).

## Policy 1: Build Tag Enforcement

### Problem

`go test`, `go build`, and `go run` require `-tags fts5` in this project. Claude forgets this after compaction.

### Mechanism

**Tool input modification** (not blocking). When the hook detects a missing flag, it rewrites `tool_input.command` and returns it on stdout. Claude Code executes the modified command. No retry needed. Supported since Claude Code v2.0.10.

### Detection Logic

```
1. Extract command from tool_input.command
2. Check if command contains "go test", "go build", or "go run" (anywhere in string, for compound commands)
3. If yes, check if command already contains "-tags" followed by "fts5" (handles -tags fts5, -tags "fts5", -tags=fts5, -tags "fts5,other")
4. If no, check if command starts with "make" → skip (make targets handle their own flags)
5. If missing: inject -tags fts5 after the go subcommand
```

### Injection Rule

Insert `-tags fts5` immediately after the `go (test|build|run)` token:

| Input | Output |
|-------|--------|
| `go test ./...` | `go test -tags fts5 ./...` |
| `go test -v -count=1 ./pkg/core` | `go test -tags fts5 -v -count=1 ./pkg/core` |
| `go build -o bin/edi ./cmd/edi` | `go build -tags fts5 -o bin/edi ./cmd/edi` |
| `cd codex && go test ./...` | `cd codex && go test -tags fts5 ./...` |
| `go test -tags fts5 ./...` | `go test -tags fts5 ./...` (no change) |
| `go test -tags "fts5,integration" ./...` | `go test -tags "fts5,integration" ./...` (no change) |
| `make test` | `make test` (no change — skip make commands) |
| `make build` | `make build` (no change) |

### Output (when modified)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "toolInput": {
      "command": "go test -tags fts5 ./..."
    }
  }
}
```

Exit code 0. If no modification needed, exit 0 with no output.

### Config

Read from `.edi/config.yaml`:

```yaml
guard:
  build_tags: ["fts5"]
```

If `guard.build_tags` is empty or missing, skip this check entirely.

### Regex

```go
// Match go test/build/run anywhere in command (handles compound commands like "cd x && go test")
goCommandRe = regexp.MustCompile(`\bgo\s+(test|build|run)\b`)

// Check if -tags already includes the required tag
hasTagRe = regexp.MustCompile(`-tags[= ]+\S*\bfts5\b`)
```

## Policy 2: Destructive Command Guard

### Problem

Project-specific destructive patterns (`rm -rf .edi/`, force-push to main) aren't covered by Claude Code's built-in protections.

### Mechanism

**Blocking** (exit code 2). stderr contains the reason, which Claude sees as an error message.

### Default Deny Patterns

```yaml
guard:
  deny_patterns:
    - pattern: "rm\\s+(-[rf]+\\s+)*(\\.edi|\\.claude)(/|\\s|$)"
      reason: "Blocked: recursive delete of .edi/ or .claude/ directory"
    - pattern: "git\\s+push\\s+.*(--force|-f).*\\b(main|master)\\b|git\\s+push\\s+.*\\b(main|master)\\b.*(--force|-f)"
      reason: "Blocked: force push to main/master"
    - pattern: "git\\s+reset\\s+--hard"
      reason: "Blocked: git reset --hard (use git stash or git checkout instead)"
```

### Pattern Design Principles

1. Patterns are Go regexes applied to the full `tool_input.command` string
2. Match both long flags (`--force`) and short flags (`-f`)
3. Handle flag-before-branch and branch-before-flag ordering
4. Anchor to word boundaries (`\b`) to avoid false positives on substrings
5. Accept false positives on `rm -rf .edi/tasks/...` — blocking any recursive delete inside `.edi/` is reasonable

### Detection Logic

```
1. Extract command from tool_input.command
2. For each pattern in deny_patterns:
   a. Compile regex (cache after first compile)
   b. Match against command
   c. If match: write reason to stderr, exit 2
3. If no match: continue to next policy
```

### Output (when blocked)

No stdout. Write to stderr:

```
edi-guard: Blocked: force push to main/master
```

Exit code 2.

### Config

```yaml
guard:
  deny_patterns:
    - pattern: "..."
      reason: "..."
```

Project config (`.edi/config.yaml`) and global config (`~/.edi/config.yaml`) deny patterns are merged (concatenated, not deduplicated). This lets teams define project-level safety rails while individuals add personal ones.

## Policy 3: Failure Loop Breaker

### Problem

Claude retries failing commands with minor variations instead of stepping back to diagnose the root cause.

### Mechanism

**Advisory** (not blocking). When consecutive Bash failures exceed a threshold, inject context via `additionalContext` in the next PreToolUse. Claude sees the advisory and can adjust its approach.

### State File

Stored at `/tmp/edi-guard-{session_id}.json`:

```json
{
  "consecutive_failures": 3,
  "advised": false,
  "last_failure_command": "go test ./...",
  "last_failure_error": "exit status 1"
}
```

### Event Flow

```
Bash command fails:
  PostToolUseFailure → read state → increment consecutive_failures → write state

Bash command succeeds:
  PostToolUse → read state → reset consecutive_failures to 0, advised to false → write state

Next Bash command attempted:
  PreToolUse → read state → if consecutive_failures >= threshold AND NOT advised:
    set advised = true, write state
    return additionalContext advisory
```

### Advisory Output (PreToolUse, when threshold exceeded)

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "additionalContext": "edi-guard: 5 consecutive Bash command failures detected. The last failure was: \"go test ./...\" → \"exit status 1\". Consider stepping back to analyze the root cause rather than retrying with small variations."
  }
}
```

Exit code 0. The command is allowed to proceed.

### Reset Semantics

- Any successful Bash command resets the counter to 0 and `advised` to false
- This means the advisory can fire again if a new failure streak begins
- `advised` prevents the same streak from injecting the advisory on every subsequent PreToolUse

### Config

```yaml
guard:
  failure_loop_threshold: 5
```

Default threshold is 5 if not configured. A lower threshold (3) is more aggressive but may fire during legitimate test-driven development loops.

### Edge Cases

- **State file doesn't exist**: treat as `{consecutive_failures: 0, advised: false}`. Create on first write.
- **State file read fails**: log to stderr, skip counter logic, continue to next policy. Never block.
- **Multiple sessions**: state file is keyed by `session_id`, so sessions don't interfere.
- **Session cleanup**: files in `/tmp/` are ephemeral. OS handles cleanup. No explicit cleanup needed.

## Policy 4: PreCompact State Snapshot

### Problem

Compaction compresses the conversation, causing Claude to lose track of active task, branch, build requirements, and prior test results.

### Mechanism

**Side effect** (file write). PreCompact hooks cannot block compaction. The hook writes mechanical facts to `{cwd}/memories/compaction-state.md`. The edi-core skill instruction (which survives compaction as part of the system prompt) tells Claude to check `/memories/` after compaction.

### Data Sources

| Fact | Source | Read method |
|------|--------|-------------|
| Task ID + subject | `{cwd}/.edi/tasks/active.yaml` | Parse YAML, find first task with `status: in_progress` |
| Git branch | `git branch --show-current` | `exec.Command` |
| Build tags | `{cwd}/.edi/config.yaml` | Parse YAML, read `guard.build_tags` |
| Last test result | `/tmp/edi-guard-{session_id}.json` | Read state file, check `consecutive_failures` |
| Agent mode | `{cwd}/.edi/config.yaml` | Parse YAML, read `agent` |

### Output File

Written to `{cwd}/memories/compaction-state.md`. Create directory with `os.MkdirAll` if it doesn't exist.

```markdown
# Session State (auto-generated by edi-guard before compaction)
- Task: TSK-042 — Implement PreCompact hook for edi-guard
- Branch: feat/edi-guard-hooks
- Build tags required: fts5
- Last test result: 3 consecutive failures (last: "go test ./..." → exit status 1)
- Agent mode: coder
- Compaction trigger: auto
```

### Content Rules

1. **Maximum 8 lines**. This file persists across compaction and consumes context tokens.
2. **Mechanical facts only**. No prose, no summaries, no "what was I working on" narratives.
3. **One line per fact**. Prefix with `- ` for consistent parsing.
4. **Omit missing data**. If no active task, omit the task line. If no failures, write `- Last test result: no recent failures`.
5. **Overwrite on each compaction**. The file always reflects the most recent pre-compaction state.

### Error Handling

Every I/O operation is best-effort. If any source is unavailable:
- Task file missing → omit task line
- Git not available → omit branch line
- Config unreadable → omit build tags and agent mode lines
- State file missing → write "no recent failures"
- Can't create directory or write file → log to stderr, exit 0

Never fail the hook. Exit 0 always.

## Configuration Schema

### Go Struct

Add to `edi/internal/config/schema.go`:

```go
// GuardConfig configures the edi-guard command hook
type GuardConfig struct {
	Enabled              bool           `yaml:"enabled" mapstructure:"enabled"`
	BuildTags            []string       `yaml:"build_tags" mapstructure:"build_tags"`
	DenyPatterns         []DenyPattern  `yaml:"deny_patterns" mapstructure:"deny_patterns"`
	FailureLoopThreshold int            `yaml:"failure_loop_threshold" mapstructure:"failure_loop_threshold"`
}

// DenyPattern is a regex pattern that blocks matching Bash commands
type DenyPattern struct {
	Pattern string `yaml:"pattern" mapstructure:"pattern"`
	Reason  string `yaml:"reason" mapstructure:"reason"`
}
```

Add `Guard GuardConfig` field to `Config` struct:

```go
type Config struct {
	// ... existing fields ...

	// Guard hook configuration
	Guard GuardConfig `yaml:"guard" mapstructure:"guard"`
}
```

### Default Config

```yaml
guard:
  enabled: true
  build_tags:
    - fts5
  deny_patterns:
    - pattern: "rm\\s+(-[rf]+\\s+)*(\\.edi|\\.claude)(/|\\s|$)"
      reason: "Blocked: recursive delete of .edi/ or .claude/ directory"
    - pattern: "git\\s+push\\s+.*(--force|-f).*\\b(main|master)\\b|git\\s+push\\s+.*\\b(main|master)\\b.*(--force|-f)"
      reason: "Blocked: force push to main/master"
    - pattern: "git\\s+reset\\s+--hard"
      reason: "Blocked: git reset --hard (use git stash or git checkout instead)"
  failure_loop_threshold: 5
```

### Config Resolution

`edi-guard` reads config from two locations, merged at load time:
1. `{cwd}/.edi/config.yaml` — project-level (checked into repo)
2. `~/.edi/config.yaml` — global (personal)

Merge rules (consistent with existing EDI config):
- `build_tags`: project replaces global (array replacement, not merge)
- `deny_patterns`: **concatenate** (both project and global patterns apply)
- `failure_loop_threshold`: project overrides global
- `enabled`: project overrides global

The binary reads `cwd` from the hook input JSON, not `os.Getwd()`, since the hook may be invoked from a different directory.

## Hook Registration

### Settings Generation

New file `edi/internal/launch/hooks.go`, following the pattern of `mcp.go`:

```go
// HooksSettings represents the hooks section of .claude/settings.json
type HooksSettings struct {
	Hooks map[string][]HookMatcherGroup `json:"hooks"`
}

type HookMatcherGroup struct {
	Matcher string        `json:"matcher"`
	Hooks   []HookHandler `json:"hooks"`
}

type HookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}
```

### Functions

```go
// ReadClaudeSettings reads the existing .claude/settings.json
func ReadClaudeSettings(projectDir string) (map[string]interface{}, error)

// UpdateHooksSettings merges edi-guard hook entries into .claude/settings.json
// preserving any existing non-edi hooks
func UpdateHooksSettings(projectDir string, cfg *config.Config) error
```

### Merge Strategy

1. Read existing `.claude/settings.json` (or `{}` if absent)
2. Read `hooks` key (or `{}` if absent)
3. For each event (`PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PreCompact`):
   - Find or create the matcher group
   - Find or create the `edi-guard` handler entry (identified by command path containing `edi-guard`)
   - Update the command path
4. Preserve all other hook entries (user-defined hooks, plugin hooks)
5. Write back

### Generated Config

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": ".*",
        "hooks": [
          { "type": "command", "command": "~/.edi/bin/edi-guard" }
        ]
      }
    ]
  }
}
```

### Integration Point

In `edi/internal/cli/launch.go`, add after the MCP config update:

```go
// Update hook configuration for edi-guard
if cfg.Guard.Enabled {
    if err := launch.UpdateHooksSettings(cwd, cfg); err != nil {
        if verbose {
            fmt.Fprintf(os.Stderr, "Warning: failed to update hooks config: %v\n", err)
        }
    }
}
```

## Binary Architecture

### Entry Point

`edi/cmd/edi-guard/main.go`

### Input Parsing

```go
type HookInput struct {
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Error         string          `json:"error"`
	IsInterrupt   bool            `json:"is_interrupt"`
	Trigger       string          `json:"trigger"`
}

type BashToolInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}
```

Use `json.RawMessage` for `tool_input` since its shape varies by tool. Only parse as `BashToolInput` when `tool_name == "Bash"`.

Read stdin with `io.LimitReader(os.Stdin, 1<<20)` (1MB limit, same as `task-sync-hook`).

### Event Dispatch

```go
func main() {
    input := parseStdin()
    cfg := loadGuardConfig(input.CWD)

    if !cfg.Enabled {
        os.Exit(0)
    }

    switch input.HookEventName {
    case "PreToolUse":
        handlePreToolUse(input, cfg)
    case "PostToolUse":
        handlePostToolUse(input)
    case "PostToolUseFailure":
        handlePostToolUseFailure(input)
    case "PreCompact":
        handlePreCompact(input, cfg)
    default:
        os.Exit(0) // Unknown event, pass through
    }
}
```

### Execution Order Within PreToolUse

1. **Deny-list check** (exit 2 if match — stops all further processing)
2. **Build tag injection** (may modify command)
3. **Failure counter read** (may add advisory context)

If both build tag injection and failure advisory apply, output both in the same JSON response:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "toolInput": {
      "command": "go test -tags fts5 ./..."
    },
    "additionalContext": "edi-guard: 5 consecutive failures detected..."
  }
}
```

### State File Operations

```go
type GuardState struct {
	ConsecutiveFailures int    `json:"consecutive_failures"`
	Advised             bool   `json:"advised"`
	LastFailureCommand  string `json:"last_failure_command"`
	LastFailureError    string `json:"last_failure_error"`
}

func stateFilePath(sessionID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("edi-guard-%s.json", sessionID))
}

func readState(sessionID string) GuardState  // returns zero-value GuardState on any error
func writeState(sessionID string, state GuardState)  // best-effort, logs errors to stderr
```

### Error Philosophy

The binary follows the same philosophy as `task-sync-hook`:

1. **Never exit non-zero except for intentional blocks** (exit 2 for deny-list matches)
2. **All I/O failures are silent** — log to stderr, exit 0
3. **Config failures fall back to defaults** — if config is unreadable, skip configurable policies (deny-list, build tags) but still run the failure counter
4. **stdin parse failure → exit 0** — if we can't understand the input, don't interfere

## Build Integration

### Makefile Changes

```makefile
# Build for current platform
build:
	go build $(CGO_TAGS) $(LDFLAGS) -o bin/$(BINARY) ./cmd/edi
	go build $(CGO_TAGS) -o bin/task-sync-hook ./cmd/task-sync-hook
	go build -o bin/edi-guard ./cmd/edi-guard

# Install to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/
	mkdir -p ~/.edi/bin
	cp bin/task-sync-hook ~/.edi/bin/
	cp bin/edi-guard ~/.edi/bin/
```

Note: `edi-guard` does **not** need `-tags fts5` or CGO. It has no SQLite dependency. Pure Go, no CGO.

### Installation Path

`~/.edi/bin/edi-guard` — same directory as `task-sync-hook`.

## Testing Strategy

### Unit Tests

`edi/cmd/edi-guard/main_test.go` — test each policy function in isolation:

| Test | Input | Expected |
|------|-------|----------|
| `TestBuildTagInjection_Missing` | `go test ./...` | `go test -tags fts5 ./...` |
| `TestBuildTagInjection_Present` | `go test -tags fts5 ./...` | no modification |
| `TestBuildTagInjection_CompoundCommand` | `cd codex && go test ./...` | `cd codex && go test -tags fts5 ./...` |
| `TestBuildTagInjection_MakeSkip` | `make test` | no modification |
| `TestBuildTagInjection_AlreadyHasOtherTags` | `go test -tags "fts5,integration" ./...` | no modification |
| `TestDenyList_ForceMain` | `git push --force origin main` | exit 2 |
| `TestDenyList_ShortForce` | `git push -f origin main` | exit 2 |
| `TestDenyList_ForceBranchNotMain` | `git push --force origin feature` | allowed |
| `TestDenyList_RmRfEdi` | `rm -rf .edi` | exit 2 |
| `TestDenyList_RmSingleFile` | `rm .edi/config.yaml` | allowed |
| `TestDenyList_ResetHard` | `git reset --hard` | exit 2 |
| `TestFailureCounter_Increment` | PostToolUseFailure | counter increments |
| `TestFailureCounter_Reset` | PostToolUse | counter resets to 0 |
| `TestFailureCounter_Advisory` | 5 failures then PreToolUse | advisory in output |
| `TestFailureCounter_AdvisoryOnce` | 6 failures, two PreToolUse calls | advisory only on first |
| `TestPreCompact_WritesFile` | PreCompact with task + branch | file written with correct content |
| `TestPreCompact_MissingTask` | PreCompact, no active.yaml | file written, task line omitted |
| `TestPreCompact_MissingGit` | PreCompact, no git repo | file written, branch line omitted |

### Integration Test

One end-to-end test that pipes JSON to the binary's stdin and checks stdout/stderr/exit code. This validates the real binary, not just extracted functions.

```go
func TestIntegration_PreToolUse_DenyList(t *testing.T) {
    // Build binary
    // Pipe PreToolUse JSON with "rm -rf .edi" to stdin
    // Assert exit code 2
    // Assert stderr contains "Blocked"
}
```

### No CGO Dependency

Tests for `edi-guard` do **not** need `-tags fts5`. The binary has zero SQLite dependency. Standard `go test ./cmd/edi-guard/...`.

## File Map

| File | Purpose | Lines (est.) |
|------|---------|-------------|
| `edi/cmd/edi-guard/main.go` | Binary entry point, event dispatch, all four policies | ~200 |
| `edi/cmd/edi-guard/main_test.go` | Unit + integration tests | ~250 |
| `edi/internal/config/schema.go` | Add `GuardConfig` + `DenyPattern` structs | +15 |
| `edi/internal/launch/hooks.go` | Settings.json generation + merge | ~100 |
| `edi/internal/launch/hooks_test.go` | Merge logic tests | ~80 |
| `edi/internal/cli/launch.go` | Add `UpdateHooksSettings` call | +5 |
| `edi/Makefile` | Add `edi-guard` build + install targets | +3 |

Total new code: ~400 lines of Go (half of which is tests).

## What This Does NOT Do

1. **No HTTP server**. No ports, no process lifecycle, no daemon.
2. **No semantic analysis**. No AST parsing, no "is this a destructive query," no code quality judgment.
3. **No conversation access**. The hook doesn't read the transcript. It reads local files (task manifest, config, git).
4. **No RECALL integration**. RECALL problems are internal to the search engine, not solvable from a hook.
5. **No agent mode enforcement**. Mode boundaries are prompt-level concerns, not hook concerns.
6. **No blocking on PostToolUse/PostToolUseFailure/PreCompact**. These events can't block. The hook only blocks on PreToolUse (deny-list).
