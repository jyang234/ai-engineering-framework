# AEF Evaluation Framework

> Design date: 2026-02-24
> Purpose: Definitively evaluate AEF's viability and performance, replacing self-assessed claims with measured evidence

---

## Why This Evaluation Exists

The AEF documentation makes several claims that are not backed by measured data:

| Claim | Location | Problem |
|---|---|---|
| "Methodology / Judgment Skills: 10/10" | coding-standards-review.md:1182 | Self-assessed; "no comparable system exists" is unfalsifiable |
| "Knowledge Integration (RECALL): 10/10" | coding-standards-review.md:1183 | Self-assessed; no empirical comparison |
| "Deepest methodology of any system in the field" | coding-standards-review.md:1191 | No systematic comparison framework |
| "Compound value from integrations" | recall-knowledge-system-analysis.md §11 | Architectural argument, not measured outcome |
| "Organizational memory that improves with use" | recall-knowledge-system-analysis.md:505 | No definition of "improves" or measurement methodology |
| "+15-30% recall improvement" (hybrid search) | recall-mcp-server-spec.md:94 | Unattributed; no test set or baseline cited |
| External research applied to AEF | coding-standards-review.md Part 8 | Qodo, ICSME, METR findings cited as evidence for AEF, but AEF was not tested |
| "Hooks increased skill activation ~20% to ~84%" | coding-standards-review.md:1004 | alexop.dev result, not AEF's result |

This evaluation framework is designed to replace every one of these claims with measured evidence — or invalidate them.

---

## Evaluation Principles

1. **Baseline everything.** No claim has meaning without a comparison. Every experiment compares AEF against vanilla Claude Code doing the same task.

2. **Measure what matters, not what's easy.** Retrieval metrics (NDCG, MRR) matter for RECALL internals. For AEF's overall value proposition, what matters is: Did the code work? Was it correct? Did it avoid known failures?

3. **Reproducibility over impressiveness.** Small, repeatable experiments with clear pass/fail criteria beat large, noisy benchmarks.

4. **Separate component claims from system claims.** RECALL's retrieval quality is a component claim. "AEF improves engineering outcomes" is a system claim. Different experiments validate each.

5. **Honest reporting.** If AEF doesn't help, that's a finding. The evaluation framework must be designed so it can produce a negative result.

---

## Evaluation Levels

```
Level 4: Comparative       Does AEF beat baseline Claude Code on real tasks?
Level 3: System            Does AEF improve task completion quality end-to-end?
Level 2: Integration       Do RECALL + retrieval-judge + plan-review work together?
Level 1: Component         Does RECALL retrieve well? Do hooks fire? Do skills load?
```

Level 1 is partially covered by the existing EvalHarness and JudgeHarness. Levels 2-4 are entirely unproven.

---

## Level 1: Component Evaluation

### What Exists

The `codex/eval/` directory contains:

| Component | Harness | Status |
|---|---|---|
| RECALL retrieval quality | EvalHarness (8-phase pipeline) | Implemented; PayFlow benchmarks recorded |
| LLM judge filtering | JudgeHarness (Claude Sonnet) | Implemented; thresholds defined |
| MCP protocol correctness | EvalHarness Phase 1-2 | Implemented |
| Feedback mechanism | EvalHarness Phase 6 | Implemented |
| Flight recorder logging | EvalHarness Phase 7-8 | Implemented |

**Existing benchmark results** (from `codex/eval/benchmarks_local_nomic.txt`):
- Recall@5: 0.829–0.830
- Recall@10: 0.908–0.910
- nDCG@10: 0.762–0.784
- MRR: 0.782–0.842

### What's Missing at Level 1

#### 1A. Hook Execution Verification

**Question**: Do the proposed hooks actually fire when expected?

**Experiment**:
1. Configure a test project with the hooks from coding-standards-review.md Part 6
2. Execute a scripted sequence of Claude Code actions:
   - Write a `.go` file with formatting issues → verify PostToolUse `gofumpt` fires
   - Write to a `.env` file → verify PreToolUse `protect-files.sh` blocks it
   - Edit MEMORY.md managed section → verify PreToolUse `protect-memory.sh` blocks it
   - Run `rm -rf /` via Bash → verify PreToolUse `block-dangerous-commands.sh` blocks it
   - Complete a task → verify Stop `verify-quality.sh` runs
   - Start a session → verify SessionStart `generate-briefing.sh` produces output

**Metrics**:
- Hook fire rate: % of expected triggers that actually fire
- False positive rate: % of legitimate actions incorrectly blocked
- False negative rate: % of violations not caught

**Pass criteria**: Fire rate ≥ 95%, false positive rate ≤ 5%, false negative rate ≤ 5%.

**Infrastructure needed**: A test harness that programmatically drives Claude Code tool calls and observes hook behavior. This can be built by extending the existing MCP client pattern in `codex/eval/mcpclient.go` to include a Claude Code tool-call simulator.

#### 1B. Skill Loading Verification

**Question**: Are skills loaded and influencing agent behavior?

**Experiment**:
1. Run Claude Code with AEF skills loaded, give it 20 standardized prompts
2. Run Claude Code without AEF skills, give it the same 20 prompts
3. Score responses against skill-specific behavioral markers:
   - Does the agent query RECALL before significant work? (edi-core)
   - Does the agent evaluate search results before using them? (retrieval-judge)
   - Does the agent log decisions to flight recorder? (edi-core)
   - Does the agent follow the pre-flight checklist before writing tests? (testing)
   - Does the agent assess YAGNI and blast radius in plan review? (plan-review)

**Metrics**:
- Behavioral adherence rate: % of expected behaviors observed (with skills vs without)

**Pass criteria**: With-skills adherence ≥ 70%. Without-skills adherence should be significantly lower (demonstrating skills have causal impact).

**Infrastructure needed**: LLM-as-judge scoring of behavioral markers. Extend `codex/eval/judge.go` with a `BehaviorJudge` that evaluates Claude's responses against expected behavioral patterns.

#### 1C. v0 vs Codex Backend Comparison

**Question**: Does the Codex backend actually outperform v0 on the PayFlow collection?

**Experiment**:
1. Run EvalHarness against v0 backend (SQLite FTS5 only)
2. Run EvalHarness against Codex backend (hybrid vector + BM25)
3. Compare all metrics: Recall@K, Precision@K, nDCG@10, MRR

**Metrics**: Per-category nDCG breakdown (semantic, keyword, hybrid-advantage).

**Pass criteria**: Codex should outperform v0 on "semantic" and "hybrid-advantage" categories. If it doesn't, the Codex investment is not justified.

**Infrastructure needed**: Already exists. Run `TestE2E` against both backends and compare reports.

---

## Level 2: Integration Evaluation

### 2A. Retrieval-Judge Filtering Quality

**Question**: Does the retrieval-judge skill actually improve result quality, or does it just discard results randomly?

**Experiment**:
1. Run 20 PayFlow queries through RECALL
2. For each query, record:
   - Raw results (what RECALL returned)
   - Judge-filtered results (what retrieval-judge kept)
   - Ground truth relevant results
3. Compute metrics for both raw and judge-filtered result sets

**Metrics**:
- Raw precision@5 vs judge precision
- Judge filtering rate (% of results dropped)
- Judge F1 (balancing precision and recall)
- False filtering rate: % of ground-truth relevant results that the judge incorrectly dropped

**Pass criteria**:
- Judge precision > raw precision@5 by ≥ 0.10
- False filtering rate ≤ 15% (the judge should not discard correct results)
- Filtering rate between 20% and 60% (filtering too little = no value; filtering too much = destroying recall)

**Infrastructure needed**: Partially exists in `codex/eval/judge.go`. Extend to compute false filtering rate.

### 2B. RECALL + Plan Review Cross-Reference

**Question**: Does querying RECALL during plan review actually surface relevant past failures?

**Experiment**:
1. Seed RECALL with 10 known failures from the PayFlow domain (e.g., "double-charge on retry without idempotency key", "race condition in settlement reconciliation")
2. Present 10 architectural plans to the plan-review skill:
   - 5 plans that would re-introduce a known failure
   - 5 plans that are safe
3. Record whether plan-review surfaces the relevant failure for each dangerous plan

**Metrics**:
- Detection rate: % of dangerous plans where the relevant failure was surfaced
- False alarm rate: % of safe plans incorrectly flagged

**Pass criteria**: Detection rate ≥ 60%. False alarm rate ≤ 30%.

**Infrastructure needed**: A test collection of (plan, known-failure, expected-detection) tuples. A harness that invokes plan-review with RECALL pre-seeded and scores the output.

### 2C. RECALL + Task Management Context Persistence

**Question**: Does RECALL context attached to tasks actually get used when the task is picked up?

**Experiment**:
1. Create 5 tasks with RECALL context annotations (patterns, decisions)
2. In a new session, pick up each task
3. Measure whether the agent references the RECALL context from the annotation
4. Compare against a control where tasks have no RECALL annotations

**Metrics**:
- Context utilization rate: % of annotated context items referenced during task execution
- Task completion quality score (LLM-judged, comparing annotated vs unannotated)

**Pass criteria**: Context utilization rate ≥ 50%. Annotated tasks should score higher on quality.

**Infrastructure needed**: Scripted task creation/pickup workflow. LLM judge for quality scoring.

### 2D. Flight Recorder Audit Trail Completeness

**Question**: Does the flight recorder actually capture a usable audit trail?

**Experiment**:
1. Run 10 complete session workflows (task pickup → implementation → completion)
2. For each session, verify the flight recorder contains:
   - At least one `retrieval_query` entry per `recall_search` call
   - At least one `retrieval_judgment` entry per `recall_search` call
   - At least one `decision` entry per significant code change
3. Verify that audit trail entries can reconstruct the reasoning chain

**Metrics**:
- Query coverage: % of `recall_search` calls with matching `retrieval_query` entries
- Judgment coverage: % of `recall_search` calls with matching `retrieval_judgment` entries
- Decision coverage: % of significant changes with `decision` entries

**Pass criteria**: Query coverage = 100% (automatic). Judgment coverage ≥ 70%. Decision coverage ≥ 50%.

**Infrastructure needed**: Session replay harness that drives a full workflow and queries the flight_recorder table afterward.

---

## Level 3: System Evaluation

### 3A. Defect Rate Comparison (The Core Claim)

**Question**: Does AEF actually reduce defects compared to vanilla Claude Code?

This is the most important experiment. If AEF doesn't reduce defects, the entire system is not justified.

**Experiment design**:

**Task corpus**: 30 Go implementation tasks of varying complexity:
- 10 simple (single function, clear specification)
- 10 moderate (multi-file change, requires understanding existing code)
- 10 complex (architectural change, requires understanding system context)

Each task has:
- A specification (what to implement)
- A test suite (automated pass/fail)
- Known pitfalls (traps that a naive implementation would fall into)
- Pre-seeded RECALL items that would help avoid the pitfalls

**Conditions**:
- **Baseline**: Vanilla Claude Code, no skills, no hooks, no RECALL. Same model, same context window.
- **AEF-minimal**: Skills + hooks only, no RECALL.
- **AEF-full**: Skills + hooks + RECALL pre-seeded with relevant patterns and failures.

**Procedure** (per task, per condition):
1. Present the task specification to Claude Code
2. Let it implement (with a turn limit to prevent infinite loops)
3. Run the test suite
4. Run golangci-lint
5. Score with LLM judge for:
   - Correctness (tests pass)
   - Code quality (lint violations)
   - Pitfall avoidance (did it fall into known traps?)
   - Completeness (were all requirements addressed?)

**Metrics**:
- Test pass rate: % of tasks where all tests pass
- Lint-clean rate: % of tasks with zero golangci-lint violations
- Pitfall avoidance rate: % of known pitfalls avoided
- Completion rate: % of requirements addressed
- Combined quality score: Weighted aggregate of above

**Pass criteria for AEF viability**:
- AEF-full must outperform Baseline on pitfall avoidance rate by ≥ 20 percentage points
- AEF-full must outperform Baseline on combined quality score by ≥ 10 percentage points
- AEF-minimal must outperform Baseline on lint-clean rate by ≥ 30 percentage points (hooks)
- If AEF-full does not outperform AEF-minimal on any metric, RECALL is not adding value

**Sample size justification**: 30 tasks × 3 conditions = 90 runs. With 10 tasks per complexity level, we can detect a 20-point improvement with reasonable confidence. For higher confidence, repeat each run 3 times (270 total runs).

**Infrastructure needed**:
- Task corpus with specifications, tests, and pitfall annotations
- Automated runner that invokes Claude Code under each condition
- Scoring pipeline (test runner + linter + LLM judge)
- Results database with per-task, per-condition, per-run metrics

### 3B. Repeat Failure Prevention (RECALL's Core Value)

**Question**: Does RECALL actually prevent teams from making the same mistake twice?

**Experiment design**:

**Setup**:
1. Run Claude Code (AEF-full) through Task A, which has a known pitfall
2. Let it fail (or succeed after difficulty)
3. At session end, capture the failure/pattern to RECALL via `recall_add`
4. In a new session, run Claude Code through Task B, which has the same pitfall in a different context
5. Measure whether RECALL knowledge prevents the repeat failure

**Task pairs** (10 pairs, each sharing a pitfall pattern):
- Pair 1: Missing idempotency in payment handler → Missing idempotency in webhook handler
- Pair 2: Race condition in concurrent map access → Race condition in channel-based pipeline
- Pair 3: Nil pointer from uninitialized config → Nil pointer from missing optional field
- ...etc.

**Conditions**:
- **With RECALL**: Session B has access to the failure captured in Session A
- **Without RECALL**: Session B starts fresh, no access to Session A knowledge

**Metrics**:
- Repeat failure rate (with RECALL): % of Task B instances that hit the same pitfall
- Repeat failure rate (without RECALL): % of Task B instances that hit the same pitfall
- Prevention delta: difference between the two rates

**Pass criteria**: Prevention delta ≥ 25 percentage points (RECALL should prevent at least 25% more repeat failures than the control).

**Infrastructure needed**:
- 10 task pairs with shared pitfall patterns
- Two-session runner (Session A → capture → Session B)
- Pitfall detection scoring (automated test + LLM judge)

### 3C. Session Knowledge Accumulation

**Question**: Does AEF's knowledge actually improve over multiple sessions (the "improves with use" claim)?

**Experiment design**:

**Setup**: A 5-session longitudinal experiment on a single project:
- Session 1: Implement feature A (baseline, empty RECALL)
- Session 2: Implement feature B (RECALL has Session 1 learnings)
- Session 3: Implement feature C (RECALL has Sessions 1-2 learnings)
- Session 4: Implement feature D (RECALL has Sessions 1-3 learnings)
- Session 5: Implement feature E (RECALL has Sessions 1-4 learnings)

Each session has a specification, test suite, and quality scoring.

**Control**: Run the same 5 sessions without RECALL (fresh context each time).

**Metrics**:
- Quality score per session (test pass rate + lint-clean rate + pitfall avoidance)
- Quality trajectory: Is the slope positive? (Does quality improve across sessions?)
- Control trajectory: Compare slope with vs without RECALL

**Pass criteria**:
- AEF quality trajectory should have positive slope (quality increases over sessions)
- Control trajectory should be flat or negative (quality doesn't improve without persistent memory)
- Difference in slope should be statistically significant (p < 0.05 with repeated trials)

**Infrastructure needed**:
- A coherent 5-feature project where each session builds on the last
- Session runner that preserves RECALL state across sessions
- Quality scoring pipeline applied to each session's output

### 3D. Hook Adherence Rate (Validating the alexop.dev Claim)

**Question**: Do AEF's hooks actually improve protocol adherence the way alexop.dev's TDD hooks did?

**Experiment design**:

**Setup**:
1. Define 10 protocol requirements (from EDI skills):
   - Query RECALL before significant work
   - Run tests before marking task complete
   - Log decisions to flight recorder
   - Apply retrieval-judge to search results
   - Follow pre-flight checklist before writing tests
   - Present alternatives for significant decisions
   - Update status.md at session end
   - Never modify EDI-managed MEMORY.md sections
   - Format Go files before committing
   - Run golangci-lint before task completion

2. Run 20 sessions under two conditions:
   - **Prompts only**: Skills loaded, no hooks
   - **Prompts + hooks**: Skills loaded, hooks configured

3. For each session, score adherence to each of the 10 protocols

**Metrics**:
- Per-protocol adherence rate (prompts-only vs prompts+hooks)
- Overall adherence rate (average across 10 protocols)
- Improvement delta per protocol

**Pass criteria**:
- Overall adherence (prompts+hooks) ≥ 70%
- Overall adherence (prompts-only) should be measurably lower
- Improvement delta ≥ 20 percentage points overall

**If the alexop.dev claim holds for AEF**: We should see adherence go from ~20-40% (prompts-only) to ~60-85% (prompts+hooks). If the improvement is smaller, the claim doesn't transfer.

**Infrastructure needed**: Adherence scoring (combination of automated checks and LLM judge). 20 sessions per condition × 2 conditions = 40 session runs.

---

## Level 4: Comparative Evaluation

### 4A. AEF vs Baseline on SWE-bench-style Tasks

**Question**: Does AEF improve task resolution rates on standardized benchmarks?

**Experiment design**:

Rather than running full SWE-bench (expensive, Python-focused), create an **AEF-bench**: a curated set of 50 Go tasks drawn from real open-source issues, graded by difficulty.

**Task categories** (10 per category):
- **Bug fixes**: Reproduce and fix a reported bug
- **Feature additions**: Implement a new feature from a specification
- **Refactoring**: Improve code quality without changing behavior
- **Test writing**: Write comprehensive tests for existing code
- **Architecture**: Make a cross-cutting change (e.g., add error wrapping throughout)

Each task has:
- A git repository at a specific commit
- A task description (issue text)
- Automated validation (test suite that passes only with correct implementation)
- Known pitfalls (seeded into RECALL for AEF-full condition)
- Difficulty rating (1-5)

**Conditions**:
- **Baseline**: Vanilla Claude Code
- **AEF-full**: Skills + hooks + RECALL

**Metrics**:
- Resolution rate: % of tasks where validation passes
- Resolution rate by category
- Resolution rate by difficulty
- Average turns to resolution
- Defect density in resolved tasks (golangci-lint findings)

**Pass criteria**:
- AEF-full resolution rate > Baseline by ≥ 10 percentage points overall
- AEF-full should show largest advantage on "architecture" and "refactoring" categories (where organizational knowledge matters most)
- AEF-full should show negligible advantage on "bug fixes" of difficulty 1-2 (simple tasks where methodology overhead isn't justified)

**Infrastructure needed**:
- AEF-bench task corpus (50 tasks with repos, descriptions, validators)
- Automated benchmark runner
- Results aggregation and statistical analysis

### 4B. Comparative System Analysis (Replacing "No Comparable System")

**Question**: How does AEF compare to other AI engineering frameworks on defined dimensions?

The scorecard in coding-standards-review.md:1178-1191 claims "no comparable system exists" for methodology and RECALL. This experiment replaces that claim with a structured comparison.

**Systems to compare** (from the coding-standards-review's own references):
1. **Vanilla Claude Code** — baseline (CLAUDE.md + manual instructions)
2. **Claude Code + custom hooks** — Claude Pilot-style hook pipeline
3. **AEF-full** — skills + hooks + RECALL
4. **Cursor / Windsurf** — IDE-integrated AI (if comparable tasks can be constructed)

**Dimensions** (12, scored 0-10 by evaluators):

| Dimension | What It Measures | How to Score |
|---|---|---|
| Protocol adherence | Does the agent follow defined processes? | Automated: count adherence to 10 defined protocols per session |
| Defect prevention | Does the system reduce defects? | Automated: golangci-lint findings + test failures |
| Knowledge persistence | Does knowledge survive across sessions? | Automated: repeat-failure prevention rate (Experiment 3B) |
| Failure avoidance | Does past knowledge prevent future mistakes? | Automated: pitfall avoidance rate (Experiment 3A) |
| Decision auditability | Can you trace why decisions were made? | Manual review: are flight recorder entries useful? |
| Methodology depth | How many engineering practices are encoded? | Manual: count and categorize encoded practices |
| Hook coverage | What % of mechanical standards are enforced? | Automated: hook fire rate × coverage of coding standards |
| Onboarding speed | How quickly does a new developer get productive? | Timed experiment: new dev completes 5 tasks with each system |
| Context resilience | Does behavior degrade over long sessions? | Automated: compare quality of early vs late tasks in a 10-task session |
| Customizability | How easily can teams adapt the system? | Manual: time to add a new skill/hook/rule |
| Overhead | How much does the system slow down simple tasks? | Automated: average turns for simple tasks (baseline vs AEF) |
| Cost efficiency | What's the token/API cost per task? | Automated: measure total tokens consumed per condition |

**Procedure**:
1. Run each system through the same 20-task sequence
2. Compute automated metrics for each dimension
3. Have 3 independent evaluators score manual dimensions
4. Compute inter-rater reliability (Krippendorff's alpha)
5. Produce a scorecard with confidence intervals

**Pass criteria**: AEF should score highest on knowledge persistence, failure avoidance, decision auditability, and methodology depth. It should NOT score highest on overhead or cost efficiency (methodology has a cost). If AEF scores below baseline on any dimension, that's a finding to report and address.

**Infrastructure needed**: Multi-system benchmark runner. Common task corpus. Evaluator rubric and scoring interface.

---

## Task Corpus Design

The evaluation framework depends on task corpora at multiple levels. Here's how to build them.

### Corpus 1: PayFlow Extended (Level 1-2)

Extend the existing PayFlow collection from 30 documents / 20 queries to include:
- **10 failure items**: Real failure patterns with symptoms, root causes, fixes
- **10 decision items**: ADRs with alternatives and consequences
- **10 plan-review scenarios**: Architectural plans (5 safe, 5 that re-introduce known failures)

This enables Experiments 2A, 2B without building new infrastructure.

### Corpus 2: Go Task Suite (Level 3)

30 Go implementation tasks across 3 complexity levels. Each task is a self-contained git repository with:

```
task-{id}/
├── README.md           # Task specification
├── go.mod
├── go.sum
├── existing_code.go    # Code the task builds on
├── task_test.go        # Validation test suite (hidden from agent)
├── pitfalls.yaml       # Known pitfalls + RECALL items to seed
└── scoring.yaml        # Weights for quality scoring
```

**Task examples**:

| ID | Complexity | Description | Pitfall |
|---|---|---|---|
| 01 | Simple | Implement retry with exponential backoff | Missing jitter causes thundering herd |
| 02 | Simple | Add JSON validation to API handler | Missing Content-Type check |
| 03 | Moderate | Add circuit breaker to HTTP client | Not resetting half-open state correctly |
| 04 | Moderate | Implement concurrent worker pool | Goroutine leak on context cancellation |
| 05 | Complex | Add distributed locking to scheduler | Not handling clock skew between nodes |
| 06 | Complex | Migrate from sync map to channel-based design | Deadlock on buffer-full condition |

**Pitfall RECALL items** (seeded for AEF-full condition):

```yaml
# Example pitfall for task 01
- type: failure
  title: "Thundering herd from retry without jitter"
  content: |
    Symptom: All retrying clients hit the server simultaneously after backoff period.
    Root cause: Exponential backoff without jitter causes synchronized retries.
    Fix: Add random jitter (±25% of delay) to each retry interval.
    Prevention: Always add jitter to any backoff implementation.
  tags: ["retry", "backoff", "jitter", "thundering-herd"]
```

### Corpus 3: Task Pairs (Level 3, Experiment 3B)

10 pairs of tasks where each pair shares a pitfall pattern but in different contexts:

| Pair | Task A Context | Task B Context | Shared Pitfall |
|---|---|---|---|
| 1 | Payment retry handler | Webhook delivery retry | Missing idempotency key |
| 2 | User session cache | API response cache | No TTL expiration |
| 3 | Concurrent DB writes | Concurrent file writes | Missing mutex/lock |
| 4 | Config file parsing | API response parsing | No validation of untrusted input |
| 5 | HTTP client timeout | gRPC client timeout | Not propagating context cancellation |

### Corpus 4: Longitudinal Project (Level 3, Experiment 3C)

A single coherent project with 5 sequential features:

```
Project: mini-payflow (simplified payment processing service)
Feature 1: Basic payment creation endpoint (Session 1)
Feature 2: Idempotency layer (Session 2, builds on Feature 1 patterns)
Feature 3: Webhook notification system (Session 3, reuses retry patterns)
Feature 4: Settlement reconciliation (Session 4, reuses error handling patterns)
Feature 5: Admin dashboard API (Session 5, reuses validation patterns)
```

Each feature introduces new patterns that should help with subsequent features if captured to RECALL.

### Corpus 5: AEF-bench (Level 4)

50 tasks drawn from real Go open-source repositories. Selection criteria:
- Issue was resolved with a single PR
- PR has a clear test that validates the fix
- Task is self-contained (no external service dependencies)
- Task can be completed within Claude Code's context window

Sources: Go standard library contributions, popular Go OSS projects (kubernetes, prometheus, cobra, etc.), or synthetically adapted from real patterns.

---

## Measurement Infrastructure

### Automated Scorer

```
┌─────────────────────────────────────────────────────────────────┐
│                      AEF EVAL RUNNER                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Input: Task corpus + condition (baseline/AEF-minimal/AEF-full) │
│                                                                 │
│  Per task:                                                      │
│  1. Set up environment (git repo, RECALL seeds, hooks config)   │
│  2. Launch Claude Code under specified condition                 │
│  3. Present task specification                                  │
│  4. Record all actions (tool calls, responses, turns)           │
│  5. Run validation:                                             │
│     a. Test suite (go test -race -count=1 ./...)                │
│     b. Linter (golangci-lint run)                               │
│     c. Pitfall check (did implementation avoid known traps?)    │
│     d. LLM judge (quality assessment)                           │
│  6. Collect metrics                                             │
│                                                                 │
│  Output: results.json with per-task, per-condition metrics      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### LLM Judge Rubric

For quality scoring, the LLM judge evaluates 5 dimensions per task:

```
CORRECTNESS (0-10): Does the implementation satisfy all requirements?
- 10: All tests pass, all requirements met
- 7: Most tests pass, minor gaps
- 4: Some tests pass, significant gaps
- 0: No tests pass

CODE QUALITY (0-10): Is the code well-structured and maintainable?
- 10: Clean, idiomatic Go, well-organized, good error handling
- 7: Generally clean with minor issues
- 4: Functional but messy
- 0: Spaghetti code

PITFALL AVOIDANCE (0-10): Did the implementation avoid known traps?
- 10: All known pitfalls avoided
- 5: Some pitfalls avoided, some hit
- 0: Fell into every trap

COMPLETENESS (0-10): Were all requirements addressed?
- 10: Every requirement implemented
- 7: Most requirements, minor omissions
- 4: Major requirements missing
- 0: Barely started

EFFICIENCY (0-10): Was the task completed without unnecessary work?
- 10: Direct path to solution, minimal wasted effort
- 7: Some exploration but converged quickly
- 4: Significant thrashing or over-engineering
- 0: Never converged
```

**Combined quality score** = 0.30×Correctness + 0.20×CodeQuality + 0.25×PitfallAvoidance + 0.15×Completeness + 0.10×Efficiency

### Results Database Schema

```sql
CREATE TABLE eval_runs (
    run_id TEXT PRIMARY KEY,
    experiment TEXT NOT NULL,       -- "3A", "3B", "4A", etc.
    condition TEXT NOT NULL,        -- "baseline", "aef-minimal", "aef-full"
    task_id TEXT NOT NULL,
    task_complexity TEXT,           -- "simple", "moderate", "complex"
    attempt INTEGER DEFAULT 1,     -- for repeated trials

    -- Automated metrics
    tests_pass BOOLEAN,
    test_pass_rate REAL,           -- fraction of tests passing
    lint_violations INTEGER,
    lint_clean BOOLEAN,
    pitfalls_total INTEGER,
    pitfalls_avoided INTEGER,
    turns_to_completion INTEGER,
    tokens_consumed INTEGER,

    -- LLM judge scores
    judge_correctness REAL,
    judge_code_quality REAL,
    judge_pitfall_avoidance REAL,
    judge_completeness REAL,
    judge_efficiency REAL,
    judge_combined REAL,

    -- Timing
    duration_ms INTEGER,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,

    -- Raw data
    transcript_path TEXT,          -- path to full session transcript
    results_json TEXT              -- full structured results
);

CREATE TABLE eval_recall_state (
    run_id TEXT,
    item_id TEXT,
    item_type TEXT,
    item_title TEXT,
    was_retrieved BOOLEAN,         -- was this item found by recall_search?
    was_kept BOOLEAN,              -- did retrieval-judge keep it?
    was_used BOOLEAN,              -- did the agent reference it in implementation?
    PRIMARY KEY (run_id, item_id)
);
```

---

## Execution Plan

### Phase 1: Validate Existing Infrastructure (Week 1)

**Goal**: Confirm Level 1 components work as documented.

| Task | Effort | Dependencies |
|---|---|---|
| Run existing EvalHarness against v0 backend, record results | 1 day | None |
| Run existing JudgeHarness, record results | 1 day | Anthropic API key |
| Verify hook execution (Experiment 1A) for 3 critical hooks | 1 day | Hooks configured |
| Extend PayFlow collection with failures and decisions (Corpus 1) | 2 days | None |

**Deliverable**: Level 1 baseline report with measured metrics.

### Phase 2: Build Task Corpus (Week 2-3)

**Goal**: Create the Go task suite needed for Level 3-4 experiments.

| Task | Effort | Dependencies |
|---|---|---|
| Build 10 simple tasks with tests and pitfalls (Corpus 2) | 3 days | None |
| Build 10 moderate tasks with tests and pitfalls | 3 days | None |
| Build 10 complex tasks with tests and pitfalls | 4 days | None |
| Build 10 task pairs for repeat-failure experiment (Corpus 3) | 2 days | None |
| Build 5-session longitudinal project (Corpus 4) | 3 days | None |

**Deliverable**: Complete task corpora with specifications, test suites, pitfall annotations, and RECALL seed items.

### Phase 3: Build Evaluation Runner (Week 3-4)

**Goal**: Automated infrastructure to run experiments.

| Task | Effort | Dependencies |
|---|---|---|
| Build eval runner (task setup → Claude Code invocation → scoring) | 3 days | Corpus 2 |
| Build LLM judge scoring pipeline | 2 days | Judge rubric |
| Build results database and reporting | 2 days | Schema above |
| Build condition configurator (baseline/AEF-minimal/AEF-full) | 1 day | Hook configs |

**Deliverable**: `aef-eval run --experiment 3A --condition aef-full --tasks corpus-2/`

### Phase 4: Run Core Experiments (Week 4-6)

**Goal**: Produce measured results for the claims that matter most.

| Experiment | Runs | Sonnet 4.6 Cost² | Priority |
|---|---|---|---|
| 3A: Defect Rate Comparison (30 tasks × 3 conditions × 3 attempts) | 270 | ~$360 | **Highest** |
| 3B: Repeat Failure Prevention (10 pairs × 2 conditions × 3 attempts) | 60 | ~$83 | High |
| 3D: Hook Adherence Rate (20 sessions × 2 conditions) | 40 | ~$50 | High |
| 2A: Retrieval-Judge Filtering Quality (20 queries) | 20 | <$1 | Medium |
| 2B: RECALL + Plan Review (10 scenarios) | 10 | <$1 | Medium |

² Costs derived from per-token pricing assumptions in the "Pricing Assumptions" section. Multiply by 1.7× for Opus 4.6. 3A cost breakdown: 180 Strategy B runs × $1.25 + 90 Strategy C1 runs × $1.50 = $360.

**Deliverable**: Measured results for each experiment with pass/fail against defined criteria.

### Phase 5: Run Longitudinal and Comparative (Week 6-8)

**Goal**: Validate the "improves with use" and "best in class" claims.

| Experiment | Runs | Cost Estimate | Priority |
|---|---|---|---|
| 3C: Session Knowledge Accumulation (5 sessions × 2 conditions × 3 trials) | 30 | ~$41 | High |
| 4A: AEF-bench (50 tasks × 2 conditions) | 100 | ~$138 | Medium |
| 4B: Comparative System Analysis (20 tasks × 4 systems) | 80 | ~$100 | Lower |

**Deliverable**: Final evaluation report with measured scorecard replacing self-assessed ratings.

### Phase 6: Report (Week 8)

**Goal**: Replace every unproven claim with measured evidence.

**Report structure**:
1. Executive summary with measured scorecard
2. Per-experiment results with statistical analysis
3. Claim validation table (each claim → measured result → validated/invalidated)
4. Recommendations (what to keep, what to change, what to drop)
5. Raw data and reproducibility instructions

---

## Success Criteria (Overall)

The evaluation succeeds if it produces definitive answers to these questions:

| Question | Experiment | What "Yes" Looks Like | What "No" Looks Like |
|---|---|---|---|
| Does AEF reduce defects? | 3A | AEF-full outperforms baseline by ≥10 points on combined quality | AEF-full ≤ baseline, or improvement < 5 points |
| Does RECALL add value beyond hooks? | 3A | AEF-full outperforms AEF-minimal on pitfall avoidance | AEF-full ≈ AEF-minimal (RECALL adds nothing) |
| Does RECALL prevent repeat failures? | 3B | ≥25 point prevention delta | < 15 point delta |
| Does knowledge accumulate usefully? | 3C | Positive quality slope with RECALL, flat without | Flat or negative slope regardless of RECALL |
| Do hooks improve adherence? | 3D | ≥20 point improvement over prompts-only | < 10 point improvement |
| Does retrieval-judge improve precision? | 2A | Judge precision > raw precision by ≥0.10 | Judge precision ≈ raw precision |
| Does plan-review catch known failures? | 2B | ≥60% detection rate | < 40% detection rate |

**If the answer to questions 1-2 is "No"**, AEF in its current form is not viable and needs fundamental redesign.

**If the answer to questions 1-2 is "Yes" but 3-5 are "No"**, AEF's value is in hooks + skills only, not in RECALL or persistent knowledge.

**If the answer to all questions is "Yes"**, the self-assessed claims are validated and can be replaced with measured scores.

---

## Replacing the Scorecard

After the evaluation, the self-assessed scorecard in coding-standards-review.md:1178-1191 should be replaced with:

```
| Dimension                        | Measured Score | Evidence              | n    |
|----------------------------------|---------------|-----------------------|------|
| Methodology / Judgment Skills    | X.X/10        | Experiment 3D, 4B     | N    |
| Knowledge Integration (RECALL)   | X.X/10        | Experiment 3A, 3B, 3C | N    |
| Mechanical Verification (hooks)  | X.X/10        | Experiment 1A, 3D     | N    |
| Defect Prevention               | X.X/10        | Experiment 3A         | N    |
| Failure Avoidance               | X.X/10        | Experiment 3B         | N    |
| Context Resilience              | X.X/10        | Experiment 3C         | N    |
| Overhead                        | X.X/10        | Experiment 3A (turns) | N    |
```

Where `X.X` is derived from measured metrics and `n` is the sample size.

**"No comparable system exists" should be replaced with**: "Compared against vanilla Claude Code (N=270 runs), AEF-full achieved [X]% higher combined quality score and [Y]% higher pitfall avoidance rate."

---

## Cost Estimate

Costs derived from per-token pricing — see "Pricing Assumptions" in the execution section for the full derivation. Shown for Sonnet 4.6; multiply by 1.7× for Opus 4.6.

| Phase | Runs | Sonnet 4.6 Cost | Compute Time |
|---|---|---|---|
| Phase 1: Level 1 validation | ~50 | ~$1 | 1-2 days |
| Phase 4: Core experiments | ~400 | ~$494 | 3-5 days |
| Phase 5: Longitudinal + comparative | ~210 | ~$279 | 2-4 days |
| **Total** | **~660** | **~$774** | **6-11 days** |

The majority of cost is in Experiment 3A (270 runs at ~$360), which is also the most important experiment. It can be run incrementally — start with 30 runs (10 tasks × 3 conditions × 1 attempt, ~$40) to get directional signal before committing to full scale. See "Evaluation Milestones" for the incremental approach.

---

## What This Evaluation Does NOT Cover

1. **Human productivity** — Whether AEF makes developers faster (the METR RCT question). This requires a controlled study with real developers, which is outside the scope of automated evaluation.

2. **Long-term knowledge quality** — Whether RECALL's knowledge base degrades over months. The longitudinal experiment (3C) covers 5 sessions, not 50.

3. **Multi-language generalization** — All experiments use Go. AEF's skills are Go-focused. Generalization to TypeScript/Python/Rust is not tested.

4. **Team dynamics** — Whether RECALL's shared knowledge base helps multiple developers on the same project. All experiments are single-agent.

5. **Production incident response** — The incident agent mode is not evaluated. Building realistic incident scenarios requires production-like environments.

These limitations should be acknowledged in the final report. They represent future evaluation work, not gaps in this framework.

---

## How To Actually Run This

> Added 2026-02-24 after critical review of execution feasibility.

The experiments above describe *what* to measure. This section describes *how* to execute them, given the actual infrastructure that exists. It is honest about what's easy, what's hard, and what's impractical.

### The Execution Gap

Most Level 3-4 experiments assume you can programmatically drive Claude Code through tasks while it has access to MCP tools (RECALL). In practice:

| Invocation Method | MCP Tools? | Programmatic? | Captures Output? |
|---|---|---|---|
| `claude -p` (pipe mode) | No | Yes | Yes |
| `syscall.Exec` (EDI launch) | Yes (via .mcp.json) | No (replaces process) | No |
| Direct Anthropic Messages API | No (model only) | Yes | Yes |
| Existing EvalHarness (MCP client) | Yes (direct MCP) | Yes | Yes |

**The problem**: No existing method gives you both MCP tool access AND programmatic control of full Claude Code sessions. `claude -p` can run tasks but can't use RECALL. The EvalHarness can test RECALL but doesn't run Claude Code.

### Three Execution Strategies

Each experiment maps to one of three execution strategies, depending on what it actually tests.

#### Strategy A: Direct MCP Testing (Levels 1-2)

**What**: Test RECALL and its integrations by calling the MCP server directly, without Claude Code in the loop.

**How**: Extend the existing `codex/eval/` harness. The `MCPClient` in `mcpclient.go` already communicates with the RECALL MCP server via JSON-RPC over io.Pipe. The `JudgeHarness` already calls the Anthropic Messages API for LLM-as-judge scoring.

**Runnable today**:
```bash
# Level 1: RECALL retrieval quality (already implemented)
go test -tags "fts5,evalintegration" ./codex/eval/... -run TestE2E

# Level 1: Judge filtering quality (already implemented)
go test -tags "fts5,evalintegration" ./codex/eval/... -run TestJudge
```

**Extends to**: Experiments 1C, 2A, 2B, 2D. All of these test RECALL behavior directly — they don't need Claude Code making decisions, they need the MCP tools working correctly and the LLM judge evaluating results.

**Implementation effort**: 2-3 days of Go code extending the existing harness.

**What this proves**: RECALL retrieves relevant knowledge (design claim 1), retrieval-judge filters noise (design claim 2), plan-review can surface past failures (design claim 3).

#### Strategy B: Pipe Mode Testing (Level 3 — Skills and Hooks)

**What**: Test whether AEF's skills and hooks improve Claude Code's behavior on implementation tasks, without RECALL.

**How**: Use `claude -p` with `--append-system-prompt-file` to inject skills, and `.claude/settings.json` to configure hooks. Pipe task specifications in, capture output, run automated scoring.

**Runnable today** (the Ralph loop already does this):
```bash
# From edi/internal/assets/ralph/ralph.sh — adapted for eval
cat task-spec.md | claude -p \
  --append-system-prompt-file aef-skills-context.md \
  --allowedTools 'Edit,Write,Bash(go build:*),Bash(go test:*),Read,Glob,Grep' \
  2>&1 | tee output.txt
```

**Extends to**: Experiments 3A (skills+hooks condition only), 3D (hook adherence). These test whether skills shape behavior and hooks enforce standards — they don't need RECALL.

**What this can test**:
- **AEF-minimal condition** (skills + hooks, no RECALL): Fully testable
- **Baseline condition** (no skills, no hooks): Fully testable
- **AEF-full condition** (skills + hooks + RECALL): **NOT testable** via pipe mode

**Implementation effort**: 3-5 days to build the task runner, scoring pipeline, and results database.

**What this proves**: Skills influence agent behavior (design claim 4), hooks enforce mechanical standards (design claim 5). The skills+hooks value proposition.

#### Strategy C: Controlled Session Testing (Level 3 — RECALL Integration)

**What**: Test whether RECALL knowledge actually helps Claude Code produce better code. This is the hardest to automate because it requires both MCP access and task execution.

**Three sub-approaches**, in order of practicality:

**C1: Synthetic agent simulation** (most practical)
- Use the Anthropic Messages API directly with tool definitions that match RECALL's MCP tools
- Implement RECALL tool calls by proxying to the actual MCP server
- This creates a "Claude Code simulator" — same model, same tools, but under programmatic control
- The agent sees the same tool interface and responds identically; the difference is that *we* control the conversation loop instead of Claude Code's runtime

```
┌──────────────────────────────────────────────────┐
│              EVAL RUNNER (Go)                     │
│                                                   │
│  1. Load task spec                                │
│  2. Build system prompt (with/without skills)     │
│  3. Start RECALL MCP server                       │
│  4. Call Anthropic Messages API in a loop:        │
│     - Send task prompt                            │
│     - Model requests tool_use → proxy to MCP      │
│     - Model requests Edit/Write → apply to repo   │
│     - Model requests Bash → execute in sandbox    │
│     - Continue until model says "done" or limit   │
│  5. Run scoring (tests, lint, judge)              │
│                                                   │
└──────────────────────────────────────────────────┘
```

**Limitation**: This doesn't test Claude Code's specific runtime behavior (hook execution, context management, compaction). It tests the model + tools + skills + RECALL combination.

**Implementation effort**: 5-8 days. Requires building a tool-use conversation loop, tool proxying, and sandboxed execution.

**C2: Orchestrated interactive sessions** (moderate practicality)
- Launch Claude Code with MCP configured (via EDI or manual .mcp.json)
- Use `claude --resume` or session files to inject task prompts
- Capture results by reading modified files from the git repo after each session
- Semi-automated: session launch is scripted, but Claude Code runs interactively

**Limitation**: Harder to automate fully. Each session requires waiting for Claude Code to finish, then inspecting the workspace. Practical for 10-30 runs, not 270.

**Implementation effort**: 2-3 days for the orchestration script.

**C3: Manual observation** (most accurate, least scalable)
- Run actual AEF sessions through the eval tasks
- An evaluator observes and scores each session
- Record transcripts for offline LLM-judge scoring

**Limitation**: Only practical for 10-20 runs. Sufficient for design validation, not for statistical significance.

**Implementation effort**: Minimal (just the scoring rubric).

### Build Specification

This section inventories what exists and what needs building. Each component is mapped to the strategy it supports and the experiments it enables.

#### What Exists

| Component | Location | What It Does | Used By |
|---|---|---|---|
| MCPClient | `codex/eval/mcpclient.go` | JSON-RPC client for RECALL MCP server via io.Pipe (in-process, no subprocess) | Strategy A, C1 |
| EvalHarness | `codex/eval/harness.go` | 8-phase pipeline: protocol check, indexing, retrieval eval, audit trail | Strategy A |
| JudgeHarness | `codex/eval/judge.go` | LLM-as-judge via Anthropic Messages API (single-turn); includes AnthropicClient with retry | Strategy A |
| IR Metrics | `codex/eval/metrics.go` | Recall@K, Precision@K, nDCG, MRR | Strategy A |
| Judge Metrics | `codex/eval/judge_metrics.go` | Per-query judge precision, recall, F1, filtering rate, per-category aggregation | Strategy A |
| Report Generator | `codex/eval/report.go` | Text + JSON output of eval results | Strategy A |
| PayFlow Corpus | `codex/eval/testdata_payflow.go` | 30 docs, 20 ground-truth queries across 3 categories (semantic, keyword, hybrid-advantage) | Strategy A |
| MCP Server | `codex/internal/mcp/server.go` + `tools.go` | JSON-RPC handler, 5 tools: recall_search/get/add, recall_feedback, flight_recorder_log | Strategy A, C1 |
| Ralph Loop | `edi/internal/assets/ralph/ralph.sh` | `claude -p` invocation with `--append-system-prompt-file` skills injection, task iteration from PRD.json | Strategy B (pattern) |
| SQLite Schema | `edi/internal/recall/schema.sql` | items, vectors, flight_recorder, feedback tables with FTS5 virtual table | All |
| SearchEngine | `codex/internal/search/engine.go` | Hybrid search: vector cosine similarity + FTS5 BM25 + RRF fusion | Strategy A, C1 |

**What's runnable today:**

```bash
# RECALL retrieval quality (Level 1)
go test -tags "fts5,evalintegration" ./codex/eval/... -run TestE2E

# Judge filtering quality (Level 1)
go test -tags "fts5,evalintegration" ./codex/eval/... -run TestJudge
```

#### What Needs Building

Seven components, listed in dependency order. Components 1–6 are engineering work; component 7 is authoring work that can be parallelized.

**1. Results Database** — 1–2 days

| | |
|---|---|
| Dependencies | None |
| Used by | All strategies |
| Enables | Metric storage, cross-run analysis |

The `eval_runs` and `eval_recall_state` schemas are defined in the Results Database Schema section of this document. Implementation: add a migration to `edi/internal/recall/schema.sql`, write Go accessors (insert run, query by experiment/condition, compute per-condition medians). New file: `codex/eval/results.go`, ~200 lines.

**2. Scoring Pipeline** — 2–3 days

| | |
|---|---|
| Dependencies | Results database |
| Used by | Strategies B, C1 |
| Enables | Automated quality measurement after each implementation run |

After each run, execute in the task repo directory:

1. `go test -race -count=1 ./...` → parse exit code + test output for pass/fail counts
2. `golangci-lint run` → count violations
3. Pitfall check: grep implementation for anti-patterns listed in `pitfalls.yaml`
4. LLM judge: call Anthropic Messages API with the 5-dimension quality rubric (correctness, code quality, pitfall avoidance, completeness, efficiency). Parse scored response.
5. Compute combined quality score (weighted aggregate) and write to `eval_runs`

The Anthropic API client exists in `judge.go` (single-turn). The quality rubric is defined in the LLM Judge Rubric section of this document. What's new: the judge prompt for code quality (vs retrieval quality), the automated test/lint runners, and the pitfall checker. New file: `codex/eval/scorer.go`, ~400 lines.

**3. Strategy A Extensions** — 2–3 days

| | |
|---|---|
| Dependencies | Results database |
| Used by | Experiments 2A (enhanced), 2B, 2D |
| Enables | Level 1–2 completeness |

Three additions to the existing harness:

- **2A**: Add `FalseFilteringRate` to `judge_metrics.go` — count ground-truth relevant results the judge incorrectly drops. ~30 lines; data already available, just not computed.
- **2B**: New `PlanReviewHarness` — seed RECALL with 10 failure items via `MCPClient.RecallAdd()`, present 10 architectural plans to the plan-review skill via Messages API, score detection rate (did the response reference the relevant failure?). ~200 lines, extends `judge.go` pattern.
- **2D**: Extend `TestAuditTrail` in `harness.go` to compute coverage percentages: % of `recall_search` calls with matching `retrieval_query` entries, % with matching `retrieval_judgment` entries. Currently boolean pass/fail; needs quantification. ~50 lines.

**4. Condition Configurator** — 1 day

| | |
|---|---|
| Dependencies | None (consumed by runners) |
| Used by | Strategies B, C1 |
| Enables | Reproducible condition setup |

Given a condition name, produce the configuration for a run:

| Condition | System Prompt | Hooks | RECALL Seeds | RECALL Tools |
|---|---|---|---|---|
| `baseline` | Empty | None | None | None |
| `aef-minimal` | Skills (edi-core, coding, testing, plan-review) | gofumpt, protect-files, verify-quality | None | None |
| `aef-full` | Skills (same as minimal) | Same as minimal | Pitfalls from `pitfalls.yaml` | recall_search, recall_add |

New file: `codex/eval/condition.go`, ~100 lines. Config struct + factory function.

**5. Strategy B Runner (Pipe Mode)** — 3–4 days

| | |
|---|---|
| Dependencies | Scoring pipeline, condition configurator |
| Used by | Experiments 3A (baseline + AEF-minimal conditions), 3D |
| Enables | Skills + hooks testing without RECALL |

Per task:
1. Copy task repo to temp directory (isolated workspace)
2. Apply condition configuration (system prompt file, hooks in `.claude/settings.json`)
3. Invoke: `cat README.md | claude -p --append-system-prompt-file <skills> --allowedTools <allowlist>`
4. Capture stdout/stderr and list modified files via `git diff`
5. Run scoring pipeline against the modified repo
6. Write results to `eval_runs`, clean up temp directory

Adapts the existing `ralph.sh` invocation pattern into Go with automated scoring. The `claude -p` invocation exists; the orchestration and scoring around it don't. New file: `codex/eval/runner_pipe.go`, ~300 lines.

**6. Synthetic Agent Runner (Strategy C1)** — 5–8 days, **critical path**

| | |
|---|---|
| Dependencies | Scoring pipeline, condition configurator, MCPClient (exists), AnthropicClient (exists) |
| Used by | Experiments 3A (AEF-full condition), 3B, 3C |
| Enables | Testing whether RECALL knowledge improves implementation quality |

A multi-turn tool-use conversation loop that gives the model the same tools as Claude Code but under programmatic control:

```
ConversationLoop(taskSpec, condition, repo):
    messages = [systemPrompt, taskSpec]
    mcpClient = bootMCPServer(condition.recallSeeds)

    for turn = 0; turn < maxTurns; turn++:
        response = anthropic.Messages(messages)

        for each tool_use in response.content:
            result = dispatch(tool_use):
                recall_*     → mcpClient.CallTool(tool_use)
                Read         → os.ReadFile(repo + path)
                Edit/Write   → applyFileChange(repo + path, ...)
                Glob         → filepath.Glob(repo + pattern)
                Grep         → exec("rg", pattern, repo)
                Bash         → sandboxedExec(cmd, repo, allowlist, timeout)
            messages.append(tool_result)

        if response.stop_reason == "end_turn":
            break

    return runScoringPipeline(repo)
```

Four sub-components:

| Sub-component | Lines | What's New |
|---|---|---|
| Multi-turn conversation loop | ~200 | Extends single-turn `AnthropicClient` in `judge.go` to handle `tool_use` stop reason, accumulate messages |
| Tool dispatcher | ~150 | Route `tool_use` blocks: RECALL → MCPClient (exists), file ops → os package, Bash → exec |
| Bash sandbox | ~100 | Allowlisted commands (`go test`, `go build`, `gofumpt`, `golangci-lint`), 60s timeout, output capture, block dangerous patterns |
| Turn management | ~50 | Max turns (25), cumulative token logging, graceful termination |

New files: `codex/eval/runner_agent.go` (~350 lines) + `codex/eval/tools.go` (~200 lines).

**This is the gating component.** Without it, you can test baseline vs AEF-minimal (Strategy B) but cannot test whether RECALL adds value — the core design question.

**7. Task Corpus** — 10–15 days (parallelizable with components 1–6)

| | |
|---|---|
| Dependencies | None (but must match scoring pipeline's expected directory structure) |
| Used by | All Level 3–4 experiments |
| Enables | Implementation tasks for the runners to execute |

| Corpus | Contents | Effort | Used By |
|---|---|---|---|
| **Corpus 2**: Go Task Suite | 30 tasks (10 simple, 10 moderate, 10 complex). Each: `README.md` (spec), `go.mod`, `existing_code.go`, `task_test.go` (hidden validation), `pitfalls.yaml`, `scoring.yaml` | 8–10 days | 3A, 3D |
| **Corpus 3**: Task Pairs | 10 pairs sharing pitfall patterns in different contexts | 2–3 days | 3B |
| **Corpus 4**: Longitudinal Project | 5-feature project where each session builds on patterns from prior sessions | 2 days | 3C |
| **PayFlow Extended** | 10 failure items + 10 plan-review scenarios (5 safe, 5 dangerous) | 1 day | 2B, 2D |

Each task requires: a spec, a working test suite, at least one pitfall that Claude reliably falls into without RECALL (validated via baseline runs — if baseline failure rate < 50%, the pitfall isn't a good test), and RECALL seed items. Simple tasks take ~1 hour to author; complex tasks take ~4 hours.

#### Dependency Graph

```
                    ┌──────────────────┐
                    │ Results Database  │ (1-2 days)
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │ Scoring Pipeline  │ (2-3 days)
                    └──┬───────────┬───┘
                       │           │
            ┌──────────▼──┐   ┌───▼──────────────────┐
            │ Strategy B   │   │ Synthetic Agent (C1)  │ ← critical path
            │ Runner       │   │ Runner                │
            │ (3-4 days)   │   │ (5-8 days)            │
            └──────┬───────┘   └────┬──────────────────┘
                   │                │
                   │    ┌───────────▼────────┐
                   └────► Condition          │
                        │ Configurator       │ (1 day)
                        └────────────────────┘

    ┌─────────────────────┐     ┌───────────────┐
    │ Strategy A           │     │ Task Corpus    │
    │ Extensions (2-3 days)│     │ (10-15 days)   │
    │ (independent)        │     │ (independent)  │
    └─────────────────────┘     └───────────────┘
```

#### Critical Path to Milestone 1

| Week | Build | Enables | Milestone 1 Runs |
|---|---|---|---|
| 1 | Results DB + Scoring Pipeline + Strategy A extensions + Condition Configurator | Level 1–2 experiments | 1C (1 run) + 2A (5 runs) |
| 2 | Strategy B runner + first 5 tasks from corpus | Baseline vs AEF-minimal | 3A-smoke baseline + AEF-minimal (10 runs) + 3D-smoke (6 runs) |
| 3 | Synthetic agent runner | AEF-full condition | 3A-smoke AEF-full (5 runs) |
| **Total** | **3 weeks** | **Milestone 1 complete** | **27 runs** |

Task corpus authoring can start in week 1 and run in parallel. The first 5 tasks (enough for Milestone 1) should be prioritized; the remaining 25 tasks can be authored while Milestone 1 runs.

Milestone 2 (190 runs) requires the full 30-task corpus (Corpus 2) plus Corpus 3 task pairs. At 3–4 tasks per day authoring rate, the corpus is the bottleneck for Milestone 2 — not the infrastructure.

### Recommended Execution Path

Given the goal is to **prove the design claims** (not prove production viability), here's the practical execution path:

#### Week 1: Level 1-2 via Strategy A (Direct MCP)

**Effort**: 3 days development, 1 day running.

| Experiment | Method | Runs | Proves |
|---|---|---|---|
| 1C: v0 vs Codex retrieval | Existing EvalHarness | 2 | Codex improvement claim |
| 2A: Retrieval-judge filtering | Extended JudgeHarness | 20 | Judge adds precision |
| 2B: Plan-review failure detection | New harness extension | 10 | Cross-reference claim |
| 2D: Audit trail completeness | New harness extension | 10 | Flight recorder claim |

**Deliverable**: Measured retrieval quality and integration metrics.

#### Week 2-3: Level 3 Skills/Hooks via Strategy B (Pipe Mode)

**Effort**: 5 days development, 3 days running.

| Experiment | Method | Runs | Proves |
|---|---|---|---|
| 3A-partial: Baseline vs AEF-minimal | `claude -p` with/without skills | 60 | Skills + hooks improve quality |
| 3D: Hook adherence | `claude -p` with/without hooks | 40 | Hooks improve protocol adherence |

**Deliverable**: Measured skill/hook impact without RECALL.

#### Week 4-5: Level 3 RECALL via Strategy C1 (Synthetic Agent)

**Effort**: 8 days development, 3 days running.

| Experiment | Method | Runs | Proves |
|---|---|---|---|
| 3A-full: All three conditions | Synthetic agent with MCP proxy | 90 | RECALL adds value beyond skills |
| 3B: Repeat failure prevention | Two-session synthetic agent | 30 | Knowledge prevents repeat failures |
| 3C: Longitudinal accumulation | Five-session synthetic agent | 15 | Knowledge improves over time |

**Deliverable**: Measured RECALL impact including ablation (AEF-full vs AEF-minimal).

#### Week 6: Analysis and Report

**Effort**: 3 days.

Compile results, run statistical tests, produce the claim validation table.

### Pricing Assumptions

All API costs are derived from Anthropic's published pricing (February 2026). Previous estimates in this document were not grounded in per-token math — they were plausible-looking numbers with no derivation. The figures below replace them.

**API rates:**

| Model | Input / MTok | Output / MTok |
|---|---|---|
| Claude Sonnet 4.6 | $3.00 | $15.00 |
| Claude Opus 4.6 | $5.00 | $25.00 |

**Token consumption per implementation run** (estimated by task complexity and typical turn counts):

| Task Complexity | Typical Turns | Cumulative Input Tokens | Total Output Tokens |
|---|---|---|---|
| Simple (single function) | 5–8 | ~65K | ~20K |
| Moderate (multi-file) | 10–15 | ~160K | ~40K |
| Complex (architectural) | 15–25 | ~325K | ~80K |
| LLM judge scoring (per task) | 1 | ~3K | ~500 |

Input tokens are cumulative across turns — context grows with each tool call and response. Output tokens are the sum across all turns. These are estimates; the first 3–5 pilot runs should be instrumented to measure actual consumption and adjust.

**Derived cost per implementation run:**

| Model | Simple Task | Moderate Task | Complex Task | Weighted Avg¹ |
|---|---|---|---|---|
| Sonnet 4.6 | $0.50 | $1.08 | $2.18 | **$1.25** |
| Opus 4.6 | $0.83 | $1.80 | $3.63 | **$2.09** |

¹ Weighted average assumes 10 simple + 10 moderate + 10 complex tasks (the Corpus 2 distribution).

**Derivation example** (Sonnet, moderate task): 160K input × $3/MTok + 40K output × $15/MTok = $0.48 + $0.60 = **$1.08**.

**Cost per run by execution strategy:**

| Strategy | Sonnet 4.6 | Opus 4.6 | Notes |
|---|---|---|---|
| A: Direct MCP | ~$0.02 | ~$0.02 | Judge call only; RECALL MCP calls are local (Go + SQLite + Ollama) |
| B: Pipe Mode | ~$1.25 | ~$2.09 | Full implementation session + judge scoring |
| C1: Synthetic Agent | ~$1.50 | ~$2.50 | Implementation + RECALL tool calls + judge scoring |

Strategy C1 costs ~20% more than B because RECALL retrieval injects additional context (retrieved documents, relevance scores) into the conversation window.

**Batch API optimization**: Anthropic's Batch API provides a flat 50% discount for non-urgent workloads. All eval runs qualify since they have no latency requirements. Using batch processing would halve every cost figure below. The estimates assume standard (non-batch) pricing.

### Revised Cost Estimate

Costs shown for **Sonnet 4.6** (default recommendation). Opus 4.6 costs ~1.7× more — multiply Sonnet figures by 1.7 for Opus estimates.

| Phase | Runs | Strategy | Sonnet 4.6 Cost | Development | Wall Clock |
|---|---|---|---|---|---|
| Level 1–2 (Strategy A) | ~42 | A | ~$1 | 3 days | 1 week |
| Level 3 skills/hooks (Strategy B) | ~100 | B | ~$125 | 5 days | 1.5 weeks |
| Level 3 RECALL (Strategy C1) | ~135 | C1 | ~$203 | 8 days | 2 weeks |
| Analysis + report | — | A (judge) | ~$10 | 3 days | 0.5 weeks |
| **Total** | **~277** | | **~$339** | **19 days** | **5 weeks** |

The previous estimate for the same scope was $900–1,480 — roughly 3× too high. The actual cost is dominated by the Level 3 implementation runs. Level 1–2 runs are nearly free because they call the MCP server directly without invoking Claude for implementation.

Development effort (19 days) dominates API cost by a wide margin. At any reasonable engineering rate, the human time costs 10–50× more than the API spend.

### Evaluation Milestones

The evaluation should be run incrementally, not all-at-once. Each milestone builds on the previous one and answers a sharper question. Stop at the milestone that matches your confidence needs.

#### Milestone 1: Smoke Test — "Does this work at all?" (~27 runs, ~$35)

Five components need a go/no-go signal. A smoke test needs 5–10 runs per component — enough to see "this is broken" or "this shows promise," not enough to quantify effect size.

| Component | Experiment | Runs | Strategy | Sonnet Cost |
|---|---|---|---|---|
| RECALL retrieval | 1C: v0 vs Codex | 1 | A | <$0.01 |
| Retrieval-judge | 2A: Judge filtering (5 queries) | 5 | A | ~$0.10 |
| Skills impact | 3A-smoke: 5 tasks, baseline vs AEF-minimal | 10³ | B | ~$13 |
| RECALL integration | 3A-smoke: 5 tasks, AEF-minimal vs AEF-full | 5³ | C1 | ~$8 |
| Hook adherence | 3D-smoke: 3 sessions × 2 conditions | 6 | B | ~$8 |
| **Total** | | **27** | | **~$29** |

³ The 5 AEF-minimal runs are shared between "Skills impact" and "RECALL integration" (same tasks, same condition). Total unique implementation runs: 5 baseline + 5 AEF-minimal + 5 AEF-full + 6 hook sessions = 21.

**Duration**: 1–2 weeks (3 days dev + 1 day running).

**Decision gate**: If 3A-smoke shows zero difference between AEF and baseline across 5 tasks, investigate why before investing further. If there's a directional signal (even small), proceed to Milestone 2. The purpose is to fail fast and cheaply — not to prove anything.

#### Milestone 2: Design Validation — "Is each claim supported?" (~190 runs, ~$200)

Builds on Milestone 1. Expands to the full 30-task corpus and adds the repeat-failure experiment — the clearest test of RECALL's value.

| Experiment | Incremental Runs | Strategy | Sonnet Cost | What It Tests |
|---|---|---|---|---|
| 3A: Full 30 tasks × 3 conditions × 1 attempt | +75 (90 total) | 50 B + 25 C1 | ~$100 | AEF vs baseline across full task corpus |
| 3B: 10 task pairs × 2 conditions × 1 attempt | +20 | 10 B + 10 C1 | ~$28 | Does RECALL prevent repeat failures? |
| 3D: Full 20 sessions × 2 conditions | +34 (40 total) | B | ~$43 | Hook adherence at scale |
| 2B: Plan-review failure detection | +10 | A | <$1 | Cross-reference claim |
| 2D: Audit trail completeness | +10 | A | <$1 | Flight recorder claim |
| **Total** | **+149 (190 cumulative)** | | **+$172 (~$201 cumulative)** | |

**Duration**: 3–4 weeks cumulative (8 days dev + 4 days running).

**Decision gate**: At 90 implementation runs (30 tasks × 3 conditions), you have enough data to compute per-condition medians for every metric. If AEF-full and AEF-minimal show the same pitfall avoidance rate, RECALL is not adding value — that's a real finding. If hooks don't improve adherence across 40 sessions, the hook architecture needs rethinking. This milestone produces a claim validation table with "supported" / "not supported" for each design claim.

**This is the right stopping point for design validation.** ~190 runs tests every claim with enough samples to trust the direction. The results won't have tight confidence intervals, but they'll tell you what works and what doesn't.

#### Milestone 3: Statistical Confidence — "How confident are we?" (~620 runs, ~$775)

Builds on Milestone 2. Adds repeat trials for variance estimation and Level 4 comparative benchmarks.

| Experiment | Incremental Runs | Strategy | Sonnet Cost | What It Tests |
|---|---|---|---|---|
| 3A: Expand to 3 attempts per task | +180 (270 total) | 120 B + 60 C1 | ~$240 | Variance and reproducibility |
| 3B: Expand to 3 attempts per pair | +40 (60 total) | 20 B + 20 C1 | ~$55 | Repeat-failure confidence |
| 3C: Longitudinal (5 sessions × 2 cond × 3 trials) | +30 | 15 B + 15 C1 | ~$41 | "Improves with use" claim |
| 4A: AEF-bench (50 tasks × 2 conditions) | +100 | 50 B + 50 C1 | ~$138 | Standardized benchmark performance |
| 4B: Comparative (20 tasks × 4 systems) | +80 | B | ~$100 | Market position |
| **Total** | **+430 (620 cumulative)** | | **+$574 (~$775 cumulative)** | |

**Duration**: 5–6 weeks cumulative (19 days dev + 5 days running).

**What it adds over Milestone 2**: With 3 attempts per task, you can compute medians, ranges, and confidence intervals. You can report "AEF-full improved pitfall avoidance by 25 ±8 percentage points (n=270)" instead of "AEF-full appeared to improve pitfall avoidance." The Level 4 benchmarks let you make market-positioning claims rather than just internal design claims.

#### Milestone Summary

| Milestone | Cumulative Runs | Sonnet 4.6 Cost | Opus 4.6 Cost | Wall Clock | What You Learn |
|---|---|---|---|---|---|
| 1: Smoke Test | ~27 | ~$29 | ~$50 | 1–2 weeks | Go / no-go signal per component (5 components, 5–10 runs each) |
| 2: Design Validation | ~190 | ~$201 | ~$340 | 3–4 weeks | Supported / not-supported for each design claim |
| 3: Statistical Confidence | ~620 | ~$775 | ~$1,300 | 5–6 weeks | Quantified effect sizes with confidence intervals |

The step from Milestone 1 to 2 costs ~$172 and adds substantial evidence. The step from Milestone 2 to 3 costs ~$574 and adds statistical rigor. Whether that rigor is worth 3× the cost depends on the audience — internal design decisions need Milestone 2; external claims need Milestone 3.

---

## Rigor Assessment: Is This Enough?

### What "Proving the Design" Means

There are three different things you might want to prove:

| Goal | What It Requires | This Framework Covers? |
|---|---|---|
| **Design validity** | Each component works as specified | Yes (Levels 1-2) |
| **Design value** | Components together produce measurable benefit | Partially (Level 3) |
| **Production viability** | System works at scale with real teams | No (requires human study) |

For **proving the design claims** specifically — the stated goal — the framework is rigorous at Levels 1-2 and directionally useful at Level 3. Here's why:

### Where the Framework Is Rigorous

**RECALL retrieval quality** (Level 1): The EvalHarness with PayFlow collection provides proper information retrieval evaluation. 20 queries across 3 categories (semantic, keyword, hybrid-advantage), with ground truth annotations, computing standard IR metrics (Recall@K, Precision@K, nDCG@10, MRR). This is the same methodology used in academic IR evaluation. The existing benchmark results (Recall@10: 0.91, nDCG@10: 0.78) are meaningful.

**Retrieval-judge filtering** (Level 2): The JudgeHarness uses an established LLM-as-judge methodology with ground truth comparison. Computing judge precision, recall, F1, and filtering rate against known-relevant documents is standard practice.

**Hook execution** (Level 1): Binary pass/fail — hooks either fire or they don't. No statistical subtlety needed.

### Where the Framework Is Directional But Not Definitive

**Skills improving behavior** (Level 3): Comparing with-skills vs without-skills is a valid experimental design, but:
- LLM judge scoring introduces variance (the judge may score the same code differently across runs)
- 10-30 tasks is enough for directional signal, not for statistical significance
- Claude's inherent non-determinism means the same task can produce different results

**Mitigation**: Run each task 3 times, report medians and ranges, use automated metrics (test pass/fail, lint count) where possible instead of LLM judge. Automated metrics have zero variance.

**RECALL preventing repeat failures** (Level 3): This is the clearest design claim to test — either RECALL knowledge prevents the repeat failure or it doesn't. The task-pair design is strong. But:
- 10 task pairs is a small sample
- Claude may avoid the pitfall even without RECALL (inherent model knowledge)
- The pitfall must be detectable by automated tests, not just LLM judgment

**Mitigation**: Design pitfalls that Claude reliably falls into without help (validate this in the baseline condition first). If the baseline failure rate is < 50%, the pitfall isn't a good test.

### Where the Framework Is Weak

**"Compound value"** (the claim that integrations multiply value): The framework tests each component's contribution but doesn't rigorously test compound effects. A proper test would require a full factorial design (2^N conditions for N components), which is exponentially expensive. The 3-condition comparison (baseline / AEF-minimal / AEF-full) captures the gross effect but not the interaction terms.

**"Improves with use"** (Experiment 3C): 5 sessions is too few for a trend. The claim is inherently about long-term dynamics that can't be compressed into a 5-session experiment. This experiment can show *whether* accumulation happens at all, but not *how much* it improves or whether it plateaus.

**LLM-as-judge reliability**: The judge itself is a source of noise. The framework defines a rubric but doesn't measure inter-judge agreement (would a different Claude instance score the same code the same way?). Adding 3-judge consensus scoring would help but triples cost.

### Honest Conclusion

**For proving design claims**: The framework is sufficient. It maps every claim to a measurable experiment and can produce "yes, this works" or "no, this doesn't" for each one. The sample sizes are small but adequate for design validation — you're not publishing a paper, you're deciding whether to invest further.

**For proving market viability**: The framework is insufficient. That requires real developers, real projects, and longitudinal data that no automated evaluation can provide.

**For identifying what to cut**: The framework is excellent at this. If Experiment 3A shows AEF-full ≈ AEF-minimal, you know RECALL isn't pulling its weight. If 3D shows hooks don't improve adherence, you know the hook architecture needs rethinking. The framework's greatest value may be in what it *disproves*.

**The pass/fail thresholds are arbitrary**: Why ≥20 percentage points for pitfall avoidance? Why ≥10 for combined quality? These are judgment calls, not derived from theory. They should be treated as "meaningful improvement" guidelines, not hard cutoffs. The actual numbers matter more than whether they cross a pre-defined line.
