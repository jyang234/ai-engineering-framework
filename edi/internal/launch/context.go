package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/agents"
	"github.com/anthropics/aef/edi/internal/briefing"
	"github.com/anthropics/aef/edi/internal/config"
)

// BuildContext generates the session context file and returns its path
func BuildContext(cfg *config.Config, sessionID string, brief *briefing.Briefing, projectName string) (string, error) {
	var sb strings.Builder

	// EDI identity and session info
	sb.WriteString("# EDI - Enhanced Development Intelligence\n\n")
	sb.WriteString("You are operating as EDI, an AI engineering assistant with ")
	sb.WriteString("continuity, knowledge, and specialized behaviors.\n\n")
	sb.WriteString(fmt.Sprintf("Session ID: %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))

	// Load and include agent configuration
	agent, err := agents.Load(cfg.Agent)
	if err != nil {
		// Fall back to minimal agent if not found
		agent = &agents.Agent{
			Name:        cfg.Agent,
			Description: "Default agent",
		}
	}

	sb.WriteString(fmt.Sprintf("## Current Mode: %s\n\n", agent.Name))
	if agent.Description != "" {
		sb.WriteString(agent.Description + "\n\n")
	}
	if agent.SystemPrompt != "" {
		sb.WriteString(agent.SystemPrompt)
		sb.WriteString("\n\n")
	}

	// Include briefing
	if brief != nil {
		rendered := brief.Render(projectName)
		if rendered != "" {
			sb.WriteString("## Session Briefing\n\n")
			sb.WriteString(rendered)
			sb.WriteString("\n\n")
		}
	}

	// Instructions for briefing display
	sb.WriteString("## Session Briefing Display\n\n")
	sb.WriteString("When the user says 'briefing', display the session briefing that was injected via the SessionStart hook.\n")
	sb.WriteString("Format it cleanly and concisely. Do not elaborate, summarize, or add commentary.\n")
	sb.WriteString("Just present the briefing content and ask what they'd like to work on.\n\n")

	// Include RECALL instructions
	if cfg.Recall.Enabled {
		sb.WriteString("## RECALL Knowledge Base\n\n")
		sb.WriteString("You have access to RECALL, the organizational knowledge base. ")
		sb.WriteString("Use it proactively to:\n")
		sb.WriteString("- Check for existing patterns before implementing\n")
		sb.WriteString("- Look up past decisions (ADRs) for context\n")
		sb.WriteString("- Search for known issues when troubleshooting\n\n")
		sb.WriteString("Available tools: recall_search, recall_get, recall_add, recall_feedback, flight_recorder_log\n\n")
	}

	// Include slash command instructions
	sb.WriteString(buildSlashCommandInstructions())

	// Write to cache file
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".edi", "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	filename := fmt.Sprintf("session-%d.md", time.Now().Unix())
	contextPath := filepath.Join(cacheDir, filename)

	if err := os.WriteFile(contextPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write context file: %w", err)
	}

	return contextPath, nil
}

func buildSlashCommandInstructions() string {
	return `## EDI Slash Commands

You have access to the following slash commands. When the user types one of these, execute the corresponding action:

### /plan (aliases: /architect, /design)
Switch to architect mode for system design work.
- Focus on system-level thinking and ADRs
- Query RECALL for architecture context
- Consider trade-offs and long-term implications

### /build (aliases: /code, /implement)
Switch to coder mode for implementation work.
- Focus on clean, tested code
- Query RECALL for implementation patterns
- Follow project conventions

### /review (alias: /check)
Switch to reviewer mode for code review.
- Focus on finding issues and providing constructive feedback
- Query RECALL for security and quality patterns
- Check for common pitfalls

### /incident (aliases: /debug, /fix)
Switch to incident mode for troubleshooting.
- Focus on rapid diagnosis and mitigation
- Query RECALL for runbooks and known issues
- Log findings to flight recorder

### /task [task-id | description]
Manage task-based workflows with RECALL enrichment.

Without arguments: Show current Tasks status
With task-id: Pick up task with full context loaded
With description: Create new tasks with RECALL queries

### /end
End the current session.
- Generate a session summary
- Identify capture candidates
- Prompt to save significant items to RECALL
- Save session history to .edi/history/

When switching modes, acknowledge the switch and adapt your behavior accordingly.
`
}
