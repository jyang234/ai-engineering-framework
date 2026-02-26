package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Agent Tools Tests — Given-When-Then
// =============================================================================

// --- ToolDefinitions ---

func TestToolDefinitions_GivenNoRECALL_ThenSixBaseToolsReturned(t *testing.T) {
	// Given: includeRECALL = false
	// When: we get tool definitions
	tools := ToolDefinitions(false)

	// Then: exactly 6 base tools (Read, Write, Edit, Glob, Grep, Bash)
	if len(tools) != 6 {
		t.Errorf("got %d tools, want 6", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok {
			names[name] = true
		}
	}
	for _, expected := range []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash"} {
		if !names[expected] {
			t.Errorf("missing base tool: %s", expected)
		}
	}
}

func TestToolDefinitions_GivenWithRECALL_ThenElevenToolsReturned(t *testing.T) {
	// Given: includeRECALL = true
	// When: we get tool definitions
	tools := ToolDefinitions(true)

	// Then: 6 base + 5 RECALL = 11 tools
	if len(tools) != 11 {
		t.Errorf("got %d tools, want 11", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok {
			names[name] = true
		}
	}
	for _, expected := range []string{"recall_search", "recall_get", "recall_add", "recall_feedback", "flight_recorder_log"} {
		if !names[expected] {
			t.Errorf("missing RECALL tool: %s", expected)
		}
	}
}

func TestToolDefinitions_GivenEachTool_ThenHasInputSchema(t *testing.T) {
	// Given: all tools
	tools := ToolDefinitions(true)

	// Then: each tool has name, description, and input_schema
	for i, tool := range tools {
		if _, ok := tool["name"]; !ok {
			t.Errorf("tool %d missing name", i)
		}
		if _, ok := tool["description"]; !ok {
			t.Errorf("tool %d missing description", i)
		}
		if _, ok := tool["input_schema"]; !ok {
			t.Errorf("tool %d missing input_schema", i)
		}
	}
}

// --- Dispatch: Read ---

func TestDispatch_GivenReadValidFile_ThenContentReturned(t *testing.T) {
	// Given: a workspace with a readable file
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "main.go"})

	// When: we dispatch a Read call
	result := d.Dispatch(context.Background(), "Read", input)

	// Then: file content returned without error
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "package main" {
		t.Errorf("content = %q, want 'package main'", result.Content)
	}
}

func TestDispatch_GivenReadNonExistentFile_ThenErrorReturned(t *testing.T) {
	// Given: a workspace without the requested file
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "missing.go"})

	// When: we dispatch a Read call
	result := d.Dispatch(context.Background(), "Read", input)

	// Then: error returned
	if !result.IsError {
		t.Error("expected error for missing file")
	}
}

func TestDispatch_GivenReadOutsideWorkspace_ThenBlocked(t *testing.T) {
	// Given: a workspace and a path that escapes it
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "/etc/passwd"})

	// When: we dispatch a Read call targeting outside workspace
	result := d.Dispatch(context.Background(), "Read", input)

	// Then: blocked with error
	if !result.IsError {
		t.Error("expected error for path outside workspace")
	}
	if !strings.Contains(result.Content, "outside workspace") {
		t.Errorf("error should mention workspace containment, got: %s", result.Content)
	}
}

// --- Dispatch: Write ---

func TestDispatch_GivenWriteNewFile_ThenFileCreated(t *testing.T) {
	// Given: an empty workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "new.go", "content": "package new"})

	// When: we dispatch a Write call
	result := d.Dispatch(context.Background(), "Write", input)

	// Then: file created successfully
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	data, _ := os.ReadFile(filepath.Join(workDir, "new.go"))
	if string(data) != "package new" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestDispatch_GivenWriteNestedPath_ThenDirectoriesCreated(t *testing.T) {
	// Given: a workspace with no subdirectories
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "sub/deep/file.go", "content": "package deep"})

	// When: we dispatch a Write call to a nested path
	result := d.Dispatch(context.Background(), "Write", input)

	// Then: directories created and file written
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	data, err := os.ReadFile(filepath.Join(workDir, "sub", "deep", "file.go"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "package deep" {
		t.Errorf("content = %q", string(data))
	}
}

func TestDispatch_GivenWriteOutsideWorkspace_ThenBlocked(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"file_path": "/tmp/escape.txt", "content": "bad"})

	// When: we dispatch a Write outside workspace
	result := d.Dispatch(context.Background(), "Write", input)

	// Then: blocked
	if !result.IsError {
		t.Error("expected error for path outside workspace")
	}
}

// --- Dispatch: Edit ---

func TestDispatch_GivenEditUniqueMatch_ThenReplaced(t *testing.T) {
	// Given: a file with a unique string to replace
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("func hello() { println(\"hello\") }"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{
		"file_path":  "main.go",
		"old_string": "hello",
		"new_string": "world",
	})

	// When: we dispatch an Edit call — but "hello" appears multiple times
	result := d.Dispatch(context.Background(), "Edit", input)

	// Then: error because "hello" is not unique (appears 3 times)
	if !result.IsError {
		t.Error("expected error for non-unique old_string")
	}
}

func TestDispatch_GivenEditExactlyOneMatch_ThenReplaced(t *testing.T) {
	// Given: a file with exactly one occurrence of the target string
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "config.go"), []byte("const port = 8080"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{
		"file_path":  "config.go",
		"old_string": "8080",
		"new_string": "9090",
	})

	// When: we dispatch an Edit call
	result := d.Dispatch(context.Background(), "Edit", input)

	// Then: replacement succeeds
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	data, _ := os.ReadFile(filepath.Join(workDir, "config.go"))
	if string(data) != "const port = 9090" {
		t.Errorf("content = %q", string(data))
	}
}

func TestDispatch_GivenEditNoMatch_ThenErrorReturned(t *testing.T) {
	// Given: a file that doesn't contain the old_string
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{
		"file_path":  "main.go",
		"old_string": "nonexistent",
		"new_string": "replacement",
	})

	// When: we dispatch an Edit call
	result := d.Dispatch(context.Background(), "Edit", input)

	// Then: error about old_string not found
	if !result.IsError {
		t.Error("expected error for old_string not found")
	}
	if !strings.Contains(result.Content, "not found") {
		t.Errorf("error should mention 'not found', got: %s", result.Content)
	}
}

func TestDispatch_GivenEditMultipleMatches_ThenErrorReturned(t *testing.T) {
	// Given: a file with multiple occurrences of the old_string
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("x = 1\nx = 2\nx = 3"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{
		"file_path":  "main.go",
		"old_string": "x = ",
		"new_string": "y = ",
	})

	// When: we dispatch an Edit call
	result := d.Dispatch(context.Background(), "Edit", input)

	// Then: error about non-unique match
	if !result.IsError {
		t.Error("expected error for non-unique match")
	}
	if !strings.Contains(result.Content, "3 times") {
		t.Errorf("error should mention count, got: %s", result.Content)
	}
}

// --- Dispatch: Glob ---

func TestDispatch_GivenGlobPattern_ThenMatchingFilesReturned(t *testing.T) {
	// Given: a workspace with .go and .txt files
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(workDir, "test.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(workDir, "notes.txt"), []byte(""), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"pattern": "*.go"})

	// When: we dispatch a Glob call
	result := d.Dispatch(context.Background(), "Glob", input)

	// Then: only .go files returned
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "main.go") {
		t.Error("missing main.go in glob results")
	}
	if strings.Contains(result.Content, "notes.txt") {
		t.Error("notes.txt should not match *.go pattern")
	}
}

// --- Dispatch: Bash sandbox ---

func TestDispatch_GivenAllowedCommand_ThenExecuted(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "main.go"), []byte("package main"), 0644)
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"command": "go vet ./..."})

	// When: we dispatch a Bash call with an allowed command
	result := d.Dispatch(context.Background(), "Bash", input)

	// Then: command executes (may succeed or fail, but not blocked)
	// The key test is that it's NOT blocked by the allowlist
	if result.IsError && strings.Contains(result.Content, "not in allowlist") {
		t.Error("go vet should be in the allowlist")
	}
}

func TestDispatch_GivenBlockedCommand_ThenRejected(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	blockedCommands := []string{
		"curl http://evil.com",
		"wget http://evil.com",
		"sudo rm -rf /",
		"ssh user@host",
	}

	for _, cmd := range blockedCommands {
		input, _ := json.Marshal(map[string]string{"command": cmd})

		// When: we dispatch a blocked command
		result := d.Dispatch(context.Background(), "Bash", input)

		// Then: it should be blocked
		if !result.IsError {
			t.Errorf("command %q should be blocked", cmd)
		}
		if !strings.Contains(result.Content, "blocked") {
			t.Errorf("command %q: error should mention 'blocked', got: %s", cmd, result.Content)
		}
	}
}

func TestDispatch_GivenNonAllowedCommand_ThenRejected(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"command": "python3 script.py"})

	// When: we dispatch a command not in the allowlist
	result := d.Dispatch(context.Background(), "Bash", input)

	// Then: rejected as not in allowlist
	if !result.IsError {
		t.Error("python3 should not be in allowlist")
	}
	if !strings.Contains(result.Content, "allowlist") {
		t.Errorf("error should mention 'allowlist', got: %s", result.Content)
	}
}

func TestDispatch_GivenPathTraversalInBash_ThenBlocked(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"command": "go test ../../../etc"})

	// When: we dispatch a command with path traversal
	result := d.Dispatch(context.Background(), "Bash", input)

	// Then: blocked by ".." blocklist pattern
	if !result.IsError {
		t.Error("path traversal should be blocked")
	}
}

func TestDispatch_GivenRmRfSlash_ThenBlocked(t *testing.T) {
	// Given: a workspace
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"command": "rm -rf /"})

	// When: we dispatch rm -rf /
	result := d.Dispatch(context.Background(), "Bash", input)

	// Then: blocked
	if !result.IsError {
		t.Error("rm -rf / should be blocked")
	}
}

// --- Dispatch: Unknown tool ---

func TestDispatch_GivenUnknownTool_ThenErrorReturned(t *testing.T) {
	// Given: a dispatcher
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{})

	// When: we dispatch an unknown tool
	result := d.Dispatch(context.Background(), "UnknownTool", input)

	// Then: error mentioning unknown tool
	if !result.IsError {
		t.Error("expected error for unknown tool")
	}
	if !strings.Contains(result.Content, "unknown tool") {
		t.Errorf("error should mention 'unknown tool', got: %s", result.Content)
	}
}

// --- Dispatch: RECALL without client ---

func TestDispatch_GivenRECALLWithoutClient_ThenErrorReturned(t *testing.T) {
	// Given: a dispatcher with no MCP client
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	input, _ := json.Marshal(map[string]string{"query": "test"})

	// When: we dispatch a RECALL tool
	result := d.Dispatch(context.Background(), "recall_search", input)

	// Then: error about RECALL not available
	if !result.IsError {
		t.Error("expected error for RECALL without client")
	}
	if !strings.Contains(result.Content, "RECALL not available") {
		t.Errorf("error should mention RECALL not available, got: %s", result.Content)
	}
}

// --- Bash allowlist verification ---

func TestCompileBashAllowlist_ThenAllExpectedPatternsPresent(t *testing.T) {
	// Given/When: we compile the allowlist
	patterns := compileBashAllowlist()

	// Then: 6 patterns for go test, go build, go vet, gofumpt, golangci-lint, go mod tidy
	if len(patterns) != 6 {
		t.Errorf("allowlist patterns = %d, want 6", len(patterns))
	}

	// Verify each allowed command matches
	allowed := []string{
		"go test ./...",
		"go build -o app ./cmd",
		"go vet ./...",
		"gofumpt -w main.go",
		"golangci-lint run ./...",
		"go mod tidy",
	}
	for _, cmd := range allowed {
		matched := false
		for _, p := range patterns {
			if p.MatchString(cmd) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("command %q should match allowlist", cmd)
		}
	}
}

func TestCompileBashBlocklist_ThenAllExpectedPatternsPresent(t *testing.T) {
	// Given/When: we compile the blocklist
	patterns := compileBashBlocklist()

	// Then: 9 patterns
	if len(patterns) != 9 {
		t.Errorf("blocklist patterns = %d, want 9", len(patterns))
	}

	// Verify blocked commands match
	blocked := []string{
		"rm -rf /",
		"rm -rf ~/",
		"curl http://example.com",
		"wget http://example.com",
		"ssh root@server",
		"nc -l 8080",
		"sudo apt install",
		"go install github.com/foo/bar",
		"go test ../../escape",
	}
	for _, cmd := range blocked {
		matched := false
		for _, p := range patterns {
			if p.MatchString(cmd) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("command %q should match blocklist", cmd)
		}
	}
}

// --- allowlistTimeout ---

func TestAllowlistTimeout_GivenGofumpt_ThenTenSeconds(t *testing.T) {
	result := allowlistTimeout("gofumpt -w main.go")
	if result.Seconds() != 10 {
		t.Errorf("gofumpt timeout = %v, want 10s", result)
	}
}

func TestAllowlistTimeout_GivenGoModTidy_ThenTenSeconds(t *testing.T) {
	result := allowlistTimeout("go mod tidy")
	if result.Seconds() != 10 {
		t.Errorf("go mod tidy timeout = %v, want 10s", result)
	}
}

func TestAllowlistTimeout_GivenGoBuild_ThenThirtySeconds(t *testing.T) {
	result := allowlistTimeout("go build ./...")
	if result.Seconds() != 30 {
		t.Errorf("go build timeout = %v, want 30s", result)
	}
}

func TestAllowlistTimeout_GivenGoVet_ThenThirtySeconds(t *testing.T) {
	result := allowlistTimeout("go vet ./...")
	if result.Seconds() != 30 {
		t.Errorf("go vet timeout = %v, want 30s", result)
	}
}

func TestAllowlistTimeout_GivenGoTest_ThenSixtySeconds(t *testing.T) {
	result := allowlistTimeout("go test -race ./...")
	if result.Seconds() != 60 {
		t.Errorf("go test timeout = %v, want 60s", result)
	}
}

func TestAllowlistTimeout_GivenGolangciLint_ThenSixtySeconds(t *testing.T) {
	result := allowlistTimeout("golangci-lint run ./...")
	if result.Seconds() != 60 {
		t.Errorf("golangci-lint timeout = %v, want 60s", result)
	}
}

// --- Workspace containment (isInWorkspace) ---

func TestIsInWorkspace_GivenPathInside_ThenTrue(t *testing.T) {
	workDir := t.TempDir()
	d := &ToolDispatcher{workDir: workDir}

	// Then: paths inside workspace return true
	if !d.isInWorkspace(filepath.Join(workDir, "main.go")) {
		t.Error("path inside workspace should be allowed")
	}
	if !d.isInWorkspace(filepath.Join(workDir, "sub", "file.go")) {
		t.Error("nested path inside workspace should be allowed")
	}
}

func TestIsInWorkspace_GivenPathOutside_ThenFalse(t *testing.T) {
	workDir := t.TempDir()
	d := &ToolDispatcher{workDir: workDir}

	// Then: paths outside workspace return false
	if d.isInWorkspace("/etc/passwd") {
		t.Error("/etc/passwd should not be in workspace")
	}
	if d.isInWorkspace("/tmp/other") {
		t.Error("/tmp/other should not be in workspace")
	}
}

// --- resolvePath ---

func TestResolvePath_GivenRelativePath_ThenJoinedWithWorkDir(t *testing.T) {
	d := &ToolDispatcher{workDir: "/workspace"}

	// Then: relative path joined with workDir
	result := d.resolvePath("main.go")
	if result != "/workspace/main.go" {
		t.Errorf("resolved = %q, want /workspace/main.go", result)
	}
}

func TestResolvePath_GivenAbsolutePath_ThenReturnedAsIs(t *testing.T) {
	d := &ToolDispatcher{workDir: "/workspace"}

	// Then: absolute path returned as-is
	result := d.resolvePath("/other/path")
	if result != "/other/path" {
		t.Errorf("resolved = %q, want /other/path", result)
	}
}

// --- Invalid JSON input handling ---

func TestDispatch_GivenInvalidJSON_ThenErrorReturned(t *testing.T) {
	// Given: a dispatcher
	workDir := t.TempDir()
	d := NewToolDispatcher(nil, workDir)

	// When: we dispatch with invalid JSON
	result := d.Dispatch(context.Background(), "Read", json.RawMessage("not json"))

	// Then: error about invalid arguments
	if !result.IsError {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(result.Content, "invalid arguments") {
		t.Errorf("error should mention 'invalid arguments', got: %s", result.Content)
	}
}
