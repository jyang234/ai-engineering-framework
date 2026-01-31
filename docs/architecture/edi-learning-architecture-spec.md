# EDI Learning Architecture Specification

> **Implementation Status (January 31, 2026):** Not implemented. Influenced the recall_add/recall_feedback tool design but the type hierarchy, confidence levels, LLM judge, sandbox verification, and freshness management are not built.

**Status**: Design Complete
**Created**: January 24, 2026
**Last Updated**: January 24, 2026
**Version**: 0.1

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Knowledge Type System](#2-knowledge-type-system)
3. [Capture Workflow](#3-capture-workflow)
4. [Failure Attribution & LLM Judge](#4-failure-attribution--llm-judge)
5. [Retrieval & Validation](#5-retrieval--validation)
6. [Scope Hierarchy](#6-scope-hierarchy)
7. [Freshness & Staleness Management](#7-freshness--staleness-management)
8. [HITL Tooling](#8-hitl-tooling)
9. [Integration Points](#9-integration-points)
10. [Open Questions](#10-open-questions)

---

## 1. Executive Summary

### Problem Statement

AI assistants like Claude Code lack persistent memory across sessions. Community approaches to solving this (claude-mem, Memory Bank pattern) have limitations:

| Approach | Limitation |
|----------|------------|
| **Auto-capture everything** | Signal-to-noise problem; 90% routine operations |
| **Human-triggered capture** | Requires discipline; users forget |
| **AI-summarized memory** | Compression loses reasoning; retrieval relevance unclear |
| **Flat file storage** | No curation feedback; file bloat over time |

### EDI's Learning Architecture

EDI implements a **hybrid capture system** with:

- **Typed knowledge** with confidence levels (Evidence > Decision > Pattern > Observation > Failure)
- **Human-approved capture** with auto-suggestion to reduce friction
- **LLM judge** for failure attribution and escalation routing
- **Sandbox verification** to promote observations to evidence
- **Active staleness management** with freshness scoring and re-verification

### Core Differentiators

| vs. Claude-Mem | vs. Memory Bank |
|----------------|-----------------|
| Typed knowledge with confidence levels | Automated suggestions (not manual trigger) |
| Verification loop via Sandbox | Retrieval-time filtering by freshness |
| Preventability analysis for failures | Scope hierarchy (Project → Domain → Global) |
| Friction budgeting per session | Quality feedback loop |

### Architecture Overview

```
SESSION
├── Flight Recorder (continuous, low-level)
│   └── Sandbox telemetry, all tool calls
│
├── Capture Candidates (auto-detected)
│   └── Decisions, patterns, self-corrections
│
└── Human Approval (at session end)
    └── Promote to Codex with type + scope

CODEX
├── Evidence (Sandbox-verified)
├── Decisions (human-approved)
├── Patterns (generalizable)
├── Observations (noted, unverified)
└── Failures (preventable, with resolution)

RETRIEVAL
├── Hybrid search (vector + BM25)
├── Freshness weighting
├── Type weighting (Evidence > Decision > Pattern > ...)
└── Failure surfacing (proactive warnings)

VERIFICATION
├── Sandbox experiments validate Evidence
├── Contradictions trigger deprecation review
└── Staleness decay + refresh cycle
```

---

## 2. Knowledge Type System

### Type Hierarchy

Knowledge items are typed by confidence level, which affects retrieval weighting, staleness rules, and UI presentation.

| Type | Confidence | Description | Example |
|------|------------|-------------|---------|
| **Evidence** | Highest | Verified by Sandbox experiment; has metrics attached | "PaymentService handles 1000 req/s with <50ms p99" |
| **Decision** | High | Explicit choice with rationale; links to alternatives | "Chose Postgres over MongoDB for ACID requirements" |
| **Pattern** | Medium | Generalizable technique; may need validation in new contexts | "Use circuit breaker for external API calls" |
| **Observation** | Lower | Noted but not verified; may be context-dependent | "Legacy auth service times out under load" |
| **Failure** | Special | What was tried, what went wrong, why; links to resolution | "charge() deprecated; use processPayment()" |

### Schema Definition

```yaml
knowledge_item:
  # Identity
  id: uuid
  type: evidence | decision | pattern | observation | failure
  
  # Core content
  summary: string        # One-line description (required)
  detail: string         # Full context, reasoning, specifics (optional)
  
  # Provenance
  created_at: timestamp
  created_by: user_id
  last_verified: timestamp | null     # For evidence items
  source_session: session_id
  source_experiment: experiment_id | null  # Links to Sandbox
  
  # Scope
  scope:
    type: global | domain | project
    domain: string | null              # e.g., "payments", "auth"
    project: project_id | null         # null for global items
  files: [file_paths]                  # Files this knowledge relates to
  services: [service_names]            # Services this knowledge relates to
  
  # Lifecycle
  status: active | deprecated | superseded
  superseded_by: uuid | null           # If superseded, link to replacement
  confidence: float                    # 0.0-1.0, can decay over time
  
  # Retrieval metadata
  last_retrieved: timestamp | null
  retrieval_count: integer
  useful_count: integer                # Times marked useful after retrieval
  
  # Failure-specific fields (only for type=failure)
  failure_context:
    attempted_approach: string         # What was tried
    failure_mode: string               # How it failed
    root_cause: string                 # Why it failed
    resolution: string | null          # What worked instead
    preventable: boolean               # Could EDI have avoided this?
    attribution: knowledge_gap | retrieval_miss | reasoning_error | 
                 requirements_ambig | transient | novel
```

### Type-Specific Behavior

| Behavior | Evidence | Decision | Pattern | Observation | Failure |
|----------|----------|----------|---------|-------------|---------|
| **Retrieval weight** | 1.0 | 0.9 | 0.7 | 0.5 | 0.8 (proactive) |
| **Staleness half-life** | 30 days | 90 days | 180 days | 60 days | 90 days |
| **Re-verification** | Yes (Sandbox) | No | No | Can promote | No |
| **Auto-capture eligible** | Yes | Yes | No | No | Self-correct only |
| **Requires approval** | No | No | Yes | Yes | User-corrected only |

---

## 3. Capture Workflow

### Capture Confidence Levels

EDI auto-detects capture candidates based on session activity:

| Event | Confidence | Auto-Capture? | Prompt User? |
|-------|------------|---------------|--------------|
| ADR created/modified | Very High | Yes | No |
| Sandbox experiment completed | Very High | Yes | No |
| Self-correct succeeded | High | Yes (brief) | No |
| User-corrected failure | High | No | Yes |
| External API integration | Medium-High | No | Yes |
| New pattern applied | Medium | No | Yes |
| Multi-file refactor | Medium | No | Batch at end |
| Single file edit | Low | No | No |
| Routine test run | Low | No | No |
| Git operations | Low | No | No |

### Silent Capture Tier

Items captured without prompting (high confidence, low noise risk):

```yaml
silent_capture:
  always:
    - ADRs created or modified
    - Sandbox experiments (verified evidence)
    - Self-corrections (failure + fix pairs)
  
  never_without_approval:
    - Inferred patterns
    - Observations without verification
    - Anything promoted to global or domain scope
```

### Session-End Capture UX

```
┌─────────────────────────────────────────────────────────────────┐
│  EDI detected capture-worthy items:                             │
│                                                                 │
│  ✓ [Decision] Chose event-driven architecture for OrderService  │
│    → "Decouples order processing from payment confirmation"     │
│                                                                 │
│  ✓ [Evidence] Sandbox experiment #47 passed                     │
│    → "Stripe webhook retry logic handles idempotency correctly" │
│                                                                 │
│  ? [Pattern] Used repository pattern for data access            │
│    → "Abstracts database queries behind interface"              │
│                                                                 │
│  [Save Selected] [Edit] [Skip All] [Preferences...]             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**UX elements:**
- ✓ = Pre-selected (high confidence)
- ? = Suggested but not pre-selected (medium confidence)
- User can uncheck, edit summary, or add items EDI missed
- "Preferences..." allows setting capture rules (e.g., "Never prompt about patterns")

### Friction Budget

To prevent prompt fatigue, EDI allocates a "friction budget" per session:

| Session Length | Max Interactions | Rationale |
|----------------|------------------|-----------|
| Short (<30 min) | 1-2 | Minimal interruption |
| Medium (30-90 min) | 2-4 | Balanced |
| Long (>90 min) | 4-6 | More work = more capture-worthy items |

**Interaction costs:**

| Interaction Type | Cost | Notes |
|------------------|------|-------|
| Retrieval preview (planning) | 1 | High value, expected |
| Capture prompt (session end) | 1 | Expected location |
| Mid-session escalation | 2 | Interruption penalty |
| Failure review request | 1 | Contextual |

**When budget exhausted:**
- Batch remaining items for session end
- Auto-handle with conservative defaults
- Log for async review

### User Preference Learning

```yaml
user_preferences:
  # Explicit settings
  capture_threshold: conservative | balanced | aggressive
  always_prompt_for: [patterns, global_promotion]
  never_prompt_for: [evidence, self_corrections]
  
  # Adaptive behavior
  if_user_skips_3_consecutive_prompts:
    action: increase auto-capture threshold, decrease prompt frequency
  
  if_user_engages_with_most_prompts:
    action: maintain current balance
```

---

## 4. Failure Attribution & LLM Judge

### The Attribution Problem

When EDI makes a mistake, the cause could be:

| Attribution | Description | Mitigation |
|-------------|-------------|------------|
| `knowledge_gap` | Codex didn't have relevant information | Add to Codex |
| `retrieval_miss` | Codex had it, retrieval didn't surface it | Tune embeddings/reranking |
| `reasoning_error` | Retrieved but EDI ignored or misapplied | Review skill/prompt |
| `requirements_ambig` | User's instruction was unclear | Human clarification |
| `transient` | Environmental (network, rate limit) | Ignore |
| `novel` | Genuinely new edge case | Capture as new knowledge |

### LLM Judge Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    FAILURE TRIAGE FLOW                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  FAILURE DETECTED                                               │
│         │                                                       │
│         ▼                                                       │
│  ┌─────────────────┐                                            │
│  │  LLM JUDGE      │  Inputs:                                   │
│  │  (Claude Haiku) │  - Failure description                     │
│  │                 │  - What EDI attempted                      │
│  │                 │  - Codex retrieval results (if any)        │
│  │                 │  - Session context                         │
│  └────────┬────────┘                                            │
│           │                                                     │
│           ▼                                                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  CLASSIFICATION OUTPUT                                  │    │
│  │                                                         │    │
│  │  confidence: high | medium | low                        │    │
│  │  attribution: <one of six types above>                  │    │
│  │  suggested_action: string                               │    │
│  │  escalation_reason: string | null                       │    │
│  │                                                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│           │                                                     │
│           ▼                                                     │
│  ┌─────────────────┐     ┌─────────────────┐                    │
│  │ HIGH CONFIDENCE │     │ LOW CONFIDENCE  │                    │
│  │ + AUTO-MITIGATE │     │ OR ESCALATE     │                    │
│  │                 │     │                 │                    │
│  │ Execute action  │     │ Queue for human │                    │
│  │ Log decision    │     │ review          │                    │
│  └─────────────────┘     └─────────────────┘                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Routing Rules

| Attribution | Confidence | Action |
|-------------|------------|--------|
| `knowledge_gap` | High | Auto: Add to Codex as Observation |
| `knowledge_gap` | Low | Escalate: Human reviews before adding |
| `retrieval_miss` | High | Auto: Flag for retrieval tuning |
| `retrieval_miss` | Low | Escalate: May be misattributed |
| `reasoning_error` | Any | Escalate: Skill/prompt review needed |
| `requirements_ambig` | Any | Escalate: Human clarification needed |
| `transient` | High | Auto: Ignore |
| `novel` | Any | Escalate: Human decides if capture-worthy |
| `uncertain` | — | Escalate: Judge couldn't determine |

**Principle:** Conservative about automation, liberal about escalation (initially). Tune thresholds as data accumulates.

### Judge Evaluation (Sampling)

```yaml
judge_evaluation:
  frequency: weekly
  sample_size: 20 decisions (or 10% of volume, whichever smaller)
  
  review_criteria:
    - Was the attribution correct?
    - Was the action appropriate?
    - Should this have been escalated?
    - Was the escalation necessary?
  
  metrics:
    attribution_accuracy: percentage of correct classifications
    escalation_precision: percentage of escalations that needed human
    escalation_recall: percentage of human-needed cases that were escalated
    false_automation_rate: percentage of auto-actions that were wrong
  
  tuning_triggers:
    - false_automation_rate > 10%: lower confidence thresholds
    - escalation_precision < 50%: raise confidence thresholds
```

### Failure Schema Extension

For captured failures, additional fields support learning:

```yaml
failure_context:
  attempted_approach: "What EDI tried"
  failure_mode: "How it failed"
  root_cause: "Why it failed"
  resolution: "What worked instead" | null
  preventable: boolean
  attribution: knowledge_gap | retrieval_miss | reasoning_error | 
               requirements_ambig | transient | novel
  
  # Judge metadata
  judge_confidence: high | medium | low
  judge_reasoning: "Why the judge classified this way"
  human_reviewed: boolean
  human_override: attribution | null  # If human disagreed
```

---

## 5. Retrieval & Validation

### When Retrieval Matters Most

| Phase | Retrieval Value | Rationale |
|-------|-----------------|-----------|
| Planning/Design | **Very High** | Architectural decisions, past patterns, "we tried X and it failed" |
| Task decomposition | High | Understanding scope, dependencies, related work |
| Implementation start | Medium-High | API contracts, coding patterns, gotchas |
| Mid-implementation | Medium | Mostly working from established context |
| Debugging/fixing | High | Past failures, troubleshooting knowledge |
| PR/Review | Medium | Standards, checklists (mostly in Skills) |

**Insight:** Front-load validation. Wrong retrieval at planning affects everything downstream.

### Retrieval Preview UX (Planning Phase)

```
┌─────────────────────────────────────────────────────────────────┐
│  EDI: Planning authentication refactor                          │
│                                                                 │
│  I found relevant context from Codex:                           │
│                                                                 │
│  ✓ [Decision] Using JWT tokens for session management           │
│    "Chosen over session cookies for API-first architecture"     │
│                                                                 │
│  ✓ [Evidence] Auth service handles 10k logins/min               │
│    "Load tested Jan 15, 2026"                                   │
│                                                                 │
│  ? [Pattern] Rate limiting pattern for login endpoints          │
│    "Used in PaymentService, may apply here"                     │
│                                                                 │
│  ⚠ [Failure] OAuth refresh token race condition                 │
│    "Fixed in PR #423, use distributed lock"                     │
│                                                                 │
│  [Looks good] [Remove irrelevant] [Add missing context]         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**UX principles:**
- Show retrieval results at planning phase, not buried in execution
- Pre-check high-confidence items (✓), flag uncertain ones (?)
- Surface failures proactively (⚠)
- One-click to remove irrelevant items
- Natural language to add missing context ("also consider the SSO integration work")
- Non-blocking: user can say "looks good" and move on

### Implicit Feedback Signals

| Signal | Type | How Used |
|--------|------|----------|
| Item shown but removed | Negative | Lower retrieval score for similar queries |
| Item shown and kept | Weak positive | Maintain retrieval score |
| Item used in output | Strong positive | Boost retrieval score |
| User added context manually | Gap signal | Analyze what was missing |

### Explicit Feedback (Optional)

- Thumbs up/down on specific items in preview
- "This was helpful" / "This was noise" at session end
- Used to validate implicit signals

### Feedback Loop Timeline

| Week | Activity |
|------|----------|
| 1-4 | Log all retrieval decisions and user modifications (implicit feedback) |
| 4-8 | Analyze patterns: what gets removed? what gets added? |
| 8+ | Adjust retrieval weights based on observed patterns |
| Ongoing | Sample explicit feedback for validation |

---

## 6. Scope Hierarchy

### Three-Tier Model

```
┌─────────────────────────────────────────────────────────────────┐
│  GLOBAL (Enterprise Architect managed)                          │
│  ├── Always retrieved for all projects                          │
│  ├── Examples:                                                  │
│  │   - "All services must use mTLS for internal communication"  │
│  │   - "Authentication must go through central auth service"    │
│  │   - "PII must never be logged"                               │
│  └── Promotion: Requires enterprise architect approval          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  DOMAIN (Optional middle tier)                                  │
│  ├── Retrieved for projects in that domain                      │
│  ├── Examples:                                                  │
│  │   - [payments] "All payment amounts in cents, not dollars"   │
│  │   - [auth] "Session timeout is 30 minutes"                   │
│  └── Promotion: Requires domain owner approval                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  PROJECT (Default)                                              │
│  ├── Retrieved only for this project                            │
│  ├── Examples:                                                  │
│  │   - "OrderService uses event sourcing"                       │
│  │   - "We chose Postgres for this project because..."          │
│  └── No approval needed — project team autonomy                 │
└─────────────────────────────────────────────────────────────────┘
```

### Retrieval Behavior by Scope

| Scope | Retrieved When | Managed By |
|-------|----------------|------------|
| Global | Always, for all projects | Enterprise architect |
| Domain | Project is in that domain | Domain owner |
| Project | Current project matches | Project team |

### Promotion Flow

```
Project knowledge item
         │
         ▼ "This seems useful beyond this project"
┌────────────────────┐
│  PROMOTION REQUEST │
│                    │
│  From: Project X   │
│  To: Domain/Global │
│                    │
│  Justification:    │
│  "Used successfully│
│   in 3 projects"   │
└────────────────────┘
         │
         ▼
┌────────────────────┐
│  APPROVAL QUEUE    │
│                    │
│  Domain → Domain   │
│           owner    │
│  Global → EA       │
└────────────────────┘
         │
         ▼
  Approved: Scope updated
  Rejected: Stays at project level
```

### Scope Schema

```yaml
scope:
  type: global | domain | project
  domain: string | null        # Required if type=domain
  project: project_id | null   # Required if type=project; null for global
  
  # Promotion tracking
  promoted_from: project_id | null
  promoted_at: timestamp | null
  promoted_by: user_id | null
  promotion_justification: string | null
```

---

## 7. Freshness & Staleness Management

### Freshness Scoring Algorithm

```python
def compute_freshness(item: KnowledgeItem) -> float:
    """Returns 0.0 (stale) to 1.0 (fresh)"""
    
    # Base freshness decays with age
    base_freshness = age_decay(
        item.created_at, 
        half_life=HALF_LIFE_BY_TYPE[item.type]
    )
    
    # Boost for verified evidence
    if item.type == "evidence" and item.last_verified:
        verification_freshness = age_decay(
            item.last_verified, 
            half_life=30  # days
        )
        base_freshness = max(base_freshness, verification_freshness)
    
    # Boost if recently retrieved and not contradicted
    if item.last_retrieved and not item.contradicted_since_retrieval:
        retrieval_freshness = age_decay(
            item.last_retrieved, 
            half_life=14  # days
        )
        base_freshness = max(base_freshness, retrieval_freshness * 0.5)
    
    # Penalty for deprecated
    if item.status == "deprecated":
        return 0.0
    
    return base_freshness

# Half-life by type (days)
HALF_LIFE_BY_TYPE = {
    "evidence": 30,
    "decision": 90,
    "pattern": 180,
    "observation": 60,
    "failure": 90
}
```

### Active Deprecation Triggers

| Trigger | Action |
|---------|--------|
| Contradicted by newer item | Mark `superseded_by(new_id)` |
| Scoped file deleted | Mark `deprecated` |
| Scoped service removed | Mark `deprecated` |
| Human flags as outdated | Mark `deprecated` |
| Sandbox experiment contradicts | Flag for review |

### Sandbox Re-Verification (Evidence Only)

For high-value Evidence items, periodic re-verification via Sandbox:

```yaml
re_verification:
  eligible: Evidence items only
  triggers:
    - last_verified > 30 days ago
    - retrieved frequently (retrieval_count > 5 in last 30 days)
    - high potential harm if stale (e.g., performance claims)
  
  process:
    1. Generate Sandbox experiment to re-verify claim
    2. If passes: Update last_verified timestamp
    3. If fails: Flag for human review
    4. Human decides: Update evidence, deprecate, or mark superseded
  
  frequency: Weekly batch, limited to 10 items per run
```

**Example:**

```
Evidence: "OrderService processes 500 orders/min"
Last verified: 45 days ago
Retrieval count: 12 in last month

Action: Generate load test in Sandbox
Result: Now handles 800 orders/min

Options:
  [Update] → "OrderService processes 800 orders/min" + new timestamp
  [Deprecate] → Mark old evidence deprecated
  [Keep] → Mark reviewed, no change needed
```

---

## 8. HITL Tooling

### Day 1: Minimal Approach

```
┌─────────────────────────────────────────────────────────────────┐
│  ESCALATION QUEUE (Simple)                                      │
│                                                                 │
│  • Slack channel: #edi-escalations                              │
│  • EDI posts: "Failure needs review: [link to Sandbox UI]"      │
│  • Human reviews in Sandbox UI                                  │
│  • Human marks resolved via Slack reaction or Sandbox UI        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Escalation message format:**

```
🔍 EDI Escalation: Failure Attribution Review

Type: User-corrected failure
Session: abc-123
Attribution (judge): reasoning_error (medium confidence)
Summary: EDI used deprecated PaymentService.charge() API

Judge reasoning: "Codex had deprecation notice but EDI 
generated code using old API anyway"

[View in Sandbox] [Mark Resolved] [Override Attribution]
```

### Day N: Sandbox UI Extended

```
┌─────────────────────────────────────────────────────────────────┐
│  SANDBOX UI                                                     │
│                                                                 │
│  Existing:                                                      │
│  ├── Experiment results                                         │
│  ├── Telemetry/traces                                           │
│  └── Output files                                               │
│                                                                 │
│  New tabs:                                                      │
│  ├── Escalation Queue                                           │
│  │   └── List of pending reviews with filters                   │
│  ├── Promotion Requests                                         │
│  │   └── Project → Domain/Global approvals                      │
│  ├── Judge Dashboard                                            │
│  │   └── Accuracy metrics, sample reviews                       │
│  └── Knowledge Browser                                          │
│      └── Search/filter Codex items, manage lifecycle            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### LangSmith Integration (Optional)

Consider LangSmith if/when needed for:

| Use Case | LangSmith Value |
|----------|-----------------|
| Failure trace inspection | High — detailed LLM call analysis |
| Judge evaluation datasets | High — golden set management |
| Retrieval quality evaluation | Medium — can track retrieval performance |
| Prompt/skill A/B testing | High — built-in experimentation |

**Not recommended for:**
- Approval queues (build simple custom)
- Promotion workflows (build simple custom)

**Decision:** Defer LangSmith until evaluation infrastructure becomes a bottleneck.

---

## 9. Integration Points

### EDI Session Lifecycle

```
SESSION START
    │
    ├── Load relevant Codex items (via RECALL)
    ├── Load applicable Skills (GUIDE)
    └── Initialize Flight Recorder
    │
    ▼
PLANNING PHASE
    │
    ├── Query Codex for task-relevant knowledge
    ├── Present Retrieval Preview to user
    ├── User validates/adjusts context
    └── Proceed with validated context
    │
    ▼
EXECUTION PHASE
    │
    ├── Flight Recorder captures all operations
    ├── On failure: LLM Judge triages
    │   ├── Auto-mitigate if high confidence
    │   └── Escalate if low confidence or requires human
    └── On success: Note potential capture candidates
    │
    ▼
SESSION END
    │
    ├── Present Capture Prompt (within friction budget)
    ├── User approves/edits/skips
    ├── Approved items → Codex
    └── Flight Recorder data retained for forensics
```

### Codex MCP Server Interface

```yaml
tools:
  codex_search:
    description: "Search Codex for relevant knowledge"
    parameters:
      query: string
      scope: global | domain | project
      types: [evidence, decision, pattern, observation, failure]
      min_freshness: float (0.0-1.0)
      limit: integer
    returns:
      items: [KnowledgeItem]
      
  codex_add:
    description: "Add item to Codex"
    parameters:
      type: evidence | decision | pattern | observation | failure
      summary: string
      detail: string
      scope: {type, domain?, project?}
      source_session: session_id
      source_experiment: experiment_id?
    returns:
      id: uuid
      status: created | pending_approval
      
  codex_feedback:
    description: "Provide feedback on retrieved item"
    parameters:
      item_id: uuid
      feedback: useful | not_useful | outdated
      context: string?
    returns:
      acknowledged: boolean
```

### Sandbox Integration

```yaml
# Sandbox experiment result feeds into Codex
experiment_complete:
  on_success:
    - Extract verifiable claims
    - Create Evidence items (or update existing)
    - Link experiment_id for provenance
    
  on_failure:
    - Create Failure item with full context
    - Trigger LLM Judge for attribution
    - Route based on attribution
```

---

## 10. Open Questions

### Resolved in This Design

| Question | Resolution |
|----------|------------|
| How to balance auto-capture vs human approval? | Friction budget + confidence tiers |
| How to attribute failures? | LLM Judge with escalation routing |
| How to handle staleness? | Freshness scoring + active deprecation + re-verification |
| How to scope knowledge? | Project (default) → Domain → Global hierarchy |
| What HITL tooling to use? | Start with Slack + Sandbox UI; defer LangSmith |

### Deferred Questions

| Question | Deferred Until |
|----------|----------------|
| Retrieval evaluation framework (golden datasets) | After initial retrieval implementation |
| Judge tuning thresholds | After 4+ weeks of judge data |
| Domain tier necessity | After multi-project usage patterns emerge |
| LangSmith integration | When evaluation infrastructure is a bottleneck |

### Implementation Dependencies

| Component | Depends On |
|-----------|------------|
| Capture workflow | Codex MCP server |
| LLM Judge | Failure detection in EDI |
| Retrieval preview | Codex MCP server + EDI planning phase |
| Re-verification | Sandbox experiment generation |
| Promotion workflow | Scope hierarchy in Codex schema |

---

## Appendix A: Decision Log

| Decision | Date | Rationale |
|----------|------|-----------|
| Typed knowledge system | Jan 24, 2026 | Different confidence levels need different handling |
| Human approval for capture | Jan 24, 2026 | Quality over quantity; prevents noise |
| LLM Judge for attribution | Jan 24, 2026 | Scales better than human-reviews-everything |
| Friction budget model | Jan 24, 2026 | Prevents prompt fatigue |
| Project-first scoping | Jan 24, 2026 | Simple default; promote when proven valuable |
| Slack + Sandbox UI for HITL | Jan 24, 2026 | Start simple; avoid tool sprawl |
| Defer LangSmith | Jan 24, 2026 | Not needed until evaluation is a bottleneck |

---

## Appendix B: Related Documents

- [AEF Architecture Specification v0.5](./aef-architecture-specification-v0_5.md)
- [EDI Specification Plan](./edi-specification-plan.md)
- [Codex Architecture Deep Dive](./codex-architecture-deep-dive.md)
- EDI Quick Reference (./edi-quick-reference.md)
