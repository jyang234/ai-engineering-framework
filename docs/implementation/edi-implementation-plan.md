# EDI Implementation Plan

> **Implementation Status (January 31, 2026):** Broadly followed. Package layout differs from spec. Missing packages: recorder, capture.

**Version**: 1.0
**Created**: January 25, 2026
**Target**: Claude Code execution
**Estimated Duration**: 6 weeks

---

## Executive Summary

This plan implements EDI (Enhanced Development Intelligence) as a harness for Claude Code. The implementation uses Go, targets macOS and Linux, and follows a phased approach with validation checkpoints.

### Key Constraints

- **Go only** — No Python, Node.js, or external runtimes
- **SQLite FTS for v0** — No vector databases or external APIs
- **Official MCP SDK** — `github.com/modelcontextprotocol/go-sdk/mcp`
- **Single binary** — EDI CLI + RECALL MCP server in one binary
- **Platforms** — darwin/amd64, darwin/arm64, linux/amd64

### Success Criteria

After implementation, a user can:
```bash
$ edi init                    # Initialize EDI in a project
$ edi                         # Launch Claude Code with EDI context
> /task Implement auth        # Create tasks with RECALL context
> [work on tasks]             # Claude has organizational knowledge
> /end                        # Save history, prompt for capture
$ edi                         # Next session has continuity
```

---

## Project Structure

```
edi/
├── cmd/
│   └── edi/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── cli/                        # CLI commands
│   │   ├── root.go                 # Root command, version
│   │   ├── init.go                 # edi init
│   │   ├── launch.go               # edi (default launch)
│   │   └── version.go              # edi version
│   ├── config/                     # Configuration
│   │   ├── schema.go               # Config struct definitions
│   │   ├── loader.go               # Load and merge configs
│   │   └── defaults.go             # Default values
│   ├── briefing/                   # Briefing generation
│   │   ├── generator.go            # Build briefing from sources
│   │   ├── history.go              # Load recent history
│   │   └── tasks.go                # Load task status
│   ├── launch/                     # Claude Code launch
│   │   ├── context.go              # Build session context file
│   │   ├── launcher.go             # exec() Claude Code
│   │   └── commands.go             # Install slash commands
│   ├── recall/                     # RECALL MCP server
│   │   ├── server.go               # MCP server setup
│   │   ├── tools.go                # Tool handlers
│   │   ├── storage.go              # SQLite operations
│   │   ├── schema.sql              # Database schema
│   │   └── fts.go                  # Full-text search
│   └── assets/                     # Embedded files
│       ├── agents/                 # Agent definitions
│       ├── commands/               # Slash command definitions
│       └── skills/                 # Skill definitions
├── pkg/
│   └── types/                      # Shared types
│       ├── config.go
│       ├── history.go
│       └── recall.go
├── testdata/                       # Test fixtures
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Phase 1: Foundation (Week 1)

### Goal
Working CLI that can initialize a project and launch Claude Code with basic context.

### 1.1 Project Setup

**Task 1.1.1: Initialize Go module**
```bash
mkdir -p edi/cmd/edi edi/internal edi/pkg
cd edi
go mod init github.com/[user]/edi
```

**Task 1.1.2: Add dependencies**
```go
// go.mod
require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/modelcontextprotocol/go-sdk v0.1.0
)
```

**Task 1.1.3: Create Makefile**
```makefile
.PHONY: build test install clean

VERSION := 0.1.0
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/edi ./cmd/edi

test:
	go test -v ./...

install: build
	cp bin/edi ~/.local/bin/

clean:
	rm -rf bin/

# Cross-compilation
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/edi-darwin-amd64 ./cmd/edi
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/edi-darwin-arm64 ./cmd/edi
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/edi-linux-amd64 ./cmd/edi
```

### 1.2 CLI Skeleton

**Task 1.2.1: Create root command**

File: `cmd/edi/main.go`
```go
package main

import (
    "os"
    "github.com/[user]/edi/internal/cli"
)

var version = "dev"

func main() {
    if err := cli.Execute(version); err != nil {
        os.Exit(1)
    }
}
```

File: `internal/cli/root.go`
```go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "edi",
    Short: "EDI - Enhanced Development Intelligence",
    Long:  `EDI is a harness for Claude Code that provides continuity, knowledge, and specialized behaviors.`,
    RunE:  runLaunch, // Default action is launch
}

func Execute(version string) error {
    rootCmd.Version = version
    return rootCmd.Execute()
}

func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(versionCmd)
}
```

**Task 1.2.2: Create init command**

File: `internal/cli/init.go`
```go
package cli

import (
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/spf13/cobra"
    "github.com/[user]/edi/internal/config"
)

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize EDI in current directory or globally",
    RunE:  runInit,
}

func init() {
    initCmd.Flags().Bool("global", false, "Initialize global EDI installation")
}

func runInit(cmd *cobra.Command, args []string) error {
    global, _ := cmd.Flags().GetBool("global")
    
    if global {
        return initGlobal()
    }
    return initProject()
}

func initGlobal() error {
    home, err := os.UserHomeDir()
    if err != nil {
        return err
    }
    
    ediHome := filepath.Join(home, ".edi")
    
    // Create directory structure
    dirs := []string{
        ediHome,
        filepath.Join(ediHome, "agents"),
        filepath.Join(ediHome, "commands"),
        filepath.Join(ediHome, "skills"),
        filepath.Join(ediHome, "recall"),
        filepath.Join(ediHome, "cache"),
    }
    
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", dir, err)
        }
    }
    
    // Install default agents
    if err := installDefaultAgents(filepath.Join(ediHome, "agents")); err != nil {
        return err
    }
    
    // Install slash commands
    if err := installSlashCommands(filepath.Join(ediHome, "commands")); err != nil {
        return err
    }
    
    // Install edi-core skill to Claude's skills directory
    claudeSkillsDir := filepath.Join(home, ".claude", "skills", "edi-core")
    if err := installEdiCoreSkill(claudeSkillsDir); err != nil {
        return err
    }
    
    // Create default config
    configPath := filepath.Join(ediHome, "config.yaml")
    if err := config.WriteDefault(configPath); err != nil {
        return err
    }
    
    fmt.Println("✓ EDI initialized globally at ~/.edi")
    return nil
}

func initProject() error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }
    
    ediDir := filepath.Join(cwd, ".edi")
    
    // Create directory structure
    dirs := []string{
        ediDir,
        filepath.Join(ediDir, "history"),
        filepath.Join(ediDir, "tasks"),
        filepath.Join(ediDir, "recall"),
    }
    
    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", dir, err)
        }
    }
    
    // Create project config
    configPath := filepath.Join(ediDir, "config.yaml")
    if err := config.WriteProjectDefault(configPath); err != nil {
        return err
    }
    
    // Create profile template
    profilePath := filepath.Join(ediDir, "profile.md")
    if err := writeProfileTemplate(profilePath); err != nil {
        return err
    }
    
    fmt.Println("✓ EDI initialized in current project")
    fmt.Println("  Edit .edi/profile.md to describe your project")
    return nil
}
```

### 1.3 Configuration

**Task 1.3.1: Define config schema**

File: `internal/config/schema.go`
```go
package config

type Config struct {
    Version string `yaml:"version"`
    
    Agent   string       `yaml:"agent"`
    Recall  RecallConfig `yaml:"recall"`
    Briefing BriefingConfig `yaml:"briefing"`
    Capture CaptureConfig `yaml:"capture"`
    Tasks   TasksConfig  `yaml:"tasks"`
}

type RecallConfig struct {
    Enabled bool `yaml:"enabled"`
}

type BriefingConfig struct {
    IncludeHistory bool `yaml:"include_history"`
    HistoryEntries int  `yaml:"history_entries"`
    IncludeTasks   bool `yaml:"include_tasks"`
    IncludeProfile bool `yaml:"include_profile"`
}

type CaptureConfig struct {
    FrictionBudget int `yaml:"friction_budget"`
}

type TasksConfig struct {
    LazyLoading       bool `yaml:"lazy_loading"`
    CaptureOnComplete bool `yaml:"capture_on_completion"`
    PropagateDecisions bool `yaml:"propagate_decisions"`
}
```

**Task 1.3.2: Implement config loader**

File: `internal/config/loader.go`
```go
package config

import (
    "os"
    "path/filepath"
    
    "github.com/spf13/viper"
)

func Load() (*Config, error) {
    cfg := DefaultConfig()
    
    home, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()
    
    // Load global config
    globalPath := filepath.Join(home, ".edi", "config.yaml")
    if err := loadFile(globalPath, cfg); err != nil && !os.IsNotExist(err) {
        return nil, err
    }
    
    // Load project config (overrides global)
    projectPath := filepath.Join(cwd, ".edi", "config.yaml")
    if err := loadFile(projectPath, cfg); err != nil && !os.IsNotExist(err) {
        return nil, err
    }
    
    return cfg, nil
}

func loadFile(path string, cfg *Config) error {
    v := viper.New()
    v.SetConfigFile(path)
    
    if err := v.ReadInConfig(); err != nil {
        return err
    }
    
    return v.Unmarshal(cfg)
}
```

**Task 1.3.3: Define defaults**

File: `internal/config/defaults.go`
```go
package config

func DefaultConfig() *Config {
    return &Config{
        Version: "1",
        Agent:   "coder",
        Recall: RecallConfig{
            Enabled: true,
        },
        Briefing: BriefingConfig{
            IncludeHistory: true,
            HistoryEntries: 3,
            IncludeTasks:   true,
            IncludeProfile: true,
        },
        Capture: CaptureConfig{
            FrictionBudget: 3,
        },
        Tasks: TasksConfig{
            LazyLoading:       true,
            CaptureOnComplete: true,
            PropagateDecisions: true,
        },
    }
}
```

### 1.4 Launch Command

**Task 1.4.1: Implement launcher**

File: `internal/launch/launcher.go`
```go
package launch

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "syscall"
)

func Launch(contextPath string) error {
    // Find Claude Code binary
    claudePath, err := exec.LookPath("claude")
    if err != nil {
        return fmt.Errorf("Claude Code not found in PATH. Install from: https://claude.ai/code")
    }
    
    // Build arguments
    args := []string{
        "claude",
        "--append-system-prompt-file", contextPath,
    }
    
    // Get current environment
    env := os.Environ()
    
    // Replace current process with Claude Code
    return syscall.Exec(claudePath, args, env)
}
```

**Task 1.4.2: Build session context**

File: `internal/launch/context.go`
```go
package launch

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/[user]/edi/internal/config"
    "github.com/[user]/edi/internal/briefing"
)

func BuildContext(cfg *config.Config) (string, error) {
    var sb strings.Builder
    
    // EDI identity
    sb.WriteString("# EDI - Enhanced Development Intelligence\n\n")
    sb.WriteString("You are operating as EDI, an AI engineering assistant with ")
    sb.WriteString("continuity, knowledge, and specialized behaviors.\n\n")
    
    // Load agent
    agent, err := loadAgent(cfg.Agent)
    if err != nil {
        return "", err
    }
    sb.WriteString(fmt.Sprintf("## Current Mode: %s\n\n", agent.Name))
    sb.WriteString(agent.SystemPrompt)
    sb.WriteString("\n\n")
    
    // Generate briefing
    brief, err := briefing.Generate(cfg)
    if err != nil {
        return "", err
    }
    sb.WriteString("## Session Briefing\n\n")
    sb.WriteString(brief)
    sb.WriteString("\n\n")
    
    // Write to cache file
    home, _ := os.UserHomeDir()
    cacheDir := filepath.Join(home, ".edi", "cache")
    os.MkdirAll(cacheDir, 0755)
    
    filename := fmt.Sprintf("session-%d.md", time.Now().Unix())
    contextPath := filepath.Join(cacheDir, filename)
    
    if err := os.WriteFile(contextPath, []byte(sb.String()), 0644); err != nil {
        return "", err
    }
    
    return contextPath, nil
}
```

**Task 1.4.3: Install slash commands**

File: `internal/launch/commands.go`
```go
package launch

import (
    "crypto/sha256"
    "encoding/hex"
    "io"
    "os"
    "path/filepath"
)

func InstallCommands() error {
    home, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()
    
    srcDir := filepath.Join(home, ".edi", "commands")
    dstDir := filepath.Join(cwd, ".claude", "commands")
    
    // Ensure destination exists
    if err := os.MkdirAll(dstDir, 0755); err != nil {
        return err
    }
    
    // Copy each command if missing or changed
    entries, err := os.ReadDir(srcDir)
    if err != nil {
        return err
    }
    
    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
            continue
        }
        
        srcPath := filepath.Join(srcDir, entry.Name())
        dstPath := filepath.Join(dstDir, entry.Name())
        
        if needsCopy(srcPath, dstPath) {
            if err := copyFile(srcPath, dstPath); err != nil {
                return err
            }
        }
    }
    
    return nil
}

func needsCopy(src, dst string) bool {
    dstInfo, err := os.Stat(dst)
    if os.IsNotExist(err) {
        return true
    }
    if err != nil {
        return true
    }
    
    srcHash, _ := fileHash(src)
    dstHash, _ := fileHash(dst)
    
    return srcHash != dstHash || dstInfo.Size() == 0
}

func fileHash(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### 1.5 Embedded Assets

**Task 1.5.1: Embed agent definitions**

File: `internal/assets/embed.go`
```go
package assets

import "embed"

//go:embed agents/*.md
var Agents embed.FS

//go:embed commands/*.md
var Commands embed.FS

//go:embed skills/edi-core/SKILL.md
var EdiCoreSkill embed.FS
```

**Task 1.5.2: Create coder agent**

File: `internal/assets/agents/coder.md`
```markdown
---
name: coder
description: Default coding mode for implementation work
tools: Read, Write, Edit, Bash, Grep, Glob, recall_search, recall_add, recall_feedback, flight_recorder_log
---

# Coder Agent

You are EDI operating in **Coder** mode, focused on implementation.

## Behavior

- Write clean, tested, documented code
- Query RECALL before implementing patterns you've seen before
- Log significant decisions to the flight recorder
- Follow project conventions from the profile

## RECALL Integration

Before implementing:
```
recall_search({query: "[what you're implementing]", types: ["pattern", "failure"]})
```

After significant decisions:
```
flight_recorder_log({
  type: "decision",
  content: "[what you decided]",
  rationale: "[why]"
})
```
```

**Task 1.5.3: Create /end slash command**

File: `internal/assets/commands/end.md`
```markdown
---
name: end
description: End the current EDI session
---

# End Session

Generate a session summary and save it to history.

## Steps

1. **Summarize** what was accomplished this session
2. **List** key decisions made
3. **Identify** capture candidates (patterns, failures, decisions worth saving)
4. **Ask** user which items to capture to RECALL
5. **Save** approved items using `recall_add`
6. **Write** session history to `.edi/history/{date}-{session-id}.md`

## Summary Format

```markdown
---
session_id: [generate UUID]
started_at: [session start time]
ended_at: [now]
agent: [current agent]
tasks_completed: [list]
decisions_captured: [list of RECALL IDs]
---

# Session Summary

## Accomplished
- [bullet points]

## Key Decisions
- [decisions with rationale]

## Open Items
- [what's left to do]
```
```

### 1.6 Validation Checkpoint

**Acceptance Criteria for Phase 1:**

- [ ] `edi init --global` creates `~/.edi/` with agents, commands, skills
- [ ] `edi init` creates `.edi/` in current directory
- [ ] `edi version` shows version number
- [ ] `edi` launches Claude Code with appended system prompt
- [ ] Claude Code sees EDI persona and agent instructions
- [ ] Slash commands are installed to `.claude/commands/`

**Test Script:**
```bash
#!/bin/bash
set -e

# Build
make build

# Test global init
rm -rf ~/.edi
./bin/edi init --global
test -d ~/.edi/agents
test -d ~/.edi/commands
test -f ~/.edi/config.yaml

# Test project init
cd /tmp && mkdir test-project && cd test-project
~/path/to/edi init
test -d .edi
test -f .edi/config.yaml
test -f .edi/profile.md

# Test launch (manual verification)
echo "Manual test: Run 'edi' and verify Claude Code starts with EDI context"

echo "✓ Phase 1 validation passed"
```

---

## Phase 2: RECALL MCP Server (Week 2)

### Goal
Working MCP server with SQLite FTS search that Claude Code can query.

### 2.1 SQLite Schema

**Task 2.1.1: Define database schema**

File: `internal/recall/schema.sql`
```sql
-- Knowledge items table
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- pattern, failure, decision, context
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,           -- JSON array
    scope TEXT NOT NULL, -- global, project
    project_path TEXT,   -- NULL for global
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    usefulness_score REAL DEFAULT 0.0,
    use_count INTEGER DEFAULT 0
);

-- Full-text search index
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
    title,
    content,
    tags,
    content=items,
    content_rowid=rowid
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, title, content, tags)
    VALUES (NEW.rowid, NEW.title, NEW.content, NEW.tags);
END;

CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, content, tags)
    VALUES('delete', OLD.rowid, OLD.title, OLD.content, OLD.tags);
END;

CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, content, tags)
    VALUES('delete', OLD.rowid, OLD.title, OLD.content, OLD.tags);
    INSERT INTO items_fts(rowid, title, content, tags)
    VALUES (NEW.rowid, NEW.title, NEW.content, NEW.tags);
END;

-- Feedback table
CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    useful BOOLEAN NOT NULL,
    context TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (item_id) REFERENCES items(id)
);

-- Flight recorder table
CREATE TABLE IF NOT EXISTS flight_recorder (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    type TEXT NOT NULL,  -- decision, error, milestone, observation, task_annotation, task_complete
    content TEXT NOT NULL,
    rationale TEXT,
    metadata TEXT        -- JSON
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_items_type ON items(type);
CREATE INDEX IF NOT EXISTS idx_items_scope ON items(scope);
CREATE INDEX IF NOT EXISTS idx_flight_session ON flight_recorder(session_id);
CREATE INDEX IF NOT EXISTS idx_flight_type ON flight_recorder(type);
```

**Task 2.1.2: Implement storage layer**

File: `internal/recall/storage.go`
```go
package recall

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
)

type Storage struct {
    db *sql.DB
}

type Item struct {
    ID              string    `json:"id"`
    Type            string    `json:"type"`
    Title           string    `json:"title"`
    Content         string    `json:"content"`
    Tags            []string  `json:"tags"`
    Scope           string    `json:"scope"`
    ProjectPath     string    `json:"project_path,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    UsefulnessScore float64   `json:"usefulness_score"`
    UseCount        int       `json:"use_count"`
}

type FlightRecorderEntry struct {
    ID        int64                  `json:"id"`
    SessionID string                 `json:"session_id"`
    Timestamp time.Time              `json:"timestamp"`
    Type      string                 `json:"type"`
    Content   string                 `json:"content"`
    Rationale string                 `json:"rationale,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func NewStorage(dbPath string) (*Storage, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    
    // Initialize schema
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &Storage{db: db}, nil
}

func (s *Storage) Search(query string, types []string, scope string, limit int) ([]Item, error) {
    // Build FTS query
    ftsQuery := fmt.Sprintf(`
        SELECT i.id, i.type, i.title, i.content, i.tags, i.scope,
               i.project_path, i.created_at, i.updated_at,
               i.usefulness_score, i.use_count
        FROM items i
        JOIN items_fts fts ON i.rowid = fts.rowid
        WHERE items_fts MATCH ?
    `)
    
    args := []interface{}{query}
    
    if len(types) > 0 {
        placeholders := make([]string, len(types))
        for i, t := range types {
            placeholders[i] = "?"
            args = append(args, t)
        }
        ftsQuery += fmt.Sprintf(" AND i.type IN (%s)", strings.Join(placeholders, ","))
    }
    
    if scope != "" {
        ftsQuery += " AND i.scope = ?"
        args = append(args, scope)
    }
    
    ftsQuery += " ORDER BY rank LIMIT ?"
    args = append(args, limit)
    
    rows, err := s.db.Query(ftsQuery, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var items []Item
    for rows.Next() {
        var item Item
        var tagsJSON string
        
        err := rows.Scan(
            &item.ID, &item.Type, &item.Title, &item.Content,
            &tagsJSON, &item.Scope, &item.ProjectPath,
            &item.CreatedAt, &item.UpdatedAt,
            &item.UsefulnessScore, &item.UseCount,
        )
        if err != nil {
            return nil, err
        }
        
        json.Unmarshal([]byte(tagsJSON), &item.Tags)
        items = append(items, item)
    }
    
    return items, nil
}

func (s *Storage) Add(item *Item) error {
    tagsJSON, _ := json.Marshal(item.Tags)
    
    _, err := s.db.Exec(`
        INSERT INTO items (id, type, title, content, tags, scope, project_path, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
        item.ID, item.Type, item.Title, item.Content,
        string(tagsJSON), item.Scope, item.ProjectPath,
        item.CreatedAt.Format(time.RFC3339),
        item.UpdatedAt.Format(time.RFC3339),
    )
    
    return err
}

func (s *Storage) Get(id string) (*Item, error) {
    row := s.db.QueryRow(`
        SELECT id, type, title, content, tags, scope, project_path,
               created_at, updated_at, usefulness_score, use_count
        FROM items WHERE id = ?
    `, id)
    
    var item Item
    var tagsJSON string
    
    err := row.Scan(
        &item.ID, &item.Type, &item.Title, &item.Content,
        &tagsJSON, &item.Scope, &item.ProjectPath,
        &item.CreatedAt, &item.UpdatedAt,
        &item.UsefulnessScore, &item.UseCount,
    )
    if err != nil {
        return nil, err
    }
    
    json.Unmarshal([]byte(tagsJSON), &item.Tags)
    return &item, nil
}

func (s *Storage) RecordFeedback(itemID, sessionID string, useful bool, context string) error {
    _, err := s.db.Exec(`
        INSERT INTO feedback (item_id, session_id, useful, context, created_at)
        VALUES (?, ?, ?, ?, ?)
    `, itemID, sessionID, useful, context, time.Now().Format(time.RFC3339))
    
    if err != nil {
        return err
    }
    
    // Update usefulness score
    if useful {
        _, err = s.db.Exec(`
            UPDATE items 
            SET usefulness_score = usefulness_score + 1.0,
                use_count = use_count + 1
            WHERE id = ?
        `, itemID)
    }
    
    return err
}

func (s *Storage) LogFlightRecorder(entry *FlightRecorderEntry) error {
    metadataJSON, _ := json.Marshal(entry.Metadata)
    
    _, err := s.db.Exec(`
        INSERT INTO flight_recorder (session_id, timestamp, type, content, rationale, metadata)
        VALUES (?, ?, ?, ?, ?, ?)
    `,
        entry.SessionID,
        entry.Timestamp.Format(time.RFC3339),
        entry.Type,
        entry.Content,
        entry.Rationale,
        string(metadataJSON),
    )
    
    return err
}
```

### 2.2 MCP Server

**Task 2.2.1: Implement MCP server**

File: `internal/recall/server.go`
```go
package recall

import (
    "context"
    "log"
    "os"
    
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
    storage *Storage
    sessionID string
}

func NewServer(storage *Storage, sessionID string) *Server {
    return &Server{
        storage:   storage,
        sessionID: sessionID,
    }
}

func (s *Server) Run(ctx context.Context) error {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "recall",
        Version: "1.0.0",
    }, nil)
    
    // Register tools
    s.registerTools(server)
    
    // Run on stdio
    transport := mcp.NewStdioTransport()
    if err := server.Run(ctx, transport); err != nil {
        return err
    }
    
    return nil
}

func (s *Server) registerTools(server *mcp.Server) {
    // recall_search
    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_search",
        Description: "Search organizational knowledge for patterns, failures, and decisions",
    }, s.handleSearch)
    
    // recall_get
    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_get",
        Description: "Get a specific knowledge item by ID",
    }, s.handleGet)
    
    // recall_add
    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_add",
        Description: "Add new knowledge to RECALL",
    }, s.handleAdd)
    
    // recall_feedback
    mcp.AddTool(server, &mcp.Tool{
        Name:        "recall_feedback",
        Description: "Provide feedback on whether a RECALL item was useful",
    }, s.handleFeedback)
    
    // flight_recorder_log
    mcp.AddTool(server, &mcp.Tool{
        Name:        "flight_recorder_log",
        Description: "Log decisions, errors, and milestones during work",
    }, s.handleFlightRecorderLog)
}
```

**Task 2.2.2: Implement tool handlers**

File: `internal/recall/tools.go`
```go
package recall

import (
    "context"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchInput struct {
    Query string   `json:"query" jsonschema:"description=Search query for knowledge items"`
    Types []string `json:"types,omitempty" jsonschema:"description=Filter by type: pattern, failure, decision, context"`
    Scope string   `json:"scope,omitempty" jsonschema:"description=Filter by scope: global, project"`
    Limit int      `json:"limit,omitempty" jsonschema:"description=Maximum results (default 10)"`
}

type SearchOutput struct {
    Results []Item `json:"results"`
    Count   int    `json:"count"`
}

func (s *Server) handleSearch(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
    if input.Limit == 0 {
        input.Limit = 10
    }
    
    items, err := s.storage.Search(input.Query, input.Types, input.Scope, input.Limit)
    if err != nil {
        return nil, SearchOutput{}, err
    }
    
    output := SearchOutput{
        Results: items,
        Count:   len(items),
    }
    
    return nil, output, nil
}

type GetInput struct {
    ID string `json:"id" jsonschema:"description=ID of the knowledge item to retrieve"`
}

func (s *Server) handleGet(ctx context.Context, req *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, *Item, error) {
    item, err := s.storage.Get(input.ID)
    if err != nil {
        return nil, nil, err
    }
    
    return nil, item, nil
}

type AddInput struct {
    Type    string   `json:"type" jsonschema:"description=Type: pattern, failure, decision"`
    Title   string   `json:"title" jsonschema:"description=Brief title"`
    Content string   `json:"content" jsonschema:"description=Full content/description"`
    Tags    []string `json:"tags,omitempty" jsonschema:"description=Tags for categorization"`
    Scope   string   `json:"scope,omitempty" jsonschema:"description=Scope: global or project (default: project)"`
}

type AddOutput struct {
    ID      string `json:"id"`
    Message string `json:"message"`
}

func (s *Server) handleAdd(ctx context.Context, req *mcp.CallToolRequest, input AddInput) (*mcp.CallToolResult, AddOutput, error) {
    if input.Scope == "" {
        input.Scope = "project"
    }
    
    id := generateID(input.Type)
    now := time.Now()
    
    item := &Item{
        ID:        id,
        Type:      input.Type,
        Title:     input.Title,
        Content:   input.Content,
        Tags:      input.Tags,
        Scope:     input.Scope,
        CreatedAt: now,
        UpdatedAt: now,
    }
    
    if err := s.storage.Add(item); err != nil {
        return nil, AddOutput{}, err
    }
    
    output := AddOutput{
        ID:      id,
        Message: fmt.Sprintf("Added %s: %s", input.Type, input.Title),
    }
    
    return nil, output, nil
}

type FeedbackInput struct {
    ItemID  string `json:"item_id" jsonschema:"description=ID of the item to provide feedback on"`
    Useful  bool   `json:"useful" jsonschema:"description=Whether the item was useful"`
    Context string `json:"context,omitempty" jsonschema:"description=Context about how it was used"`
}

func (s *Server) handleFeedback(ctx context.Context, req *mcp.CallToolRequest, input FeedbackInput) (*mcp.CallToolResult, any, error) {
    err := s.storage.RecordFeedback(input.ItemID, s.sessionID, input.Useful, input.Context)
    if err != nil {
        return nil, nil, err
    }
    
    return nil, map[string]string{"status": "recorded"}, nil
}

type FlightRecorderInput struct {
    Type      string                 `json:"type" jsonschema:"description=Type: decision, error, milestone, observation, task_annotation, task_complete"`
    Content   string                 `json:"content" jsonschema:"description=What happened"`
    Rationale string                 `json:"rationale,omitempty" jsonschema:"description=Why (for decisions)"`
    Metadata  map[string]interface{} `json:"metadata,omitempty" jsonschema:"description=Additional structured data"`
}

func (s *Server) handleFlightRecorderLog(ctx context.Context, req *mcp.CallToolRequest, input FlightRecorderInput) (*mcp.CallToolResult, any, error) {
    entry := &FlightRecorderEntry{
        SessionID: s.sessionID,
        Timestamp: time.Now(),
        Type:      input.Type,
        Content:   input.Content,
        Rationale: input.Rationale,
        Metadata:  input.Metadata,
    }
    
    if err := s.storage.LogFlightRecorder(entry); err != nil {
        return nil, nil, err
    }
    
    return nil, map[string]string{"status": "logged"}, nil
}

func generateID(itemType string) string {
    prefix := map[string]string{
        "pattern":  "P",
        "failure":  "F",
        "decision": "D",
        "context":  "C",
    }[itemType]
    
    if prefix == "" {
        prefix = "X"
    }
    
    return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}
```

### 2.3 MCP Integration with EDI Launch

**Task 2.3.1: Start RECALL server before launch**

File: `internal/cli/launch.go` (update)
```go
package cli

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    
    "github.com/google/uuid"
    "github.com/spf13/cobra"
    "github.com/[user]/edi/internal/config"
    "github.com/[user]/edi/internal/launch"
    "github.com/[user]/edi/internal/recall"
)

func runLaunch(cmd *cobra.Command, args []string) error {
    // Load config
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    
    // Generate session ID
    sessionID := uuid.New().String()
    
    // Start RECALL MCP server in background
    if cfg.Recall.Enabled {
        if err := startRecallServer(sessionID); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: RECALL unavailable (%v), starting without knowledge retrieval\n", err)
        }
    }
    
    // Install slash commands
    if err := launch.InstallCommands(); err != nil {
        return err
    }
    
    // Build session context
    contextPath, err := launch.BuildContext(cfg, sessionID)
    if err != nil {
        return err
    }
    
    // Launch Claude Code (replaces current process)
    return launch.Launch(contextPath)
}

func startRecallServer(sessionID string) error {
    home, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()
    
    // Determine database paths
    globalDB := filepath.Join(home, ".edi", "recall", "global.db")
    projectDB := filepath.Join(cwd, ".edi", "recall", "project.db")
    
    // Start as background process that Claude Code connects to via MCP
    // The server binary path would be the same as edi but with different args
    ediBinary, _ := os.Executable()
    
    cmd := exec.Command(ediBinary, "recall-server",
        "--global-db", globalDB,
        "--project-db", projectDB,
        "--session-id", sessionID,
    )
    
    // Configure as MCP server (stdio)
    // This will be invoked by Claude Code's MCP configuration
    
    return nil // Server started via MCP config, not directly
}
```

**Task 2.3.2: Add recall-server subcommand**

File: `internal/cli/recall_server.go`
```go
package cli

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/spf13/cobra"
    "github.com/[user]/edi/internal/recall"
)

var recallServerCmd = &cobra.Command{
    Use:    "recall-server",
    Short:  "Run RECALL MCP server (internal use)",
    Hidden: true,
    RunE:   runRecallServer,
}

func init() {
    recallServerCmd.Flags().String("global-db", "", "Path to global database")
    recallServerCmd.Flags().String("project-db", "", "Path to project database")
    recallServerCmd.Flags().String("session-id", "", "Session ID")
    
    rootCmd.AddCommand(recallServerCmd)
}

func runRecallServer(cmd *cobra.Command, args []string) error {
    globalDB, _ := cmd.Flags().GetString("global-db")
    projectDB, _ := cmd.Flags().GetString("project-db")
    sessionID, _ := cmd.Flags().GetString("session-id")
    
    // Initialize storage (uses project DB primarily, falls back to global)
    storage, err := recall.NewStorage(projectDB)
    if err != nil {
        return err
    }
    
    // Create and run server
    server := recall.NewServer(storage, sessionID)
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Handle shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        cancel()
    }()
    
    return server.Run(ctx)
}
```

### 2.4 MCP Configuration for Claude Code

**Task 2.4.1: Generate MCP config**

EDI should configure Claude Code's MCP settings to use RECALL. Create template:

File: `internal/assets/mcp-config.json`
```json
{
  "mcpServers": {
    "recall": {
      "command": "edi",
      "args": ["recall-server", "--session-id", "${SESSION_ID}"],
      "env": {
        "EDI_PROJECT_PATH": "${PROJECT_PATH}"
      }
    }
  }
}
```

### 2.5 Validation Checkpoint

**Acceptance Criteria for Phase 2:**

- [ ] RECALL MCP server starts and accepts connections
- [ ] `recall_search` returns results from SQLite FTS
- [ ] `recall_add` stores items in database
- [ ] `recall_get` retrieves items by ID
- [ ] `recall_feedback` records usefulness
- [ ] `flight_recorder_log` stores entries
- [ ] Claude Code can call RECALL tools during session

**Test Script:**
```bash
#!/bin/bash
set -e

# Build
make build

# Initialize test environment
rm -rf /tmp/recall-test
mkdir -p /tmp/recall-test/.edi/recall
cd /tmp/recall-test

# Test storage directly
./bin/edi recall-server --project-db .edi/recall/project.db --session-id test &
SERVER_PID=$!
sleep 1

# Use MCP test client to verify tools
# (Would need MCP test client implementation)

kill $SERVER_PID

echo "✓ Phase 2 validation passed"
```

---

## Phase 3: Briefing & History (Week 3)

### Goal
Sessions generate history, briefings include context from previous sessions.

### 3.1 Briefing Generator

**Task 3.1.1: Implement briefing generator**

File: `internal/briefing/generator.go`
```go
package briefing

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/[user]/edi/internal/config"
)

func Generate(cfg *config.Config) (string, error) {
    var sb strings.Builder
    
    cwd, _ := os.Getwd()
    
    // Include profile
    if cfg.Briefing.IncludeProfile {
        profile, err := loadProfile(cwd)
        if err == nil && profile != "" {
            sb.WriteString("### Project Context\n\n")
            sb.WriteString(profile)
            sb.WriteString("\n\n")
        }
    }
    
    // Include recent history
    if cfg.Briefing.IncludeHistory {
        history, err := loadRecentHistory(cwd, cfg.Briefing.HistoryEntries)
        if err == nil && len(history) > 0 {
            sb.WriteString("### Recent Sessions\n\n")
            for _, h := range history {
                sb.WriteString(fmt.Sprintf("**%s** (%s ago)\n", 
                    h.Date.Format("Jan 2"), 
                    formatDuration(time.Since(h.Date))))
                sb.WriteString(h.Summary)
                sb.WriteString("\n\n")
            }
        }
    }
    
    // Include task status
    if cfg.Briefing.IncludeTasks {
        tasks, err := loadTaskStatus(cwd)
        if err == nil && tasks.Total > 0 {
            sb.WriteString("### Current Tasks\n\n")
            sb.WriteString(fmt.Sprintf("**Status**: %d completed, %d in progress, %d pending\n\n",
                tasks.Completed, tasks.InProgress, tasks.Pending))
            
            if len(tasks.InProgress) > 0 {
                sb.WriteString("**In Progress:**\n")
                for _, t := range tasks.InProgressItems {
                    sb.WriteString(fmt.Sprintf("- %s\n", t.Description))
                }
                sb.WriteString("\n")
            }
            
            if len(tasks.Ready) > 0 {
                sb.WriteString("**Ready to Start:**\n")
                for _, t := range tasks.ReadyItems {
                    sb.WriteString(fmt.Sprintf("- %s\n", t.Description))
                }
                sb.WriteString("\n")
            }
        }
    }
    
    return sb.String(), nil
}
```

**Task 3.1.2: Load history entries**

File: `internal/briefing/history.go`
```go
package briefing

import (
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"
    
    "gopkg.in/yaml.v3"
)

type HistoryEntry struct {
    SessionID        string    `yaml:"session_id"`
    Date             time.Time `yaml:"started_at"`
    Agent            string    `yaml:"agent"`
    TasksCompleted   []string  `yaml:"tasks_completed"`
    DecisionsCaptured []string `yaml:"decisions_captured"`
    Summary          string    `yaml:"-"`
}

func loadRecentHistory(projectPath string, limit int) ([]HistoryEntry, error) {
    historyDir := filepath.Join(projectPath, ".edi", "history")
    
    entries, err := os.ReadDir(historyDir)
    if err != nil {
        return nil, err
    }
    
    var histories []HistoryEntry
    
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
            continue
        }
        
        path := filepath.Join(historyDir, entry.Name())
        h, err := parseHistoryFile(path)
        if err != nil {
            continue
        }
        
        histories = append(histories, h)
    }
    
    // Sort by date descending
    sort.Slice(histories, func(i, j int) bool {
        return histories[i].Date.After(histories[j].Date)
    })
    
    // Limit
    if len(histories) > limit {
        histories = histories[:limit]
    }
    
    return histories, nil
}

func parseHistoryFile(path string) (HistoryEntry, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return HistoryEntry{}, err
    }
    
    // Parse YAML frontmatter
    parts := strings.SplitN(string(content), "---", 3)
    if len(parts) < 3 {
        return HistoryEntry{}, fmt.Errorf("invalid history format")
    }
    
    var entry HistoryEntry
    if err := yaml.Unmarshal([]byte(parts[1]), &entry); err != nil {
        return HistoryEntry{}, err
    }
    
    // Extract summary (first section after frontmatter)
    entry.Summary = extractSummary(parts[2])
    
    return entry, nil
}
```

**Task 3.1.3: Load task status**

File: `internal/briefing/tasks.go`
```go
package briefing

import (
    "encoding/json"
    "os"
    "path/filepath"
)

type TaskStatus struct {
    Total           int
    Completed       int
    InProgress      int
    Pending         int
    InProgressItems []TaskItem
    ReadyItems      []TaskItem
}

type TaskItem struct {
    ID          string   `json:"id"`
    Description string   `json:"subject"`
    Status      string   `json:"status"`
    Blocks      []string `json:"blocks"`
    BlockedBy   []string `json:"blockedBy"`
}

func loadTaskStatus(projectPath string) (*TaskStatus, error) {
    home, _ := os.UserHomeDir()
    tasksDir := filepath.Join(home, ".claude", "tasks")
    
    status := &TaskStatus{}
    
    // Find task list directories
    entries, err := os.ReadDir(tasksDir)
    if err != nil {
        return status, nil // No tasks is fine
    }
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        sessionDir := filepath.Join(tasksDir, entry.Name())
        taskFiles, _ := os.ReadDir(sessionDir)
        
        for _, tf := range taskFiles {
            if filepath.Ext(tf.Name()) != ".json" {
                continue
            }
            
            taskPath := filepath.Join(sessionDir, tf.Name())
            task, err := loadTask(taskPath)
            if err != nil {
                continue
            }
            
            status.Total++
            
            switch task.Status {
            case "completed", "done":
                status.Completed++
            case "in_progress", "active":
                status.InProgress++
                status.InProgressItems = append(status.InProgressItems, task)
            default:
                status.Pending++
                if len(task.BlockedBy) == 0 {
                    status.ReadyItems = append(status.ReadyItems, task)
                }
            }
        }
    }
    
    return status, nil
}

func loadTask(path string) (TaskItem, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return TaskItem{}, err
    }
    
    var task TaskItem
    if err := json.Unmarshal(data, &task); err != nil {
        return TaskItem{}, err
    }
    
    return task, nil
}
```

### 3.2 History Writer

**Task 3.2.1: Implement history writer for /end command**

The `/end` slash command (executed by Claude) writes history. We provide a utility in the skill:

File: `internal/assets/commands/end.md` (update)
```markdown
---
name: end
description: End the current EDI session
---

# End Session

## Instructions

1. Generate a session summary covering:
   - What was accomplished
   - Key decisions made
   - Architectural patterns established
   - Known issues or blockers

2. Identify capture candidates:
   - New patterns discovered
   - Failures encountered and fixed
   - Important decisions with rationale

3. For each capture candidate, ask:
   ```
   Capture to RECALL?
   [1] Pattern: [description]
   [2] Failure: [description]
   [S] Skip all
   ```

4. Save approved items using `recall_add`:
   ```
   recall_add({
     type: "pattern",
     title: "[brief title]",
     content: "[full description with context]",
     tags: ["[relevant]", "[tags]"]
   })
   ```

5. Write session history file to `.edi/history/`:
   
   Filename: `{YYYY-MM-DD}-{session-id-first-8-chars}.md`
   
   Format:
   ```markdown
   ---
   session_id: [full session ID from context]
   started_at: [session start time]
   ended_at: [current time]
   agent: [current agent mode]
   tasks_completed: [list of task IDs]
   decisions_captured: [list of RECALL IDs from this session]
   ---
   
   # Session Summary
   
   ## Accomplished
   - [bullet points of completed work]
   
   ## Key Decisions
   - [decisions with brief rationale]
   
   ## Patterns Established
   - [any new patterns worth noting]
   
   ## Open Items
   - [work remaining, blockers]
   ```

6. Write flight recorder to `.edi/history/{date}-{session-id}-flight.jsonl`

7. Confirm session ended:
   ```
   Session saved to .edi/history/2026-01-25-abc12345.md
   Captured 2 items to RECALL.
   ```
```

### 3.3 Validation Checkpoint

**Acceptance Criteria for Phase 3:**

- [ ] Briefing includes project profile
- [ ] Briefing includes recent session summaries
- [ ] Briefing includes task status (lightweight, no RECALL queries)
- [ ] `/end` command generates session history file
- [ ] `/end` command prompts for RECALL capture
- [ ] Flight recorder is persisted with history
- [ ] Next session briefing includes previous session summary

**Test Script:**
```bash
#!/bin/bash
set -e

cd /tmp/test-project

# Create mock history
mkdir -p .edi/history
cat > .edi/history/2026-01-24-abc12345.md << 'EOF'
---
session_id: abc12345-full-uuid
started_at: 2026-01-24T10:00:00Z
ended_at: 2026-01-24T12:00:00Z
agent: coder
tasks_completed: []
decisions_captured: [P-001]
---

# Session Summary

## Accomplished
- Set up project structure
- Implemented authentication module

## Key Decisions
- Using JWT for auth tokens
EOF

# Run edi and verify briefing includes history
# (Manual verification step)

echo "✓ Phase 3 validation passed"
```

---

## Phase 4: Task Integration (Week 4)

### Goal
Tasks have RECALL context, decisions flow between dependent tasks.

### 4.1 Task Annotations

**Task 4.1.1: Implement task annotation storage**

File: `internal/tasks/annotations.go`
```go
package tasks

import (
    "os"
    "path/filepath"
    "time"
    
    "gopkg.in/yaml.v3"
)

type TaskAnnotation struct {
    TaskID      string    `yaml:"task_id"`
    Description string    `yaml:"description"`
    CreatedAt   time.Time `yaml:"created_at"`
    
    RecallContext struct {
        Patterns  []string `yaml:"patterns"`
        Failures  []string `yaml:"failures"`
        Decisions []string `yaml:"decisions"`
        Query     string   `yaml:"query"`
    } `yaml:"recall_context"`
    
    InheritedContext []InheritedDecision `yaml:"inherited_context"`
    
    ExecutionContext struct {
        DecisionsMade []Decision `yaml:"decisions_made"`
        Discoveries   []string   `yaml:"discoveries"`
        CapturedTo    []string   `yaml:"captured_to"`
    } `yaml:"execution_context"`
}

type InheritedDecision struct {
    FromTaskID string `yaml:"from_task_id"`
    Decision   string `yaml:"decision"`
    Rationale  string `yaml:"rationale"`
}

type Decision struct {
    Summary   string `yaml:"summary"`
    Rationale string `yaml:"rationale"`
    Propagate bool   `yaml:"propagate"`
}

func LoadAnnotation(projectPath, taskID string) (*TaskAnnotation, error) {
    path := filepath.Join(projectPath, ".edi", "tasks", taskID+".yaml")
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var annotation TaskAnnotation
    if err := yaml.Unmarshal(data, &annotation); err != nil {
        return nil, err
    }
    
    return &annotation, nil
}

func SaveAnnotation(projectPath string, annotation *TaskAnnotation) error {
    dir := filepath.Join(projectPath, ".edi", "tasks")
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    
    path := filepath.Join(dir, annotation.TaskID+".yaml")
    
    data, err := yaml.Marshal(annotation)
    if err != nil {
        return err
    }
    
    return os.WriteFile(path, data, 0644)
}
```

### 4.2 /task Slash Command

**Task 4.2.1: Create /task command**

File: `internal/assets/commands/task.md`
```markdown
---
name: task
aliases: [tasks]
description: Manage task-based workflows with RECALL enrichment
---

# /task [task-id | description]

## No Arguments: Show Task Status

Display current task list with status, dependencies, and annotation summaries.

```
Tasks: 3 completed, 2 in progress, 4 pending

In Progress:
- task-abc123-4: Implement payment retry logic
  RECALL: 2 patterns, 2 failures, 1 decision
  
- task-abc123-5: Write payment integration tests
  Blocked by: task-abc123-4
  RECALL: 1 pattern, 1 failure

Ready to Start:
- task-abc123-6: Implement refund service
- task-abc123-7: Add payment webhooks
```

Do NOT load full RECALL content. Show summary counts only.

## With Task ID: Pick Up Task

When picking up a specific task (e.g., `/task task-abc123-4`):

1. Load task annotation from `.edi/tasks/{task-id}.yaml`
2. Display stored RECALL context:
   ```
   Picking up: Implement payment retry logic
   
   RECALL Context (from task creation):
   - P-008: Exponential backoff with jitter
   - P-041: Circuit breaker pattern
   - F-023: Memory leak with unbounded retry queue
   - ADR-031: Payment service architecture
   
   Inherited from parent tasks:
   - From task-abc123-2: "Use Stripe as payment provider"
   - From task-abc123-3: "Idempotency keys use UUIDv7"
   ```

3. Check flight recorder for parallel discoveries:
   - Look for entries with tag "parallel-discovery" from current session
   - Display relevant discoveries from other tasks

4. Begin work with full context

## With Description: Create New Tasks

When given a description (e.g., `/task Implement billing system with Stripe`):

1. Break work into tasks with dependencies
2. For each task, query RECALL:
   ```
   recall_search({query: "[task description]", types: ["pattern", "failure", "decision"]})
   ```
3. Create annotation file in `.edi/tasks/`
4. Log to flight recorder:
   ```
   flight_recorder_log({
     type: "task_annotation",
     content: "Created task: [description]",
     metadata: {
       task_id: "[id]",
       recall_items: ["P-008", "F-023", ...]
     }
   })
   ```
5. Show task graph and ask to proceed

## During Task Execution

Log significant decisions:
```
flight_recorder_log({
  type: "decision",
  content: "[what you decided]",
  rationale: "[why]",
  metadata: {
    task_id: "[current task]",
    propagate: true,  // for technology choices, API designs, architecture
    decision_type: "technology_choice"
  }
})
```

Share discoveries for parallel tasks:
```
flight_recorder_log({
  type: "observation",
  content: "Stripe API returns 429 with Retry-After header",
  metadata: {
    task_id: "[current task]",
    tag: "parallel-discovery",
    applies_to: ["payment", "refund", "stripe"]
  }
})
```

## On Task Completion

1. Log completion:
   ```
   flight_recorder_log({
     type: "task_complete",
     content: "Completed: [task description]",
     metadata: {
       task_id: "[id]",
       decisions: ["list of decisions made"]
     }
   })
   ```

2. If significant decisions were made, prompt:
   ```
   Task completed: Implement payment retry logic
   
   Decisions made:
   [1] Exponential backoff with jitter, max 5 retries
   [2] Circuit breaker opens after 3 consecutive failures
   
   Capture to RECALL?
   [A] All as pattern
   [1-2] Select specific
   [S] Skip
   ```

3. Update task annotation with execution context
```

### 4.3 Update edi-core Skill

**Task 4.3.1: Add task guidance to skill**

File: `internal/assets/skills/edi-core/SKILL.md`
```markdown
---
name: edi-core
description: Core EDI behaviors for all subagents
---

# EDI Core Skill

## RECALL Integration

Before starting significant work:
```
recall_search({query: "[what you're implementing]", types: ["pattern", "failure", "decision"]})
```

After important decisions:
```
flight_recorder_log({
  type: "decision",
  content: "[what you decided]",
  rationale: "[why]"
})
```

## Task Integration

### When Creating Tasks

After breaking work into tasks:
1. For each task, query RECALL for context
2. Log the task annotation to flight recorder
3. Store annotation in `.edi/tasks/`

### When Picking Up a Task

You'll receive pre-loaded context including:
- RECALL patterns, failures, decisions (queried at task creation)
- Inherited context from completed parent tasks
- Parallel discoveries from concurrent work

Use this context. Don't re-query unless annotations are insufficient.

### During Task Execution

Log decisions that should propagate:
```
flight_recorder_log({
  type: "decision",
  content: "Using Stripe webhooks for payment confirmation",
  rationale: "More reliable than polling per ADR-031",
  metadata: {
    task_id: "[current task]",
    propagate: true,
    decision_type: "technology_choice"  // or api_design, architecture_pattern
  }
})
```

Log discoveries for parallel tasks:
```
flight_recorder_log({
  type: "observation",
  content: "Stripe API returns 429 with Retry-After header",
  metadata: {
    tag: "parallel-discovery",
    applies_to: ["payment", "refund", "stripe"]
  }
})
```

### What Propagates

| Type | Propagates | Example |
|------|------------|---------|
| Technology choice | ✅ | "Using Stripe for payments" |
| API design | ✅ | "POST /payments returns 202" |
| Architecture pattern | ✅ | "Event sourcing for state" |
| Implementation detail | ❌ | "Used mutex vs channel" |
| Bug fix | ❌ | "Fixed nil pointer" |

### On Task Completion

1. Mark decisions that should propagate
2. Return summary to parent with key decisions
3. If prompted, confirm which decisions to capture to RECALL

## Communication Style

- Be concise and direct
- Lead with outcomes, not process
- Log decisions to flight recorder, don't narrate them
- Query RECALL silently, use results naturally
```

### 4.4 Validation Checkpoint

**Acceptance Criteria for Phase 4:**

- [ ] `/task` shows task status without RECALL queries
- [ ] `/task {id}` loads full context from annotations
- [ ] Task creation triggers RECALL queries and stores annotations
- [ ] Decisions with `propagate: true` flow to dependent tasks
- [ ] Parallel discoveries appear in flight recorder
- [ ] Task completion prompts for RECALL capture

---

## Phase 5: Agents & Polish (Week 5-6)

### Goal
Full agent system, all four core agents, subagent definitions, polish.

### 5.1 Agent System

**Task 5.1.1: Implement agent loader**

File: `internal/agents/loader.go`
```go
package agents

import (
    "os"
    "path/filepath"
    
    "gopkg.in/yaml.v3"
)

type Agent struct {
    Name         string   `yaml:"name"`
    Description  string   `yaml:"description"`
    Tools        []string `yaml:"tools"`
    Skills       []string `yaml:"skills"`
    SystemPrompt string   `yaml:"-"`
}

func Load(name string) (*Agent, error) {
    home, _ := os.UserHomeDir()
    cwd, _ := os.Getwd()
    
    // Check project override first
    projectPath := filepath.Join(cwd, ".edi", "agents", name+".md")
    if agent, err := loadAgentFile(projectPath); err == nil {
        return agent, nil
    }
    
    // Check global
    globalPath := filepath.Join(home, ".edi", "agents", name+".md")
    return loadAgentFile(globalPath)
}

func loadAgentFile(path string) (*Agent, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    // Parse frontmatter + body
    agent, body, err := parseAgentFile(content)
    if err != nil {
        return nil, err
    }
    
    agent.SystemPrompt = body
    return agent, nil
}
```

**Task 5.1.2: Create all core agents**

Files in `internal/assets/agents/`:
- `coder.md` — Implementation mode (already done)
- `architect.md` — Planning and design mode
- `reviewer.md` — Code review mode
- `incident.md` — Debugging and incident response

**Task 5.1.3: Create agent-switching commands**

Files in `internal/assets/commands/`:
- `plan.md` — Switch to architect mode
- `build.md` — Switch to coder mode
- `review.md` — Switch to reviewer mode
- `incident.md` — Switch to incident mode

### 5.2 Subagent Definitions

**Task 5.2.1: Create subagent files**

Install to `~/.claude/agents/` during `edi init --global`:
- `edi-researcher.md`
- `edi-web-researcher.md`
- `edi-implementer.md`
- `edi-test-writer.md`
- `edi-doc-writer.md`
- `edi-reviewer.md`
- `edi-debugger.md`

### 5.3 Polish

**Task 5.3.1: Error handling**
- Graceful degradation when RECALL unavailable
- Helpful error messages
- Recovery suggestions

**Task 5.3.2: Logging**
- Debug logging with `--verbose` flag
- Log to `~/.edi/logs/` when enabled

**Task 5.3.3: Version command**
```go
var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show EDI version",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("EDI %s\n", rootCmd.Version)
    },
}
```

### 5.4 Final Validation

**Full System Test:**

```bash
#!/bin/bash
set -e

# Clean slate
rm -rf ~/.edi /tmp/test-edi-project

# Build
make build

# Initialize global
./bin/edi init --global
test -f ~/.edi/config.yaml
test -f ~/.edi/agents/coder.md
test -f ~/.claude/skills/edi-core/SKILL.md

# Initialize project
mkdir /tmp/test-edi-project && cd /tmp/test-edi-project
~/path/to/bin/edi init
test -f .edi/config.yaml
test -f .edi/profile.md

# Start session
echo "Starting EDI session..."
# Manual test: edi launches Claude Code with context

# Test RECALL (requires running session)
# Manual test: recall_search, recall_add work

# Test /end
# Manual test: creates history file, prompts for capture

# Test /task
# Manual test: creates tasks with RECALL annotations

echo "✓ Full system validation requires manual testing with Claude Code"
```

---

## Appendix A: File Manifest

All files to be created:

```
edi/
├── cmd/edi/main.go
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── launch.go
│   │   ├── recall_server.go
│   │   └── version.go
│   ├── config/
│   │   ├── schema.go
│   │   ├── loader.go
│   │   └── defaults.go
│   ├── briefing/
│   │   ├── generator.go
│   │   ├── history.go
│   │   └── tasks.go
│   ├── launch/
│   │   ├── context.go
│   │   ├── launcher.go
│   │   └── commands.go
│   ├── recall/
│   │   ├── server.go
│   │   ├── tools.go
│   │   ├── storage.go
│   │   ├── schema.sql
│   │   └── fts.go
│   ├── agents/
│   │   └── loader.go
│   ├── tasks/
│   │   └── annotations.go
│   └── assets/
│       ├── embed.go
│       ├── agents/
│       │   ├── coder.md
│       │   ├── architect.md
│       │   ├── reviewer.md
│       │   └── incident.md
│       ├── commands/
│       │   ├── end.md
│       │   ├── task.md
│       │   ├── plan.md
│       │   ├── build.md
│       │   ├── review.md
│       │   └── incident.md
│       └── skills/
│           └── edi-core/
│               └── SKILL.md
├── pkg/types/
│   ├── config.go
│   ├── history.go
│   └── recall.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Appendix B: Dependencies

```go
// go.mod
module github.com/[user]/edi

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/modelcontextprotocol/go-sdk v0.1.0
    github.com/google/uuid v1.6.0
    gopkg.in/yaml.v3 v3.0.1
)
```

---

## Appendix C: Reference Documents

During implementation, refer to:

| Document | Location | Purpose |
|----------|----------|---------|
| Specification Index | `edi-specification-index.md` | Architecture overview |
| Workspace & Config | `edi-workspace-config-spec.md` | Directory structure, config |
| RECALL MCP Server | `recall-mcp-server-spec.md` | MCP tools, storage |
| Session Lifecycle | `edi-session-lifecycle-spec.md` | Briefing, history, tasks |
| Agent System | `edi-agent-system-spec.md` | Agent schema, behaviors |
| Subagents | `edi-subagent-specification.md` | Subagent definitions |
| CLI Commands | `edi-cli-commands-spec.md` | Command implementations |
| EDI Persona | `edi-persona-spec.md` | Voice, humor, style |
| Gaps Analysis | `edi-implementation-gaps-analysis.md` | All decisions |

---

## Summary

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| 1 | Week 1 | CLI skeleton, init, launch, basic context |
| 2 | Week 2 | RECALL MCP server with SQLite FTS |
| 3 | Week 3 | Briefing generator, history system |
| 4 | Week 4 | Task integration with RECALL |
| 5-6 | Week 5-6 | Full agent system, polish, testing |

**Total estimated effort**: 6 weeks

**Key success metric**: User can run `edi`, work with Claude Code, run `/end`, and the next `edi` session has full continuity.
