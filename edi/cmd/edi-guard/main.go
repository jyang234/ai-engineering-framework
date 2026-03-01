// edi-guard is a Claude Code command hook that enforces build tags,
// blocks destructive commands, detects failure loops, and snapshots
// session state before compaction.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/aef/edi/internal/config"
	"github.com/anthropics/aef/edi/internal/guard"
	"gopkg.in/yaml.v3"
)

func main() {
	input := guard.ParseStdin()
	if input == nil {
		os.Exit(0)
	}

	// Skip non-EDI projects
	if _, err := os.Stat(filepath.Join(input.CWD, ".edi")); os.IsNotExist(err) {
		os.Exit(0)
	}

	cfg := loadGuardConfig(input.CWD)
	if !cfg.Guard.Enabled {
		os.Exit(0)
	}

	registry := guard.DefaultRegistry(&cfg.Guard)
	hctx := &guard.HookContext{
		Input:     input,
		Config:    &cfg.Guard,
		SessionID: input.SessionID,
		CWD:       input.CWD,
		Agent:     cfg.Agent,
	}
	ctx := context.Background()

	switch input.HookEventName {
	case "PreToolUse":
		dispatchPreToolUse(ctx, registry, hctx)
	case "PostToolUse":
		dispatchPostToolUse(ctx, registry, hctx)
	case "PostToolUseFailure":
		dispatchPostToolUseFailure(ctx, registry, hctx)
	case "PreCompact":
		dispatchPreCompact(ctx, registry, hctx)
	}
}

// ---------------------------------------------------------------------------
// Dispatch functions
// ---------------------------------------------------------------------------

func dispatchPreToolUse(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
	if hctx.Input.ToolName != "Bash" {
		return
	}
	bash := guard.ParseBashInput(hctx.Input.ToolInput)
	if bash == nil || bash.Command == "" {
		return
	}

	command := bash.Command
	var advisories []string
	modified := false

	for _, policy := range reg.PreToolUsePolicies() {
		result := policy.EvalPreToolUse(ctx, hctx, command)
		if result == nil {
			continue
		}

		// Block short-circuits immediately
		if result.Block {
			fmt.Fprintf(os.Stderr, "edi-guard: %s\n", result.Reason)
			os.Exit(2)
		}

		// Apply command modification (chained — each policy sees previous modifications)
		if result.ModifiedCommand != "" {
			command = result.ModifiedCommand
			modified = true
		}

		// Collect advisories
		if result.Advisory != "" {
			advisories = append(advisories, result.Advisory)
		}
	}

	// Build response if anything changed
	if modified || len(advisories) > 0 {
		resp := guard.BuildPreToolUseResponse(command, advisories, modified)
		data, err := json.Marshal(resp)
		if err == nil {
			fmt.Println(string(data))
		}
	}
}

func dispatchPostToolUse(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
	if hctx.Input.ToolName != "Bash" {
		return
	}
	for _, policy := range reg.PostToolUsePolicies() {
		policy.OnPostToolUse(ctx, hctx)
	}
}

func dispatchPostToolUseFailure(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
	if hctx.Input.ToolName != "Bash" {
		return
	}
	for _, policy := range reg.PostToolUseFailurePolicies() {
		policy.OnPostToolUseFailure(ctx, hctx)
	}
}

func dispatchPreCompact(ctx context.Context, reg *guard.Registry, hctx *guard.HookContext) {
	for _, policy := range reg.PreCompactPolicies() {
		policy.OnPreCompact(ctx, hctx)
	}
}

// ---------------------------------------------------------------------------
// Config loading (stays in main.go — binary-specific YAML merge rules)
// ---------------------------------------------------------------------------

// guardConfigFile is the resolved config after merging all layers.
type guardConfigFile struct {
	Guard config.GuardConfig
	Agent string
}

// guardConfigOverlay is the YAML shape we unmarshal from .edi/config.yaml.
// Uses *bool for Enabled so we can distinguish "not set" from "set to false."
type guardConfigOverlay struct {
	Guard struct {
		Enabled              *bool                `yaml:"enabled"`
		BuildTags            []string             `yaml:"build_tags"`
		DenyPatterns         []config.DenyPattern `yaml:"deny_patterns"`
		FailureLoopThreshold int                  `yaml:"failure_loop_threshold"`
	} `yaml:"guard"`
	Agent string `yaml:"agent"`
}

func loadGuardConfig(cwd string) *guardConfigFile {
	cfg := &guardConfigFile{
		Guard: config.DefaultConfig().Guard,
		Agent: "coder",
	}

	// Load global config into a separate overlay struct so an empty "guard:"
	// key doesn't zero out defaults.
	home, err := os.UserHomeDir()
	if err == nil {
		var global guardConfigOverlay
		if loadYAMLInto(filepath.Join(home, ".edi", "config.yaml"), &global) == nil {
			mergeGuardOverlay(cfg, &global)
		}
	}

	// Project config overrides global. Deny patterns are concatenated,
	// other arrays replace.
	var project guardConfigOverlay
	if loadYAMLInto(filepath.Join(cwd, ".edi", "config.yaml"), &project) == nil {
		mergeGuardOverlay(cfg, &project)
	}

	return cfg
}

// mergeGuardOverlay merges a config overlay into cfg. Only fields explicitly
// set in the overlay are applied. Enabled uses *bool so "not set" (nil) is
// distinguishable from "set to false."
func mergeGuardOverlay(cfg *guardConfigFile, overlay *guardConfigOverlay) {
	if overlay.Agent != "" {
		cfg.Agent = overlay.Agent
	}
	if overlay.Guard.Enabled != nil {
		cfg.Guard.Enabled = *overlay.Guard.Enabled
	}
	if len(overlay.Guard.BuildTags) > 0 {
		cfg.Guard.BuildTags = overlay.Guard.BuildTags
	}
	if overlay.Guard.FailureLoopThreshold > 0 {
		cfg.Guard.FailureLoopThreshold = overlay.Guard.FailureLoopThreshold
	}
	if len(overlay.Guard.DenyPatterns) > 0 {
		cfg.Guard.DenyPatterns = append(cfg.Guard.DenyPatterns, overlay.Guard.DenyPatterns...)
	}
}

func loadYAMLInto(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}
