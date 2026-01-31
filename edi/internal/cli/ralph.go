package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/anthropics/aef/edi/internal/assets"
)

var (
	ralphPRDPath         string
	ralphPromptPath      string
	ralphMaxIterations   int
	ralphStuckThreshold  int
	ralphInitForce       bool
)

var ralphCmd = &cobra.Command{
	Use:   "ralph",
	Short: "Run the Ralph autonomous execution loop",
	Long: `Run well-defined coding tasks autonomously from a PRD.json file.

Each iteration starts a fresh context window, reads the next task from PRD.json,
implements it, commits, and moves on. State lives in files and git.

Use 'edi ralph init' to scaffold a PRD.json template.`,
	RunE: runRalph,
}

var ralphInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a PRD.json template in the current directory",
	RunE:  runRalphInit,
}

func init() {
	ralphCmd.Flags().StringVar(&ralphPRDPath, "prd", "PRD.json", "Path to PRD.json file")
	ralphCmd.Flags().StringVar(&ralphPromptPath, "prompt", "", "Path to custom PROMPT.md file")
	ralphCmd.Flags().IntVar(&ralphMaxIterations, "max-iterations", 50, "Maximum loop iterations")
	ralphCmd.Flags().IntVar(&ralphStuckThreshold, "stuck-threshold", 3, "Consecutive errors before auto-escalate")

	ralphInitCmd.Flags().BoolVar(&ralphInitForce, "force", false, "Overwrite existing PRD.json")
	ralphCmd.AddCommand(ralphInitCmd)
}

func runRalphInit(cmd *cobra.Command, args []string) error {
	target := "PRD.json"
	if !ralphInitForce && exists(target) {
		return fmt.Errorf("PRD.json already exists (use --force to overwrite)")
	}

	data, err := assets.Ralph.ReadFile("ralph/example-PRD.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded example-PRD.json: %w", err)
	}

	if err := os.WriteFile(target, data, 0644); err != nil {
		return fmt.Errorf("failed to write PRD.json: %w", err)
	}

	fmt.Println("Created PRD.json from template.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit PRD.json with your project and user stories")
	fmt.Println("  2. Run: edi ralph")
	return nil
}

func runRalph(cmd *cobra.Command, args []string) error {
	// Check for PRD
	prdPath := ralphPRDPath
	if !exists(prdPath) {
		return fmt.Errorf("PRD not found at %s\n\nTo get started:\n  edi ralph init    # creates a PRD.json template\n  $EDITOR PRD.json  # define your tasks\n  edi ralph         # run the loop", prdPath)
	}

	// Create .ralph/ working directory
	if err := os.MkdirAll(".ralph", 0755); err != nil {
		return fmt.Errorf("failed to create .ralph/ directory: %w", err)
	}

	// Ensure .ralph/ is in .gitignore
	ensureGitignore(".ralph/")

	// Write embedded ralph.sh
	scriptData, err := assets.Ralph.ReadFile("ralph/ralph.sh")
	if err != nil {
		return fmt.Errorf("failed to read embedded ralph.sh: %w", err)
	}
	scriptPath := filepath.Join(".ralph", "ralph.sh")
	if err := os.WriteFile(scriptPath, scriptData, 0755); err != nil {
		return fmt.Errorf("failed to write ralph.sh: %w", err)
	}

	// Handle PROMPT.md — write to .ralph/PROMPT.md (not CWD)
	if ralphPromptPath != "" {
		if err := copyFile(ralphPromptPath, filepath.Join(".ralph", "PROMPT.md")); err != nil {
			return fmt.Errorf("failed to copy prompt file: %w", err)
		}
	} else if !exists("PROMPT.md") {
		// Write embedded default to .ralph/PROMPT.md
		promptData, err := assets.Ralph.ReadFile("ralph/PROMPT.md")
		if err != nil {
			return fmt.Errorf("failed to read embedded PROMPT.md: %w", err)
		}
		if err := os.WriteFile(filepath.Join(".ralph", "PROMPT.md"), promptData, 0644); err != nil {
			return fmt.Errorf("failed to write PROMPT.md: %w", err)
		}
	}

	// Handle PRD path if not default
	prdCopied := false
	if prdPath != "PRD.json" {
		if err := copyFile(prdPath, "PRD.json"); err != nil {
			return fmt.Errorf("failed to copy PRD to working directory: %w", err)
		}
		prdCopied = true
	}

	// Set env vars
	env := os.Environ()
	env = append(env, fmt.Sprintf("MAX_ITERATIONS=%d", ralphMaxIterations))
	env = append(env, fmt.Sprintf("STUCK_THRESHOLD=%d", ralphStuckThreshold))

	// Run ralph.sh via exec.Command (stay alive for cleanup)
	bashCmd := exec.Command("bash", scriptPath)
	bashCmd.Env = env
	bashCmd.Stdin = os.Stdin
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr

	// Intercept signals so cleanup runs even on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		if bashCmd.Process != nil {
			bashCmd.Process.Signal(sig)
		}
	}()

	runErr := bashCmd.Run()
	signal.Stop(sigCh)

	// Copy modified PRD.json back to original path if we copied it in
	if prdCopied {
		if err := copyFile("PRD.json", prdPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to copy PRD.json back to %s: %v\n", prdPath, err)
		}
		os.Remove("PRD.json")
	}

	return runErr
}

func ensureGitignore(entry string) {
	const gitignore = ".gitignore"
	data, _ := os.ReadFile(gitignore)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return
		}
	}
	f, err := os.OpenFile(gitignore, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString(entry + "\n")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
