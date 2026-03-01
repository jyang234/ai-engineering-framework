package guard

import "encoding/json"

// HookInput represents the JSON Claude Code sends to command hooks on stdin.
type HookInput struct {
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Error         string          `json:"error"`
	Trigger       string          `json:"trigger"`
}

// BashToolInput is the tool_input shape for Bash tool calls.
type BashToolInput struct {
	Command string `json:"command"`
}
