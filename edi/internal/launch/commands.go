package launch

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

// InstallCommands copies slash commands from ~/.edi/commands/ to .claude/commands/
func InstallCommands() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	srcDir := filepath.Join(home, ".edi", "commands")
	dstDir := filepath.Join(cwd, ".claude", "commands")

	// Check if source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil // No commands to install
	}

	// Ensure destination exists
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Copy each command if missing or changed
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if needsCopy(srcPath, dstPath) {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// needsCopy checks if the source file is newer or different from destination
func needsCopy(src, dst string) bool {
	dstInfo, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return true
	}

	// Check if file has content
	if dstInfo.Size() == 0 {
		return true
	}

	// Compare hashes
	srcHash, err := fileHash(src)
	if err != nil {
		return true
	}

	dstHash, err := fileHash(dst)
	if err != nil {
		return true
	}

	return srcHash != dstHash
}

// fileHash computes SHA256 hash of a file
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) (err error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := destFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
