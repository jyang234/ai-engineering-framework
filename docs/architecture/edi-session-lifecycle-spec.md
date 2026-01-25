# EDI Session Lifecycle Specification

**Status**: Draft  
**Created**: January 25, 2026  
**Version**: 0.3  
**Depends On**: Workspace & Configuration Spec v0.1, RECALL MCP Server Spec v0.3

---

## Table of Contents

1. [Overview](#1-overview)
2. [Session Lifecycle Flow](#2-session-lifecycle-flow)
3. [Flight Recorder](#3-flight-recorder)
4. [History System](#4-history-system)
5. [Briefing System](#5-briefing-system)
6. [Capture System](#6-capture-system)
7. [Data Flow & Integration](#7-data-flow--integration)
8. [Implementation](#8-implementation)
9. [Tasks Integration](#9-tasks-integration)

---

## 1. Overview

### Purpose

This specification defines the four interconnected systems that manage EDI session state:

| System | Purpose | When Active |
|--------|---------|-------------|
| **Flight Recorder** | Capture raw events during session | During session (continuous) |
| **History** | Store and retrieve session summaries | Session end (write), Session start (read) |
| **Briefing** | Generate contextual summary for Claude | Session start |
| **Capture** | Preserve significant learnings to RECALL | Session end |

### Core Principle

**History ≠ State.** Claude Code Tasks handles what's in progress. EDI History captures *why decisions were made* — the reasoning and context that would otherwise be lost between sessions.

### Key Relationships

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          SESSION START                                   │
│                                                                          │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│   │   Sessions   │    │   History    │    │    Tasks     │              │
│   │  (recent)    │    │  (curated)   │    │  (current)   │              │
│   └──────┬───────┘    └──────┬───────┘    └──────┬───────┘              │
│          │                   │                   │                       │
│          │            ┌──────┴───────┐           │                       │
│          │            │   RECALL     │           │                       │
│          │            │  (relevant)  │           │                       │
│          │            └──────┬───────┘           │                       │
│          └───────────────────┼───────────────────┘                       │
│                              ▼                                           │
│                    ┌─────────────────┐                                   │
│                    │    BRIEFING     │                                   │
│                    │   (generated)   │                                   │
│                    └─────────────────┘                                   │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                        DURING SESSION                                    │
│                                                                          │
│   Claude logs significant events via flight_recorder_log                │
│   └── .edi/sessions/{id}/events.jsonl                                   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                           SESSION END                                    │
│                                                                          │
│   ┌──────────────┐                                                       │
│   │   Session    │                                                       │
│   │   Events     │                                                       │
│   └──────┬───────┘                                                       │
│          │                                                               │
│          ▼                                                               │
│   ┌──────────────┐         ┌──────────────┐                             │
│   │   HISTORY    │         │   CAPTURE    │                             │
│   │   (summary)  │         │  (knowledge) │                             │
│   └──────┬───────┘         └──────┬───────┘                             │
│          │                        │                                      │
│          ▼                        ▼                                      │
│   .edi/history/            RECALL (via recall_add)                      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Session Lifecycle Flow

### 2.1 Complete Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  1. SESSION START                                                        │
│                                                                          │
│  User: $ edi                                                             │
│                                                                          │
│  EDI:                                                                    │
│  ├─ Detect project (.edi/ directory)                                    │
│  ├─ Load configuration (global + project merged)                        │
│  ├─ Load agent (default or specified)                                   │
│  ├─ Generate session ID                                                 │
│  ├─ Start RECALL MCP server (if not running)                            │
│  ├─ Read recent history (.edi/history/)                                 │
│  ├─ Read Claude Code Tasks (if available)                               │
│  ├─ Query RECALL for relevant context                                   │
│  ├─ Generate briefing                                                   │
│  └─ Launch Claude Code with context                                     │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  2. DURING SESSION                                                       │
│                                                                          │
│  • Claude Code handles task execution (native)                          │
│  • RECALL provides knowledge on demand (MCP tools)                      │
│  • Agent behaviors guide Claude (Skills)                                │
│  • User can switch agents (/plan, /build, /review)                      │
│  • Claude logs significant events via flight_recorder_log               │
│    └── Decisions, errors, milestones → .edi/sessions/{id}/events.jsonl │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  3. SESSION END                                                          │
│                                                                          │
│  User: /end                                                              │
│                                                                          │
│  EDI:                                                                    │
│  ├─ Generate session summary                                            │
│  ├─ Identify capture candidates                                         │
│  ├─ Present capture prompt (if candidates exist)                        │
│  │   └─ User: approve / edit / skip                                     │
│  ├─ Save approved items to RECALL                                       │
│  ├─ Save session summary to .edi/history/                               │
│  ├─ Apply retention policy (cleanup old history)                        │
│  └─ Exit                                                                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Session State

```go
// Session represents an EDI session
type Session struct {
    // Identity
    ID        string    `json:"id"`         // UUID
    ProjectID string    `json:"project_id"` // From .edi/config.yaml
    StartTime time.Time `json:"start_time"`
    EndTime   time.Time `json:"end_time,omitempty"`

    // Configuration
    Agent     string   `json:"agent"`      // Active agent name
    Skills    []string `json:"skills"`     // Loaded skills

    // Activity tracking
    TasksCompleted []string `json:"tasks_completed"` // Task IDs
    TasksStarted   []string `json:"tasks_started"`   // Task IDs
    FilesModified  []string `json:"files_modified"`  // File paths
    
    // Capture tracking
    CapturesSuggested int `json:"captures_suggested"`
    CapturesApproved  int `json:"captures_approved"`
    CapturesSkipped   int `json:"captures_skipped"`
}
```

---

## 3. Flight Recorder

### 3.1 Purpose

The flight recorder provides **local audit trail** and **session continuity** by capturing significant events during a session. Unlike History (curated summaries) or RECALL (organizational knowledge), the flight recorder captures raw events as they happen.

| Data Layer | Content | Retention | Scope |
|------------|---------|-----------|-------|
| **Flight Recorder** | Raw events (decisions, errors, milestones) | 30 days | Local only |
| **History** | Curated session summaries | Indefinite | Local only |
| **RECALL** | Promoted organizational knowledge | Indefinite | Searchable |

### 3.2 What Flight Recorder Captures

Claude uses the `flight_recorder_log` MCP tool to record:

| Event Type | When to Log | Example |
|------------|-------------|---------|
| `decision` | Architectural or implementation choices | "Chose exponential backoff for retry logic" |
| `error` | Errors encountered and how resolved | "Race condition in token refresh; fixed with mutex" |
| `milestone` | Significant progress points | "All tests passing for payment module" |
| `agent_switch` | When switching between agent modes | "Switching from architect to coder" |
| `observation` | Notable observations worth remembering | "Auth service response time degrades under load" |

### 3.3 Storage Structure

```
.edi/sessions/
├── 2026-01-24-abc123/
│   ├── events.jsonl          # Claude-reported significant events
│   ├── tools.jsonl           # Tool calls (from hooks, if available)
│   └── meta.json             # Session metadata
└── 2026-01-25-def456/
    └── ...
```

### 3.4 File Formats

**events.jsonl** (Claude self-reports via `flight_recorder_log` tool):

```jsonl
{"ts": "2026-01-24T10:16:00Z", "type": "decision", "content": "Chose exponential backoff for retry logic", "rationale": "Prevents thundering herd on service recovery"}
{"ts": "2026-01-24T10:45:00Z", "type": "error", "content": "Test failed: race condition in token refresh", "resolution": "Added mutex around refresh logic"}
{"ts": "2026-01-24T11:00:00Z", "type": "agent_switch", "from_agent": "architect", "to_agent": "coder", "content": "Switching to implementation mode"}
{"ts": "2026-01-24T11:30:00Z", "type": "milestone", "content": "Retry logic implementation complete, all tests passing"}
```

**tools.jsonl** (captured via hooks or post-processing, if available):

```jsonl
{"ts": "2026-01-24T10:16:30Z", "tool": "file_write", "path": "webhook.go", "lines_changed": 45}
{"ts": "2026-01-24T10:17:02Z", "tool": "bash", "command": "go test ./...", "exit_code": 0, "duration_ms": 1234}
{"ts": "2026-01-24T10:18:15Z", "tool": "recall_search", "query": "retry patterns", "results": 3}
```

**meta.json**:

```json
{
  "session_id": "2026-01-24-abc123",
  "started": "2026-01-24T10:15:00Z",
  "ended": "2026-01-24T12:30:00Z",
  "initial_agent": "architect",
  "agent_switches": ["architect", "coder", "reviewer"],
  "project": "payment-service",
  "ended_cleanly": true,
  "events_count": 12,
  "tools_count": 47
}
```

### 3.5 Retention Policy

| Data | Retention | Cleanup |
|------|-----------|---------|
| Flight recorder sessions | 30 days | Auto-purge on `edi` launch |
| History summaries | Indefinite | Manual only |
| RECALL knowledge | Indefinite | Manual only |

```go
// CleanupOldSessions removes flight recorder data older than retention period
func CleanupOldSessions(projectPath string, retentionDays int) error {
    sessionsDir := filepath.Join(projectPath, ".edi", "sessions")
    cutoff := time.Now().AddDate(0, 0, -retentionDays)
    
    entries, err := os.ReadDir(sessionsDir)
    if err != nil {
        return err
    }
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        metaPath := filepath.Join(sessionsDir, entry.Name(), "meta.json")
        meta, err := loadMeta(metaPath)
        if err != nil {
            continue // Skip if can't read
        }
        
        if meta.Ended.Before(cutoff) {
            os.RemoveAll(filepath.Join(sessionsDir, entry.Name()))
        }
    }
    
    return nil
}
```

### 3.6 Capture Method

**v1 Approach**: Claude self-reports + optional hook logging

1. **Claude self-reports** significant events via `flight_recorder_log` MCP tool
2. **Agent prompts** instruct Claude when to log (see below)
3. **Hooks** capture tool usage metadata (if Claude Code hooks support this)
4. **Post-processing** of Claude Code transcripts (if accessible — to investigate)

**Agent prompt guidance for flight recorder:**

```markdown
## Flight Recorder

Log significant events using the `flight_recorder_log` tool:

**ALWAYS log:**
- Architectural or implementation decisions (with rationale)
- Errors that required debugging (with resolution)
- Significant milestones (feature complete, tests passing)
- Agent mode switches

**DON'T log:**
- Routine operations (file reads, minor edits)
- Obvious steps (running tests after code changes)
- Information already in commit messages

Keep logs concise but include enough context to be useful in future briefings.
```

### 3.7 How Flight Recorder Feeds Briefings

At session start, EDI reads recent flight recorder data to generate richer briefings:

```python
def generate_briefing(project_path):
    briefing_sources = []
    
    # 1. Recent flight recorder events (detailed, recent)
    recent_sessions = get_sessions_since(project_path, days=7)
    for session in recent_sessions[-3:]:  # Last 3 sessions
        events = load_events(session.path)
        briefing_sources.append({
            "type": "recent_session",
            "date": session.date,
            "decisions": [e for e in events if e.type == "decision"],
            "errors": [e for e in events if e.type == "error"],
            "milestones": [e for e in events if e.type == "milestone"],
        })
    
    # 2. History summaries (curated, longer horizon)
    history = get_history(project_path, days=30)
    briefing_sources.append({
        "type": "history",
        "summaries": history
    })
    
    # 3. Current work state
    tasks = get_claude_tasks(project_path)
    briefing_sources.append({
        "type": "tasks",
        "in_progress": [t for t in tasks if t.status == "in_progress"],
        "recent_completed": [t for t in tasks if t.recently_completed]
    })
    
    # 4. RECALL context query
    recall_results = recall_search(infer_context_query(briefing_sources))
    briefing_sources.append({
        "type": "recall",
        "relevant": recall_results
    })
    
    return compile_briefing(briefing_sources)
```

### 3.8 Edge Cases

| Edge Case | Handling |
|-----------|----------|
| Session ends without `/end` | Next `edi` launch detects orphaned session (no `ended` in meta.json), prompts: "Last session ended unexpectedly. Would you like to review it?" |
| Multiple concurrent sessions | Each gets unique session ID based on timestamp + random suffix |
| Very long session (multi-day) | Single session directory; events accumulate; `/update` writes checkpoint marker |
| Sensitive data logged | User responsibility; events.jsonl stays local; review before promoting anything to RECALL |
| Flight recorder write fails | Log warning, continue session; not critical path |

### 3.9 What Flight Recorder Does NOT Do

| Flight Recorder Does | Flight Recorder Does NOT |
|---------------------|-------------------------|
| Capture Claude-selected significant events | Capture full conversation transcript |
| Store locally in `.edi/sessions/` | Upload to RECALL automatically |
| Feed into briefing generation | Replace History or RECALL |
| Auto-purge after 30 days | Persist indefinitely |

---

## 4. History System

### 4.1 Purpose

History stores **session summaries** — concise records of what happened and why. Unlike RECALL (which stores reusable knowledge), history captures the narrative of a specific work session.

### 4.2 What History Contains

| Content | Example |
|---------|---------|
| What was accomplished | "Implemented user profile API endpoints" |
| Decisions made | "Chose Stripe over Paddle for billing" |
| Questions raised | "How should we handle failed payments?" |
| Blockers encountered | "Waiting for API keys from finance team" |
| Files touched | `src/server/routes/users.ts`, `docs/adr/003-stripe.md` |

### 4.3 What History Does NOT Contain

| Not In History | Where It Lives |
|----------------|----------------|
| Task status (in progress, done) | Claude Code Tasks |
| Reusable patterns | RECALL (type: pattern) |
| Verified facts | RECALL (type: evidence) |
| Code diffs | Git |

### 4.4 Storage Format

**Location**: `.edi/history/{date}-{session_id}.md`

**Example**: `.edi/history/2026-01-24-a1b2c3d4.md`

```markdown
---
session_id: a1b2c3d4
date: 2026-01-24
start_time: "09:15:00"
end_time: "11:30:00"
duration_minutes: 135
agent: coder
skills:
  - coding
  - testing
tasks_completed:
  - task-001
  - task-002
  - task-003
tasks_started:
  - task-004
files_modified:
  - src/server/routes/users.ts
  - src/server/utils/dates.ts
  - src/shared/billing.ts
  - docs/adr/003-stripe-billing.md
captures:
  - type: decision
    id: dec-789
    summary: "Chose Stripe over Paddle"
  - type: evidence
    id: ev-456
    summary: "Profile API handles 500 req/s"
---

# Session Summary: January 24, 2026

## What We Did

1. **Implemented user profile API** (task-001)
   - Added GET /api/users/:id endpoint
   - Added PATCH /api/users/:id endpoint
   - Wrote unit tests for both

2. **Fixed date formatting bug** (task-002)
   - Issue was timezone handling in date utils
   - Added timezone-aware formatting function

3. **Started billing integration** (task-004)
   - Reviewed Stripe API documentation
   - Created initial types in `src/shared/billing.ts`
   - Blocked: Need API keys from finance team

## Decisions Made

- **Chose Stripe over Paddle** (captured → RECALL)
  - Better API documentation
  - More features we need (subscriptions, usage billing)
  - See ADR-003

## Open Questions

- How should we handle failed payments?
- Do we need webhook retry logic?

## Next Session

- Continue billing integration once API keys arrive
- Start on payment webhook handlers
```

### 4.5 History Schema (Go)

```go
package history

import "time"

// Entry represents a session history entry
type Entry struct {
    // Frontmatter fields
    SessionID       string    `yaml:"session_id"`
    Date            string    `yaml:"date"`            // YYYY-MM-DD
    StartTime       string    `yaml:"start_time"`      // HH:MM:SS
    EndTime         string    `yaml:"end_time"`        // HH:MM:SS
    DurationMinutes int       `yaml:"duration_minutes"`
    Agent           string    `yaml:"agent"`
    Skills          []string  `yaml:"skills"`
    TasksCompleted  []string  `yaml:"tasks_completed"`
    TasksStarted    []string  `yaml:"tasks_started"`
    FilesModified   []string  `yaml:"files_modified"`
    Captures        []Capture `yaml:"captures"`

    // Body content (markdown)
    Body string `yaml:"-"`
}

// Capture records an item captured to RECALL
type Capture struct {
    Type    string `yaml:"type"`    // decision, evidence, pattern, etc.
    ID      string `yaml:"id"`      // RECALL document ID
    Summary string `yaml:"summary"` // One-line description
}

// EntryFilter for querying history
type EntryFilter struct {
    Since     time.Time // Entries after this time
    Until     time.Time // Entries before this time
    Agent     string    // Filter by agent
    Limit     int       // Max entries to return
    HasCaptures bool    // Only entries with captures
}
```

### 4.6 History Operations

```go
package history

import (
    "context"
    "os"
    "path/filepath"
    "sort"
    "time"
)

// Store manages history entries
type Store struct {
    projectPath string
}

// NewStore creates a history store for a project
func NewStore(projectPath string) *Store {
    return &Store{projectPath: projectPath}
}

func (s *Store) historyDir() string {
    return filepath.Join(s.projectPath, ".edi", "history")
}

// Save writes a history entry
func (s *Store) Save(ctx context.Context, entry *Entry) error {
    // Ensure directory exists
    if err := os.MkdirAll(s.historyDir(), 0755); err != nil {
        return fmt.Errorf("creating history dir: %w", err)
    }

    // Generate filename
    filename := fmt.Sprintf("%s-%s.md", entry.Date, entry.SessionID[:8])
    path := filepath.Join(s.historyDir(), filename)

    // Render to markdown
    content, err := renderEntry(entry)
    if err != nil {
        return fmt.Errorf("rendering entry: %w", err)
    }

    // Write file
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        return fmt.Errorf("writing entry: %w", err)
    }

    return nil
}

// List returns history entries matching filter
func (s *Store) List(ctx context.Context, filter *EntryFilter) ([]*Entry, error) {
    entries, err := s.loadAll(ctx)
    if err != nil {
        return nil, err
    }

    // Apply filters
    var filtered []*Entry
    for _, e := range entries {
        if s.matchesFilter(e, filter) {
            filtered = append(filtered, e)
        }
    }

    // Sort by date descending (most recent first)
    sort.Slice(filtered, func(i, j int) bool {
        return filtered[i].Date > filtered[j].Date
    })

    // Apply limit
    if filter.Limit > 0 && len(filtered) > filter.Limit {
        filtered = filtered[:filter.Limit]
    }

    return filtered, nil
}

// Recent returns the N most recent history entries
func (s *Store) Recent(ctx context.Context, n int) ([]*Entry, error) {
    return s.List(ctx, &EntryFilter{Limit: n})
}

// Cleanup removes entries older than retention policy
func (s *Store) Cleanup(ctx context.Context, maxAge time.Duration, maxEntries int) error {
    entries, err := s.loadAll(ctx)
    if err != nil {
        return err
    }

    // Sort by date descending
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].Date > entries[j].Date
    })

    cutoff := time.Now().Add(-maxAge)
    
    for i, e := range entries {
        entryDate, _ := time.Parse("2006-01-02", e.Date)
        
        // Delete if over max entries OR older than max age
        if i >= maxEntries || entryDate.Before(cutoff) {
            path := filepath.Join(s.historyDir(), 
                fmt.Sprintf("%s-%s.md", e.Date, e.SessionID[:8]))
            os.Remove(path)
        }
    }

    return nil
}
```

### 4.7 Summary Generation

The session summary is generated by Claude at session end:

```go
package history

// SummaryRequest contains data for summary generation
type SummaryRequest struct {
    Session       *Session
    TasksContext  string   // From Claude Code Tasks
    FilesDiff     []string // Changed files summary
    Conversations string   // Key conversation excerpts
}

// GenerateSummaryPrompt creates the prompt for Claude
func GenerateSummaryPrompt(req *SummaryRequest) string {
    return fmt.Sprintf(`Generate a session summary for this EDI session.

## Session Info
- Duration: %d minutes
- Agent: %s
- Tasks completed: %d
- Tasks started: %d
- Files modified: %d

## Tasks Context
%s

## Files Changed
%s

## Instructions
Write a concise session summary with:
1. **What We Did** - List main accomplishments (2-5 items)
2. **Decisions Made** - Any choices with rationale
3. **Open Questions** - Unresolved issues for next session
4. **Next Session** - Suggested continuation points

Format as markdown. Be specific but concise. Focus on decisions and reasoning, not routine operations.`,
        req.Session.EndTime.Sub(req.Session.StartTime).Minutes(),
        req.Session.Agent,
        len(req.Session.TasksCompleted),
        len(req.Session.TasksStarted),
        len(req.Session.FilesModified),
        req.TasksContext,
        strings.Join(req.FilesDiff, "\n"),
    )
}
```

---

## 5. Briefing System

### 5.1 Purpose

Briefings provide Claude with **proactive context** at session start — the information needed to continue work effectively without the user having to re-explain everything.

### 5.2 Briefing Sources

| Source | What It Provides | Priority |
|--------|------------------|----------|
| **Profile** | Project overview, architecture, conventions | High |
| **History** | Recent decisions, open questions, blockers | High |
| **Tasks** | Current task state, in-progress work | High |
| **RECALL** | Relevant knowledge for likely tasks | Medium |

### 5.3 Briefing Format

```markdown
# EDI Briefing: my-awesome-project

## Project Context
[From .edi/profile.md - summarized]

TypeScript web application with React frontend and Node.js backend.
Key patterns: Repository pattern for data access, Zustand for state.

## Recent Sessions

### Yesterday (Jan 24)
- Implemented user profile API
- Started billing integration (blocked on API keys)
- Decision: Chose Stripe over Paddle

### 2 Days Ago (Jan 23)
- Set up authentication flow
- Created user registration endpoint

## Open Questions
- How should we handle failed payments?
- Do we need webhook retry logic?

## Current Tasks
[From Claude Code Tasks]

- **In Progress**: Billing integration (TICKET-789)
- **Blocked**: Waiting for Stripe API keys
- **Next Up**: Payment webhook handlers

## Relevant Knowledge
[From RECALL - auto-queried]

- ADR-003: Stripe billing decision (captured yesterday)
- Pattern: Error handling for external APIs
- Evidence: Auth service handles 1000 req/s

---

Ready to continue. What would you like to work on?
```

### 5.4 Briefing Generation

```go
package briefing

import (
    "context"
    "strings"
)

// Generator creates session briefings
type Generator struct {
    history *history.Store
    recall  *recall.Client
    tasks   *tasks.Reader
    config  *config.Config
}

// Briefing represents a generated briefing
type Briefing struct {
    ProjectContext  string
    RecentSessions  []SessionSummary
    OpenQuestions   []string
    CurrentTasks    []TaskSummary
    RelevantKnowledge []KnowledgeItem
    GeneratedAt     time.Time
}

// SessionSummary is a condensed history entry
type SessionSummary struct {
    Date          string
    Accomplishments []string
    Decisions     []string
}

// TaskSummary is a condensed task state
type TaskSummary struct {
    ID          string
    Title       string
    Status      string // in_progress, blocked, next_up
    BlockedBy   string // If blocked
}

// KnowledgeItem is a condensed RECALL result
type KnowledgeItem struct {
    Type    string // decision, evidence, pattern
    Summary string
    ID      string
}

// Generate creates a briefing for session start
func (g *Generator) Generate(ctx context.Context, projectPath string) (*Briefing, error) {
    briefing := &Briefing{
        GeneratedAt: time.Now(),
    }

    // 1. Load project profile
    profile, err := g.loadProfile(projectPath)
    if err == nil {
        briefing.ProjectContext = summarizeProfile(profile)
    }

    // 2. Load recent history
    depth := g.config.Briefing.HistoryDepth
    entries, err := g.history.Recent(ctx, depth)
    if err == nil {
        briefing.RecentSessions = summarizeHistory(entries)
        briefing.OpenQuestions = extractOpenQuestions(entries)
    }

    // 3. Load current tasks (if Claude Code Tasks available)
    if g.config.Briefing.Sources.Tasks {
        tasks, err := g.tasks.GetCurrent(ctx, projectPath)
        if err == nil {
            briefing.CurrentTasks = summarizeTasks(tasks)
        }
    }

    // 4. Query RECALL for relevant knowledge
    if g.config.Briefing.Sources.Recall && g.config.Briefing.RecallAutoQuery {
        // Build query from recent context
        query := buildContextQuery(briefing.RecentSessions, briefing.CurrentTasks)
        
        results, err := g.recall.Search(ctx, &recall.SearchOptions{
            Query: query,
            Scope: "project",
            Types: []string{"decision", "evidence", "pattern"},
            Limit: 5,
        })
        if err == nil {
            briefing.RelevantKnowledge = summarizeKnowledge(results)
        }
    }

    return briefing, nil
}

// Render converts briefing to markdown
func (b *Briefing) Render(projectName string) string {
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("# EDI Briefing: %s\n\n", projectName))

    // Project context
    if b.ProjectContext != "" {
        sb.WriteString("## Project Context\n\n")
        sb.WriteString(b.ProjectContext)
        sb.WriteString("\n\n")
    }

    // Recent sessions
    if len(b.RecentSessions) > 0 {
        sb.WriteString("## Recent Sessions\n\n")
        for _, s := range b.RecentSessions {
            sb.WriteString(fmt.Sprintf("### %s\n", s.Date))
            for _, a := range s.Accomplishments {
                sb.WriteString(fmt.Sprintf("- %s\n", a))
            }
            for _, d := range s.Decisions {
                sb.WriteString(fmt.Sprintf("- Decision: %s\n", d))
            }
            sb.WriteString("\n")
        }
    }

    // Open questions
    if len(b.OpenQuestions) > 0 {
        sb.WriteString("## Open Questions\n\n")
        for _, q := range b.OpenQuestions {
            sb.WriteString(fmt.Sprintf("- %s\n", q))
        }
        sb.WriteString("\n")
    }

    // Current tasks
    if len(b.CurrentTasks) > 0 {
        sb.WriteString("## Current Tasks\n\n")
        for _, t := range b.CurrentTasks {
            status := t.Status
            if t.BlockedBy != "" {
                status = fmt.Sprintf("Blocked: %s", t.BlockedBy)
            }
            sb.WriteString(fmt.Sprintf("- **%s**: %s (%s)\n", t.Status, t.Title, t.ID))
        }
        sb.WriteString("\n")
    }

    // Relevant knowledge
    if len(b.RelevantKnowledge) > 0 {
        sb.WriteString("## Relevant Knowledge\n\n")
        for _, k := range b.RelevantKnowledge {
            sb.WriteString(fmt.Sprintf("- [%s] %s\n", k.Type, k.Summary))
        }
        sb.WriteString("\n")
    }

    sb.WriteString("---\n\nReady to continue. What would you like to work on?\n")

    return sb.String()
}
```

### 5.5 Context Query Building

```go
// buildContextQuery creates a RECALL query from session context
func buildContextQuery(sessions []SessionSummary, tasks []TaskSummary) string {
    var terms []string

    // Extract key terms from recent sessions
    for _, s := range sessions {
        for _, a := range s.Accomplishments {
            terms = append(terms, extractKeywords(a)...)
        }
    }

    // Extract terms from current tasks
    for _, t := range tasks {
        terms = append(terms, extractKeywords(t.Title)...)
    }

    // Deduplicate and limit
    seen := make(map[string]bool)
    var unique []string
    for _, t := range terms {
        if !seen[t] {
            seen[t] = true
            unique = append(unique, t)
        }
    }

    // Return top keywords as query
    if len(unique) > 10 {
        unique = unique[:10]
    }
    
    return strings.Join(unique, " ")
}
```

### 5.6 Briefing Configuration

From `config.yaml`:

```yaml
briefing:
  # Data sources to include
  sources:
    tasks: true           # Claude Code Tasks
    history: true         # Recent session history
    recall: true          # RECALL context query

  # How many history entries to include
  history_depth: 3

  # Auto-query RECALL on session start
  recall_auto_query: true

  # Custom queries to always run (project config)
  recall_queries:
    - "project architecture overview"
    - "recent ADRs"

  # Files to always include context from
  include_files:
    - docs/ARCHITECTURE.md
    - docs/adr/
```

---

## 6. Capture System

### 6.1 Purpose

Capture identifies and preserves **significant learnings** from a session into RECALL for future retrieval. This is the primary way organizational knowledge grows.

### 6.2 What Gets Captured

| Type | Confidence | Auto-Capture? | User Prompt? |
|------|------------|---------------|--------------|
| **Decision** | High | If ADR created | Otherwise yes |
| **Evidence** | Very High | Yes (Sandbox verified) | No |
| **Pattern** | Medium | No | Yes |
| **Observation** | Lower | No | Yes |
| **Failure** | Special | If self-corrected | If user-corrected |

### 6.3 Capture Candidates Detection

```go
package capture

// CandidateType represents the type of capture candidate
type CandidateType string

const (
    CandidateDecision    CandidateType = "decision"
    CandidateEvidence    CandidateType = "evidence"
    CandidatePattern     CandidateType = "pattern"
    CandidateObservation CandidateType = "observation"
    CandidateFailure     CandidateType = "failure"
)

// Candidate represents a potential capture item
type Candidate struct {
    Type        CandidateType
    Confidence  float64  // 0.0-1.0
    Summary     string   // One-line description
    Detail      string   // Full context
    Source      string   // Where this was detected
    PreSelected bool     // Should be pre-checked in UI
}

// Detector identifies capture candidates from session activity
type Detector struct {
    config *config.CaptureConfig
}

// DetectFromSession analyzes session for capture candidates
func (d *Detector) DetectFromSession(ctx context.Context, session *Session) ([]*Candidate, error) {
    var candidates []*Candidate

    // 1. Check for ADR creation/modification
    for _, f := range session.FilesModified {
        if isADRPath(f) {
            candidates = append(candidates, &Candidate{
                Type:        CandidateDecision,
                Confidence:  0.95,
                Summary:     fmt.Sprintf("ADR created/modified: %s", filepath.Base(f)),
                Source:      f,
                PreSelected: true,
            })
        }
    }

    // 2. Check for external API integrations
    for _, f := range session.FilesModified {
        if hasExternalAPIPatterns(f) {
            candidates = append(candidates, &Candidate{
                Type:        CandidatePattern,
                Confidence:  0.7,
                Summary:     "External API integration pattern",
                Source:      f,
                PreSelected: false,
            })
        }
    }

    // 3. Analyze conversation for decisions
    decisions := extractDecisions(session.Conversations)
    for _, dec := range decisions {
        candidates = append(candidates, &Candidate{
            Type:        CandidateDecision,
            Confidence:  dec.Confidence,
            Summary:     dec.Summary,
            Detail:      dec.Rationale,
            Source:      "conversation",
            PreSelected: dec.Confidence > 0.8,
        })
    }

    // 4. Check for self-corrections (failures that were fixed)
    corrections := extractSelfCorrections(session.ToolCalls)
    for _, corr := range corrections {
        candidates = append(candidates, &Candidate{
            Type:        CandidateFailure,
            Confidence:  0.85,
            Summary:     corr.Summary,
            Detail:      corr.Resolution,
            Source:      "self-correction",
            PreSelected: true,
        })
    }

    // Filter by confidence threshold
    var filtered []*Candidate
    threshold := d.config.MinConfidence
    for _, c := range candidates {
        if c.Confidence >= threshold {
            filtered = append(filtered, c)
        }
    }

    return filtered, nil
}

func isADRPath(path string) bool {
    return strings.Contains(path, "/adr/") || 
           strings.Contains(path, "/ADR/") ||
           strings.HasPrefix(filepath.Base(path), "ADR-")
}
```

### 6.4 Capture Prompt UX

At session end, EDI presents capture candidates:

```
┌─────────────────────────────────────────────────────────────────────────┐
│  EDI detected capture-worthy items:                                      │
│                                                                          │
│  ✓ [Decision] Chose event-driven architecture for OrderService          │
│    → "Decouples order processing from payment confirmation"              │
│                                                                          │
│  ✓ [Failure] charge() deprecated; use processPayment()                  │
│    → Self-corrected after API error                                      │
│                                                                          │
│  ? [Pattern] Used repository pattern for data access                     │
│    → "Abstracts database queries behind interface"                       │
│                                                                          │
│  Actions:                                                                │
│  [1] Save Selected  [2] Edit  [3] Skip All  [4] Preferences             │
│                                                                          │
│  Enter choice (or item number to toggle): _                              │
└─────────────────────────────────────────────────────────────────────────┘
```

**Legend:**
- `✓` = Pre-selected (high confidence)
- `?` = Suggested but not pre-selected (medium confidence)

### 6.5 Capture Prompt Implementation

```go
package capture

import (
    "bufio"
    "fmt"
    "os"
    "strings"
)

// Prompt handles user interaction for capture decisions
type Prompt struct {
    candidates []*Candidate
    selected   map[int]bool
}

// NewPrompt creates a capture prompt
func NewPrompt(candidates []*Candidate) *Prompt {
    selected := make(map[int]bool)
    for i, c := range candidates {
        selected[i] = c.PreSelected
    }
    return &Prompt{
        candidates: candidates,
        selected:   selected,
    }
}

// Run executes the interactive prompt
func (p *Prompt) Run() ([]*Candidate, error) {
    reader := bufio.NewReader(os.Stdin)

    for {
        p.render()
        
        fmt.Print("Enter choice (or item number to toggle): ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)

        switch input {
        case "1", "save":
            return p.getSelected(), nil
        case "2", "edit":
            p.editMode(reader)
        case "3", "skip":
            return nil, nil
        case "4", "prefs":
            p.showPreferences()
        default:
            // Try to parse as item number
            if idx, err := strconv.Atoi(input); err == nil && idx > 0 && idx <= len(p.candidates) {
                p.selected[idx-1] = !p.selected[idx-1]
            }
        }
    }
}

func (p *Prompt) render() {
    fmt.Println("\n┌─────────────────────────────────────────────────────────────────────────┐")
    fmt.Println("│  EDI detected capture-worthy items:                                      │")
    fmt.Println("│                                                                          │")

    for i, c := range p.candidates {
        checkbox := "?"
        if p.selected[i] {
            checkbox = "✓"
        }
        
        fmt.Printf("│  %d. %s [%s] %s\n", i+1, checkbox, c.Type, c.Summary)
        if c.Detail != "" {
            fmt.Printf("│     → \"%s\"\n", truncate(c.Detail, 60))
        }
        fmt.Println("│")
    }

    fmt.Println("│  Actions:                                                                │")
    fmt.Println("│  [1] Save Selected  [2] Edit  [3] Skip All  [4] Preferences             │")
    fmt.Println("└─────────────────────────────────────────────────────────────────────────┘")
}

func (p *Prompt) getSelected() []*Candidate {
    var result []*Candidate
    for i, selected := range p.selected {
        if selected {
            result = append(result, p.candidates[i])
        }
    }
    return result
}
```

### 6.6 Saving to RECALL

```go
package capture

import (
    "context"
)

// Saver persists approved captures to RECALL
type Saver struct {
    recall *recall.Client
}

// Save stores approved candidates in RECALL
func (s *Saver) Save(ctx context.Context, session *Session, candidates []*Candidate) ([]string, error) {
    var savedIDs []string

    for _, c := range candidates {
        doc := &recall.AddOptions{
            Type:    string(c.Type),
            Summary: c.Summary,
            Detail:  c.Detail,
            Scope:   recall.ScopeProject, // Default; could be promoted later
            Metadata: map[string]string{
                "source_session": session.ID,
                "source":         c.Source,
                "confidence":     fmt.Sprintf("%.2f", c.Confidence),
            },
        }

        result, err := s.recall.Add(ctx, doc)
        if err != nil {
            return savedIDs, fmt.Errorf("saving %s: %w", c.Summary, err)
        }

        savedIDs = append(savedIDs, result.ID)
    }

    return savedIDs, nil
}
```

### 6.7 Friction Budget

To prevent prompt fatigue, EDI limits interactions per session:

```go
package capture

// FrictionBudget tracks interaction costs
type FrictionBudget struct {
    MaxInteractions int
    Used            int
}

// NewFrictionBudget creates a budget based on session length
func NewFrictionBudget(durationMinutes int) *FrictionBudget {
    var max int
    switch {
    case durationMinutes < 30:
        max = 2  // Short session
    case durationMinutes < 90:
        max = 4  // Medium session
    default:
        max = 6  // Long session
    }
    return &FrictionBudget{MaxInteractions: max}
}

// InteractionCost defines cost of different interactions
var InteractionCost = map[string]int{
    "briefing_review":    1, // Reviewing briefing
    "capture_prompt":     1, // End-of-session capture
    "mid_session_prompt": 2, // Interruption penalty
    "failure_review":     1, // Reviewing failure
}

// CanInteract checks if budget allows interaction
func (b *FrictionBudget) CanInteract(interactionType string) bool {
    cost := InteractionCost[interactionType]
    return b.Used+cost <= b.MaxInteractions
}

// Spend records an interaction
func (b *FrictionBudget) Spend(interactionType string) {
    b.Used += InteractionCost[interactionType]
}

// Remaining returns available budget
func (b *FrictionBudget) Remaining() int {
    return b.MaxInteractions - b.Used
}
```

### 6.8 Silent Capture

Some items are captured without prompting:

```go
// SilentCapture handles auto-capture of high-confidence items
func (s *Saver) SilentCapture(ctx context.Context, session *Session) error {
    // ADRs are always captured
    for _, f := range session.FilesModified {
        if isADRPath(f) {
            content, err := os.ReadFile(f)
            if err != nil {
                continue
            }
            
            s.recall.Add(ctx, &recall.AddOptions{
                Type:    "decision",
                Summary: extractADRTitle(content),
                Detail:  string(content),
                Scope:   recall.ScopeProject,
                Metadata: map[string]string{
                    "source_file":    f,
                    "source_session": session.ID,
                    "auto_captured":  "true",
                },
            })
        }
    }

    // Self-corrections are captured as failures
    for _, corr := range session.SelfCorrections {
        s.recall.Add(ctx, &recall.AddOptions{
            Type:    "failure",
            Summary: corr.Summary,
            Detail:  corr.Resolution,
            Scope:   recall.ScopeProject,
            Metadata: map[string]string{
                "source_session":    session.ID,
                "attribution":       "self_correction",
                "auto_captured":     "true",
            },
        })
    }

    return nil
}
```

### 6.9 Noise Control

To prevent RECALL from accumulating low-value or redundant items, EDI implements several noise control mechanisms.

#### 6.9.1 Capacity Management

```go
package capture

// CapacityChecker monitors RECALL storage limits
type CapacityChecker struct {
    recall *recall.Client
    config *config.CapacityConfig
}

// CapacityConfig defines limits per scope
type CapacityConfig struct {
    ProjectLimit   int     // Max items per project (default: 500)
    GlobalLimit    int     // Max global items (default: 1000)
    WarningPercent float64 // Warn at this % of capacity (default: 0.8)
}

// Status represents current capacity state
type CapacityStatus struct {
    Scope       string
    CurrentCount int
    Limit       int
    Percent     float64
    Warning     bool
    Full        bool
}

// Check returns capacity status for a scope
func (c *CapacityChecker) Check(ctx context.Context, scope string) (*CapacityStatus, error) {
    count, err := c.recall.Count(ctx, scope)
    if err != nil {
        return nil, err
    }

    limit := c.config.ProjectLimit
    if scope == "global" {
        limit = c.config.GlobalLimit
    }

    percent := float64(count) / float64(limit)
    
    return &CapacityStatus{
        Scope:        scope,
        CurrentCount: count,
        Limit:        limit,
        Percent:      percent,
        Warning:      percent >= c.config.WarningPercent,
        Full:         count >= limit,
    }, nil
}

// WarnIfNeeded prints capacity warning if approaching limit
func (c *CapacityChecker) WarnIfNeeded(ctx context.Context, scope string) {
    status, err := c.Check(ctx, scope)
    if err != nil {
        return
    }

    if status.Full {
        fmt.Printf("⚠️ RECALL is at capacity (%d/%d items in %s scope)\n", 
            status.CurrentCount, status.Limit, scope)
        fmt.Println("   Consider archiving old or low-usefulness items.")
        fmt.Println("   Run: edi recall cleanup --suggest")
    } else if status.Warning {
        fmt.Printf("⚠️ RECALL is at %.0f%% capacity (%d/%d items in %s scope)\n", 
            status.Percent*100, status.CurrentCount, status.Limit, scope)
    }
}
```

#### 6.9.2 Deduplication

Before adding items to RECALL, check for duplicates:

```go
package capture

// DeduplicationResult indicates whether an item is a duplicate
type DeduplicationResult struct {
    IsDuplicate bool
    ExistingID  string
    Similarity  float64
    Suggestion  string // "merge", "keep_both", "skip"
}

// Deduplicator checks for similar existing items
type Deduplicator struct {
    recall *recall.Client
}

// CheckDuplicate looks for items with matching summary (exact match for v0)
func (d *Deduplicator) CheckDuplicate(ctx context.Context, candidate *Candidate) (*DeduplicationResult, error) {
    // v0: Exact match on normalized summary
    normalizedSummary := normalizeSummary(candidate.Summary)
    
    existing, err := d.recall.Search(ctx, &recall.SearchOptions{
        Query:  normalizedSummary,
        Limit:  5,
        Types:  []string{string(candidate.Type)},
    })
    if err != nil {
        return nil, err
    }

    for _, item := range existing.Results {
        if normalizeSummary(item.Summary) == normalizedSummary {
            return &DeduplicationResult{
                IsDuplicate: true,
                ExistingID:  item.ID,
                Similarity:  1.0,
                Suggestion:  "skip",
            }, nil
        }
        
        // v0: Simple substring check for high similarity
        if containsSubstantialOverlap(normalizedSummary, normalizeSummary(item.Summary)) {
            return &DeduplicationResult{
                IsDuplicate: true,
                ExistingID:  item.ID,
                Similarity:  0.85,
                Suggestion:  "merge",
            }, nil
        }
    }

    return &DeduplicationResult{IsDuplicate: false}, nil
}

func normalizeSummary(s string) string {
    s = strings.ToLower(s)
    s = strings.TrimSpace(s)
    // Remove common filler words
    s = regexp.MustCompile(`\b(the|a|an|is|are|was|were|be|been)\b`).ReplaceAllString(s, "")
    // Normalize whitespace
    s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
    return strings.TrimSpace(s)
}

func containsSubstantialOverlap(a, b string) bool {
    // Simple heuristic: if 80% of words match, consider it overlap
    wordsA := strings.Fields(a)
    wordsB := strings.Fields(b)
    
    if len(wordsA) == 0 || len(wordsB) == 0 {
        return false
    }
    
    matches := 0
    for _, wa := range wordsA {
        for _, wb := range wordsB {
            if wa == wb {
                matches++
                break
            }
        }
    }
    
    overlapA := float64(matches) / float64(len(wordsA))
    overlapB := float64(matches) / float64(len(wordsB))
    
    return overlapA >= 0.8 || overlapB >= 0.8
}
```

#### 6.9.3 Updated Saver with Noise Control

```go
package capture

// Saver persists approved captures to RECALL with noise control
type Saver struct {
    recall      *recall.Client
    capacity    *CapacityChecker
    dedup       *Deduplicator
}

// Save stores approved candidates in RECALL with dedup and capacity checks
func (s *Saver) Save(ctx context.Context, session *Session, candidates []*Candidate) (*SaveResult, error) {
    result := &SaveResult{}

    // Check capacity before saving
    status, err := s.capacity.Check(ctx, "project")
    if err != nil {
        return nil, fmt.Errorf("checking capacity: %w", err)
    }
    
    if status.Full {
        return nil, fmt.Errorf("RECALL is at capacity (%d/%d items). Archive old items first.", 
            status.CurrentCount, status.Limit)
    }

    for _, c := range candidates {
        // Check for duplicates
        dupResult, err := s.dedup.CheckDuplicate(ctx, c)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("dedup check for %s: %v", c.Summary, err))
            continue
        }

        if dupResult.IsDuplicate {
            if dupResult.Suggestion == "skip" {
                result.Skipped = append(result.Skipped, SkippedItem{
                    Summary:    c.Summary,
                    Reason:     "duplicate",
                    ExistingID: dupResult.ExistingID,
                })
                continue
            }
            // For "merge" suggestions in v0, we skip and note it
            // v1 will support actual merging
            result.Skipped = append(result.Skipped, SkippedItem{
                Summary:    c.Summary,
                Reason:     "similar_exists",
                ExistingID: dupResult.ExistingID,
            })
            continue
        }

        // Save to RECALL
        doc := &recall.AddOptions{
            Type:    string(c.Type),
            Summary: c.Summary,
            Detail:  c.Detail,
            Scope:   recall.ScopeProject,
            Metadata: map[string]string{
                "source_session": session.ID,
                "source":         c.Source,
                "confidence":     fmt.Sprintf("%.2f", c.Confidence),
            },
        }

        addResult, err := s.recall.Add(ctx, doc)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("saving %s: %v", c.Summary, err))
            continue
        }

        result.SavedIDs = append(result.SavedIDs, addResult.ID)
    }

    // Warn if approaching capacity after save
    s.capacity.WarnIfNeeded(ctx, "project")

    return result, nil
}

// SaveResult captures what happened during save
type SaveResult struct {
    SavedIDs []string
    Skipped  []SkippedItem
    Errors   []string
}

// SkippedItem records why an item wasn't saved
type SkippedItem struct {
    Summary    string
    Reason     string // "duplicate", "similar_exists", "capacity"
    ExistingID string
}
```

#### 6.9.4 Configuration

```yaml
# .edi/config.yaml
capture:
  # Noise control
  capacity:
    project_limit: 500      # Max items per project
    global_limit: 1000      # Max global items
    warning_percent: 0.8    # Warn at 80%
    
  deduplication:
    enabled: true
    # v0: exact match + word overlap
    # v1: semantic similarity via Codex
    
  # Existing settings
  min_confidence: 0.6
  friction_budget:
    short_session: 2
    medium_session: 4
    long_session: 6
```

---

## 7. Data Flow ## 6. Data Flow & Integration Integration

### 7.1 Session Start Flow

```go
package session

import (
    "context"
)

// Manager orchestrates session lifecycle
type Manager struct {
    config   *config.Config
    history  *history.Store
    briefing *briefing.Generator
    capture  *capture.Detector
    recall   *recall.Client
}

// Start initializes a new EDI session
func (m *Manager) Start(ctx context.Context, projectPath string) (*Session, *briefing.Briefing, error) {
    // 1. Create session
    session := &Session{
        ID:        generateSessionID(),
        ProjectID: m.config.Project.Name,
        StartTime: time.Now(),
        Agent:     m.config.Defaults.Agent,
        Skills:    m.config.Defaults.Skills,
    }

    // 2. Generate briefing
    brief, err := m.briefing.Generate(ctx, projectPath)
    if err != nil {
        // Non-fatal; continue without briefing
        log.Printf("Warning: briefing generation failed: %v", err)
    }

    // 3. Start RECALL MCP server if configured
    if m.config.Recall.AutoStart {
        if err := m.recall.EnsureRunning(ctx); err != nil {
            log.Printf("Warning: RECALL server failed to start: %v", err)
        }
    }

    return session, brief, nil
}
```

### 7.2 Session End Flow

```go
// End finalizes a session
func (m *Manager) End(ctx context.Context, session *Session) error {
    session.EndTime = time.Now()

    // 1. Silent capture (ADRs, self-corrections)
    saver := &capture.Saver{recall: m.recall}
    if err := saver.SilentCapture(ctx, session); err != nil {
        log.Printf("Warning: silent capture failed: %v", err)
    }

    // 2. Detect capture candidates
    detector := &capture.Detector{config: m.config.Capture}
    candidates, err := detector.DetectFromSession(ctx, session)
    if err != nil {
        log.Printf("Warning: capture detection failed: %v", err)
    }

    // 3. Prompt for capture (if candidates exist and budget allows)
    var captures []*capture.Candidate
    if len(candidates) > 0 && m.config.Capture.PromptOnEnd {
        budget := capture.NewFrictionBudget(int(session.EndTime.Sub(session.StartTime).Minutes()))
        
        if budget.CanInteract("capture_prompt") {
            prompt := capture.NewPrompt(candidates)
            captures, _ = prompt.Run()
            budget.Spend("capture_prompt")
        }
    }

    // 4. Save approved captures to RECALL
    var captureRecords []history.Capture
    if len(captures) > 0 {
        savedIDs, err := saver.Save(ctx, session, captures)
        if err != nil {
            log.Printf("Warning: capture save failed: %v", err)
        }
        
        for i, c := range captures {
            captureRecords = append(captureRecords, history.Capture{
                Type:    string(c.Type),
                ID:      savedIDs[i],
                Summary: c.Summary,
            })
        }
    }

    // 5. Generate session summary
    summary, err := generateSummary(ctx, session)
    if err != nil {
        return fmt.Errorf("generating summary: %w", err)
    }

    // 6. Save history entry
    entry := &history.Entry{
        SessionID:       session.ID,
        Date:            session.StartTime.Format("2006-01-02"),
        StartTime:       session.StartTime.Format("15:04:05"),
        EndTime:         session.EndTime.Format("15:04:05"),
        DurationMinutes: int(session.EndTime.Sub(session.StartTime).Minutes()),
        Agent:           session.Agent,
        Skills:          session.Skills,
        TasksCompleted:  session.TasksCompleted,
        TasksStarted:    session.TasksStarted,
        FilesModified:   session.FilesModified,
        Captures:        captureRecords,
        Body:            summary,
    }

    if err := m.history.Save(ctx, entry); err != nil {
        return fmt.Errorf("saving history: %w", err)
    }

    // 7. Apply retention policy
    maxAge := time.Duration(m.config.History.Retention.MaxAgeDays) * 24 * time.Hour
    maxEntries := m.config.History.Retention.MaxEntries
    if err := m.history.Cleanup(ctx, maxAge, maxEntries); err != nil {
        log.Printf("Warning: history cleanup failed: %v", err)
    }

    return nil
}
```

### 7.3 Claude Code Tasks Integration

```go
package tasks

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// Reader reads Claude Code task state
type Reader struct{}

// Task represents a Claude Code task
type Task struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Status      string   `json:"status"` // pending, in_progress, completed, blocked
    BlockedBy   string   `json:"blocked_by,omitempty"`
    DependsOn   []string `json:"depends_on,omitempty"`
    CreatedAt   string   `json:"created_at"`
    CompletedAt string   `json:"completed_at,omitempty"`
}

// GetCurrent reads current tasks from Claude Code
func (r *Reader) GetCurrent(ctx context.Context, projectPath string) ([]*Task, error) {
    // Claude Code stores tasks in .claude/tasks/
    tasksDir := filepath.Join(projectPath, ".claude", "tasks")
    
    if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
        return nil, nil // No tasks directory = no tasks
    }

    var tasks []*Task

    files, err := os.ReadDir(tasksDir)
    if err != nil {
        return nil, err
    }

    for _, f := range files {
        if filepath.Ext(f.Name()) != ".json" {
            continue
        }

        content, err := os.ReadFile(filepath.Join(tasksDir, f.Name()))
        if err != nil {
            continue
        }

        var task Task
        if err := json.Unmarshal(content, &task); err != nil {
            continue
        }

        tasks = append(tasks, &task)
    }

    return tasks, nil
}

// GetInProgress returns tasks currently being worked on
func (r *Reader) GetInProgress(ctx context.Context, projectPath string) ([]*Task, error) {
    all, err := r.GetCurrent(ctx, projectPath)
    if err != nil {
        return nil, err
    }

    var inProgress []*Task
    for _, t := range all {
        if t.Status == "in_progress" {
            inProgress = append(inProgress, t)
        }
    }
    return inProgress, nil
}
```

---

## 8. Implementation

### 8.1 Package Structure

```
edi/
├── cmd/
│   └── edi/
│       └── main.go              # CLI entry point
├── internal/
│   ├── session/
│   │   ├── manager.go           # Session lifecycle
│   │   └── state.go             # Session state
│   ├── history/
│   │   ├── store.go             # History storage
│   │   ├── entry.go             # Entry types
│   │   └── render.go            # Markdown rendering
│   ├── briefing/
│   │   ├── generator.go         # Briefing generation
│   │   └── render.go            # Markdown rendering
│   ├── capture/
│   │   ├── detector.go          # Candidate detection
│   │   ├── prompt.go            # Interactive prompt
│   │   ├── saver.go             # RECALL integration
│   │   └── budget.go            # Friction budget
│   └── tasks/
│       └── reader.go            # Claude Code Tasks reader
└── pkg/
    └── api/
        └── types.go             # Shared types
```

### 8.2 Implementation Plan

#### Phase 2.1: History System (Week 1)

- [ ] History entry schema and types
- [ ] Markdown parser (YAML frontmatter + body)
- [ ] History store (save, list, recent, cleanup)
- [ ] Summary generation prompt
- [ ] Unit tests

**Exit Criteria**: Can save and retrieve history entries.

#### Phase 2.2: Briefing System (Week 2)

- [ ] Briefing generator
- [ ] Profile loading and summarization
- [ ] History summarization for briefing
- [ ] RECALL integration for context query
- [ ] Tasks reader (Claude Code integration)
- [ ] Briefing render to markdown
- [ ] Unit tests

**Exit Criteria**: Can generate briefing from history + profile + RECALL.

#### Phase 2.3: Capture System (Week 3)

- [ ] Capture candidate types
- [ ] Candidate detection from session activity
- [ ] Interactive prompt UI
- [ ] Friction budget tracking
- [ ] RECALL saver integration
- [ ] Silent capture for ADRs and self-corrections
- [ ] Unit tests

**Exit Criteria**: Can detect, prompt, and save captures.

#### Phase 2.4: Integration (Week 4)

- [ ] Session manager (start/end orchestration)
- [ ] CLI commands (/end integration)
- [ ] Configuration integration
- [ ] End-to-end testing

**Exit Criteria**: Full session lifecycle works end-to-end.

### 8.3 Validation Criteria

| Metric | Target | Measurement |
|--------|--------|-------------|
| Briefing generation time | < 2s | Without RECALL query |
| Briefing generation time | < 5s | With RECALL query |
| History save time | < 100ms | Single entry |
| Capture detection time | < 1s | Per session |
| History retention | Configurable | 100 entries, 365 days default |

---

## 9. Tasks Integration

### 9.1 Overview

Claude Code's native Tasks feature (released January 2026) provides persistent, dependency-aware task management. EDI integrates deeply with Tasks to leverage their full power.

**Native Tasks Capabilities:**

| Feature | Description |
|---------|-------------|
| **Persistence** | Tasks stored in `~/.claude/tasks` survive session close |
| **Dependencies** | Tasks can depend on other tasks |
| **Cross-session** | Multiple sessions share task list via `CLAUDECODETASKLISTID` |
| **Subagent coordination** | Subagents can update shared task list |

**EDI Integration Principles:**

| Principle | Implementation |
|-----------|----------------|
| **Annotate once, use many** | RECALL context captured at task creation, not re-queried every session |
| **Dependency context flows** | Decisions from parent tasks propagate to dependent tasks |
| **Lazy loading** | RECALL queries only when a task is picked up for execution |
| **Per-task capture** | Capture prompts at task completion, not just session end |
| **Parallel awareness** | Concurrent subagents share discoveries via flight recorder |

### 9.2 Task Lifecycle with EDI

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        TASK LIFECYCLE WITH EDI                               │
│                                                                              │
│  ┌──────────────┐                                                            │
│  │ TASK CREATED │                                                            │
│  └──────┬───────┘                                                            │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────────────────────────────┐                                    │
│  │ Query RECALL for task description    │                                    │
│  │ Store annotations WITH the task      │  ◄── Happens ONCE                  │
│  │ (patterns, failures, decisions)      │                                    │
│  └──────┬───────────────────────────────┘                                    │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐                                                            │
│  │ TASK PENDING │  Task waits for dependencies                               │
│  └──────┬───────┘                                                            │
│         │                                                                    │
│         ▼  Dependencies complete                                             │
│  ┌──────────────────────────────────────┐                                    │
│  │ Inherit context from parent tasks    │  ◄── Decisions flow DOWN           │
│  │ Load stored RECALL annotations       │      the dependency graph          │
│  └──────┬───────────────────────────────┘                                    │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐                                                            │
│  │ TASK ACTIVE  │  Subagent executes with full context                       │
│  │              │  Logs decisions to flight recorder                         │
│  │              │  Posts discoveries for parallel tasks                      │
│  └──────┬───────┘                                                            │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────────────────────────────┐                                    │
│  │ TASK COMPLETE                        │                                    │
│  │ • Prompt: Capture to RECALL?         │  ◄── Per-task capture              │
│  │ • Store completion context           │                                    │
│  │ • Propagate decisions to dependents  │                                    │
│  └──────────────────────────────────────┘                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Task Annotations (Store Once, Use Many)

When a task is created, EDI queries RECALL and stores the results WITH the task:

```go
// Task annotation structure stored in .edi/tasks/{task-id}.yaml
type TaskAnnotation struct {
    TaskID      string    `yaml:"task_id"`
    Description string    `yaml:"description"`
    CreatedAt   time.Time `yaml:"created_at"`
    
    // RECALL context captured at creation
    RecallContext struct {
        Patterns  []string `yaml:"patterns"`   // e.g., ["P-008", "P-012"]
        Failures  []string `yaml:"failures"`   // e.g., ["F-023"]
        Decisions []string `yaml:"decisions"`  // e.g., ["ADR-031"]
        Query     string   `yaml:"query"`      // Original query used
    } `yaml:"recall_context"`
    
    // Inherited from completed parent tasks
    InheritedContext []InheritedDecision `yaml:"inherited_context"`
    
    // Populated during/after execution
    ExecutionContext struct {
        DecisionsMade []Decision `yaml:"decisions_made"`
        Discoveries   []string   `yaml:"discoveries"`
        CapturedTo    []string   `yaml:"captured_to"`  // RECALL IDs if captured
    } `yaml:"execution_context"`
}

type InheritedDecision struct {
    FromTaskID  string `yaml:"from_task_id"`
    Decision    string `yaml:"decision"`
    Rationale   string `yaml:"rationale"`
}
```

**Example annotation file:**

```yaml
# .edi/tasks/task-004.yaml
task_id: task-004
description: "Implement payment retry logic"
created_at: 2026-01-25T10:30:00Z

recall_context:
  patterns:
    - P-008  # Exponential backoff with jitter
    - P-041  # Circuit breaker pattern
  failures:
    - F-023  # Memory leak with unbounded retry queue
    - F-067  # Race condition in retry counter
  decisions:
    - ADR-031  # Payment service architecture
  query: "payment retry implementation pattern"

inherited_context:
  - from_task_id: task-002
    decision: "Use Stripe as payment provider"
    rationale: "Selected for better webhook reliability per ADR-031"
  - from_task_id: task-003
    decision: "Idempotency keys use UUIDv7"
    rationale: "Sortable, unique, no coordination needed"

execution_context:
  decisions_made: []  # Populated during execution
  discoveries: []
  captured_to: []
```

### 9.4 Task Creation Flow

When Claude creates tasks, EDI annotates each one:

```go
func onTaskCreated(task *ClaudeTask) error {
    // 1. Query RECALL for this task's context
    recallResults := recallSearch(RecallQuery{
        Query: task.Description,
        Types: []string{"pattern", "failure", "decision"},
        Limit: 5,
    })
    
    // 2. Create annotation file
    annotation := TaskAnnotation{
        TaskID:      task.ID,
        Description: task.Description,
        CreatedAt:   time.Now(),
        RecallContext: RecallContext{
            Patterns:  extractIDs(recallResults, "pattern"),
            Failures:  extractIDs(recallResults, "failure"),
            Decisions: extractIDs(recallResults, "decision"),
            Query:     task.Description,
        },
    }
    
    // 3. Store annotation
    path := fmt.Sprintf(".edi/tasks/%s.yaml", task.ID)
    return writeYAML(path, annotation)
}
```

**Flight recorder entry:**

```go
flightRecorderLog(Event{
    Type: "milestone",
    Content: fmt.Sprintf("Task created: %s", task.Description),
    Metadata: map[string]interface{}{
        "task_id":      task.ID,
        "recall_items": len(recallResults),
        "dependencies": task.DependsOn,
    },
})
```

### 9.5 Dependency Context Flow

When a task's dependencies complete, their decisions flow to the dependent task:

```go
func onTaskCompleted(task *ClaudeTask) error {
    annotation := loadAnnotation(task.ID)
    
    // 1. Gather decisions made during this task
    decisions := annotation.ExecutionContext.DecisionsMade
    
    // 2. Find dependent tasks
    dependentTasks := findTasksDependingOn(task.ID)
    
    // 3. Propagate decisions to each dependent
    for _, depTask := range dependentTasks {
        depAnnotation := loadAnnotation(depTask.ID)
        
        for _, decision := range decisions {
            if decision.ShouldPropagate {
                depAnnotation.InheritedContext = append(
                    depAnnotation.InheritedContext,
                    InheritedDecision{
                        FromTaskID: task.ID,
                        Decision:   decision.Summary,
                        Rationale:  decision.Rationale,
                    },
                )
            }
        }
        
        saveAnnotation(depAnnotation)
    }
    
    // 4. Prompt for capture (per-task, not just session-end)
    if shouldPromptCapture(annotation) {
        promptTaskCapture(annotation)
    }
    
    return nil
}
```

**Decisions that propagate:**

| Decision Type | Propagates? | Example |
|---------------|-------------|---------|
| Technology choice | ✅ Yes | "Using Stripe for payments" |
| API design | ✅ Yes | "POST /payments returns 202 Accepted" |
| Architecture pattern | ✅ Yes | "Event sourcing for payment state" |
| Implementation detail | ❌ No | "Used mutex instead of channel" |
| Bug fix | ❌ No | "Fixed nil pointer in handler" |

### 9.6 Lazy Loading at Task Pickup

Session start shows status only — RECALL context loads when task is executed:

```go
// Session start: lightweight status only
func loadTaskSummary() *TaskSummary {
    tasks := loadClaudeCodeTasks()
    
    return &TaskSummary{
        Total:      len(tasks),
        Completed:  countByStatus(tasks, "completed"),
        InProgress: countByStatus(tasks, "in_progress"),
        Pending:    countByStatus(tasks, "pending"),
        // NO RECALL queries here!
    }
}

// Task pickup: full context loaded
func loadTaskContext(taskID string) *TaskContext {
    task := loadClaudeTask(taskID)
    annotation := loadAnnotation(taskID)
    
    // Now load the stored RECALL items
    recallItems := make([]RecallItem, 0)
    for _, id := range annotation.RecallContext.Patterns {
        recallItems = append(recallItems, recallGet(id))
    }
    for _, id := range annotation.RecallContext.Failures {
        recallItems = append(recallItems, recallGet(id))
    }
    for _, id := range annotation.RecallContext.Decisions {
        recallItems = append(recallItems, recallGet(id))
    }
    
    return &TaskContext{
        Task:             task,
        RecallItems:      recallItems,
        InheritedContext: annotation.InheritedContext,
    }
}
```

**Token savings:**

| Scenario | Eager Loading | Lazy Loading |
|----------|---------------|--------------|
| 15 pending tasks, work on 2 | 15 RECALL queries | 2 RECALL queries |
| Session start overhead | ~3000 tokens | ~200 tokens |
| Wasted context | High | None |

### 9.7 Per-Task Capture Prompts

Capture happens at task completion, not just session end:

```go
func promptTaskCapture(annotation *TaskAnnotation) {
    decisions := annotation.ExecutionContext.DecisionsMade
    
    // Filter to significant decisions
    significant := filterSignificant(decisions)
    
    if len(significant) == 0 {
        return  // Nothing worth capturing
    }
    
    // Present capture options
    fmt.Println("\n📋 Task completed: " + annotation.Description)
    fmt.Println("\nDecisions made:")
    
    for i, d := range significant {
        fmt.Printf("  [%d] %s\n", i+1, d.Summary)
        fmt.Printf("      Rationale: %s\n", d.Rationale)
    }
    
    fmt.Println("\nCapture to RECALL?")
    fmt.Println("  [A] All as pattern")
    fmt.Println("  [1-N] Select specific")
    fmt.Println("  [S] Skip")
    fmt.Println("  [D] Defer to session end")
}
```

**When to prompt:**

| Condition | Prompt? |
|-----------|---------|
| Task has 2+ significant decisions | ✅ Yes |
| Task encountered and solved a failure | ✅ Yes |
| Task created an ADR | ✅ Yes (auto-capture) |
| Task was simple implementation | ❌ No |
| Friction budget exhausted | ❌ No (defer) |

### 9.8 Parallel Task Coordination

When multiple subagents work parallel tasks, they share discoveries:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     PARALLEL TASK COORDINATION                               │
│                                                                              │
│  Main Agent                                                                  │
│  ├── Task A: "Implement payment retry"     (no deps, can start)              │
│  ├── Task B: "Implement refund service"    (no deps, can start)              │
│  └── Task C: "Integration tests"           (depends on A, B)                 │
│                                                                              │
│  Spawns parallel subagents:                                                  │
│                                                                              │
│  edi-implementer (Task A)              edi-implementer (Task B)              │
│  ├── Load Task A annotations           ├── Load Task B annotations           │
│  ├── Execute implementation            ├── Execute implementation            │
│  │                                     │                                     │
│  │   DISCOVERY: "Stripe API returns    │                                     │
│  │   429 with Retry-After header"      │                                     │
│  │        │                            │                                     │
│  │        ▼                            │                                     │
│  │   Log to flight recorder ───────────┼──► Reads flight recorder            │
│  │   with tag: "parallel-discovery"    │    Sees: "Note from Task A:         │
│  │                                     │    handle Stripe 429 + Retry-After" │
│  │                                     │        │                            │
│  │                                     │        ▼                            │
│  │                                     │    Applies to refund service        │
│  │                                     │                                     │
│  ├── Complete, log decisions           ├── Complete, log decisions           │
│  └── Return summary                    └── Return summary                    │
│                                                                              │
│  Task C now inherits from both A and B                                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Parallel discovery logging:**

```go
// In edi-core skill guidance
flightRecorderLog(Event{
    Type: "observation",
    Content: "Stripe API returns 429 with Retry-After header for rate limits",
    Metadata: map[string]interface{}{
        "task_id":    "task-004",
        "tag":        "parallel-discovery",
        "applies_to": []string{"payment", "refund", "stripe"},
    },
})
```

**Subagent startup includes parallel notes:**

```go
func loadParallelDiscoveries(taskID string, session *Session) []Discovery {
    // Find flight recorder entries from parallel tasks in this session
    entries := flightRecorderQuery(FlightRecorderQuery{
        SessionID: session.ID,
        Tags:      []string{"parallel-discovery"},
        NotTaskID: taskID,  // Exclude own task
    })
    
    return entriesToDiscoveries(entries)
}
```

### 9.9 Briefing: Lightweight Task Status

Session briefing shows status without loading full RECALL context:

```markdown
## Current Work: Tasks

**Status**: 3 completed, 2 in progress, 4 pending

### In Progress
- **task-004**: Implement payment retry logic
  - Blocked by: none
  - RECALL annotations: 2 patterns, 2 failures, 1 decision (load on pickup)
  
- **task-005**: Write payment integration tests
  - Blocked by: task-004 (in progress)
  - RECALL annotations: 1 pattern, 1 failure (load on pickup)

### Ready to Start (no blocking deps)
- **task-006**: Implement refund service
- **task-007**: Add payment webhooks

### Recently Completed
- task-001: Design payment architecture ✓
- task-002: Select payment provider (→ Stripe) ✓
- task-003: Implement idempotency keys ✓

To see full RECALL context for a task, pick it up with `/task task-004`.
```

### 9.10 The `/task` Command

```markdown
---
name: task
aliases: [tasks]
description: Manage task-based workflows with RECALL enrichment
---

# /task [task-id | description]

## No arguments: Show task status
Display current task list with status, dependencies, and annotation summaries.
Do NOT load full RECALL context (lazy loading).

## With task-id: Pick up specific task
1. Load task annotations (stored RECALL context)
2. Load inherited context from completed dependencies
3. Check for parallel discoveries in flight recorder
4. Present full context and begin execution

## With description: Create new task workflow
1. Break description into tasks with dependencies
2. For each task, query RECALL and create annotation
3. Log task creation to flight recorder
4. Show task graph and ask to proceed

## During task execution
- Log significant decisions to flight recorder
- Mark decisions that should propagate to dependents
- On completion, prompt for RECALL capture

## Examples

`/task`
→ Show: "3 done, 2 in progress, 4 pending" with dependency graph

`/task task-004`
→ Load task-004 with full RECALL context, inherited decisions, parallel notes

`/task Implement billing system with Stripe`
→ Break into tasks, annotate each with RECALL, show graph
```

### 9.11 Configuration

```yaml
# .edi/config.yaml

tasks:
  # Store task annotations
  annotations_dir: .edi/tasks
  
  # Lazy loading (recommended)
  eager_recall_loading: false
  
  # Per-task capture prompts
  capture_on_completion: true
  
  # Parallel discovery sharing
  share_parallel_discoveries: true
  
  # Auto-propagate decisions to dependents
  propagate_decisions: true
  
  # Decisions that auto-propagate (others require explicit marking)
  auto_propagate_types:
    - technology_choice
    - api_design
    - architecture_pattern
  
  # Briefing settings
  briefing:
    show_task_status: true
    max_tasks_shown: 10
    show_recall_summary: true  # "2 patterns, 1 failure" not full content
```

### 9.12 Integration Summary

| EDI Component | Tasks Integration |
|---------------|-------------------|
| **Task Creation** | Query RECALL → store annotations with task |
| **Session Start** | Show status only (lazy loading) |
| **Task Pickup** | Load stored annotations + inherited context |
| **During Execution** | Log decisions to flight recorder |
| **Parallel Work** | Share discoveries via flight recorder tags |
| **Task Completion** | Propagate decisions → prompt capture |
| **Briefing** | Lightweight status, annotation summaries |
| **Capture** | Per-task prompts, not just session-end |

### 9.13 Comparison: Before and After

| Aspect | Shallow Integration | Deep Integration |
|--------|---------------------|------------------|
| RECALL queries | Every session, all tasks | Once at creation, load on pickup |
| Token usage | O(tasks × sessions) | O(tasks) |
| Dependency value | Execution order only | Context flows down graph |
| Capture timing | Session end only | Per-task completion |
| Parallel coordination | Independent | Shared discoveries |
| Briefing cost | High (full RECALL) | Low (status + summaries) |

---

## Appendix A: Quick Reference

### Session Commands

| Command | Action |
|---------|--------|
| `edi` | Start session with briefing |
| `/end` | End session, save history, prompt capture |

### File Locations

| What | Where |
|------|-------|
| History entries | `.edi/history/{date}-{session_id}.md` |
| Project profile | `.edi/profile.md` |
| Config | `.edi/config.yaml` |

### Configuration Keys

| Key | Default | Purpose |
|-----|---------|---------|
| `briefing.history_depth` | 3 | History entries in briefing |
| `briefing.recall_auto_query` | true | Auto-query RECALL at start |
| `history.retention.max_entries` | 100 | Max history entries |
| `history.retention.max_age_days` | 365 | Max history age |
| `capture.prompt_on_end` | true | Show capture prompt |

---

## Appendix B: Related Specifications

| Spec | Relationship |
|------|--------------|
| Workspace & Configuration | Directory paths, config schema |
| RECALL MCP Server | Knowledge storage for captures |
| Agent System | Agent loading at session start |
| CLI Architecture | Command implementation |

---

## Appendix C: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 25, 2026 | History ≠ State | Tasks handles state; history captures reasoning |
| Jan 25, 2026 | Markdown with YAML frontmatter | Human-readable, structured metadata |
| Jan 25, 2026 | Friction budget | Prevent prompt fatigue |
| Jan 25, 2026 | Silent capture for ADRs | High confidence, low noise |
| Jan 25, 2026 | Briefing from multiple sources | Profile + History + Tasks + RECALL |
| Jan 25, 2026 | Capture prompt at session end | Natural pause point, expected location |
| Jan 25, 2026 | Task annotations (store once) | Avoid O(tasks × sessions) RECALL queries |
| Jan 25, 2026 | Dependency context flow | Decisions from parent tasks propagate to dependents |
| Jan 25, 2026 | Lazy loading | Only load RECALL when task picked up, not at session start |
| Jan 25, 2026 | Per-task capture prompts | Capture at completion, not just session end |
| Jan 25, 2026 | Parallel discovery sharing | Concurrent subagents share findings via flight recorder |
