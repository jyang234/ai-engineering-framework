package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ToolDispatcher routes tool_use calls from the synthetic agent to their implementations.
type ToolDispatcher struct {
	mcpClient *MCPClient // For RECALL tools
	workDir   string     // Task workspace directory
	allowlist []*regexp.Regexp
	blocklist []*regexp.Regexp
}

// ToolResult is the response sent back to the model as a tool_result.
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// NewToolDispatcher creates a tool dispatcher for the synthetic agent.
func NewToolDispatcher(mcpClient *MCPClient, workDir string) *ToolDispatcher {
	return &ToolDispatcher{
		mcpClient: mcpClient,
		workDir:   workDir,
		allowlist: compileBashAllowlist(),
		blocklist: compileBashBlocklist(),
	}
}

// Dispatch routes a tool call to the appropriate handler.
func (d *ToolDispatcher) Dispatch(ctx context.Context, toolName string, input json.RawMessage) *ToolResult {
	switch toolName {
	case "recall_search", "recall_get", "recall_add", "recall_feedback", "flight_recorder_log":
		return d.handleRECALL(ctx, toolName, input)
	case "Read":
		return d.handleRead(input)
	case "Write":
		return d.handleWrite(input)
	case "Edit":
		return d.handleEdit(input)
	case "Glob":
		return d.handleGlob(input)
	case "Grep":
		return d.handleGrep(input)
	case "Bash":
		return d.handleBash(ctx, input)
	default:
		return &ToolResult{Content: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}
	}
}

// ToolDefinitions returns the tool schemas that the synthetic agent can use.
func ToolDefinitions(includeRECALL bool) []map[string]interface{} {
	tools := []map[string]interface{}{
		{
			"name":        "Read",
			"description": "Read a file from the workspace.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]string{"type": "string", "description": "Relative path to the file"},
				},
				"required": []string{"file_path"},
			},
		},
		{
			"name":        "Write",
			"description": "Write content to a file.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]string{"type": "string", "description": "Relative path to the file"},
					"content":   map[string]string{"type": "string", "description": "File content"},
				},
				"required": []string{"file_path", "content"},
			},
		},
		{
			"name":        "Edit",
			"description": "Replace a string in a file.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path":  map[string]string{"type": "string", "description": "Relative path to the file"},
					"old_string": map[string]string{"type": "string", "description": "Text to replace"},
					"new_string": map[string]string{"type": "string", "description": "Replacement text"},
				},
				"required": []string{"file_path", "old_string", "new_string"},
			},
		},
		{
			"name":        "Glob",
			"description": "Find files matching a glob pattern.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]string{"type": "string", "description": "Glob pattern"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "Grep",
			"description": "Search file contents with regex.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]string{"type": "string", "description": "Regex pattern"},
					"path":    map[string]string{"type": "string", "description": "Directory or file to search"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "Bash",
			"description": "Execute a shell command (sandboxed).",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]string{"type": "string", "description": "Command to execute"},
				},
				"required": []string{"command"},
			},
		},
	}

	if includeRECALL {
		tools = append(tools,
			map[string]interface{}{
				"name":        "recall_search",
				"description": "Search organizational knowledge.",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]string{"type": "string", "description": "Search query"},
						"types": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
						"limit": map[string]string{"type": "integer"},
					},
					"required": []string{"query"},
				},
			},
			map[string]interface{}{
				"name":        "recall_get",
				"description": "Get a knowledge item by ID.",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]string{"type": "string"},
					},
					"required": []string{"id"},
				},
			},
			map[string]interface{}{
				"name":        "recall_add",
				"description": "Add a knowledge item.",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":    map[string]string{"type": "string"},
						"title":   map[string]string{"type": "string"},
						"content": map[string]string{"type": "string"},
						"tags":    map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
					},
					"required": []string{"type", "title", "content"},
				},
			},
			map[string]interface{}{
				"name":        "recall_feedback",
				"description": "Record feedback on a knowledge item.",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"item_id": map[string]string{"type": "string"},
						"useful":  map[string]string{"type": "boolean"},
					},
					"required": []string{"item_id", "useful"},
				},
			},
			map[string]interface{}{
				"name":        "flight_recorder_log",
				"description": "Log a decision or observation.",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type":    map[string]string{"type": "string"},
						"content": map[string]string{"type": "string"},
					},
					"required": []string{"type", "content"},
				},
			},
		)
	}

	return tools
}

// RECALL tools — proxy to MCP client
func (d *ToolDispatcher) handleRECALL(ctx context.Context, toolName string, input json.RawMessage) *ToolResult {
	if d.mcpClient == nil {
		return &ToolResult{Content: "RECALL not available in this condition", IsError: true}
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	result, err := d.mcpClient.CallTool(ctx, toolName, args)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("RECALL error: %v", err), IsError: true}
	}

	return &ToolResult{Content: string(result)}
}

// File operations
func (d *ToolDispatcher) handleRead(input json.RawMessage) *ToolResult {
	var args struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	path := d.resolvePath(args.FilePath)
	if !d.isInWorkspace(path) {
		return &ToolResult{Content: "path is outside workspace", IsError: true}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("read error: %v", err), IsError: true}
	}

	content := string(data)
	if len(content) > 100000 {
		content = content[:100000] + "\n... (truncated)"
	}
	return &ToolResult{Content: content}
}

func (d *ToolDispatcher) handleWrite(input json.RawMessage) *ToolResult {
	var args struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	path := d.resolvePath(args.FilePath)
	if !d.isInWorkspace(path) {
		return &ToolResult{Content: "path is outside workspace", IsError: true}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return &ToolResult{Content: fmt.Sprintf("mkdir error: %v", err), IsError: true}
	}

	if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
		return &ToolResult{Content: fmt.Sprintf("write error: %v", err), IsError: true}
	}

	return &ToolResult{Content: fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath)}
}

func (d *ToolDispatcher) handleEdit(input json.RawMessage) *ToolResult {
	var args struct {
		FilePath  string `json:"file_path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	path := d.resolvePath(args.FilePath)
	if !d.isInWorkspace(path) {
		return &ToolResult{Content: "path is outside workspace", IsError: true}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("read error: %v", err), IsError: true}
	}

	content := string(data)
	count := strings.Count(content, args.OldString)
	if count == 0 {
		return &ToolResult{Content: "old_string not found in file", IsError: true}
	}
	if count > 1 {
		return &ToolResult{Content: fmt.Sprintf("old_string found %d times (must be unique)", count), IsError: true}
	}

	newContent := strings.Replace(content, args.OldString, args.NewString, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &ToolResult{Content: fmt.Sprintf("write error: %v", err), IsError: true}
	}

	return &ToolResult{Content: fmt.Sprintf("Edited %s", args.FilePath)}
}

func (d *ToolDispatcher) handleGlob(input json.RawMessage) *ToolResult {
	var args struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	pattern := filepath.Join(d.workDir, args.Pattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("glob error: %v", err), IsError: true}
	}

	// Make paths relative to workspace
	var relative []string
	for _, m := range matches {
		rel, err := filepath.Rel(d.workDir, m)
		if err == nil {
			relative = append(relative, rel)
		}
	}

	return &ToolResult{Content: strings.Join(relative, "\n")}
}

func (d *ToolDispatcher) handleGrep(input json.RawMessage) *ToolResult {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	searchDir := d.workDir
	if args.Path != "" {
		searchDir = d.resolvePath(args.Path)
		if !d.isInWorkspace(searchDir) {
			return &ToolResult{Content: "path is outside workspace", IsError: true}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "grep", "-rn", args.Pattern, searchDir)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	_ = cmd.Run()

	output := stdout.String()
	if len(output) > 10000 {
		output = output[:10000] + "\n... (truncated)"
	}
	return &ToolResult{Content: output}
}

// Bash — sandboxed execution
func (d *ToolDispatcher) handleBash(ctx context.Context, input json.RawMessage) *ToolResult {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return &ToolResult{Content: fmt.Sprintf("invalid arguments: %v", err), IsError: true}
	}

	// Check blocklist first
	for _, pattern := range d.blocklist {
		if pattern.MatchString(args.Command) {
			return &ToolResult{
				Content: fmt.Sprintf("blocked command: %s (matches blocked pattern)", args.Command),
				IsError: true,
			}
		}
	}

	// Check allowlist
	allowed := false
	var timeout time.Duration
	for _, pattern := range d.allowlist {
		if pattern.MatchString(args.Command) {
			allowed = true
			timeout = allowlistTimeout(args.Command)
			break
		}
	}

	if !allowed {
		return &ToolResult{
			Content: fmt.Sprintf("command not in allowlist: %s", args.Command),
			IsError: true,
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
	cmd.Dir = d.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}
	if len(output) > 10000 {
		output = output[:10000] + "\n... (truncated)"
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return &ToolResult{Content: "command timed out after " + timeout.String(), IsError: true}
		}
		return &ToolResult{Content: output + "\nexit code: " + err.Error(), IsError: true}
	}

	return &ToolResult{Content: output}
}

func (d *ToolDispatcher) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(d.workDir, p)
}

func (d *ToolDispatcher) isInWorkspace(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	workAbs, err := filepath.Abs(d.workDir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(abs, workAbs)
}

func compileBashAllowlist() []*regexp.Regexp {
	patterns := []string{
		`^go test\b`,
		`^go build\b`,
		`^go vet\b`,
		`^gofumpt\b`,
		`^golangci-lint\b`,
		`^go mod tidy\b`,
	}
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(p)
	}
	return compiled
}

func compileBashBlocklist() []*regexp.Regexp {
	patterns := []string{
		`rm\s+-rf\s+/`,
		`rm\s+-rf\s+~`,
		`\bcurl\b`,
		`\bwget\b`,
		`\bssh\b`,
		`\bnc\b`,
		`\bsudo\b`,
		`\bgo install\b`,
		`\.\.`, // path traversal
	}
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(p)
	}
	return compiled
}

func allowlistTimeout(cmd string) time.Duration {
	if strings.HasPrefix(cmd, "gofumpt") {
		return 10 * time.Second
	}
	if strings.HasPrefix(cmd, "go mod tidy") {
		return 10 * time.Second
	}
	if strings.HasPrefix(cmd, "go build") || strings.HasPrefix(cmd, "go vet") {
		return 30 * time.Second
	}
	return 60 * time.Second // go test, golangci-lint
}
