package guard

import (
	"context"

	"github.com/anthropics/aef/edi/internal/config"
)

// PolicyResult is returned by policy evaluation methods.
type PolicyResult struct {
	// Block stops the tool call. When true, Reason is written to stderr
	// and the hook exits with code 2. Only meaningful for PreToolUse.
	Block  bool
	Reason string

	// ModifiedCommand replaces tool_input.command when non-empty.
	// Multiple policies may modify the command; they are applied in order.
	// Only meaningful for PreToolUse.
	ModifiedCommand string

	// Advisory is injected as additionalContext.
	// Multiple advisories from different policies are joined with newlines.
	// Only meaningful for PreToolUse.
	Advisory string
}

// HookContext carries the parsed hook input and session metadata.
// Agent is session-level context (not part of GuardConfig) needed by
// policies like CompactionSnapshotPolicy that write session state.
type HookContext struct {
	Input     *HookInput
	Config    *config.GuardConfig
	SessionID string
	CWD       string
	Agent     string
}

// PreToolUsePolicy is implemented by policies that evaluate before tool execution.
type PreToolUsePolicy interface {
	// Name returns a short identifier for logging/debugging (e.g., "deny-list").
	Name() string

	// EvalPreToolUse evaluates the policy for a PreToolUse event.
	// command is the current command (may have been modified by an earlier policy).
	// Returns a result. A nil result means "no opinion."
	EvalPreToolUse(ctx context.Context, hctx *HookContext, command string) *PolicyResult
}

// PostToolUsePolicy is implemented by policies that react to successful tool completion.
type PostToolUsePolicy interface {
	Name() string
	OnPostToolUse(ctx context.Context, hctx *HookContext)
}

// PostToolUseFailurePolicy is implemented by policies that react to tool failures.
type PostToolUseFailurePolicy interface {
	Name() string
	OnPostToolUseFailure(ctx context.Context, hctx *HookContext)
}

// PreCompactPolicy is implemented by policies that act before context compaction.
type PreCompactPolicy interface {
	Name() string
	OnPreCompact(ctx context.Context, hctx *HookContext)
}
