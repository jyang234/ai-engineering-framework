package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/aef/codex/internal/core"
	"github.com/google/uuid"
)

// AgentRunner executes evaluation runs using a synthetic agent (Strategy C1).
// It calls the Anthropic Messages API in a multi-turn tool-use loop,
// proxying RECALL tools to a real MCP server.
type AgentRunner struct {
	resultsDB *ResultsDB
	taskDir   string
	logDir    string
}

// AgentRunConfig configures a single agent-mode evaluation run.
type AgentRunConfig struct {
	RunID      string // Optional: caller-assigned run ID (auto-generated if empty)
	Experiment string
	Condition  *Condition
	TaskID     string
	Complexity string
	Attempt    int
	Model      string
	TaskSpec   string
	Pitfalls   []PitfallSpec
}

// Turn limits by complexity (from spec).
var maxTurns = map[string]int{
	"simple":   25,
	"moderate": 35,
	"complex":  50,
}

// NewAgentRunner creates a synthetic agent runner.
func NewAgentRunner(resultsDB *ResultsDB, taskDir, logDir string) *AgentRunner {
	return &AgentRunner{
		resultsDB: resultsDB,
		taskDir:   taskDir,
		logDir:    logDir,
	}
}

// DiscoverTasks returns the exported task list for use by the CLI.
func (r *AgentRunner) DiscoverTasks() ([]TaskInfo, error) {
	entries, err := os.ReadDir(r.taskDir)
	if err != nil {
		return nil, err
	}
	var tasks []TaskInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		readmePath := filepath.Join(r.taskDir, taskID, "README.md")
		specData, err := os.ReadFile(readmePath)
		if err != nil {
			continue
		}
		info := TaskInfo{
			ID:   taskID,
			Spec: string(specData),
		}
		scoringPath := filepath.Join(r.taskDir, taskID, "scoring.yaml")
		if data, err := os.ReadFile(scoringPath); err == nil {
			info.Complexity = parseScoringComplexity(string(data))
		} else {
			info.Complexity = "moderate"
		}
		pitfallPath := filepath.Join(r.taskDir, taskID, "pitfalls.yaml")
		if data, err := os.ReadFile(pitfallPath); err == nil {
			info.Pitfalls = parsePitfalls(data)
		}
		tasks = append(tasks, info)
	}
	return tasks, nil
}

// RunAgent executes a single task using the synthetic agent loop.
// This is the CLI-facing method that delegates to RunTask.
func (r *AgentRunner) RunAgent(ctx context.Context, config *AgentRunConfig) (*EvalRun, error) {
	return r.RunTask(ctx, config)
}

// RunTask executes a single task using the synthetic agent loop.
func (r *AgentRunner) RunTask(ctx context.Context, config *AgentRunConfig) (*EvalRun, error) {
	runID := fmt.Sprintf("%s-%s-%s-%d-%s",
		config.Experiment, config.Condition.Name, config.TaskID,
		config.Attempt, uuid.New().String()[:8])

	run := &EvalRun{
		RunID:          runID,
		Experiment:     config.Experiment,
		Condition:      string(config.Condition.Name),
		TaskID:         config.TaskID,
		TaskComplexity: config.Complexity,
		Attempt:        config.Attempt,
		StartedAt:      time.Now().UTC(),
	}

	if err := r.resultsDB.InsertRun(run); err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	// Set up workspace
	workDir, err := r.setupWorkspace(config)
	if err != nil {
		return run, fmt.Errorf("setup workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Boot MCP server for RECALL (if condition includes it)
	var mcpClient *MCPClient
	if len(config.Condition.RECALLTools) > 0 {
		mcpClient, err = r.bootMCP(ctx, config, workDir)
		if err != nil {
			return run, fmt.Errorf("boot MCP: %w", err)
		}
		defer mcpClient.Close()

		// Seed RECALL with pitfall items
		if err := r.seedRECALL(ctx, mcpClient, config.Condition.RECALLSeeds); err != nil {
			log.Printf("[%s] Warning: seed RECALL failed: %v", runID, err)
		}
	}

	// Run the conversation loop
	dispatcher := NewToolDispatcher(mcpClient, workDir)
	loop := newConversationLoop(config, dispatcher, runID)

	log.Printf("[%s] Starting agent loop (condition=%s, task=%s, max_turns=%d)",
		runID, config.Condition.Name, config.TaskID, loop.maxTurns)

	startTime := time.Now()
	loopResult := loop.run(ctx)
	duration := time.Since(startTime)
	durMs := duration.Milliseconds()
	run.DurationMs = &durMs
	run.TurnsToComplete = &loopResult.turns
	run.TokensConsumed = &loopResult.totalTokens

	if loopResult.terminationReason != "" {
		run.TerminationReason = &loopResult.terminationReason
	}

	// Log the transcript
	r.writeTranscript(runID, loopResult)

	// Score the implementation
	log.Printf("[%s] Scoring...", runID)
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	scorer := NewScorer(apiKey, workDir)
	scoreResult, scoreErr := scorer.Score(ctx, config.TaskSpec, config.Pitfalls)
	if scoreErr != nil {
		log.Printf("[%s] Scoring error: %v", runID, scoreErr)
	} else {
		scorer.ApplyToRun(scoreResult, run)
	}

	// Track RECALL state
	if mcpClient != nil {
		r.trackRECALLState(runID, config.Condition.RECALLSeeds, loopResult)
	}

	if updateErr := r.resultsDB.UpdateRun(run); updateErr != nil {
		log.Printf("[%s] Update run error: %v", runID, updateErr)
	}

	log.Printf("[%s] Complete: turns=%d tokens=%d tests_pass=%v judge_combined=%v duration=%v",
		runID, loopResult.turns, loopResult.totalTokens, run.TestsPass, run.JudgeCombined, duration)

	return run, nil
}

func (r *AgentRunner) setupWorkspace(config *AgentRunConfig) (string, error) {
	workDir, err := os.MkdirTemp("", fmt.Sprintf("eval-agent-%s-*", config.TaskID))
	if err != nil {
		return "", err
	}

	taskSrc := filepath.Join(r.taskDir, config.TaskID)
	if err := copyDir(taskSrc, workDir); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("copy task: %w", err)
	}

	return workDir, nil
}

func (r *AgentRunner) bootMCP(ctx context.Context, config *AgentRunConfig, workDir string) (*MCPClient, error) {
	tmpDir, err := os.MkdirTemp("", "eval-mcp-*")
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(tmpDir, "eval-recall.db")
	engineConfig := core.Config{
		MetadataDBPath:      dbPath,
		LocalEmbeddingURL:   os.Getenv("LOCAL_EMBEDDING_URL"),
		LocalEmbeddingModel: os.Getenv("LOCAL_EMBEDDING_MODEL"),
		ScoreThreshold:      0,
	}

	engine, err := core.NewSearchEngine(ctx, engineConfig)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("create search engine: %w", err)
	}

	client := NewMCPClient(engine, "eval-"+config.TaskID)
	if err := client.Initialize(ctx); err != nil {
		engine.Close()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("MCP init: %w", err)
	}

	return client, nil
}

func (r *AgentRunner) seedRECALL(ctx context.Context, client *MCPClient, seeds []RECALLSeed) error {
	for _, seed := range seeds {
		doc := TestDocument{
			Type:    seed.Type,
			Title:   seed.Title,
			Content: seed.Content,
			Tags:    seed.Tags,
			Scope:   "project",
		}
		if _, err := client.RecallAdd(ctx, doc); err != nil {
			return fmt.Errorf("seed %q: %w", seed.Title, err)
		}
		log.Printf("Seeded RECALL: %s (%s)", seed.Title, seed.Type)
	}
	return nil
}

func (r *AgentRunner) writeTranscript(runID string, result *conversationResult) {
	if r.logDir == "" {
		return
	}

	logDir := filepath.Join(r.logDir, runID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	data, _ := json.MarshalIndent(result.messages, "", "  ")
	_ = os.WriteFile(filepath.Join(logDir, "transcript.json"), data, 0644)

	meta := map[string]interface{}{
		"turns":              result.turns,
		"total_tokens":       result.totalTokens,
		"termination_reason": result.terminationReason,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(logDir, "meta.json"), metaData, 0644)
}

func (r *AgentRunner) trackRECALLState(runID string, seeds []RECALLSeed, result *conversationResult) {
	for _, seed := range seeds {
		state := &EvalRecallState{
			RunID:     runID,
			ItemType:  seed.Type,
			ItemTitle: seed.Title,
			ItemID:    seed.Title, // Use title as proxy ID since MCP assigns real IDs
		}

		// Check if any search result referenced this seed
		for _, msg := range result.messages {
			msgStr := fmt.Sprintf("%v", msg)
			if containsIgnoreCase(msgStr, seed.Title) {
				state.WasRetrieved = true
				break
			}
		}

		_ = r.resultsDB.InsertRecallState(state)
	}
}

// conversationLoop manages the multi-turn tool-use conversation.
type conversationLoop struct {
	anthropic    *AnthropicClient
	dispatcher   *ToolDispatcher
	messages     []map[string]interface{}
	tools        []map[string]interface{}
	maxTurns     int
	model        string
	systemPrompt string
	runID        string

	// Loop detection
	lastToolCall string
	repeatCount  int
}

type conversationResult struct {
	messages          []map[string]interface{}
	turns             int
	totalTokens       int
	terminationReason string
}

func newConversationLoop(config *AgentRunConfig, dispatcher *ToolDispatcher, runID string) *conversationLoop {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	model := config.Model
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	mt := maxTurns[config.Complexity]
	if mt == 0 {
		mt = 35
	}

	includeRECALL := len(config.Condition.RECALLTools) > 0

	return &conversationLoop{
		anthropic:    NewAnthropicClient(apiKey),
		dispatcher:   dispatcher,
		tools:        ToolDefinitions(includeRECALL),
		maxTurns:     mt,
		model:        model,
		systemPrompt: config.Condition.SystemPrompt,
		runID:        runID,
		messages: []map[string]interface{}{
			{"role": "user", "content": config.TaskSpec},
		},
	}
}

func (l *conversationLoop) run(ctx context.Context) *conversationResult {
	result := &conversationResult{}

	timeout := time.Duration(l.maxTurns) * 30 * time.Second
	if timeout > 20*time.Minute {
		timeout = 20 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for turn := 0; turn < l.maxTurns; turn++ {
		select {
		case <-ctx.Done():
			result.terminationReason = "timeout"
			break
		default:
		}

		if result.terminationReason != "" {
			break
		}

		// Call Messages API
		resp, err := l.callAPI(ctx)
		if err != nil {
			log.Printf("[%s] API error on turn %d: %v", l.runID, turn, err)
			result.terminationReason = "api_error"
			break
		}

		result.turns = turn + 1
		result.totalTokens += resp.inputTokens + resp.outputTokens

		// Process response content blocks
		var toolResults []map[string]interface{}
		hasToolUse := false

		for _, block := range resp.content {
			switch block.blockType {
			case "text":
				// Model is thinking/responding — no action needed
			case "tool_use":
				hasToolUse = true

				// Loop detection
				callSig := block.toolName + ":" + string(block.toolInput)
				if callSig == l.lastToolCall {
					l.repeatCount++
				} else {
					l.lastToolCall = callSig
					l.repeatCount = 0
				}

				if l.repeatCount >= 3 {
					// Inject a nudge
					toolResults = append(toolResults, map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": block.toolID,
						"content":     "You appear to be in a loop. Please finish your work.",
						"is_error":    true,
					})
					continue
				}

				// Dispatch tool call
				tr := l.dispatcher.Dispatch(ctx, block.toolName, block.toolInput)
				toolResults = append(toolResults, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": block.toolID,
					"content":     tr.Content,
					"is_error":    tr.IsError,
				})
			}
		}

		// Append assistant message
		l.messages = append(l.messages, resp.rawAssistant)

		// If the model used tools, send results back
		if hasToolUse && len(toolResults) > 0 {
			l.messages = append(l.messages, map[string]interface{}{
				"role":    "user",
				"content": toolResults,
			})
		}

		// If stop reason is end_turn, the model is done
		if resp.stopReason == "end_turn" {
			break
		}
	}

	if result.terminationReason == "" && result.turns >= l.maxTurns {
		result.terminationReason = "max_turns"
	}

	result.messages = l.messages
	return result
}

// apiResponse holds the parsed response from the Messages API.
type apiResponse struct {
	content      []contentBlock
	stopReason   string
	inputTokens  int
	outputTokens int
	rawAssistant map[string]interface{}
}

type contentBlock struct {
	blockType string
	text      string
	toolID    string
	toolName  string
	toolInput json.RawMessage
}

func (l *conversationLoop) callAPI(ctx context.Context) (*apiResponse, error) {
	reqBody := map[string]interface{}{
		"model":      l.model,
		"max_tokens": 4096,
		"messages":   l.messages,
		"tools":      l.tools,
	}
	if l.systemPrompt != "" {
		reqBody["system"] = l.systemPrompt
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// Use RawJudge's retry infrastructure but we need the full response.
	// Call the API directly with retry logic.
	text, err := l.callMessagesAPI(ctx, body)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	resp := &apiResponse{}

	// Parse stop_reason
	if sr, ok := raw["stop_reason"].(string); ok {
		resp.stopReason = sr
	}

	// Parse usage
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		if it, ok := usage["input_tokens"].(float64); ok {
			resp.inputTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok {
			resp.outputTokens = int(ot)
		}
	}

	// Parse content blocks
	if content, ok := raw["content"].([]interface{}); ok {
		for _, c := range content {
			block, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			cb := contentBlock{}
			if t, ok := block["type"].(string); ok {
				cb.blockType = t
			}
			switch cb.blockType {
			case "text":
				if t, ok := block["text"].(string); ok {
					cb.text = t
				}
			case "tool_use":
				if id, ok := block["id"].(string); ok {
					cb.toolID = id
				}
				if name, ok := block["name"].(string); ok {
					cb.toolName = name
				}
				if input, ok := block["input"]; ok {
					inputBytes, _ := json.Marshal(input)
					cb.toolInput = inputBytes
				}
			}
			resp.content = append(resp.content, cb)
		}
	}

	// Build raw assistant message for conversation history
	resp.rawAssistant = map[string]interface{}{
		"role":    "assistant",
		"content": raw["content"],
	}

	return resp, nil
}

func (l *conversationLoop) callMessagesAPI(ctx context.Context, body []byte) (string, error) {
	return l.anthropic.RawHTTPPost(ctx, body)
}

func containsIgnoreCase(s, substr string) bool {
	return len(substr) > 0 && strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
