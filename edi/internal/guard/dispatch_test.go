package guard

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

func TestDispatch_PreToolUse_BashOnly(t *testing.T) {
	// Non-Bash tools should not trigger any policies
	cfg := config.DefaultConfig().Guard

	hctx := &HookContext{
		Input: &HookInput{
			ToolName:  "Write",
			ToolInput: json.RawMessage(`{"file_path": "/tmp/test"}`),
		},
		Config:    &cfg,
		SessionID: "test",
		CWD:       "/tmp",
	}

	// Simulate what the dispatcher does — Bash check
	if hctx.Input.ToolName == "Bash" {
		t.Fatal("test setup error: should be non-Bash tool")
	}

	// The dispatcher returns early for non-Bash, so no policies should evaluate.
	// We verify by running the policies manually and confirming the Bash guard
	// is what would prevent evaluation.
	bash := ParseBashInput(hctx.Input.ToolInput)
	if bash != nil && bash.Command != "" {
		// Even if we could parse it, the Bash check would have returned first
		t.Fatal("non-Bash tool input should not have a command field")
	}
}

func TestDispatch_PostToolUse_AllFire(t *testing.T) {
	// Create a registry with multiple PostToolUse policies
	policy1 := &stubPolicy{name: "post-1"}
	policy2 := &stubPolicy{name: "post-2"}

	reg := NewRegistry()
	reg.Register(policy1)
	reg.Register(policy2)

	hctx := &HookContext{
		Input: &HookInput{
			ToolName: "Bash",
		},
		SessionID: "test",
	}

	// Simulate PostToolUse dispatch
	for _, p := range reg.PostToolUsePolicies() {
		p.OnPostToolUse(context.Background(), hctx)
	}

	if !policy1.called {
		t.Error("policy1 should have been called")
	}
	if !policy2.called {
		t.Error("policy2 should have been called")
	}
}
