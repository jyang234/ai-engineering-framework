package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	openaiBaseURL      = "https://api.openai.com/v1/embeddings"
	openaiModel        = "text-embedding-3-large"
	openaiBatchSize    = 2048 // OpenAI max batch size
	openaiMaxRetries   = 3
	openaiInitialDelay = 1 * time.Second
)

// OpenAIClient handles OpenAI embeddings
type OpenAIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type openaiRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openaiResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: openaiBaseURL,
		client:  &http.Client{},
	}
}

// EmbedDocuments embeds documentation for storage
// Automatically handles batching if texts exceed OpenAI's batch size limit
func (c *OpenAIClient) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// If within batch size, process directly
	if len(texts) <= openaiBatchSize {
		return c.embed(ctx, texts)
	}

	// Split into batches
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += openaiBatchSize {
		end := i + openaiBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := c.embed(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// EmbedDocument embeds a single document
func (c *OpenAIClient) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedQuery embeds a search query
func (c *OpenAIClient) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return c.EmbedDocument(ctx, query)
}

func (c *OpenAIClient) embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	if len(texts) > openaiBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds OpenAI limit of %d", len(texts), openaiBatchSize)
	}

	req := openaiRequest{
		Input: texts,
		Model: openaiModel,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry with exponential backoff
	var lastErr error
	for attempt := 0; attempt < openaiMaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt))) * openaiInitialDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		// Read response body for error handling
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Handle non-200 responses
		if resp.StatusCode != http.StatusOK {
			// Try to parse error response
			var openaiErr openaiError
			if json.Unmarshal(respBody, &openaiErr) == nil && openaiErr.Error.Message != "" {
				lastErr = fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, openaiErr.Error.Message)
			} else {
				lastErr = fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
			}

			// Retry on rate limit (429) or server errors (5xx)
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				continue
			}

			// Don't retry on client errors (4xx except 429)
			return nil, lastErr
		}

		// Parse successful response
		var openaiResp openaiResponse
		if err := json.Unmarshal(respBody, &openaiResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(openaiResp.Data) != len(texts) {
			return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(openaiResp.Data))
		}

		embeddings := make([][]float32, len(openaiResp.Data))
		for _, d := range openaiResp.Data {
			if d.Index < 0 || d.Index >= len(embeddings) {
				return nil, fmt.Errorf("invalid embedding index: %d", d.Index)
			}
			embeddings[d.Index] = d.Embedding
		}

		return embeddings, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", openaiMaxRetries, lastErr)
}
