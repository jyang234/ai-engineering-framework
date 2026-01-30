package eval

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/aef/codex/internal/core"
)

// EvalHarness orchestrates end-to-end evaluation of the Codex system.
type EvalHarness struct {
	client     *MCPClient
	engine     *core.SearchEngine
	collection TestCollection
	// Maps test doc ID (e.g. "adr-001") to MCP-assigned ID (e.g. "D-abc12345")
	idMap      map[string]string
	// Maps MCP-assigned ID back to test doc ID
	reverseMap map[string]string
	// Maps title to test doc ID for matching search results
	titleToTestID map[string]string
}

// NewEvalHarness creates a harness with a real SearchEngine using a temp DB.
// Uses local Ollama nomic-embed-text model. Set LOCAL_EMBEDDING_URL to override
// the Ollama endpoint (default: http://localhost:11434/api/embed).
// Set LOCAL_EMBEDDING_MODEL to override the model (default: nomic-embed-text).
func NewEvalHarness(ctx context.Context) (*EvalHarness, error) {
	localURL := os.Getenv("LOCAL_EMBEDDING_URL")
	localModel := os.Getenv("LOCAL_EMBEDDING_MODEL")

	tmpDir, err := os.MkdirTemp("", "codex-eval-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	dbPath := filepath.Join(tmpDir, "eval.db")
	config := core.Config{
		LocalEmbeddingURL:   localURL,
		LocalEmbeddingModel: localModel,
		MetadataDBPath:      dbPath,
		ScoreThreshold:      0,
	}

	engine, err := core.NewSearchEngine(ctx, config)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("create engine: %w", err)
	}

	collection := NewPayFlowCollection()
	titleMap := make(map[string]string, len(collection.Documents))
	for _, doc := range collection.Documents {
		titleMap[doc.Title] = doc.ID
	}

	return &EvalHarness{
		engine:        engine,
		collection:    collection,
		idMap:         make(map[string]string),
		reverseMap:    make(map[string]string),
		titleToTestID: titleMap,
	}, nil
}

// Close releases all resources.
func (h *EvalHarness) Close() {
	if h.client != nil {
		h.client.Close()
	}
	if h.engine != nil {
		h.engine.Close()
	}
}

// Boot creates the MCP client and performs the initialize handshake.
func (h *EvalHarness) Boot(ctx context.Context) error {
	h.client = NewMCPClient(h.engine, "eval-session")
	return h.client.Initialize(ctx)
}

// VerifyProtocol lists tools and confirms all 5 are present.
func (h *EvalHarness) VerifyProtocol(ctx context.Context) ([]string, error) {
	tools, err := h.client.ListTools(ctx)
	if err != nil {
		return nil, err
	}

	expected := map[string]bool{
		"recall_search":       false,
		"recall_get":          false,
		"recall_add":          false,
		"recall_feedback":     false,
		"flight_recorder_log": false,
	}

	for _, name := range tools {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}

	for name, found := range expected {
		if !found {
			return tools, fmt.Errorf("missing tool: %s", name)
		}
	}

	return tools, nil
}

// IndexCollection indexes all documents via recall_add through MCP.
func (h *EvalHarness) IndexCollection(ctx context.Context) (int, error) {
	indexed := 0
	for i, doc := range h.collection.Documents {
		// Rate limit: Voyage free tier allows 3 RPM. Add delay between
		// requests that use Voyage (pattern, failure, code types).
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		mcpID, err := h.client.RecallAdd(ctx, doc)
		if err != nil {
			return indexed, fmt.Errorf("index %s: %w", doc.ID, err)
		}
		h.idMap[doc.ID] = mcpID
		h.reverseMap[mcpID] = doc.ID
		indexed++
		log.Printf("Indexed %s -> %s (%s)", doc.ID, mcpID, doc.Title)
	}
	return indexed, nil
}

// VerifyIndexed retrieves each document via recall_get and verifies content.
func (h *EvalHarness) VerifyIndexed(ctx context.Context) (int, error) {
	verified := 0
	for testID, mcpID := range h.idMap {
		item, err := h.client.RecallGet(ctx, mcpID)
		if err != nil {
			return verified, fmt.Errorf("get %s (mcp=%s): %w", testID, mcpID, err)
		}
		if item.Title == "" {
			return verified, fmt.Errorf("empty title for %s", testID)
		}
		verified++
	}
	return verified, nil
}

// RunRetrieval runs all queries and computes retrieval quality metrics.
func (h *EvalHarness) RunRetrieval(ctx context.Context) (*EvalSummary, error) {
	summary := &EvalSummary{
		Pipeline:     "hybrid",
		ByCategory:   make(map[string]float64),
		QueryResults: make([]QueryResult, 0, len(h.collection.Queries)),
	}

	catCounts := make(map[string]int)
	catNDCG := make(map[string]float64)

	var totalRecall5, totalRecall10, totalPrec5, totalNDCG10, totalMRR float64

	for _, q := range h.collection.Queries {
		results, err := h.client.RecallSearch(ctx, q.Query, 10)
		if err != nil {
			return nil, fmt.Errorf("search %s: %w", q.ID, err)
		}

		// Map MCP result IDs back to test document IDs via title matching
		retrievedIDs := h.mapResultsToTestIDs(results)

		r5 := RecallAtK(retrievedIDs, q.RelevantIDs, 5)
		r10 := RecallAtK(retrievedIDs, q.RelevantIDs, 10)
		p5 := PrecisionAtK(retrievedIDs, q.RelevantIDs, 5)
		ndcg := NDCG(retrievedIDs, q.RelevantIDs, 10)
		mrr := MRR(retrievedIDs, q.RelevantIDs)

		totalRecall5 += r5
		totalRecall10 += r10
		totalPrec5 += p5
		totalNDCG10 += ndcg
		totalMRR += mrr

		catCounts[q.Category]++
		catNDCG[q.Category] += ndcg

		summary.QueryResults = append(summary.QueryResults, QueryResult{
			QueryID:      q.ID,
			Query:        q.Query,
			Category:     q.Category,
			RetrievedIDs: retrievedIDs,
			RelevantIDs:  q.RelevantIDs,
			RecallAt5:    r5,
			PrecisionAt5: p5,
			NDCGAt10:     ndcg,
			MRRScore:     mrr,
		})

		log.Printf("Query %s [%s]: R@5=%.2f P@5=%.2f nDCG=%.2f MRR=%.2f retrieved=%v",
			q.ID, q.Category, r5, p5, ndcg, mrr, retrievedIDs)
	}

	n := float64(len(h.collection.Queries))
	summary.RecallAt5 = totalRecall5 / n
	summary.RecallAt10 = totalRecall10 / n
	summary.PrecisionAt5 = totalPrec5 / n
	summary.NDCGAt10 = totalNDCG10 / n
	summary.MRRScore = totalMRR / n

	for cat, count := range catCounts {
		summary.ByCategory[cat] = catNDCG[cat] / float64(count)
	}

	return summary, nil
}

// mapResultsToTestIDs converts MCP search results to test document IDs by title matching.
func (h *EvalHarness) mapResultsToTestIDs(results []SearchResultFromMCP) []string {
	ids := make([]string, 0, len(results))
	for _, r := range results {
		// Try reverse map first (direct MCP ID match)
		if testID, ok := h.reverseMap[r.ID]; ok {
			ids = append(ids, testID)
			continue
		}
		// Fall back to title matching
		if testID, ok := h.titleToTestID[r.Title]; ok {
			ids = append(ids, testID)
			continue
		}
		// Unknown result — include raw ID (won't match ground truth IDs)
		ids = append(ids, r.ID)
	}
	return ids
}

// TestFeedback tests the feedback loop via MCP.
func (h *EvalHarness) TestFeedback(ctx context.Context) error {
	// Use the first indexed doc
	for _, mcpID := range h.idMap {
		return h.client.RecallFeedback(ctx, mcpID, true)
	}
	return fmt.Errorf("no indexed documents")
}

// TestFlightRecorder tests the flight recorder via MCP.
func (h *EvalHarness) TestFlightRecorder(ctx context.Context) error {
	return h.client.FlightRecorderLog(ctx, "milestone", "E2E evaluation completed")
}

// TestAuditTrail verifies that recall_search auto-logs retrieval_query entries
// to the flight recorder. Performs a search, then reads back flight recorder
// entries and checks for the corresponding retrieval_query entry.
func (h *EvalHarness) TestAuditTrail(ctx context.Context) (*AuditTrailResult, error) {
	testQuery := "idempotency key implementation for payment creation"

	// Perform a search (which should auto-log a retrieval_query entry)
	results, err := h.client.RecallSearch(ctx, testQuery, 10)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Also log a retrieval_judgment entry (simulating what the agent would do)
	kept := make([]map[string]interface{}, 0)
	dropped := make([]map[string]interface{}, 0)
	for i, r := range results {
		entry := map[string]interface{}{
			"id":    r.ID,
			"title": r.Title,
		}
		if i < 3 {
			entry["reason"] = "relevant to idempotency"
			kept = append(kept, entry)
		} else {
			entry["reason"] = "not directly about idempotency"
			dropped = append(dropped, entry)
		}
	}

	err = h.client.FlightRecorderLogWithMetadata(ctx, "retrieval_judgment",
		fmt.Sprintf("%d/%d results relevant for %q", len(kept), len(results), testQuery),
		map[string]interface{}{
			"query":   testQuery,
			"kept":    kept,
			"dropped": dropped,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("log judgment: %w", err)
	}

	// Read back flight recorder entries and verify
	entries, err := h.engine.GetFlightRecorderEntries("eval-session")
	if err != nil {
		return nil, fmt.Errorf("get entries: %w", err)
	}

	result := &AuditTrailResult{}
	for _, e := range entries {
		switch e.Type {
		case "retrieval_query":
			result.QueryEntries++
			if meta, ok := e.Metadata["query"].(string); ok && meta == testQuery {
				result.MatchedQuery = true
			}
			if scores, ok := e.Metadata["results"].([]interface{}); ok {
				result.ResultsLogged = len(scores)
			}
		case "retrieval_judgment":
			result.JudgmentEntries++
			if meta, ok := e.Metadata["query"].(string); ok && meta == testQuery {
				result.MatchedJudgment = true
			}
			if kept, ok := e.Metadata["kept"].([]interface{}); ok {
				result.KeptCount = len(kept)
			}
			if dropped, ok := e.Metadata["dropped"].([]interface{}); ok {
				result.DroppedCount = len(dropped)
			}
		}
	}

	return result, nil
}

// AuditTrailResult holds the results of audit trail verification.
type AuditTrailResult struct {
	QueryEntries   int  `json:"query_entries"`    // number of retrieval_query entries found
	JudgmentEntries int `json:"judgment_entries"`  // number of retrieval_judgment entries found
	MatchedQuery   bool `json:"matched_query"`     // found entry matching the test query
	MatchedJudgment bool `json:"matched_judgment"` // found judgment matching the test query
	ResultsLogged  int  `json:"results_logged"`    // number of result scores in the query entry
	KeptCount      int  `json:"kept_count"`        // number of kept results in judgment
	DroppedCount   int  `json:"dropped_count"`     // number of dropped results in judgment
}

// RunFull executes the complete evaluation pipeline.
func (h *EvalHarness) RunFull(ctx context.Context) (*FullEvalReport, error) {
	report := &FullEvalReport{}

	// Phase 1: Boot
	log.Println("Phase 1: Boot MCP server...")
	if err := h.Boot(ctx); err != nil {
		return report, fmt.Errorf("boot: %w", err)
	}

	// Phase 2: Verify protocol
	log.Println("Phase 2: Verify MCP protocol...")
	tools, err := h.VerifyProtocol(ctx)
	if err != nil {
		return report, fmt.Errorf("verify protocol: %w", err)
	}
	report.MCPProtocol = true
	report.ToolCount = len(tools)

	// Phase 3: Index collection
	log.Println("Phase 3: Index collection...")
	indexed, err := h.IndexCollection(ctx)
	if err != nil {
		return report, fmt.Errorf("index: %w", err)
	}
	report.DocsIndexed = indexed

	// Phase 4: Verify indexed
	log.Println("Phase 4: Verify indexed documents...")
	verified, err := h.VerifyIndexed(ctx)
	if err != nil {
		return report, fmt.Errorf("verify indexed: %w", err)
	}
	report.DocsVerified = verified

	// Phase 5: Run retrieval
	log.Println("Phase 5: Run retrieval evaluation...")
	summary, err := h.RunRetrieval(ctx)
	if err != nil {
		return report, fmt.Errorf("retrieval: %w", err)
	}
	report.Summaries = []EvalSummary{*summary}

	// Phase 6: Test feedback
	log.Println("Phase 6: Test feedback...")
	if err := h.TestFeedback(ctx); err != nil {
		log.Printf("Warning: feedback test failed: %v", err)
	} else {
		report.FeedbackOK = true
	}

	// Phase 7: Test flight recorder
	log.Println("Phase 7: Test flight recorder...")
	if err := h.TestFlightRecorder(ctx); err != nil {
		log.Printf("Warning: flight recorder test failed: %v", err)
	} else {
		report.FlightRecordOK = true
	}

	// Phase 8: Test audit trail
	log.Println("Phase 8: Test audit trail...")
	auditResult, err := h.TestAuditTrail(ctx)
	if err != nil {
		log.Printf("Warning: audit trail test failed: %v", err)
	} else {
		report.AuditTrail = auditResult
	}

	return report, nil
}
