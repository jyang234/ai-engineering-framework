package guard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

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

	cfg := &config.GuardConfig{BuildTags: []string{"fts5"}}
	policy := NewCompactionSnapshotPolicy(cfg)
	hctx := &HookContext{
		Input: &HookInput{
			SessionID:     "test-compact",
			CWD:           dir,
			HookEventName: "PreCompact",
			Trigger:       "auto",
		},
		Config:    cfg,
		SessionID: "test-compact",
		CWD:       dir,
		Agent:     "coder",
	}

	policy.OnPreCompact(context.Background(), hctx)

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

	cfg := &config.GuardConfig{BuildTags: []string{"fts5"}}
	policy := NewCompactionSnapshotPolicy(cfg)
	hctx := &HookContext{
		Input: &HookInput{
			SessionID:     "test-compact-no-task",
			CWD:           dir,
			HookEventName: "PreCompact",
			Trigger:       "manual",
		},
		Config:    cfg,
		SessionID: "test-compact-no-task",
		CWD:       dir,
		Agent:     "coder",
	}

	policy.OnPreCompact(context.Background(), hctx)

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

	cfg := &config.GuardConfig{}
	policy := NewCompactionSnapshotPolicy(cfg)
	hctx := &HookContext{
		Input: &HookInput{
			SessionID:     "test-compact-multi",
			CWD:           dir,
			HookEventName: "PreCompact",
			Trigger:       "auto",
		},
		Config:    cfg,
		SessionID: "test-compact-multi",
		CWD:       dir,
		Agent:     "coder",
	}

	policy.OnPreCompact(context.Background(), hctx)

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
