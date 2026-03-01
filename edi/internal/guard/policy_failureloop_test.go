package guard

import (
	"os"
	"testing"
)

func TestFailureCounter_IncrementAndReset(t *testing.T) {
	sessionID := "test-session-counter"
	defer os.Remove(stateFilePath(sessionID))

	// Start at zero
	state := readState(sessionID)
	if state.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0, got %d", state.ConsecutiveFailures)
	}

	// Increment
	state.ConsecutiveFailures++
	state.LastFailureCommand = "go test ./..."
	state.LastFailureError = "exit status 1"
	writeState(sessionID, state)

	state = readState(sessionID)
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("expected 1, got %d", state.ConsecutiveFailures)
	}

	// Reset
	writeState(sessionID, guardState{})
	state = readState(sessionID)
	if state.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0 after reset, got %d", state.ConsecutiveFailures)
	}
}

func TestFailureCounter_Advisory(t *testing.T) {
	sessionID := "test-session-advisory"
	defer os.Remove(stateFilePath(sessionID))

	// Simulate 5 failures
	state := guardState{
		ConsecutiveFailures: 5,
		Advised:             false,
		LastFailureCommand:  "go test ./...",
		LastFailureError:    "exit status 1",
	}
	writeState(sessionID, state)

	// Read and check advisory would fire
	state = readState(sessionID)
	if state.ConsecutiveFailures < 5 || state.Advised {
		t.Fatal("advisory should be ready to fire")
	}

	// Mark as advised
	state.Advised = true
	writeState(sessionID, state)

	// Check advisory won't fire again
	state = readState(sessionID)
	if !state.Advised {
		t.Fatal("advisory should be marked as fired")
	}
}
