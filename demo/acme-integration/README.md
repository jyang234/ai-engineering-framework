# Acme Integration Demo

A self-contained walkthrough that exercises the full EDI lifecycle: init, RECALL retrieval, agent mode switching, implementation, review, Ralph autonomous loop, and session end with knowledge capture.

## Prerequisites

- EDI installed and `edi init --global` completed
- Claude Code CLI (`claude`) in PATH
- `jq` for verification scripts
- No prior state needed — demo is fully isolated

## Quick Start

```bash
cd demo/acme-integration
./setup.sh
```

## Walkthrough

### Step 1: Setup

```bash
./setup.sh
./verify.sh 01-init
```

This initializes a git repo, runs `edi init`, and seeds RECALL with three project knowledge items (two ADRs and meeting notes). The CLI now writes to the same Codex database that EDI sessions read from.

### Step 2: Verify RECALL Seed

```bash
./verify.sh 02-recall-seed
```

Verifies that seeded ADRs and meeting notes were persisted to the Codex database.

### Step 3: Architect Mode

```bash
edi
# Inside EDI session:
#   "Search RECALL for auth decisions" → verify ADRs surface
#   /plan
#   "Design the Acme integration. Check RECALL for existing decisions."
# EDI should reference ADR-001 (OAuth) and ADR-002 (JSON) from RECALL
# Exit the session (Ctrl+C or /end)
./verify.sh 03-architect
```

### Step 4: Implement

```bash
edi
# Inside EDI session:
#   /build
#   "Implement US-001 (OAuth token client) and US-004 (health endpoint)"
#   Ask it to commit when done
# Exit
./verify.sh 04-implement
```

### Step 5: Review

```bash
edi
# Inside EDI session:
#   /review
#   "Review the implementation for security and correctness"
# Exit
./verify.sh 05-review
```

### Step 6: Ralph Loop

```bash
edi ralph --prd docs/PRD.json
# Ralph runs autonomously, implementing remaining tasks
./verify.sh 06-ralph
```

### Step 7: Session End

```bash
edi
# Inside EDI session:
#   /end
#   Save at least one decision to RECALL when prompted
./verify.sh 07-end
```

### Step 8: Results

```bash
./verify.sh summary
cat .demo/RESULTS.md
```

## Recovery

If a step fails:
- Fix the issue manually or re-run the step
- `./verify.sh <step>` can be re-run — results append to the log
- `./teardown.sh` resets everything; `./setup.sh` starts fresh

## What This Proves

- CLI and MCP server share the same Codex database
- RECALL retrieval surfaces relevant project context
- Agent mode switching works (/plan, /build, /review)
- Ralph autonomous loop completes defined tasks
- Knowledge capture round-trips through RECALL
- Deviations are recorded for bug triage
