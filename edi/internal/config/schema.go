package config

// Config represents the full EDI configuration
type Config struct {
	Version string `yaml:"version" mapstructure:"version"`

	// Current agent mode
	Agent string `yaml:"agent" mapstructure:"agent"`

	// RECALL configuration
	Recall RecallConfig `yaml:"recall" mapstructure:"recall"`

	// Briefing configuration
	Briefing BriefingConfig `yaml:"briefing" mapstructure:"briefing"`

	// Capture configuration
	Capture CaptureConfig `yaml:"capture" mapstructure:"capture"`

	// Tasks configuration
	Tasks TasksConfig `yaml:"tasks" mapstructure:"tasks"`

	// Project-specific settings (only in project config)
	Project ProjectConfig `yaml:"project" mapstructure:"project"`
}

// RecallConfig configures the RECALL knowledge system
type RecallConfig struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

// BriefingConfig configures session briefing generation
type BriefingConfig struct {
	IncludeHistory bool `yaml:"include_history" mapstructure:"include_history"`
	HistoryEntries int  `yaml:"history_entries" mapstructure:"history_entries"`
	IncludeTasks   bool `yaml:"include_tasks" mapstructure:"include_tasks"`
	IncludeProfile bool `yaml:"include_profile" mapstructure:"include_profile"`
}

// CaptureConfig configures the capture workflow
type CaptureConfig struct {
	FrictionBudget int `yaml:"friction_budget" mapstructure:"friction_budget"`
}

// TasksConfig configures task integration
type TasksConfig struct {
	LazyLoading        bool `yaml:"lazy_loading" mapstructure:"lazy_loading"`
	CaptureOnComplete  bool `yaml:"capture_on_completion" mapstructure:"capture_on_completion"`
	PropagateDecisions bool `yaml:"propagate_decisions" mapstructure:"propagate_decisions"`
}

// ProjectConfig holds project-specific settings
type ProjectConfig struct {
	Name string `yaml:"name" mapstructure:"name"`
}
