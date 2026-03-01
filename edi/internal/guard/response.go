package guard

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// ParseStdin reads and decodes the hook input from stdin.
func ParseStdin() *HookInput {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return nil
	}
	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}
	if input.CWD == "" {
		input.CWD, _ = os.Getwd()
	}
	return &input
}

// ParseBashInput extracts the command from Bash tool_input JSON.
// Exported: called by both the main.go dispatcher and FailureLoopPolicy.
func ParseBashInput(raw json.RawMessage) *BashToolInput {
	var b BashToolInput
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	return &b
}

// BuildPreToolUseResponse constructs the JSON response for PreToolUse.
// advisories is a slice; multiple advisories are joined with newlines
// to produce the single additionalContext string required by the
// Claude Code hook protocol.
func BuildPreToolUseResponse(command string, advisories []string, modified bool) map[string]interface{} {
	hso := map[string]interface{}{
		"hookEventName":      "PreToolUse",
		"permissionDecision": "allow",
	}
	if modified {
		hso["updatedInput"] = map[string]string{
			"command": command,
		}
	}
	if len(advisories) > 0 {
		hso["additionalContext"] = strings.Join(advisories, "\n")
	}
	return map[string]interface{}{
		"hookSpecificOutput": hso,
	}
}
