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
	voyageBaseURL      = "https://api.voyageai.com/v1/embeddings"
	voyageModel        = "voyage-code-3"
	voyageBatchSize    = 128 // Voyage API max batch size
	voyageMaxRetries   = 3
	voyageInitialDelay = 1 * time.Second
)

// VoyageClient handles Voyage AI embeddings
type VoyageClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type voyageRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type"`
}

type voyageResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

type voyageError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewVoyageClient creates a new Voyage client
func NewVoyageClient(apiKey string) *VoyageClient {
	return &VoyageClient{
		apiKey:  apiKey,
		baseURL: voyageBaseURL,
		client:  &http.Client{},
	}
}

// EmbedCode embeds code snippets for storage
func (c *VoyageClient) EmbedCode(ctx context.Context, texts []string) ([]float32, error) {
	embeddings, err := c.embed(ctx, texts, "document")
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedCodeBatch embeds multiple code snippets
// Automatically handles batching if texts exceed Voyage's batch size limit
func (c *VoyageClient) EmbedCodeBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// If within batch size, process directly
	if len(texts) <= voyageBatchSize {
		return c.embed(ctx, texts, "document")
	}

	// Split into batches
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += voyageBatchSize {
		end := i + voyageBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := c.embed(ctx, batch, "document")
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// EmbedCodeQuery embeds a search query
func (c *VoyageClient) EmbedCodeQuery(ctx context.Context, query string) ([]float32, error) {
	embeddings, err := c.embed(ctx, []string{query}, "query")
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

func (c *VoyageClient) embed(ctx context.Context, texts []string, inputType string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("VOYAGE_API_KEY not set")
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	if len(texts) > voyageBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds Voyage limit of %d", len(texts), voyageBatchSize)
	}

	req := voyageRequest{
		Input:     texts,
		Model:     voyageModel,
		InputType: inputType,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry with exponential backoff
	var lastErr error
	for attempt := 0; attempt < voyageMaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt))) * voyageInitialDelay
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
			var voyageErr voyageError
			if json.Unmarshal(respBody, &voyageErr) == nil && voyageErr.Error.Message != "" {
				lastErr = fmt.Errorf("Voyage API error (%d): %s", resp.StatusCode, voyageErr.Error.Message)
			} else {
				lastErr = fmt.Errorf("Voyage API error (%d): %s", resp.StatusCode, string(respBody))
			}

			// Retry on rate limit (429) or server errors (5xx)
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				continue
			}

			// Don't retry on client errors (4xx except 429)
			return nil, lastErr
		}

		// Parse successful response
		var voyageResp voyageResponse
		if err := json.Unmarshal(respBody, &voyageResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(voyageResp.Data) != len(texts) {
			return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(voyageResp.Data))
		}

		embeddings := make([][]float32, len(voyageResp.Data))
		for i, d := range voyageResp.Data {
			embeddings[i] = d.Embedding
		}

		return embeddings, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", voyageMaxRetries, lastErr)
}
