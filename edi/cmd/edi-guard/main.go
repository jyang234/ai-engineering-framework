// edi-guard is a Claude Code command hook that enforces build tags,
// blocks destructive commands, detects failure loops, and snapshots
// session state before compaction.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/aef/edi/internal/config"
	"gopkg.in/yaml.v3"
)

// hookInput represents the JSON Claude Code sends to command hooks on stdin.
type hookInput struct {
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	Error         string          `json:"error"`
	Trigger       string          `json:"trigger"`
}

// bashToolInput is the tool_input shape for Bash tool calls.
type bashToolInput struct {
	Command string `json:"command"`
}

// guardState is the file-based state for the failure loop counter.
type guardState struct {
	ConsecutiveFailures int    `json:"consecutive_failures"`
	Advised             bool   `json:"advised"`
	LastFailureCommand  string `json:"last_failure_command"`
	LastFailureError    string `json:"last_failure_error"`
}

// guardConfigFile is the resolved config after merging all layers.
type guardConfigFile struct {
	Guard config.GuardConfig
	Agent string
}

// guardConfigOverlay is the YAML shape we unmarshal from .edi/config.yaml.
// Uses *bool for Enabled so we can distinguish "not set" from "set to false."
type guardConfigOverlay struct {
	Guard struct {
		Enabled              *bool                `yaml:"enabled"`
		BuildTags            []string             `yaml:"build_tags"`
		DenyPatterns         []config.DenyPattern `yaml:"deny_patterns"`
		FailureLoopThreshold int                  `yaml:"failure_loop_threshold"`
	} `yaml:"guard"`
	Agent string `yaml:"agent"`
}

func main() {
	input := parseStdin()
	if input == nil {
		os.Exit(0)
	}

	// Skip non-EDI projects
	if _, err := os.Stat(filepath.Join(input.CWD, ".edi")); os.IsNotExist(err) {
		os.Exit(0)
	}

	cfg := loadGuardConfig(input.CWD)
	if !cfg.Guard.Enabled {
		os.Exit(0)
	}

	// Pre-compile deny patterns once per invocation
	denyPatterns := compileDenyPatterns(cfg.Guard.DenyPatterns)

	switch input.HookEventName {
	case "PreToolUse":
		handlePreToolUse(input, cfg, denyPatterns)
	case "PostToolUse":
		handlePostToolUse(input)
	case "PostToolUseFailure":
		handlePostToolUseFailure(input)
	case "PreCompact":
		handlePreCompact(input, cfg)
	}
}

// ---------------------------------------------------------------------------
// Input parsing
// ---------------------------------------------------------------------------

func parseStdin() *hookInput {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return nil
	}
	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}
	if input.CWD == "" {
		input.CWD, _ = os.Getwd()
	}
	return &input
}

func parseBashInput(raw json.RawMessage) *bashToolInput {
	var b bashToolInput
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil
	}
	return &b
}

// ---------------------------------------------------------------------------
// Config loading
// ---------------------------------------------------------------------------

func loadGuardConfig(cwd string) *guardConfigFile {
	cfg := &guardConfigFile{
		Guard: config.DefaultConfig().Guard,
		Agent: "coder",
	}

	// Load global config into a separate overlay struct so an empty "guard:"
	// key doesn't zero out defaults.
	home, err := os.UserHomeDir()
	if err == nil {
		var global guardConfigOverlay
		if loadYAMLInto(filepath.Join(home, ".edi", "config.yaml"), &global) == nil {
			mergeGuardOverlay(cfg, &global)
		}
	}

	// Project config overrides global. Deny patterns are concatenated,
	// other arrays replace.
	var project guardConfigOverlay
	if loadYAMLInto(filepath.Join(cwd, ".edi", "config.yaml"), &project) == nil {
		mergeGuardOverlay(cfg, &project)
	}

	return cfg
}

// mergeGuardOverlay merges a config overlay into cfg. Only fields explicitly
// set in the overlay are applied. Enabled uses *bool so "not set" (nil) is
// distinguishable from "set to false."
func mergeGuardOverlay(cfg *guardConfigFile, overlay *guardConfigOverlay) {
	if overlay.Agent != "" {
		cfg.Agent = overlay.Agent
	}
	if overlay.Guard.Enabled != nil {
		cfg.Guard.Enabled = *overlay.Guard.Enabled
	}
	if len(overlay.Guard.BuildTags) > 0 {
		cfg.Guard.BuildTags = overlay.Guard.BuildTags
	}
	if overlay.Guard.FailureLoopThreshold > 0 {
		cfg.Guard.FailureLoopThreshold = overlay.Guard.FailureLoopThreshold
	}
	if len(overlay.Guard.DenyPatterns) > 0 {
		cfg.Guard.DenyPatterns = append(cfg.Guard.DenyPatterns, overlay.Guard.DenyPatterns...)
	}
}

func loadYAMLInto(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// ---------------------------------------------------------------------------
// PreToolUse: deny-list, build tags, failure counter
// ---------------------------------------------------------------------------

func handlePreToolUse(input *hookInput, cfg *guardConfigFile, denyPatterns []compiledDenyPattern) {
	if input.ToolName != "Bash" {
		return
	}
	bash := parseBashInput(input.ToolInput)
	if bash == nil || bash.Command == "" {
		return
	}

	// 1. Deny-list check (blocks on match)
	if reason := checkDenyList(bash.Command, denyPatterns); reason != "" {
		fmt.Fprintf(os.Stderr, "edi-guard: %s\n", reason)
		os.Exit(2)
	}

	// 2. Build tag injection (may modify command)
	modified, newCommand := injectBuildTags(bash.Command, cfg.Guard.BuildTags)

	// 3. Failure counter advisory (may add context)
	state := readState(input.SessionID)
	var advisory string
	threshold := cfg.Guard.FailureLoopThreshold
	if threshold <= 0 {
		threshold = 5
	}
	if state.ConsecutiveFailures >= threshold && !state.Advised {
		advisory = fmt.Sprintf(
			"edi-guard: %d consecutive Bash command failures detected. The last failure was: %q → %q. Consider stepping back to analyze the root cause rather than retrying with small variations.",
			state.ConsecutiveFailures, state.LastFailureCommand, state.LastFailureError,
		)
		state.Advised = true
		writeState(input.SessionID, state)
	}

	// Output response if anything changed
	if modified || advisory != "" {
		resp := buildPreToolUseResponse(newCommand, advisory, modified)
		data, err := json.Marshal(resp)
		if err == nil {
			fmt.Println(string(data))
		}
	}
}

// compiledDenyPattern pairs a compiled regex with its reason string.
type compiledDenyPattern struct {
	re     *regexp.Regexp
	reason string
}

// compileDenyPatterns compiles deny patterns once at startup, skipping invalid regexes.
func compileDenyPatterns(patterns []config.DenyPattern) []compiledDenyPattern {
	compiled := make([]compiledDenyPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "edi-guard: invalid deny pattern %q: %v\n", p.Pattern, err)
			continue
		}
		compiled = append(compiled, compiledDenyPattern{re: re, reason: p.Reason})
	}
	return compiled
}

// checkDenyList returns the reason string if the command matches any deny pattern.
func checkDenyList(command string, patterns []compiledDenyPattern) string {
	for _, p := range patterns {
		if p.re.MatchString(command) {
			return p.reason
		}
	}
	return ""
}

// goCommandRe matches go test/build/run anywhere in a string.
var goCommandRe = regexp.MustCompile(`\bgo\s+(test|build|run)\b`)

// clauseSplitRe splits shell commands on &&, ||, and ;
var clauseSplitRe = regexp.MustCompile(`\s*(&&|\|\||;)\s*`)

// injectBuildTags checks if any clause needs build tags injected.
// Returns (modified, newFullCommand).
func injectBuildTags(command string, tags []string) (bool, string) {
	if len(tags) == 0 {
		return false, command
	}

	// Split into clauses and delimiters
	delimiters := clauseSplitRe.FindAllString(command, -1)
	clauses := clauseSplitRe.Split(command, -1)

	anyModified := false
	for i, clause := range clauses {
		trimmed := strings.TrimSpace(clause)
		// Skip make commands
		if strings.HasPrefix(trimmed, "make ") || trimmed == "make" {
			continue
		}
		if goCommandRe.MatchString(trimmed) && !hasAllTags(trimmed, tags) {
			clauses[i] = injectTagsIntoClause(clause, tags)
			anyModified = true
		}
	}

	if !anyModified {
		return false, command
	}

	// Reassemble
	var b strings.Builder
	for i, clause := range clauses {
		b.WriteString(clause)
		if i < len(delimiters) {
			b.WriteString(delimiters[i])
		}
	}
	return true, b.String()
}

// hasAllTags checks if a command clause already contains all required tags.
func hasAllTags(clause string, tags []string) bool {
	for _, tag := range tags {
		re := regexp.MustCompile(`-tags[= ]+\S*\b` + regexp.QuoteMeta(tag) + `\b`)
		if !re.MatchString(clause) {
			return false
		}
	}
	return true
}

// injectTagsIntoClause inserts -tags "tag1,tag2" after the go subcommand.
func injectTagsIntoClause(clause string, tags []string) string {
	tagValue := strings.Join(tags, ",")
	var inject string
	if len(tags) > 1 {
		inject = fmt.Sprintf(` -tags "%s"`, tagValue)
	} else {
		inject = fmt.Sprintf(` -tags %s`, tagValue)
	}

	loc := goCommandRe.FindStringIndex(clause)
	if loc == nil {
		return clause
	}
	return clause[:loc[1]] + inject + clause[loc[1]:]
}

// buildPreToolUseResponse constructs the JSON response for PreToolUse.
func buildPreToolUseResponse(command, advisory string, modified bool) map[string]interface{} {
	hso := map[string]interface{}{
		"hookEventName":    "PreToolUse",
		"permissionDecision": "allow",
	}
	if modified {
		hso["updatedInput"] = map[string]string{
			"command": command,
		}
	}
	if advisory != "" {
		hso["additionalContext"] = advisory
	}
	return map[string]interface{}{
		"hookSpecificOutput": hso,
	}
}

// ---------------------------------------------------------------------------
// PostToolUse: reset failure counter on Bash success
// ---------------------------------------------------------------------------

func handlePostToolUse(input *hookInput) {
	if input.ToolName != "Bash" {
		return
	}
	state := readState(input.SessionID)
	if state.ConsecutiveFailures == 0 && !state.Advised {
		return // nothing to reset
	}
	writeState(input.SessionID, guardState{})
}

// ---------------------------------------------------------------------------
// PostToolUseFailure: increment failure counter
// ---------------------------------------------------------------------------

func handlePostToolUseFailure(input *hookInput) {
	if input.ToolName != "Bash" {
		return
	}
	bash := parseBashInput(input.ToolInput)
	state := readState(input.SessionID)
	state.ConsecutiveFailures++
	state.Advised = false // allow re-advisory on continued failures after a reset
	if bash != nil {
		state.LastFailureCommand = bash.Command
	}
	state.LastFailureError = input.Error
	writeState(input.SessionID, state)
}

// ---------------------------------------------------------------------------
// PreCompact: snapshot session state to /memories/compaction-state.md
// ---------------------------------------------------------------------------

func handlePreCompact(input *hookInput, cfg *guardConfigFile) {
	var lines []string
	lines = append(lines, "# Session State (auto-generated by edi-guard before compaction)")

	// Task
	if tasks := readActiveTasks(input.CWD); len(tasks) > 0 {
		for i, t := range tasks {
			if i >= 2 {
				remaining := len(tasks) - 2
				lines[len(lines)-1] += fmt.Sprintf(" (+%d more)", remaining)
				break
			}
			lines = append(lines, fmt.Sprintf("- Task: %s — %s", t.id, t.subject))
		}
	}

	// Git branch
	if branch := gitBranch(); branch != "" {
		lines = append(lines, fmt.Sprintf("- Branch: %s", branch))
	}

	// Build tags
	if len(cfg.Guard.BuildTags) > 0 {
		lines = append(lines, fmt.Sprintf("- Build tags required: %s", strings.Join(cfg.Guard.BuildTags, ", ")))
	}

	// Last test result
	state := readState(input.SessionID)
	if state.ConsecutiveFailures > 0 {
		lines = append(lines, fmt.Sprintf("- Last test result: %d consecutive failures (last: %q → %q)",
			state.ConsecutiveFailures, state.LastFailureCommand, state.LastFailureError))
	} else {
		lines = append(lines, "- Last test result: no recent failures")
	}

	// Agent mode
	if cfg.Agent != "" {
		lines = append(lines, fmt.Sprintf("- Agent mode: %s", cfg.Agent))
	}

	// Compaction trigger
	if input.Trigger != "" {
		lines = append(lines, fmt.Sprintf("- Compaction trigger: %s", input.Trigger))
	}

	// Write file
	dir := filepath.Join(input.CWD, "memories")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "edi-guard: failed to create memories dir: %v\n", err)
		return
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "compaction-state.md"), []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "edi-guard: failed to write compaction-state.md: %v\n", err)
	}
}

type taskInfo struct {
	id      string
	subject string
}

// readActiveTasks reads in-progress tasks from the manifest.
func readActiveTasks(cwd string) []taskInfo {
	data, err := os.ReadFile(filepath.Join(cwd, ".edi", "tasks", "active.yaml"))
	if err != nil {
		return nil
	}
	var manifest struct {
		Tasks []struct {
			ID      string `yaml:"id"`
			Subject string `yaml:"subject"`
			Status  string `yaml:"status"`
		} `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	var result []taskInfo
	for _, t := range manifest.Tasks {
		if t.Status == "in_progress" {
			result = append(result, taskInfo{id: t.ID, subject: t.Subject})
		}
	}
	return result
}

func gitBranch() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// State file operations
// ---------------------------------------------------------------------------

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
