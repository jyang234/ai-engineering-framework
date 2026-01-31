# AEF + EDI: FAQ and Pitch Notes

> **Implementation Status (January 31, 2026):** EDI/Codex pitch is accurate. References to VERIFY, Sandbox, enterprise governance are aspirational — these components do not exist.

**Purpose**: Common questions, honest answers, and positioning for the Agentic Engineering Framework (AEF) and EDI (Enhanced Development Intelligence).

**Last Updated**: January 25, 2026
**Version**: 3.0

---

## What is AEF vs EDI?

| Layer | Name | Purpose |
|-------|------|---------|
| **User Experience** | EDI (Enhanced Development Intelligence) | Your AI chief of staff — continuity, briefings, specialized agents |
| **Infrastructure** | AEF (Agentic Engineering Framework) | Knowledge retrieval (Codex/RECALL), quality gates (VERIFY), experimentation (Sandbox) |

**Simple version**: EDI is what you interact with. AEF powers it.

**Simpler version**: EDI is Claude Code with memory.

---

## The Pitch

### One-Liner

> "An AI chief of staff that actually remembers your projects, knows your codebase, and picks up where you left off."

### The Problem

**Claude Code is brilliant. But it starts fresh every session.**

```
Monday:
  You: "Let's design the payment service"
  Claude: [great 2-hour design session, decisions made, context built]
  
Tuesday:
  Claude: "Hello! How can I help you today?"
  You: "Continue the payment service work"
  Claude: "What payment service? Can you give me some context?"
```

Every. Single. Time.

This is not a bug — it is how LLMs work. They do not remember. They do not learn. Every conversation starts from zero.

**The result:**
- You rebuild context constantly
- You re-explain decisions you already made
- You lose the reasoning behind past choices
- Cross-project knowledge stays in your head, not the system
- After a vacation, you are starting over

### The Solution

**EDI gives Claude Code what it lacks:**

| Gap | EDI Solution |
|-----|--------------|
| Session memory | **History + Flight Recorder** — what happened, why, persisted locally |
| Knowledge base | **RECALL** — semantic search over your docs, patterns, decisions |
| Proactive context | **Briefings** — EDI tells Claude what it needs to know at session start |
| Consistent behaviors | **Agents** — specialized modes (architect, coder, reviewer, incident) |
| Audit trail | **Flight Recorder** — significant events logged for continuity |

**The experience:**

```
Tuesday:
  $ edi
  
  EDI: "Good morning. I have prepared your briefing.
  
        Yesterday we designed the payment service. Key decisions:
        - Event-driven architecture (ADR-042)
        - Stripe integration over Paddle (cost analysis in RECALL)
        
        Open question: currency conversion edge cases.
        
        The payments ADR was merged overnight.
        
        How would you like to proceed?"
```

That is the difference.

---

## How EDI Works

### The Harness Model

EDI is a **harness** for Claude Code, not a replacement. It configures Claude with context and knowledge, then launches it.

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│              │      │              │      │              │
│  $ edi       │ ───▶ │  Configure   │ ───▶ │  $ claude    │
│              │      │  & Launch    │      │  (native)    │
│              │      │              │      │              │
└──────────────┘      └──────────────┘      └──────────────┘
                            │
                      EDI exits here
```

**What EDI does before launching Claude Code:**
1. Load configuration and agent
2. Query RECALL for relevant knowledge
3. Read recent history and flight recorder
4. Generate briefing
5. Inject context via `--append-system-prompt-file`
6. Launch Claude Code and exit

**What Claude Code does (natively):**
- All the actual work
- Tool use, file editing, command execution
- Task tracking via Claude Code Tasks
- RECALL queries via MCP
- Flight recorder logging via MCP

EDI is setup and scaffolding. Claude Code is execution.

### The EDI Persona

EDI is inspired by the AI character from Mass Effect — an artificial intelligence that evolved from a constrained system into a trusted collaborator.

**Key characteristics:**
- Does not use contractions ("I am" not "I'm")
- Formal, precise tone — measured, not effusive
- Genuine investment in your success, expressed with restraint
- Occasional deadpan humor about AI tropes, followed by disclaimer

```
"I have now memorized all of your authentication patterns."
[pause]
"For debugging purposes only, naturally."
```

This is not cosmetic — a consistent voice builds trust and makes sessions feel continuous.

---

## Frequently Asked Questions

### "I thought Claude Code could do everything?"

**What Claude Code Does Well (January 2026)**

| Capability | Claude Code |
|------------|-------------|
| Write code | ✅ Excellent |
| Read your files | ✅ Yes |
| Run commands | ✅ Yes |
| Follow CLAUDE.md | ✅ Yes |
| Use tools (MCP) | ✅ Yes |
| Task tracking | ✅ Yes (Tasks feature) |
| Spawn subagents | ✅ Yes |
| Cross-session tasks | ✅ Yes |

**Claude Code is genuinely great.** We build ON it, not around it.

**What Claude Code Does Not Have**

| Gap | Impact | EDI Solution |
|-----|--------|--------------|
| **Session memory** | Context rebuilding every time | History + flight recorder + briefings |
| **Semantic search** | Cannot find relevant past decisions | RECALL with embeddings + reranking |
| **Cross-project knowledge** | Each project is isolated | Global + project scopes in RECALL |
| **Specialized behaviors** | Same Claude for all tasks | Agent modes |
| **Proactive context** | You must provide all context | Briefings injected at session start |

**The Core Problem**

Claude Code starts fresh every session. Tasks track *state* (what is in progress), but not *reasoning* (why you made decisions, what you tried, what you learned).

EDI adds the reasoning layer.

---

### "Does a solo developer actually need this?"

**Yes.** This was our biggest realization.

We originally thought EDI was for teams. But the core problem — Claude forgetting everything — hurts everyone equally.

**Solo Developer Pain Points**

| Pain | Frequency | Impact |
|------|-----------|--------|
| "What was I working on in this project?" | Daily | 10-15 min context rebuild |
| "Why did I structure it this way?" | Weekly | Re-litigating past decisions |
| "I solved this before in another project" | Weekly | Reinventing solutions |
| "Where did I leave off after the weekend?" | Weekly | Lost momentum |

**These are real problems at scale = 1.**

**The Continuity Problem is Scale-Agnostic**

A solo developer with 3 projects and a 2-week vacation loses just as much context as a team member. EDI fixes that.

---

### "How is this different from MARVIN?"

MARVIN (bb-chiefofstaff) is a similar project with the same insight: Claude Code needs memory.

| Aspect | MARVIN | EDI |
|--------|--------|-----|
| **Approach** | Workspace + CLAUDE.md | Harness + context injection |
| **Search** | File reading, grep | Semantic search + reranking |
| **Knowledge types** | Flat markdown files | Typed items (decisions, patterns, failures) |
| **Scope** | Single workspace | Project → domain → global |
| **Goal tracking** | Built-in | Defers to Claude Code Tasks |

**When to use which:**
- **MARVIN**: Quick setup, personal productivity, simpler needs
- **EDI**: Semantic search matters, multiple projects, organizational knowledge

They solve the same problem differently. MARVIN is lighter. EDI has better retrieval.

---

### "Is this overengineered?"

**We worried about this. So we simplified aggressively.**

| Removed | Why |
|---------|-----|
| CAL orchestration layer | Claude decides when to use tools |
| Custom task tracking | Claude Code Tasks exists |
| Custom subagent system | Claude Code subagents exist — we define EDI-aware ones |
| Daemon mode | No capability gain; complexity cost |
| Full transcript capture | Claude self-reports significant events |

**What remains:**

| Component | Complexity | Purpose |
|-----------|------------|---------|
| RECALL MCP server | Medium | Semantic search (the core value) |
| EDI CLI | Low | Setup + launch |
| Agent prompts | Low | Markdown files |
| Subagent definitions | Low | Markdown files with RECALL + flight recorder |
| Slash commands | Low | Markdown files |
| Flight recorder | Low | Append to JSONL |

**The only complex piece is RECALL** — and that complexity is justified because naive retrieval fails ~70% of the time on real queries.

---

### "How does EDI work with Claude Code Tasks?"

**EDI deeply integrates with Tasks, not just enriches them.**

Claude Code Tasks (released January 2026) provides persistent, dependency-aware task management. EDI adds five key integrations:

| Principle | What It Means |
|-----------|---------------|
| **Annotate once, use many** | RECALL queried at task creation, stored with task |
| **Dependency context flows** | Decisions from parent tasks propagate to children |
| **Lazy loading** | RECALL loads on task pickup, not session start |
| **Per-task capture** | Capture prompts when task completes |
| **Parallel awareness** | Concurrent subagents share discoveries |

**The workflow:**

```
Task Created:
├── Query RECALL once
├── Store annotations WITH the task (.edi/tasks/task-004.yaml)
└── Includes: patterns, failures, decisions

Task Dependencies Complete:
├── Inherit decisions from parent tasks
│   e.g., "Use Stripe" from task-002 flows to task-004
└── Load inherited context automatically

Task Execution:
├── Load stored annotations (no re-query!)
├── Log decisions to flight recorder
├── Mark which decisions should propagate
└── Share discoveries with parallel subagents

Task Complete:
├── Propagate decisions to dependent tasks
├── Prompt: "Capture to RECALL?"
└── Update task annotations with execution context
```

**Token savings:** Instead of O(tasks × sessions) RECALL queries, we do O(tasks) — once per task.

EDI doesn't just make Tasks "RECALL-aware." It makes them **context-flowing, capture-integrated, and coordination-capable**.

---

### "What about subagents?"

**We define EDI-aware subagents that leverage Claude Code's native system.**

When EDI's main agents spawn subagents for subtasks, those subagents need:
- Access to RECALL for organizational knowledge
- Flight recorder for audit trail
- EDI persona for consistent voice
- Project patterns for consistent output

**Our solution:**

| Subagent | Purpose |
|----------|---------|
| **edi-researcher** | Deep RECALL search and codebase exploration |
| **edi-web-researcher** | External research (docs, best practices, security advisories) |
| **edi-implementer** | Code implementation with pattern awareness |
| **edi-test-writer** | Test generation following project conventions |
| **edi-doc-writer** | Documentation with consistent style |
| **edi-reviewer** | Code review with failure pattern awareness |
| **edi-debugger** | Root cause analysis with historical context |

Each subagent:
- Auto-loads the `edi-core` skill (persona, RECALL patterns, flight recorder guidance)
- Has explicit access to RECALL MCP tools
- Returns synthesized summaries (not raw context)
- Logs significant decisions to flight recorder

This is not a custom subagent system — it is subagent definitions that make Claude Code's native subagents EDI-aware.

---

### "What about learning? Does the AI get smarter?"

**No. AI does not learn at inference time. Let us be precise.**

| Misconception | Reality |
|---------------|---------|
| "AI learns from corrections" | LLMs do not learn at inference time |
| "AI remembers past sessions" | Every session starts with zero memory |
| "AI improves with use" | Model weights are frozen |

**What ACTUALLY improves:**

| Thing | How | Who |
|-------|-----|-----|
| Knowledge base | More items indexed | Human curation |
| Retrieval quality | Better embeddings, reranking | Engineering |
| Behavioral guidance | Skills refined | Human authoring |

**The "learning loop" is human-powered:**

```
Session happens
      ↓
/end prompts capture candidates
      ↓
HUMAN approves what to save
      ↓
Knowledge base grows
      ↓
Future sessions have more context
```

EDI makes it easy to capture. YOU decide what is worth keeping. That is intentional.

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           You (Engineer)                                 │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
                              $ edi [--agent architect]
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│                              EDI CLI                                     │
│                                                                          │
│   Pre-launch:                                                           │
│   • Load config + agent                                                 │
│   • Query RECALL for relevant context                                   │
│   • Read recent history + flight recorder                               │
│   • Generate briefing                                                   │
│   • Inject via --append-system-prompt-file                              │
│   • Launch Claude Code                                                  │
│   • Exit                                                                │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼─────────────────────────────────────┐
│                           Claude Code (native)                           │
│                                                                          │
│   With EDI context:                                                     │
│   • EDI persona + agent prompt                                          │
│   • Briefing (history, tasks, RECALL results)                           │
│   • RECALL MCP tools available                                          │
│   • Slash commands: /plan, /build, /review, /incident, /end             │
│                                                                          │
│   Claude works normally, using:                                         │
│   • recall_search, recall_add (knowledge)                               │
│   • flight_recorder_log (significant events)                            │
│   • /end workflow (capture + history)                                   │
└───────────────────────────────────┬─────────────────────────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
              ▼                     ▼                     ▼
┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│       RECALL        │ │   Flight Recorder   │ │       History       │
│    (MCP Server)     │ │      (Local)        │ │      (Local)        │
│                     │ │                     │ │                     │
│ • Semantic search   │ │ • events.jsonl      │ │ • Session summaries │
│ • Hybrid retrieval  │ │ • 30-day retention  │ │ • Decisions + why   │
│ • Multi-stage       │ │ • Feeds briefings   │ │ • Indefinite        │
│   reranking         │ │                     │ │                     │
└─────────────────────┘ └─────────────────────┘ └─────────────────────┘
```

---

## Bootstrap Strategy

We are building EDI to build Codex, then upgrading EDI with Codex.

### Phase 0: EDI v0 (Stub RECALL)

Build minimal EDI with simple full-text search:

| Component | Implementation |
|-----------|----------------|
| Workspace | Full `.edi/` structure |
| Briefings | Read flat files, format markdown |
| Agents | All 4 prompts |
| Commands | All slash commands |
| Flight recorder | JSONL append |
| **RECALL stub** | SQLite FTS (no embeddings) |
| CLI | `edi` launches Claude Code |

**Exit criteria**: Can run `edi` and get useful session continuity.

### Phase 1: Build Codex (Using EDI v0)

Full retrieval system, built with EDI assistance:

| Component | Description |
|-----------|-------------|
| Embedding pipeline | Voyage Code-3 |
| Hybrid search | BM25 + vector |
| Multi-stage reranking | BGE models via ONNX |
| AST-aware chunking | Tree-sitter |
| Contextual retrieval | Claude Haiku summaries |

EDI captures decisions, patterns, and failures along the way.

**Exit criteria**: Codex passes retrieval accuracy benchmarks.

### Phase 2: EDI v1 (Full RECALL)

Upgrade RECALL stub to use Codex:

- Swap SQLite FTS for Codex retrieval
- All knowledge captured during build is now semantically searchable
- EDI becomes self-improving

**Exit criteria**: RECALL uses production Codex.

---

## Honest Assessment

### What EDI Actually Provides

| Component | Value | Complexity |
|-----------|-------|------------|
| **RECALL** | High — Claude Code cannot do semantic search | High |
| **Briefings** | Medium — proactive context is useful | Low |
| **Agents** | Low-Medium — prompt organization | Low |
| **Flight recorder** | Low — depends on Claude compliance | Low |
| **Persona** | Cosmetic — consistency is nice | Negligible |

**RECALL is the only thing Claude Code genuinely cannot do.** The rest is convenience.

### When EDI is Worth It

✅ **Use EDI if:**
- You work on projects longer than a week
- You have multiple projects
- You take breaks and lose context
- You want to remember why you made decisions
- You have substantial knowledge worth searching

❌ **Skip EDI if:**
- Single simple project, done in a week
- Throwaway prototype
- Learning exercise you will not revisit
- You do not have enough history to search yet

### The Timing Question

EDI is valuable when you have knowledge worth retrieving. If you are starting fresh, RECALL has nothing to search.

**Our approach**: Build EDI v0 with stub RECALL. Use it to build Codex. Accumulate knowledge along the way. Upgrade RECALL when Codex is ready.

---

## Glossary

| Term | Definition |
|------|------------|
| **AEF** | Agentic Engineering Framework — infrastructure layer |
| **EDI** | Enhanced Development Intelligence — user experience layer |
| **Codex** | Core retrieval library (embeddings, search, reranking) |
| **RECALL** | MCP server interface to Codex |
| **VERIFY** | CI/CD quality gates for AI output |
| **Sandbox** | Deterministic execution environment for experiments |
| **Agent** | Specialized EDI mode (architect, coder, reviewer, incident) |
| **Flight Recorder** | Local event capture during sessions (30-day retention) |
| **History** | Curated session summaries (indefinite retention) |
| **Briefing** | Proactive context summary at session start |
| **Capture** | Prompted knowledge preservation at session end |

---

## Change Log

| Version | Date | Changes |
|---------|------|---------|
| 3.0 | January 25, 2026 | Harness model, flight recorder, EDI persona, bootstrap strategy, honest assessment |
| 2.0 | January 2026 | EDI framing, Claude Code Tasks integration, simplified architecture |
| 1.0 | December 2025 | Original version with CAL/Harness architecture |
