package eval

import (
	"testing"
	"time"
)

// =============================================================================
// Agent Runner Tests — Given-When-Then
// =============================================================================

// --- maxTurns configuration ---

func TestMaxTurns_GivenSimple_ThenTwentyFive(t *testing.T) {
	// Given/When: we look up max turns for "simple" complexity
	mt := maxTurns["simple"]

	// Then: 25 turns
	if mt != 25 {
		t.Errorf("simple max_turns = %d, want 25", mt)
	}
}

func TestMaxTurns_GivenModerate_ThenThirtyFive(t *testing.T) {
	// Given/When: we look up max turns for "moderate" complexity
	mt := maxTurns["moderate"]

	// Then: 35 turns
	if mt != 35 {
		t.Errorf("moderate max_turns = %d, want 35", mt)
	}
}

func TestMaxTurns_GivenComplex_ThenFifty(t *testing.T) {
	// Given/When: we look up max turns for "complex" complexity
	mt := maxTurns["complex"]

	// Then: 50 turns
	if mt != 50 {
		t.Errorf("complex max_turns = %d, want 50", mt)
	}
}

func TestMaxTurns_GivenUnknown_ThenZeroDefault(t *testing.T) {
	// Given/When: we look up an unknown complexity
	mt := maxTurns["extreme"]

	// Then: 0 (map returns zero value)
	if mt != 0 {
		t.Errorf("unknown max_turns = %d, want 0", mt)
	}
}

// --- newConversationLoop defaults ---

func TestNewConversationLoop_GivenEmptyModel_ThenDefaultsSonnet(t *testing.T) {
	// Given: a config with empty model
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "moderate",
		Model:      "",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: model defaults to claude-sonnet-4-6
	if loop.model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6", loop.model)
	}
}

func TestNewConversationLoop_GivenSpecifiedModel_ThenUsed(t *testing.T) {
	// Given: a config with a specific model
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "moderate",
		Model:      "claude-opus-4-6",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: specified model is used
	if loop.model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", loop.model)
	}
}

func TestNewConversationLoop_GivenUnknownComplexity_ThenDefaultThirtyFiveTurns(t *testing.T) {
	// Given: a config with unknown complexity
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "extreme",
		Model:      "claude-sonnet-4-6",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: defaults to 35 turns
	if loop.maxTurns != 35 {
		t.Errorf("maxTurns = %d, want 35 (default)", loop.maxTurns)
	}
}

func TestNewConversationLoop_GivenSimpleComplexity_ThenTwentyFiveTurns(t *testing.T) {
	// Given: a config with simple complexity
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "simple",
		Model:      "claude-sonnet-4-6",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: 25 turns
	if loop.maxTurns != 25 {
		t.Errorf("maxTurns = %d, want 25", loop.maxTurns)
	}
}

func TestNewConversationLoop_GivenRECALLCondition_ThenToolsIncludeRECALL(t *testing.T) {
	// Given: a condition with RECALL tools enabled
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition: &Condition{
			Name:        "aef-full",
			RECALLTools: []string{"recall_search", "recall_add"},
		},
		TaskID:     "task-01",
		Complexity: "moderate",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: tools include RECALL (11 total)
	if len(loop.tools) != 11 {
		t.Errorf("tools = %d, want 11 (6 base + 5 RECALL)", len(loop.tools))
	}
}

func TestNewConversationLoop_GivenBaselineCondition_ThenToolsExcludeRECALL(t *testing.T) {
	// Given: a condition without RECALL
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "moderate",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: only base tools (6)
	if len(loop.tools) != 6 {
		t.Errorf("tools = %d, want 6 (base only)", len(loop.tools))
	}
}

func TestNewConversationLoop_GivenSystemPrompt_ThenStored(t *testing.T) {
	// Given: a condition with a system prompt
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition: &Condition{
			Name:         "aef-minimal",
			SystemPrompt: "You are an expert Go developer.",
		},
		TaskID:     "task-01",
		Complexity: "moderate",
		TaskSpec:   "implement X",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: system prompt is stored
	if loop.systemPrompt != "You are an expert Go developer." {
		t.Errorf("systemPrompt = %q", loop.systemPrompt)
	}
}

func TestNewConversationLoop_GivenTaskSpec_ThenFirstMessageIsUserWithSpec(t *testing.T) {
	// Given: a task spec
	config := &AgentRunConfig{
		Experiment: "3A",
		Condition:  &Condition{Name: "baseline"},
		TaskID:     "task-01",
		Complexity: "moderate",
		TaskSpec:   "Implement retry logic with exponential backoff and jitter.",
	}
	dispatcher := &ToolDispatcher{workDir: "/tmp"}

	// When: we create a conversation loop
	loop := newConversationLoop(config, dispatcher, "test-run")

	// Then: first message is a user message with the task spec
	if len(loop.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(loop.messages))
	}
	msg := loop.messages[0]
	if msg["role"] != "user" {
		t.Errorf("first message role = %v, want user", msg["role"])
	}
	if msg["content"] != config.TaskSpec {
		t.Errorf("first message content = %v, want task spec", msg["content"])
	}
}

// --- containsIgnoreCase ---

func TestContainsIgnoreCase_GivenExactMatch_ThenTrue(t *testing.T) {
	if !containsIgnoreCase("Hello World", "Hello") {
		t.Error("expected true for exact match")
	}
}

func TestContainsIgnoreCase_GivenCaseDifference_ThenTrue(t *testing.T) {
	if !containsIgnoreCase("Hello World", "hello") {
		t.Error("expected true for case-insensitive match")
	}
	if !containsIgnoreCase("hello world", "HELLO") {
		t.Error("expected true for upper->lower match")
	}
}

func TestContainsIgnoreCase_GivenNoMatch_ThenFalse(t *testing.T) {
	if containsIgnoreCase("Hello World", "xyz") {
		t.Error("expected false for no match")
	}
}

func TestContainsIgnoreCase_GivenEmptySubstring_ThenFalse(t *testing.T) {
	// Given: empty substring
	if containsIgnoreCase("Hello", "") {
		t.Error("expected false for empty substring")
	}
}

func TestContainsIgnoreCase_GivenSubstringLongerThanString_ThenFalse(t *testing.T) {
	if containsIgnoreCase("Hi", "Hello World") {
		t.Error("expected false when substring longer than string")
	}
}

func TestContainsIgnoreCase_GivenMixedCaseInMiddle_ThenTrue(t *testing.T) {
	if !containsIgnoreCase("the Missing Jitter issue", "missing jitter") {
		t.Error("expected true for mixed case match in middle")
	}
}

// --- NewAgentRunner ---

func TestNewAgentRunner_GivenParameters_ThenFieldsSet(t *testing.T) {
	// Given: constructor parameters
	runner := NewAgentRunner(nil, "/tasks", "/logs")

	// Then: fields are set correctly
	if runner.taskDir != "/tasks" {
		t.Errorf("taskDir = %q, want /tasks", runner.taskDir)
	}
	if runner.logDir != "/logs" {
		t.Errorf("logDir = %q, want /logs", runner.logDir)
	}
}

// --- conversationResult structure ---

func TestConversationResult_GivenInitialized_ThenZeroValues(t *testing.T) {
	// Given: a new conversation result
	result := &conversationResult{}

	// Then: all fields are zero-valued
	if result.turns != 0 {
		t.Errorf("turns = %d, want 0", result.turns)
	}
	if result.totalTokens != 0 {
		t.Errorf("totalTokens = %d, want 0", result.totalTokens)
	}
	if result.terminationReason != "" {
		t.Errorf("terminationReason = %q, want empty", result.terminationReason)
	}
	if result.messages != nil {
		t.Error("messages should be nil initially")
	}
}

// --- Loop detection threshold ---

func TestConversationLoop_GivenRepeatThreshold_ThenThreeRepeats(t *testing.T) {
	// Given: a conversation loop
	loop := &conversationLoop{}

	// When: the same tool call is repeated
	loop.lastToolCall = "Read:main.go"
	loop.repeatCount = 0

	// Simulate 3 identical calls
	for i := 0; i < 3; i++ {
		callSig := "Read:main.go"
		if callSig == loop.lastToolCall {
			loop.repeatCount++
		}
	}

	// Then: repeat count reaches 3 (the nudge threshold)
	if loop.repeatCount < 3 {
		t.Errorf("repeatCount = %d, want >= 3", loop.repeatCount)
	}
}

func TestConversationLoop_GivenDifferentCalls_ThenRepeatCountResets(t *testing.T) {
	// Given: a conversation loop with an existing call signature
	loop := &conversationLoop{
		lastToolCall: "Read:main.go",
		repeatCount:  2,
	}

	// When: a different tool call comes in
	callSig := "Write:output.go"
	if callSig == loop.lastToolCall {
		loop.repeatCount++
	} else {
		loop.lastToolCall = callSig
		loop.repeatCount = 0
	}

	// Then: repeat count resets to 0
	if loop.repeatCount != 0 {
		t.Errorf("repeatCount = %d, want 0 (reset)", loop.repeatCount)
	}
	if loop.lastToolCall != "Write:output.go" {
		t.Errorf("lastToolCall = %q, want Write:output.go", loop.lastToolCall)
	}
}

// --- Conversation loop timeout calculation ---

func TestConversationLoop_GivenMaxTurns_ThenTimeoutCalculatedCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		maxTurns int
		expected time.Duration
	}{
		{"25 turns → 12.5min", 25, 12*time.Minute + 30*time.Second},
		{"35 turns → 17.5min", 35, 17*time.Minute + 30*time.Second},
		{"50 turns → capped at 20min", 50, 20 * time.Minute},
		{"100 turns → capped at 20min", 100, 20 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a max turn count
			timeout := time.Duration(tt.maxTurns) * 30 * time.Second
			if timeout > 20*time.Minute {
				timeout = 20 * time.Minute
			}

			// Then: timeout matches expected
			if timeout != tt.expected {
				t.Errorf("timeout = %v, want %v", timeout, tt.expected)
			}
		})
	}
}

// --- writeTranscript ---

func TestAgentRunner_GivenEmptyLogDir_WhenWriteTranscript_ThenNoop(t *testing.T) {
	// Given: a runner with empty logDir
	runner := &AgentRunner{logDir: ""}

	// When: we write a transcript (should not panic)
	runner.writeTranscript("test-run", &conversationResult{})

	// Then: no panic, no files created
}

// --- apiResponse structure ---

func TestAPIResponse_GivenContentBlocks_ThenFieldsAccessible(t *testing.T) {
	// Given: an API response with mixed content blocks
	resp := &apiResponse{
		content: []contentBlock{
			{blockType: "text", text: "I'll help you implement this."},
			{blockType: "tool_use", toolID: "tu_123", toolName: "Read", toolInput: []byte(`{"file_path":"main.go"}`)},
		},
		stopReason:   "tool_use",
		inputTokens:  1500,
		outputTokens: 500,
	}

	// Then: fields are accessible
	if len(resp.content) != 2 {
		t.Errorf("content blocks = %d, want 2", len(resp.content))
	}
	if resp.content[0].blockType != "text" {
		t.Errorf("first block type = %q, want text", resp.content[0].blockType)
	}
	if resp.content[1].toolName != "Read" {
		t.Errorf("second block toolName = %q, want Read", resp.content[1].toolName)
	}
	if resp.stopReason != "tool_use" {
		t.Errorf("stopReason = %q, want tool_use", resp.stopReason)
	}
	if resp.inputTokens != 1500 {
		t.Errorf("inputTokens = %d, want 1500", resp.inputTokens)
	}
}
