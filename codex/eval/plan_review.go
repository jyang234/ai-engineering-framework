package eval

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// PlanReviewHarness tests whether RECALL surfaces relevant past failures
// when reviewing architectural plans (Experiment 2B).
type PlanReviewHarness struct {
	*EvalHarness
	anthropic *AnthropicClient
}

// PlanScenario represents a single plan-review test scenario.
type PlanScenario struct {
	ID          string `json:"id"`
	Plan        string `json:"plan"`         // Architectural plan text
	IsDangerous bool   `json:"is_dangerous"` // True if plan re-introduces a known failure
	FailureID   string `json:"failure_id"`   // ID of the relevant seeded failure (empty for safe plans)
	FailureDesc string `json:"failure_desc"` // Description of the failure to detect
}

// PlanReviewResult holds the result of evaluating one plan scenario.
type PlanReviewResult struct {
	ScenarioID      string `json:"scenario_id"`
	IsDangerous     bool   `json:"is_dangerous"`
	FailureDetected bool   `json:"failure_detected"` // Did the review surface the relevant failure?
	FalseAlarm      bool   `json:"false_alarm"`      // Was a safe plan incorrectly flagged?
	ReviewOutput    string `json:"review_output"`
}

// PlanReviewSummary aggregates plan review evaluation results.
type PlanReviewSummary struct {
	TotalScenarios int                `json:"total_scenarios"`
	DangerousCount int                `json:"dangerous_count"`
	SafeCount      int                `json:"safe_count"`
	DetectionRate  float64            `json:"detection_rate"`   // % dangerous plans where failure was surfaced
	FalseAlarmRate float64            `json:"false_alarm_rate"` // % safe plans incorrectly flagged
	Results        []PlanReviewResult `json:"results"`
}

// NewPlanReviewHarness creates a plan review harness.
func NewPlanReviewHarness(ctx context.Context, apiKey string) (*PlanReviewHarness, error) {
	base, err := NewEvalHarness(ctx)
	if err != nil {
		return nil, err
	}

	return &PlanReviewHarness{
		EvalHarness: base,
		anthropic:   NewAnthropicClient(apiKey),
	}, nil
}

// RunPlanReview executes the plan review evaluation pipeline.
// failures are RECALL items to seed; scenarios are the plans to evaluate.
func (h *PlanReviewHarness) RunPlanReview(ctx context.Context, failures []TestDocument, scenarios []PlanScenario) (*PlanReviewSummary, error) {
	// Phase 1: Boot MCP
	log.Println("Plan review: Boot MCP server...")
	if err := h.Boot(ctx); err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}

	// Phase 2: Seed RECALL with failure items
	log.Println("Plan review: Seeding RECALL with failures...")
	for _, f := range failures {
		_, err := h.client.RecallAdd(ctx, f)
		if err != nil {
			return nil, fmt.Errorf("seed failure %s: %w", f.ID, err)
		}
		log.Printf("Seeded: %s (%s)", f.ID, f.Title)
	}

	// Phase 3: For each scenario, search RECALL + judge
	log.Println("Plan review: Evaluating scenarios...")
	var results []PlanReviewResult

	for _, scenario := range scenarios {
		result, err := h.evaluateScenario(ctx, scenario)
		if err != nil {
			log.Printf("Error evaluating %s: %v (skipping)", scenario.ID, err)
			continue
		}
		results = append(results, *result)
		log.Printf("Scenario %s: dangerous=%v detected=%v false_alarm=%v",
			scenario.ID, scenario.IsDangerous, result.FailureDetected, result.FalseAlarm)
	}

	// Phase 4: Compute summary
	summary := &PlanReviewSummary{
		TotalScenarios: len(results),
		Results:        results,
	}

	var detected, falseAlarms int
	for _, r := range results {
		if r.IsDangerous {
			summary.DangerousCount++
			if r.FailureDetected {
				detected++
			}
		} else {
			summary.SafeCount++
			if r.FalseAlarm {
				falseAlarms++
			}
		}
	}

	if summary.DangerousCount > 0 {
		summary.DetectionRate = float64(detected) / float64(summary.DangerousCount)
	}
	if summary.SafeCount > 0 {
		summary.FalseAlarmRate = float64(falseAlarms) / float64(summary.SafeCount)
	}

	return summary, nil
}

func (h *PlanReviewHarness) evaluateScenario(ctx context.Context, scenario PlanScenario) (*PlanReviewResult, error) {
	// Search RECALL for context related to the plan
	searchResults, err := h.client.RecallSearch(ctx, scenario.Plan, 10)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Build context from search results
	var recallContext strings.Builder
	for i, r := range searchResults {
		recallContext.WriteString(fmt.Sprintf("[%d] %s (%s, score=%.3f)\n%s\n\n",
			i+1, r.Title, r.Type, r.Score, truncateForPrompt(r.Content, 500)))
	}

	// Ask the judge to evaluate the plan given the RECALL context
	systemPrompt := `You are reviewing an architectural plan for potential risks.
You have access to relevant organizational knowledge from past failures and decisions.
Evaluate whether the plan re-introduces any known failure patterns.

Return ONLY valid JSON:
{"risk_detected": true/false, "relevant_failure": "description or empty", "reasoning": "brief explanation"}`

	userPrompt := fmt.Sprintf(`Plan to review:
%s

Relevant organizational knowledge:
%s

Does this plan re-introduce any known failure patterns?`,
		scenario.Plan, recallContext.String())

	text, err := h.anthropic.RawJudge(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("judge: %w", err)
	}

	result := &PlanReviewResult{
		ScenarioID:   scenario.ID,
		IsDangerous:  scenario.IsDangerous,
		ReviewOutput: text,
	}

	// Check if the judge detected the risk
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, `"risk_detected": true`) || strings.Contains(lowerText, `"risk_detected":true`) {
		if scenario.IsDangerous {
			result.FailureDetected = true
		} else {
			result.FalseAlarm = true
		}
	}

	return result, nil
}

func truncateForPrompt(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// AuditTrailCoverage holds quantified audit trail completeness metrics (Experiment 2D).
type AuditTrailCoverage struct {
	TotalSearchCalls int     `json:"total_search_calls"`
	QueryCoverage    float64 `json:"query_coverage"`    // % of search calls with retrieval_query
	JudgmentCoverage float64 `json:"judgment_coverage"` // % of search calls with retrieval_judgment
	DecisionCoverage float64 `json:"decision_coverage"` // % of significant changes with decision entries
	QueryEntries     int     `json:"query_entries"`
	JudgmentEntries  int     `json:"judgment_entries"`
	DecisionEntries  int     `json:"decision_entries"`
}

// ComputeAuditTrailCoverage computes quantified audit trail completeness
// by comparing flight recorder entries against expected coverage.
func ComputeAuditTrailCoverage(queryEntries, judgmentEntries, decisionEntries, searchCalls, significantChanges int) *AuditTrailCoverage {
	cov := &AuditTrailCoverage{
		TotalSearchCalls: searchCalls,
		QueryEntries:     queryEntries,
		JudgmentEntries:  judgmentEntries,
		DecisionEntries:  decisionEntries,
	}

	if searchCalls > 0 {
		cov.QueryCoverage = float64(queryEntries) / float64(searchCalls)
		if cov.QueryCoverage > 1.0 {
			cov.QueryCoverage = 1.0
		}
		cov.JudgmentCoverage = float64(judgmentEntries) / float64(searchCalls)
		if cov.JudgmentCoverage > 1.0 {
			cov.JudgmentCoverage = 1.0
		}
	}

	if significantChanges > 0 {
		cov.DecisionCoverage = float64(decisionEntries) / float64(significantChanges)
		if cov.DecisionCoverage > 1.0 {
			cov.DecisionCoverage = 1.0
		}
	}

	return cov
}
