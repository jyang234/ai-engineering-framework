package testutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// MCPTestClient wraps stdin/stdout communication with an MCP server.
type MCPTestClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  io.ReadCloser
	nextID  int64
	mu      sync.Mutex
	timeout time.Duration
}

// MCPRequest represents a JSON-RPC 2.0 request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ListToolsResult represents the result of tools/list
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolResult represents the result of tools/call
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents content returned from a tool call
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// InitializeResult represents the result of initialize
type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

// ServerInfo represents MCP server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewMCPTestClient creates a new MCP test client that spawns the given server binary.
func NewMCPTestClient(serverBinary string, args ...string) (*MCPTestClient, error) {
	cmd := exec.Command(serverBinary, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	return &MCPTestClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		stderr:  stderr,
		timeout: 5 * time.Second,
	}, nil
}

// SetTimeout sets the timeout for requests.
func (c *MCPTestClient) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Initialize performs the MCP initialize handshake.
func (c *MCPTestClient) Initialize() (*InitializeResult, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	resp, err := c.sendRequest("initialize", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Send initialized notification (no response expected)
	if err := c.sendNotification("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return &result, nil
}

// ListTools retrieves the list of available tools from the server.
func (c *MCPTestClient) ListTools() ([]Tool, error) {
	resp, err := c.sendRequest("tools/list", nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/list result: %w", err)
	}

	return result.Tools, nil
}

// CallTool calls a tool with the given arguments and returns the result.
func (c *MCPTestClient) CallTool(name string, args map[string]interface{}) (*CallToolResult, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	resp, err := c.sendRequest("tools/call", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools/call result: %w", err)
	}

	return &result, nil
}

// CallToolRaw calls a tool and returns the raw text content.
func (c *MCPTestClient) CallToolRaw(name string, args map[string]interface{}) (string, error) {
	result, err := c.CallTool(name, args)
	if err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", nil
	}

	return result.Content[0].Text, nil
}

// SendRaw sends a raw JSON message and returns the response.
func (c *MCPTestClient) SendRaw(message []byte) (*MCPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Send message with newline
	if _, err := c.stdin.Write(append(message, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	return c.readResponse()
}

// Close shuts down the MCP server and cleans up.
func (c *MCPTestClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stdin.Close()

	// Give the process time to exit gracefully
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		c.cmd.Process.Kill()
		return <-done
	}
}

// GetStderr returns any stderr output from the server.
func (c *MCPTestClient) GetStderr() (string, error) {
	data, err := io.ReadAll(c.stderr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *MCPTestClient) sendRequest(method string, params interface{}) (*MCPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.nextID, 1)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request with newline
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	return c.readResponse()
}

func (c *MCPTestClient) sendNotification(method string, params interface{}) error {
	req := MCPRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Send notification with newline
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

func (c *MCPTestClient) readResponse() (*MCPResponse, error) {
	// Set read deadline
	type readResult struct {
		line []byte
		err  error
	}

	resultCh := make(chan readResult, 1)
	go func() {
		line, err := c.stdout.ReadBytes('\n')
		resultCh <- readResult{line, err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			return nil, fmt.Errorf("failed to read response: %w", result.err)
		}

		var resp MCPResponse
		if err := json.Unmarshal(result.line, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w (raw: %s)", err, string(result.line))
		}

		return &resp, nil

	case <-time.After(c.timeout):
		return nil, fmt.Errorf("timeout waiting for response after %v", c.timeout)
	}
}
