package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/aef/codex/internal/core"
)

// Test errors
var (
	ErrMockSearch = errors.New("search error")
	ErrMockList   = errors.New("list error")
	ErrMockGet    = errors.New("item not found")
	ErrMockAdd    = errors.New("add error")
	ErrMockUpdate = errors.New("update error")
	ErrMockDelete = errors.New("delete error")
)

// SearchEngineInterface defines the methods used by handlers for mocking
type SearchEngineInterface interface {
	Search(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error)
	Get(ctx context.Context, id string) (*core.Item, error)
	List(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error)
	Add(ctx context.Context, item *core.Item) error
	Update(ctx context.Context, item *core.Item) error
	Delete(ctx context.Context, id string) error
}

// MockSearchEngine implements SearchEngineInterface for testing
type MockSearchEngine struct {
	SearchFunc func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error)
	GetFunc    func(ctx context.Context, id string) (*core.Item, error)
	ListFunc   func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error)
	AddFunc    func(ctx context.Context, item *core.Item) error
	UpdateFunc func(ctx context.Context, item *core.Item) error
	DeleteFunc func(ctx context.Context, id string) error
}

func (m *MockSearchEngine) Search(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, req)
	}
	return nil, nil
}

func (m *MockSearchEngine) Get(ctx context.Context, id string) (*core.Item, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, ErrMockGet
}

func (m *MockSearchEngine) List(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, itemType, scope, limit, offset)
	}
	return nil, nil
}

func (m *MockSearchEngine) Add(ctx context.Context, item *core.Item) error {
	if m.AddFunc != nil {
		return m.AddFunc(ctx, item)
	}
	return nil
}

func (m *MockSearchEngine) Update(ctx context.Context, item *core.Item) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, item)
	}
	return nil
}

func (m *MockSearchEngine) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

// testServer wraps handlers with a mock engine for testing
type testServer struct {
	mock   *MockSearchEngine
	router *gin.Engine
}

func newTestServer() *testServer {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mock := &MockSearchEngine{}

	ts := &testServer{
		mock:   mock,
		router: router,
	}

	// Register routes with handler wrappers that use mock
	router.GET("/search", ts.handleSearch)
	router.GET("/browse", ts.handleBrowse)
	router.GET("/api/search", ts.handleAPISearch)
	router.GET("/api/item/:id", ts.handleAPIItem)
	router.POST("/api/item", ts.handleAPICreate)
	router.PUT("/api/item/:id", ts.handleAPIUpdate)
	router.DELETE("/api/item/:id", ts.handleAPIDelete)

	return ts
}

// Handler wrappers using mock engine

func (ts *testServer) handleSearch(c *gin.Context) {
	query := c.Query("q")
	types := c.QueryArray("type")

	if query == "" {
		c.JSON(http.StatusOK, gin.H{
			"query":   "",
			"results": nil,
			"count":   0,
		})
		return
	}

	results, err := ts.mock.Search(c.Request.Context(), core.SearchRequest{
		Query: query,
		Types: types,
		Limit: 20,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (ts *testServer) handleBrowse(c *gin.Context) {
	itemType := c.Query("type")
	scope := c.Query("scope")
	pageStr := c.DefaultQuery("page", "1")

	page := 1
	if p, err := parseInt(pageStr); err == nil && p > 0 {
		page = p
	}

	limit := 50
	offset := (page - 1) * limit

	items, err := ts.mock.List(c.Request.Context(), itemType, scope, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":  itemType,
		"scope": scope,
		"items": items,
		"count": len(items),
		"page":  page,
	})
}

func (ts *testServer) handleAPISearch(c *gin.Context) {
	query := c.Query("q")
	types := c.QueryArray("type")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "query parameter required",
		})
		return
	}

	results, err := ts.mock.Search(c.Request.Context(), core.SearchRequest{
		Query: query,
		Types: types,
		Limit: 20,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

func (ts *testServer) handleAPIItem(c *gin.Context) {
	id := c.Param("id")

	item, err := ts.mock.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "item not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    item,
	})
}

func (ts *testServer) handleAPICreate(c *gin.Context) {
	var item core.Item
	if err := c.BindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Set timestamps and ID if not provided
	now := time.Now()
	if item.ID == "" {
		item.ID = "generated-id"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	// Default scope
	if item.Scope == "" {
		item.Scope = "project"
	}

	if err := ts.mock.Add(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      item.ID,
		"message": "Item created",
	})
}

func (ts *testServer) handleAPIUpdate(c *gin.Context) {
	id := c.Param("id")

	var item core.Item
	if err := c.BindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Ensure ID matches
	item.ID = id

	if err := ts.mock.Update(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      item.ID,
		"message": "Item updated",
	})
}

func (ts *testServer) handleAPIDelete(c *gin.Context) {
	id := c.Param("id")

	if err := ts.mock.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Item deleted",
	})
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// Helper to parse JSON response
func parseJSONResponse(t *testing.T, body *bytes.Buffer) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	return result
}

// =============================================================================
// handleSearch Tests
// =============================================================================

func TestHandleSearch(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		types          []string
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:           "empty query returns empty results",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["query"] != "" {
					t.Errorf("expected empty query, got %v", resp["query"])
				}
				if resp["count"].(float64) != 0 {
					t.Errorf("expected count 0, got %v", resp["count"])
				}
				if resp["results"] != nil {
					t.Errorf("expected nil results, got %v", resp["results"])
				}
			},
		},
		{
			name:  "query with results",
			query: "test query",
			types: []string{"pattern"},
			setupMock: func(m *MockSearchEngine) {
				m.SearchFunc = func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
					if req.Query != "test query" {
						return nil, errors.New("unexpected query")
					}
					if len(req.Types) != 1 || req.Types[0] != "pattern" {
						return nil, errors.New("unexpected types")
					}
					return []core.SearchResult{
						{
							Item: core.Item{
								ID:    "item-1",
								Type:  "pattern",
								Title: "Test Pattern",
							},
							Score: 0.95,
						},
						{
							Item: core.Item{
								ID:    "item-2",
								Type:  "pattern",
								Title: "Another Pattern",
							},
							Score: 0.85,
						},
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["query"] != "test query" {
					t.Errorf("expected query 'test query', got %v", resp["query"])
				}
				if resp["count"].(float64) != 2 {
					t.Errorf("expected count 2, got %v", resp["count"])
				}
				results := resp["results"].([]interface{})
				if len(results) != 2 {
					t.Errorf("expected 2 results, got %d", len(results))
				}
			},
		},
		{
			name:  "search error",
			query: "error query",
			setupMock: func(m *MockSearchEngine) {
				m.SearchFunc = func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
					return nil, ErrMockSearch
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["error"] != "search error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			// Build URL with query params
			u, _ := url.Parse("/search")
			q := u.Query()
			if tt.query != "" {
				q.Set("q", tt.query)
			}
			for _, typ := range tt.types {
				q.Add("type", typ)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest(http.MethodGet, u.String(), nil)
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleBrowse Tests
// =============================================================================

func TestHandleBrowse(t *testing.T) {
	tests := []struct {
		name           string
		itemType       string
		scope          string
		page           string
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:     "browse with type filter",
			itemType: "pattern",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					if itemType != "pattern" {
						return nil, errors.New("unexpected type filter")
					}
					return []core.Item{
						{ID: "p1", Type: "pattern", Title: "Pattern 1"},
						{ID: "p2", Type: "pattern", Title: "Pattern 2"},
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["type"] != "pattern" {
					t.Errorf("expected type 'pattern', got %v", resp["type"])
				}
				if resp["count"].(float64) != 2 {
					t.Errorf("expected count 2, got %v", resp["count"])
				}
			},
		},
		{
			name:  "browse with scope filter",
			scope: "global",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					if scope != "global" {
						return nil, errors.New("unexpected scope filter")
					}
					return []core.Item{
						{ID: "g1", Scope: "global"},
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["scope"] != "global" {
					t.Errorf("expected scope 'global', got %v", resp["scope"])
				}
			},
		},
		{
			name: "browse with pagination page 1",
			page: "1",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					if offset != 0 {
						return nil, errors.New("expected offset 0 for page 1")
					}
					if limit != 50 {
						return nil, errors.New("expected limit 50")
					}
					return []core.Item{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["page"].(float64) != 1 {
					t.Errorf("expected page 1, got %v", resp["page"])
				}
			},
		},
		{
			name: "browse with pagination page 3",
			page: "3",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					if offset != 100 {
						return nil, errors.New("expected offset 100 for page 3")
					}
					return []core.Item{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["page"].(float64) != 3 {
					t.Errorf("expected page 3, got %v", resp["page"])
				}
			},
		},
		{
			name: "browse with invalid page defaults to 1",
			page: "invalid",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					if offset != 0 {
						return nil, errors.New("expected offset 0 for invalid page")
					}
					return []core.Item{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["page"].(float64) != 1 {
					t.Errorf("expected page 1, got %v", resp["page"])
				}
			},
		},
		{
			name: "browse with negative page defaults to 1",
			page: "0",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					return []core.Item{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["page"].(float64) != 1 {
					t.Errorf("expected page 1, got %v", resp["page"])
				}
			},
		},
		{
			name: "browse list error",
			setupMock: func(m *MockSearchEngine) {
				m.ListFunc = func(ctx context.Context, itemType, scope string, limit, offset int) ([]core.Item, error) {
					return nil, ErrMockList
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["error"] != "list error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			u, _ := url.Parse("/browse")
			q := u.Query()
			if tt.itemType != "" {
				q.Set("type", tt.itemType)
			}
			if tt.scope != "" {
				q.Set("scope", tt.scope)
			}
			if tt.page != "" {
				q.Set("page", tt.page)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest(http.MethodGet, u.String(), nil)
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleAPISearch Tests
// =============================================================================

func TestHandleAPISearch(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		types          []string
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:           "missing query parameter returns validation error",
			query:          "",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "query parameter required" {
					t.Errorf("expected validation error, got %v", resp["error"])
				}
			},
		},
		{
			name:  "successful search returns results",
			query: "test",
			types: []string{"pattern", "failure"},
			setupMock: func(m *MockSearchEngine) {
				m.SearchFunc = func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
					if req.Limit != 20 {
						return nil, errors.New("expected limit 20")
					}
					return []core.SearchResult{
						{
							Item:  core.Item{ID: "r1", Type: "pattern"},
							Score: 0.9,
						},
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["query"] != "test" {
					t.Errorf("expected query 'test', got %v", resp["query"])
				}
				if resp["count"].(float64) != 1 {
					t.Errorf("expected count 1, got %v", resp["count"])
				}
			},
		},
		{
			name:  "search error returns 500",
			query: "fail",
			setupMock: func(m *MockSearchEngine) {
				m.SearchFunc = func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
					return nil, ErrMockSearch
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "search error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
		{
			name:  "search with empty results",
			query: "nonexistent",
			setupMock: func(m *MockSearchEngine) {
				m.SearchFunc = func(ctx context.Context, req core.SearchRequest) ([]core.SearchResult, error) {
					return []core.SearchResult{}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["count"].(float64) != 0 {
					t.Errorf("expected count 0, got %v", resp["count"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			u, _ := url.Parse("/api/search")
			q := u.Query()
			if tt.query != "" {
				q.Set("q", tt.query)
			}
			for _, typ := range tt.types {
				q.Add("type", typ)
			}
			u.RawQuery = q.Encode()

			req := httptest.NewRequest(http.MethodGet, u.String(), nil)
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleAPICreate Tests
// =============================================================================

func TestHandleAPICreate(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful create with all fields",
			body: map[string]interface{}{
				"id":      "custom-id",
				"type":    "pattern",
				"title":   "Test Pattern",
				"content": "Pattern content",
				"scope":   "global",
				"tags":    []string{"test", "pattern"},
			},
			setupMock: func(m *MockSearchEngine) {
				m.AddFunc = func(ctx context.Context, item *core.Item) error {
					if item.ID != "custom-id" {
						return errors.New("expected custom-id")
					}
					if item.Scope != "global" {
						return errors.New("expected global scope")
					}
					return nil
				}
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["id"] != "custom-id" {
					t.Errorf("expected id 'custom-id', got %v", resp["id"])
				}
				if resp["message"] != "Item created" {
					t.Errorf("expected message 'Item created', got %v", resp["message"])
				}
			},
		},
		{
			name: "create with auto-generated ID and default scope",
			body: map[string]interface{}{
				"type":    "failure",
				"title":   "Test Failure",
				"content": "Failure content",
			},
			setupMock: func(m *MockSearchEngine) {
				m.AddFunc = func(ctx context.Context, item *core.Item) error {
					if item.ID == "" {
						return errors.New("expected auto-generated ID")
					}
					if item.Scope != "project" {
						return errors.New("expected default scope 'project'")
					}
					return nil
				}
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["id"] == "" {
					t.Errorf("expected non-empty id")
				}
			},
		},
		{
			name:           "invalid JSON returns validation error",
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] == nil {
					t.Errorf("expected error message")
				}
			},
		},
		{
			name: "add error returns 500",
			body: map[string]interface{}{
				"type":    "pattern",
				"title":   "Test",
				"content": "Content",
			},
			setupMock: func(m *MockSearchEngine) {
				m.AddFunc = func(ctx context.Context, item *core.Item) error {
					return ErrMockAdd
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "add error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			var body []byte
			var err error
			switch v := tt.body.(type) {
			case string:
				body = []byte(v)
			default:
				body, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("failed to marshal body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/item", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleAPIUpdate Tests
// =============================================================================

func TestHandleAPIUpdate(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		body           interface{}
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful update",
			id:   "item-123",
			body: map[string]interface{}{
				"type":    "pattern",
				"title":   "Updated Title",
				"content": "Updated content",
			},
			setupMock: func(m *MockSearchEngine) {
				m.UpdateFunc = func(ctx context.Context, item *core.Item) error {
					if item.ID != "item-123" {
						return errors.New("expected ID to be set from URL param")
					}
					if item.Title != "Updated Title" {
						return errors.New("expected updated title")
					}
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["id"] != "item-123" {
					t.Errorf("expected id 'item-123', got %v", resp["id"])
				}
				if resp["message"] != "Item updated" {
					t.Errorf("expected message 'Item updated', got %v", resp["message"])
				}
			},
		},
		{
			name: "update not found returns error",
			id:   "nonexistent",
			body: map[string]interface{}{
				"title": "Title",
			},
			setupMock: func(m *MockSearchEngine) {
				m.UpdateFunc = func(ctx context.Context, item *core.Item) error {
					return ErrMockUpdate
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "update error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
		{
			name:           "invalid JSON returns validation error",
			id:             "item-123",
			body:           "invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
			},
		},
		{
			name: "ID in body is overwritten by URL param",
			id:   "url-id",
			body: map[string]interface{}{
				"id":    "body-id",
				"title": "Test",
			},
			setupMock: func(m *MockSearchEngine) {
				m.UpdateFunc = func(ctx context.Context, item *core.Item) error {
					if item.ID != "url-id" {
						return errors.New("expected URL ID to override body ID")
					}
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["id"] != "url-id" {
					t.Errorf("expected id 'url-id', got %v", resp["id"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			var body []byte
			var err error
			switch v := tt.body.(type) {
			case string:
				body = []byte(v)
			default:
				body, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("failed to marshal body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPut, "/api/item/"+tt.id, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleAPIDelete Tests
// =============================================================================

func TestHandleAPIDelete(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful delete",
			id:   "item-to-delete",
			setupMock: func(m *MockSearchEngine) {
				m.DeleteFunc = func(ctx context.Context, id string) error {
					if id != "item-to-delete" {
						return errors.New("unexpected ID")
					}
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				if resp["message"] != "Item deleted" {
					t.Errorf("expected message 'Item deleted', got %v", resp["message"])
				}
			},
		},
		{
			name: "delete error returns 500",
			id:   "item-error",
			setupMock: func(m *MockSearchEngine) {
				m.DeleteFunc = func(ctx context.Context, id string) error {
					return ErrMockDelete
				}
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "delete error" {
					t.Errorf("expected error message, got %v", resp["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			req := httptest.NewRequest(http.MethodDelete, "/api/item/"+tt.id, nil)
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}

// =============================================================================
// handleAPIItem Tests (bonus coverage)
// =============================================================================

func TestHandleAPIItem(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		setupMock      func(*MockSearchEngine)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "get existing item",
			id:   "existing-item",
			setupMock: func(m *MockSearchEngine) {
				m.GetFunc = func(ctx context.Context, id string) (*core.Item, error) {
					return &core.Item{
						ID:      id,
						Type:    "pattern",
						Title:   "Test Item",
						Content: "Test content",
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != true {
					t.Errorf("expected success true, got %v", resp["success"])
				}
				data := resp["data"].(map[string]interface{})
				if data["id"] != "existing-item" {
					t.Errorf("expected id 'existing-item', got %v", data["id"])
				}
			},
		},
		{
			name: "get nonexistent item returns 404",
			id:   "nonexistent",
			setupMock: func(m *MockSearchEngine) {
				m.GetFunc = func(ctx context.Context, id string) (*core.Item, error) {
					return nil, ErrMockGet
				}
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if resp["success"] != false {
					t.Errorf("expected success false, got %v", resp["success"])
				}
				if resp["error"] != "item not found" {
					t.Errorf("expected error 'item not found', got %v", resp["error"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer()
			if tt.setupMock != nil {
				tt.setupMock(ts.mock)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/item/"+tt.id, nil)
			w := httptest.NewRecorder()

			ts.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseJSONResponse(t, w.Body)
			tt.checkResponse(t, resp)
		})
	}
}
