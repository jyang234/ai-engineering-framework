# EDI CLI & Commands Specification

> **Implementation Status (January 31, 2026):** Core launch mechanism (syscall.Exec, --append-system-prompt-file) matches. Command tree evolved: added doctor, sync; actual Claude flags differ from spec (no --mcp-server, --skills-path, --allowedTools flags). Ralph noted as future.

**Status**: Draft
**Created**: January 25, 2026
**Version**: 0.1
**Depends On**: All previous specs (Workspace, Session Lifecycle, Agent System)

---

## Table of Contents

1. [Overview](#1-overview)
2. [CLI Architecture](#2-cli-architecture)
3. [Command Specifications](#3-command-specifications)
4. [Claude Code Integration](#4-claude-code-integration)
5. [Installation & Setup](#5-installation--setup)
6. [Implementation](#6-implementation)

---

## 1. Overview

### What is the EDI CLI?

The EDI CLI is the primary interface for interacting with EDI. It:
- Launches Claude Code with EDI configuration
- Manages sessions (start, end)
- Switches agents during sessions
- Handles configuration and initialization

### Command Types

| Type | Examples | Implementation |
|------|----------|----------------|
| **Shell commands** | `edi`, `edi init` | Go binary |
| **Slash commands** | `/plan`, `/build`, `/end` | Injected into Claude Code |
| **Utility commands** | `edi config`, `edi recall` | Go binary |

### Design Principles

1. **Thin wrapper**: EDI launches Claude Code, doesn't replace it
2. **Convention over configuration**: Sensible defaults, override when needed
3. **Non-intrusive**: Degrades gracefully if components unavailable
4. **Fast startup**: Minimize latency before Claude Code launches

---

## 2. CLI Architecture

### 2.1 High-Level Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              USER                                        │
│                         $ edi [command]                                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           EDI CLI (Go)                                   │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   edi       │  │  edi init   │  │ edi config  │  │ edi recall  │    │
│  │  (start)    │  │  (setup)    │  │  (manage)   │  │  (query)    │    │
│  └──────┬──────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│         │                                                                │
│         ▼                                                                │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     Session Manager                              │    │
│  │  • Load config (global + project)                               │    │
│  │  • Load agent                                                   │    │
│  │  • Start RECALL MCP server                                      │    │
│  │  • Generate briefing                                            │    │
│  │  • Build Claude Code command                                    │    │
│  └──────────────────────────────────┬──────────────────────────────┘    │
│                                     │                                    │
└─────────────────────────────────────┼────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Claude Code                                     │
│                                                                          │
│  Launched with:                                                          │
│  • --system-prompt (agent + skills + briefing)                          │
│  • --mcp-server recall (RECALL MCP server)                              │
│  • --skills-path (EDI skills directory)                                 │
│  • --allowedTools (based on agent config)                               │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Slash Command Handlers                        │    │
│  │  /plan, /build, /review, /incident, /end                        │    │
│  │  (Processed by Claude using injected instructions)              │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 CLI Structure (Cobra)

```go
package main

import (
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "edi",
        Short: "Enhanced Development Intelligence - AI engineering assistant",
        Long:  `EDI provides continuity, knowledge, and specialized behaviors on top of Claude Code.`,
    }

    // Main command (start session)
    rootCmd.AddCommand(startCmd())
    
    // Initialization
    rootCmd.AddCommand(initCmd())
    
    // Configuration
    rootCmd.AddCommand(configCmd())
    
    // RECALL utilities
    rootCmd.AddCommand(recallCmd())
    
    // History utilities
    rootCmd.AddCommand(historyCmd())
    
    // Agent utilities
    rootCmd.AddCommand(agentCmd())
    
    // Version
    rootCmd.AddCommand(versionCmd())

    rootCmd.Execute()
}
```

### 2.3 Command Tree

```
edi                           # Start session (default command)
├── init                      # Initialize EDI
│   ├── --global              # Initialize global ~/.edi/
│   └── --force               # Overwrite existing
├── sync                      # Sync assets to install locations
├── config                    # Configuration management
│   ├── show                  # Show merged config
│   ├── edit                  # Open config in editor
│   │   └── --global          # Edit global config
│   ├── validate              # Validate configuration
│   └── set <key> <value>     # Set config value
├── recall                    # RECALL utilities
│   ├── search <query>        # Search knowledge base
│   ├── index <path>          # Index files
│   ├── status                # Show RECALL status
│   └── server                # Server management
│       ├── start             # Start MCP server
│       ├── stop              # Stop MCP server
│       └── status            # Check server status
├── history                   # History utilities
│   ├── list                  # List recent sessions
│   ├── show <id>             # Show session details
│   └── cleanup               # Apply retention policy
├── agent                     # Agent utilities
│   ├── list                  # List available agents
│   ├── show <name>           # Show agent details
│   └── validate <file>       # Validate agent file
├── ralph                     # (future) Run Ralph loop from EDI
│   └── Currently standalone via ~/.edi/ralph/ralph.sh
└── version                   # Show version info
```

---

## 3. Command Specifications

### 3.1 `edi` (Start Session)

The main command that starts an EDI session.

**Usage**:
```bash
edi [flags] [initial-prompt]
```

**Flags**:
| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--agent` | `-a` | Agent to start with | From config |
| `--no-briefing` | | Skip briefing generation | false |
| `--no-recall` | | Don't start RECALL server | false |
| `--project` | `-p` | Project path | Current directory |
| `--resume` | `-r` | Resume last session | false |
| `--verbose` | `-v` | Verbose output | false |

**Examples**:
```bash
# Start with defaults
edi

# Start with specific agent
edi --agent architect

# Start with initial prompt
edi "Let's work on the payment integration"

# Start in different project
edi --project ~/projects/other-project

# Resume previous session
edi --resume
```

**Implementation**:
```go
func startCmd() *cobra.Command {
    var opts StartOptions
    
    cmd := &cobra.Command{
        Use:   "edi [initial-prompt]",
        Short: "Start an EDI session",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) > 0 {
                opts.InitialPrompt = args[0]
            }
            return runStart(opts)
        },
    }
    
    cmd.Flags().StringVarP(&opts.Agent, "agent", "a", "", "Agent to start with")
    cmd.Flags().BoolVar(&opts.NoBriefing, "no-briefing", false, "Skip briefing")
    cmd.Flags().BoolVar(&opts.NoRecall, "no-recall", false, "Don't start RECALL")
    cmd.Flags().StringVarP(&opts.Project, "project", "p", ".", "Project path")
    cmd.Flags().BoolVarP(&opts.Resume, "resume", "r", false, "Resume last session")
    cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
    
    return cmd
}

func runStart(opts StartOptions) error {
    // 1. Resolve project path
    projectPath, err := resolveProjectPath(opts.Project)
    if err != nil {
        return err
    }

    // 2. Load configuration
    cfg, err := config.LoadConfig(projectPath)
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }

    // 3. Initialize session manager
    mgr := session.NewManager(cfg, projectPath)

    // 4. Create session
    sess, briefing, err := mgr.Start(context.Background(), &session.StartOptions{
        Agent:       opts.Agent,
        NoBriefing:  opts.NoBriefing,
        NoRecall:    opts.NoRecall,
        Resume:      opts.Resume,
    })
    if err != nil {
        return fmt.Errorf("starting session: %w", err)
    }

    // 5. Build Claude Code command
    ccCmd := buildClaudeCodeCommand(sess, briefing, cfg, opts)

    // 6. Print briefing (if generated)
    if briefing != nil && !opts.NoBriefing {
        fmt.Println(briefing.Render(cfg.Project.Name))
        fmt.Println()
    }

    // 7. Launch Claude Code
    return launchClaudeCode(ccCmd)
}
```

### 3.2 `edi sync`

Sync embedded assets to their install locations without touching configuration or data.

**Usage**:
```bash
edi sync
```

**What it syncs**:

| Asset | Source (embedded in binary) | Destination |
|-------|---------------------------|-------------|
| Agents | `internal/assets/agents/*.md` | `~/.edi/agents/` |
| Commands | `internal/assets/commands/*.md` | `~/.edi/commands/` |
| Skills (6) | `internal/assets/skills/*/SKILL.md` | `~/.claude/skills/` |
| Subagents | `internal/assets/subagents/*.md` | `~/.claude/agents/` |

**What it does NOT touch**:
- `~/.edi/config.yaml`
- `~/.edi/recall/` (knowledge database)
- `~/.edi/cache/`, `~/.edi/logs/`
- `.edi/` (project-level directories)

**When to use**:
- After updating agent definitions, skills, or commands in EDI source
- As part of `make sync` (build + install + sync)
- When you want to update assets without a full `edi init --global --force`

**Example**:
```bash
$ edi sync
  Synced agents
  Synced commands
  Synced skills (6)
  Synced subagents

Assets synced successfully.
```

**Prerequisite**: `~/.edi/` must exist. Run `edi init --global` first if it doesn't.

### 3.3 `edi init`

Initialize EDI workspace.

**Usage**:
```bash
edi init [flags]
```

**Flags**:
| Flag | Description | Default |
|------|-------------|---------|
| `--global` | Initialize global ~/.edi/ | false (init project) |
| `--force` | Overwrite existing files | false |

**Behavior**:

**Global init** (`edi init --global`):
- Creates `~/.edi/` directory structure
- Installs default agents
- Creates default config.yaml
- Sets up RECALL directories
- Downloads ONNX models (if not present)

**Project init** (`edi init`):
- Creates `.edi/` in current directory
- Creates minimal config.yaml
- Creates template profile.md
- Creates history/ directory

**Implementation**:
```go
func initCmd() *cobra.Command {
    var global, force bool
    
    cmd := &cobra.Command{
        Use:   "init",
        Short: "Initialize EDI workspace",
        RunE: func(cmd *cobra.Command, args []string) error {
            if global {
                return initGlobal(force)
            }
            return initProject(force)
        },
    }
    
    cmd.Flags().BoolVar(&global, "global", false, "Initialize global ~/.edi/")
    cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")
    
    return cmd
}

func initGlobal(force bool) error {
    home, _ := os.UserHomeDir()
    ediHome := filepath.Join(home, ".edi")

    // Check existing
    if exists(ediHome) && !force {
        return fmt.Errorf("~/.edi already exists (use --force to overwrite)")
    }

    // Create directory structure
    dirs := []string{
        "agents", "skills", "commands", "recall", "bin", "cache/models", "logs",
    }
    for _, d := range dirs {
        os.MkdirAll(filepath.Join(ediHome, d), 0755)
    }

    // Write default config
    writeFile(filepath.Join(ediHome, "config.yaml"), defaultGlobalConfig)

    // Write built-in agents
    for name, content := range builtinAgents {
        writeFile(filepath.Join(ediHome, "agents", name+".md"), content)
    }

    // Write built-in commands
    for name, content := range builtinCommands {
        writeFile(filepath.Join(ediHome, "commands", name+".md"), content)
    }

    fmt.Println("✓ Initialized global EDI at ~/.edi/")
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Set API keys: export VOYAGE_API_KEY=... OPENAI_API_KEY=...")
    fmt.Println("  2. cd to a project and run: edi init")
    fmt.Println("  3. Start a session: edi")
    
    return nil
}

func initProject(force bool) error {
    ediDir := ".edi"

    if exists(ediDir) && !force {
        return fmt.Errorf(".edi already exists (use --force to overwrite)")
    }

    // Create directories
    os.MkdirAll(filepath.Join(ediDir, "history"), 0755)

    // Write config
    projectName := filepath.Base(mustGetwd())
    config := fmt.Sprintf(projectConfigTemplate, projectName)
    writeFile(filepath.Join(ediDir, "config.yaml"), config)

    // Write profile template
    profile := fmt.Sprintf(profileTemplate, projectName)
    writeFile(filepath.Join(ediDir, "profile.md"), profile)

    fmt.Println("✓ Initialized EDI in .edi/")
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Edit .edi/profile.md to describe your project")
    fmt.Println("  2. Start a session: edi")
    
    return nil
}
```

### 3.3 `edi config`

Manage configuration.

**Subcommands**:

```bash
# Show merged configuration
edi config show

# Edit configuration
edi config edit           # Edit project config
edi config edit --global  # Edit global config

# Validate configuration
edi config validate

# Set a value
edi config set defaults.agent architect
edi config set --global briefing.history_depth 5
```

### 3.4 `edi recall`

RECALL utilities for direct interaction.

**Subcommands**:

```bash
# Search knowledge base
edi recall search "authentication patterns"
edi recall search --type decision "API design"
edi recall search --scope global "error handling"

# Index files
edi recall index docs/adr/
edi recall index --recursive src/

# Check status
edi recall status

# Server management
edi recall server start
edi recall server stop
edi recall server status
```

**Implementation**:
```go
func recallSearchCmd() *cobra.Command {
    var types []string
    var scope string
    var limit int
    
    cmd := &cobra.Command{
        Use:   "search <query>",
        Short: "Search RECALL knowledge base",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := recall.NewClient()
            if err != nil {
                return err
            }
            
            results, err := client.Search(context.Background(), &recall.SearchOptions{
                Query: args[0],
                Types: types,
                Scope: scope,
                Limit: limit,
            })
            if err != nil {
                return err
            }
            
            printSearchResults(results)
            return nil
        },
    }
    
    cmd.Flags().StringSliceVar(&types, "type", nil, "Filter by type")
    cmd.Flags().StringVar(&scope, "scope", "all", "Scope: project, global, all")
    cmd.Flags().IntVar(&limit, "limit", 10, "Max results")
    
    return cmd
}
```

### 3.5 `edi history`

Session history utilities.

**Subcommands**:

```bash
# List recent sessions
edi history list
edi history list --limit 20
edi history list --since 2026-01-01

# Show session details
edi history show 2026-01-24-abc123

# Cleanup old entries
edi history cleanup
edi history cleanup --dry-run
```

### 3.6 `edi agent`

Agent utilities.

**Subcommands**:

```bash
# List available agents
edi agent list

# Show agent details
edi agent show architect

# Validate agent file
edi agent validate .edi/agents/custom.md
```

---

## 4. Claude Code Integration

### 4.1 Launching Claude Code

EDI launches Claude Code with specific flags:

```go
func buildClaudeCodeCommand(sess *Session, briefing *Briefing, cfg *Config, opts StartOptions) *exec.Cmd {
    args := []string{}

    // System prompt (agent + skills + briefing)
    systemPrompt := buildSystemPrompt(sess, briefing, cfg)
    args = append(args, "--system-prompt", systemPrompt)

    // MCP servers
    if !opts.NoRecall {
        recallServer := fmt.Sprintf("recall:%s", cfg.Recall.ServerPath)
        args = append(args, "--mcp-server", recallServer)
    }

    // Skills path
    skillsPath := cfg.ClaudeCode.SkillsPath
    if skillsPath != "" {
        args = append(args, "--skills-path", skillsPath)
    }

    // Allowed tools (from agent config)
    agent, _ := loadAgent(sess.Agent)
    if len(agent.Tools.Required) > 0 {
        tools := append(agent.Tools.Required, agent.Tools.Optional...)
        args = append(args, "--allowedTools", strings.Join(tools, ","))
    }

    // Initial prompt (if provided)
    if opts.InitialPrompt != "" {
        args = append(args, "--prompt", opts.InitialPrompt)
    }

    // Resume (if requested)
    if opts.Resume {
        args = append(args, "--resume")
    }

    return exec.Command("claude", args...)
}
```

### 4.2 Slash Commands in Claude Code

Slash commands are handled by Claude through the injected system prompt. EDI injects instructions for each command:

```go
func buildSlashCommandInstructions() string {
    return `
## EDI Slash Commands

You have access to the following slash commands. When the user types one of these, execute the corresponding action:

### /plan (aliases: /architect, /design)
Switch to architect mode for system design work.
- Load the architect agent configuration
- Query RECALL for architecture context
- Focus on system-level thinking and ADRs

### /build (aliases: /code, /implement)
Switch to coder mode for implementation work.
- Load the coder agent configuration
- Query RECALL for implementation patterns
- Focus on clean, tested code

### /review (alias: /check)
Switch to reviewer mode for code review.
- Load the reviewer agent configuration
- Query RECALL for security and quality patterns
- Focus on finding issues and providing constructive feedback

### /incident (aliases: /debug, /fix)
Switch to incident mode for troubleshooting.
- Load the incident agent configuration
- Query RECALL for runbooks and known issues
- Focus on rapid diagnosis and mitigation

### /end
End the current session.
- Generate a session summary
- Identify capture candidates
- Prompt to save significant items to RECALL
- Save session history

### /task (aliases: /tasks)
Manage task-based workflows with deep RECALL integration.

Without arguments:
- Show current Tasks status (completed, in progress, pending)
- Show dependency graph
- Show annotation summaries (e.g., "2 patterns, 1 failure")
- Do NOT load full RECALL context (lazy loading)

With task-id (e.g., `/task task-004`):
- Load task annotations (stored RECALL context from creation)
- Load inherited context from completed parent tasks
- Check for parallel discoveries in flight recorder
- Present full context and begin execution

With description (e.g., `/task Implement billing with Stripe`):
- Break into tasks with dependencies
- For each task, query RECALL and store annotations
- Log task creation to flight recorder
- Show task graph and ask to proceed

During task execution:
- Log significant decisions to flight recorder
- Mark decisions that should propagate to dependent tasks
- Share discoveries with parallel subagents
- On completion, prompt for RECALL capture

When switching agents, acknowledge the switch and query RECALL for relevant context.
`
}
```

### 4.3 System Prompt Construction

```go
func buildSystemPrompt(sess *Session, briefing *Briefing, cfg *Config) string {
    var sb strings.Builder

    // 1. EDI identity
    sb.WriteString("# EDI - Enhanced Development Intelligence\n\n")
    sb.WriteString("You are operating as EDI, an AI engineering assistant with ")
    sb.WriteString("continuity, knowledge, and specialized behaviors.\n\n")

    // 2. Current agent
    agent, _ := loadAgent(sess.Agent)
    sb.WriteString(fmt.Sprintf("## Current Mode: %s\n\n", agent.Name))
    sb.WriteString(agent.SystemPrompt)
    sb.WriteString("\n\n")

    // 3. Behaviors
    if len(agent.Behaviors) > 0 {
        sb.WriteString("## Key Behaviors\n\n")
        for _, b := range agent.Behaviors {
            sb.WriteString(fmt.Sprintf("- %s\n", b))
        }
        sb.WriteString("\n")
    }

    // 4. Loaded skills
    for _, skillName := range agent.Skills {
        skill, err := loadSkill(skillName)
        if err != nil {
            continue
        }
        sb.WriteString(fmt.Sprintf("---\n\n## Skill: %s\n\n", skillName))
        sb.WriteString(skill.Content)
        sb.WriteString("\n\n")
    }

    // 5. Slash command instructions
    sb.WriteString("---\n\n")
    sb.WriteString(buildSlashCommandInstructions())
    sb.WriteString("\n\n")

    // 6. Briefing (if present)
    if briefing != nil {
        sb.WriteString("---\n\n")
        sb.WriteString("## Session Briefing\n\n")
        sb.WriteString(briefing.Render(cfg.Project.Name))
        sb.WriteString("\n\n")
    }

    // 7. RECALL instructions
    sb.WriteString("---\n\n")
    sb.WriteString("## RECALL Knowledge Base\n\n")
    sb.WriteString("You have access to RECALL, the organizational knowledge base. ")
    sb.WriteString("Use it proactively to:\n")
    sb.WriteString("- Check for existing patterns before implementing\n")
    sb.WriteString("- Look up past decisions (ADRs) for context\n")
    sb.WriteString("- Search for known issues when troubleshooting\n\n")
    sb.WriteString("Available tools: recall_search, recall_get, recall_context, recall_add\n")

    return sb.String()
}
```

### 4.4 Session End Handling

When the user types `/end`, Claude executes the session end workflow:

```go
// Injected as part of /end command handling
const endSessionInstructions = `
When the user types /end:

1. **Generate Summary**
   Summarize what was accomplished in this session:
   - What tasks were completed
   - What decisions were made
   - What's still in progress

2. **Identify Captures**
   Look for significant items worth preserving:
   - Decisions with rationale
   - Patterns that could be reused
   - Issues discovered and resolved
   
3. **Present Capture Prompt**
   Show the user what you'd suggest capturing:
   
   "I identified these items worth capturing to RECALL:
   
   ✓ [Decision] Chose X over Y because...
   ✓ [Pattern] Used approach Z for...
   ? [Observation] Noticed that...
   
   Would you like to save these? [Save All / Edit / Skip]"

4. **Save to RECALL**
   Use recall_add to save approved items.

5. **Save History**
   The session summary will be saved to .edi/history/ automatically.

6. **Confirm End**
   "Session ended. Summary saved to .edi/history/
   Captured 2 items to RECALL.
   
   See you next time!"
`
```

---

## 5. Installation & Setup

### 5.1 Installation Methods

#### Homebrew (macOS/Linux)
```bash
brew tap yourorg/edi
brew install edi
```

#### Direct Download
```bash
curl -fsSL https://edi.example.com/install.sh | bash
```

#### From Source
```bash
git clone https://github.com/yourorg/edi
cd edi
make install
```

### 5.2 Installation Script

```bash
#!/bin/bash
# install.sh - EDI installer

set -e

VERSION="${EDI_VERSION:-latest}"
INSTALL_DIR="${EDI_INSTALL_DIR:-$HOME/.edi/bin}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing EDI $VERSION for $OS/$ARCH..."

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download binary
DOWNLOAD_URL="https://github.com/yourorg/edi/releases/download/$VERSION/edi-$OS-$ARCH"
curl -fsSL "$DOWNLOAD_URL" -o "$INSTALL_DIR/edi"
chmod +x "$INSTALL_DIR/edi"

# Download RECALL MCP server
curl -fsSL "$DOWNLOAD_URL-recall-mcp" -o "$INSTALL_DIR/recall-mcp"
chmod +x "$INSTALL_DIR/recall-mcp"

# Add to PATH (if not already)
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> ~/.bashrc
    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> ~/.zshrc
    echo ""
    echo "Added $INSTALL_DIR to PATH. Restart your shell or run:"
    echo "  source ~/.bashrc  # or ~/.zshrc"
fi

echo ""
echo "✓ EDI installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Initialize EDI:  edi init --global"
echo "  2. Set API keys:"
echo "     export VOYAGE_API_KEY=your-key"
echo "     export OPENAI_API_KEY=your-key"
echo "  3. Initialize a project:  cd your-project && edi init"
echo "  4. Start a session:  edi"
```

### 5.3 First Run Experience

```
$ edi init --global

Initializing EDI...

Creating directories:
  ✓ ~/.edi/agents/
  ✓ ~/.edi/skills/
  ✓ ~/.edi/commands/
  ✓ ~/.edi/recall/
  ✓ ~/.edi/cache/

Installing default agents:
  ✓ architect.md
  ✓ coder.md
  ✓ reviewer.md
  ✓ incident.md

Creating configuration:
  ✓ config.yaml

Checking dependencies:
  ✓ claude (Claude Code CLI found)
  ✓ VOYAGE_API_KEY (set)
  ✓ OPENAI_API_KEY (set)
  ⚠ ANTHROPIC_API_KEY (not set - Stage 3 reranking disabled)

Downloading models:
  ✓ bge-reranker-base-onnx (Stage 1)
  ✓ bge-reranker-v2-m3-onnx (Stage 2)

✓ EDI initialized successfully!

Next steps:
  1. cd to a project directory
  2. Run: edi init
  3. Edit .edi/profile.md to describe your project
  4. Start a session: edi
```

### 5.4 Dependency Checking

```go
// CheckDependencies verifies required dependencies
func CheckDependencies() []DependencyStatus {
    return []DependencyStatus{
        checkClaudeCLI(),
        checkAPIKey("VOYAGE_API_KEY", true),
        checkAPIKey("OPENAI_API_KEY", true),
        checkAPIKey("ANTHROPIC_API_KEY", false), // Optional
        checkONNXRuntime(),
    }
}

func checkClaudeCLI() DependencyStatus {
    _, err := exec.LookPath("claude")
    if err != nil {
        return DependencyStatus{
            Name:     "claude",
            Found:    false,
            Required: true,
            Message:  "Claude Code CLI not found. Install from: https://claude.ai/code",
        }
    }
    
    // Check version
    out, _ := exec.Command("claude", "--version").Output()
    version := strings.TrimSpace(string(out))
    
    return DependencyStatus{
        Name:    "claude",
        Found:   true,
        Version: version,
    }
}
```

### 5.5 Uninstallation

```bash
# Remove EDI
rm -rf ~/.edi
rm -f /usr/local/bin/edi
rm -f /usr/local/bin/recall-mcp

# Or with make
make uninstall
```

---

## 6. Implementation

### 6.1 Package Structure

```
edi/
├── cmd/
│   └── edi/
│       ├── main.go           # Entry point
│       ├── start.go          # edi (start session)
│       ├── init.go           # edi init
│       ├── config.go         # edi config
│       ├── recall.go         # edi recall
│       ├── history.go        # edi history
│       ├── agent.go          # edi agent
│       └── version.go        # edi version
├── internal/
│   ├── cli/
│   │   ├── launcher.go       # Claude Code launcher
│   │   ├── prompt.go         # System prompt builder
│   │   └── install.go        # Installation helpers
│   ├── session/              # (from Session Lifecycle spec)
│   ├── agent/                # (from Agent System spec)
│   ├── config/               # (from Workspace spec)
│   ├── history/              # (from Session Lifecycle spec)
│   ├── briefing/             # (from Session Lifecycle spec)
│   └── capture/              # (from Session Lifecycle spec)
└── pkg/
    └── recall/               # RECALL client
```

### 6.2 Implementation Plan

#### Phase 4.1: CLI Architecture (Week 1)

- [ ] Cobra command structure
- [ ] `edi` (start) command
- [ ] `edi init` command
- [ ] `edi version` command
- [ ] Claude Code launcher
- [ ] Basic system prompt builder

**Exit Criteria**: Can launch Claude Code with EDI configuration.

#### Phase 4.2: Command Specifications (Week 2)

- [ ] `edi config` subcommands
- [ ] `edi recall` subcommands
- [ ] `edi history` subcommands
- [ ] `edi agent` subcommands
- [ ] Slash command injection

**Exit Criteria**: All commands implemented and functional.

#### Phase 4.3: Installation & Setup (Week 3)

- [ ] Installation script
- [ ] Homebrew formula
- [ ] First-run experience
- [ ] Dependency checking
- [ ] Model downloading
- [ ] Documentation

**Exit Criteria**: Can install EDI from scratch and run first session.

### 6.3 Validation Criteria

| Metric | Target |
|--------|--------|
| `edi` startup time | < 500ms (to Claude Code launch) |
| `edi init --global` | < 30s (including model download) |
| `edi init` (project) | < 1s |
| Binary size | < 20MB |
| Installation time | < 2 min |

---

## Appendix A: Command Quick Reference

### Shell Commands

| Command | Description |
|---------|-------------|
| `edi` | Start EDI session |
| `edi init` | Initialize project |
| `edi init --global` | Initialize global EDI |
| `edi sync` | Sync assets to ~/.edi and ~/.claude |
| `edi config show` | Show configuration |
| `edi config edit` | Edit configuration |
| `edi recall search` | Search RECALL |
| `edi recall index` | Index files |
| `edi history list` | List sessions |
| `edi agent list` | List agents |
| `edi ralph` | (future) Run Ralph loop — currently standalone via `~/.edi/ralph/ralph.sh` |

### Slash Commands (in Claude Code)

| Command | Action |
|---------|--------|
| `/plan` | Switch to architect agent |
| `/build` | Switch to coder agent |
| `/review` | Switch to reviewer agent |
| `/incident` | Switch to incident agent |
| `/end` | End session |

---

## Appendix B: Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `VOYAGE_API_KEY` | Voyage AI embeddings | Yes |
| `OPENAI_API_KEY` | OpenAI embeddings | Yes |
| `ANTHROPIC_API_KEY` | Stage 3 reranking | No |
| `EDI_HOME` | Override ~/.edi | No |
| `EDI_CONFIG` | Override config path | No |
| `EDI_DEBUG` | Enable debug logging | No |

---

## Appendix C: Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Dependency missing |
| 4 | RECALL server error |
| 5 | Claude Code launch failed |

---

## Appendix D: Related Specifications

| Spec | Relationship |
|------|--------------|
| Workspace & Configuration | Paths, config loading |
| Session Lifecycle | Session management, briefing, capture |
| Agent System | Agent loading, switching |
| RECALL MCP Server | MCP integration, search |

---

## Appendix E: Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Jan 25, 2026 | Go for CLI | Single binary, fast startup, matches RECALL |
| Jan 25, 2026 | Cobra for commands | Standard Go CLI framework |
| Jan 25, 2026 | Slash commands via injection | Works within Claude Code, no custom parsing |
| Jan 25, 2026 | Installation via shell script | Cross-platform, simple |
| Jan 25, 2026 | Models downloaded on init | Better first-run UX than runtime download |
