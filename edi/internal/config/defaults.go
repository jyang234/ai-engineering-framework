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
			Backend: "v0", // Default to v0 for backward compatibility
		},
		Codex: CodexConfig{
			QdrantAddr: "localhost:6334",
			Collection: "recall",
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
  backend: v0  # "v0" (SQLite FTS) or "codex" (hybrid vector search)

# Codex v1 backend configuration (used when recall.backend = "codex")
# Requires: Qdrant server, VOYAGE_API_KEY, OPENAI_API_KEY env vars
# codex:
#   qdrant_addr: localhost:6334
#   collection: recall
#   models_path: ~/.edi/models
#   metadata_db: ~/.edi/codex.db

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

// WriteDefaultWithBackend writes the default global configuration with a specific backend
func WriteDefaultWithBackend(path string, backend string) error {
	codexSection := `# Codex v1 backend configuration (used when recall.backend = "codex")
# Requires: Qdrant server, VOYAGE_API_KEY, OPENAI_API_KEY env vars
# codex:
#   qdrant_addr: localhost:6334
#   collection: recall
#   models_path: ~/.edi/models
#   metadata_db: ~/.edi/codex.db`

	if backend == "codex" {
		codexSection = `# Codex v1 backend configuration
codex:
  qdrant_addr: localhost:6334
  collection: recall
  models_path: ~/.edi/models
  metadata_db: ~/.edi/codex.db`
	}

	content := `# EDI Global Configuration
version: "1"

# Default agent mode
agent: coder

# RECALL knowledge system
recall:
  enabled: true
  backend: ` + backend + `  # "v0" (SQLite FTS) or "codex" (hybrid vector search)

` + codexSection + `

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
