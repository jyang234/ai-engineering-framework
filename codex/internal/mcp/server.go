package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/anthropics/aef/codex/internal/core"
)

// Server implements the MCP server for Codex
type Server struct {
	engine    *core.SearchEngine
	sessionID string
}

// NewServer creates a new MCP server
func NewServer(engine *core.SearchEngine, sessionID string) *Server {
	return &Server{
		engine:    engine,
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

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			if err := s.sendResponse(writer, resp); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(ctx, req)
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
				Name:    "codex",
				Version: "1.0.0",
			},
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{},
			},
		},
	}
}

func (s *Server) handleListTools(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ListToolsResult{Tools: getToolDefinitions()},
	}
}

func (s *Server) handleCallTool(ctx context.Context, req *MCPRequest) *MCPResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid params"},
		}
	}

	handler := NewToolHandler(s.engine, s.sessionID)
	result, err := handler.Handle(ctx, params.Name, params.Arguments)

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
