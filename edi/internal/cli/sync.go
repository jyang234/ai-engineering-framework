package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/assets"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync embedded assets to their install locations",
	Long: `Re-copies skills, agents, commands, and subagents from the EDI binary
to their installed locations (~/.edi/ and ~/.claude/).

This is a lightweight alternative to 'edi init --global --force' that
only updates assets without touching configuration, recall database,
or directory structure.`,
	RunE: runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ediHome := filepath.Join(home, ".edi")
	if !exists(ediHome) {
		return fmt.Errorf("~/.edi does not exist; run 'edi init --global' first")
	}

	// Sync agents
	if err := installEmbeddedDir(assets.Agents, "agents", filepath.Join(ediHome, "agents")); err != nil {
		return fmt.Errorf("failed to sync agents: %w", err)
	}
	fmt.Println("  Synced agents")

	// Sync commands
	if err := installEmbeddedDir(assets.Commands, "commands", filepath.Join(ediHome, "commands")); err != nil {
		return fmt.Errorf("failed to sync commands: %w", err)
	}
	fmt.Println("  Synced commands")

	// Sync skills
	skills := []struct {
		name string
		fs   embed.FS
	}{
		{"edi-core", assets.EdiCoreSkill},
		{"retrieval-judge", assets.RetrievalJudgeSkill},
		{"coding", assets.CodingSkill},
		{"testing", assets.TestingSkill},
		{"scaffolding-tests", assets.ScaffoldingTestsSkill},
		{"refactoring-planning", assets.RefactoringPlanningSkill},
	}
	for _, skill := range skills {
		if err := installSkill(home, skill.name, skill.fs); err != nil {
			return fmt.Errorf("failed to sync %s skill: %w", skill.name, err)
		}
	}
	fmt.Printf("  Synced skills (%d)\n", len(skills))

	// Sync subagents
	claudeAgentsDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(claudeAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create Claude agents directory: %w", err)
	}
	if err := installEmbeddedDir(assets.Subagents, "subagents", claudeAgentsDir); err != nil {
		return fmt.Errorf("failed to sync subagents: %w", err)
	}
	fmt.Println("  Synced subagents")

	// Sync Ralph loop files
	ralphDir := filepath.Join(ediHome, "ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return fmt.Errorf("failed to create ralph directory: %w", err)
	}
	if err := installEmbeddedDir(assets.Ralph, "ralph", ralphDir); err != nil {
		return fmt.Errorf("failed to sync ralph files: %w", err)
	}
	if err := os.Chmod(filepath.Join(ralphDir, "ralph.sh"), 0755); err != nil {
		return fmt.Errorf("failed to make ralph.sh executable: %w", err)
	}
	fmt.Println("  Synced ralph")

	fmt.Println("\nAssets synced successfully.")
	return nil
}
