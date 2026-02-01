package config

import (
	"os"
	"path/filepath"
)

// ResolvePath expands ~ to the home directory and resolves relative paths to absolute.
func ResolvePath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = filepath.Join(home, path[1:])
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	return path
}
