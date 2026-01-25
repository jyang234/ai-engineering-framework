package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/user/edi/internal/briefing"
	"github.com/user/edi/internal/config"
	"github.com/user/edi/internal/launch"
)

func runLaunch(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Install slash commands to .claude/commands/
	if err := launch.InstallCommands(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to install commands: %v\n", err)
		}
	}

	// Generate briefing
	brief, err := briefing.Generate(cfg)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to generate briefing: %v\n", err)
		}
	}

	// Get project name for briefing
	cwd, _ := os.Getwd()
	projectName := filepath.Base(cwd)
	if cfg.Project.Name != "" {
		projectName = cfg.Project.Name
	}

	// Write briefing to file for SessionStart hook to read
	if brief != nil {
		home, _ := os.UserHomeDir()
		briefingPath := filepath.Join(home, ".edi", "cache", "current-briefing.md")
		os.MkdirAll(filepath.Dir(briefingPath), 0755)
		os.WriteFile(briefingPath, []byte(brief.Render(projectName)), 0644)
	}

	// Build session context
	contextPath, err := launch.BuildContext(cfg, sessionID, brief, projectName)
	if err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	if verbose {
		fmt.Printf("Session ID: %s\n", sessionID)
		fmt.Printf("Context file: %s\n", contextPath)
		fmt.Printf("Agent: %s\n", cfg.Agent)
		fmt.Println()
	}

	// Launch Claude Code (replaces current process)
	return launch.Launch(contextPath)
}
