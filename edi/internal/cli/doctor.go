package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/codex"
	"github.com/anthropics/aef/edi/internal/config"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check EDI installation health",
	Long:  `Runs diagnostic checks on the EDI installation and reports pass/fail for each component.`,
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ediHome := filepath.Join(home, ".edi")
	passed := 0
	failed := 0

	check := func(name string, ok bool, detail string) {
		if ok {
			fmt.Printf("  ✓ %s\n", name)
			passed++
		} else {
			fmt.Printf("  ✗ %s — %s\n", name, detail)
			failed++
		}
	}

	// Global installation
	fmt.Println("Global installation:")
	check("~/.edi/ directory", exists(ediHome), "run: edi init --global")
	check("~/.edi/config.yaml", exists(filepath.Join(ediHome, "config.yaml")), "run: edi init --global")
	check("~/.edi/agents/", exists(filepath.Join(ediHome, "agents")), "run: edi init --global")
	check("~/.edi/commands/", exists(filepath.Join(ediHome, "commands")), "run: edi init --global")

	// Load config to check backend
	cfg, cfgErr := config.Load()

	fmt.Println()
	fmt.Println("RECALL backend:")
	if cfgErr != nil {
		check("config readable", false, cfgErr.Error())
	} else {
		check("config readable", true, "")
		backend := cfg.Recall.Backend
		if backend == "" {
			backend = "v0"
		}
		fmt.Printf("  → backend: %s\n", backend)

		if backend == "codex" {
			binExists, binPath := codex.CheckBinaryExists()
			check("recall-mcp binary", binExists, fmt.Sprintf("not found at %s", binPath))

			dbPath := cfg.Codex.MetadataDB
			if dbPath == "" {
				dbPath = filepath.Join(ediHome, "codex.db")
			}
			check("codex database", exists(expandHomePath(dbPath)), fmt.Sprintf("will be created at %s on first use", dbPath))
		} else {
			dbPath := filepath.Join(ediHome, "recall", "global.db")
			check("recall v0 database", exists(dbPath), "will be created on first use")
		}
	}

	// Ollama
	fmt.Println()
	fmt.Println("Ollama (for Codex hybrid search):")
	ollamaAvail, modelAvail := codex.CheckOllama()
	check("ollama installed", ollamaAvail, "install from https://ollama.com")
	if ollamaAvail {
		check("nomic-embed-text model", modelAvail, "run: ollama pull nomic-embed-text")
	}

	// Claude Code
	fmt.Println()
	fmt.Println("Claude Code:")
	_, claudeErr := exec.LookPath("claude")
	check("claude binary", claudeErr == nil, "install Claude Code CLI")

	// Project init
	fmt.Println()
	fmt.Println("Project (current directory):")
	cwd, _ := os.Getwd()
	projectEdi := filepath.Join(cwd, ".edi")
	check(".edi/ directory", exists(projectEdi), "run: edi init")
	if exists(projectEdi) {
		check(".edi/profile.md", exists(filepath.Join(projectEdi, "profile.md")), "run: edi init")
		check(".edi/config.yaml", exists(filepath.Join(projectEdi, "config.yaml")), "run: edi init")
	}

	// Summary
	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)

	return nil
}

func expandHomePath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
