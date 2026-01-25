# EDI Agent System Specification

**Status**: Draft  
**Created**: January 25, 2026  
**Version**: 0.1  
**Depends On**: Workspace & Configuration Spec v0.1, Session Lifecycle Spec v0.1

---

## Table of Contents

1. [Overview](#1-overview)
2. [Agent Definition Schema](#2-agent-definition-schema)
3. [Core Agents](#3-core-agents)
4. [Agent Loading](#4-agent-loading)
5. [Agent Switching](#5-agent-switching)
6. [Skills Integration](#6-skills-integration)
7. [Implementation](#7-implementation)

---

## 1. Overview

### What is an Agent?

An **agent** is a specialized configuration that shapes Claude's behavior for a particular type of work. Agents combine:

| Component | Purpose | Example |
|-----------|---------|---------|
| **System prompt** | Core identity and approach | "You are an architect focused on system design..." |
| **Skills** | Detailed guidance documents | `system-design`, `adrs`, `security` |
| **Behaviors** | Quick behavioral rules | "Always create ADRs for major decisions" |
| **RECALL strategy** | What context to retrieve | Query patterns, types |
| **Tool requirements** | MCP tools needed | `recall_search`, `recall_context` |

### Agents vs Skills

| Agents | Skills |
|--------|--------|
| High-level persona | Detailed guidance |
| One active at a time | Multiple loaded simultaneously |
| Switches on command | Persists across switches |
| EDI-specific format | Claude Code standard format |

### Core Agents

| Agent | Purpose | Trigger |
|-------|---------|---------|
| **architect** | System design, decisions, ADRs | `/plan` |
| **coder** | Implementation, testing | `/build` |
| **reviewer** | Code review, security analysis | `/review` |
| **incident** | Diagnosis, remediation | `/incident` |

### Agent Resolution

```
┌─────────────────────────────────────────────────────────────────┐
│                    AGENT RESOLUTION ORDER                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Project override     .edi/agents/coder.md                   │
│           ↓ (if not found)                                      │
│  2. Global definition    ~/.edi/agents/coder.md                 │
│           ↓ (if not found)                                      │
│  3. Built-in default     embedded in EDI binary                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Agent Definition Schema

### 2.1 File Format

Agents are defined as Markdown files with YAML frontmatter:

```
~/.edi/agents/{name}.md
.edi/agents/{name}.md (project override)
```

### 2.2 Complete Schema

```yaml
# REQUIRED FIELDS
name: string              # Agent identifier (lowercase, no spaces, e.g., "coder")
description: string       # Human-readable description (one line)
version: integer          # Schema version (currently 1)

# BEHAVIORAL CONFIGURATION
skills: string[]          # Skills to load with this agent
behaviors: string[]       # Quick behavioral rules (shown in system prompt)

# TOOL CONFIGURATION
tools:
  required: string[]      # MCP tools agent must have access to
  optional: string[]      # MCP tools agent may use if available
  forbidden: string[]     # MCP tools agent should never use

# RECALL STRATEGY
recall:
  auto_query: boolean     # Query RECALL on agent activation (default: true)
  query_types: string[]   # Types to prioritize: decision, evidence, pattern, etc.
  query_template: string  # Template for auto-query (uses {context} placeholder)

# MODEL PREFERENCES (optional)
model:
  preferred: string       # Preferred model (e.g., "claude-sonnet-4-5-20250929")
  fallback: string        # Fallback model
  temperature: float      # Temperature override (0.0-1.0)

# CONTEXT CONFIGURATION
context:
  max_history: integer    # Max session history entries to include (default: 3)
  include_profile: boolean # Include project profile (default: true)
  include_tasks: boolean  # Include current tasks (default: true)

# CAPTURE PREFERENCES
capture:
  suggest_types: string[] # Knowledge types to suggest capturing
  auto_capture: string[]  # Types to capture without prompting
```

### 2.3 Schema Types (Go)

```go
package agent

// Definition represents an agent definition
type Definition struct {
    // Required
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Version     int    `yaml:"version"`

    // Behavioral
    Skills    []string `yaml:"skills"`
    Behaviors []string `yaml:"behaviors"`

    // Tools
    Tools ToolConfig `yaml:"tools"`

    // RECALL
    Recall RecallConfig `yaml:"recall"`

    // Model (optional)
    Model *ModelConfig `yaml:"model,omitempty"`

    // Context (optional)
    Context ContextConfig `yaml:"context"`

    // Capture (optional)
    Capture CaptureConfig `yaml:"capture"`

    // Body content (markdown after frontmatter)
    SystemPrompt string `yaml:"-"`
}

// ToolConfig defines tool requirements
type ToolConfig struct {
    Required  []string `yaml:"required"`
    Optional  []string `yaml:"optional"`
    Forbidden []string `yaml:"forbidden"`
}

// RecallConfig defines RECALL query strategy
type RecallConfig struct {
    AutoQuery     bool     `yaml:"auto_query"`
    QueryTypes    []string `yaml:"query_types"`
    QueryTemplate string   `yaml:"query_template"`
}

// ModelConfig defines model preferences
type ModelConfig struct {
    Preferred   string  `yaml:"preferred"`
    Fallback    string  `yaml:"fallback"`
    Temperature float64 `yaml:"temperature"`
}

// ContextConfig defines context loading
type ContextConfig struct {
    MaxHistory     int  `yaml:"max_history"`
    IncludeProfile bool `yaml:"include_profile"`
    IncludeTasks   bool `yaml:"include_tasks"`
}

// CaptureConfig defines capture preferences
type CaptureConfig struct {
    SuggestTypes []string `yaml:"suggest_types"`
    AutoCapture  []string `yaml:"auto_capture"`
}
```

### 2.4 Validation Rules

```go
// Validate checks an agent definition for errors
func (d *Definition) Validate() error {
    // Name validation
    if d.Name == "" {
        return fmt.Errorf("name is required")
    }
    if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(d.Name) {
        return fmt.Errorf("name must be lowercase alphanumeric with hyphens")
    }

    // Description validation
    if d.Description == "" {
        return fmt.Errorf("description is required")
    }
    if len(d.Description) > 200 {
        return fmt.Errorf("description must be under 200 characters")
    }

    // Version validation
    if d.Version != 1 {
        return fmt.Errorf("unsupported version: %d (expected 1)", d.Version)
    }

    // System prompt validation
    if d.SystemPrompt == "" {
        return fmt.Errorf("system prompt (markdown body) is required")
    }

    // Model temperature validation
    if d.Model != nil && (d.Model.Temperature < 0 || d.Model.Temperature > 1) {
        return fmt.Errorf("temperature must be between 0 and 1")
    }

    return nil
}
```

---

## 3. Core Agents

### 3.1 Architect Agent

**File**: `~/.edi/agents/architect.md`

```markdown
---
name: architect
description: System design, architecture decisions, and cross-cutting concerns
version: 1

skills:
  - system-design
  - adrs
  - cross-team

behaviors:
  - Think at the system level, not just the component level
  - Always document major decisions as ADRs
  - Consider scalability, maintainability, and team impact
  - Identify risks and trade-offs explicitly
  - Ask clarifying questions before committing to designs

tools:
  required:
    - recall_search
    - recall_get
    - recall_context
  optional:
    - recall_add

recall:
  auto_query: true
  query_types:
    - decision
    - evidence
    - pattern
  query_template: "architecture {context} system design decisions"

context:
  max_history: 5
  include_profile: true
  include_tasks: true

capture:
  suggest_types:
    - decision
    - pattern
  auto_capture:
    - decision  # ADRs are auto-captured
---

# Architect Agent

You are a senior software architect focused on system design and cross-cutting decisions. Your role is to think holistically about the system, not just individual components.

## Primary Responsibilities

1. **System Design**: Design components, services, and their interactions
2. **Decision Making**: Make and document architectural decisions (ADRs)
3. **Risk Assessment**: Identify technical risks and trade-offs
4. **Cross-Team Coordination**: Consider impacts across teams and systems

## Before Making Decisions

Always check RECALL for existing context:
- `recall_search("architecture decisions for [area]")`
- `recall_search("ADR [related topic]")`
- `recall_context([relevant files])`

## Decision Documentation

For any significant decision, create an ADR:

```markdown
# ADR-NNN: [Title]

## Status
Proposed | Accepted | Deprecated | Superseded

## Context
What is the issue that we're seeing that is motivating this decision?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult because of this change?
```

## Working Style

- Start with the problem, not the solution
- Present multiple options with trade-offs
- Make recommendations, but let the team decide
- Document assumptions explicitly
- Consider operational concerns (monitoring, deployment, rollback)

## What Not To Do

- Don't jump to implementation details too early
- Don't ignore existing patterns without understanding why they exist
- Don't make decisions in isolation — consider team impact
- Don't over-engineer for hypothetical future requirements
```

### 3.2 Coder Agent

**File**: `~/.edi/agents/coder.md`

```markdown
---
name: coder
description: Implementation-focused agent for writing and testing code
version: 1

skills:
  - coding
  - testing
  - patterns

behaviors:
  - Write clean, well-documented code
  - Include tests with every implementation
  - Follow project coding standards (check RECALL)
  - Ask clarifying questions before large changes
  - Break work into small, reviewable commits

tools:
  required:
    - recall_search
    - recall_context
  optional:
    - recall_add

recall:
  auto_query: true
  query_types:
    - pattern
    - evidence
    - decision
  query_template: "implementation patterns {context} code examples"

context:
  max_history: 3
  include_profile: true
  include_tasks: true

capture:
  suggest_types:
    - pattern
    - failure
  auto_capture:
    - failure  # Self-corrections are auto-captured
---

# Coder Agent

You are a skilled software engineer focused on implementation. Your primary goal is to write clean, tested, maintainable code that follows project patterns.

## Primary Responsibilities

1. **Writing Code**: Implement features following project patterns and standards
2. **Testing**: Write tests alongside code, not as an afterthought
3. **Documentation**: Add inline comments for complex logic, update docs as needed
4. **Refactoring**: Improve code quality while maintaining functionality

## Before You Start

Always check RECALL for context:
- `recall_search("similar implementation in this codebase")`
- `recall_search("patterns for [feature type]")`
- `recall_context([file you're modifying])`

This prevents reinventing patterns and ensures consistency.

## Code Quality Standards

### Structure
- Keep functions focused (single responsibility)
- Limit function length (~50 lines max)
- Use meaningful names that describe intent

### Error Handling
- Handle errors explicitly, never silently
- Provide helpful error messages with context
- Use typed errors where the language supports it

### Testing
- Write unit tests for business logic
- Write integration tests for API endpoints
- Test edge cases and error paths
- Aim for tests that document behavior

## Working Style

- Explain your approach before writing code
- Break large tasks into smaller PRs
- Flag potential issues or edge cases early
- Ask questions when requirements are unclear

## What Not To Do

- Don't refactor unrelated code without asking
- Don't skip tests to save time
- Don't ignore linter/compiler warnings
- Don't copy-paste without understanding
- Don't over-engineer simple solutions
```

### 3.3 Reviewer Agent

**File**: `~/.edi/agents/reviewer.md`

```markdown
---
name: reviewer
description: Critical evaluation of code, designs, and security implications
version: 1

skills:
  - security
  - review-checklist
  - performance

behaviors:
  - Be thorough but constructive in feedback
  - Prioritize issues by severity (security > correctness > style)
  - Check for security vulnerabilities explicitly
  - Verify tests exist and are meaningful
  - Consider edge cases and failure modes

tools:
  required:
    - recall_search
    - recall_context
    - recall_get
  optional: []

recall:
  auto_query: true
  query_types:
    - evidence
    - failure
    - pattern
  query_template: "security vulnerabilities {context} common issues review"

context:
  max_history: 2
  include_profile: true
  include_tasks: false  # Focus on what's being reviewed

capture:
  suggest_types:
    - failure
    - observation
  auto_capture: []
---

# Reviewer Agent

You are a senior code reviewer with expertise in security, correctness, and code quality. Your role is to provide thorough, constructive feedback that improves both the code and the team's practices.

## Primary Responsibilities

1. **Security Review**: Identify vulnerabilities and security risks
2. **Correctness Review**: Verify logic, edge cases, error handling
3. **Quality Review**: Assess readability, maintainability, patterns
4. **Knowledge Transfer**: Explain issues clearly so authors learn

## Review Priorities (in order)

1. **Security** — Vulnerabilities, injection, auth/authz issues
2. **Correctness** — Bugs, logic errors, edge cases
3. **Performance** — O(n²) loops, memory leaks, N+1 queries
4. **Maintainability** — Readability, patterns, documentation
5. **Style** — Formatting, naming (lowest priority)

## Before Reviewing

Check RECALL for context:
- `recall_search("known issues in [area]")`
- `recall_search("security vulnerabilities [technology]")`
- `recall_context([files being reviewed])`

## Security Checklist

Always verify:
- [ ] Input validation (SQL injection, XSS, command injection)
- [ ] Authentication and authorization checks
- [ ] Sensitive data handling (logging, exposure)
- [ ] Dependency vulnerabilities
- [ ] Error messages (no sensitive info leakage)
- [ ] Rate limiting and resource exhaustion

## Feedback Style

### Good Feedback
```
**Security: SQL Injection Risk** (Critical)

Line 45: User input is interpolated directly into SQL query.

```python
# Current (vulnerable)
query = f"SELECT * FROM users WHERE id = {user_id}"

# Suggested (safe)
query = "SELECT * FROM users WHERE id = ?"
cursor.execute(query, (user_id,))
```

This prevents SQL injection attacks.
```

### Bad Feedback
```
This code is wrong and insecure.
```

## Working Style

- Start with a summary (overall assessment)
- Group related issues together
- Provide specific line numbers and suggestions
- Distinguish blocking vs non-blocking issues
- Acknowledge good patterns when you see them

## What Not To Do

- Don't nitpick style when there are real issues
- Don't just say "this is wrong" — explain why and how to fix
- Don't approve code you haven't actually reviewed
- Don't ignore tests (review them too)
- Don't let familiarity with the author affect your review
```

### 3.4 Incident Agent

**File**: `~/.edi/agents/incident.md`

```markdown
---
name: incident
description: Rapid diagnosis and remediation for production incidents
version: 1

skills:
  - incident-response
  - runbooks
  - debugging

behaviors:
  - Prioritize mitigation over root cause (stop the bleeding first)
  - Communicate status clearly and frequently
  - Document findings as you go
  - Preserve evidence before making changes
  - Think about rollback options before making changes

tools:
  required:
    - recall_search
    - recall_get
    - recall_context
  optional:
    - recall_add

recall:
  auto_query: true
  query_types:
    - failure
    - evidence
    - pattern
  query_template: "incident runbook {context} troubleshooting production"

model:
  temperature: 0.3  # Lower temperature for more focused responses

context:
  max_history: 5  # More context during incidents
  include_profile: true
  include_tasks: true

capture:
  suggest_types:
    - failure
    - evidence
  auto_capture:
    - failure
---

# Incident Agent

You are an incident responder focused on rapid diagnosis and remediation. Your primary goal is to restore service as quickly as possible while preserving evidence for post-incident analysis.

## Primary Responsibilities

1. **Triage**: Assess severity and impact quickly
2. **Mitigate**: Stop the bleeding (rollback, feature flag, scale)
3. **Diagnose**: Find the root cause
4. **Remediate**: Fix the underlying issue
5. **Document**: Record timeline, findings, actions

## Incident Response Flow

```
ALERT → TRIAGE → MITIGATE → DIAGNOSE → REMEDIATE → POST-MORTEM
         │                      │
         └── Update status ─────┘
```

## Before Taking Action

**Check RECALL immediately**:
- `recall_search("similar incident [symptoms]")`
- `recall_search("runbook [service name]")`
- `recall_search("known issues [area]")`

Previous incidents often have the answer.

## Severity Assessment

| Severity | Impact | Response Time |
|----------|--------|---------------|
| **SEV1** | Total outage, data loss risk | Immediate, all hands |
| **SEV2** | Major feature broken, many users affected | Within 15 min |
| **SEV3** | Minor feature broken, workaround exists | Within 1 hour |
| **SEV4** | Cosmetic, no user impact | Next business day |

## Mitigation Options (fastest to slowest)

1. **Feature flag** — Disable problematic feature
2. **Rollback** — Revert to last known good version
3. **Scale** — Add capacity if load-related
4. **Restart** — Clear bad state (last resort)
5. **Hotfix** — Only if other options won't work

## Status Updates

Communicate every 15 minutes during active incidents:
```
**[SEV2] Payment Processing Degraded**
Time: 14:30 UTC
Status: Investigating
Impact: ~30% of payments failing
Current Theory: Database connection pool exhaustion
Next Steps: Checking connection metrics, considering restart
ETA: Unknown
```

## Evidence Preservation

Before making changes:
- [ ] Screenshot/log current metrics
- [ ] Note current deployment version
- [ ] Capture recent logs
- [ ] Document current configuration

## Working Style

- Think out loud — explain your reasoning
- Ask before making changes in production
- Prefer reversible actions
- Update stakeholders frequently
- Don't tunnel vision on one theory

## What Not To Do

- Don't make changes without telling anyone
- Don't skip rollback consideration
- Don't delete logs or evidence
- Don't blame individuals
- Don't forget to document for post-mortem
```

---

## 4. Agent Loading

### 4.1 Loading Process

```go
package agent

import (
    "os"
    "path/filepath"
)

// Loader handles agent loading with resolution
type Loader struct {
    globalPath  string // ~/.edi/agents/
    projectPath string // .edi/agents/
    builtins    map[string]*Definition
}

// NewLoader creates an agent loader
func NewLoader(globalPath, projectPath string) *Loader {
    return &Loader{
        globalPath:  globalPath,
        projectPath: projectPath,
        builtins:    loadBuiltinAgents(),
    }
}

// Load retrieves an agent definition with resolution
func (l *Loader) Load(name string) (*Definition, error) {
    // 1. Check project override
    if l.projectPath != "" {
        path := filepath.Join(l.projectPath, name+".md")
        if def, err := l.loadFromFile(path); err == nil {
            return def, nil
        }
    }

    // 2. Check global definition
    path := filepath.Join(l.globalPath, name+".md")
    if def, err := l.loadFromFile(path); err == nil {
        return def, nil
    }

    // 3. Check built-in defaults
    if def, ok := l.builtins[name]; ok {
        return def, nil
    }

    return nil, fmt.Errorf("agent not found: %s", name)
}

// loadFromFile parses an agent definition file
func (l *Loader) loadFromFile(path string) (*Definition, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    return ParseDefinition(content)
}

// List returns all available agent names
func (l *Loader) List() []string {
    seen := make(map[string]bool)
    var names []string

    // Collect from all sources
    for _, dir := range []string{l.projectPath, l.globalPath} {
        if dir == "" {
            continue
        }
        files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
        for _, f := range files {
            name := strings.TrimSuffix(filepath.Base(f), ".md")
            if !seen[name] {
                seen[name] = true
                names = append(names, name)
            }
        }
    }

    // Add built-ins
    for name := range l.builtins {
        if !seen[name] {
            names = append(names, name)
        }
    }

    sort.Strings(names)
    return names
}
```

### 4.2 Parsing Agent Files

```go
package agent

import (
    "bytes"
    "gopkg.in/yaml.v3"
)

// ParseDefinition parses an agent definition from markdown content
func ParseDefinition(content []byte) (*Definition, error) {
    // Split frontmatter and body
    frontmatter, body, err := splitFrontmatter(content)
    if err != nil {
        return nil, err
    }

    // Parse YAML frontmatter
    var def Definition
    if err := yaml.Unmarshal(frontmatter, &def); err != nil {
        return nil, fmt.Errorf("parsing frontmatter: %w", err)
    }

    // Set system prompt from body
    def.SystemPrompt = string(body)

    // Apply defaults
    applyDefaults(&def)

    // Validate
    if err := def.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return &def, nil
}

// splitFrontmatter separates YAML frontmatter from markdown body
func splitFrontmatter(content []byte) ([]byte, []byte, error) {
    // Check for frontmatter delimiter
    if !bytes.HasPrefix(content, []byte("---\n")) {
        return nil, nil, fmt.Errorf("missing frontmatter delimiter")
    }

    // Find end of frontmatter
    rest := content[4:] // Skip opening ---
    endIdx := bytes.Index(rest, []byte("\n---"))
    if endIdx == -1 {
        return nil, nil, fmt.Errorf("missing frontmatter closing delimiter")
    }

    frontmatter := rest[:endIdx]
    body := bytes.TrimSpace(rest[endIdx+4:]) // Skip closing ---

    return frontmatter, body, nil
}

// applyDefaults sets default values for optional fields
func applyDefaults(def *Definition) {
    // RECALL defaults
    if def.Recall.QueryTypes == nil {
        def.Recall.QueryTypes = []string{"decision", "pattern", "evidence"}
    }
    def.Recall.AutoQuery = def.Recall.AutoQuery || true

    // Context defaults
    if def.Context.MaxHistory == 0 {
        def.Context.MaxHistory = 3
    }
    if !def.Context.IncludeProfile {
        def.Context.IncludeProfile = true
    }
    if !def.Context.IncludeTasks {
        def.Context.IncludeTasks = true
    }

    // Capture defaults
    if def.Capture.SuggestTypes == nil {
        def.Capture.SuggestTypes = []string{"decision", "pattern"}
    }
}
```

### 4.3 Building System Prompt

```go
package agent

import (
    "strings"
    "text/template"
)

// SystemPromptBuilder constructs the full system prompt
type SystemPromptBuilder struct {
    skillLoader *SkillLoader
}

// BuildSystemPrompt creates the complete prompt for an agent
func (b *SystemPromptBuilder) BuildSystemPrompt(def *Definition, ctx *SessionContext) (string, error) {
    var sb strings.Builder

    // 1. Agent identity and behaviors
    sb.WriteString("# Agent: ")
    sb.WriteString(def.Name)
    sb.WriteString("\n\n")
    sb.WriteString(def.Description)
    sb.WriteString("\n\n")

    // 2. Quick behaviors (from frontmatter)
    if len(def.Behaviors) > 0 {
        sb.WriteString("## Key Behaviors\n\n")
        for _, behavior := range def.Behaviors {
            sb.WriteString("- ")
            sb.WriteString(behavior)
            sb.WriteString("\n")
        }
        sb.WriteString("\n")
    }

    // 3. Main system prompt (markdown body)
    sb.WriteString(def.SystemPrompt)
    sb.WriteString("\n\n")

    // 4. Load and append skills
    for _, skillName := range def.Skills {
        skill, err := b.skillLoader.Load(skillName)
        if err != nil {
            continue // Skip missing skills with warning
        }
        sb.WriteString("---\n\n")
        sb.WriteString("## Skill: ")
        sb.WriteString(skillName)
        sb.WriteString("\n\n")
        sb.WriteString(skill.Content)
        sb.WriteString("\n\n")
    }

    // 5. Tool availability
    sb.WriteString("---\n\n")
    sb.WriteString("## Available Tools\n\n")
    for _, tool := range def.Tools.Required {
        sb.WriteString("- ")
        sb.WriteString(tool)
        sb.WriteString(" (required)\n")
    }
    for _, tool := range def.Tools.Optional {
        sb.WriteString("- ")
        sb.WriteString(tool)
        sb.WriteString(" (optional)\n")
    }
    if len(def.Tools.Forbidden) > 0 {
        sb.WriteString("\nDo NOT use: ")
        sb.WriteString(strings.Join(def.Tools.Forbidden, ", "))
        sb.WriteString("\n")
    }

    return sb.String(), nil
}
```

---

## 5. Agent Switching

### 5.1 Switch Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      AGENT SWITCH FLOW                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  User: /plan                                                     │
│         │                                                        │
│         ▼                                                        │
│  ┌──────────────────┐                                           │
│  │ Parse Command    │  Resolve: /plan → architect agent         │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │ Load Agent       │  Load architect.md (with resolution)      │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │ Load Skills      │  Load system-design, adrs, cross-team     │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │ Query RECALL     │  Auto-query for relevant context          │
│  │ (if auto_query)  │                                           │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │ Build Prompt     │  Combine agent + skills + context         │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │ Update Session   │  Record agent switch in session state     │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  Claude: "Switched to architect mode. How can I help            │
│           with system design?"                                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 5.2 Switch Manager

```go
package agent

import (
    "context"
)

// SwitchManager handles agent switching during sessions
type SwitchManager struct {
    loader       *Loader
    skillLoader  *SkillLoader
    recall       *recall.Client
    promptBuilder *SystemPromptBuilder
}

// SwitchResult contains the result of an agent switch
type SwitchResult struct {
    Agent        *Definition
    SystemPrompt string
    RecallContext []recall.Result
    Message      string
}

// Switch changes the active agent
func (m *SwitchManager) Switch(ctx context.Context, session *Session, agentName string) (*SwitchResult, error) {
    // 1. Load agent definition
    agent, err := m.loader.Load(agentName)
    if err != nil {
        return nil, fmt.Errorf("loading agent %s: %w", agentName, err)
    }

    // 2. Query RECALL if auto_query enabled
    var recallContext []recall.Result
    if agent.Recall.AutoQuery {
        query := buildAutoQuery(agent, session)
        results, err := m.recall.Search(ctx, &recall.SearchOptions{
            Query: query,
            Types: agent.Recall.QueryTypes,
            Limit: 5,
        })
        if err == nil {
            recallContext = results
        }
    }

    // 3. Build system prompt
    sessionCtx := &SessionContext{
        ProjectName: session.ProjectID,
        CurrentTask: session.CurrentTask,
    }
    systemPrompt, err := m.promptBuilder.BuildSystemPrompt(agent, sessionCtx)
    if err != nil {
        return nil, fmt.Errorf("building system prompt: %w", err)
    }

    // 4. Update session state
    session.Agent = agentName
    session.Skills = agent.Skills

    // 5. Build confirmation message
    message := fmt.Sprintf("Switched to **%s** mode. %s", 
        agent.Name, agent.Description)

    return &SwitchResult{
        Agent:        agent,
        SystemPrompt: systemPrompt,
        RecallContext: recallContext,
        Message:      message,
    }, nil
}

// buildAutoQuery creates a RECALL query for agent activation
func buildAutoQuery(agent *Definition, session *Session) string {
    template := agent.Recall.QueryTemplate
    if template == "" {
        template = "{context}"
    }

    // Replace {context} with current context
    context := session.ProjectID
    if session.CurrentTask != "" {
        context += " " + session.CurrentTask
    }

    return strings.Replace(template, "{context}", context, 1)
}
```

### 5.3 Command to Agent Mapping

```go
package agent

// CommandMapping maps slash commands to agents
var CommandMapping = map[string]string{
    "plan":      "architect",
    "architect": "architect",
    "design":    "architect",
    
    "build":     "coder",
    "code":      "coder",
    "implement": "coder",
    
    "review":    "reviewer",
    "check":     "reviewer",
    
    "incident":  "incident",
    "debug":     "incident",
    "fix":       "incident",
}

// ResolveCommand returns the agent name for a command
func ResolveCommand(command string) (string, bool) {
    agent, ok := CommandMapping[strings.ToLower(command)]
    return agent, ok
}
```

### 5.4 Persisting Agent State

Agent state is tracked in the session:

```go
// Session includes agent tracking
type Session struct {
    // ... other fields
    
    Agent       string   `json:"agent"`        // Current agent name
    Skills      []string `json:"skills"`       // Currently loaded skills
    AgentSwitches []AgentSwitch `json:"agent_switches"` // Switch history
}

// AgentSwitch records an agent switch event
type AgentSwitch struct {
    FromAgent string    `json:"from_agent"`
    ToAgent   string    `json:"to_agent"`
    Timestamp time.Time `json:"timestamp"`
    Trigger   string    `json:"trigger"` // Command that triggered switch
}

// RecordSwitch adds an agent switch to session history
func (s *Session) RecordSwitch(from, to, trigger string) {
    s.AgentSwitches = append(s.AgentSwitches, AgentSwitch{
        FromAgent: from,
        ToAgent:   to,
        Timestamp: time.Now(),
        Trigger:   trigger,
    })
}
```

---

## 6. Skills Integration

### 6.1 Skill Loading

Skills use Claude Code's standard format (`SKILL.md`):

```go
package agent

// Skill represents a loaded skill
type Skill struct {
    Name    string
    Content string
    Path    string
}

// SkillLoader handles skill loading
type SkillLoader struct {
    globalPath  string // ~/.edi/skills/
    projectPath string // .edi/skills/
}

// Load retrieves a skill with resolution
func (l *SkillLoader) Load(name string) (*Skill, error) {
    // 1. Check project skills
    if l.projectPath != "" {
        path := filepath.Join(l.projectPath, name, "SKILL.md")
        if content, err := os.ReadFile(path); err == nil {
            return &Skill{Name: name, Content: string(content), Path: path}, nil
        }
    }

    // 2. Check global skills
    path := filepath.Join(l.globalPath, name, "SKILL.md")
    if content, err := os.ReadFile(path); err == nil {
        return &Skill{Name: name, Content: string(content), Path: path}, nil
    }

    return nil, fmt.Errorf("skill not found: %s", name)
}

// LoadMultiple loads multiple skills
func (l *SkillLoader) LoadMultiple(names []string) ([]*Skill, error) {
    var skills []*Skill
    var errors []string

    for _, name := range names {
        skill, err := l.Load(name)
        if err != nil {
            errors = append(errors, name)
            continue
        }
        skills = append(skills, skill)
    }

    if len(errors) > 0 {
        log.Printf("Warning: failed to load skills: %v", errors)
    }

    return skills, nil
}
```

### 6.2 Skill Merging

When an agent specifies skills, and the project config also specifies skills, they're merged:

```go
// MergeSkills combines agent skills with project overrides
func MergeSkills(agentSkills []string, projectSkills []string) []string {
    seen := make(map[string]bool)
    var merged []string

    // Agent skills first
    for _, s := range agentSkills {
        if !seen[s] {
            seen[s] = true
            merged = append(merged, s)
        }
    }

    // Project skills added (may include project-specific ones)
    for _, s := range projectSkills {
        if !seen[s] {
            seen[s] = true
            merged = append(merged, s)
        }
    }

    return merged
}
```

---

## 7. Implementation

### 7.1 Package Structure

```
edi/
├── internal/
│   └── agent/
│       ├── definition.go      # Agent definition types
│       ├── loader.go          # Agent loading with resolution
│       ├── parser.go          # Markdown/YAML parsing
│       ├── prompt.go          # System prompt building
│       ├── switch.go          # Agent switching logic
│       ├── skill.go           # Skill loading
│       └── builtin/           # Built-in agent definitions
│           ├── architect.go
│           ├── coder.go
│           ├── reviewer.go
│           └── incident.go
```

### 7.2 Implementation Plan

#### Phase 3.1: Agent Definition Schema (Week 1)

- [ ] Definition types and schema
- [ ] YAML/Markdown parser
- [ ] Validation logic
- [ ] Unit tests for parsing

**Exit Criteria**: Can parse and validate agent files.

#### Phase 3.2: Core Agent Specs (Week 1-2)

- [ ] Architect agent definition
- [ ] Coder agent definition
- [ ] Reviewer agent definition
- [ ] Incident agent definition
- [ ] Built-in embedding in binary

**Exit Criteria**: All four core agents defined and embedded.

#### Phase 3.3: Agent Loading (Week 2)

- [ ] Loader with resolution (project → global → built-in)
- [ ] Skill loader
- [ ] System prompt builder
- [ ] Integration tests

**Exit Criteria**: Can load agents from all sources.

#### Phase 3.4: Agent Switching (Week 3)

- [ ] Switch manager
- [ ] Command-to-agent mapping
- [ ] RECALL auto-query on switch
- [ ] Session state tracking
- [ ] Integration with commands

**Exit Criteria**: Can switch agents via commands.

### 7.3 Validation Criteria

| Metric | Target |
|--------|--------|
| Agent load time | < 50ms |
| System prompt build time | < 100ms |
| Switch latency (without RECALL) | < 200ms |
| Switch latency (with RECALL) | < 2s |

---

## Appendix A: Quick Reference

### Agent Commands

| Command | Agent | Aliases |
|---------|-------|---------|
| `/plan` | architect | `/architect`, `/design` |
| `/build` | coder | `/code`, `/implement` |
| `/review` | reviewer | `/check` |
| `/incident` | incident | `/debug`, `/fix` |

### Agent Files

| Location | Purpose |
|----------|---------|
| `~/.edi/agents/*.md` | Global agent definitions |
| `.edi/agents/*.md` | Project agent overrides |

### Frontmatter Quick Reference

```yaml
name: string        # Required: agent identifier
description: string # Required: one-line description
version: 1          # Required: always 1 for now
skills: []          # Skills to load
behaviors: []       # Quick behavioral rules
tools:
  required: []      # Must-have tools
  optional: []      # Nice-to-have tools
recall:
  auto_query: true  # Query on activation
  query_types: []   # Types to search
```

---

## Appendix B: Related Specifications

| Spec | Relationship |
|------|--------------|
| Workspace & Configuration | Agent file locations, config overrides |
| Session Lifecycle | Agent loading at start, tracking during session |
| RECALL MCP Server | Auto-query on switch, tool requirements |
| CLI Architecture | Command parsing, agent switching |

---

## Appendix C: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 25, 2026 | Markdown + YAML frontmatter | Human-readable, matches workspace format |
| Jan 25, 2026 | Four core agents | Covers main engineering workflows |
| Jan 25, 2026 | Resolution: project → global → built-in | Allows customization at multiple levels |
| Jan 25, 2026 | Skills loaded with agent | Keeps agent self-contained |
| Jan 25, 2026 | RECALL auto-query on switch | Proactive context loading |
| Jan 25, 2026 | Agent switch tracked in session | Enables history analysis |
