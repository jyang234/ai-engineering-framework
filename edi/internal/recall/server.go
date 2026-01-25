package recall

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

// Server implements the MCP server for RECALL
type Server struct {
	storage   *Storage
	sessionID string
}

// NewServer creates a new RECALL MCP server
func NewServer(storage *Storage, sessionID string) *Server {
	return &Server{
		storage:   storage,
		sessionID: sessionID,
	}
}

// MCP Protocol Types
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Run starts the MCP server on stdio
func (s *Server) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read line (JSON-RPC message)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(writer, nil, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			if err := s.sendResponse(writer, resp); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleRequest(req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	case "notifications/initialized":
		return nil // Notification, no response
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Method not found"},
		}
	}
}

func (s *Server) handleInitialize(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    "recall",
				Version: "1.0.0",
			},
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{},
			},
		},
	}
}

func (s *Server) handleListTools(req *MCPRequest) *MCPResponse {
	tools := []Tool{
		{
			Name:        "recall_search",
			Description: "Search organizational knowledge for patterns, failures, and decisions",
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
						"description": "Filter by type: pattern, failure, decision, context",
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
			Description: "Add new knowledge to RECALL",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type: pattern, failure, decision",
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
			Description: "Provide feedback on whether a RECALL item was useful",
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
						"description": "Type: decision, error, milestone, observation, task_annotation, task_complete",
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

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListToolsResult{Tools: tools},
	}
}

func (s *Server) handleCallTool(req *MCPRequest) *MCPResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid params"},
		}
	}

	var result interface{}
	var err error

	switch params.Name {
	case "recall_search":
		result, err = s.handleSearch(params.Arguments)
	case "recall_get":
		result, err = s.handleGet(params.Arguments)
	case "recall_add":
		result, err = s.handleAdd(params.Arguments)
	case "recall_feedback":
		result, err = s.handleFeedback(params.Arguments)
	case "flight_recorder_log":
		result, err = s.handleFlightRecorderLog(params.Arguments)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Unknown tool"},
		}
	}

	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			},
		}
	}

	resultJSON, _ := json.Marshal(result)
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: CallToolResult{
			Content: []ToolContent{{Type: "text", Text: string(resultJSON)}},
		},
	}
}

func (s *Server) handleSearch(args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
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

	items, err := s.storage.Search(query, types, scope, limit)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"results": items,
		"count":   len(items),
	}, nil
}

func (s *Server) handleGet(args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.storage.Get(id)
}

func (s *Server) handleAdd(args map[string]interface{}) (interface{}, error) {
	itemType, _ := args["type"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	scope, _ := args["scope"].(string)

	if itemType == "" || title == "" || content == "" {
		return nil, fmt.Errorf("type, title, and content are required")
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

	item := &Item{
		ID:        id,
		Type:      itemType,
		Title:     title,
		Content:   content,
		Tags:      tags,
		Scope:     scope,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.storage.Add(item); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":      id,
		"message": fmt.Sprintf("Added %s: %s", itemType, title),
	}, nil
}

func (s *Server) handleFeedback(args map[string]interface{}) (interface{}, error) {
	itemID, _ := args["item_id"].(string)
	useful, _ := args["useful"].(bool)
	ctx, _ := args["context"].(string)

	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	if err := s.storage.RecordFeedback(itemID, s.sessionID, useful, ctx); err != nil {
		return nil, err
	}

	return map[string]string{"status": "recorded"}, nil
}

func (s *Server) handleFlightRecorderLog(args map[string]interface{}) (interface{}, error) {
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

	entry := &FlightRecorderEntry{
		SessionID: s.sessionID,
		Timestamp: time.Now(),
		Type:      entryType,
		Content:   content,
		Rationale: rationale,
		Metadata:  metadata,
	}

	if err := s.storage.LogFlightRecorder(entry); err != nil {
		return nil, err
	}

	return map[string]string{"status": "logged"}, nil
}

func (s *Server) sendResponse(w io.Writer, resp *MCPResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func (s *Server) sendError(w io.Writer, id interface{}, code int, message string) error {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
	return s.sendResponse(w, resp)
}

func generateID(itemType string) string {
	prefix := map[string]string{
		"pattern":  "P",
		"failure":  "F",
		"decision": "D",
		"context":  "C",
	}[itemType]

	if prefix == "" {
		prefix = "X"
	}

	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}
