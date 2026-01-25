package cli

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/edi/internal/assets"
	"github.com/user/edi/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize EDI in current directory or globally",
	Long: `Initialize EDI workspace.

Without flags: Creates .edi/ in the current directory for project-specific configuration.
With --global: Creates ~/.edi/ with default agents, commands, and skills.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().Bool("global", false, "Initialize global EDI installation at ~/.edi/")
	initCmd.Flags().Bool("force", false, "Overwrite existing files")
}

func runInit(cmd *cobra.Command, args []string) error {
	global, _ := cmd.Flags().GetBool("global")
	force, _ := cmd.Flags().GetBool("force")

	if global {
		return initGlobal(force)
	}
	return initProject(force)
}

func initGlobal(force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ediHome := filepath.Join(home, ".edi")

	// Check existing
	if exists(ediHome) && !force {
		return fmt.Errorf("~/.edi already exists (use --force to overwrite)")
	}

	// Create directory structure
	dirs := []string{
		ediHome,
		filepath.Join(ediHome, "agents"),
		filepath.Join(ediHome, "commands"),
		filepath.Join(ediHome, "skills"),
		filepath.Join(ediHome, "recall"),
		filepath.Join(ediHome, "cache"),
		filepath.Join(ediHome, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Install default agents from embedded assets
	if err := installEmbeddedDir(assets.Agents, "agents", filepath.Join(ediHome, "agents")); err != nil {
		return fmt.Errorf("failed to install agents: %w", err)
	}

	// Install slash commands
	if err := installEmbeddedDir(assets.Commands, "commands", filepath.Join(ediHome, "commands")); err != nil {
		return fmt.Errorf("failed to install commands: %w", err)
	}

	// Install edi-core skill to Claude's skills directory
	claudeSkillsDir := filepath.Join(home, ".claude", "skills", "edi-core")
	if err := os.MkdirAll(claudeSkillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create Claude skills directory: %w", err)
	}
	if err := installEdiCoreSkill(claudeSkillsDir); err != nil {
		return fmt.Errorf("failed to install edi-core skill: %w", err)
	}

	// Install subagents to Claude's agents directory
	claudeAgentsDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(claudeAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create Claude agents directory: %w", err)
	}
	if err := installEmbeddedDir(assets.Subagents, "subagents", claudeAgentsDir); err != nil {
		return fmt.Errorf("failed to install subagents: %w", err)
	}

	// Create default config
	configPath := filepath.Join(ediHome, "config.yaml")
	if err := config.WriteDefault(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Initialized global EDI at ~/.edi/")
	fmt.Println("")
	fmt.Println("Created:")
	fmt.Println("  ~/.edi/agents/         - Agent definitions")
	fmt.Println("  ~/.edi/commands/       - Slash commands")
	fmt.Println("  ~/.edi/skills/         - Skills")
	fmt.Println("  ~/.edi/recall/         - Knowledge database")
	fmt.Println("  ~/.edi/config.yaml     - Configuration")
	fmt.Println("")
	fmt.Println("Installed to Claude Code:")
	fmt.Println("  ~/.claude/skills/edi-core/  - EDI core skill")
	fmt.Println("  ~/.claude/agents/           - EDI subagents (7)")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  1. cd to a project directory")
	fmt.Println("  2. Run: edi init")
	fmt.Println("  3. Edit .edi/profile.md to describe your project")
	fmt.Println("  4. Start a session: edi")

	return nil
}

func initProject(force bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	ediDir := filepath.Join(cwd, ".edi")

	// Check existing
	if exists(ediDir) && !force {
		return fmt.Errorf(".edi already exists (use --force to overwrite)")
	}

	// Create directory structure
	dirs := []string{
		ediDir,
		filepath.Join(ediDir, "history"),
		filepath.Join(ediDir, "tasks"),
		filepath.Join(ediDir, "recall"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create project config
	configPath := filepath.Join(ediDir, "config.yaml")
	if err := config.WriteProjectDefault(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create profile template
	profilePath := filepath.Join(ediDir, "profile.md")
	if err := writeProfileTemplate(profilePath); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	fmt.Println("Initialized EDI in current project")
	fmt.Println("")
	fmt.Println("Created:")
	fmt.Println("  .edi/config.yaml  - Project configuration")
	fmt.Println("  .edi/profile.md   - Project description")
	fmt.Println("  .edi/history/     - Session history")
	fmt.Println("  .edi/tasks/       - Task annotations")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit .edi/profile.md to describe your project")
	fmt.Println("  2. Start a session: edi")

	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func installEmbeddedDir(fsys embed.FS, srcDir, dstDir string) error {
	return fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Read embedded file
		content, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Determine destination path
		relPath, _ := filepath.Rel(srcDir, path)
		dstPath := filepath.Join(dstDir, relPath)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		// Write file
		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}

		return nil
	})
}

func installEdiCoreSkill(dstDir string) error {
	content, err := assets.EdiCoreSkill.ReadFile("skills/edi-core/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read edi-core skill: %w", err)
	}

	dstPath := filepath.Join(dstDir, "SKILL.md")
	if err := os.WriteFile(dstPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write skill: %w", err)
	}

	return nil
}

func writeProfileTemplate(path string) error {
	template := `# Project Profile

## Overview

<!-- Describe what this project does, its main purpose, and target users -->

## Architecture

<!-- High-level architecture: key components, data flow, external dependencies -->

## Tech Stack

<!-- Programming languages, frameworks, databases, and key libraries -->

## Conventions

<!-- Coding style, naming conventions, file organization patterns -->

## Key Decisions

<!-- Important architectural or technical decisions that should guide development -->

## Getting Started

<!-- Quick setup instructions for new contributors -->
`
	return os.WriteFile(path, []byte(template), 0644)
}
