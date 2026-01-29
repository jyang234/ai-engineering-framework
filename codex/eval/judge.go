package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	anthropicBaseURL    = "https://api.anthropic.com/v1/messages"
	anthropicModel      = "claude-sonnet-4-20250514"
	anthropicVersion    = "2023-06-01"
	anthropicMaxRetries = 5
	anthropicInitDelay  = 2 * time.Second
	maxSnippetLen       = 300
)

// AnthropicClient calls the Anthropic Messages API for LLM-as-judge evaluation.
type AnthropicClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// JudgmentResult is the parsed JSON output from the judge LLM.
type JudgmentResult struct {
	RelevantResults []int  `json:"relevant_results"`
	Reasoning       string `json:"reasoning"`
}

// NewAnthropicClient creates a client for the Anthropic Messages API.
func NewAnthropicClient(apiKey string) *AnthropicClient {
	return &AnthropicClient{
		apiKey:  apiKey,
		baseURL: anthropicBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Judge sends a system+user prompt to the Anthropic API and parses the JSON response.
func (c *AnthropicClient) Judge(ctx context.Context, systemPrompt, userPrompt string) (*JudgmentResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	req := anthropicRequest{
		Model:     anthropicModel,
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < anthropicMaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * anthropicInitDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicVersion)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				continue
			}
			return nil, lastErr
		}

		var apiResp anthropicResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if len(apiResp.Content) == 0 {
			return nil, fmt.Errorf("empty response content")
		}

		text := apiResp.Content[0].Text
		return parseJudgment(text)
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", anthropicMaxRetries, lastErr)
}

// parseJudgment extracts JSON from the judge response, handling markdown code fences.
func parseJudgment(text string) (*JudgmentResult, error) {
	// Strip markdown code fences if present
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```") {
		// Remove opening fence (with optional language tag)
		if idx := strings.Index(cleaned, "\n"); idx >= 0 {
			cleaned = cleaned[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(cleaned, "```"); idx >= 0 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var result JudgmentResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse judge JSON: %w (raw: %s)", err, text)
	}
	return &result, nil
}

// JudgeHarness composes the existing EvalHarness with LLM-as-judge evaluation.
type JudgeHarness struct {
	*EvalHarness
	anthropic   *AnthropicClient
	skillPrompt string
}

// NewJudgeHarness creates a judge harness that wraps an existing eval harness.
func NewJudgeHarness(ctx context.Context, skillPath string) (*JudgeHarness, error) {
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY must be set")
	}

	base, err := NewEvalHarness(ctx)
	if err != nil {
		return nil, err
	}

	skillContent, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read retrieval-judge skill: %w", err)
	}

	return &JudgeHarness{
		EvalHarness: base,
		anthropic:   NewAnthropicClient(anthropicKey),
		skillPrompt: string(skillContent),
	}, nil
}

// RunJudgeEval executes the full judge evaluation pipeline.
func (jh *JudgeHarness) RunJudgeEval(ctx context.Context) (*JudgeSummary, error) {
	// Phase 1: Boot + Index (reuse harness)
	log.Println("Judge eval: Boot MCP server...")
	if err := jh.Boot(ctx); err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}

	log.Println("Judge eval: Index collection...")
	if _, err := jh.IndexCollection(ctx); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}

	// Phase 2: For each query, search + judge
	log.Println("Judge eval: Running judge evaluation...")
	var perQuery []JudgeMetrics

	for _, q := range jh.collection.Queries {
		results, err := jh.client.RecallSearch(ctx, q.Query, 10)
		if err != nil {
			return nil, fmt.Errorf("search %s: %w", q.ID, err)
		}

		retrievedIDs := jh.mapResultsToTestIDs(results)

		// Build user prompt with numbered results
		userPrompt := buildJudgePrompt(q.Query, results)

		// Call judge
		judgment, err := jh.anthropic.Judge(ctx, jh.skillPrompt, userPrompt)
		if err != nil {
			log.Printf("Judge error for %s: %v (skipping)", q.ID, err)
			continue
		}

		// Map 1-indexed result indices to doc IDs
		judgedIDs := mapIndicesToIDs(judgment.RelevantResults, retrievedIDs)

		// Compute metrics
		rawP5 := PrecisionAtK(retrievedIDs, q.RelevantIDs, 5)
		prec, rec, f1, filterRate := computeJudgeMetrics(judgedIDs, retrievedIDs, q.RelevantIDs)

		m := JudgeMetrics{
			QueryID:         q.ID,
			Query:           q.Query,
			Category:        q.Category,
			JudgePrecision:  prec,
			JudgeRecall:     rec,
			JudgeF1:         f1,
			FilteringRate:   filterRate,
			RawPrecisionAt5: rawP5,
			Improvement:     prec - rawP5,
		}
		perQuery = append(perQuery, m)

		log.Printf("Query %s [%s]: judge_prec=%.2f judge_rec=%.2f f1=%.2f filter=%.2f raw_p5=%.2f improvement=%+.2f judged=%v",
			q.ID, q.Category, prec, rec, f1, filterRate, rawP5, m.Improvement, judgedIDs)
	}

	return aggregateJudgeMetrics(perQuery), nil
}

// VerifyAuditTrail checks that recall_search calls during judge evaluation
// produced retrieval_query entries in the flight recorder.
func (jh *JudgeHarness) VerifyAuditTrail() (*AuditTrailResult, error) {
	entries, err := jh.engine.GetFlightRecorderEntries("eval-session")
	if err != nil {
		return nil, err
	}

	result := &AuditTrailResult{}
	for _, e := range entries {
		switch e.Type {
		case "retrieval_query":
			result.QueryEntries++
			if scores, ok := e.Metadata["results"].([]interface{}); ok && len(scores) > 0 {
				result.MatchedQuery = true
				result.ResultsLogged = len(scores)
			}
		case "retrieval_judgment":
			result.JudgmentEntries++
			result.MatchedJudgment = true
		}
	}

	return result, nil
}

// buildJudgePrompt constructs the user prompt for the judge LLM.
func buildJudgePrompt(query string, results []SearchResultFromMCP) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Query: %q\n\nSearch Results:\n", query)

	for i, r := range results {
		snippet := r.Content
		if len(snippet) > maxSnippetLen {
			snippet = snippet[:maxSnippetLen] + "..."
		}
		fmt.Fprintf(&b, "[%d] %s (%s) — score: %.4f\n%s\n\n", i+1, r.Title, r.Type, r.Score, snippet)
	}

	b.WriteString("Evaluate each result for relevance to the query. Return ONLY valid JSON:\n")
	b.WriteString(`{"relevant_results": [1, 3], "reasoning": "..."}`)
	b.WriteString("\n")

	return b.String()
}

// mapIndicesToIDs converts 1-indexed result indices to document IDs.
func mapIndicesToIDs(indices []int, retrievedIDs []string) []string {
	var ids []string
	for _, idx := range indices {
		i := idx - 1 // convert to 0-indexed
		if i >= 0 && i < len(retrievedIDs) {
			ids = append(ids, retrievedIDs[i])
		}
	}
	return ids
}
