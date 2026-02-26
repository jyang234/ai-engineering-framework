//go:build fts5

package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// =============================================================================
// Level 3-4 Report Generation
//
// Produces condition comparison tables, claim validation summaries, and
// per-experiment reports as specified in the evaluation framework doc
// (Report Phase, Week 8).
// =============================================================================

// ExperimentReport holds the complete report for a single experiment.
type ExperimentReport struct {
	Experiment  string                `json:"experiment"`
	Description string                `json:"description"`
	Conditions  []ConditionSummary    `json:"conditions"`
	Comparisons []ConditionComparison `json:"comparisons"`
	Claims      []ClaimResult         `json:"claims"`
}

// ConditionSummary aggregates metrics for one condition within an experiment.
type ConditionSummary struct {
	Condition          string  `json:"condition"`
	RunCount           int     `json:"run_count"`
	MeanTestPassRate   float64 `json:"mean_test_pass_rate"`
	MeanJudgeCombined  float64 `json:"mean_judge_combined"`
	MeanPitfallRate    float64 `json:"mean_pitfall_avoidance_rate"`
	MeanTurns          float64 `json:"mean_turns"`
	MeanTokens         float64 `json:"mean_tokens"`
	CI95TestPassRate   BootstrapCI `json:"ci95_test_pass_rate"`
	CI95JudgeCombined  BootstrapCI `json:"ci95_judge_combined"`
}

// ClaimResult evaluates a specific claim against measured data.
type ClaimResult struct {
	ClaimID     string  `json:"claim_id"`
	Description string  `json:"description"`
	Metric      string  `json:"metric"`
	Threshold   float64 `json:"threshold"`
	Measured    float64 `json:"measured"`
	Validated   bool    `json:"validated"`
	Evidence    string  `json:"evidence"`
}

// Claim defines an expected outcome from the eval framework success criteria.
type Claim struct {
	ID          string
	Description string
	Experiment  string
	Metric      string             // which metric to compare
	Condition1  string             // condition expected to be better
	Condition2  string             // baseline condition
	Threshold   float64            // minimum improvement required
	Direction   ClaimDirection     // higher_is_better or lower_is_better
}

// ClaimDirection indicates whether a higher or lower metric value is "better".
type ClaimDirection string

const (
	HigherIsBetter ClaimDirection = "higher"
	LowerIsBetter  ClaimDirection = "lower"
)

// EvalFrameworkClaims returns the claims from the spec's success criteria table.
func EvalFrameworkClaims() []Claim {
	return []Claim{
		{
			ID:          "C1",
			Description: "AEF reduces defects: AEF-full outperforms baseline by >=10 points on combined quality",
			Experiment:  "3A",
			Metric:      "judge_combined",
			Condition1:  "aef-full",
			Condition2:  "baseline",
			Threshold:   10.0,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C2",
			Description: "RECALL adds value beyond hooks: AEF-full outperforms AEF-minimal on pitfall avoidance",
			Experiment:  "3A",
			Metric:      "pitfall_avoidance_rate",
			Condition1:  "aef-full",
			Condition2:  "aef-minimal",
			Threshold:   0.0,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C3",
			Description: "RECALL prevents repeat failures: >=25 point prevention delta",
			Experiment:  "3B",
			Metric:      "pitfall_avoidance_rate",
			Condition1:  "aef-full",
			Condition2:  "baseline",
			Threshold:   25.0,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C4",
			Description: "Knowledge accumulates usefully: positive quality slope with RECALL",
			Experiment:  "3C",
			Metric:      "judge_combined",
			Condition1:  "aef-full",
			Condition2:  "baseline",
			Threshold:   0.0,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C5",
			Description: "Hooks improve adherence: >=20 point improvement over prompts-only",
			Experiment:  "3D",
			Metric:      "lint_clean_rate",
			Condition1:  "aef-minimal",
			Condition2:  "baseline",
			Threshold:   20.0,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C6",
			Description: "Retrieval-judge improves precision: judge precision > raw precision by >=0.10",
			Experiment:  "2A",
			Metric:      "judge_precision_improvement",
			Condition1:  "aef-full",
			Condition2:  "baseline",
			Threshold:   0.10,
			Direction:   HigherIsBetter,
		},
		{
			ID:          "C7",
			Description: "Plan-review catches known failures: >=60% detection rate",
			Experiment:  "2B",
			Metric:      "detection_rate",
			Condition1:  "aef-full",
			Condition2:  "",
			Threshold:   60.0,
			Direction:   HigherIsBetter,
		},
	}
}

// L3ReportGenerator generates Level 3-4 reports from the results database.
type L3ReportGenerator struct {
	db *ResultsDB
}

// NewL3ReportGenerator creates a report generator.
func NewL3ReportGenerator(db *ResultsDB) *L3ReportGenerator {
	return &L3ReportGenerator{db: db}
}

// GenerateExperimentReport produces a complete report for one experiment.
func (g *L3ReportGenerator) GenerateExperimentReport(experiment string) (*ExperimentReport, error) {
	conditions := AllConditionNames()
	report := &ExperimentReport{
		Experiment:  experiment,
		Description: experimentDescription(experiment),
	}

	conditionData := make(map[string][]float64) // condition → combined scores
	conditionPitfalls := make(map[string][]float64)
	conditionTurns := make(map[string][]float64)
	conditionTokens := make(map[string][]float64)
	conditionTestRates := make(map[string][]float64)

	for _, cond := range conditions {
		name := string(cond)
		runs, err := g.db.QueryByExperiment(experiment, name)
		if err != nil {
			continue
		}
		if len(runs) == 0 {
			continue
		}

		var testRates, combined, pitfallRates, turns, tokens []float64
		for _, r := range runs {
			if r.TestPassRate != nil {
				testRates = append(testRates, *r.TestPassRate)
			}
			if r.JudgeCombined != nil {
				combined = append(combined, *r.JudgeCombined)
			}
			if r.PitfallsTotal != nil && r.PitfallsAvoided != nil && *r.PitfallsTotal > 0 {
				pitfallRates = append(pitfallRates, float64(*r.PitfallsAvoided)/float64(*r.PitfallsTotal)*100)
			}
			if r.TurnsToComplete != nil {
				turns = append(turns, float64(*r.TurnsToComplete))
			}
			if r.TokensConsumed != nil {
				tokens = append(tokens, float64(*r.TokensConsumed))
			}
		}

		conditionData[name] = combined
		conditionPitfalls[name] = pitfallRates
		conditionTurns[name] = turns
		conditionTokens[name] = tokens
		conditionTestRates[name] = testRates

		summary := ConditionSummary{
			Condition:         name,
			RunCount:          len(runs),
			MeanTestPassRate:  mean(testRates),
			MeanJudgeCombined: mean(combined),
			MeanPitfallRate:   mean(pitfallRates),
			MeanTurns:         mean(turns),
			MeanTokens:        mean(tokens),
			CI95TestPassRate:  Bootstrap(testRates, 0.95, 10000),
			CI95JudgeCombined: Bootstrap(combined, 0.95, 10000),
		}
		report.Conditions = append(report.Conditions, summary)
	}

	// Generate pairwise comparisons
	condNames := make([]string, 0, len(conditionData))
	for name := range conditionData {
		condNames = append(condNames, name)
	}
	for i := 0; i < len(condNames); i++ {
		for j := i + 1; j < len(condNames); j++ {
			if len(conditionData[condNames[i]]) > 0 && len(conditionData[condNames[j]]) > 0 {
				comp := CompareConditions(
					condNames[i], conditionData[condNames[i]],
					condNames[j], conditionData[condNames[j]],
					"judge_combined",
				)
				report.Comparisons = append(report.Comparisons, comp)
			}
		}
	}

	// Validate claims
	for _, claim := range EvalFrameworkClaims() {
		if claim.Experiment != experiment {
			continue
		}
		result := g.validateClaim(claim, conditionData, conditionPitfalls, conditionTestRates)
		report.Claims = append(report.Claims, result)
	}

	return report, nil
}

// validateClaim checks a single claim against measured data.
func (g *L3ReportGenerator) validateClaim(claim Claim, combined, pitfalls, testRates map[string][]float64) ClaimResult {
	result := ClaimResult{
		ClaimID:     claim.ID,
		Description: claim.Description,
		Metric:      claim.Metric,
		Threshold:   claim.Threshold,
	}

	// Select the right metric data
	var data1, data2 []float64
	switch claim.Metric {
	case "judge_combined":
		data1 = combined[claim.Condition1]
		data2 = combined[claim.Condition2]
	case "pitfall_avoidance_rate":
		data1 = pitfalls[claim.Condition1]
		data2 = pitfalls[claim.Condition2]
	case "lint_clean_rate":
		data1 = testRates[claim.Condition1]
		data2 = testRates[claim.Condition2]
	default:
		result.Evidence = "metric not available in results"
		return result
	}

	if len(data1) == 0 {
		result.Evidence = fmt.Sprintf("no data for condition %q", claim.Condition1)
		return result
	}

	mean1 := mean(data1)
	if claim.Condition2 == "" {
		// Absolute threshold (e.g., C7: detection rate >= 60%)
		result.Measured = mean1
		result.Validated = mean1 >= claim.Threshold
		result.Evidence = fmt.Sprintf("measured=%.1f, threshold=%.1f, n=%d",
			mean1, claim.Threshold, len(data1))
		return result
	}

	if len(data2) == 0 {
		result.Evidence = fmt.Sprintf("no data for condition %q", claim.Condition2)
		return result
	}

	mean2 := mean(data2)
	diff := mean1 - mean2

	result.Measured = diff
	if claim.Direction == HigherIsBetter {
		result.Validated = diff >= claim.Threshold
	} else {
		result.Validated = diff <= -claim.Threshold
	}

	mw := MannWhitneyU(data1, data2)
	result.Evidence = fmt.Sprintf("diff=%.1f (%.1f vs %.1f), p=%.4f, sig=%v, n1=%d, n2=%d",
		diff, mean1, mean2, mw.P, mw.Significant, len(data1), len(data2))

	return result
}

// FormatExperimentReport formats an experiment report as a text table.
func FormatExperimentReport(r *ExperimentReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Experiment %s: %s\n", r.Experiment, r.Description)
	b.WriteString(strings.Repeat("=", 70) + "\n\n")

	// Condition summaries table
	if len(r.Conditions) > 0 {
		b.WriteString("Condition Summaries:\n")
		fmt.Fprintf(&b, "%-15s %6s %10s %10s %10s %8s\n",
			"Condition", "Runs", "TestPass%", "Combined", "Pitfall%", "Turns")
		b.WriteString(strings.Repeat("-", 65) + "\n")

		for _, c := range r.Conditions {
			fmt.Fprintf(&b, "%-15s %6d %9.1f%% %10.2f %9.1f%% %8.1f\n",
				c.Condition, c.RunCount, c.MeanTestPassRate*100,
				c.MeanJudgeCombined, c.MeanPitfallRate, c.MeanTurns)
		}
		b.WriteString("\n")
	}

	// Pairwise comparisons
	if len(r.Comparisons) > 0 {
		b.WriteString("Pairwise Comparisons (Mann-Whitney U):\n")
		fmt.Fprintf(&b, "%-15s %-15s %8s %8s %8s %6s\n",
			"Condition A", "Condition B", "Diff", "Z", "P", "Sig?")
		b.WriteString(strings.Repeat("-", 68) + "\n")

		for _, c := range r.Comparisons {
			sig := "No"
			if c.MannWhitney.Significant {
				sig = "Yes"
			}
			fmt.Fprintf(&b, "%-15s %-15s %+8.2f %8.2f %8.4f %6s\n",
				c.Condition1, c.Condition2, c.MeanDiff,
				c.MannWhitney.Z, c.MannWhitney.P, sig)
		}
		b.WriteString("\n")
	}

	// Claim validation
	if len(r.Claims) > 0 {
		b.WriteString("Claim Validation:\n")
		fmt.Fprintf(&b, "%-6s %-50s %10s %6s\n",
			"ID", "Description", "Measured", "Valid?")
		b.WriteString(strings.Repeat("-", 74) + "\n")

		for _, c := range r.Claims {
			valid := "NO"
			if c.Validated {
				valid = "YES"
			}
			desc := c.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(&b, "%-6s %-50s %10.2f %6s\n",
				c.ClaimID, desc, c.Measured, valid)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatExperimentReportJSON returns the report as indented JSON.
func FormatExperimentReportJSON(r *ExperimentReport) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatScorecard formats the measured scorecard that replaces self-assessed scores.
func FormatScorecard(reports []*ExperimentReport) string {
	var b strings.Builder

	b.WriteString("Measured Scorecard\n")
	b.WriteString(strings.Repeat("=", 80) + "\n\n")

	fmt.Fprintf(&b, "| %-35s | %12s | %-20s | %4s |\n",
		"Dimension", "Score", "Evidence", "n")
	b.WriteString("|" + strings.Repeat("-", 37) + "|" +
		strings.Repeat("-", 14) + "|" +
		strings.Repeat("-", 22) + "|" +
		strings.Repeat("-", 6) + "|\n")

	// Extract metrics from reports
	for _, r := range reports {
		for _, c := range r.Claims {
			score := "N/A"
			if c.Validated {
				score = fmt.Sprintf("%.1f", c.Measured)
			}
			desc := c.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			fmt.Fprintf(&b, "| %-35s | %12s | %-20s | %4s |\n",
				desc, score, r.Experiment, "")
		}
	}
	b.WriteString("\n")

	return b.String()
}

// experimentDescription returns a human-readable description for an experiment ID.
func experimentDescription(experiment string) string {
	descriptions := map[string]string{
		"1A": "RECALL Retrieval Quality (Existing Harness)",
		"1B": "Judge Filtering Quality (Existing JudgeHarness)",
		"1C": "Flight Recorder Audit Trail Coverage",
		"2A": "Retrieval-Judge Filtering Quality (Enhanced)",
		"2B": "RECALL + Plan Review Detection Rate",
		"2D": "Audit Trail Quantification",
		"3A": "Defect Rate Comparison (30 tasks x 3 conditions x 3 attempts)",
		"3B": "Repeat Failure Prevention (10 pairs x 2 conditions x 3 attempts)",
		"3C": "Session Knowledge Accumulation (5 sessions x 2 conditions x 3 trials)",
		"3D": "Hook Adherence Rate (20 sessions x 2 conditions)",
		"4A": "AEF-bench (50 tasks x 2 conditions)",
		"4B": "Comparative System Analysis (20 tasks x 4 systems)",
	}
	if desc, ok := descriptions[experiment]; ok {
		return desc
	}
	return "Unknown experiment"
}
