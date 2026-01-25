package config

import (
	"os"
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Version: "1",
		Agent:   "coder",
		Recall: RecallConfig{
			Enabled: true,
		},
		Briefing: BriefingConfig{
			IncludeHistory: true,
			HistoryEntries: 3,
			IncludeTasks:   true,
			IncludeProfile: true,
		},
		Capture: CaptureConfig{
			FrictionBudget: 3,
		},
		Tasks: TasksConfig{
			LazyLoading:        true,
			CaptureOnComplete:  true,
			PropagateDecisions: true,
		},
	}
}

// WriteDefault writes the default global configuration to a file
func WriteDefault(path string) error {
	content := `# EDI Global Configuration
version: "1"

# Default agent mode
agent: coder

# RECALL knowledge system
recall:
  enabled: true

# Session briefing
briefing:
  include_history: true
  history_entries: 3
  include_tasks: true
  include_profile: true

# Capture workflow
capture:
  # Maximum captures to prompt per session (0 = unlimited)
  friction_budget: 3

# Task integration
tasks:
  lazy_loading: true
  capture_on_completion: true
  propagate_decisions: true
`
	return os.WriteFile(path, []byte(content), 0644)
}

// WriteProjectDefault writes the default project configuration to a file
func WriteProjectDefault(path string) error {
	content := `# EDI Project Configuration
version: "1"

# Project information
project:
  name: ""  # Auto-detected from directory name if empty

# Override global settings as needed
# agent: coder
# recall:
#   enabled: true
# briefing:
#   include_history: true
#   history_entries: 3
`
	return os.WriteFile(path, []byte(content), 0644)
}
