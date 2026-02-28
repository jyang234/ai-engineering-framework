# AI Engineering Framework (AEF)

AEF gives Claude Code persistent memory, specialized agents, and session continuity. Your AI assistant remembers what it learned.

## What You Get

- **Session continuity (briefings)** — Each session starts with project profile, recent history, and open tasks. Helps with multi-day work so you do not re-explain context. Limited by summarization quality — history entries are only as good as the `/end` summaries you write.
- **Organizational memory (RECALL)** — Stores patterns, decisions, and failures. Searchable across sessions. Only as good as what you capture — EDI prompts you but does not capture automatically. Keyword search works out of the box; semantic search requires Ollama.
- **Specialized agents** — Four modes (coder, architect, reviewer, incident) with different system prompts and priorities. They guide behavior through prompting but do not enforce constraints — Claude can still do whatever it wants.
- **Hybrid search (Codex backend)** — Combines vector similarity with FTS5 keyword matching via RRF fusion. Requires Ollama running locally with nomic-embed-text. Without Ollama, falls back to keyword-only (v0 backend), which still works fine for exact queries.
- **Session memory** — Uses Claude Code's memory tool (`/memories/`) as a session cache for decisions and insights. Important items are promoted to RECALL at session end via `/end`. Project instructions live in `CLAUDE.md` (loaded natively by Claude Code).
- **Evaluation framework** — Measure whether RECALL and skills actually help. The `aef-eval` CLI runs controlled experiments comparing baseline Claude Code against AEF-enhanced sessions. Includes LLM-as-judge scoring, statistical tests, and retrieval quality metrics.
- **Local-first** — Single SQLite file, local embeddings, no API keys for core features. Privacy and offline-capable. Tradeoff: local embedding model (nomic-embed-text) is good but not as strong as cloud embedding APIs.

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

After making changes to agents, skills, or commands in the source:

```bash
cd edi

# Fast: rebuild binary and sync only assets (no config overwrite)
make sync

# Full: clean rebuild and reinitialize everything
make reinstall
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

## Skills

EDI ships with 8 skills that provide specialized guidance to agents:

| Skill | Used By | Purpose |
|-------|---------|---------|
| **edi-core** | All agents | Core EDI behaviors, persona, RECALL integration, task workflows |
| **retrieval-judge** | All agents | Evaluate and filter RECALL search results for relevance |
| **coding** | Coder | Coding standards — error handling, naming, function design |
| **golang-idioms** | Coder | Idiomatic Go patterns from Effective Go |
| **testing** | Coder, Test Writer | Testing standards — table-driven tests, coverage, anti-patterns |
| **scaffolding-tests** | Coder, Test Writer | Golden master / characterization tests for safe refactoring |
| **refactoring-planning** | Architect | Structured methodology for planning and executing refactoring |
| **plan-review** | Reviewer | Review plans for regression risk, complexity, and over-engineering before implementation |

Skills are installed to `~/.claude/skills/` by `edi init --global` and loaded into the system prompt based on each agent's `skills` list. See [edi/README.md](edi/README.md#skills) for detailed usage and examples.

## Evaluation Framework

AEF includes a structured evaluation system for measuring whether RECALL retrieval, skills, and agents actually improve code generation outcomes. The framework is designed to produce evidence, not assumptions.

### What It Measures

| Dimension | How | Tool |
|-----------|-----|------|
| **Retrieval quality** | Precision@K, Recall@K, MRR, NDCG against ground truth | `EvalHarness` |
| **Retrieval filtering** | LLM-as-judge relevance assessment of search results | `JudgeHarness` |
| **Code generation quality** | Correctness, code quality, pitfall avoidance, completeness, efficiency | `Scorer` (LLM judge + tests + lint) |
| **AEF vs. baseline** | Controlled A/B: baseline Claude Code vs. AEF-minimal vs. AEF-full | `aef-eval` CLI |
| **Statistical rigor** | Mann-Whitney U, bootstrap CIs, Wilcoxon signed-rank, Spearman correlation | `stats` module |

### Evaluation Levels

- **Level 1 (Component)** — Implemented. Tests individual components: MCP protocol, indexing, search quality, feedback, flight recorder, audit trail. Run via `EvalHarness`.
- **Level 2 (Integration)** — Implemented. Tests RECALL + skills together in multi-turn agent loops. Uses `AgentRunner` (synthetic Claude agent with tool-use loop) or `PipeRunner` (Claude pipe mode).
- **Level 3 (System)** — Implemented. Compares three conditions (baseline, aef-minimal, aef-full) across experiments. Generates condition comparison tables, claim validation, and scorecards.
- **Level 4 (Comparative)** — Infrastructure ready. Cross-experiment trend analysis using Spearman correlation. Requires experiment data to produce results.

### Running Evaluations

```bash
# Build the eval CLI
cd codex && CGO_ENABLED=1 go build -tags fts5 ./cmd/aef-eval

# Run a retrieval quality evaluation (requires Ollama)
cd codex && go test -tags fts5 -run TestEvalHarness ./eval/

# Run a controlled experiment (pipe mode)
aef-eval run --experiment 3A --condition baseline --task-dir ./tasks --strategy pipe

# Run with AEF skills enabled
aef-eval run --experiment 3A --condition aef-full --task-dir ./tasks --skill-dir ~/.claude/skills/ --strategy pipe

# Run with synthetic agent (multi-turn tool-use loop)
aef-eval run --experiment 3A --condition aef-full --task-dir ./tasks --strategy agent

# Score a completed run
aef-eval score --run-id <id> --task-dir ./tasks

# Generate reports
aef-eval report --experiment 3A --format text
aef-eval report --all --format json

# List available tasks and runs
aef-eval list --tasks --task-dir ./tasks
aef-eval list --runs
aef-eval list --experiments
```

### Task Corpus Format

Evaluation tasks are YAML files in a task directory:

```yaml
id: "error-handler-001"
complexity: "moderate"    # simple, moderate, complex
spec: |
  Implement an HTTP error handler middleware...
pitfalls:
  - id: "swallowed-errors"
    description: "Errors logged but not propagated"
    detection:
      method: grep          # grep, test, or judge
      pattern: "return err"
      files: ["*.go"]
```

See [AEF Evaluation Framework](docs/architecture/aef-evaluation-framework.md) for the full specification, experiment designs, and claim validation methodology.

## Ralph Loop

Ralph is an autonomous execution mode for batch coding tasks. Each iteration starts with a fresh context window, reads the next task from `PRD.json`, implements it, commits, and moves on. State lives in files and git, not in the LLM's memory.

**When it helps**: Well-specified tasks that can be completed independently. API scaffolding from an OpenAPI spec, batch migrations across files, documentation sweeps. Simple tasks complete in 1-2 minutes each; complex tasks may take longer or escalate to human review if Claude encounters blockers.

**When it does not help**: Debugging (needs accumulated context), architecture decisions (needs back-and-forth), security-sensitive code (needs scrutiny), exploratory work (needs adaptive planning). Ralph follows the spec but does not think strategically — it will execute what you wrote, even if what you wrote turns out to be wrong.

**Expected throughput**: Depends on task complexity and spec quality. Simple scaffolding tasks (generate CRUD endpoints, add logging) complete in 1-2 minutes. Complex tasks (refactor with cross-file dependencies, implement state machine) may take 5-10 minutes or escalate to human. If your spec is ambiguous or underspecified, expect escalations.

```bash
# Scaffold a PRD template
edi ralph init
$EDITOR PRD.json

# Run the loop
edi ralph

# Or with options
edi ralph --prd path/to/PRD.json --max-iterations 30
```

Use `/ralph` in an EDI session to author a PRD through a guided interview before execution.

| | Ralph Loop | Continuous Session |
|---|---|---|
| **Best for** | Batch of independent, well-specified tasks | Exploratory, interconnected, design work |
| **Context** | Fresh each iteration | Accumulated across session |
| **State** | PRD.json + git | LLM memory + RECALL |

See [edi/README.md](edi/README.md#ralph-loop) for usage details and [Ralph Loop Specification](docs/architecture/ralph-loop-specification.md) for the full spec.

## Components

| Component | What it does | README |
|-----------|-------------|--------|
| **EDI** | Claude Code harness — agents, briefings, history, session management | [edi/README.md](edi/README.md) |
| **Codex** | Knowledge engine — hybrid search, local embeddings, single-file storage | [codex/README.md](codex/README.md) |

## Project Status

**EDI v0**: Complete. CLI harness with 4 agents, 8 skills, 9 slash commands, 7 subagents, session briefings, stale session recovery, auto memory integration, and Ralph autonomous loop.

**Codex v1**: Substantially complete. Hybrid search (vector KNN + FTS5 + RRF fusion), MCP server with 5 tools, AST chunking (7 languages via Tree-sitter), markdown chunking, web UI, admin CLI, and v0-to-v1 migration. Reranking and contextual chunking are stubbed — search works without them.

**Evaluation**: Infrastructure complete across all 4 levels. `aef-eval` CLI supports run, score, report, and list commands. Statistical analysis (Mann-Whitney U, bootstrap CIs, Wilcoxon, Spearman) is implemented. Experiment data collection is the next step.

## Further Reading

- [EDI + Codex Technical Deep-Dive](docs/edi-codex-deep-dive.md) — full system architecture, data flows, eval framework, and operational guide
- [AEF Evaluation Framework](docs/architecture/aef-evaluation-framework.md) — experiment designs, claim validation, statistical methods
- [AEF Components Overview](docs/aef-components.md)
- [RECALL MCP Server Spec](docs/architecture/recall-mcp-server-spec.md)
- [EDI Session Lifecycle](docs/architecture/edi-session-lifecycle-spec.md)
- [EDI CLI Commands](docs/architecture/edi-cli-commands-spec.md)

## License

MIT
