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
	if len(path) >= 2 && path[0] == '~' && path[1] == '/' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = filepath.Join(home, path[2:])
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		path = home
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	return path
}
