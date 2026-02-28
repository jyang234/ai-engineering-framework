package embedding

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrefixes(t *testing.T) {
	var gotBodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBodies = append(gotBodies, string(body))
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
		})
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	ctx := context.Background()

	// EmbedDocument
	vec, err := client.EmbedDocument(ctx, "hello world")
	if err != nil {
		t.Fatalf("EmbedDocument: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("EmbedDocument returned %d dims, want 3", len(vec))
	}

	// EmbedQuery
	vec, err = client.EmbedQuery(ctx, "hello world")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("EmbedQuery returned %d dims, want 3", len(vec))
	}

	// Verify prefixes
	if len(gotBodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(gotBodies))
	}

	var docReq ollamaEmbedRequest
	json.Unmarshal([]byte(gotBodies[0]), &docReq)
	if docReq.Input != "search_document: hello world" {
		t.Errorf("document input = %q, want %q", docReq.Input, "search_document: hello world")
	}
	if docReq.Model != "nomic-embed-text" {
		t.Errorf("document model = %q, want %q", docReq.Model, "nomic-embed-text")
	}

	var queryReq ollamaEmbedRequest
	json.Unmarshal([]byte(gotBodies[1]), &queryReq)
	if queryReq.Input != "search_query: hello world" {
		t.Errorf("query input = %q, want %q", queryReq.Input, "search_query: hello world")
	}
}

func TestSuccessfulRoundTrip(t *testing.T) {
	expected := []float32{0.5, 0.6, 0.7, 0.8}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{expected},
		})
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	vec, err := client.EmbedQuery(context.Background(), "test")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(vec) != len(expected) {
		t.Fatalf("got %d dims, want %d", len(vec), len(expected))
	}
	for i, v := range vec {
		if v != expected[i] {
			t.Errorf("vec[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestRetryOn5xx(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n <= 2 {
			w.WriteHeader(500)
			w.Write([]byte("server overloaded"))
			return
		}
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
		})
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	vec, err := client.EmbedQuery(context.Background(), "test")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("got %d dims, want 3", len(vec))
	}
	if got := count.Load(); got != 3 {
		t.Errorf("server received %d requests, want 3 (2 failures + 1 success)", got)
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(400)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	_, err := client.EmbedQuery(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "local embedding error (400)") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "local embedding error (400)")
	}
	if got := count.Load(); got != 1 {
		t.Errorf("server received %d requests, want 1 (no retry on 4xx)", got)
	}
}

func TestMaxRetriesExhausted(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(500)
		w.Write([]byte("always failing"))
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	// Use a timeout to prevent test from waiting the full 30s of retries.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, err := client.EmbedQuery(ctx, "test")
	if err == nil {
		t.Fatal("expected error after max retries")
	}
	if !strings.Contains(err.Error(), "max retries") {
		// Might be context deadline exceeded if timeout hit first
		if !strings.Contains(err.Error(), "context") {
			t.Errorf("error = %q, want to contain 'max retries' or 'context'", err.Error())
		}
	}
	// Should have made at least 2 requests before context might cancel
	if got := count.Load(); got < 2 {
		t.Errorf("server received %d requests, want at least 2", got)
	}
}

func TestContextCancellation(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(500)
		w.Write([]byte("failing"))
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first retry delay starts
	go func() {
		// Wait a tiny bit for the first request to complete, then cancel
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	_, err := client.EmbedQuery(ctx, "test")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// Should have made 1 request before cancel kicks in during retry delay
	got := count.Load()
	if got == 0 {
		t.Error("expected at least 1 request before cancellation")
	}
	if got >= int32(localMaxRetries) {
		t.Errorf("received %d requests — cancellation didn't stop retries", got)
	}
}

func TestEmptyEmbeddingsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{},
		})
	}))
	defer srv.Close()

	client := NewLocalClient(WithLocalBaseURL(srv.URL))
	_, err := client.EmbedQuery(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty embeddings")
	}
	if !strings.Contains(err.Error(), "no embeddings returned") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no embeddings returned")
	}
}

func TestCustomURLAndModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ollamaEmbedRequest
		json.Unmarshal(body, &req)
		gotModel = req.Model
		json.NewEncoder(w).Encode(ollamaEmbedResponse{
			Embeddings: [][]float32{{0.1}},
		})
	}))
	defer srv.Close()

	client := NewLocalClient(
		WithLocalBaseURL(srv.URL),
		WithLocalModel("custom-embed-model"),
	)
	vec, err := client.EmbedQuery(context.Background(), "test")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if len(vec) != 1 {
		t.Errorf("got %d dims, want 1", len(vec))
	}
	if gotModel != "custom-embed-model" {
		t.Errorf("model = %q, want %q", gotModel, "custom-embed-model")
	}
}
