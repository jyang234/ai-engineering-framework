package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/anthropics/aef/codex/internal/mcp"
)

// MCPClient communicates with the MCP server via JSON-RPC over io.Pipe.
type MCPClient struct {
	server    *mcp.Server
	writer    io.Writer
	reader    *bufio.Reader
	mu        sync.Mutex // serializes writes
	nextID    atomic.Int64
	cancel    context.CancelFunc
	serverErr chan error
}

// jsonRPCRequest is the JSON-RPC 2.0 request format.
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is the JSON-RPC 2.0 response format.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callToolResult is the MCP CallToolResult shape.
type callToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// NewMCPClient creates an MCP client connected to a real server via io.Pipe.
func NewMCPClient(engine *core.SearchEngine, sessionID string) *MCPClient {
	serverIn_r, serverIn_w := io.Pipe()   // client writes -> server reads
	serverOut_r, serverOut_w := io.Pipe() // server writes -> client reads

	server := mcp.NewServer(engine, sessionID)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		err := server.RunForIO(ctx, serverIn_r, serverOut_w)
		serverOut_w.Close()
		errCh <- err
	}()

	return &MCPClient{
		server:    server,
		writer:    serverIn_w,
		reader:    bufio.NewReader(serverOut_r),
		cancel:    cancel,
		serverErr: errCh,
	}
}

// Close shuts down the client and server.
func (c *MCPClient) Close() {
	c.cancel()
	if w, ok := c.writer.(*io.PipeWriter); ok {
		w.Close()
	}
}

// call sends a JSON-RPC request and reads the response.
func (c *MCPClient) call(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	c.mu.Lock()
	_, err = c.writer.Write(data)
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// Initialize performs the MCP initialize handshake.
func (c *MCPClient) Initialize(ctx context.Context) error {
	_, err := c.call("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]string{
			"name":    "codex-eval",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification (no response expected, but server consumes it)
	// We need to send it but not read a response since notifications don't get responses.
	// However the server returns nil for notifications, so no line is written.
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	c.mu.Lock()
	c.writer.Write(data)
	c.mu.Unlock()

	return nil
}

// ListTools returns the available MCP tools.
func (c *MCPClient) ListTools(ctx context.Context) ([]string, error) {
	result, err := c.call("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var listResult struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, err
	}

	names := make([]string, len(listResult.Tools))
	for i, t := range listResult.Tools {
		names[i] = t.Name
	}
	return names, nil
}

// CallTool calls an MCP tool and returns the raw JSON result text.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (json.RawMessage, error) {
	result, err := c.call("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}

	var toolResult callToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("unmarshal tool result: %w", err)
	}

	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return nil, fmt.Errorf("tool error: %s", toolResult.Content[0].Text)
		}
		return nil, fmt.Errorf("tool error (no details)")
	}

	if len(toolResult.Content) == 0 {
		return nil, fmt.Errorf("empty tool result")
	}

	return json.RawMessage(toolResult.Content[0].Text), nil
}

// RecallAdd adds a document through the MCP protocol.
func (c *MCPClient) RecallAdd(ctx context.Context, doc TestDocument) (string, error) {
	args := map[string]interface{}{
		"type":    doc.Type,
		"title":   doc.Title,
		"content": doc.Content,
		"tags":    doc.Tags,
		"scope":   doc.Scope,
	}

	result, err := c.CallTool(ctx, "recall_add", args)
	if err != nil {
		return "", err
	}

	var addResult struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result, &addResult); err != nil {
		return "", err
	}

	return addResult.ID, nil
}

// RecallSearch searches through the MCP protocol.
func (c *MCPClient) RecallSearch(ctx context.Context, query string, limit int) ([]SearchResultFromMCP, error) {
	args := map[string]interface{}{
		"query": query,
		"limit": limit,
	}

	result, err := c.CallTool(ctx, "recall_search", args)
	if err != nil {
		return nil, err
	}

	var searchResult struct {
		Results []SearchResultFromMCP `json:"results"`
		Count   int                   `json:"count"`
	}
	if err := json.Unmarshal(result, &searchResult); err != nil {
		return nil, err
	}

	return searchResult.Results, nil
}

// RecallGet retrieves an item through the MCP protocol.
func (c *MCPClient) RecallGet(ctx context.Context, id string) (*ItemFromMCP, error) {
	result, err := c.CallTool(ctx, "recall_get", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	var item ItemFromMCP
	if err := json.Unmarshal(result, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// RecallFeedback records feedback through the MCP protocol.
func (c *MCPClient) RecallFeedback(ctx context.Context, itemID string, useful bool) error {
	_, err := c.CallTool(ctx, "recall_feedback", map[string]interface{}{
		"item_id": itemID,
		"useful":  useful,
	})
	return err
}

// FlightRecorderLog logs through the MCP protocol.
func (c *MCPClient) FlightRecorderLog(ctx context.Context, entryType, content string) error {
	_, err := c.CallTool(ctx, "flight_recorder_log", map[string]interface{}{
		"type":    entryType,
		"content": content,
	})
	return err
}

// FlightRecorderLogWithMetadata logs with metadata through the MCP protocol.
func (c *MCPClient) FlightRecorderLogWithMetadata(ctx context.Context, entryType, content string, metadata map[string]interface{}) error {
	_, err := c.CallTool(ctx, "flight_recorder_log", map[string]interface{}{
		"type":     entryType,
		"content":  content,
		"metadata": metadata,
	})
	return err
}
