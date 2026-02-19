package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectAutoMemoryDir returns the Claude auto memory directory for the given
// project path. Returns empty string if the directory does not exist.
//
// Claude Code stores auto memory at:
//
//	~/.claude/projects/-<sanitized-git-root>/memory/
//
// The project path is the git repository root (not necessarily cwd).
// The path has "/" replaced with "-" to form the directory name.
func DetectAutoMemoryDir(projectPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	root := gitRoot(projectPath)
	sanitized := sanitizePath(root)
	memDir := filepath.Join(home, ".claude", "projects", sanitized, "memory")

	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		return ""
	}
	return memDir
}

// EnsureAutoMemoryDir creates the auto memory directory if it does not exist
// and returns its path. Returns empty string on error.
func EnsureAutoMemoryDir(projectPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	root := gitRoot(projectPath)
	sanitized := sanitizePath(root)
	memDir := filepath.Join(home, ".claude", "projects", sanitized, "memory")

	if err := os.MkdirAll(memDir, 0755); err != nil {
		return ""
	}
	return memDir
}

// MemoryFilePath returns the path to MEMORY.md for the given project.
// Returns empty string if the directory cannot be resolved.
func MemoryFilePath(projectPath string) string {
	dir := EnsureAutoMemoryDir(projectPath)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "MEMORY.md")
}

// gitRoot returns the git repository root for the given path.
// Falls back to the path itself if not inside a git repo.
func gitRoot(path string) string {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return path // Not a git repo — use path as-is
	}
	return strings.TrimSpace(string(out))
}

// sanitizePath converts an absolute path to the format Claude Code uses
// for project directory names: replaces "/" with "-".
// Example: /home/user/my-project -> -home-user-my-project
func sanitizePath(path string) string {
	return strings.ReplaceAll(path, string(filepath.Separator), "-")
}
