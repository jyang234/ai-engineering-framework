# EDI - Enhanced Development Intelligence

EDI is a harness for Claude Code that provides continuity, knowledge, and specialized behaviors.

## Overview

EDI configures Claude Code with:
- **Agents** - Specialized modes for coding, architecture, review, and incident response
- **RECALL** - Knowledge retrieval via MCP for patterns, failures, and decisions
- **Briefings** - Context from previous sessions and project profile
- **History** - Session summaries for continuity between sessions

## Installation

### Build from Source

```bash
cd edi
make build
make install  # Installs to ~/.local/bin/
```

### Requirements

- Go 1.22+
- Claude Code CLI installed and in PATH

## Quick Start

```bash
# Initialize global EDI (once per machine)
edi init --global

# Initialize EDI in a project
cd your-project
edi init

# Edit project profile
$EDITOR .edi/profile.md

# Start an EDI session
edi
```

## Commands

### Shell Commands

| Command | Description |
|---------|-------------|
| `edi` | Start EDI session (launches Claude Code) |
| `edi init` | Initialize EDI in current project |
| `edi init --global` | Initialize global EDI at ~/.edi/ |
| `edi version` | Show version information |

### Slash Commands (in Claude Code)

| Command | Description |
|---------|-------------|
| `/plan` | Switch to architect mode |
| `/build` | Switch to coder mode |
| `/review` | Switch to reviewer mode |
| `/incident` | Switch to incident mode |
| `/task` | Manage tasks with RECALL context |
| `/end` | End session and save history |

## Agents

EDI includes four core agents:

- **Coder** - Implementation focused, clean tested code
- **Architect** - System design, trade-offs, ADRs
- **Reviewer** - Code review, security, quality
- **Incident** - Debugging, rapid resolution

## RECALL

RECALL is the knowledge retrieval system, available via MCP tools:

- `recall_search` - Search patterns, failures, decisions
- `recall_get` - Retrieve item by ID
- `recall_add` - Add new knowledge
- `recall_feedback` - Provide usefulness feedback
- `flight_recorder_log` - Log session events

## Directory Structure

### Global (~/.edi/)

```
~/.edi/
├── agents/      # Agent definitions
├── commands/    # Slash commands
├── skills/      # Skills
├── recall/      # Knowledge database
├── cache/       # Temporary files
└── config.yaml  # Global configuration
```

### Project (.edi/)

```
.edi/
├── config.yaml  # Project configuration
├── profile.md   # Project description
├── history/     # Session history
├── tasks/       # Task annotations
└── recall/      # Project knowledge
```

## Configuration

### Global Config (~/.edi/config.yaml)

```yaml
version: "1"
agent: coder

recall:
  enabled: true

briefing:
  include_history: true
  history_entries: 3
  include_tasks: true
  include_profile: true

capture:
  friction_budget: 3

tasks:
  lazy_loading: true
  capture_on_completion: true
  propagate_decisions: true
```

### Project Config (.edi/config.yaml)

```yaml
version: "1"

project:
  name: my-project

# Override global settings as needed
```

## License

MIT
