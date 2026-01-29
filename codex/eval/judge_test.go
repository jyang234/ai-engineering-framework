//go:build fts5 && evalintegration

package eval_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/anthropics/aef/codex/eval"
)

func skipIfNoJudgeKeys(t *testing.T) {
	t.Helper()
	if os.Getenv("VOYAGE_API_KEY") == "" || os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("VOYAGE_API_KEY and OPENAI_API_KEY required")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY required for judge eval")
	}
}

// TestJudge indexes once, runs the judge evaluation, then verifies both
// judge quality metrics and audit trail in subtests.
func TestJudge(t *testing.T) {
	skipIfNoJudgeKeys(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	t.Cleanup(cancel)

	skillPath := "../edi/internal/assets/skills/retrieval-judge/SKILL.md"
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		skillPath = "../../edi/internal/assets/skills/retrieval-judge/SKILL.md"
	}

	jh, err := eval.NewJudgeHarness(ctx, skillPath)
	if err != nil {
		t.Fatalf("NewJudgeHarness: %v", err)
	}
	t.Cleanup(jh.Close)

	// Run judge eval once (boots, indexes, searches+judges all 20 queries)
	summary, err := jh.RunJudgeEval(ctx)
	if err != nil {
		t.Fatalf("RunJudgeEval: %v", err)
	}

	t.Run("Quality", func(t *testing.T) {
		t.Logf("Judge Precision:  %.3f", summary.AvgJudgePrecision)
		t.Logf("Judge Recall:     %.3f", summary.AvgJudgeRecall)
		t.Logf("Judge F1:         %.3f", summary.AvgJudgeF1)
		t.Logf("Filtering Rate:   %.3f", summary.AvgFilteringRate)
		t.Logf("Avg Improvement:  %+.3f", summary.AvgImprovement)

		for cat, cm := range summary.ByCategory {
			t.Logf("Category %s: prec=%.3f rec=%.3f f1=%.3f filter=%.3f improvement=%+.3f (n=%d)",
				cat, cm.AvgJudgePrecision, cm.AvgJudgeRecall, cm.AvgJudgeF1,
				cm.AvgFilteringRate, cm.AvgImprovement, cm.Count)
		}

		for _, m := range summary.PerQuery {
			t.Logf("  %s [%s]: judge_prec=%.2f judge_rec=%.2f f1=%.2f filter=%.2f raw_p5=%.2f improvement=%+.2f",
				m.QueryID, m.Category, m.JudgePrecision, m.JudgeRecall, m.JudgeF1,
				m.FilteringRate, m.RawPrecisionAt5, m.Improvement)
		}

		if summary.AvgJudgePrecision < 0.6 {
			t.Errorf("Judge precision too low: %.3f (min 0.6)", summary.AvgJudgePrecision)
		}
		if summary.AvgImprovement < 0.15 {
			t.Errorf("Improvement over raw too low: %+.3f (min +0.15)", summary.AvgImprovement)
		}
	})

	t.Run("AuditTrail", func(t *testing.T) {
		result, err := jh.VerifyAuditTrail()
		if err != nil {
			t.Fatalf("VerifyAuditTrail: %v", err)
		}

		t.Logf("Audit trail: query_entries=%d matched_query=%v results_logged=%d",
			result.QueryEntries, result.MatchedQuery, result.ResultsLogged)

		// Every search should produce a retrieval_query entry.
		// The judge runs 20 queries, so we expect at least 20 entries.
		if result.QueryEntries < 20 {
			t.Errorf("expected at least 20 retrieval_query entries, got %d", result.QueryEntries)
		}
		if !result.MatchedQuery {
			t.Error("no retrieval_query entry found with result scores")
		}
		if result.ResultsLogged == 0 {
			t.Error("retrieval_query entries have no result scores logged")
		}
	})
}
