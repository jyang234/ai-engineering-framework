package guard

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/anthropics/aef/edi/internal/config"
)

// DenyListPolicy blocks commands matching configured deny patterns.
type DenyListPolicy struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	re     *regexp.Regexp
	reason string
}

// NewDenyListPolicy compiles the deny patterns and returns a policy.
// Invalid regex patterns are skipped with a warning on stderr.
func NewDenyListPolicy(patterns []config.DenyPattern) *DenyListPolicy {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "edi-guard: invalid deny pattern %q: %v\n", p.Pattern, err)
			continue
		}
		compiled = append(compiled, compiledPattern{re: re, reason: p.Reason})
	}
	return &DenyListPolicy{patterns: compiled}
}

func (d *DenyListPolicy) Name() string { return "deny-list" }

func (d *DenyListPolicy) EvalPreToolUse(_ context.Context, _ *HookContext, command string) *PolicyResult {
	for _, p := range d.patterns {
		if p.re.MatchString(command) {
			return &PolicyResult{Block: true, Reason: p.reason}
		}
	}
	return nil
}
