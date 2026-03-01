package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/aef/edi/internal/config"
)

// ---------------------------------------------------------------------------
// Config loading tests (remain in main_test.go — test main package)
// ---------------------------------------------------------------------------

func TestLoadGuardConfig_EmptyGuardKeyPreservesDefaults(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	// Write config with empty guard: key
	cfg := `guard:
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)

	// Defaults should be preserved
	defaults := config.DefaultConfig().Guard
	if result.Guard.Enabled != defaults.Enabled {
		t.Errorf("expected Enabled=%v, got %v", defaults.Enabled, result.Guard.Enabled)
	}
	if len(result.Guard.BuildTags) != len(defaults.BuildTags) {
		t.Errorf("expected %d build tags, got %d", len(defaults.BuildTags), len(result.Guard.BuildTags))
	}
	if len(result.Guard.DenyPatterns) != len(defaults.DenyPatterns) {
		t.Errorf("expected %d deny patterns, got %d", len(defaults.DenyPatterns), len(result.Guard.DenyPatterns))
	}
}

func TestLoadGuardConfig_ProjectOverridesBuildTags(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	cfg := `guard:
  build_tags: ["fts5", "integration"]
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)

	if len(result.Guard.BuildTags) != 2 {
		t.Fatalf("expected 2 build tags, got %d", len(result.Guard.BuildTags))
	}
	if result.Guard.BuildTags[0] != "fts5" || result.Guard.BuildTags[1] != "integration" {
		t.Fatalf("unexpected tags: %v", result.Guard.BuildTags)
	}
}

func TestLoadGuardConfig_ProjectAppendsDenyPatterns(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	cfg := `guard:
  deny_patterns:
    - pattern: "docker.*rm"
      reason: "no docker rm"
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)

	defaults := config.DefaultConfig().Guard
	expected := len(defaults.DenyPatterns) + 1
	if len(result.Guard.DenyPatterns) != expected {
		t.Fatalf("expected %d deny patterns (default + 1), got %d", expected, len(result.Guard.DenyPatterns))
	}
}

func TestLoadGuardConfig_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)
	// No config.yaml

	result := loadGuardConfig(dir)

	defaults := config.DefaultConfig().Guard
	if result.Guard.Enabled != defaults.Enabled {
		t.Error("defaults should apply when no config file")
	}
	if len(result.Guard.DenyPatterns) != len(defaults.DenyPatterns) {
		t.Errorf("expected default deny patterns, got %d", len(result.Guard.DenyPatterns))
	}
}

func TestLoadGuardConfig_AgentOverride(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	cfg := `agent: architect
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)
	if result.Agent != "architect" {
		t.Fatalf("expected architect, got %q", result.Agent)
	}
}

func TestLoadGuardConfig_ExplicitDisable(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	// Explicitly disabled — no other guard fields needed
	cfg := `guard:
  enabled: false
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)
	if result.Guard.Enabled {
		t.Error("guard should be disabled when explicitly set to false")
	}
	// Defaults should still be preserved for other fields
	defaults := config.DefaultConfig().Guard
	if len(result.Guard.DenyPatterns) != len(defaults.DenyPatterns) {
		t.Errorf("deny patterns should be preserved, got %d", len(result.Guard.DenyPatterns))
	}
}

func TestLoadGuardConfig_ExplicitDisableWithThreshold(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".edi"), 0755)

	cfg := `guard:
  enabled: false
  failure_loop_threshold: 10
`
	os.WriteFile(filepath.Join(dir, ".edi", "config.yaml"), []byte(cfg), 0644)

	result := loadGuardConfig(dir)
	if result.Guard.Enabled {
		t.Error("guard should be disabled")
	}
	if result.Guard.FailureLoopThreshold != 10 {
		t.Errorf("expected threshold 10, got %d", result.Guard.FailureLoopThreshold)
	}
}
