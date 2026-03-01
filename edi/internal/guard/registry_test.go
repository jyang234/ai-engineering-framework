package guard

import (
	"context"
	"testing"
)

// stubPolicy is a test helper that implements all four interfaces.
type stubPolicy struct {
	name     string
	evalFunc func(ctx context.Context, hctx *HookContext, command string) *PolicyResult
	called   bool
}

func (s *stubPolicy) Name() string { return s.name }

func (s *stubPolicy) EvalPreToolUse(ctx context.Context, hctx *HookContext, command string) *PolicyResult {
	s.called = true
	if s.evalFunc != nil {
		return s.evalFunc(ctx, hctx, command)
	}
	return nil
}

func (s *stubPolicy) OnPostToolUse(_ context.Context, _ *HookContext)        { s.called = true }
func (s *stubPolicy) OnPostToolUseFailure(_ context.Context, _ *HookContext) { s.called = true }
func (s *stubPolicy) OnPreCompact(_ context.Context, _ *HookContext)         { s.called = true }

func TestRegistry_RegisterInvalidPanics(t *testing.T) {
	reg := NewRegistry()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when registering non-policy type")
		}
	}()
	reg.Register(42) // int implements no policy interface
}

func TestRegistry_BlockShortCircuits(t *testing.T) {
	blocker := &stubPolicy{
		name: "blocker",
		evalFunc: func(_ context.Context, _ *HookContext, _ string) *PolicyResult {
			return &PolicyResult{Block: true, Reason: "blocked"}
		},
	}
	after := &stubPolicy{name: "after"}

	reg := NewRegistry()
	reg.Register(blocker)
	reg.Register(after)

	policies := reg.PreToolUsePolicies()
	command := "test"

	for _, p := range policies {
		result := p.EvalPreToolUse(context.Background(), nil, command)
		if result != nil && result.Block {
			break
		}
	}

	if !blocker.called {
		t.Error("blocker should have been called")
	}
	if after.called {
		t.Error("policy after blocker should not have been called")
	}
}

func TestRegistry_CommandChaining(t *testing.T) {
	policyA := &stubPolicy{
		name: "modifier-a",
		evalFunc: func(_ context.Context, _ *HookContext, command string) *PolicyResult {
			return &PolicyResult{ModifiedCommand: command + " -a"}
		},
	}
	policyB := &stubPolicy{
		name: "modifier-b",
		evalFunc: func(_ context.Context, _ *HookContext, command string) *PolicyResult {
			// Should see policyA's modification
			if command != "original -a" {
				return &PolicyResult{Advisory: "unexpected: " + command}
			}
			return &PolicyResult{ModifiedCommand: command + " -b"}
		},
	}

	reg := NewRegistry()
	reg.Register(policyA)
	reg.Register(policyB)

	command := "original"
	for _, p := range reg.PreToolUsePolicies() {
		result := p.EvalPreToolUse(context.Background(), nil, command)
		if result != nil && result.ModifiedCommand != "" {
			command = result.ModifiedCommand
		}
	}

	if command != "original -a -b" {
		t.Fatalf("expected chained command 'original -a -b', got %q", command)
	}
}

func TestRegistry_AdvisoriesMerge(t *testing.T) {
	policyA := &stubPolicy{
		name: "advisor-a",
		evalFunc: func(_ context.Context, _ *HookContext, _ string) *PolicyResult {
			return &PolicyResult{Advisory: "warning A"}
		},
	}
	policyB := &stubPolicy{
		name: "advisor-b",
		evalFunc: func(_ context.Context, _ *HookContext, _ string) *PolicyResult {
			return &PolicyResult{Advisory: "warning B"}
		},
	}

	reg := NewRegistry()
	reg.Register(policyA)
	reg.Register(policyB)

	var advisories []string
	for _, p := range reg.PreToolUsePolicies() {
		result := p.EvalPreToolUse(context.Background(), nil, "cmd")
		if result != nil && result.Advisory != "" {
			advisories = append(advisories, result.Advisory)
		}
	}

	if len(advisories) != 2 {
		t.Fatalf("expected 2 advisories, got %d", len(advisories))
	}
	if advisories[0] != "warning A" || advisories[1] != "warning B" {
		t.Fatalf("unexpected advisories: %v", advisories)
	}
}

func TestRegistry_EmptyResultSkipped(t *testing.T) {
	noop := &stubPolicy{name: "noop"}
	advisor := &stubPolicy{
		name: "advisor",
		evalFunc: func(_ context.Context, _ *HookContext, _ string) *PolicyResult {
			return &PolicyResult{Advisory: "heads up"}
		},
	}

	reg := NewRegistry()
	reg.Register(noop)
	reg.Register(advisor)

	var advisories []string
	for _, p := range reg.PreToolUsePolicies() {
		result := p.EvalPreToolUse(context.Background(), nil, "cmd")
		if result == nil {
			continue
		}
		if result.Advisory != "" {
			advisories = append(advisories, result.Advisory)
		}
	}

	if len(advisories) != 1 || advisories[0] != "heads up" {
		t.Fatalf("expected single advisory 'heads up', got %v", advisories)
	}
}
