package guard

import (
	"context"
	"fmt"
)

// FailureLoopPolicy detects consecutive Bash failures and advises
// Claude to step back and analyze the root cause.
type FailureLoopPolicy struct {
	threshold int
}

// NewFailureLoopPolicy returns a policy that fires after threshold consecutive failures.
func NewFailureLoopPolicy(threshold int) *FailureLoopPolicy {
	if threshold <= 0 {
		threshold = 5
	}
	return &FailureLoopPolicy{threshold: threshold}
}

func (f *FailureLoopPolicy) Name() string { return "failure-loop" }

func (f *FailureLoopPolicy) EvalPreToolUse(_ context.Context, hctx *HookContext, _ string) *PolicyResult {
	state := readState(hctx.SessionID)
	if state.ConsecutiveFailures < f.threshold || state.Advised {
		return nil
	}
	advisory := fmt.Sprintf(
		"edi-guard: %d consecutive Bash command failures detected. The last failure was: %q → %q. Consider stepping back to analyze the root cause rather than retrying with small variations.",
		state.ConsecutiveFailures, state.LastFailureCommand, state.LastFailureError,
	)
	state.Advised = true
	writeState(hctx.SessionID, state)
	return &PolicyResult{Advisory: advisory}
}

func (f *FailureLoopPolicy) OnPostToolUse(_ context.Context, hctx *HookContext) {
	state := readState(hctx.SessionID)
	if state.ConsecutiveFailures == 0 && !state.Advised {
		return
	}
	writeState(hctx.SessionID, guardState{})
}

func (f *FailureLoopPolicy) OnPostToolUseFailure(_ context.Context, hctx *HookContext) {
	bash := ParseBashInput(hctx.Input.ToolInput)
	state := readState(hctx.SessionID)
	state.ConsecutiveFailures++
	state.Advised = false
	if bash != nil {
		state.LastFailureCommand = bash.Command
	}
	state.LastFailureError = hctx.Input.Error
	writeState(hctx.SessionID, state)
}
