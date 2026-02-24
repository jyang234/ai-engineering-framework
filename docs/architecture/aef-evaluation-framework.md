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

| Experiment | Runs | Cost Estimate | Priority |
|---|---|---|---|
| 3A: Defect Rate Comparison (30 tasks × 3 conditions × 3 attempts) | 270 | ~$800-1200 | **Highest** |
| 3B: Repeat Failure Prevention (10 pairs × 2 conditions × 3 attempts) | 60 | ~$200-350 | High |
| 3D: Hook Adherence Rate (20 sessions × 2 conditions) | 40 | ~$150-250 | High |
| 2A: Retrieval-Judge Filtering Quality (20 queries) | 20 | ~$30-50 | Medium |
| 2B: RECALL + Plan Review (10 scenarios) | 10 | ~$30-50 | Medium |

**Deliverable**: Measured results for each experiment with pass/fail against defined criteria.

### Phase 5: Run Longitudinal and Comparative (Week 6-8)

**Goal**: Validate the "improves with use" and "best in class" claims.

| Experiment | Runs | Cost Estimate | Priority |
|---|---|---|---|
| 3C: Session Knowledge Accumulation (5 sessions × 2 conditions × 3 trials) | 30 | ~$150-250 | High |
| 4A: AEF-bench (50 tasks × 2 conditions) | 100 | ~$400-600 | Medium |
| 4B: Comparative System Analysis (20 tasks × 4 systems) | 80 | ~$300-500 | Lower |

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

| Phase | Runs | Estimated API Cost | Compute Time |
|---|---|---|---|
| Phase 1: Level 1 validation | ~50 | $50-100 | 1-2 days |
| Phase 4: Core experiments | ~400 | $1,200-1,900 | 3-5 days |
| Phase 5: Longitudinal + comparative | ~210 | $850-1,350 | 2-4 days |
| **Total** | **~660** | **$2,100-3,350** | **6-11 days** |

The majority of cost is in Experiment 3A (270 runs), which is also the most important experiment. It can be run incrementally — start with 30 runs (10 tasks × 3 conditions × 1 attempt) to get directional signal before committing to full scale.

---

## What This Evaluation Does NOT Cover

1. **Human productivity** — Whether AEF makes developers faster (the METR RCT question). This requires a controlled study with real developers, which is outside the scope of automated evaluation.

2. **Long-term knowledge quality** — Whether RECALL's knowledge base degrades over months. The longitudinal experiment (3C) covers 5 sessions, not 50.

3. **Multi-language generalization** — All experiments use Go. AEF's skills are Go-focused. Generalization to TypeScript/Python/Rust is not tested.

4. **Team dynamics** — Whether RECALL's shared knowledge base helps multiple developers on the same project. All experiments are single-agent.

5. **Production incident response** — The incident agent mode is not evaluated. Building realistic incident scenarios requires production-like environments.

These limitations should be acknowledged in the final report. They represent future evaluation work, not gaps in this framework.
