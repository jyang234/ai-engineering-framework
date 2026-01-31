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

### Updating After Source Changes

When you modify agents, skills, commands, or subagents in the EDI source:

```bash
cd edi

# Sync only — rebuild binary and copy assets to ~/.edi/ and ~/.claude/
# Does NOT touch config.yaml, recall database, or history
make sync

# Full reinstall — clean build + edi init --global --force
# Overwrites everything including config
make reinstall
```

You can also run `edi sync` directly after a manual `make install`:

```bash
# Sync assets without rebuilding
edi sync
```

**Expected output from `edi sync`:**
```
  Synced agents
  Synced commands
  Synced skills (6)
  Synced subagents

Assets synced successfully.
```

## Skills

Skills are detailed guidance documents loaded into the system prompt based on each agent's `skills` list. EDI ships with 6 skills, all installed to `~/.claude/skills/` by `edi init --global` or `edi sync`.

| Skill | Agents | What it does |
|-------|--------|-------------|
| `edi-core` | All | Core EDI behaviors, persona, RECALL integration, task workflows |
| `retrieval-judge` | All | Evaluates RECALL search results for relevance before presenting |
| `coding` | Coder | Coding standards — error handling, naming, function design |
| `testing` | Coder, Test Writer | Testing standards — table-driven tests, coverage, anti-patterns |
| `scaffolding-tests` | Coder, Test Writer | Golden master / characterization tests for safe refactoring |
| `refactoring-planning` | Architect | Structured methodology for planning and executing refactoring |

### EDI Core

Loaded by every agent and subagent. Defines the EDI persona (formal tone, no contractions, constructive push-back), and provides integration patterns for RECALL and the task system.

**Key behaviors it enables:**
- Query RECALL before starting significant work
- Log decisions to the flight recorder with rationale
- Propagate decisions to dependent tasks
- Surface deviations from plans immediately with structured options
- Maintain a components registry for multi-component projects

**Example — RECALL query before implementation:**
```
recall_search({query: "payment retry logic after provider timeout", types: ["pattern", "failure"]})
```

**Example — logging a decision:**
```
flight_recorder_log({
  type: "decision",
  content: "Using Stripe for payments",
  rationale: "Best Go SDK, team familiarity",
  metadata: {propagate: true}
})
```

### Retrieval Judge

Loaded by every agent. Prevents EDI from blindly trusting RECALL search results. After every `recall_search` call, the agent must evaluate each result for relevance, log a judgment, and show a summary.

**What it enforces:**
- Evaluate title, content, and applicability of each result
- Discard keyword-adjacent results that don't address the actual query
- Log kept/dropped results with reasoning to the flight recorder
- Show a one-line summary: `RECALL: 3/7 results kept for "payment retry timeout"`

**Example — poor query vs. good query:**
```
# Bad: too vague, returns noise
recall_search({query: "retry"})

# Good: specific domain, technology, and concern
recall_search({query: "payment retry logic after provider timeout"})
```

### Coding

Loaded by the coder agent. Provides Go-focused coding standards covering function design, error handling, naming, and package organization.

**Key standards:**
- Functions < 30 lines, < 4 parameters, context first, error last
- Accept interfaces, return structs
- Handle errors immediately — never `_` ignore
- Use sentinel errors (`var ErrNotFound = errors.New(...)`) with `errors.Is`/`errors.As`
- One change type per commit — bug fixes don't include refactoring

**Example — error handling pattern:**
```go
user, err := db.Find(ctx, userID)
if err != nil {
    return nil, fmt.Errorf("find user %s: %w", userID, err)
}
```

**Example — scope discipline:**
```
Original task: Fix discount calculation bug
Observed: OrderService could use refactoring

Correct action:  Fix the bug only. Note refactoring opportunity separately.
Incorrect action: Refactor while fixing bug.
```

### Testing

Loaded by the coder agent and test-writer subagent. Defines testing philosophy and Go-specific practices.

**Key standards:**
- Write edge case and error path tests **first** — they catch real bugs
- Table-driven tests with descriptive names
- `t.Parallel()` unless shared state, `t.Helper()` on all helpers, `t.TempDir()` not `os.MkdirTemp`
- Use `errors.Is`/`errors.As` for error assertions, not string matching
- Never change assertions to match broken behavior — escalate unexpected failures

**Example — table-driven test:**
```go
tests := []struct {
    name    string
    amount  int64
    wantErr error
}{
    {"valid", 100, nil},
    {"negative", -1, ErrInvalidAmount},
    {"zero", 0, ErrInvalidAmount},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        _, err := svc.Charge(card, tt.amount)
        if !errors.Is(err, tt.wantErr) {
            t.Errorf("got %v, want %v", err, tt.wantErr)
        }
    })
}
```

**Anti-patterns it catches:** testing unexported details, `time.Sleep` in tests, `os.Chdir`, missing `t.Helper()`, shared mutable state between tests.

### Scaffolding Tests

Loaded by the coder agent and test-writer subagent. Teaches EDI to generate characterization tests (golden master tests) that capture current behavior before refactoring.

**When to use:** Before any refactoring that modifies code paths, especially when existing test coverage is insufficient.

**Workflow:**
1. **Identify** components to scaffold (public API surface, high fan-in functions, data boundaries)
2. **Generate** representative inputs (happy path, boundaries, errors, unicode)
3. **Capture** current outputs as golden master files
4. **Refactor** code, running scaffolding tests after each change
5. **Cleanup** — replace scaffolding with proper unit tests, delete golden files

**Commands:**
```bash
# Generate golden files (first run)
UPDATE_GOLDEN=1 go test -v -run TestScaffold_ ./...

# Verify behavior preserved after refactoring
go test -v -run TestScaffold_ ./...

# Cleanup after refactoring is complete
rm -rf testdata/golden/ *_scaffold_test.go
```

**Naming conventions:**

| Element | Convention |
|---------|------------|
| Test file | `{component}_scaffold_test.go` |
| Test function | `TestScaffold_{Component}_{Operation}` |
| Golden directory | `testdata/golden/{component}/` |
| Golden file | `{test_case}.golden.json` |

**Key distinction from regular tests:** Scaffolding tests assert *sameness* (behavior unchanged), not *correctness* (behavior is right). They are temporary — delete them after the refactoring is complete.

### Refactoring Planning

Loaded by the architect agent. Provides a structured methodology for planning refactoring work before execution begins.

**When to use:** Any refactoring that touches multiple files, changes architectural patterns, or modifies public interfaces.

**Workflow:**
1. **Define** the refactoring goal, target state, and explicit scope boundary
2. **Map** all affected code (directly modified + indirectly affected callers + test files)
3. **Identify** core components that need scaffolding tests vs. those that can be skipped
4. **Assess** risk and complexity for each component
5. **Design** a phased migration path (scaffold → extract → refine → cleanup)
6. **Output** a refactoring spec (structured YAML) that the coder agent can execute

**What to scaffold vs. skip:**

| Always Scaffold | Skip |
|----------------|------|
| Public API surface | Internal helpers with single caller |
| High fan-in functions | Code with existing comprehensive tests |
| Data boundaries (DB, API, file I/O) | Code being deleted |
| Complex business logic | Pure data structures |

**Example — using both skills together:**
```
You: /plan
EDI: Switched to architect mode.

You: I want to extract business logic from our HTTP handlers into service objects.

EDI: [Uses refactoring-planning skill to produce a refactoring spec with
     impact map, risk assessment, and phased migration path]

You: /build
EDI: Switched to coder mode.

You: Let's start with scaffolding tests for the user handlers.

EDI: [Uses scaffolding-tests skill to generate golden master tests,
     capturing current behavior before any code changes]

You: [Refactors handler, moves logic to UserService]

EDI: [Runs scaffolding tests to verify behavior is preserved]
```

## Ralph Loop

Ralph is an autonomous execution mode for running well-defined coding tasks in a loop. Each iteration starts with a fresh context window — no accumulated cruft, full attention on one task.

### When to Use

- Batch of independent, well-specified tasks with clear acceptance criteria
- Executing a reviewed and complete spec (the planning is done)
- Grunt work where focus > creativity

Don't use Ralph for exploratory work, architecture decisions, debugging, or multi-file refactors needing consistent vision.

### How to Invoke

```bash
# Copy files to your project
cp ~/.edi/ralph/ralph.sh .
cp ~/.edi/ralph/PROMPT.md .
cp ~/.edi/ralph/example-PRD.json PRD.json  # edit this

# Or symlink
ln -s ~/.edi/ralph/ralph.sh .
ln -s ~/.edi/ralph/PROMPT.md .

# Run
./ralph.sh
```

### PRD.json Format

Tasks are defined in `PRD.json` with dependencies:

```json
{
  "project": "my-project",
  "description": "What this project does",
  "userStories": [
    {
      "id": "US-001",
      "title": "Project setup",
      "description": "Full description of what to implement",
      "criteria": ["Go module initialized", "Makefile with build target"],
      "passes": false
    },
    {
      "id": "US-002",
      "title": "Health endpoint",
      "description": "Create /health endpoint",
      "criteria": ["GET /health returns 200"],
      "passes": false,
      "depends_on": ["US-001"]
    }
  ]
}
```

### Escalation

Ralph escalates to the human when:

- **Stuck** — same error 3+ times, blocked by external factors, doesn't know how to proceed
- **Deviation** — spec appears wrong, scope larger than expected, needs out-of-scope changes, security concern

When escalation fires, you choose: select a numbered option, provide custom guidance, skip the task, retry, or abort.

### The Planning → Execution → Capture Flow

RECALL is for the **planning** phase, not the Ralph execution loop. The correct flow:

1. **Plan** (interactive session, uses RECALL) — query patterns, design, write `PRD.json` with complete specs
2. **Execute** (Ralph loop, no RECALL) — implement exactly what spec says, escalate if wrong
3. **Capture** (post-execution) — review completed work, save new patterns/failures to RECALL

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_ITERATIONS` | 50 | Maximum loop iterations |
| `STUCK_THRESHOLD` | 3 | Consecutive errors before auto-escalate |

### References

- [Ralph Loop Specification](../docs/architecture/ralph-loop-specification.md) — full technical spec
- [ADR: Ralph Wiggum Integration](../docs/architecture/adr-ralph-wiggum-integration.md) — design decision record

### Real-World Use Cases

**Works well:**

- **API scaffolding from spec** — "Implement 5 REST endpoints per the OpenAPI spec." Each endpoint is independent, criteria are clear (correct routes, request/response shapes, status codes). Ralph completes these methodically without context bleed.
- **Migration batch** — "Update 12 database queries from raw SQL to sqlc generated code." Repetitive, mechanical, well-defined. Each query is independent.
- **Documentation sweep** — "Add godoc comments to all exported functions in pkg/." Low-risk, clear acceptance criteria, no design decisions.

**Does not work well:**

- **Debugging a flaky test** — Requires investigation, hypothesis testing, and accumulated context. Ralph's fresh start each iteration means it cannot build on previous debugging insights.
- **Refactoring a module** — Interconnected changes across files need consistent vision. Ralph sees one task at a time and cannot maintain architectural coherence across iterations.
- **Security-sensitive code** — Needs threat model awareness and holistic review. Ralph focuses narrowly on acceptance criteria.

### Expected Behavior and Outcomes

- Each iteration invokes `claude -p` (pipe mode) with a built prompt containing the task details and PROMPT.md instructions. Typical iteration: 1-3 minutes for straightforward tasks.
- Escalation rate depends on spec quality. Well-specified tasks with clear criteria: ~5-10% escalation. Vague specs with ambiguous criteria: 30%+ escalation. Write better specs to reduce escalation.
- Completion detection looks for explicit signals ("Task US-001 complete") in Claude's output. All-done detection uses `<promise>DONE</promise>`. If Claude does not produce these signals, the iteration counts as incomplete and the loop retries.
- Git commits happen after each completed task. If the loop is interrupted, progress is preserved in git.

### Troubleshooting

Common issues and their causes:

- **Vague acceptance criteria → false completion** — Ralph marks tasks complete when it sees "Task X complete" in output. If criteria are vague, Claude may declare completion prematurely. Fix: write specific, verifiable criteria ("GET /users returns 200 with JSON array" not "users endpoint works").
- **Circular dependencies → immediate exit** — If PRD.json has circular `depends_on` references, no task will be eligible and the loop exits immediately with "All Tasks Complete" despite nothing being done. Fix: check your dependency graph.
- **Environmental errors → auto-escalation** — Missing tools (jq, claude CLI), permissions issues, or network problems trigger repeated identical errors. After 3 consecutive identical errors (configurable via STUCK_THRESHOLD), Ralph auto-escalates.
- **Underspecified PRD → Claude fills gaps** — When the description is sparse, Claude makes assumptions. These assumptions may not match your intent. Ralph will not catch this — it only checks for completion signals. Fix: front-load the detail in planning phase.
- **No completion signal detected** — If Claude's output does not match the completion patterns, the task is not marked done and the loop retries the same task. This burns iterations. Check `.ralph/output_N.txt` to see what Claude actually said.

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
