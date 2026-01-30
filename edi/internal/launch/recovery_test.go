package launch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/aef/edi/internal/tasks"
)

func TestDetectStaleSession(t *testing.T) {
	t.Run("NoManifest", func(t *testing.T) {
		tmpDir := t.TempDir()
		stale, err := DetectStaleSession(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stale != nil {
			t.Error("expected nil for missing manifest")
		}
	})

	t.Run("EmptySessionID", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := tasks.NewManifest()
		if err := tasks.SaveManifest(tmpDir, m); err != nil {
			t.Fatal(err)
		}

		stale, err := DetectStaleSession(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stale != nil {
			t.Error("expected nil for empty session ID")
		}
	})

	t.Run("StaleSession_NoHistoryDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := tasks.NewManifest()
		m.LastSessionID = "abcdef12-3456-7890-abcd-ef1234567890"
		if err := tasks.SaveManifest(tmpDir, m); err != nil {
			t.Fatal(err)
		}

		stale, err := DetectStaleSession(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stale == nil {
			t.Fatal("expected stale session")
		}
		if stale.SessionID != m.LastSessionID {
			t.Errorf("expected session ID %s, got %s", m.LastSessionID, stale.SessionID)
		}
	})

	t.Run("StaleSession_NoMatchingFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := tasks.NewManifest()
		m.LastSessionID = "abcdef12-3456-7890-abcd-ef1234567890"
		if err := tasks.SaveManifest(tmpDir, m); err != nil {
			t.Fatal(err)
		}

		// Create history dir with unrelated file
		historyDir := filepath.Join(tmpDir, ".edi", "history")
		os.MkdirAll(historyDir, 0755)
		os.WriteFile(filepath.Join(historyDir, "2026-01-29-ffffffff.md"), []byte("---\n---\n"), 0644)

		stale, err := DetectStaleSession(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stale == nil {
			t.Fatal("expected stale session")
		}
	})

	t.Run("CleanSession_MatchingFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := tasks.NewManifest()
		m.LastSessionID = "abcdef12-3456-7890-abcd-ef1234567890"
		if err := tasks.SaveManifest(tmpDir, m); err != nil {
			t.Fatal(err)
		}

		// Create matching history file
		historyDir := filepath.Join(tmpDir, ".edi", "history")
		os.MkdirAll(historyDir, 0755)
		os.WriteFile(filepath.Join(historyDir, "2026-01-29-abcdef12.md"), []byte("---\n---\n"), 0644)

		stale, err := DetectStaleSession(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stale != nil {
			t.Error("expected nil for clean session")
		}
	})
}
