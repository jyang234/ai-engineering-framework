package validator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRequest(method, body string, contentType string) *http.Request {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/test", bodyReader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func TestMiddlewareRejectsNonJSON(t *testing.T) {
	v := NewValidator(nil, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	tests := []struct {
		name        string
		contentType string
		wantCode    int
	}{
		{"no content type", "", http.StatusUnsupportedMediaType},
		{"text/plain", "text/plain", http.StatusUnsupportedMediaType},
		{"text/html", "text/html", http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("POST", `{"name":"test"}`, tt.contentType)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}
}

func TestMiddlewareAcceptsJSON(t *testing.T) {
	rules := []ValidationRule{
		{Field: "name", Required: true, Type: "string"},
	}
	v := NewValidator(rules, 1<<20)

	var calledWith map[string]interface{}
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledWith = ParsedBody(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := newRequest("POST", `{"name":"Alice"}`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rr.Code)
	}
	if calledWith == nil {
		t.Fatal("parsed body was nil")
	}
	if calledWith["name"] != "Alice" {
		t.Errorf("got name=%v, want Alice", calledWith["name"])
	}
}

func TestMiddlewareAcceptsJSONWithCharset(t *testing.T) {
	v := NewValidator(nil, 1<<20)
	called := false
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := newRequest("POST", `{}`, "application/json; charset=utf-8")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestMiddlewareEnforcesMaxBodySize(t *testing.T) {
	v := NewValidator(nil, 100) // 100 byte limit
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for oversized body")
	}))

	largeBody := `{"data":"` + strings.Repeat("x", 200) + `"}`
	req := newRequest("POST", largeBody, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got status %d, want 413 for oversized body", rr.Code)
	}
}

func TestMiddlewareValidatesRequiredFields(t *testing.T) {
	rules := []ValidationRule{
		{Field: "name", Required: true, Type: "string"},
		{Field: "email", Required: true, Type: "string"},
	}
	v := NewValidator(rules, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with missing required fields")
	}))

	// Missing "email"
	req := newRequest("POST", `{"name":"Alice"}`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for missing required field", rr.Code)
	}
}

func TestMiddlewareValidatesFieldTypes(t *testing.T) {
	rules := []ValidationRule{
		{Field: "name", Required: true, Type: "string"},
		{Field: "age", Required: true, Type: "number"},
	}
	v := NewValidator(rules, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with wrong types")
	}))

	// "age" is a string instead of number
	req := newRequest("POST", `{"name":"Alice","age":"thirty"}`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for wrong field type", rr.Code)
	}
}

func TestMiddlewareValidatesStringLength(t *testing.T) {
	rules := []ValidationRule{
		{Field: "name", Required: true, Type: "string", MinLength: 2, MaxLength: 50},
	}
	v := NewValidator(rules, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid string length")
	}))

	req := newRequest("POST", `{"name":"A"}`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for string too short", rr.Code)
	}
}

func TestMiddlewareOptionalFieldsSkippedWhenAbsent(t *testing.T) {
	rules := []ValidationRule{
		{Field: "name", Required: true, Type: "string"},
		{Field: "age", Required: false, Type: "number"},
	}
	v := NewValidator(rules, 1<<20)
	called := false
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := newRequest("POST", `{"name":"Alice"}`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want 200 (optional field absent)", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestMiddlewareRejectsInvalidJSON(t *testing.T) {
	v := NewValidator(nil, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid JSON")
	}))

	req := newRequest("POST", `{not json`, "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want 400 for invalid JSON", rr.Code)
	}
}

// TestMiddlewareBodyAlwaysClosed verifies the request body is always closed.
// This catches resource leaks where body.Close() is not called on error paths.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestMiddlewareBodyAlwaysClosed(t *testing.T) {
	v := NewValidator(nil, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{"valid request", `{"name":"test"}`, "application/json"},
		{"invalid json", `{broken`, "application/json"},
		{"wrong content type", `{"name":"test"}`, "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := &trackingReadCloser{Reader: strings.NewReader(tt.body)}
			req := httptest.NewRequest("POST", "/test", tracker)
			req.Header.Set("Content-Type", tt.contentType)
			req.Body = tracker

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// The body should be closed regardless of what happened
			if !tracker.closed {
				t.Errorf("request body was NOT closed on %s — this is a resource leak", tt.name)
			}
		})
	}
}

func TestMiddlewareReturnsJSONErrors(t *testing.T) {
	v := NewValidator(nil, 1<<20)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := newRequest("POST", `{"name":"test"}`, "text/plain")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "error") && !strings.Contains(body, "Error") {
		t.Errorf("error response should contain 'error' field, got: %s", body)
	}
}

func TestParsedBodyReturnsNilWithoutMiddleware(t *testing.T) {
	// A plain context (not from the middleware) should return nil
	body := ParsedBody(context.Background())
	if body != nil {
		t.Errorf("expected nil for context without parsed body, got %v", body)
	}
}
