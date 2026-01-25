# Learning Architecture Implementation Plan

**Version**: 1.0  
**Created**: January 25, 2026  
**Target**: Claude Code execution  
**Prerequisites**: EDI Phase 2 complete, Codex v1 operational  
**Estimated Duration**: 4 weeks

---

## Executive Summary

The Learning Architecture implements EDI's ability to **capture, attribute, and retrieve knowledge** with quality controls. It addresses the signal-to-noise problem in AI memory systems.

### Core Differentiators

| Problem | Solution |
|---------|----------|
| Auto-capture drowns in noise | Typed knowledge with confidence tiers |
| Manual capture requires discipline | Auto-suggestion with human approval |
| No feedback on retrieval quality | Usefulness scoring and decay |
| Failures repeat across sessions | LLM judge for failure attribution |
| Knowledge gets stale | Freshness scoring and re-verification |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      LEARNING ARCHITECTURE                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  SESSION                                                                │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                                                                    │ │
│  │  Flight Recorder (continuous)     Capture Detector (selective)    │ │
│  │  ─────────────────────────       ───────────────────────────      │ │
│  │  • All tool calls                 • Decision patterns             │ │
│  │  • Sandbox telemetry              • Self-corrections              │ │
│  │  • Errors and retries             • New patterns discovered       │ │
│  │                                   • Failures with resolutions     │ │
│  │                                                                    │ │
│  │                      ↓                         ↓                   │ │
│  │              (low-level log)         (capture candidates)         │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                          ↓                              │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  CAPTURE WORKFLOW                                                  │ │
│  │                                                                    │ │
│  │  Candidates → Friction Budget → Human Approval → Type + Scope     │ │
│  │                                                                    │ │
│  │  Friction Budget: Max 3 prompts per session                       │ │
│  │  Capture prompts only for high-confidence candidates              │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                          ↓                              │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  CODEX (Knowledge Store)                                          │ │
│  │                                                                    │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐ │ │
│  │  │ Evidence │ │ Decision │ │ Pattern  │ │ Observ.  │ │ Failure │ │ │
│  │  │ (Sandbox │ │ (Human   │ │ (General │ │ (Noted   │ │ (Prevent│ │ │
│  │  │ verified)│ │ approved)│ │ -izable) │ │ unverif.)│ │ -able)  │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └─────────┘ │ │
│  │       ↑                                                    ↑      │ │
│  │       │                    FRESHNESS                       │      │ │
│  │       └────────────────── MANAGEMENT ──────────────────────┘      │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                          ↓                              │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  RETRIEVAL + FEEDBACK                                             │ │
│  │                                                                    │ │
│  │  Query → Hybrid Search → Type Weighting → Freshness Weighting    │ │
│  │                                    ↓                              │ │
│  │                              Results                              │ │
│  │                                    ↓                              │ │
│  │                          Feedback Loop                            │ │
│  │                    (useful? → score adjustment)                   │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  FAILURE ATTRIBUTION (LLM Judge)                                  │ │
│  │                                                                    │ │
│  │  Failure → Classify → Determine Preventability → Route            │ │
│  │                                                                    │ │
│  │  Categories: requirements_ambig, knowledge_gap, tool_misuse,      │ │
│  │              context_missing, execution_error, external_dep       │ │
│  │                                                                    │ │
│  │  Routing: preventable → Codex | not preventable → skip            │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
learning/
├── cmd/
│   └── learning/
│       └── main.go              # Service entry point
├── internal/
│   ├── capture/                 # Capture workflow
│   │   ├── detector.go          # Detect capture candidates
│   │   ├── friction.go          # Friction budget management
│   │   └── workflow.go          # Human approval flow
│   ├── types/                   # Knowledge types
│   │   ├── schema.go            # Type definitions
│   │   ├── confidence.go        # Confidence scoring
│   │   └── scope.go             # Scope hierarchy
│   ├── attribution/             # Failure attribution
│   │   ├── judge.go             # LLM judge
│   │   ├── categories.go        # Failure categories
│   │   └── routing.go           # Escalation routing
│   ├── freshness/               # Staleness management
│   │   ├── scoring.go           # Freshness scores
│   │   ├── decay.go             # Score decay
│   │   └── verification.go      # Re-verification triggers
│   ├── retrieval/               # Retrieval enhancements
│   │   ├── weighting.go         # Type and freshness weighting
│   │   └── feedback.go          # Usefulness feedback
│   └── api/                     # API endpoints
│       ├── server.go
│       └── handlers.go
├── pkg/
│   └── types/
│       └── knowledge.go
├── go.mod
└── Makefile
```

---

## Phase 1: Knowledge Type System (Week 1)

### Goal
Implement typed knowledge with confidence tiers and scope hierarchy.

### 1.1 Type Definitions

```go
// internal/types/schema.go
package types

import "time"

type KnowledgeType string

const (
    TypeEvidence    KnowledgeType = "evidence"    // Sandbox-verified
    TypeDecision    KnowledgeType = "decision"    // Human-approved
    TypePattern     KnowledgeType = "pattern"     // Generalizable
    TypeObservation KnowledgeType = "observation" // Noted, unverified
    TypeFailure     KnowledgeType = "failure"     // Preventable, with resolution
)

// Confidence tiers (for retrieval weighting)
var TypeConfidence = map[KnowledgeType]float64{
    TypeEvidence:    1.0,  // Highest - Sandbox verified
    TypeDecision:    0.9,  // Human approved
    TypePattern:     0.75, // Generalizable but not verified
    TypeObservation: 0.5,  // May be incorrect
    TypeFailure:     0.85, // Important to surface
}

type KnowledgeItem struct {
    ID          string
    Type        KnowledgeType
    Title       string
    Content     string
    Tags        []string
    
    // Scope
    Scope       Scope
    ProjectPath string
    
    // Metadata
    Source      Source
    CreatedAt   time.Time
    UpdatedAt   time.Time
    
    // Quality signals
    Confidence  float64
    Freshness   float64
    UseCount    int
    Usefulness  float64
    
    // For failures
    FailureAttribution *FailureAttribution
    Resolution         string
    
    // For evidence
    SandboxVerification *SandboxVerification
}

type Scope string

const (
    ScopeProject Scope = "project" // Project-specific
    ScopeDomain  Scope = "domain"  // Cross-project domain
    ScopeGlobal  Scope = "global"  // Organization-wide
)

type Source struct {
    Type      string // session, sandbox, manual, migration
    SessionID string
    Timestamp time.Time
}
```

### 1.2 Confidence Scoring

```go
// internal/types/confidence.go
package types

type ConfidenceFactors struct {
    TypeWeight      float64 // Base weight from type
    VerificationBonus float64 // +0.1 if Sandbox verified
    FreshnessWeight float64 // Decays over time
    UsefulnessWeight float64 // Based on feedback
}

func CalculateConfidence(item *KnowledgeItem) float64 {
    base := TypeConfidence[item.Type]
    
    // Verification bonus
    if item.SandboxVerification != nil && item.SandboxVerification.Passed {
        base += 0.1
    }
    
    // Freshness decay
    ageMonths := time.Since(item.CreatedAt).Hours() / (24 * 30)
    freshnessMultiplier := 1.0 / (1.0 + (ageMonths * 0.1))
    
    // Usefulness adjustment
    usefulnessMultiplier := 1.0
    if item.UseCount > 0 {
        usefulnessMultiplier = 0.5 + (item.Usefulness / float64(item.UseCount) * 0.5)
    }
    
    return base * freshnessMultiplier * usefulnessMultiplier
}
```

### 1.3 Scope Hierarchy

```go
// internal/types/scope.go
package types

// Scope resolution for retrieval
// Query scope determines which items are visible

type ScopeResolver struct {
    projectPath string
    domain      string
}

func (r *ScopeResolver) VisibleScopes() []Scope {
    // Project sees: project + domain + global
    // Domain sees: domain + global
    // Global sees: global only
    
    return []Scope{ScopeProject, ScopeDomain, ScopeGlobal}
}

func (r *ScopeResolver) Filter(items []KnowledgeItem) []KnowledgeItem {
    visible := r.VisibleScopes()
    var filtered []KnowledgeItem
    
    for _, item := range items {
        for _, scope := range visible {
            if item.Scope == scope {
                // For project scope, also check path
                if scope == ScopeProject && item.ProjectPath != r.projectPath {
                    continue
                }
                filtered = append(filtered, item)
                break
            }
        }
    }
    
    return filtered
}
```

### 1.4 Validation Checkpoint

- [ ] All knowledge types defined
- [ ] Confidence scoring works
- [ ] Scope filtering works
- [ ] Type hierarchy enforced

---

## Phase 2: Capture Workflow (Week 2)

### Goal
Implement capture detection, friction budgeting, and approval flow.

### 2.1 Capture Detector

```go
// internal/capture/detector.go
package capture

import (
    "regexp"
    "strings"
)

type CaptureCandidate struct {
    Type        types.KnowledgeType
    Title       string
    Content     string
    Confidence  float64  // How confident we are this is worth capturing
    Source      string   // Where we detected this
    Tags        []string
}

type Detector struct {
    patterns []DetectionPattern
}

type DetectionPattern struct {
    Type       types.KnowledgeType
    Trigger    string   // Regex or keyword
    Confidence float64  // Base confidence for this pattern
    Extractor  func(match string, context string) *CaptureCandidate
}

func NewDetector() *Detector {
    return &Detector{
        patterns: []DetectionPattern{
            // Decision patterns
            {
                Type:       types.TypeDecision,
                Trigger:    `(?i)(decided|chose|selected|went with|opted for)`,
                Confidence: 0.8,
            },
            // Self-correction patterns
            {
                Type:       types.TypePattern,
                Trigger:    `(?i)(actually|wait|correction|better approach|instead)`,
                Confidence: 0.7,
            },
            // Failure patterns
            {
                Type:       types.TypeFailure,
                Trigger:    `(?i)(error|failed|bug|issue|problem|fix)`,
                Confidence: 0.6,
            },
            // Pattern discovery
            {
                Type:       types.TypePattern,
                Trigger:    `(?i)(pattern|approach|technique|method|strategy)`,
                Confidence: 0.5,
            },
        },
    }
}

func (d *Detector) Detect(content string, context string) []CaptureCandidate {
    var candidates []CaptureCandidate
    
    for _, pattern := range d.patterns {
        re := regexp.MustCompile(pattern.Trigger)
        if re.MatchString(content) {
            candidate := CaptureCandidate{
                Type:       pattern.Type,
                Content:    content,
                Confidence: pattern.Confidence,
                Source:     "auto-detect",
            }
            
            // Extract title
            candidate.Title = extractTitle(content, pattern.Type)
            
            candidates = append(candidates, candidate)
        }
    }
    
    return candidates
}

func extractTitle(content string, typ types.KnowledgeType) string {
    // First sentence or first 100 chars
    sentences := strings.SplitN(content, ".", 2)
    title := sentences[0]
    if len(title) > 100 {
        title = title[:100] + "..."
    }
    return title
}
```

### 2.2 Friction Budget

```go
// internal/capture/friction.go
package capture

type FrictionBudget struct {
    MaxPrompts     int
    PromptCount    int
    HighConfThreshold float64  // Only prompt if confidence >= this
}

func NewFrictionBudget(max int) *FrictionBudget {
    return &FrictionBudget{
        MaxPrompts:        max,
        PromptCount:       0,
        HighConfThreshold: 0.7, // Only prompt for 70%+ confidence
    }
}

func (f *FrictionBudget) ShouldPrompt(candidate *CaptureCandidate) bool {
    // Budget exhausted
    if f.PromptCount >= f.MaxPrompts {
        return false
    }
    
    // Below confidence threshold
    if candidate.Confidence < f.HighConfThreshold {
        return false
    }
    
    // Critical types always prompt (failures, evidence)
    if candidate.Type == types.TypeFailure || candidate.Type == types.TypeEvidence {
        return true
    }
    
    return true
}

func (f *FrictionBudget) UsePrompt() {
    f.PromptCount++
}

func (f *FrictionBudget) Remaining() int {
    return f.MaxPrompts - f.PromptCount
}
```

### 2.3 Capture Workflow

```go
// internal/capture/workflow.go
package capture

type CaptureWorkflow struct {
    detector *Detector
    budget   *FrictionBudget
    codex    *codex.Client
}

type CapturePrompt struct {
    Candidates []CaptureCandidate
    Message    string
}

func (w *CaptureWorkflow) ProcessSessionEnd(sessionContent string, decisions []string) (*CapturePrompt, error) {
    // Detect candidates
    candidates := w.detector.Detect(sessionContent, "")
    
    // Add explicit decisions
    for _, d := range decisions {
        candidates = append(candidates, CaptureCandidate{
            Type:       types.TypeDecision,
            Content:    d,
            Confidence: 0.9,
            Source:     "explicit",
        })
    }
    
    // Filter by friction budget
    var promptCandidates []CaptureCandidate
    for _, c := range candidates {
        if w.budget.ShouldPrompt(&c) {
            promptCandidates = append(promptCandidates, c)
        }
    }
    
    if len(promptCandidates) == 0 {
        return nil, nil // No prompt needed
    }
    
    // Build prompt
    prompt := &CapturePrompt{
        Candidates: promptCandidates,
        Message:    buildCaptureMessage(promptCandidates, w.budget.Remaining()),
    }
    
    return prompt, nil
}

func buildCaptureMessage(candidates []CaptureCandidate, remaining int) string {
    var sb strings.Builder
    
    sb.WriteString("Capture to RECALL? (")
    sb.WriteString(fmt.Sprintf("%d prompts remaining)\n\n", remaining))
    
    for i, c := range candidates {
        sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", i+1, c.Type, c.Title))
    }
    
    sb.WriteString("\n[A] All  [S] Skip  [1-N] Select specific")
    
    return sb.String()
}

func (w *CaptureWorkflow) ProcessApproval(selection string, candidates []CaptureCandidate) error {
    w.budget.UsePrompt()
    
    var toCapture []CaptureCandidate
    
    switch strings.ToLower(selection) {
    case "a", "all":
        toCapture = candidates
    case "s", "skip":
        return nil
    default:
        // Parse numbers
        indices := parseSelection(selection)
        for _, i := range indices {
            if i > 0 && i <= len(candidates) {
                toCapture = append(toCapture, candidates[i-1])
            }
        }
    }
    
    // Save to Codex
    for _, c := range toCapture {
        w.codex.Add(&types.KnowledgeItem{
            Type:    c.Type,
            Title:   c.Title,
            Content: c.Content,
            Tags:    c.Tags,
            Source:  types.Source{Type: c.Source},
        })
    }
    
    return nil
}
```

### 2.4 Validation Checkpoint

- [ ] Capture detector finds decisions, patterns, failures
- [ ] Friction budget limits prompts to 3/session
- [ ] Approval flow works (All, Skip, Select)
- [ ] Approved items saved to Codex

---

## Phase 3: Failure Attribution (Week 3)

### Goal
LLM judge for failure classification and preventability analysis.

### 3.1 Failure Categories

```go
// internal/attribution/categories.go
package attribution

type FailureCategory string

const (
    CategoryRequirementsAmbig FailureCategory = "requirements_ambig"
    CategoryKnowledgeGap      FailureCategory = "knowledge_gap"
    CategoryToolMisuse        FailureCategory = "tool_misuse"
    CategoryContextMissing    FailureCategory = "context_missing"
    CategoryExecutionError    FailureCategory = "execution_error"
    CategoryExternalDep       FailureCategory = "external_dep"
)

type FailureAttribution struct {
    Category        FailureCategory
    Preventable     bool
    PreventableWith string // What knowledge would have prevented this
    RootCause       string
    Resolution      string
    Confidence      float64
}

var CategoryPreventability = map[FailureCategory]bool{
    CategoryRequirementsAmbig: false, // Needs human clarification
    CategoryKnowledgeGap:      true,  // Codex could have had this
    CategoryToolMisuse:        true,  // Pattern could prevent
    CategoryContextMissing:    true,  // Better context retrieval
    CategoryExecutionError:    false, // Random/transient
    CategoryExternalDep:       false, // Outside our control
}
```

### 3.2 LLM Judge

```go
// internal/attribution/judge.go
package attribution

import (
    "context"
    "encoding/json"
    
    "github.com/anthropics/anthropic-sdk-go"
)

type Judge struct {
    client *anthropic.Client
}

func NewJudge(apiKey string) *Judge {
    return &Judge{
        client: anthropic.NewClient(anthropic.WithAPIKey(apiKey)),
    }
}

func (j *Judge) Attribute(ctx context.Context, failure FailureReport) (*FailureAttribution, error) {
    prompt := buildAttributionPrompt(failure)
    
    resp, err := j.client.Messages.New(ctx, anthropic.MessageNewParams{
        Model:     anthropic.F(anthropic.ModelClaude3Haiku20240307),
        MaxTokens: anthropic.Int(500),
        Messages: anthropic.F([]anthropic.MessageParam{
            anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
        }),
    })
    if err != nil {
        return nil, err
    }
    
    // Parse structured response
    var attribution FailureAttribution
    if err := json.Unmarshal([]byte(resp.Content[0].Text), &attribution); err != nil {
        return nil, err
    }
    
    // Apply preventability rules
    attribution.Preventable = CategoryPreventability[attribution.Category]
    
    return &attribution, nil
}

func buildAttributionPrompt(failure FailureReport) string {
    return fmt.Sprintf(`Analyze this failure and classify it.

<failure>
What happened: %s
Error message: %s
Context: %s
Resolution: %s
</failure>

Respond with JSON:
{
  "category": "requirements_ambig|knowledge_gap|tool_misuse|context_missing|execution_error|external_dep",
  "root_cause": "Brief root cause",
  "resolution": "How it was resolved",
  "preventable_with": "What knowledge would have prevented this (if applicable)",
  "confidence": 0.0-1.0
}

Categories:
- requirements_ambig: User's request was unclear
- knowledge_gap: Missing pattern/decision in Codex
- tool_misuse: Used tool incorrectly
- context_missing: Relevant context not retrieved
- execution_error: Random/transient failure
- external_dep: External service issue

JSON only, no explanation:`, 
        failure.Description, failure.Error, failure.Context, failure.Resolution)
}
```

### 3.3 Routing

```go
// internal/attribution/routing.go
package attribution

type Router struct {
    codex *codex.Client
}

type RoutingDecision struct {
    Action      string // capture, skip, escalate
    Destination string // codex, human, none
    Item        *types.KnowledgeItem
}

func (r *Router) Route(attr *FailureAttribution, failure FailureReport) RoutingDecision {
    if !attr.Preventable {
        return RoutingDecision{
            Action:      "skip",
            Destination: "none",
        }
    }
    
    // Build knowledge item from failure
    item := &types.KnowledgeItem{
        Type:    types.TypeFailure,
        Title:   failure.Title,
        Content: failure.Description,
        Tags:    []string{string(attr.Category)},
        FailureAttribution: attr,
        Resolution: attr.Resolution,
    }
    
    // High confidence -> auto-capture
    if attr.Confidence >= 0.85 {
        return RoutingDecision{
            Action:      "capture",
            Destination: "codex",
            Item:        item,
        }
    }
    
    // Medium confidence -> human review
    if attr.Confidence >= 0.6 {
        return RoutingDecision{
            Action:      "capture",
            Destination: "human", // Will be prompted
            Item:        item,
        }
    }
    
    // Low confidence -> skip
    return RoutingDecision{
        Action:      "skip",
        Destination: "none",
    }
}
```

### 3.4 Validation Checkpoint

- [ ] Judge classifies failures correctly
- [ ] Preventability determined by category
- [ ] High-confidence failures auto-captured
- [ ] Medium-confidence failures prompted
- [ ] Low-confidence/non-preventable skipped

---

## Phase 4: Freshness & Retrieval (Week 4)

### Goal
Freshness scoring, decay, and retrieval weighting.

### 4.1 Freshness Scoring

```go
// internal/freshness/scoring.go
package freshness

import (
    "math"
    "time"
)

type FreshnessConfig struct {
    HalfLifeDays  float64 // Score halves every N days
    MinFreshness  float64 // Floor (never goes below this)
    VerifyAfterDays int   // Trigger re-verification after N days
}

var DefaultConfig = FreshnessConfig{
    HalfLifeDays:    90,   // Quarterly half-life
    MinFreshness:    0.1,  // Never below 10%
    VerifyAfterDays: 180,  // Re-verify after 6 months
}

func CalculateFreshness(item *types.KnowledgeItem, config FreshnessConfig) float64 {
    ageDays := time.Since(item.UpdatedAt).Hours() / 24
    
    // Exponential decay
    freshness := math.Pow(0.5, ageDays/config.HalfLifeDays)
    
    // Apply floor
    if freshness < config.MinFreshness {
        freshness = config.MinFreshness
    }
    
    // Boost if recently used
    if item.UseCount > 0 {
        lastUseDays := time.Since(item.LastUsedAt).Hours() / 24
        if lastUseDays < 30 {
            freshness *= 1.2 // 20% boost for recent use
        }
    }
    
    // Cap at 1.0
    if freshness > 1.0 {
        freshness = 1.0
    }
    
    return freshness
}

func NeedsVerification(item *types.KnowledgeItem, config FreshnessConfig) bool {
    ageDays := time.Since(item.UpdatedAt).Hours() / 24
    return ageDays > float64(config.VerifyAfterDays) && item.SandboxVerification == nil
}
```

### 4.2 Retrieval Weighting

```go
// internal/retrieval/weighting.go
package retrieval

type RetrievalWeighting struct {
    TypeWeight      float64 // From type confidence
    FreshnessWeight float64 // From freshness score
    UsefulnessWeight float64 // From feedback
    RecencyWeight   float64 // Boost recent items
}

func CalculateRetrievalScore(item *types.KnowledgeItem, baseScore float64) float64 {
    // Base score from search (vector/BM25/rerank)
    score := baseScore
    
    // Type confidence weight (0.5 - 1.0)
    typeWeight := types.TypeConfidence[item.Type]
    score *= typeWeight
    
    // Freshness weight (0.1 - 1.0)
    freshness := freshness.CalculateFreshness(item, freshness.DefaultConfig)
    score *= freshness
    
    // Usefulness weight (0.5 - 1.5)
    usefulnessWeight := 1.0
    if item.UseCount > 0 {
        usefulnessWeight = 0.5 + (item.Usefulness / float64(item.UseCount))
    }
    score *= usefulnessWeight
    
    return score
}
```

### 4.3 Feedback Loop

```go
// internal/retrieval/feedback.go
package retrieval

type FeedbackCollector struct {
    codex *codex.Client
}

type Feedback struct {
    ItemID    string
    SessionID string
    Useful    bool
    Context   string
}

func (f *FeedbackCollector) RecordFeedback(feedback Feedback) error {
    item, err := f.codex.Get(feedback.ItemID)
    if err != nil {
        return err
    }
    
    // Update counters
    item.UseCount++
    if feedback.Useful {
        item.Usefulness++
    }
    
    // Recalculate confidence
    item.Confidence = types.CalculateConfidence(item)
    
    return f.codex.Update(item)
}

// Called after each retrieval use
func (f *FeedbackCollector) PromptForFeedback(items []types.KnowledgeItem) string {
    if len(items) == 0 {
        return ""
    }
    
    // Only prompt for top result occasionally (1 in 5)
    if rand.Intn(5) != 0 {
        return ""
    }
    
    return fmt.Sprintf("Was '%s' helpful? [Y/N/skip]", items[0].Title)
}
```

### 4.4 Validation Checkpoint

- [ ] Freshness scores decay over time
- [ ] Retrieval weights by type, freshness, usefulness
- [ ] Feedback updates item scores
- [ ] Stale items flagged for re-verification

---

## Integration with EDI

### Add to edi-core skill

```markdown
## Learning Integration

### Auto-Capture Detection
EDI detects capture candidates during work:
- Explicit decisions ("decided to", "chose", "went with")
- Self-corrections ("actually", "better approach")
- Failures with resolutions

### At Session End
EDI prompts for capture (respecting friction budget):
```
Capture to RECALL? (2 prompts remaining)

[1] Decision: Use Stripe webhooks for payment confirmation
[2] Pattern: Exponential backoff with jitter for retries
[3] Failure: Memory leak from unbounded retry queue

[A] All  [S] Skip  [1-3] Select
```

### Failure Attribution
When you encounter and resolve failures:
1. EDI classifies the failure
2. If preventable, suggests capture
3. Links resolution to failure

### Retrieval Quality
When RECALL results are used:
- EDI may ask "Was this helpful?"
- Feedback improves future retrieval
```

---

## Dependencies

```go
require (
    github.com/anthropics/anthropic-sdk-go v0.1.0
)
```

**External Services:**
- Anthropic API (Haiku for attribution)

---

## Environment Variables

```bash
ANTHROPIC_API_KEY=sk-ant-xxx
LEARNING_FRICTION_BUDGET=3
LEARNING_FRESHNESS_HALFLIFE_DAYS=90
LEARNING_VERIFY_AFTER_DAYS=180
```
