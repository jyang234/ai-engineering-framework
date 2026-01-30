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
	defaultLocalBaseURL = "http://localhost:11434/api/embed"
	defaultLocalModel   = "nomic-embed-text"
	localMaxRetries     = 5
	localInitialDelay   = 1 * time.Second
)

// LocalClient handles embedding via an Ollama-compatible API.
// It implements core.Embedder using nomic-embed-text by default.
// Uses nomic task prefixes: "search_document: " for indexing,
// "search_query: " for queries.
type LocalClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// LocalClientOption configures a LocalClient.
type LocalClientOption func(*LocalClient)

// WithLocalBaseURL sets the inference server URL.
func WithLocalBaseURL(url string) LocalClientOption {
	return func(c *LocalClient) { c.baseURL = url }
}

// WithLocalModel sets the model name.
func WithLocalModel(model string) LocalClientOption {
	return func(c *LocalClient) { c.model = model }
}

// NewLocalClient creates a local embedding client that talks to an
// Ollama-compatible HTTP endpoint. Defaults to localhost:11434 with
// nomic-embed-text.
func NewLocalClient(opts ...LocalClientOption) *LocalClient {
	c := &LocalClient{
		baseURL: defaultLocalBaseURL,
		model:   defaultLocalModel,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ollamaEmbedRequest is the Ollama /api/embed request body.
type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// ollamaEmbedResponse is the Ollama /api/embed response body.
type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// EmbedDocument embeds a text for storage/indexing.
// Uses "search_document: " prefix for asymmetric retrieval.
func (c *LocalClient) EmbedDocument(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, "search_document: "+text)
}

// EmbedQuery embeds a search query.
// Uses "search_query: " prefix for asymmetric retrieval.
func (c *LocalClient) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return c.embed(ctx, "search_query: "+query)
}

func (c *LocalClient) embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model: c.model,
		Input: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < localMaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * localInitialDelay
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
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("local embedding request failed: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("local embedding error (%d): %s", resp.StatusCode, string(respBody))
			if resp.StatusCode >= 500 {
				continue
			}
			return nil, lastErr
		}

		var embedResp ollamaEmbedResponse
		if err := json.Unmarshal(respBody, &embedResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(embedResp.Embeddings) == 0 {
			return nil, fmt.Errorf("no embeddings returned")
		}

		return embedResp.Embeddings[0], nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", localMaxRetries, lastErr)
}
