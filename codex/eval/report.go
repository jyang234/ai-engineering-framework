package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EvalSummary holds retrieval quality metrics for one pipeline configuration.
type EvalSummary struct {
	Pipeline    string             `json:"pipeline"` // hybrid, vector-only, fts5-only
	RecallAt5   float64            `json:"recall_at_5"`
	RecallAt10  float64            `json:"recall_at_10"`
	PrecisionAt5 float64           `json:"precision_at_5"`
	NDCGAt10    float64            `json:"ndcg_at_10"`
	MRRScore    float64            `json:"mrr"`
	ByCategory  map[string]float64 `json:"by_category"` // category -> NDCG@10
	QueryResults []QueryResult     `json:"query_results"`
}

// QueryResult holds per-query evaluation details.
type QueryResult struct {
	QueryID     string   `json:"query_id"`
	Query       string   `json:"query"`
	Category    string   `json:"category"`
	RetrievedIDs []string `json:"retrieved_ids"`
	RelevantIDs  []string `json:"relevant_ids"`
	RecallAt5   float64  `json:"recall_at_5"`
	PrecisionAt5 float64 `json:"precision_at_5"`
	NDCGAt10    float64  `json:"ndcg_at_10"`
	MRRScore    float64  `json:"mrr"`
}

// FullEvalReport contains the complete evaluation results.
type FullEvalReport struct {
	MCPProtocol    bool              `json:"mcp_protocol"`
	ToolCount      int               `json:"tool_count"`
	DocsIndexed    int               `json:"docs_indexed"`
	DocsVerified   int               `json:"docs_verified"`
	FeedbackOK     bool              `json:"feedback_ok"`
	FlightRecordOK bool              `json:"flight_record_ok"`
	AuditTrail     *AuditTrailResult `json:"audit_trail,omitempty"`
	Summaries      []EvalSummary     `json:"summaries"`
}

// FormatReport generates a text report from the full evaluation.
func FormatReport(report *FullEvalReport) string {
	var b strings.Builder

	b.WriteString("Codex End-to-End System Evaluation\n")
	b.WriteString("=====================================\n\n")

	check := func(ok bool) string {
		if ok { return "✓" }
		return "✗"
	}

	fmt.Fprintf(&b, "MCP Protocol:      %s Initialize, ListTools (%d/5), CallTool\n", check(report.MCPProtocol), report.ToolCount)
	fmt.Fprintf(&b, "Index Pipeline:    %s %d/%d documents indexed via recall_add\n", check(report.DocsIndexed > 0), report.DocsIndexed, report.DocsIndexed)
	fmt.Fprintf(&b, "Storage Roundtrip: %s %d/%d documents verified via recall_get\n", check(report.DocsVerified > 0), report.DocsVerified, report.DocsIndexed)
	fmt.Fprintf(&b, "Feedback System:   %s recall_feedback recorded\n", check(report.FeedbackOK))
	fmt.Fprintf(&b, "Flight Recorder:   %s flight_recorder_log recorded\n", check(report.FlightRecordOK))
	if report.AuditTrail != nil {
		at := report.AuditTrail
		fmt.Fprintf(&b, "Audit Trail:       %s retrieval_query=%d retrieval_judgment=%d\n",
			check(at.MatchedQuery && at.MatchedJudgment), at.QueryEntries, at.JudgmentEntries)
	}

	if len(report.Summaries) > 0 {
		b.WriteString("\nRetrieval Quality (20 queries, through MCP recall_search):\n")
		fmt.Fprintf(&b, "%-16s", "Metric")
		for _, s := range report.Summaries {
			fmt.Fprintf(&b, "| %-12s", s.Pipeline)
		}
		b.WriteString("\n")

		metrics := []struct {
			name string
			get  func(s EvalSummary) float64
		}{
			{"Recall@5", func(s EvalSummary) float64 { return s.RecallAt5 }},
			{"Recall@10", func(s EvalSummary) float64 { return s.RecallAt10 }},
			{"Precision@5", func(s EvalSummary) float64 { return s.PrecisionAt5 }},
			{"nDCG@10", func(s EvalSummary) float64 { return s.NDCGAt10 }},
			{"MRR", func(s EvalSummary) float64 { return s.MRRScore }},
		}

		for _, m := range metrics {
			fmt.Fprintf(&b, "%-16s", m.name)
			for _, s := range report.Summaries {
				fmt.Fprintf(&b, "| %-12.2f", m.get(s))
			}
			b.WriteString("\n")
		}

		// Per-category breakdown
		categories := []string{"semantic", "keyword", "hybrid-advantage"}
		b.WriteString("\nPer-Category (nDCG@10):\n")
		for _, cat := range categories {
			fmt.Fprintf(&b, "  %-20s", cat+":")
			for _, s := range report.Summaries {
				fmt.Fprintf(&b, " %s=%.2f", s.Pipeline, s.ByCategory[cat])
			}
			b.WriteString("\n")
		}
	}

	// Verdict
	b.WriteString("\n")
	if len(report.Summaries) > 0 && report.MCPProtocol && report.DocsIndexed > 0 {
		b.WriteString("System Verdict: PASS — end-to-end system operational\n")
	} else {
		b.WriteString("System Verdict: FAIL — see details above\n")
	}

	return b.String()
}

// FormatJSON returns the report as indented JSON.
func FormatJSON(report *FullEvalReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
