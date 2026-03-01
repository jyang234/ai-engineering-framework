package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// guardState is the file-based state for the failure loop counter.
type guardState struct {
	ConsecutiveFailures int    `json:"consecutive_failures"`
	Advised             bool   `json:"advised"`
	LastFailureCommand  string `json:"last_failure_command"`
	LastFailureError    string `json:"last_failure_error"`
}

func stateFilePath(sessionID string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("edi-guard-%s.json", sessionID))
}

func readState(sessionID string) guardState {
	if sessionID == "" {
		return guardState{}
	}
	data, err := os.ReadFile(stateFilePath(sessionID))
	if err != nil {
		return guardState{}
	}
	var s guardState
	if err := json.Unmarshal(data, &s); err != nil {
		return guardState{}
	}
	return s
}

func writeState(sessionID string, state guardState) {
	if sessionID == "" {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(stateFilePath(sessionID), data, 0600)
}
