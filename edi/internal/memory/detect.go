package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectAutoMemoryDir returns the Claude auto memory directory for the given
// project path. Returns empty string if the directory does not exist.
//
// Claude Code stores auto memory at:
//
//	~/.claude/projects/-<sanitized-cwd>/memory/
//
// where the cwd path has "/" replaced with "-" and leading slash becomes "-".
func DetectAutoMemoryDir(projectPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	sanitized := sanitizePath(projectPath)
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

	sanitized := sanitizePath(projectPath)
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

// sanitizePath converts an absolute path to the format Claude Code uses
// for project directory names: replaces "/" with "-".
// Example: /home/user/my-project -> -home-user-my-project
func sanitizePath(path string) string {
	return strings.ReplaceAll(path, string(filepath.Separator), "-")
}
