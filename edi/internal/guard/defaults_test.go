package guard

import (
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

func TestDefaultRegistry_HasAllPolicies(t *testing.T) {
	cfg := config.DefaultConfig().Guard
	reg := DefaultRegistry(&cfg)

	if got := len(reg.PreToolUsePolicies()); got != 3 {
		t.Errorf("expected 3 PreToolUse policies, got %d", got)
	}
	if got := len(reg.PostToolUsePolicies()); got != 1 {
		t.Errorf("expected 1 PostToolUse policy, got %d", got)
	}
	if got := len(reg.PostToolUseFailurePolicies()); got != 1 {
		t.Errorf("expected 1 PostToolUseFailure policy, got %d", got)
	}
	if got := len(reg.PreCompactPolicies()); got != 1 {
		t.Errorf("expected 1 PreCompact policy, got %d", got)
	}
}

func TestDefaultRegistry_OrderPreserved(t *testing.T) {
	cfg := config.DefaultConfig().Guard
	reg := DefaultRegistry(&cfg)

	policies := reg.PreToolUsePolicies()
	if len(policies) != 3 {
		t.Fatalf("expected 3 policies, got %d", len(policies))
	}

	expected := []string{"deny-list", "build-tags", "failure-loop"}
	for i, p := range policies {
		if p.Name() != expected[i] {
			t.Errorf("policy[%d]: expected %q, got %q", i, expected[i], p.Name())
		}
	}
}
