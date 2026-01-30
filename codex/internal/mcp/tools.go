package mcp

import (
	"context"
	"fmt"
	"time"

	"os"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/google/uuid"
)

// ToolHandler handles MCP tool calls
type ToolHandler struct {
	engine    *core.SearchEngine
	sessionID string
}

// NewToolHandler creates a new tool handler
func NewToolHandler(engine *core.SearchEngine, sessionID string) *ToolHandler {
	return &ToolHandler{
		engine:    engine,
		sessionID: sessionID,
	}
}

// Handle dispatches a tool call to the appropriate handler
func (h *ToolHandler) Handle(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "recall_search":
		return h.handleSearch(ctx, args)
	case "recall_get":
		return h.handleGet(ctx, args)
	case "recall_add":
		return h.handleAdd(ctx, args)
	case "recall_feedback":
		return h.handleFeedback(args)
	case "flight_recorder_log":
		return h.handleFlightRecorderLog(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

const (
	maxContentSize = 1 << 20  // 1MB
	maxQuerySize   = 10 << 10 // 10KB
)

func (h *ToolHandler) handleSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if len(query) > maxQuerySize {
		return nil, fmt.Errorf("query exceeds maximum size of 10KB")
	}

	var types []string
	if t, ok := args["types"].([]interface{}); ok {
		for _, v := range t {
			if str, ok := v.(string); ok {
				types = append(types, str)
			}
		}
	}

	scope, _ := args["scope"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := h.engine.Search(ctx, core.SearchRequest{
		Query: query,
		Types: types,
		Scope: scope,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}

	// Auto-log successful search for audit trail
	scores := make([]map[string]interface{}, len(results))
	for i, r := range results {
		scores[i] = map[string]interface{}{
			"id": r.ID, "title": r.Title, "type": r.Type, "score": r.Score,
		}
	}
	_ = h.engine.LogFlightRecorder(&core.FlightRecorderEntry{
		ID:        uuid.New().String(),
		SessionID: h.sessionID,
		Timestamp: time.Now(),
		Type:      core.FlightTypeRetrievalQuery,
		Content:   fmt.Sprintf("recall_search: %q → %d results", query, len(results)),
		Metadata: map[string]interface{}{
			"query": query, "types": types, "scope": scope,
			"limit": limit, "results": scores,
		},
	})

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, nil
}

func (h *ToolHandler) handleGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return h.engine.Get(ctx, id)
}

func (h *ToolHandler) handleAdd(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	itemType, _ := args["type"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	scope, _ := args["scope"].(string)

	if itemType == "" || title == "" || content == "" {
		return nil, fmt.Errorf("type, title, and content are required")
	}
	if len(content) > maxContentSize {
		return nil, fmt.Errorf("content exceeds maximum size of 1MB")
	}

	if scope == "" {
		scope = "project"
	}

	var tags []string
	if t, ok := args["tags"].([]interface{}); ok {
		for _, v := range t {
			if str, ok := v.(string); ok {
				tags = append(tags, str)
			}
		}
	}

	id := generateID(itemType)
	now := time.Now()

	// Auto-inject project attribution from environment
	var metadata map[string]interface{}
	if projectName := os.Getenv("EDI_PROJECT_NAME"); projectName != "" {
		metadata = map[string]interface{}{
			"project_name": projectName,
		}
		if projectPath := os.Getenv("EDI_PROJECT_PATH"); projectPath != "" {
			metadata["project_path"] = projectPath
		}
	}

	item := &core.Item{
		ID:        id,
		Type:      itemType,
		Title:     title,
		Content:   content,
		Tags:      tags,
		Scope:     scope,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.engine.Add(ctx, item); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":      id,
		"message": fmt.Sprintf("Added %s: %s", itemType, title),
	}, nil
}

func (h *ToolHandler) handleFeedback(args map[string]interface{}) (interface{}, error) {
	itemID, _ := args["item_id"].(string)
	useful, _ := args["useful"].(bool)
	ctx, _ := args["context"].(string)

	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	feedback := &core.Feedback{
		ItemID:    itemID,
		SessionID: h.sessionID,
		Useful:    useful,
		Context:   ctx,
		Timestamp: time.Now(),
	}

	if err := h.engine.RecordFeedback(feedback); err != nil {
		return nil, err
	}

	return map[string]string{"status": "recorded"}, nil
}

func (h *ToolHandler) handleFlightRecorderLog(args map[string]interface{}) (interface{}, error) {
	entryType, _ := args["type"].(string)
	content, _ := args["content"].(string)
	rationale, _ := args["rationale"].(string)

	if entryType == "" || content == "" {
		return nil, fmt.Errorf("type and content are required")
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	entry := &core.FlightRecorderEntry{
		ID:        uuid.New().String(),
		SessionID: h.sessionID,
		Timestamp: time.Now(),
		Type:      entryType,
		Content:   content,
		Rationale: rationale,
		Metadata:  metadata,
	}

	if err := h.engine.LogFlightRecorder(entry); err != nil {
		return nil, err
	}

	return map[string]string{"status": "logged"}, nil
}

func generateID(itemType string) string {
	prefix := map[string]string{
		core.TypePattern:  "P",
		core.TypeFailure:  "F",
		core.TypeDecision: "D",
		core.TypeContext:  "C",
		core.TypeCode:     "X",
		core.TypeDoc:      "O",
		core.TypeRunbook:  "R",
	}[itemType]

	if prefix == "" {
		prefix = "I"
	}

	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}

// getToolDefinitions returns the MCP tool definitions
func getToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "recall_search",
			Description: "Search organizational knowledge for patterns, failures, decisions, and code",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query for knowledge items",
					},
					"types": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Filter by type: pattern, failure, decision, context, code, doc",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Filter by scope: global, project, all",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum results (default 10)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "recall_get",
			Description: "Get a specific knowledge item by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the knowledge item to retrieve",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "recall_add",
			Description: "Add new knowledge to Codex",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type: pattern, failure, decision, code, doc",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Brief title",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Full content/description",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Tags for categorization",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Scope: global or project (default: project)",
					},
				},
				"required": []string{"type", "title", "content"},
			},
		},
		{
			Name:        "recall_feedback",
			Description: "Provide feedback on whether a Codex item was useful",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"item_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the item to provide feedback on",
					},
					"useful": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the item was useful",
					},
					"context": map[string]interface{}{
						"type":        "string",
						"description": "Context about how it was used",
					},
				},
				"required": []string{"item_id", "useful"},
			},
		},
		{
			Name:        "flight_recorder_log",
			Description: "Log decisions, errors, and milestones during work",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type: decision, error, milestone, observation, retrieval_query, retrieval_judgment",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "What happened",
					},
					"rationale": map[string]interface{}{
						"type":        "string",
						"description": "Why (for decisions)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Additional structured data",
					},
				},
				"required": []string{"type", "content"},
			},
		},
	}
}
