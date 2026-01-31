# EDI Learning Architecture - Noise Control Addendum

> **Implementation Status (January 31, 2026):** Not implemented. Staging tiers, LLM judge prompt, and archival workflow do not exist.

**Status**: Draft
**Created**: January 25, 2026
**Version**: 0.2
**Extends**: edi-learning-architecture-spec.md v0.1

---

## Overview

This addendum formalizes the noise control mechanisms for EDI's learning architecture:

1. **Staging Tiers** — Which captures go directly to Codex vs need review
2. **LLM Judge Prompt** — Concrete prompt for failure attribution
3. **Archival Workflow** — How to clean up stale/low-value items
4. **v0 vs v1 Scope** — What gets implemented when

---

## 1. Staging Tiers

Not all captures should go directly to Codex. Items are classified into tiers based on confidence and risk.

### Tier Definitions

| Tier | Destination | Human Approval | Examples |
|------|-------------|----------------|----------|
| **Tier 1** | Direct to Codex | No (silent capture) | ADRs, verified evidence, edited captures |
| **Tier 2** | Staging queue | Yes (before commit) | Auto-detected patterns, judge-triggered knowledge_gap |
| **Tier 3** | Flight recorder only | Only if explicit | Routine decisions, observations, self-corrections |

### Tier 1: Direct to Codex

High confidence, low noise risk. Captured without prompting.

```yaml
tier_1_criteria:
  - type: adr
    condition: created_or_modified
    
  - type: evidence
    condition: sandbox_verified
    
  - type: any
    condition: user_edited_before_save
    
  - type: decision
    condition: confidence >= 0.95 AND has_rationale
```

### Tier 2: Staging Queue

Medium confidence. Require human review before committing to Codex.

```yaml
tier_2_criteria:
  - type: pattern
    condition: auto_detected
    
  - type: observation
    condition: any
    
  - type: failure
    condition: judge_attribution == knowledge_gap AND judge_confidence < high
    
  - type: any
    condition: promotion_to_domain_or_global
```

**Staging Queue UX:**

```
┌─────────────────────────────────────────────────────────────────────────┐
│  STAGING QUEUE (3 items pending review)                                  │
│                                                                          │
│  1. [Pattern] Repository pattern for data access                        │
│     Source: Session Jan 20 (auto-detected)                              │
│     Confidence: 0.72                                                    │
│     ⚠️ Similar existing: "Data access abstraction" (78% match)          │
│     [Promote] [Merge with existing] [Discard]                           │
│                                                                          │
│  2. [Observation] Service X times out under heavy load                  │
│     Source: Judge attribution (knowledge_gap, medium confidence)        │
│     Confidence: 0.65                                                    │
│     [Promote] [Edit] [Discard]                                          │
│                                                                          │
│  3. [Decision] Use Redis for session caching (PROMOTING TO GLOBAL)      │
│     Source: Session Jan 22 (user requested promotion)                   │
│     Current scope: project → global                                     │
│     [Approve promotion] [Keep project-scoped] [Discard]                 │
└─────────────────────────────────────────────────────────────────────────┘
```

### Tier 3: Flight Recorder Only

Low confidence or not generalizable. Stay in flight recorder unless explicitly promoted.

```yaml
tier_3_criteria:
  - type: decision
    condition: confidence < 0.7 AND NOT has_rationale
    
  - type: self_correction
    condition: first_occurrence  # Promote if pattern emerges
    
  - type: any
    condition: session_specific_only
```

**Promotion from Tier 3:**

Items can be promoted from flight recorder to Codex if:
- Same type of self-correction occurs 3+ times → auto-suggest promotion
- User explicitly requests: "Save this to Codex"
- Pattern analysis identifies recurring theme

### Staging Queue Management

```go
package staging

// Queue manages items pending review before Codex commit
type Queue struct {
    store   *storage.StagingStore
    config  *config.StagingConfig
}

// StagedItem is a capture candidate awaiting review
type StagedItem struct {
    ID          string
    Candidate   *capture.Candidate
    StagedAt    time.Time
    Source      string            // "auto_detected", "judge_knowledge_gap", "promotion_request"
    SimilarTo   []string          // IDs of similar existing items (from dedup check)
    ExpiresAt   time.Time         // Auto-discard if not reviewed
}

// Config for staging behavior
type StagingConfig struct {
    Enabled         bool          // v0: false, v1: true
    MaxQueueSize    int           // Default: 50
    ExpirationDays  int           // Auto-discard after N days (default: 14)
    ReviewReminder  bool          // Remind user of pending items
}

// Add places an item in the staging queue
func (q *Queue) Add(ctx context.Context, candidate *capture.Candidate, source string) error {
    // Check queue size
    if q.count() >= q.config.MaxQueueSize {
        return fmt.Errorf("staging queue full (%d items); review pending items first", q.config.MaxQueueSize)
    }

    item := &StagedItem{
        ID:        generateID(),
        Candidate: candidate,
        StagedAt:  time.Now(),
        Source:    source,
        ExpiresAt: time.Now().AddDate(0, 0, q.config.ExpirationDays),
    }

    // Check for similar existing items (deduplication preview)
    similar, _ := q.findSimilar(ctx, candidate)
    item.SimilarTo = similar

    return q.store.Save(item)
}

// Promote moves an item from staging to Codex
func (q *Queue) Promote(ctx context.Context, itemID string, edits *CandidateEdits) (string, error) {
    item, err := q.store.Get(itemID)
    if err != nil {
        return "", err
    }

    // Apply any edits
    if edits != nil {
        item.Candidate.Summary = edits.Summary
        item.Candidate.Detail = edits.Detail
    }

    // Save to Codex via RECALL
    result, err := q.recall.Add(ctx, candidateToAddOptions(item.Candidate))
    if err != nil {
        return "", err
    }

    // Remove from staging
    q.store.Delete(itemID)

    return result.ID, nil
}

// Discard removes an item without saving
func (q *Queue) Discard(ctx context.Context, itemID string, reason string) error {
    // Log discarded items for analysis (what did humans reject?)
    item, _ := q.store.Get(itemID)
    if item != nil {
        q.logDiscard(item, reason)
    }
    return q.store.Delete(itemID)
}

// Cleanup removes expired items
func (q *Queue) Cleanup(ctx context.Context) (int, error) {
    expired, err := q.store.FindExpired(time.Now())
    if err != nil {
        return 0, err
    }

    for _, item := range expired {
        q.Discard(ctx, item.ID, "expired")
    }

    return len(expired), nil
}
```

---

## 2. LLM Judge Prompt

Concrete prompt for attributing user corrections.

### Trigger

The judge is invoked when:
1. User explicitly corrects Claude (detected via conversation patterns)
2. Claude self-corrects after an error
3. A captured failure needs attribution

### Input Context

```yaml
judge_input:
  # What happened
  claude_action: "What Claude did or generated"
  error_or_correction: "The error that occurred or correction user provided"
  user_message: "Exact user message if it was a correction"
  
  # What was available
  session_context: "What was in Claude's context at time of error"
  codex_search_results: "Results from searching Codex for relevant terms"
  codex_had_relevant: boolean  # Did Codex contain relevant info?
  relevant_item_ids: [string]  # Which items were relevant?
  items_in_context: [string]   # Which of those were actually in session context?
  
  # Session metadata
  session_id: string
  timestamp: timestamp
  files_involved: [string]
```

### Judge Prompt

```markdown
You are evaluating whether a user correction to Claude was preventable with existing knowledge.

## What Happened

Claude's action:
{claude_action}

Error or correction:
{error_or_correction}

User's exact message (if correction):
{user_message}

## What Was Available

### In Claude's session context at time of error:
{session_context}

### Codex search for relevant terms returned:
{codex_search_results}

### Analysis:
- Codex contained relevant information: {codex_had_relevant}
- Relevant item IDs: {relevant_item_ids}
- Items that were in session context: {items_in_context}

## Your Task

Determine the attribution for this error. Choose ONE:

1. **knowledge_gap**: No relevant knowledge exists in Codex. This is genuinely new information that should be captured.

2. **retrieval_miss**: Relevant knowledge EXISTS in Codex (items: {relevant_item_ids}) but was NOT in session context. The retrieval system failed to surface it.

3. **reasoning_error**: Relevant knowledge WAS in session context (items: {items_in_context}), but Claude did not apply it correctly.

4. **requirements_ambig**: The user's original request was ambiguous. Claude's interpretation was reasonable given the information provided.

5. **transient**: This was an environmental issue (timeout, rate limit, etc.) not a knowledge or reasoning problem.

6. **novel**: This is a unique edge case that doesn't generalize. Not worth capturing.

## Response Format

Respond in JSON:

```json
{
  "attribution": "<one of the six types>",
  "confidence": "high" | "medium" | "low",
  "evidence": "<2-3 sentence explanation of why you chose this attribution>",
  "relevant_item_id": "<if retrieval_miss or reasoning_error, which item should have helped>",
  "suggested_action": "<what should happen next>",
  "capture_recommended": true | false,
  "capture_summary": "<if capture_recommended, one-line summary for new knowledge item>"
}
```

## Guidelines

- Be conservative: if uncertain between retrieval_miss and knowledge_gap, prefer knowledge_gap (safer to capture than miss)
- For reasoning_error, be specific about what Claude should have done differently
- Only mark requirements_ambig if the user's request was genuinely unclear
- Consider transient only for clear environmental issues, not for bugs
```

### Judge Response Handling

```go
package judge

// Attribution is the judge's classification
type Attribution string

const (
    KnowledgeGap     Attribution = "knowledge_gap"
    RetrievalMiss    Attribution = "retrieval_miss"
    ReasoningError   Attribution = "reasoning_error"
    RequirementsAmbig Attribution = "requirements_ambig"
    Transient        Attribution = "transient"
    Novel            Attribution = "novel"
)

// JudgeResult is the structured response from the judge
type JudgeResult struct {
    Attribution       Attribution `json:"attribution"`
    Confidence        string      `json:"confidence"` // high, medium, low
    Evidence          string      `json:"evidence"`
    RelevantItemID    string      `json:"relevant_item_id,omitempty"`
    SuggestedAction   string      `json:"suggested_action"`
    CaptureRecommended bool       `json:"capture_recommended"`
    CaptureSummary    string      `json:"capture_summary,omitempty"`
}

// Router determines action based on judge result
type Router struct {
    staging  *staging.Queue
    recall   *recall.Client
    feedback *feedback.Store
}

// Route processes a judge result
func (r *Router) Route(ctx context.Context, result *JudgeResult, original *CorrectionEvent) error {
    switch result.Attribution {
    case KnowledgeGap:
        if result.Confidence == "high" && result.CaptureRecommended {
            // High confidence: stage for capture (Tier 2)
            return r.staging.Add(ctx, &capture.Candidate{
                Type:       capture.CandidateObservation,
                Summary:    result.CaptureSummary,
                Confidence: 0.8,
            }, "judge_knowledge_gap")
        }
        // Low/medium confidence: log for later review
        return r.logForReview(result, original)

    case RetrievalMiss:
        // Log as retrieval feedback for tuning
        return r.feedback.LogRetrievalMiss(ctx, &feedback.RetrievalMiss{
            Query:            original.ContextQuery,
            ShouldHaveFound:  result.RelevantItemID,
            ActuallyReturned: original.CodexResults,
            SessionID:        original.SessionID,
        })

    case ReasoningError:
        // Always escalate - needs human review
        return r.escalate(ctx, result, original)

    case RequirementsAmbig:
        // Log but no action
        return r.logForAnalysis(result, original)

    case Transient:
        // Ignore
        return nil

    case Novel:
        // Log but no automatic capture
        return r.logForAnalysis(result, original)
    }

    return nil
}
```

---

## 3. Archival Workflow

How to clean up stale, low-value, or redundant items from Codex.

### Archival Triggers

| Trigger | Condition | Action |
|---------|-----------|--------|
| **Low usefulness** | `usefulness_rate < 0.2` after 10+ retrievals | Flag for review |
| **Stale** | Not retrieved in 180 days AND `freshness < 0.3` | Flag for review |
| **Duplicate detected** | User marks via `recall_feedback(duplicate_of=X)` | Merge or remove |
| **Capacity warning** | Scope at 80% capacity | Suggest cleanup |
| **Contradicted** | New knowledge explicitly supersedes | Mark superseded |

### Archival States

```
Active ──────▶ Flagged ──────▶ Archived ──────▶ Deleted
              (review)        (hidden from      (permanent)
                              search)
```

```yaml
knowledge_item:
  status: active | flagged | archived | deleted
  flagged_reason: low_usefulness | stale | duplicate | superseded | user_requested
  flagged_at: timestamp | null
  archived_at: timestamp | null
  superseded_by: item_id | null
```

### Cleanup Command

```bash
# Suggest items for archival
$ edi recall cleanup --suggest

📦 RECALL Cleanup Suggestions

Low usefulness (retrieved but rarely helpful):
  1. [Observation] "Legacy API timeout" — 5% useful over 12 retrievals
  2. [Pattern] "Retry pattern v1" — 10% useful, superseded by ADR-031

Stale (not retrieved in 6+ months):
  3. [Decision] "Initial auth approach" — last retrieved 8 months ago
  4. [Evidence] "Benchmark from v1.0" — outdated metrics

Duplicates flagged:
  5. [Decision] "Use Redis caching" — duplicate of item #42

Archive these? [a]ll / [s]elect / [r]eview each / [n]one: 
```

### Review UX

```
┌─────────────────────────────────────────────────────────────────────────┐
│  REVIEWING: [Observation] "Legacy API timeout"                           │
│                                                                          │
│  Summary: Legacy auth API times out under load                          │
│  Created: 6 months ago                                                  │
│  Last retrieved: 3 months ago                                           │
│                                                                          │
│  Stats:                                                                 │
│  - Retrieved 12 times                                                   │
│  - Marked useful: 1 time (8%)                                           │
│  - Marked not_useful: 5 times                                           │
│  - No feedback: 6 times                                                 │
│                                                                          │
│  Flagged reason: low_usefulness                                         │
│                                                                          │
│  [Archive] [Keep (reset stats)] [Edit & Keep] [Delete permanently]      │
└─────────────────────────────────────────────────────────────────────────┘
```

### Archival Implementation

```go
package archival

// Archiver manages the cleanup workflow
type Archiver struct {
    recall *recall.Client
    config *config.ArchivalConfig
}

type ArchivalConfig struct {
    LowUsefulnessThreshold float64 // Default: 0.2
    MinRetrievalsForReview int     // Default: 10
    StaleDays              int     // Default: 180
    FreshnessThreshold     float64 // Default: 0.3
}

// FindCandidates returns items that may need archival
func (a *Archiver) FindCandidates(ctx context.Context, scope string) (*ArchivalCandidates, error) {
    candidates := &ArchivalCandidates{}

    // Low usefulness
    lowUsefulness, _ := a.recall.Query(ctx, `
        SELECT * FROM knowledge_items 
        WHERE status = 'active'
          AND retrieval_count >= ?
          AND usefulness_rate < ?
          AND scope = ?
    `, a.config.MinRetrievalsForReview, a.config.LowUsefulnessThreshold, scope)
    candidates.LowUsefulness = lowUsefulness

    // Stale
    staleCutoff := time.Now().AddDate(0, 0, -a.config.StaleDays)
    stale, _ := a.recall.Query(ctx, `
        SELECT * FROM knowledge_items
        WHERE status = 'active'
          AND last_retrieved < ?
          AND freshness < ?
          AND scope = ?
    `, staleCutoff, a.config.FreshnessThreshold, scope)
    candidates.Stale = stale

    // Flagged duplicates
    duplicates, _ := a.recall.Query(ctx, `
        SELECT * FROM knowledge_items
        WHERE status = 'active'
          AND flagged_duplicate_of IS NOT NULL
          AND scope = ?
    `, scope)
    candidates.Duplicates = duplicates

    return candidates, nil
}

// Archive moves an item to archived status
func (a *Archiver) Archive(ctx context.Context, itemID string, reason string) error {
    return a.recall.Update(ctx, itemID, map[string]interface{}{
        "status":        "archived",
        "flagged_reason": reason,
        "archived_at":   time.Now(),
    })
}

// Restore returns an archived item to active status
func (a *Archiver) Restore(ctx context.Context, itemID string) error {
    return a.recall.Update(ctx, itemID, map[string]interface{}{
        "status":        "active",
        "flagged_reason": nil,
        "archived_at":   nil,
        // Reset stats to give it a fresh start
        "retrieval_count":  0,
        "useful_count":     0,
        "not_useful_count": 0,
    })
}
```

---

## 4. v0 vs v1 Scope

### v0: Minimal Noise Control

| Feature | v0 Implementation |
|---------|-------------------|
| **Staging tiers** | ❌ All approved captures go direct to Codex |
| **Deduplication** | ✅ Exact match on normalized summary |
| **Capacity limits** | ✅ Warn at threshold, block at limit |
| **Usefulness tracking** | ✅ Store feedback, no automatic action |
| **LLM Judge** | ❌ Log corrections only, no attribution |
| **Archival workflow** | ⚠️ Manual via `edi recall cleanup` |

### v1: Full Noise Control

| Feature | v1 Implementation |
|---------|-------------------|
| **Staging tiers** | ✅ Tier 2 items go to staging queue |
| **Deduplication** | ✅ Semantic similarity via Codex embeddings |
| **Capacity limits** | ✅ With automated review workflow |
| **Usefulness tracking** | ✅ Auto-flag low usefulness items |
| **LLM Judge** | ✅ Full attribution with routing |
| **Archival workflow** | ✅ Triggered by usefulness + staleness signals |

### Migration Path

```
v0 deployed
      │
      ▼
Accumulate feedback data (usefulness, corrections)
      │
      ▼
Analyze patterns (what gets flagged, what's useful)
      │
      ▼
Tune thresholds based on real data
      │
      ▼
v1 deployed with validated thresholds
```

---

## 5. Configuration

```yaml
# .edi/config.yaml

learning:
  # Staging (v1 only)
  staging:
    enabled: false  # v0: false, v1: true
    max_queue_size: 50
    expiration_days: 14
    
  # Deduplication
  deduplication:
    enabled: true
    method: exact_match  # v0: exact_match, v1: semantic
    similarity_threshold: 0.85  # for semantic matching
    
  # Capacity
  capacity:
    project_limit: 500
    global_limit: 1000
    warning_percent: 0.8
    
  # Usefulness tracking
  usefulness:
    enabled: true
    auto_flag_threshold: 0.2   # flag if below this after min_retrievals
    min_retrievals_to_flag: 10
    
  # Archival
  archival:
    stale_days: 180
    freshness_threshold: 0.3
    auto_suggest: true  # suggest cleanup when approaching capacity
    
  # LLM Judge (v1 only)
  judge:
    enabled: false  # v0: false, v1: true
    model: claude-3-haiku
    confidence_threshold_for_auto: high
    
  # Feedback
  feedback:
    log_corrections: true  # v0: just log, v1: route to judge
    log_retrieval_misses: true
```

---

## Summary

This addendum specifies:

1. **Staging Tiers** — Tier 1 (direct), Tier 2 (staged), Tier 3 (flight recorder)
2. **LLM Judge** — Complete prompt and routing logic for correction attribution
3. **Archival Workflow** — Triggers, states, cleanup command, and review UX
4. **v0 vs v1** — Clear scope split with migration path

The goal is defense in depth against noise, with minimal friction in v0 and full automation in v1.
