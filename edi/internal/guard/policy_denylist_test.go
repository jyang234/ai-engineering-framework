package guard

import (
	"context"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

var defaultPolicy = NewDenyListPolicy(config.DefaultConfig().Guard.DenyPatterns)

func TestDenyList_ForceMain(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push --force origin main")
	if result == nil || !result.Block {
		t.Fatal("expected deny")
	}
}

func TestDenyList_ShortForce(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push -f origin main")
	if result == nil || !result.Block {
		t.Fatal("expected deny")
	}
}

func TestDenyList_ForceBranchNotMain(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push --force origin feature")
	if result != nil {
		t.Fatalf("expected allow, got block: %q", result.Reason)
	}
}

func TestDenyList_ForceWithLeaseAllowed(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push --force-with-lease origin main")
	if result != nil {
		t.Fatalf("--force-with-lease should be allowed, got block: %q", result.Reason)
	}
}

func TestDenyList_RmRfEdi(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "rm -rf .edi")
	if result == nil || !result.Block {
		t.Fatal("expected deny")
	}
}

func TestDenyList_RmRfEdiSlash(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "rm -rf .edi/")
	if result == nil || !result.Block {
		t.Fatal("expected deny")
	}
}

func TestDenyList_RmSingleFile(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "rm .edi/config.yaml")
	if result != nil {
		t.Fatalf("expected allow for single file delete, got block: %q", result.Reason)
	}
}

func TestDenyList_RmFSingleFile(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "rm -f .edi/config.yaml")
	if result != nil {
		t.Fatalf("expected allow for rm -f single file, got block: %q", result.Reason)
	}
}

func TestDenyList_ResetHard(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git reset --hard")
	if result == nil || !result.Block {
		t.Fatal("expected deny")
	}
}

func TestDenyList_SafeCommand(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "go test -tags fts5 ./...")
	if result != nil {
		t.Fatalf("expected allow, got block: %q", result.Reason)
	}
}

func TestDenyList_RmFrVariant(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "rm -fr .edi")
	if result == nil || !result.Block {
		t.Fatal("expected deny for rm -fr")
	}
}

func TestDenyList_ForcePushReversedArgs(t *testing.T) {
	result := defaultPolicy.EvalPreToolUse(context.Background(), nil, "git push origin main --force")
	if result == nil || !result.Block {
		t.Fatal("expected deny for reversed force push")
	}
}

func TestNewDenyListPolicy_InvalidPattern(t *testing.T) {
	patterns := []config.DenyPattern{
		{Pattern: `[invalid`, Reason: "bad pattern"},
		{Pattern: `valid.*`, Reason: "good pattern"},
	}
	policy := NewDenyListPolicy(patterns)

	// The valid pattern should block
	result := policy.EvalPreToolUse(context.Background(), nil, "valid-match")
	if result == nil || !result.Block {
		t.Fatal("expected valid pattern to block")
	}
	if result.Reason != "good pattern" {
		t.Fatalf("expected reason 'good pattern', got %q", result.Reason)
	}

	// A non-matching command should pass (invalid pattern was skipped)
	result = policy.EvalPreToolUse(context.Background(), nil, "safe command")
	if result != nil {
		t.Fatalf("expected allow, got block: %q", result.Reason)
	}
}
