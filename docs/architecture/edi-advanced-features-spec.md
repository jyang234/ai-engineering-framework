# EDI Advanced Features Specification

**Status**: Draft  
**Created**: January 25, 2026  
**Version**: 0.1  
**Depends On**: All Phase 1-4 specs

---

## Table of Contents

1. [Overview](#1-overview)
2. [Multi-Project Management](#2-multi-project-management)
3. [External Integrations](#3-external-integrations)
4. [VERIFY Integration](#4-verify-integration)
5. [Implementation](#5-implementation)

---

## 1. Overview

Phase 5 covers advanced features that extend EDI's core functionality:

| Feature | Purpose | Priority |
|---------|---------|----------|
| **Multi-Project Management** | Work across multiple projects with shared knowledge | Medium |
| **External Integrations** | Connect to calendars, issue trackers, etc. | Low |
| **VERIFY Integration** | CI/CD quality gates for AI-generated code | Medium |

These features are optional but valuable for:
- Developers working on multiple projects
- Teams needing external tool integration
- Organizations requiring quality gates

---

## 2. Multi-Project Management

### 2.1 Problem Statement

Developers often work on multiple projects:
- Microservices that depend on each other
- Library + applications that use it
- Client work across different codebases

Without multi-project support:
- Knowledge is siloed per project
- Switching projects loses context
- Cross-project patterns aren't shared

### 2.2 Project Registry

The project registry tracks all EDI-enabled projects:

**Location**: `~/.edi/projects.yaml`

```yaml
# EDI Project Registry
version: 1

projects:
  - id: proj-abc123
    name: payment-service
    path: ~/projects/payment-service
    domain: payments
    last_accessed: 2026-01-24T15:30:00Z
    tags:
      - backend
      - golang
      - grpc

  - id: proj-def456
    name: user-service
    path: ~/projects/user-service
    domain: identity
    last_accessed: 2026-01-24T10:15:00Z
    tags:
      - backend
      - golang
      - rest

  - id: proj-ghi789
    name: web-app
    path: ~/projects/web-app
    domain: frontend
    last_accessed: 2026-01-23T09:00:00Z
    tags:
      - frontend
      - typescript
      - react

domains:
  - name: payments
    description: Payment processing services
    owner: payment-team
    
  - name: identity
    description: User identity and authentication
    owner: identity-team
    
  - name: frontend
    description: User-facing applications
    owner: frontend-team
```

### 2.3 Project Registration

Projects are registered automatically on `edi init` or manually:

```bash
# Automatic (during init)
cd ~/projects/new-project
edi init
# → Project registered in ~/.edi/projects.yaml

# Manual registration
edi project add ~/projects/existing-project --domain backend

# List registered projects
edi project list

# Remove from registry
edi project remove proj-abc123
```

**Implementation**:

```go
package project

// Registry manages the project registry
type Registry struct {
    path string // ~/.edi/projects.yaml
}

// Project represents a registered project
type Project struct {
    ID           string    `yaml:"id"`
    Name         string    `yaml:"name"`
    Path         string    `yaml:"path"`
    Domain       string    `yaml:"domain,omitempty"`
    LastAccessed time.Time `yaml:"last_accessed"`
    Tags         []string  `yaml:"tags,omitempty"`
}

// Register adds a project to the registry
func (r *Registry) Register(project *Project) error {
    projects, err := r.load()
    if err != nil {
        return err
    }

    // Check for duplicates
    for _, p := range projects {
        if p.Path == project.Path {
            return fmt.Errorf("project already registered: %s", project.Path)
        }
    }

    // Generate ID if not provided
    if project.ID == "" {
        project.ID = generateProjectID()
    }

    projects = append(projects, project)
    return r.save(projects)
}

// Find locates a project by path, name, or ID
func (r *Registry) Find(query string) (*Project, error) {
    projects, err := r.load()
    if err != nil {
        return nil, err
    }

    for _, p := range projects {
        if p.ID == query || p.Name == query || p.Path == query {
            return p, nil
        }
    }

    return nil, fmt.Errorf("project not found: %s", query)
}

// Recent returns recently accessed projects
func (r *Registry) Recent(limit int) ([]*Project, error) {
    projects, err := r.load()
    if err != nil {
        return nil, err
    }

    // Sort by last accessed
    sort.Slice(projects, func(i, j int) bool {
        return projects[i].LastAccessed.After(projects[j].LastAccessed)
    })

    if len(projects) > limit {
        projects = projects[:limit]
    }

    return projects, nil
}

// UpdateAccess updates the last accessed time
func (r *Registry) UpdateAccess(projectID string) error {
    projects, err := r.load()
    if err != nil {
        return err
    }

    for i, p := range projects {
        if p.ID == projectID {
            projects[i].LastAccessed = time.Now()
            return r.save(projects)
        }
    }

    return fmt.Errorf("project not found: %s", projectID)
}
```

### 2.4 Project Switching

Switch between projects without losing context:

```bash
# Switch to another project
edi switch payment-service
edi switch ~/projects/payment-service
edi switch proj-abc123

# Quick switch to recent project
edi switch --recent    # Interactive picker
edi switch -           # Switch to previous project

# Switch with initial prompt
edi switch user-service "Continue the auth refactoring"
```

**Switch Flow**:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        PROJECT SWITCH FLOW                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  User: edi switch payment-service                                        │
│                                                                          │
│  EDI:                                                                    │
│  1. End current session (if active)                                      │
│     ├─ Generate summary                                                  │
│     ├─ Prompt for capture                                                │
│     └─ Save history                                                      │
│                                                                          │
│  2. Resolve target project                                               │
│     ├─ Find in registry                                                  │
│     └─ Validate path exists                                              │
│                                                                          │
│  3. Update registry                                                      │
│     └─ Set last_accessed = now                                           │
│                                                                          │
│  4. Start new session                                                    │
│     ├─ cd to project path                                                │
│     ├─ Load project config                                               │
│     ├─ Generate briefing                                                 │
│     └─ Launch Claude Code                                                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.5 Cross-Project Knowledge

RECALL supports knowledge at three scopes:

| Scope | Retrieved When | Examples |
|-------|----------------|----------|
| **Project** | Working on that project | "OrderService uses event sourcing" |
| **Domain** | Working on any project in domain | "All payments use Stripe" |
| **Global** | Always | "Use mTLS for internal services" |

**Scope Hierarchy**:

```
┌─────────────────────────────────────────────────────────────────┐
│  GLOBAL                                                          │
│  ├── Retrieved for ALL projects                                  │
│  ├── Managed by: Enterprise architect / Tech lead                │
│  └── Examples: Security policies, API standards                  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  DOMAIN (e.g., "payments")                                  │ │
│  │  ├── Retrieved for projects in this domain                  │ │
│  │  ├── Managed by: Domain owner                               │ │
│  │  └── Examples: Payment patterns, Stripe integration         │ │
│  │                                                             │ │
│  │  ┌─────────────────────────────────────────────────────────┐│ │
│  │  │  PROJECT (e.g., "payment-service")                      ││ │
│  │  │  ├── Retrieved only for this project                    ││ │
│  │  │  ├── Managed by: Project team                           ││ │
│  │  │  └── Examples: Service-specific decisions               ││ │
│  │  └─────────────────────────────────────────────────────────┘│ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Querying Across Scopes**:

```go
// RECALL search with scope expansion
func (c *Client) Search(ctx context.Context, opts *SearchOptions) ([]Result, error) {
    // Determine scopes to query
    scopes := []string{opts.Project}
    
    if opts.IncludeDomain && opts.Domain != "" {
        scopes = append(scopes, fmt.Sprintf("domain:%s", opts.Domain))
    }
    
    if opts.IncludeGlobal {
        scopes = append(scopes, "global")
    }
    
    // Query each scope
    var results []Result
    for _, scope := range scopes {
        scopeResults, err := c.searchScope(ctx, opts.Query, scope)
        if err != nil {
            continue // Log warning, don't fail
        }
        results = append(results, scopeResults...)
    }
    
    // Rerank combined results
    return c.rerank(ctx, opts.Query, results)
}
```

### 2.6 Knowledge Promotion

Promote knowledge from project → domain → global:

```bash
# Promote a knowledge item
edi recall promote <item-id> --to domain
edi recall promote <item-id> --to global

# List items pending promotion
edi recall promotions --pending
```

**Promotion Flow**:

```
┌─────────────────────────────────────────────────────────────────┐
│                      PROMOTION FLOW                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. User requests promotion                                      │
│     edi recall promote item-123 --to domain                      │
│                                                                  │
│  2. Create promotion request                                     │
│     ┌──────────────────────────────────────────────────────┐    │
│     │  Promotion Request                                    │    │
│     │  ├── Item: item-123                                   │    │
│     │  ├── From: project:payment-service                    │    │
│     │  ├── To: domain:payments                              │    │
│     │  ├── Requester: john@example.com                      │    │
│     │  └── Justification: "Used in 3 projects"              │    │
│     └──────────────────────────────────────────────────────┘    │
│                                                                  │
│  3. Notify approver (domain owner)                               │
│     • Slack notification                                         │
│     • Email (optional)                                           │
│                                                                  │
│  4. Approver reviews                                             │
│     edi recall promotions --pending                              │
│     edi recall approve <request-id>                              │
│     edi recall reject <request-id> --reason "Too specific"       │
│                                                                  │
│  5. If approved: Update item scope                               │
│     If rejected: Notify requester                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.7 Cross-Project Commands

```bash
# Search across all projects
edi recall search --scope all "authentication pattern"

# Search specific domain
edi recall search --scope domain:payments "retry logic"

# List projects in a domain
edi project list --domain payments

# Show cross-project dependencies
edi project deps payment-service
```

---

## 3. External Integrations

### 3.1 Integration Architecture

External integrations are implemented as MCP servers:

```
┌─────────────────────────────────────────────────────────────────┐
│                          EDI                                     │
│                                                                  │
│   ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐   │
│   │  RECALL   │  │   Jira    │  │  Calendar │  │   Slack   │   │
│   │   MCP     │  │   MCP     │  │    MCP    │  │   MCP     │   │
│   └───────────┘  └───────────┘  └───────────┘  └───────────┘   │
│         │              │              │              │          │
└─────────┼──────────────┼──────────────┼──────────────┼──────────┘
          │              │              │              │
          ▼              ▼              ▼              ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
    │  RECALL  │   │   Jira   │   │  Google  │   │  Slack   │
    │  Server  │   │   API    │   │ Calendar │   │   API    │
    └──────────┘   └──────────┘   └──────────┘   └──────────┘
```

### 3.2 Integration Configuration

**Global config** (`~/.edi/config.yaml`):

```yaml
integrations:
  # Jira integration
  jira:
    enabled: true
    server: https://yourcompany.atlassian.net
    # Credentials via environment: JIRA_API_TOKEN
    default_project: ENG
    
  # Calendar integration  
  calendar:
    enabled: true
    provider: google  # or: outlook
    # OAuth configured separately
    
  # Slack integration
  slack:
    enabled: true
    workspace: yourcompany
    # Token via environment: SLACK_BOT_TOKEN
    channels:
      incidents: "#incidents"
      decisions: "#architecture-decisions"
      
  # GitHub integration
  github:
    enabled: true
    # Token via environment: GITHUB_TOKEN
```

### 3.3 Jira Integration

**Purpose**: Sync tasks, link decisions to tickets, update status.

**MCP Tools**:

```yaml
tools:
  jira_get_ticket:
    description: "Get details of a Jira ticket"
    parameters:
      ticket_id: string  # e.g., "ENG-123"
    returns:
      summary: string
      description: string
      status: string
      assignee: string
      priority: string
      labels: string[]

  jira_search:
    description: "Search for Jira tickets"
    parameters:
      jql: string  # Jira Query Language
      limit: integer
    returns:
      tickets: Ticket[]

  jira_update_status:
    description: "Update ticket status"
    parameters:
      ticket_id: string
      status: string  # e.g., "In Progress", "Done"
    returns:
      success: boolean

  jira_add_comment:
    description: "Add comment to ticket"
    parameters:
      ticket_id: string
      comment: string
    returns:
      success: boolean

  jira_link_decision:
    description: "Link an EDI decision to a ticket"
    parameters:
      ticket_id: string
      decision_id: string  # RECALL item ID
    returns:
      success: boolean
```

**Briefing Integration**:

When generating briefings, EDI can include Jira context:

```markdown
## Current Tasks (from Jira)

- **ENG-123**: Implement payment retry logic (In Progress)
  - Assigned to: you
  - Due: Jan 26, 2026
  
- **ENG-124**: Review auth service changes (Code Review)
  - Assigned to: you
  - PR: #456
```

### 3.4 Calendar Integration

**Purpose**: Include schedule context in briefings, suggest focus times.

**MCP Tools**:

```yaml
tools:
  calendar_today:
    description: "Get today's schedule"
    parameters: {}
    returns:
      events: Event[]

  calendar_availability:
    description: "Find available time blocks"
    parameters:
      duration_minutes: integer
      within_days: integer
    returns:
      blocks: TimeBlock[]

  calendar_create_focus:
    description: "Block focus time for deep work"
    parameters:
      title: string
      duration_minutes: integer
      preferred_time: string  # "morning", "afternoon", "any"
    returns:
      event_id: string
```

**Briefing Integration**:

```markdown
## Today's Schedule

- 09:00-09:30: Standup
- 10:00-11:30: **Focus time available** ← suggested for ENG-123
- 11:30-12:00: 1:1 with manager
- 14:00-15:00: Architecture review
- 15:00-17:00: **Focus time available**

Suggested: Use morning block for payment retry implementation.
```

### 3.5 Slack Integration

**Purpose**: Notify on decisions, escalate incidents, share updates.

**MCP Tools**:

```yaml
tools:
  slack_send:
    description: "Send message to Slack channel"
    parameters:
      channel: string
      message: string
      thread_ts: string?  # Reply to thread
    returns:
      ts: string  # Message timestamp

  slack_notify_decision:
    description: "Announce a decision to the team"
    parameters:
      decision_id: string
      channel: string
    returns:
      success: boolean

  slack_escalate_incident:
    description: "Escalate an incident"
    parameters:
      severity: string  # SEV1, SEV2, etc.
      summary: string
      channel: string
    returns:
      thread_ts: string
```

**Capture Integration**:

When capturing a decision, optionally announce to Slack:

```
┌─────────────────────────────────────────────────────────────────┐
│  Decision captured: Chose Stripe over Paddle                     │
│                                                                  │
│  Share to Slack?                                                 │
│  [Yes, #architecture-decisions] [No]                             │
└─────────────────────────────────────────────────────────────────┘
```

### 3.6 GitHub Integration

**Purpose**: Link commits to decisions, check PR status, reference issues.

**MCP Tools**:

```yaml
tools:
  github_pr_status:
    description: "Get PR status"
    parameters:
      pr_number: integer
    returns:
      state: string  # open, closed, merged
      checks: Check[]
      reviews: Review[]

  github_link_commit:
    description: "Link a commit to a RECALL decision"
    parameters:
      commit_sha: string
      decision_id: string
    returns:
      success: boolean

  github_create_issue:
    description: "Create a GitHub issue"
    parameters:
      title: string
      body: string
      labels: string[]
    returns:
      issue_number: integer
      url: string
```

### 3.7 Custom Integrations

Users can add custom MCP servers:

```yaml
# ~/.edi/config.yaml
integrations:
  custom:
    - name: internal-wiki
      command: ~/.edi/integrations/wiki-mcp
      args: ["--config", "~/.edi/wiki-config.yaml"]
      
    - name: pagerduty
      command: npx
      args: ["-y", "@yourorg/pagerduty-mcp"]
```

---

## 4. VERIFY Integration

### 4.1 Overview

VERIFY provides CI/CD quality gates for AI-generated code:

```
┌─────────────────────────────────────────────────────────────────┐
│                     VERIFY Pipeline                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Code Generated                                                 │
│         │                                                        │
│         ▼                                                        │
│   ┌───────────┐    ┌───────────┐    ┌───────────┐              │
│   │   Lint    │ → │   Test    │ → │  Pattern  │              │
│   │   Check   │    │   Run     │    │   Check   │              │
│   └─────┬─────┘    └─────┬─────┘    └─────┬─────┘              │
│         │                │                │                      │
│         ▼                ▼                ▼                      │
│   ┌───────────┐    ┌───────────┐    ┌───────────┐              │
│   │ Security  │ → │ Coverage  │ → │  Custom   │              │
│   │   Scan    │    │   Check   │    │   Rules   │              │
│   └─────┬─────┘    └─────┬─────┘    └─────┬─────┘              │
│         │                │                │                      │
│         └────────────────┴────────────────┘                      │
│                          │                                       │
│                          ▼                                       │
│                    ┌───────────┐                                │
│                    │  Result   │                                │
│                    │  Pass/Fail│                                │
│                    └─────┬─────┘                                │
│                          │                                       │
│              ┌───────────┴───────────┐                          │
│              ▼                       ▼                          │
│         [PASS]                  [FAIL]                          │
│         Continue                Self-Correct                    │
│                                 Loop                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 VERIFY as Separate Tool

VERIFY is intentionally **separate from EDI**:

| Aspect | EDI | VERIFY |
|--------|-----|--------|
| Purpose | User experience, continuity | Quality enforcement |
| Lifecycle | Interactive sessions | CI/CD pipeline |
| Trigger | User commands | Git push, PR creation |
| Output | Briefings, captures | Pass/fail, reports |

**Integration point**: EDI can trigger VERIFY checks, and VERIFY results can be captured to RECALL.

### 4.3 VERIFY Configuration

**Project config** (`.verify/config.yaml`):

```yaml
# VERIFY Configuration
version: 1

# Check stages
stages:
  lint:
    enabled: true
    tools:
      - eslint
      - prettier
    fail_on: error  # error, warning, never
    
  test:
    enabled: true
    command: npm test
    coverage:
      enabled: true
      min_coverage: 80
    timeout: 300s
    
  security:
    enabled: true
    tools:
      - npm audit
      - snyk
    fail_on: high  # critical, high, medium, low
    
  patterns:
    enabled: true
    rules:
      - no-console-log
      - prefer-async-await
      - require-error-handling
    custom_rules_path: .verify/rules/

# Self-correction
self_correct:
  enabled: true
  max_iterations: 3
  on_failure:
    - lint: auto_fix
    - test: retry_with_feedback
    - security: escalate
    - patterns: suggest_fix

# Notifications
notifications:
  slack:
    channel: "#ci-results"
    on: failure
  
  github:
    pr_comment: true
    check_run: true
```

### 4.4 VERIFY Stages

#### Lint Stage

```go
package verify

// LintStage runs linting checks
type LintStage struct {
    Tools     []string // eslint, prettier, golint, etc.
    FailOn    string   // error, warning, never
    AutoFix   bool
}

func (s *LintStage) Run(ctx context.Context, files []string) (*StageResult, error) {
    var issues []Issue
    
    for _, tool := range s.Tools {
        toolIssues, err := runLintTool(ctx, tool, files)
        if err != nil {
            return nil, err
        }
        issues = append(issues, toolIssues...)
    }
    
    // Determine pass/fail
    passed := true
    for _, issue := range issues {
        if issue.Severity == "error" && s.FailOn == "error" {
            passed = false
            break
        }
        if issue.Severity == "warning" && s.FailOn == "warning" {
            passed = false
            break
        }
    }
    
    return &StageResult{
        Stage:   "lint",
        Passed:  passed,
        Issues:  issues,
        AutoFix: s.AutoFix && !passed,
    }, nil
}
```

#### Test Stage

```go
// TestStage runs tests and checks coverage
type TestStage struct {
    Command     string
    Coverage    CoverageConfig
    Timeout     time.Duration
}

func (s *TestStage) Run(ctx context.Context, projectPath string) (*StageResult, error) {
    ctx, cancel := context.WithTimeout(ctx, s.Timeout)
    defer cancel()
    
    // Run tests
    cmd := exec.CommandContext(ctx, "sh", "-c", s.Command)
    cmd.Dir = projectPath
    output, err := cmd.CombinedOutput()
    
    testsPassed := err == nil
    
    // Check coverage if enabled
    var coverageResult *CoverageResult
    if s.Coverage.Enabled {
        coverageResult, err = s.checkCoverage(ctx, projectPath)
        if err != nil {
            return nil, err
        }
        
        if coverageResult.Percentage < s.Coverage.MinCoverage {
            testsPassed = false
        }
    }
    
    return &StageResult{
        Stage:    "test",
        Passed:   testsPassed,
        Output:   string(output),
        Coverage: coverageResult,
    }, nil
}
```

#### Security Stage

```go
// SecurityStage runs security scans
type SecurityStage struct {
    Tools   []string
    FailOn  string // critical, high, medium, low
}

func (s *SecurityStage) Run(ctx context.Context, projectPath string) (*StageResult, error) {
    var vulnerabilities []Vulnerability
    
    for _, tool := range s.Tools {
        toolVulns, err := runSecurityTool(ctx, tool, projectPath)
        if err != nil {
            continue // Log warning, don't fail
        }
        vulnerabilities = append(vulnerabilities, toolVulns...)
    }
    
    // Determine pass/fail based on severity threshold
    passed := true
    severityOrder := []string{"critical", "high", "medium", "low"}
    failIndex := indexOf(severityOrder, s.FailOn)
    
    for _, vuln := range vulnerabilities {
        vulnIndex := indexOf(severityOrder, vuln.Severity)
        if vulnIndex <= failIndex {
            passed = false
            break
        }
    }
    
    return &StageResult{
        Stage:           "security",
        Passed:          passed,
        Vulnerabilities: vulnerabilities,
    }, nil
}
```

#### Pattern Stage

```go
// PatternStage checks for organizational patterns
type PatternStage struct {
    Rules          []string
    CustomRulesPath string
}

func (s *PatternStage) Run(ctx context.Context, files []string) (*StageResult, error) {
    var violations []PatternViolation
    
    // Load rules
    rules, err := s.loadRules()
    if err != nil {
        return nil, err
    }
    
    // Check each file against rules
    for _, file := range files {
        content, err := os.ReadFile(file)
        if err != nil {
            continue
        }
        
        for _, rule := range rules {
            if violation := rule.Check(file, content); violation != nil {
                violations = append(violations, *violation)
            }
        }
    }
    
    return &StageResult{
        Stage:      "patterns",
        Passed:     len(violations) == 0,
        Violations: violations,
    }, nil
}
```

### 4.5 Self-Correction Loop

When VERIFY fails, trigger self-correction:

```go
package verify

// SelfCorrectLoop attempts to fix failures
func SelfCorrectLoop(ctx context.Context, result *PipelineResult, config *SelfCorrectConfig) (*PipelineResult, error) {
    if result.Passed || !config.Enabled {
        return result, nil
    }
    
    for iteration := 0; iteration < config.MaxIterations; iteration++ {
        // Generate fix based on failure type
        fix, err := generateFix(ctx, result)
        if err != nil {
            return result, err
        }
        
        // Apply fix
        if err := applyFix(fix); err != nil {
            continue
        }
        
        // Re-run VERIFY
        newResult, err := runPipeline(ctx, result.Files)
        if err != nil {
            continue
        }
        
        if newResult.Passed {
            return newResult, nil
        }
        
        result = newResult
    }
    
    // Max iterations reached, escalate
    return result, fmt.Errorf("self-correction failed after %d iterations", config.MaxIterations)
}

// generateFix creates a fix based on failure type
func generateFix(ctx context.Context, result *PipelineResult) (*Fix, error) {
    for _, stage := range result.Stages {
        if stage.Passed {
            continue
        }
        
        switch stage.Stage {
        case "lint":
            // Auto-fix linting issues
            return &Fix{
                Type:    "auto_fix",
                Command: "npm run lint -- --fix",
            }, nil
            
        case "test":
            // Generate prompt for Claude to fix tests
            return &Fix{
                Type:   "claude_fix",
                Prompt: buildTestFixPrompt(stage),
            }, nil
            
        case "security":
            // Escalate security issues (don't auto-fix)
            return &Fix{
                Type:    "escalate",
                Message: "Security vulnerabilities require human review",
            }, nil
            
        case "patterns":
            // Generate suggestions for pattern violations
            return &Fix{
                Type:       "suggest",
                Suggestions: buildPatternSuggestions(stage),
            }, nil
        }
    }
    
    return nil, fmt.Errorf("no fixable failures found")
}
```

### 4.6 EDI ↔ VERIFY Integration

#### Triggering VERIFY from EDI

```bash
# Manual trigger
edi verify

# Verify specific files
edi verify --files src/payment.ts

# Verify with self-correction
edi verify --self-correct

# Verify before commit (git hook)
edi verify --pre-commit
```

#### Capturing VERIFY Results to RECALL

When VERIFY finds patterns, capture to RECALL:

```go
// CaptureVerifyResults saves significant findings to RECALL
func CaptureVerifyResults(ctx context.Context, result *PipelineResult, recall *recall.Client) error {
    for _, stage := range result.Stages {
        if stage.Passed {
            continue
        }
        
        // Capture failures as knowledge
        for _, issue := range stage.Issues {
            if issue.Significant {
                recall.Add(ctx, &recall.AddOptions{
                    Type:    "failure",
                    Summary: issue.Summary,
                    Detail:  issue.Detail,
                    Scope:   recall.ScopeProject,
                    Metadata: map[string]string{
                        "source":     "verify",
                        "stage":      stage.Stage,
                        "severity":   issue.Severity,
                        "file":       issue.File,
                        "resolution": issue.Resolution,
                    },
                })
            }
        }
    }
    
    return nil
}
```

#### VERIFY-Aware Briefings

Include VERIFY status in briefings:

```markdown
## Quality Status

**Last VERIFY run**: 2 hours ago (passing ✓)

Recent issues fixed:
- Lint: 3 auto-fixed formatting issues
- Tests: 1 flaky test stabilized

**Coverage**: 84% (+2% from last week)
```

### 4.7 CI/CD Integration

**GitHub Actions**:

```yaml
# .github/workflows/verify.yml
name: VERIFY

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  verify:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup EDI
        uses: yourorg/setup-edi@v1
        with:
          recall-enabled: false  # CI doesn't need RECALL
          
      - name: Run VERIFY
        run: edi verify --ci --output json > verify-results.json
        
      - name: Upload Results
        uses: actions/upload-artifact@v4
        with:
          name: verify-results
          path: verify-results.json
          
      - name: Comment on PR
        if: github.event_name == 'pull_request'
        uses: yourorg/verify-pr-comment@v1
        with:
          results: verify-results.json
```

---

## 5. Implementation

### 5.1 Package Structure

```
edi/
├── internal/
│   ├── project/
│   │   ├── registry.go        # Project registry
│   │   ├── switch.go          # Project switching
│   │   └── domain.go          # Domain management
│   ├── integrations/
│   │   ├── jira/
│   │   │   ├── mcp.go         # Jira MCP server
│   │   │   └── client.go      # Jira API client
│   │   ├── calendar/
│   │   │   ├── mcp.go         # Calendar MCP server
│   │   │   └── google.go      # Google Calendar client
│   │   ├── slack/
│   │   │   ├── mcp.go         # Slack MCP server
│   │   │   └── client.go      # Slack API client
│   │   └── github/
│   │       ├── mcp.go         # GitHub MCP server
│   │       └── client.go      # GitHub API client
│   └── verify/
│       ├── pipeline.go        # VERIFY pipeline
│       ├── stages/
│       │   ├── lint.go
│       │   ├── test.go
│       │   ├── security.go
│       │   └── patterns.go
│       ├── selfcorrect.go     # Self-correction loop
│       └── ci.go              # CI/CD integration
```

### 5.2 Implementation Plan

#### Phase 5.1: Multi-Project Management (Week 1-2)

- [ ] Project registry
- [ ] Project registration on init
- [ ] `edi project` commands
- [ ] `edi switch` command
- [ ] Cross-project RECALL queries
- [ ] Domain management
- [ ] Knowledge promotion workflow

**Exit Criteria**: Can switch between projects and query cross-project knowledge.

#### Phase 5.2: External Integrations (Week 3)

- [ ] Integration configuration schema
- [ ] Jira MCP server
- [ ] Calendar MCP server
- [ ] Slack MCP server
- [ ] GitHub MCP server
- [ ] Briefing integration
- [ ] Custom integration support

**Exit Criteria**: Can use external integrations in sessions.

#### Phase 5.3: VERIFY Integration (Week 4)

- [ ] VERIFY configuration schema
- [ ] Lint stage
- [ ] Test stage
- [ ] Security stage
- [ ] Pattern stage
- [ ] Self-correction loop
- [ ] `edi verify` command
- [ ] CI/CD integration (GitHub Actions)
- [ ] RECALL capture of results

**Exit Criteria**: Can run VERIFY locally and in CI.

### 5.3 Validation Criteria

| Feature | Metric | Target |
|---------|--------|--------|
| Project switch | Time to new session | < 3s |
| Cross-project search | Query latency | < 500ms |
| Jira integration | API call | < 2s |
| VERIFY pipeline | Full run | < 60s |
| Self-correction | Per iteration | < 30s |

---

## Appendix A: Command Quick Reference

### Multi-Project Commands

| Command | Description |
|---------|-------------|
| `edi project list` | List registered projects |
| `edi project add <path>` | Register a project |
| `edi project remove <id>` | Unregister a project |
| `edi switch <project>` | Switch to another project |
| `edi switch -` | Switch to previous project |

### Integration Commands

| Command | Description |
|---------|-------------|
| `edi jira <ticket>` | Show Jira ticket details |
| `edi calendar` | Show today's schedule |
| `edi slack <channel> <message>` | Send Slack message |

### VERIFY Commands

| Command | Description |
|---------|-------------|
| `edi verify` | Run VERIFY pipeline |
| `edi verify --self-correct` | Run with self-correction |
| `edi verify --ci` | Run in CI mode |

---

## Appendix B: Related Specifications

| Spec | Relationship |
|------|--------------|
| RECALL MCP Server | Cross-project knowledge, scope hierarchy |
| Session Lifecycle | Project switch triggers session end |
| CLI Architecture | New commands |
| Workspace Configuration | Integration config, project registry |

---

## Appendix C: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 25, 2026 | Project registry in ~/.edi/ | Central location for global state |
| Jan 25, 2026 | Integrations as MCP servers | Consistent interface, Claude-native |
| Jan 25, 2026 | VERIFY separate from EDI | Different lifecycles, CI vs interactive |
| Jan 25, 2026 | Three-tier scope (project/domain/global) | Balances autonomy with shared knowledge |
| Jan 25, 2026 | Promotion requires approval | Prevents knowledge pollution |
| Jan 25, 2026 | Self-correction max 3 iterations | Prevent infinite loops, escalate early |
