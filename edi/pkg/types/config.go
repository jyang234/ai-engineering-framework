package types

// Config represents the full EDI configuration (exported for use by other packages)
type Config struct {
	Version  string          `yaml:"version" json:"version"`
	Agent    string          `yaml:"agent" json:"agent"`
	Recall   RecallConfig    `yaml:"recall" json:"recall"`
	Briefing BriefingConfig  `yaml:"briefing" json:"briefing"`
	Capture  CaptureConfig   `yaml:"capture" json:"capture"`
	Tasks    TasksConfig     `yaml:"tasks" json:"tasks"`
	Project  ProjectConfig   `yaml:"project" json:"project"`
}

// RecallConfig configures the RECALL knowledge system
type RecallConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// BriefingConfig configures session briefing generation
type BriefingConfig struct {
	IncludeHistory bool `yaml:"include_history" json:"include_history"`
	HistoryEntries int  `yaml:"history_entries" json:"history_entries"`
	IncludeTasks   bool `yaml:"include_tasks" json:"include_tasks"`
	IncludeProfile bool `yaml:"include_profile" json:"include_profile"`
}

// CaptureConfig configures the capture workflow
type CaptureConfig struct {
	FrictionBudget int `yaml:"friction_budget" json:"friction_budget"`
}

// TasksConfig configures task integration
type TasksConfig struct {
	LazyLoading        bool `yaml:"lazy_loading" json:"lazy_loading"`
	CaptureOnComplete  bool `yaml:"capture_on_completion" json:"capture_on_completion"`
	PropagateDecisions bool `yaml:"propagate_decisions" json:"propagate_decisions"`
}

// ProjectConfig holds project-specific settings
type ProjectConfig struct {
	Name string `yaml:"name" json:"name"`
}
