package launch

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Launch replaces the current process with Claude Code
func Launch(contextPath string) error {
	// Find Claude Code binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("Claude Code not found in PATH. Install from: https://claude.ai/code")
	}

	// Build arguments - "briefing" triggers display of session context
	args := []string{
		"claude",
		"--append-system-prompt-file", contextPath,
		"briefing",
	}

	// Get current environment
	env := os.Environ()

	// Ensure stdout is flushed before exec replaces the process
	os.Stdout.Sync()

	// Replace current process with Claude Code
	// This is the cleanest approach - EDI exits and Claude Code runs natively
	return syscall.Exec(claudePath, args, env)
}

// LaunchWithPrompt launches Claude Code with an initial prompt
func LaunchWithPrompt(contextPath, prompt string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("Claude Code not found in PATH. Install from: https://claude.ai/code")
	}

	args := []string{
		"claude",
		"--append-system-prompt-file", contextPath,
		"--prompt", prompt,
	}

	env := os.Environ()
	return syscall.Exec(claudePath, args, env)
}

// CheckClaudeInstalled checks if Claude Code is available
func CheckClaudeInstalled() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("Claude Code not found in PATH. Install from: https://claude.ai/code")
	}
	return nil
}
