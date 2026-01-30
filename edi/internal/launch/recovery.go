package launch

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/tasks"
)

// StaleSession represents a session that was not cleanly ended
type StaleSession struct {
	SessionID string
	LastSync  time.Time
}

// DetectStaleSession checks if the previous session was not cleanly ended.
// A session is stale if active.yaml has a last_session_id but no matching
// history file exists in .edi/history/.
func DetectStaleSession(projectDir string) (*StaleSession, error) {
	manifest, err := tasks.LoadManifest(projectDir)
	if err != nil {
		return nil, err
	}

	if manifest.LastSessionID == "" {
		return nil, nil
	}

	// Check if a history file exists for this session
	historyDir := filepath.Join(projectDir, ".edi", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No history dir means no history files — session is stale
			return &StaleSession{
				SessionID: manifest.LastSessionID,
				LastSync:  manifest.LastSync,
			}, nil
		}
		return nil, err
	}

	// Look for a history file containing the session ID prefix
	prefix := manifest.LastSessionID[:8]
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// History files are named {date}-{session-id-prefix}.md
		if strings.Contains(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			// Found a matching history file — not stale
			return nil, nil
		}
	}

	return &StaleSession{
		SessionID: manifest.LastSessionID,
		LastSync:  manifest.LastSync,
	}, nil
}
