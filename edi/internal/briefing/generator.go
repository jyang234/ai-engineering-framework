package briefing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/config"
)

// SessionSummary holds summarized info about a recent session
type SessionSummary struct {
	Date            time.Time
	Accomplishments []string
	Decisions       []string
}

// TaskSummary holds info about a task
type TaskSummary struct {
	ID        string
	Title     string
	Status    string
	BlockedBy string
}

// Briefing holds all the context for a session briefing
type Briefing struct {
	ProjectContext  string
	RecentSessions  []SessionSummary
	CurrentTasks    *TaskStatus
	HasProfile      bool
	HasHistory      bool
	HasTasks        bool
	ProjectStatus   string
	HasStatus       bool
}

// Generate creates a session briefing from available sources
func Generate(cfg *config.Config) (*Briefing, error) {
	cwd, _ := os.Getwd()
	b := &Briefing{}

	// Load profile
	if cfg.Briefing.IncludeProfile {
		profile, err := loadProfile(cwd)
		if err == nil && profile != "" {
			b.ProjectContext = profile
			b.HasProfile = true
		}
	}

	// Load recent history
	if cfg.Briefing.IncludeHistory {
		history, err := LoadRecentHistory(cwd, cfg.Briefing.HistoryEntries)
		if err == nil && len(history) > 0 {
			for _, h := range history {
				summary := SessionSummary{
					Date: h.Date,
				}
				if h.Summary != "" {
					summary.Accomplishments = []string{h.Summary}
				}
				b.RecentSessions = append(b.RecentSessions, summary)
			}
			b.HasHistory = true
		}
	}

	// Load project status
	if cfg.Briefing.IncludeStatus {
		status, err := loadStatus(cwd)
		if err == nil && status != "" {
			b.ProjectStatus = status
			b.HasStatus = true
		}
	}

	// Load task status
	if cfg.Briefing.IncludeTasks {
		tasks, err := loadTaskStatus(cwd)
		if err == nil && tasks.Total > 0 {
			b.CurrentTasks = tasks
			b.HasTasks = true
		}
	}

	return b, nil
}

// Render converts briefing to markdown for system prompt injection
func (b *Briefing) Render(projectName string) string {
	var sb strings.Builder

	// Header per spec
	sb.WriteString(fmt.Sprintf("# EDI Briefing: %s\n\n", projectName))

	// Project context
	if b.ProjectContext != "" {
		sb.WriteString("## Project Context\n\n")
		sb.WriteString(b.ProjectContext)
		sb.WriteString("\n\n")
	}

	// Project status
	if b.ProjectStatus != "" {
		sb.WriteString("## Project Status\n\n")
		sb.WriteString(b.ProjectStatus)
		sb.WriteString("\n\n")
	}

	// Recent sessions
	if len(b.RecentSessions) > 0 {
		sb.WriteString("## Recent Sessions\n\n")
		for _, s := range b.RecentSessions {
			age := time.Since(s.Date)
			sb.WriteString(fmt.Sprintf("### %s (%s ago)\n",
				s.Date.Format("Jan 2, 2006"),
				formatDuration(age)))
			for _, a := range s.Accomplishments {
				sb.WriteString(fmt.Sprintf("- %s\n", a))
			}
			for _, d := range s.Decisions {
				sb.WriteString(fmt.Sprintf("- Decision: %s\n", d))
			}
			sb.WriteString("\n")
		}
	}

	// Current tasks
	if b.CurrentTasks != nil && b.CurrentTasks.Total > 0 {
		tasks := b.CurrentTasks
		sb.WriteString("## Current Tasks\n\n")
		sb.WriteString(fmt.Sprintf("**Status**: %d completed, %d in progress, %d pending\n\n",
			tasks.Completed, tasks.InProgress, tasks.Pending))

		if len(tasks.InProgressItems) > 0 {
			sb.WriteString("**In Progress:**\n")
			for _, t := range tasks.InProgressItems {
				sb.WriteString(fmt.Sprintf("- %s\n", t.Description))
			}
			sb.WriteString("\n")
		}

		if len(tasks.ReadyItems) > 0 {
			sb.WriteString("**Ready to Start:**\n")
			for _, t := range tasks.ReadyItems {
				sb.WriteString(fmt.Sprintf("- %s\n", t.Description))
			}
			sb.WriteString("\n")
		}
	}

	// Closing per spec
	sb.WriteString("---\n\nReady to continue. What would you like to work on?\n")

	return sb.String()
}


func loadStatus(projectPath string) (string, error) {
	statusPath := filepath.Join(projectPath, ".edi", "status.md")
	content, err := os.ReadFile(statusPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func loadProfile(projectPath string) (string, error) {
	profilePath := filepath.Join(projectPath, ".edi", "profile.md")
	content, err := os.ReadFile(profilePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
