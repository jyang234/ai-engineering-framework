# EDI Workspace & Configuration Specification

> **Implementation Status (January 31, 2026):** Config schema implemented but significantly simplified. Missing: user config, defaults.skills, history retention, claude_code section, Viper env var prefix, config validation, migration logic.

**Status**: Draft
**Created**: January 25, 2026
**Version**: 0.1
**Depends On**: RECALL MCP Server Specification v0.2

---

## Table of Contents

1. [Overview](#1-overview)
2. [Design Principles](#2-design-principles)
3. [Directory Structure](#3-directory-structure)
4. [Global Workspace (~/.edi/)](#4-global-workspace)
5. [Project Workspace (.edi/)](#5-project-workspace)
6. [Configuration Schema](#6-configuration-schema)
7. [File Formats](#7-file-formats)
8. [Environment Variables](#8-environment-variables)
9. [Initialization & First Run](#9-initialization--first-run)
10. [Migration & Versioning](#10-migration--versioning)

---

## 1. Overview

### Purpose

This specification defines:
- **Where** EDI stores its files (workspace structure)
- **What** those files contain (formats and schemas)
- **How** configuration is loaded and merged (precedence rules)

### Scope Boundaries

| In Scope | Out of Scope |
|----------|--------------|
| Directory layout | File content generation (see History, Agents specs) |
| Configuration schemas | Runtime behavior |
| File format definitions | CLI implementation |
| Initialization process | RECALL storage (see RECALL spec) |

### Key Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                    EDI Workspace Layout                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ~/.edi/                          ~/project/.edi/                │
│  ├── config.yaml      ─────────►  ├── config.yaml               │
│  ├── agents/          (merged)    ├── agents/     (overrides)   │
│  ├── skills/                      ├── profile.md                │
│  ├── commands/                    ├── history/                  │
│  └── recall/          ─────────►  └── recall/                   │
│      (global scope)   (combined)      (project scope)           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Design Principles

### 2.1 Established Decisions

| Decision | Rationale |
|----------|-----------|
| Two-tier workspace (global + project) | Global = framework defaults; Project = customization |
| YAML for configuration | Human-readable, well-supported, allows comments |
| Markdown for content | Agents, profiles, history are prose-heavy |
| Project-level overrides | Teams can customize without forking |
| XDG-inspired paths | Familiar to Unix users; portable |

### 2.2 Precedence Rules

Configuration is merged with project taking precedence:

```
1. Environment variables (highest)
2. Project config (~/.edi/config.yaml in project)
3. Global config (~/.edi/config.yaml)
4. Built-in defaults (lowest)
```

For agents and skills:
```
1. Project-specific definition (.edi/agents/coder.md)
2. Global definition (~/.edi/agents/coder.md)
3. Built-in default (embedded in EDI binary)
```

### 2.3 Portability

| Component | Portable? | Notes |
|-----------|-----------|-------|
| `~/.edi/config.yaml` | Yes | Copy to new machine |
| `~/.edi/agents/` | Yes | Copy to new machine |
| `~/.edi/skills/` | Yes | Copy to new machine |
| `~/.edi/recall/` | **No** | Rebuild from source (contains embeddings) |
| `.edi/config.yaml` | Yes | Commit to repo |
| `.edi/profile.md` | Yes | Commit to repo |
| `.edi/history/` | Maybe | Team preference |
| `.edi/recall/` | **No** | Rebuild from source |

---

## 3. Directory Structure

### 3.1 Complete Layout

```
~/.edi/                                 # Global EDI installation
├── config.yaml                         # Global configuration
├── agents/                             # Agent definitions
│   ├── architect.md                    # System design agent
│   ├── coder.md                        # Implementation agent
│   ├── reviewer.md                     # Code review agent
│   └── incident.md                     # Incident response agent
├── skills/                             # Skill library
│   ├── org-standards/                  # Organization standards
│   │   └── SKILL.md
│   ├── coding/                         # Coding patterns
│   │   └── SKILL.md
│   ├── security/                       # Security guidelines
│   │   └── SKILL.md
│   └── testing/                        # Testing practices
│       └── SKILL.md
├── commands/                           # Slash command definitions
│   ├── edi.md                          # /edi command
│   ├── plan.md                         # /plan command
│   ├── build.md                        # /build command
│   ├── review.md                       # /review command
│   └── end.md                          # /end command
├── recall/                             # RECALL global data
│   ├── config.yaml                     # RECALL-specific config
│   ├── global.db                       # SQLite metadata
│   └── qdrant/                         # Vector storage
│       └── collections/
│           ├── recall_code_global/
│           └── recall_docs_global/
├── bin/                                # EDI binaries
│   ├── edi                             # Main CLI
│   └── recall-mcp                      # RECALL MCP server
├── cache/                              # Temporary cache
│   ├── models/                         # Downloaded ONNX models
│   └── embeddings/                     # Embedding cache
└── logs/                               # Debug logs (optional)
    └── edi.log

~/project/                              # Any project directory
├── .edi/                               # Project-specific EDI data
│   ├── config.yaml                     # Project configuration
│   ├── profile.md                      # Project context document
│   ├── agents/                         # Project agent overrides (optional)
│   │   └── coder.md                    # Override default coder
│   ├── skills/                         # Project-specific skills (optional)
│   │   └── our-api/
│   │       └── SKILL.md
│   ├── tasks/                          # Task annotations (RECALL context per task)
│   │   ├── task-001.yaml               # Annotation for task-001
│   │   ├── task-002.yaml               # Annotation for task-002
│   │   └── task-003.yaml               # Annotation for task-003
│   ├── history/                        # Session summaries
│   │   ├── 2026-01-22-abc123.md
│   │   ├── 2026-01-23-def456.md
│   │   └── 2026-01-24-ghi789.md
│   └── recall/                         # RECALL project data
│       ├── project.db                  # SQLite metadata
│       ├── indexed_paths.txt           # Tracked paths
│       └── qdrant/                     # Vector storage (if separate)
│           └── collections/
│               ├── recall_code_project/
│               └── recall_docs_project/
├── .claude/                            # Claude Code native (EDI doesn't modify)
│   ├── settings.json
│   └── tasks/                          # Native Claude Code Tasks storage
└── ... (project files)
```

### 3.2 Directory Purposes

| Directory | Purpose | Created By |
|-----------|---------|------------|
| `~/.edi/` | Global EDI installation | `edi init` or installer |
| `~/.edi/agents/` | Default agent definitions | Installation |
| `~/.edi/skills/` | Shared skill library | User curation |
| `~/.edi/commands/` | Slash command definitions | Installation |
| `~/.edi/recall/` | Global RECALL knowledge | RECALL indexing |
| `~/.edi/bin/` | EDI executables | Installation |
| `~/.edi/cache/` | Temporary data | Runtime |
| `.edi/` | Project-specific data | `edi init` in project |
| `.edi/tasks/` | Task annotations (RECALL context) | Task creation hooks |
| `.edi/history/` | Session summaries | `/end` command |
| `.edi/recall/` | Project RECALL data | RECALL indexing |
| `.claude/` | Claude Code native | Claude Code (hands off) |
| `.claude/tasks/` | Native Claude Code Tasks | Claude Code (hands off) |

### 3.3 Gitignore Recommendations

**Project `.gitignore`**:
```gitignore
# EDI - Always ignore
.edi/recall/              # Contains embeddings, rebuild locally
.edi/cache/               # Temporary data

# EDI - Optional (team decision)
# .edi/history/           # Uncomment to exclude session history
```

**What to commit**:
```
.edi/config.yaml          # Project settings
.edi/profile.md           # Project context
.edi/agents/              # Custom agents
.edi/skills/              # Project-specific skills
.edi/history/             # Optional: session history
```

---

## 4. Global Workspace (~/.edi/)

### 4.1 config.yaml

The main EDI configuration file.

```yaml
# EDI Global Configuration
# ~/.edi/config.yaml
version: 1

# User identity (optional, for attribution)
user:
  name: John Doe
  email: john@example.com

# Default agent to load on session start
defaults:
  agent: coder
  # Skills loaded for all agents
  skills:
    - org-standards
    - coding

# Briefing configuration
briefing:
  # Include these sources in session briefing
  sources:
    tasks: true           # Claude Code Tasks
    history: true         # Recent session history
    recall: true          # RECALL context query
  # How many history entries to include
  history_depth: 3
  # Auto-query RECALL on session start
  recall_auto_query: true

# History configuration
history:
  # Where to save session summaries
  location: project       # 'project' (.edi/history/) or 'global' (~/.edi/history/)
  # Retention policy
  retention:
    max_entries: 100      # Per project
    max_age_days: 365     # Delete older entries
  # Auto-save on session end
  auto_save: true

# Capture configuration
capture:
  # Prompt for capture on /end
  prompt_on_end: true
  # Default destination for captures
  default_scope: project  # 'project' or 'global'
  # Types of content to suggest capturing
  suggest_types:
    - decision
    - pattern
    - lesson

# RECALL integration
recall:
  # Path to RECALL config (or inline)
  config: ~/.edi/recall/config.yaml
  # Auto-start MCP server
  auto_start: true

# Claude Code integration
claude_code:
  # Skills directory to load
  skills_path: ~/.edi/skills
  # Commands directory
  commands_path: ~/.edi/commands
  # Additional MCP servers
  mcp_servers:
    - name: recall
      command: ~/.edi/bin/recall-mcp
      args: []

# Telemetry (optional)
telemetry:
  enabled: false
  # endpoint: https://telemetry.example.com
```

### 4.2 agents/ Directory

Agent definitions use Markdown with YAML frontmatter.

**Example: `~/.edi/agents/coder.md`**
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
  - Include tests with implementations
  - Follow project coding standards
  - Ask clarifying questions before large changes
tools:
  required:
    - recall_search
    - recall_context
  optional:
    - recall_add
---

# Coder Agent

You are a skilled software engineer focused on implementation. Your primary responsibilities are:

1. **Writing Code**: Implement features following project patterns and standards
2. **Testing**: Write tests alongside code, not as an afterthought
3. **Documentation**: Add inline comments and update docs as needed

## Before You Start

- Check RECALL for existing patterns: `recall_search("similar feature implementation")`
- Review recent decisions: `recall_search("ADR related to this area")`
- Understand the context: `recall_context(current_file)`

## Working Style

- Break large tasks into smaller commits
- Explain your approach before diving into code
- Flag potential issues or edge cases early

## What Not To Do

- Don't refactor unrelated code without asking
- Don't skip tests to save time
- Don't ignore linter warnings
```

### 4.3 skills/ Directory

Skills follow Claude Code's skill format.

**Example: `~/.edi/skills/coding/SKILL.md`**
```markdown
# Coding Standards Skill

## Code Style

- Use meaningful variable names
- Keep functions under 50 lines
- Prefer composition over inheritance

## Error Handling

- Always handle errors explicitly
- Use typed errors where possible
- Log errors with context

## Comments

- Comment "why", not "what"
- Update comments when code changes
- Use TODO/FIXME with ticket numbers
```

### 4.4 commands/ Directory

Slash commands are Markdown files with YAML frontmatter.

**Example: `~/.edi/commands/plan.md`**
```markdown
---
name: plan
aliases: [architect, design]
description: Switch to architect agent for system design
agent: architect
---

# /plan Command

Switches to the **architect** agent for high-level design work.

## When to Use

- Starting a new feature that needs design
- Making cross-cutting architectural decisions
- Creating or updating ADRs

## What Happens

1. Loads architect agent configuration
2. Queries RECALL for relevant architecture context
3. Sets design-focused behavior mode

## Example Usage

```
/plan
> I need to design a new authentication system
```
```

---

## 5. Project Workspace (.edi/)

### 5.1 config.yaml

Project-specific configuration that overrides global settings.

```yaml
# EDI Project Configuration
# ~/project/.edi/config.yaml
version: 1

# Project identification
project:
  name: my-awesome-project
  description: A web application for awesome things
  # Links to external systems (optional)
  links:
    repo: https://github.com/org/my-awesome-project
    docs: https://docs.example.com
    jira: https://jira.example.com/browse/AWESOME

# Override default agent for this project
defaults:
  agent: coder
  skills:
    - coding
    - our-api           # Project-specific skill

# Project-specific briefing settings
briefing:
  # Custom RECALL queries for briefing
  recall_queries:
    - "project architecture overview"
    - "recent ADRs"
  # Include specific files in briefing context
  include_files:
    - docs/ARCHITECTURE.md
    - docs/adr/

# History settings for this project
history:
  # Override global retention
  retention:
    max_entries: 50     # Smaller for this project

# RECALL project settings
recall:
  # Paths to auto-index
  auto_index:
    - docs/adr/
    - README.md
    - docs/*.md
  # Paths to exclude from indexing
  exclude:
    - node_modules/
    - dist/
    - "*.test.ts"
    - "*.spec.ts"
  # Default search scope
  default_scope: project  # 'project', 'global', or 'all'

# Agent overrides for this project
agents:
  coder:
    # Additional skills for coder in this project
    skills:
      - our-api
      - our-testing
    # Override behaviors
    behaviors:
      - "Use our custom logger, not console.log"
      - "All API calls go through the SDK"
```

### 5.2 profile.md

Project context document that Claude loads for understanding.

```markdown
# Project Profile: my-awesome-project

## Overview

This is a TypeScript web application built with React and Node.js. It provides
a dashboard for managing customer accounts.

## Architecture

- **Frontend**: React 18 with TypeScript, Vite bundler
- **Backend**: Node.js with Express, TypeScript
- **Database**: PostgreSQL with Prisma ORM
- **Auth**: OAuth2 via Auth0

## Key Directories

- `src/client/` - React frontend
- `src/server/` - Express backend
- `src/shared/` - Shared types and utilities
- `docs/adr/` - Architecture decision records

## Important Patterns

### API Calls
All API calls should use the SDK in `src/client/api/sdk.ts`. Never use fetch directly.

### State Management
We use Zustand for client state. See `src/client/stores/` for examples.

### Testing
- Unit tests: Vitest
- E2E tests: Playwright
- Run with `npm test`

## Team Conventions

- Branch naming: `feature/TICKET-description` or `fix/TICKET-description`
- Commit messages: Conventional Commits format
- PRs require one approval

## Current Focus

We're currently working on the billing integration (Q1 2026).
Key decisions are tracked in `docs/adr/`.
```

### 5.3 history/ Directory

Session summaries stored as Markdown files.

**Filename format**: `{date}-{session_id}.md`

**Example**: `.edi/history/2026-01-24-abc123.md`
```markdown
---
session_id: abc123
date: 2026-01-24
start_time: "09:15:00"
end_time: "11:30:00"
duration_minutes: 135
agent: coder
tasks_completed: 3
tasks_started: 1
---

# Session Summary: January 24, 2026

## What We Did

1. **Implemented user profile API** (TICKET-123)
   - Added GET /api/users/:id endpoint
   - Added PATCH /api/users/:id endpoint
   - Wrote unit tests for both

2. **Fixed date formatting bug** (TICKET-456)
   - Issue was timezone handling in date utils
   - Added timezone-aware formatting function

3. **Started billing integration** (TICKET-789)
   - Reviewed Stripe API documentation
   - Created initial types in `src/shared/billing.ts`
   - Blocked: Need API keys from finance team

## Decisions Made

- **Use Stripe instead of Paddle**: Better API, more features we need
- **Store billing state in separate table**: Cleaner separation of concerns

## Questions for Next Session

- How should we handle failed payments?
- Do we need webhook retry logic?

## Files Modified

- `src/server/routes/users.ts`
- `src/server/utils/dates.ts`
- `src/shared/billing.ts` (new)
- `docs/adr/003-stripe-billing.md` (new)
```

---

## 6. Configuration Schema

### 6.1 Schema Definitions (JSON Schema)

**Global Config Schema**:
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "EDI Global Configuration",
  "type": "object",
  "properties": {
    "version": {
      "type": "integer",
      "const": 1
    },
    "user": {
      "type": "object",
      "properties": {
        "name": { "type": "string" },
        "email": { "type": "string", "format": "email" }
      }
    },
    "defaults": {
      "type": "object",
      "properties": {
        "agent": { "type": "string", "default": "coder" },
        "skills": {
          "type": "array",
          "items": { "type": "string" },
          "default": []
        }
      }
    },
    "briefing": {
      "type": "object",
      "properties": {
        "sources": {
          "type": "object",
          "properties": {
            "tasks": { "type": "boolean", "default": true },
            "history": { "type": "boolean", "default": true },
            "recall": { "type": "boolean", "default": true }
          }
        },
        "history_depth": { "type": "integer", "minimum": 1, "maximum": 10, "default": 3 },
        "recall_auto_query": { "type": "boolean", "default": true }
      }
    },
    "history": {
      "type": "object",
      "properties": {
        "location": { "type": "string", "enum": ["project", "global"], "default": "project" },
        "retention": {
          "type": "object",
          "properties": {
            "max_entries": { "type": "integer", "minimum": 1, "default": 100 },
            "max_age_days": { "type": "integer", "minimum": 1, "default": 365 }
          }
        },
        "auto_save": { "type": "boolean", "default": true }
      }
    },
    "capture": {
      "type": "object",
      "properties": {
        "prompt_on_end": { "type": "boolean", "default": true },
        "default_scope": { "type": "string", "enum": ["project", "global"], "default": "project" },
        "suggest_types": {
          "type": "array",
          "items": { "type": "string", "enum": ["decision", "pattern", "lesson", "playbook"] },
          "default": ["decision", "pattern", "lesson"]
        }
      }
    },
    "recall": {
      "type": "object",
      "properties": {
        "config": { "type": "string" },
        "auto_start": { "type": "boolean", "default": true }
      }
    },
    "claude_code": {
      "type": "object",
      "properties": {
        "skills_path": { "type": "string" },
        "commands_path": { "type": "string" },
        "mcp_servers": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": { "type": "string" },
              "command": { "type": "string" },
              "args": { "type": "array", "items": { "type": "string" } }
            },
            "required": ["name", "command"]
          }
        }
      }
    }
  },
  "required": ["version"]
}
```

### 6.2 Configuration Loading (Go)

```go
package config

import (
    "os"
    "path/filepath"

    "github.com/spf13/viper"
)

// Config represents the merged EDI configuration
type Config struct {
    Version    int            `mapstructure:"version"`
    User       UserConfig     `mapstructure:"user"`
    Defaults   DefaultsConfig `mapstructure:"defaults"`
    Briefing   BriefingConfig `mapstructure:"briefing"`
    History    HistoryConfig  `mapstructure:"history"`
    Capture    CaptureConfig  `mapstructure:"capture"`
    Recall     RecallConfig   `mapstructure:"recall"`
    ClaudeCode ClaudeConfig   `mapstructure:"claude_code"`
    Project    ProjectConfig  `mapstructure:"project"` // Only in project config
}

// LoadConfig loads and merges global + project configuration
func LoadConfig(projectPath string) (*Config, error) {
    v := viper.New()

    // Set defaults
    setDefaults(v)

    // Load global config
    globalPath := filepath.Join(os.Getenv("HOME"), ".edi", "config.yaml")
    if exists(globalPath) {
        v.SetConfigFile(globalPath)
        if err := v.ReadInConfig(); err != nil {
            return nil, fmt.Errorf("reading global config: %w", err)
        }
    }

    // Load project config (merges with global)
    if projectPath != "" {
        projectConfigPath := filepath.Join(projectPath, ".edi", "config.yaml")
        if exists(projectConfigPath) {
            v.SetConfigFile(projectConfigPath)
            if err := v.MergeInConfig(); err != nil {
                return nil, fmt.Errorf("merging project config: %w", err)
            }
        }
    }

    // Override with environment variables
    v.SetEnvPrefix("EDI")
    v.AutomaticEnv()

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    return &cfg, nil
}

func setDefaults(v *viper.Viper) {
    v.SetDefault("version", 1)
    v.SetDefault("defaults.agent", "coder")
    v.SetDefault("defaults.skills", []string{})
    v.SetDefault("briefing.sources.tasks", true)
    v.SetDefault("briefing.sources.history", true)
    v.SetDefault("briefing.sources.recall", true)
    v.SetDefault("briefing.history_depth", 3)
    v.SetDefault("briefing.recall_auto_query", true)
    v.SetDefault("history.location", "project")
    v.SetDefault("history.retention.max_entries", 100)
    v.SetDefault("history.retention.max_age_days", 365)
    v.SetDefault("history.auto_save", true)
    v.SetDefault("capture.prompt_on_end", true)
    v.SetDefault("capture.default_scope", "project")
    v.SetDefault("capture.suggest_types", []string{"decision", "pattern", "lesson"})
    v.SetDefault("recall.auto_start", true)
}
```

### 6.3 Configuration Validation

```go
package config

import "fmt"

// Validate checks configuration for errors
func (c *Config) Validate() error {
    if c.Version != 1 {
        return fmt.Errorf("unsupported config version: %d", c.Version)
    }

    if c.Briefing.HistoryDepth < 1 || c.Briefing.HistoryDepth > 10 {
        return fmt.Errorf("briefing.history_depth must be 1-10, got %d", c.Briefing.HistoryDepth)
    }

    if c.History.Retention.MaxEntries < 1 {
        return fmt.Errorf("history.retention.max_entries must be positive")
    }

    if c.History.Location != "project" && c.History.Location != "global" {
        return fmt.Errorf("history.location must be 'project' or 'global'")
    }

    return nil
}
```

---

## 7. File Formats

### 7.1 Format Summary

| File Type | Format | Frontmatter |
|-----------|--------|-------------|
| Configuration | YAML | No |
| Agents | Markdown | YAML |
| Skills | Markdown | No (Claude Code format) |
| Commands | Markdown | YAML |
| Profile | Markdown | No |
| History | Markdown | YAML |

### 7.2 Agent Frontmatter Schema

```yaml
# Required fields
name: string              # Agent identifier (lowercase, no spaces)
description: string       # Human-readable description
version: integer          # Schema version (currently 1)

# Optional fields
skills: string[]          # Skills to load with this agent
behaviors: string[]       # Behavioral guidelines
tools:
  required: string[]      # MCP tools agent must have
  optional: string[]      # MCP tools agent may use
model:                    # Model preferences (optional)
  preferred: string       # e.g., "claude-3-5-sonnet"
  fallback: string        # e.g., "claude-3-haiku"
```

### 7.3 Command Frontmatter Schema

```yaml
# Required fields
name: string              # Command name (without /)
description: string       # Short description for help

# Optional fields
aliases: string[]         # Alternative names
agent: string             # Agent to switch to (if any)
skills: string[]          # Additional skills to load
args:                     # Command arguments
  - name: string
    type: string          # 'string', 'boolean', 'number'
    required: boolean
    default: any
    description: string
```

### 7.4 History Frontmatter Schema

```yaml
# Required fields
session_id: string        # Unique session identifier
date: string              # ISO date (YYYY-MM-DD)

# Optional fields
start_time: string        # ISO time (HH:MM:SS)
end_time: string          # ISO time (HH:MM:SS)
duration_minutes: integer # Session duration
agent: string             # Primary agent used
tasks_completed: integer  # Count of completed tasks
tasks_started: integer    # Count of started tasks
tags: string[]            # User-defined tags
```

---

## 8. Environment Variables

### 8.1 EDI Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `EDI_HOME` | Override ~/.edi location | `/opt/edi` |
| `EDI_CONFIG` | Override config file path | `/etc/edi/config.yaml` |
| `EDI_DEBUG` | Enable debug logging | `true` |
| `EDI_LOG_LEVEL` | Log verbosity | `debug`, `info`, `warn`, `error` |

### 8.2 API Keys (for RECALL)

| Variable | Purpose |
|----------|---------|
| `VOYAGE_API_KEY` | Voyage AI embeddings |
| `OPENAI_API_KEY` | OpenAI embeddings |
| `ANTHROPIC_API_KEY` | Claude API (Stage 3 reranking) |

### 8.3 Environment Variable Overrides

Any config value can be overridden via environment variable:

```bash
# Pattern: EDI_{SECTION}_{KEY}
EDI_DEFAULTS_AGENT=architect           # Override defaults.agent
EDI_BRIEFING_HISTORY_DEPTH=5           # Override briefing.history_depth
EDI_HISTORY_AUTO_SAVE=false            # Override history.auto_save
```

---

## 9. Initialization & First Run

### 9.1 Global Initialization

```bash
$ edi init --global
```

Creates:
```
~/.edi/
├── config.yaml           # Default global config
├── agents/               # Built-in agents
│   ├── architect.md
│   ├── coder.md
│   ├── reviewer.md
│   └── incident.md
├── skills/               # Empty, user populates
├── commands/             # Built-in commands
│   ├── edi.md
│   ├── plan.md
│   ├── build.md
│   ├── review.md
│   └── end.md
├── recall/
│   └── config.yaml       # Default RECALL config
├── bin/
│   ├── edi               # Symlink or copy
│   └── recall-mcp        # Symlink or copy
└── cache/
    └── models/           # Will download on first use
```

### 9.2 Project Initialization

```bash
$ cd ~/project
$ edi init
```

Creates:
```
.edi/
├── config.yaml           # Minimal project config
├── profile.md            # Template profile
└── history/              # Empty directory
```

**Generated `config.yaml`**:
```yaml
# EDI Project Configuration
version: 1

project:
  name: project           # Inferred from directory name
  description: ""         # User fills in

# Add project-specific settings below
```

**Generated `profile.md`**:
```markdown
# Project Profile: project

## Overview

[Describe what this project does]

## Architecture

[Describe the tech stack and structure]

## Key Directories

[List important directories and their purposes]

## Important Patterns

[Document project-specific patterns and conventions]

## Team Conventions

[Document team workflows and standards]
```

### 9.3 First Run Checklist

```go
package init

// FirstRunChecks validates EDI is ready to use
func FirstRunChecks() []Check {
    return []Check{
        {
            Name:        "global_dir",
            Description: "Global EDI directory exists",
            Check:       func() bool { return exists(ediHome()) },
            Fix:         "Run: edi init --global",
        },
        {
            Name:        "config_valid",
            Description: "Configuration is valid",
            Check:       func() bool { return validateConfig() == nil },
            Fix:         "Check ~/.edi/config.yaml syntax",
        },
        {
            Name:        "recall_config",
            Description: "RECALL is configured",
            Check:       func() bool { return exists(recallConfig()) },
            Fix:         "Run: edi init --global",
        },
        {
            Name:        "api_keys",
            Description: "API keys are set",
            Check:       func() bool { return hasAPIKeys() },
            Fix:         "Set VOYAGE_API_KEY and OPENAI_API_KEY",
        },
    }
}
```

---

## 10. Migration & Versioning

### 10.1 Config Version History

| Version | Changes | Migration |
|---------|---------|-----------|
| 1 | Initial release | N/A |

### 10.2 Migration Strategy

```go
package config

// Migrate updates config to current version
func Migrate(cfg *Config) (*Config, error) {
    switch cfg.Version {
    case 0:
        // Pre-release config, convert to v1
        cfg.Version = 1
        if cfg.History.Location == "" {
            cfg.History.Location = "project"
        }
        return cfg, nil
    case 1:
        // Current version, no migration needed
        return cfg, nil
    default:
        return nil, fmt.Errorf("unknown config version: %d", cfg.Version)
    }
}
```

### 10.3 Breaking Changes Policy

- **Major version bump**: Breaking changes to config schema
- **Minor version bump**: New optional fields
- **Patch version bump**: Bug fixes only

When breaking changes occur:
1. Bump config `version` field
2. Implement migration in `Migrate()`
3. Document changes in changelog
4. Provide migration guide

---

## Appendix A: Quick Reference

### File Locations

| What | Where |
|------|-------|
| Global config | `~/.edi/config.yaml` |
| Project config | `.edi/config.yaml` |
| Global agents | `~/.edi/agents/*.md` |
| Project agents | `.edi/agents/*.md` |
| Skills | `~/.edi/skills/*/SKILL.md` |
| Commands | `~/.edi/commands/*.md` |
| Profile | `.edi/profile.md` |
| History | `.edi/history/*.md` |
| RECALL global | `~/.edi/recall/` |
| RECALL project | `.edi/recall/` |

### Config Precedence

```
ENV > project config > global config > defaults
```

### Common Operations

```bash
# Initialize EDI globally
edi init --global

# Initialize EDI in project
cd ~/project && edi init

# Validate configuration
edi config validate

# Show merged configuration
edi config show

# Edit global config
edi config edit --global

# Edit project config
edi config edit
```

---

## Appendix B: Related Specifications

| Spec | Relationship |
|------|--------------|
| RECALL MCP Server | Storage paths, config format |
| History System | History file format, retention |
| Agent System | Agent file format, loading |
| CLI Architecture | Init commands, config commands |

---

## Appendix C: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 25, 2026 | Two-tier workspace | Global defaults + project overrides |
| Jan 25, 2026 | YAML for config | Human-readable, comments supported |
| Jan 25, 2026 | Markdown for content | Agents, profiles, history are prose |
| Jan 25, 2026 | XDG-inspired paths | Familiar, portable |
| Jan 25, 2026 | Project config merges | Viper handles merge semantics |
| Jan 25, 2026 | Env var overrides | Standard twelve-factor pattern |
