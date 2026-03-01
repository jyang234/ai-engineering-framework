package guard

import "github.com/anthropics/aef/edi/internal/config"

// DefaultRegistry creates a registry with all built-in policies in the
// correct evaluation order.
func DefaultRegistry(cfg *config.GuardConfig) *Registry {
	r := NewRegistry()

	// Order matters:
	// 1. Deny-list first (blocking — short-circuits everything else)
	// 2. Build tags (modifies command — must run before advisory reads it)
	// 3. Failure loop (reads command, may add advisory)
	// 4. Compaction snapshot (only fires on PreCompact, ordering is irrelevant)
	r.Register(NewDenyListPolicy(cfg.DenyPatterns))
	r.Register(NewBuildTagPolicy(cfg.BuildTags))
	r.Register(NewFailureLoopPolicy(cfg.FailureLoopThreshold))
	r.Register(NewCompactionSnapshotPolicy(cfg))

	return r
}
