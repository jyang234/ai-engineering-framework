//go:build fts5 && evalintegration

package eval_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/anthropics/aef/codex/eval"
)

func skipIfNoKeys(t *testing.T) {
	t.Helper()
	if os.Getenv("VOYAGE_API_KEY") == "" || os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("VOYAGE_API_KEY and OPENAI_API_KEY required")
	}
}

// TestE2E indexes once, then runs all retrieval and system tests as subtests.
func TestE2E(t *testing.T) {
	skipIfNoKeys(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancel)

	h, err := eval.NewEvalHarness(ctx)
	if err != nil {
		t.Fatalf("NewEvalHarness: %v", err)
	}
	t.Cleanup(h.Close)

	// Boot
	if err := h.Boot(ctx); err != nil {
		t.Fatalf("Boot: %v", err)
	}

	t.Run("MCPProtocol", func(t *testing.T) {
		tools, err := h.VerifyProtocol(ctx)
		if err != nil {
			t.Fatalf("VerifyProtocol: %v", err)
		}
		if len(tools) != 5 {
			t.Errorf("expected 5 tools, got %d: %v", len(tools), tools)
		}
		t.Logf("MCP tools: %v", tools)
	})

	// Index once — all subsequent subtests reuse this data
	indexed, err := h.IndexCollection(ctx)
	if err != nil {
		t.Fatalf("IndexCollection: %v", err)
	}

	t.Run("IndexAndRetrieve", func(t *testing.T) {
		if indexed != 30 {
			t.Errorf("expected 30 indexed, got %d", indexed)
		}
		verified, err := h.VerifyIndexed(ctx)
		if err != nil {
			t.Fatalf("VerifyIndexed: %v", err)
		}
		if verified != 30 {
			t.Errorf("expected 30 verified, got %d", verified)
		}
	})

	t.Run("RetrievalQuality", func(t *testing.T) {
		summary, err := h.RunRetrieval(ctx)
		if err != nil {
			t.Fatalf("RunRetrieval: %v", err)
		}

		t.Logf("Recall@5:    %.3f", summary.RecallAt5)
		t.Logf("Recall@10:   %.3f", summary.RecallAt10)
		t.Logf("Precision@5: %.3f", summary.PrecisionAt5)
		t.Logf("nDCG@10:     %.3f", summary.NDCGAt10)
		t.Logf("MRR:         %.3f", summary.MRRScore)

		if summary.RecallAt10 < 0.3 {
			t.Errorf("Recall@10 too low: %.3f (min 0.3)", summary.RecallAt10)
		}
		if summary.MRRScore < 0.2 {
			t.Errorf("MRR too low: %.3f (min 0.2)", summary.MRRScore)
		}

		// Per-category breakdown
		for cat, ndcg := range summary.ByCategory {
			t.Logf("Category %s: nDCG@10=%.3f", cat, ndcg)
		}
	})

	t.Run("FeedbackLoop", func(t *testing.T) {
		if err := h.TestFeedback(ctx); err != nil {
			t.Fatalf("TestFeedback: %v", err)
		}
	})

	t.Run("FlightRecorder", func(t *testing.T) {
		if err := h.TestFlightRecorder(ctx); err != nil {
			t.Fatalf("TestFlightRecorder: %v", err)
		}
	})

	t.Run("AuditTrail", func(t *testing.T) {
		result, err := h.TestAuditTrail(ctx)
		if err != nil {
			t.Fatalf("TestAuditTrail: %v", err)
		}

		t.Logf("Query entries: %d (matched=%v, results_logged=%d)",
			result.QueryEntries, result.MatchedQuery, result.ResultsLogged)
		t.Logf("Judgment entries: %d (matched=%v, kept=%d, dropped=%d)",
			result.JudgmentEntries, result.MatchedJudgment, result.KeptCount, result.DroppedCount)

		if !result.MatchedQuery {
			t.Error("no retrieval_query entry found matching the test query")
		}
		if result.ResultsLogged == 0 {
			t.Error("retrieval_query entry has no result scores logged")
		}
		if !result.MatchedJudgment {
			t.Error("no retrieval_judgment entry found matching the test query")
		}
		if result.KeptCount == 0 {
			t.Error("retrieval_judgment entry has no kept results")
		}
	})

	t.Run("FullReport", func(t *testing.T) {
		// RunFull creates its own harness, so this is an independent integration check.
		// Skip if running with -short since it re-indexes.
		if testing.Short() {
			t.Skip("skipping full report in short mode")
		}

		h2, err := eval.NewEvalHarness(ctx)
		if err != nil {
			t.Fatalf("NewEvalHarness: %v", err)
		}
		t.Cleanup(h2.Close)

		report, err := h2.RunFull(ctx)
		if err != nil {
			t.Fatalf("RunFull: %v", err)
		}

		text := eval.FormatReport(report)
		t.Log("\n" + text)

		jsonStr, err := eval.FormatJSON(report)
		if err != nil {
			t.Fatalf("FormatJSON: %v", err)
		}
		t.Logf("JSON report length: %d bytes", len(jsonStr))
	})
}
