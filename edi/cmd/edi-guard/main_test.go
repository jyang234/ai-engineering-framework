package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

// ---------------------------------------------------------------------------
// Build tag injection tests
// ---------------------------------------------------------------------------

func TestInjectBuildTags_Missing(t *testing.T) {
	modified, result := injectBuildTags("go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "go test -tags fts5 ./..." {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_Present(t *testing.T) {
	modified, _ := injectBuildTags("go test -tags fts5 ./...", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_PresentEquals(t *testing.T) {
	modified, _ := injectBuildTags("go test -tags=fts5 ./...", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_PresentComma(t *testing.T) {
	modified, _ := injectBuildTags(`go test -tags "fts5,integration" ./...`, []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_CompoundCommand(t *testing.T) {
	modified, result := injectBuildTags("cd codex && go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "cd codex && go test -tags fts5 ./..." {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_MakeSkip(t *testing.T) {
	modified, _ := injectBuildTags("make test", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification for make")
	}
}

func TestInjectBuildTags_CompoundWithMake(t *testing.T) {
	modified, result := injectBuildTags("make foo && go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification for go test clause")
	}
	if !strings.Contains(result, "make foo") {
		t.Fatal("make clause should be preserved")
	}
	if !strings.Contains(result, "go test -tags fts5 ./...") {
		t.Fatalf("go test clause not modified: %q", result)
	}
}

func TestInjectBuildTags_MultipleTags(t *testing.T) {
	modified, result := injectBuildTags("go test ./...", []string{"fts5", "integration"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != `go test -tags "fts5,integration" ./...` {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_GoBuild(t *testing.T) {
	modified, result := injectBuildTags("go build -o bin/edi ./cmd/edi", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "go build -tags fts5 -o bin/edi ./cmd/edi" {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_NoTags(t *testing.T) {
	modified, _ := injectBuildTags("go test ./...", nil)
	if modified {
		t.Fatal("expected no modification when no tags configured")
	}
}

// ---------------------------------------------------------------------------
// Deny list tests
// ---------------------------------------------------------------------------

var defaultPatterns = config.DefaultConfig().Guard.DenyPatterns

func TestDenyList_ForceMain(t *testing.T) {
	reason := checkDenyList("git push --force origin main", defaultPatterns)
	if reason == "" {
		t.Fatal("expected deny")
	}
}

func TestDenyList_ShortForce(t *testing.T) {
	reason := checkDenyList("git push -f origin main", defaultPatterns)
	if reason == "" {
		t.Fatal("expected deny")
	}
}

func TestDenyList_ForceBranchNotMain(t *testing.T) {
	reason := checkDenyList("git push --force origin feature", defaultPatterns)
	if reason != "" {
		t.Fatalf("expected allow, got %q", reason)
	}
}

func TestDenyList_RmRfEdi(t *testing.T) {
	reason := checkDenyList("rm -rf .edi", defaultPatterns)
	if reason == "" {
		t.Fatal("expected deny")
	}
}

func TestDenyList_RmRfEdiSlash(t *testing.T) {
	reason := checkDenyList("rm -rf .edi/", defaultPatterns)
	if reason == "" {
		t.Fatal("expected deny")
	}
}

func TestDenyList_RmSingleFile(t *testing.T) {
	reason := checkDenyList("rm .edi/config.yaml", defaultPatterns)
	if reason != "" {
		t.Fatalf("expected allow for single file delete, got %q", reason)
	}
}

func TestDenyList_ResetHard(t *testing.T) {
	reason := checkDenyList("git reset --hard", defaultPatterns)
	if reason == "" {
		t.Fatal("expected deny")
	}
}

func TestDenyList_SafeCommand(t *testing.T) {
	reason := checkDenyList("go test -tags fts5 ./...", defaultPatterns)
	if reason != "" {
		t.Fatalf("expected allow, got %q", reason)
	}
}

// ---------------------------------------------------------------------------
// Failure counter tests
// ---------------------------------------------------------------------------

func TestFailureCounter_IncrementAndReset(t *testing.T) {
	sessionID := "test-session-counter"
	defer os.Remove(stateFilePath(sessionID))

	// Start at zero
	state := readState(sessionID)
	if state.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0, got %d", state.ConsecutiveFailures)
	}

	// Increment
	state.ConsecutiveFailures++
	state.LastFailureCommand = "go test ./..."
	state.LastFailureError = "exit status 1"
	writeState(sessionID, state)

	state = readState(sessionID)
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1, got %d", state.ConsecutiveFailures)
	}

	// Reset
	writeState(sessionID, guardState{})
	state = readState(sessionID)
	if state.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0 after reset, got %d", state.ConsecutiveFailures)
	}
}

func TestFailureCounter_Advisory(t *testing.T) {
	sessionID := "test-session-advisory"
	defer os.Remove(stateFilePath(sessionID))

	// Simulate 5 failures
	state := guardState{
		ConsecutiveFailures: 5,
		Advised:             false,
		LastFailureCommand:  "go test ./...",
		LastFailureError:    "exit status 1",
	}
	writeState(sessionID, state)

	// Read and check advisory would fire
	state = readState(sessionID)
	if state.ConsecutiveFailures < 5 || state.Advised {
		t.Fatal("advisory should be ready to fire")
	}

	// Mark as advised
	state.Advised = true
	writeState(sessionID, state)

	// Check advisory won't fire again
	state = readState(sessionID)
	if !state.Advised {
		t.Fatal("advisory should be marked as fired")
	}
}

// ---------------------------------------------------------------------------
// PreCompact snapshot tests
// ---------------------------------------------------------------------------

func TestPreCompact_WritesFile(t *testing.T) {
	dir := t.TempDir()

	// Create .edi directory structure
	os.MkdirAll(filepath.Join(dir, ".edi", "tasks"), 0755)
	manifest := `tasks:
  - id: TSK-001
    subject: Test task
    status: in_progress
`
	os.WriteFile(filepath.Join(dir, ".edi", "tasks", "active.yaml"), []byte(manifest), 0644)

	input := &hookInput{
		SessionID:     "test-compact",
		CWD:           dir,
		HookEventName: "PreCompact",
		Trigger:       "auto",
	}
	cfg := &guardConfigFile{
		Guard: config.GuardConfig{BuildTags: []string{"fts5"}},
		Agent: "coder",
	}

	handlePreCompact(input, cfg)

	content, err := os.ReadFile(filepath.Join(dir, "memories", "compaction-state.md"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "TSK-001") {
		t.Error("missing task ID")
	}
	if !strings.Contains(s, "fts5") {
		t.Error("missing build tags")
	}
	if !strings.Contains(s, "coder") {
		t.Error("missing agent mode")
	}
	if !strings.Contains(s, "auto") {
		t.Error("missing trigger")
	}
}

func TestPreCompact_MissingTask(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	input := &hookInput{
		SessionID:     "test-compact-no-task",
		CWD:           dir,
		HookEventName: "PreCompact",
		Trigger:       "manual",
	}
	cfg := &guardConfigFile{
		Guard: config.GuardConfig{BuildTags: []string{"fts5"}},
		Agent: "coder",
	}

	handlePreCompact(input, cfg)

	content, err := os.ReadFile(filepath.Join(dir, "memories", "compaction-state.md"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}

	s := string(content)
	if strings.Contains(s, "Task:") {
		t.Error("should not contain task line when no tasks exist")
	}
	if !strings.Contains(s, "no recent failures") {
		t.Error("missing failure status")
	}
}

func TestPreCompact_MultipleInProgressTasks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi", "tasks"), 0755)
	manifest := `tasks:
  - id: TSK-001
    subject: First task
    status: in_progress
  - id: TSK-002
    subject: Second task
    status: in_progress
  - id: TSK-003
    subject: Third task
    status: in_progress
`
	os.WriteFile(filepath.Join(dir, ".edi", "tasks", "active.yaml"), []byte(manifest), 0644)

	input := &hookInput{
		SessionID:     "test-compact-multi",
		CWD:           dir,
		HookEventName: "PreCompact",
		Trigger:       "auto",
	}
	cfg := &guardConfigFile{
		Guard: config.GuardConfig{},
		Agent: "coder",
	}

	handlePreCompact(input, cfg)

	content, err := os.ReadFile(filepath.Join(dir, "memories", "compaction-state.md"))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "TSK-001") {
		t.Error("missing first task")
	}
	if !strings.Contains(s, "TSK-002") {
		t.Error("missing second task")
	}
	if !strings.Contains(s, "(+1 more)") {
		t.Errorf("missing overflow indicator, got:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// PreToolUse response building
// ---------------------------------------------------------------------------

func TestBuildPreToolUseResponse_ModifiedOnly(t *testing.T) {
	resp := buildPreToolUseResponse("go test -tags fts5 ./...", "", true)
	data, _ := json.Marshal(resp)
	s := string(data)
	if !strings.Contains(s, "updatedInput") {
		t.Error("missing updatedInput")
	}
	if strings.Contains(s, "additionalContext") {
		t.Error("should not have additionalContext")
	}
}

func TestBuildPreToolUseResponse_AdvisoryOnly(t *testing.T) {
	resp := buildPreToolUseResponse("", "you're stuck", false)
	data, _ := json.Marshal(resp)
	s := string(data)
	if strings.Contains(s, "updatedInput") {
		t.Error("should not have updatedInput")
	}
	if !strings.Contains(s, "additionalContext") {
		t.Error("missing additionalContext")
	}
}

func TestBuildPreToolUseResponse_Both(t *testing.T) {
	resp := buildPreToolUseResponse("go test -tags fts5 ./...", "you're stuck", true)
	data, _ := json.Marshal(resp)
	s := string(data)
	if !strings.Contains(s, "updatedInput") {
		t.Error("missing updatedInput")
	}
	if !strings.Contains(s, "additionalContext") {
		t.Error("missing additionalContext")
	}
}

// ---------------------------------------------------------------------------
// hasAllTags tests
// ---------------------------------------------------------------------------

func TestHasAllTags_SinglePresent(t *testing.T) {
	if !hasAllTags("go test -tags fts5 ./...", []string{"fts5"}) {
		t.Fatal("should find fts5")
	}
}

func TestHasAllTags_SingleMissing(t *testing.T) {
	if hasAllTags("go test ./...", []string{"fts5"}) {
		t.Fatal("should not find fts5")
	}
}

func TestHasAllTags_MultipleAllPresent(t *testing.T) {
	if !hasAllTags(`go test -tags "fts5,integration" ./...`, []string{"fts5", "integration"}) {
		t.Fatal("should find both")
	}
}

func TestHasAllTags_MultipleOneMissing(t *testing.T) {
	if hasAllTags("go test -tags fts5 ./...", []string{"fts5", "integration"}) {
		t.Fatal("should not find integration")
	}
}
