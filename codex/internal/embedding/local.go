package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

const (
	defaultLocalBaseURL = "http://localhost:11434/api/embed"
	defaultLocalModel   = "nomic-embed-text"
	localMaxRetries     = 3
	localInitialDelay   = 500 * time.Millisecond
	localHTTPTimeout    = 10 * time.Second

	// Circuit breaker constants
	circuitBreakerThreshold = 3                // consecutive failures to open circuit
	circuitBreakerCooldown  = 30 * time.Second // how long circuit stays open
)

// LocalClient handles embedding via an Ollama-compatible API.
// It implements core.Embedder using nomic-embed-text by default.
// Uses nomic task prefixes: "search_document: " for indexing,
// "search_query: " for queries.
//
// Includes a circuit breaker that fast-fails after consecutive failures
// to avoid blocking callers when the embedding service is down.
type LocalClient struct {
	baseURL string
	model   string
	client  *http.Client

	// Circuit breaker state
	mu              sync.Mutex
	consecutiveFail int
	lastFailTime    time.Time
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
		client:  &http.Client{Timeout: localHTTPTimeout},
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

// Ping verifies the embedding service is reachable by sending a minimal request.
// Bypasses the circuit breaker so it can be used to probe after failures.
// A successful Ping resets the circuit breaker.
func (c *LocalClient) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.embedDirect(ctx, "ping")
	if err != nil {
		return fmt.Errorf("embedding service unreachable at %s: %w", c.baseURL, err)
	}
	c.recordSuccess()
	return nil
}

// embed wraps embedDirect with circuit breaker logic.
func (c *LocalClient) embed(ctx context.Context, text string) ([]float32, error) {
	if c.circuitOpen() {
		return nil, fmt.Errorf("embedding circuit breaker open: service unavailable (cooldown %v)", circuitBreakerCooldown)
	}

	vec, err := c.embedDirect(ctx, text)
	if err != nil {
		c.recordFailure()
		return nil, err
	}

	c.recordSuccess()
	return vec, nil
}

// embedDirect performs the HTTP embedding request with retry logic.
// Does not check the circuit breaker — that's the caller's responsibility.
func (c *LocalClient) embedDirect(ctx context.Context, text string) ([]float32, error) {
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

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
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

// circuitOpen returns true if the circuit breaker is open (fast-fail mode).
func (c *LocalClient) circuitOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.consecutiveFail < circuitBreakerThreshold {
		return false
	}
	if time.Since(c.lastFailTime) > circuitBreakerCooldown {
		// Cooldown expired — allow a probe request
		c.consecutiveFail = 0
		return false
	}
	return true
}

// recordSuccess resets the circuit breaker.
func (c *LocalClient) recordSuccess() {
	c.mu.Lock()
	c.consecutiveFail = 0
	c.mu.Unlock()
}

// recordFailure increments the consecutive failure counter.
func (c *LocalClient) recordFailure() {
	c.mu.Lock()
	c.consecutiveFail++
	c.lastFailTime = time.Now()
	c.mu.Unlock()
}
