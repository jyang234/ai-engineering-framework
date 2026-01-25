# Agentic Engineering Framework (AEF) â€” Architecture Specification

**Status**: Active Development  
**Last Updated**: January 2025  
**Version**: 0.5

> **v0.5 Update**: Architecture revised to build on Claude Code's native Tasks, Skills, and Subagents primitives (announced Jan 22, 2026). AEF now focuses on the knowledge retrieval, quality assurance, and contribution management layers that Claude Code doesn't provide.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Goals & Success Criteria](#2-goals--success-criteria)
3. [Architecture Overview](#3-architecture-overview)
4. [Core Services](#4-core-services)
5. [Component Specification Status](#5-component-specification-status)
6. [Deep Dive Areas](#6-deep-dive-areas)
7. [Secondary Use Cases](#7-secondary-use-cases)
8. [Implementation Phases](#8-implementation-phases)
9. [Open Questions](#9-open-questions)
10. [Decision Log](#10-decision-log)

---

## 1. Executive Summary

The Agentic Engineering Framework (AEF) is a **knowledge and quality layer** built on top of Claude Code's native agentic primitives. Rather than replacing Claude Code's task coordination, subagent orchestration, and session management, AEF extends these capabilities with institutional knowledge retrieval, systematic quality assurance, and organizational learning.

### Core Design Philosophy

Drawing from the Context Assembly Layer (CAL) paradigm (arXiv:2512.05470v1), AEF treats "everything as context" — heterogeneous sources (memory, tools, knowledge, human input) are mounted and accessed through a uniform namespace, enabling systematic context engineering rather than ad-hoc prompt construction.

**Key Insight (v0.5)**: Claude Code's Tasks feature (Jan 2026) provides native support for task dependencies, cross-session collaboration, and multi-subagent coordination. AEF builds on these primitives rather than replacing them.

### Architectural Positioning

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Claude Code Native Primitives                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   Tasks     │  │   Skills    │  │  Subagents  │  │    Sessions     │ │
│  │(coordination)│  │ (behaviors)│  │(delegation) │  │  (persistence)  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────────────┐
│                         AEF Value Layer                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │     Codex       │  │   Evaluation    │  │   Contribution          │  │
│  │  (retrieval)    │  │  (self-correct) │  │   (knowledge loop)      │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │
│                                                                          │
│  Integration via Claude Code Hooks:                                      │
│  • PreTaskStart  → Query Codex for relevant context                      │
│  • PostTaskComplete → Run evaluation, prompt for contribution            │
│  • OnTaskFail → Log to audit trail, trigger self-correct                 │
└──────────────────────────────────────────────────────────────────────────┘
```

### What Claude Code Provides (Use, Don't Rebuild)

| Capability | Claude Code Feature | AEF Approach |
|------------|---------------------|--------------|
| Task tracking | Tasks with dependencies | Use native Tasks |
| Cross-session state | Task broadcast to all sessions | Use `CLAUDE_CODE_TASK_LIST_ID` |
| Subagent delegation | Native subagents | Use native subagents with AEF Skills |
| Behavioral customization | Skills (SKILL.md) | Implement Org DNA as Skills |
| Session persistence | `--continue`, `--resume` | Use native sessions |

### What AEF Provides (Unique Value)

| Capability | AEF Component | Why Needed |
|------------|---------------|------------|
| Knowledge retrieval | **Codex** | Claude Code has no institutional knowledge base |
| Context assembly | **CAL** | Retrieval orchestration, budget management |
| Quality assurance | **Evaluation + Self-Correct** | No automated quality gates in Claude Code |
| Knowledge loop | **Contribution Manager** | Artifacts don't flow back into knowledge base |
| Decision history | **Audit Trail** | Tasks are state, not decision rationale |
| Organizational alignment | **Org DNA** (as Skills) | Behavioral consistency at scale |

### Primary Services (Revised)

| Service | Purpose | Relation to Claude Code |
|---------|---------|-------------------------|
| **Codex** | Knowledge base with hybrid search and multi-stage reranking | Standalone; no Claude Code equivalent |
| **CAL** | Retrieval orchestration, context assembly, budget management | Hooks into Tasks via PreTaskStart |
| **Evaluation & Self-Correct** | Automated quality gates with retry loop | Hooks into Tasks via PostTaskComplete |
| **Contribution Manager** | Knowledge loop; artifact → Codex flow | Hooks on significant task outcomes |
| **Audit Trail** | Decision history, failure tracking | Captures reasoning Tasks don't store |
| **Org DNA Store** | Behavioral context: examples, guidance, rubrics | Implemented as Claude Code Skills |
| **Experiment Service** | Async prototype testing and bake-offs | Standalone; complements Tasks |

### Deprecated/Absorbed Components

| Previous Component | Status | Rationale |
|--------------------|--------|-----------|
| **Claude Harness** | → Absorbed | Claude Code's native subagents + Tasks replace orchestration |
| **Working Memory** | → Absorbed | Claude Code's Tasks + Sessions provide persistence |
| **Subagent Model** | → Absorbed | Use Claude Code's native subagent delegation |

### Supporting Systems

| System | Purpose |
|--------|---------|
| **Governance & Safety Layer** | Agent registry, behavioral guardrails, safety filters, cost control |
| **Prompt Management System** | Prompt versioning, composition, A/B testing, organizational values |
| **Human-in-the-Loop Engine** | Approval gates, escalation routing, feedback collection |

---

## 2. Goals & Success Criteria

### Primary Goals

**Goal 1: Scalability (Personal â†’ Enterprise)**
- Single developer can use the framework for personal projects
- Teams can share context, prompts, and evaluations
- Enterprise can enforce governance, compliance, and cost controls

**Goal 2: Organizational Alignment**
- Agents behave consistently with team/org values
- Outputs reflect coding standards, communication norms, decision principles
- New team members (human or AI) can onboard via the same knowledge base

**Goal 3: Consistency & Quality**
- Repeatable outputs for similar inputs
- Measurable quality metrics with regression detection
- Continuous improvement based on feedback

### Success Criteria

| Criterion | Measurement | Target |
|-----------|-------------|--------|
| Context grounding | % of outputs traceable to Codex sources | >90% |
| Quality consistency | Variance in eval scores across runs | <10% |
| Organizational compliance | % of outputs passing standards checks | >95% |
| Human override rate | % requiring human correction | <15% |
| Cost predictability | Actual vs budgeted token usage | Â±20% |

---

## 3. Architecture Overview

### High-Level Component Diagram

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                      AGENTIC ENGINEERING FRAMEWORK (AEF)                     â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚                     GOVERNANCE & SAFETY LAYER                          â”‚ â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚ â”‚
â”‚  â”‚  â”‚ Agent        â”‚ â”‚ Behavioral   â”‚ â”‚ Safety       â”‚ â”‚ Cost          â”‚ â”‚ â”‚
â”‚  â”‚  â”‚ Registry     â”‚ â”‚ Guardrails   â”‚ â”‚ Filters      â”‚ â”‚ Controller    â”‚ â”‚ â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                       â”‚                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚                     PROMPT MANAGEMENT SYSTEM                           â”‚ â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚ â”‚
â”‚  â”‚  â”‚ Prompt       â”‚ â”‚ Prompt       â”‚ â”‚ A/B Testing  â”‚ â”‚ Org DNA       â”‚ â”‚ â”‚
â”‚  â”‚  â”‚ Registry     â”‚ â”‚ Composer     â”‚ â”‚ Engine       â”‚ â”‚ Store         â”‚ â”‚ â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                       â”‚                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚                     CONTEXT ASSEMBLY LAYER (CAL)                       â”‚ â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚ â”‚
â”‚  â”‚  â”‚ Context      â”‚ â”‚ Context      â”‚ â”‚ Context      â”‚ â”‚ Agentic       â”‚ â”‚ â”‚
â”‚  â”‚  â”‚ Constructor  â”‚ â”‚ Updater      â”‚ â”‚ Evaluator    â”‚ â”‚ File System   â”‚ â”‚ â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                       â”‚                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”  â”‚
â”‚  â”‚    CODEX     â”‚  â”‚   CLAUDECODE HARNESS             â”‚  â”‚ OBSERVABILITY â”‚  â”‚
â”‚  â”‚  (Task       â”‚â—„â”€â”¤   Execution Environment          â”‚â”€â”€â–ºâ”‚    Layer      â”‚  â”‚
â”‚  â”‚  Knowledge)  â”‚  â”‚                                  â”‚  â”‚               â”‚  â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜  â”‚
â”‚                                       â”‚                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚                   HUMAN-IN-THE-LOOP ENGINE                             â”‚ â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚ â”‚
â”‚  â”‚  â”‚ Approval     â”‚ â”‚ Escalation   â”‚ â”‚ Collaborativeâ”‚ â”‚ Feedback      â”‚ â”‚ â”‚
â”‚  â”‚  â”‚ Gates        â”‚ â”‚ Router       â”‚ â”‚ Editor       â”‚ â”‚ Collector     â”‚ â”‚ â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                       â”‚                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚                EVALUATION & IMPROVEMENT LOOP                           â”‚ â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚ â”‚
â”‚  â”‚  â”‚ Eval         â”‚ â”‚ Regression   â”‚ â”‚ Feedback     â”‚ â”‚ Quality       â”‚ â”‚ â”‚
â”‚  â”‚  â”‚ Datasets     â”‚ â”‚ Test Runner  â”‚ â”‚ Analyzer     â”‚ â”‚ Trending      â”‚ â”‚ â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜

KEY SEPARATION:
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚  Org DNA Store (in Prompt Management)  â”‚  Codex (queried by CAL)            â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€  â”‚
â”‚  â€¢ Behavioral context                  â”‚  â€¢ Task knowledge                   â”‚
â”‚  â€¢ "How should I behave?"              â”‚  â€¢ "What do I need to know?"        â”‚
â”‚  â€¢ Metadata filtering by task type     â”‚  â€¢ Semantic search by content       â”‚
â”‚  â€¢ Examples, guidance, rubrics         â”‚  â€¢ Code, architecture, incidents    â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

### Data Flow

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                           REQUEST FLOW                                       â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  User Request                                                                â”‚
â”‚       â”‚                                                                      â”‚
â”‚       â–¼                                                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                                                            â”‚
â”‚  â”‚ Governance  â”‚ â”€â”€â”€ Policy checks, rate limits, cost control               â”‚
â”‚  â”‚   Check     â”‚                                                            â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜                                                            â”‚
â”‚       â”‚                                                                      â”‚
â”‚       â–¼                                                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”    â”‚
â”‚  â”‚                    PROMPT COMPOSER (Coordinator)                     â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  1. Measure required content (PR diff, files, etc.)                 â”‚    â”‚
â”‚  â”‚  2. Hard fail if exceeds model limit                                â”‚    â”‚
â”‚  â”‚  3. Calculate budgets:                                              â”‚    â”‚
â”‚  â”‚     â€¢ enhancement_budget = model_limit - required - fixed_overhead  â”‚    â”‚
â”‚  â”‚     â€¢ cal_budget = enhancement_budget - persona - org_dna - instr   â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  4. Parallel retrieval:                                             â”‚    â”‚
â”‚  â”‚     â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”    â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”              â”‚    â”‚
â”‚  â”‚     â”‚   ORG DNA STORE     â”‚    â”‚        CAL          â”‚              â”‚    â”‚
â”‚  â”‚     â”‚   (budget: 800)     â”‚    â”‚  (budget: cal_budget)â”‚              â”‚    â”‚
â”‚  â”‚     â”‚                     â”‚    â”‚                      â”‚              â”‚    â”‚
â”‚  â”‚     â”‚ â€¢ Examples          â”‚    â”‚ â€¢ Auto-detect stack  â”‚              â”‚    â”‚
â”‚  â”‚     â”‚ â€¢ Guidance          â”‚    â”‚ â€¢ Query Entity Graph â”‚              â”‚    â”‚
â”‚  â”‚     â”‚ â€¢ Rubrics           â”‚    â”‚ â€¢ Retrieve from Codexâ”‚              â”‚    â”‚
â”‚  â”‚     â”‚                     â”‚    â”‚ â€¢ Smart curation     â”‚              â”‚    â”‚
â”‚  â”‚     â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜    â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜              â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  5. Assemble prompt in order:                                       â”‚    â”‚
â”‚  â”‚     [System] â†’ [Persona] â†’ [Org DNA] â†’ [Context] â†’ [Task] â†’ [Guard] â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  6. Return composed prompt + metrics                                â”‚    â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜    â”‚
â”‚       â”‚                                                                      â”‚
â”‚       â–¼                                                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”    â”‚
â”‚  â”‚                    CLAUDECODE HARNESS                                â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  â€¢ Execute LLM call                                                 â”‚    â”‚
â”‚  â”‚  â€¢ (If complex task: decompose â†’ multiple Prompt Composer calls)    â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜    â”‚
â”‚       â”‚                                                                      â”‚
â”‚       â–¼                                                                      â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”    â”‚
â”‚  â”‚                    EVALUATION + SELF-CORRECT LOOP                    â”‚    â”‚
â”‚  â”‚                                                                      â”‚    â”‚
â”‚  â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                                                    â”‚    â”‚
â”‚  â”‚  â”‚  Evaluator  â”‚                                                    â”‚    â”‚
â”‚  â”‚  â”‚             â”‚                                                    â”‚    â”‚
â”‚  â”‚  â”‚ â€¢ Tests     â”‚        â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”   â”‚    â”‚
â”‚  â”‚  â”‚ â€¢ Linter    â”‚â”€â”€â”€â”€â”€â”€â”€â–ºâ”‚            PASS?                     â”‚   â”‚    â”‚
â”‚  â”‚  â”‚ â€¢ SAST      â”‚        â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜   â”‚    â”‚
â”‚  â”‚  â”‚ â€¢ Grounding â”‚                    â”‚                              â”‚    â”‚
â”‚  â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜           â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”´â”€â”€â”€â”€â”€â”€â”€â”€â”                     â”‚    â”‚
â”‚  â”‚                            â”‚                 â”‚                      â”‚    â”‚
â”‚  â”‚                           YES                NO                     â”‚    â”‚
â”‚  â”‚                            â”‚                 â”‚                      â”‚    â”‚
â”‚  â”‚                            â–¼                 â–¼                      â”‚    â”‚
â”‚  â”‚                    â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”   â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”       â”‚    â”‚
â”‚  â”‚                    â”‚   Output    â”‚   â”‚  Attempts < 3?      â”‚       â”‚    â”‚
â”‚  â”‚                    â”‚   Ready     â”‚   â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜       â”‚    â”‚
â”‚  â”‚                    â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜           â”‚                      â”‚    â”‚
â”‚  â”‚                                       â”Œâ”€â”€â”€â”€â”€â”€â”´â”€â”€â”€â”€â”€â”€â”               â”‚    â”‚
â”‚  â”‚                                      YES            NO              â”‚    â”‚
â”‚  â”‚                                       â”‚              â”‚               â”‚    â”‚
â”‚  â”‚                                       â–¼              â–¼               â”‚    â”‚
â”‚  â”‚                              â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”    â”‚    â”‚
â”‚  â”‚                              â”‚ Self-correctâ”‚ â”‚ Flag for human  â”‚    â”‚    â”‚
â”‚  â”‚                              â”‚ (retry)     â”‚ â”‚ + include flags â”‚    â”‚    â”‚
â”‚  â”‚                              â””â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”˜ â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜    â”‚    â”‚
â”‚  â”‚                                     â”‚                â”‚               â”‚    â”‚
â”‚  â”‚                                     â””â”€â”€â–º Back to LLM â”‚               â”‚    â”‚
â”‚  â”‚                                                      â”‚               â”‚    â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜    â”‚
â”‚                                                         â”‚                    â”‚
â”‚                                                         â–¼                    â”‚
â”‚                                                  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”             â”‚
â”‚                                                  â”‚    HITL     â”‚             â”‚
â”‚                                                  â”‚   Review    â”‚             â”‚
â”‚                                                  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜             â”‚
â”‚                                                         â”‚                    â”‚
â”‚                                                         â–¼                    â”‚
â”‚                                                  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”             â”‚
â”‚                                                  â”‚   Output    â”‚             â”‚
â”‚                                                  â”‚  Delivered  â”‚             â”‚
â”‚                                                  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜             â”‚
â”‚                                                                              â”‚
â”‚  â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•   â”‚
â”‚  All steps emit to OBSERVABILITY LAYER                                      â”‚
â”‚  â€¢ Token counts, latencies, pass/fail rates, quality scores                 â”‚
â”‚  â€¢ Used for tuning thresholds based on real data                            â”‚
â”‚  â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•   â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

### Component Responsibility Summary

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                     WHO DOES WHAT                                            â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  PROMPT COMPOSER                      â”‚  CAL                                 â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Measure tokens                     â”‚  âœ“ Auto-detect repo stack           â”‚
â”‚  âœ“ Calculate all budgets              â”‚  âœ“ Query Entity Graph (deps)        â”‚
â”‚  âœ“ Hard fail on impossibilities       â”‚  âœ“ Retrieve from Codex              â”‚
â”‚  âœ“ Query Org DNA Store                â”‚  âœ“ Smart curation within budget     â”‚
â”‚  âœ“ Call CAL with budget               â”‚  âœ“ Return context + manifest        â”‚
â”‚  âœ“ Assemble final prompt              â”‚                                      â”‚
â”‚  âœ“ Report metrics                     â”‚  âœ— NOT: Judge quality               â”‚
â”‚  âœ— NOT: Warn about quality            â”‚  âœ— NOT: Make budget decisions       â”‚
â”‚  âœ— NOT: Recommend decomposition       â”‚                                      â”‚
â”‚                                       â”‚                                      â”‚
â”‚  ORG DNA STORE                        â”‚  CODEX                               â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  â€¢ Behavioral context                 â”‚  â€¢ Task knowledge                    â”‚
â”‚  â€¢ Metadata filtering (task type)     â”‚  â€¢ Semantic search (content)         â”‚
â”‚  â€¢ Examples, guidance, rubrics        â”‚  â€¢ Code, architecture, incidents     â”‚
â”‚  â€¢ "How should I behave?"             â”‚  â€¢ "What do I need to know?"         â”‚
â”‚                                       â”‚                                      â”‚
â”‚  EVALUATOR                            â”‚  HARNESS                             â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Run tests, linter, SAST            â”‚  âœ“ Execute LLM calls                 â”‚
â”‚  âœ“ Check grounding                    â”‚  âœ“ Decompose complex tasks           â”‚
â”‚  âœ“ Trigger self-correct loop          â”‚  âœ“ Synthesize multi-pass results     â”‚
â”‚  âœ“ Flag for human when exhausted      â”‚                                      â”‚
â”‚                                       â”‚                                      â”‚
â”‚  OBSERVABILITY                        â”‚  HITL                                â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Track all metrics                  â”‚  âœ“ Review flagged outputs            â”‚
â”‚  âœ“ Surface patterns                   â”‚  âœ“ Approval gates for risky actions  â”‚
â”‚  âœ“ Inform threshold tuning            â”‚  âœ“ Feedback collection               â”‚
â”‚                                       â”‚                                      â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

---

## 4. Core Services

### 4.1 Context Assembly Layer (CAL)

**Source**: arXiv:2512.05470v1 â€” "Everything is Context"

#### 4.1.1 Agentic File System (AFS)

Unified namespace for all context resources:

```
/context/
â”œâ”€â”€ history/                    # Immutable interaction logs
â”‚   â””â”€â”€ {session_id}/
â”‚       â””â”€â”€ {timestamp}.jsonl
â”œâ”€â”€ memory/
â”‚   â”œâ”€â”€ episodic/              # Session-bounded summaries
â”‚   â”œâ”€â”€ factual/               # Persistent atomic facts
â”‚   â”œâ”€â”€ procedural/            # Function/tool definitions
â”‚   â””â”€â”€ user/                  # User-specific preferences
â”œâ”€â”€ scratchpad/
â”‚   â””â”€â”€ {task_id}/             # Temporary reasoning workspace
â”œâ”€â”€ tools/
â”‚   â””â”€â”€ {tool_namespace}/      # MCP-mounted tools
â””â”€â”€ knowledge/                  # Codex mount point
    â””â”€â”€ {domain}/
```

#### 4.1.2 Context Constructor

Responsibilities:
- Selects relevant context from persistent repository
- Applies compression/summarization for token limits
- Enforces access control and governance
- Generates Context Manifest documenting decisions

```typescript
interface ContextManifest {
  manifest_id: string;
  task_id: string;
  timestamp: ISO8601;
  
  // Budget tracking
  token_budget: number;
  token_used: number;
  
  // What was included
  sources: ContextSource[];
  
  // What was excluded and why
  excluded: ExcludedSource[];
  
  // Transformations applied
  compression_applied: CompressionRecord[];
  
  // Compliance checks
  governance_checks: GovernanceResult[];
}

interface ContextSource {
  path: string;                 // AFS path
  type: 'memory' | 'knowledge' | 'tool' | 'history';
  relevance_score: number;
  tokens: number;
  lineage: ProvenanceChain;
}

interface ExcludedSource {
  path: string;
  reason: 'token_limit' | 'low_relevance' | 'access_denied' | 'stale';
  score: number;
}
```

#### 4.1.3 Context Updater

Three delivery modes:

| Mode | Use Case | Mechanism |
|------|----------|-----------|
| Static Snapshot | Single-turn tasks | Full context injection at start |
| Incremental Streaming | Extended reasoning | Progressive fragment loading |
| Adaptive Refresh | Interactive sessions | Hot-swap stale context mid-task |

#### 4.1.4 Context Evaluator

Closes the feedback loop:
- Validates outputs against source context
- Detects hallucinations via semantic comparison
- Triggers human review when confidence < threshold
- Writes verified outputs back to memory

#### 4.1.5 Repo Context Resolution

CAL automatically resolves repo-specific context through detection, conventions, and Entity Graph queries â€” minimal configuration required.

**Design Principle:** Convention over configuration. Auto-detect what's possible, query what's known, configure only edge cases.

##### Auto-Detection (No Config Needed)

CAL detects stack from repo contents:

```typescript
interface DetectedStack {
  language: string;           // python, typescript, golang
  framework?: string;         // fastapi, nextjs, gin
  testing?: string;           // pytest, jest, go test
  inferred: boolean;          // true = auto-detected
}

function detectStack(repo: Repo): DetectedStack {
  // Detect from manifest files
  if (exists('package.json')) {
    const pkg = read('package.json');
    return {
      language: 'typescript',
      framework: pkg.dependencies?.next ? 'nextjs' 
               : pkg.dependencies?.react ? 'react' : undefined,
      testing: pkg.devDependencies?.jest ? 'jest' : undefined,
      inferred: true
    };
  }
  
  if (exists('pyproject.toml') || exists('requirements.txt')) {
    return {
      language: 'python',
      framework: contains('fastapi') ? 'fastapi'
               : contains('django') ? 'django' : undefined,
      testing: 'pytest',
      inferred: true
    };
  }
  
  if (exists('go.mod')) {
    return {
      language: 'golang',
      framework: contains('gin') ? 'gin' : undefined,
      testing: 'go test',
      inferred: true
    };
  }
  
  return { language: 'unknown', inferred: true };
}
```

**Usage:** Detected stack is applied as Codex filter tags automatically.

##### Convention-Based Context Files

CAL checks for common context files by convention:

```typescript
const CONVENTIONAL_CONTEXT_PATHS = [
  // Architecture & design
  'docs/ARCHITECTURE.md',
  'docs/DESIGN.md',
  'ARCHITECTURE.md',
  
  // Contribution guidelines  
  'docs/CONTRIBUTING.md',
  'CONTRIBUTING.md',
  
  // Project overview
  'README.md',
  
  // AEF-specific context (if exists)
  '.aef/context.md',
];

// Include if exists, low priority (task-relevant content wins)
for (const path of CONVENTIONAL_CONTEXT_PATHS) {
  if (exists(repo, path) && fitsInBudget(path)) {
    includeInContext(path, { priority: 'low' });
  }
}
```

**No configuration needed.** If these files exist, they're considered. If not, nothing breaks.

##### Dependency Resolution via Entity Graph

CAL queries the Entity Graph (Codex) for internal dependencies:

```typescript
interface DependencyContext {
  repo: string;
  relationship: 'imports' | 'calls_api' | 'shares_types';
  relevant_files: string[];  // Types, interfaces, README
}

async function resolveDependencies(
  repo: string, 
  taskContent: string
): Promise<DependencyContext[]> {
  // Query Entity Graph for this repo's dependencies
  const deps = await codex.entityGraph.query(`
    MATCH (r:Repo {name: $repo})-[:DEPENDS_ON]->(d:Repo)
    WHERE d.internal = true
    RETURN d.name, d.type, d.key_files
  `, { repo });
  
  // Filter to dependencies relevant to current task
  const relevantDeps = deps.filter(dep => 
    isRelevantToTask(dep, taskContent)
  );
  
  // Return files to include from each dependency
  return relevantDeps.map(dep => ({
    repo: dep.name,
    relationship: dep.type,
    relevant_files: selectRelevantFiles(dep, taskContent)
  }));
}
```

**Example:**
```
Task: Review PR in payments-service that uses AuthToken

Entity Graph knows: payments-service â†’ DEPENDS_ON â†’ shared-auth-lib

CAL includes:
  - shared-auth-lib/src/types/AuthToken.ts
  - shared-auth-lib/README.md (usage patterns)
```

##### Optional Configuration (Edge Cases Only)

For rare cases where auto-detection fails or conventions don't fit:

```yaml
# {repo}/.aef/context.yaml
# OPTIONAL â€” only needed for edge cases

# Override detected stack (rare)
stack:
  language: python
  framework: custom-internal-framework

# Additional dependencies not in manifests (rare)
additional_dependencies:
  - repo: legacy-billing-service
    reason: "Internal API calls not in package manifest"
    include:
      - src/api/types.py

# Exclude conventional files (rare)
exclude_conventional:
  - docs/ARCHITECTURE.md  # Outdated, misleading

# Always include specific files (rare)
always_include:
  - src/core/domain_types.py  # Central to all work in this repo
```

**This file is optional.** Most repos need nothing â€” auto-detection and conventions handle it.

##### Context Resolution Flow

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                     CAL REPO CONTEXT RESOLUTION                              â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  1. AUTO-DETECT STACK                                                        â”‚
â”‚     Read manifest files â†’ infer language/framework/testing                  â”‚
â”‚     Apply as Codex filter tags                                              â”‚
â”‚                                                                              â”‚
â”‚  2. CHECK CONVENTIONS                                                        â”‚
â”‚     Look for ARCHITECTURE.md, README.md, etc.                               â”‚
â”‚     Include if exists, low priority                                         â”‚
â”‚                                                                              â”‚
â”‚  3. QUERY ENTITY GRAPH                                                       â”‚
â”‚     "What internal repos does this depend on?"                              â”‚
â”‚     Include relevant interfaces/types from dependencies                     â”‚
â”‚                                                                              â”‚
â”‚  4. LOAD OPTIONAL CONFIG (if .aef/context.yaml exists)                      â”‚
â”‚     Apply overrides, additional deps, exclusions                            â”‚
â”‚                                                                              â”‚
â”‚  5. SEMANTIC SEARCH (existing design)                                        â”‚
â”‚     Query Codex for task-relevant patterns                                  â”‚
â”‚     Filtered by detected stack tags                                         â”‚
â”‚                                                                              â”‚
â”‚  6. ASSEMBLE WITHIN BUDGET                                                   â”‚
â”‚     Prioritize: task-relevant > dependencies > conventions                  â”‚
â”‚     Respect token budget from Prompt Composer                               â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

---

### 4.2 Codex (Knowledge Base)

**Design Principle**: Production-grade retrieval using hybrid search, contextual embeddings, and multi-stage reranking to achieve >80% retrieval accuracy.

> **ðŸ“„ Full specification: See `codex-architecture-deep-dive.md` for complete design details.**

#### 4.2.1 Architecture Summary

Codex is built on research-backed findings that naive RAG fails in production. Key techniques:

| Technique | Impact | Source |
|-----------|--------|--------|
| Hybrid search (vector + BM25) | +15-30% recall | Multiple benchmarks |
| Contextual retrieval | -67% retrieval failures | Anthropic research |
| Multi-stage reranking | +20-25% precision | Cross-encoder studies |
| Code-specific embeddings | 97.3% vs 11.7% MRR | Voyage benchmarks |

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                              CODEX                                       â”‚
â”‚              (Hybrid Search + Contextual Retrieval + Reranking)          â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                          â”‚
â”‚  INGESTION                           RETRIEVAL                           â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€                           â”€â”€â”€â”€â”€â”€â”€â”€â”€                           â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                     â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                    â”‚
â”‚  â”‚ Code Files  â”‚â”€â”€â–¶ AST Chunking     â”‚   Query     â”‚                    â”‚
â”‚  â”‚             â”‚    + Voyage Code-3  â”‚   Router    â”‚ (heuristic)        â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜                     â””â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”˜                    â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                            â”‚                           â”‚
â”‚  â”‚ Docs/ADRs   â”‚â”€â”€â–¶ Contextual Chunk        â–¼                           â”‚
â”‚  â”‚             â”‚    (Haiku) + text-emb  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                  â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜                        â”‚  Hybrid    â”‚ Vector + BM25    â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                        â”‚  Search    â”‚ + RRF            â”‚
â”‚  â”‚ Manifests   â”‚â”€â”€â–¶ Deterministic       â””â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”˜                  â”‚
â”‚  â”‚ CODEOWNERS  â”‚    Entity Extraction         â”‚                         â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜                              â–¼                         â”‚
â”‚         â”‚                            â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                 â”‚
â”‚         â–¼                            â”‚ Rerank Stage 1 â”‚ BGE-base        â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                     â”‚ (100 â†’ 30)     â”‚ ~50ms           â”‚
â”‚  â”‚   Qdrant    â”‚                     â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”˜                 â”‚
â”‚  â”‚ (Vec + BM25)â”‚                              â”‚                         â”‚
â”‚  â”‚             â”‚                              â–¼                         â”‚
â”‚  â”‚   Entity    â”‚                     â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                 â”‚
â”‚  â”‚   Graph     â”‚                     â”‚ Rerank Stage 2 â”‚ BGE-v2-m3       â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜                     â”‚ (30 â†’ 10)      â”‚ ~100ms          â”‚
â”‚                                      â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”˜                 â”‚
â”‚                                               â”‚                         â”‚
â”‚                                               â–¼                         â”‚
â”‚                                      â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”                 â”‚
â”‚                                      â”‚ Rerank Stage 3 â”‚ Claude          â”‚
â”‚                                      â”‚ (conditional)  â”‚ (complex only)  â”‚
â”‚                                      â””â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”˜                 â”‚
â”‚                                               â”‚                         â”‚
â”‚                                               â–¼                         â”‚
â”‚                                         CodexResult                     â”‚
â”‚                                                                          â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜

â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                          ORG DNA STORE                                   â”‚
â”‚                 (Behavioral Context - Metadata Filtering)                â”‚
â”‚                        ** SEPARATE FROM CODEX **                         â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                          â”‚
â”‚  Owned by: Prompt Composer (NOT CAL)                                    â”‚
â”‚  Selection: Metadata filtering by task type, language, tags            â”‚
â”‚  Purpose: "How should I behave?" (not "What do I need to know?")       â”‚
â”‚                                                                          â”‚
â”‚  org_dna_store/                                                         â”‚
â”‚  â”œâ”€â”€ examples/           # Few-shot exemplars                           â”‚
â”‚  â”‚   â”œâ”€â”€ code_review/                                                   â”‚
â”‚  â”‚   â””â”€â”€ incident_response/                                             â”‚
â”‚  â”œâ”€â”€ guidance/           # Plain text guidance per category             â”‚
â”‚  â””â”€â”€ rubrics/            # Structured rubrics (Tier 3)                  â”‚
â”‚                                                                          â”‚
â”‚  See Section 6.3 for full specification.                                â”‚
â”‚                                                                          â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

**Key Separation:**
| Store | Owner | Selection Logic | Purpose |
|-------|-------|-----------------|---------|
| Codex | CAL | Hybrid search + reranking | Task knowledge |
| Org DNA Store | Prompt Composer | Metadata filtering by task type | Behavioral context |

#### 4.2.2 Key Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| Vector DB | Qdrant | Hybrid search (vector + BM25), open source |
| Code Embeddings | Voyage Code-3 | 97.3% MRR on code retrieval |
| Doc Embeddings | text-embedding-3-large | Strong general-purpose |
| Contextual Gen | Claude Haiku | Prepend context at index time |
| Reranker Stage 1 | BGE-Reranker-base | Fast bulk filter (self-hosted) |
| Reranker Stage 2 | BGE-Reranker-v2-m3 | Precision rerank (self-hosted) |
| Reranker Stage 3 | Claude Sonnet | Complex queries only (existing API) |
| Entity Graph | Qdrant + code analysis | Deterministic relationship extraction |

#### 4.2.3 Query Router

Heuristic-based routing (no ML model required):

```python
class QueryRouter:
    CODE_INDICATORS = ['function', 'class', 'implement', 'code', '.py', '.ts']
    RELATIONSHIP_INDICATORS = ['depends on', 'uses', 'calls', 'owns', 'who owns']
    
    def route(self, query: str) -> QueryRoute:
        if any(ind in query.lower() for ind in self.RELATIONSHIP_INDICATORS):
            return QueryRoute.ENTITY_GRAPH
        if any(ind in query.lower() for ind in self.CODE_INDICATORS):
            return QueryRoute.CODE
        return QueryRoute.HYBRID_ALL  # Search all, merge results
```

**Routing Examples:**

| Query | Route | Rationale |
|-------|-------|-----------|
| "Show me payment processing code" | CODE | Code indicators present |
| "What depends on user-auth?" | ENTITY_GRAPH | Relationship indicator |
| "How do we handle errors?" | HYBRID_ALL | Ambiguous, search all |
| "Why did we choose Kafka?" | HYBRID_ALL | ADR search, no specific indicator |

#### 4.2.4 Expected Accuracy

| Query Type | Naive RAG | Codex Phase 1+2 |
|------------|-----------|-----------------|
| Code retrieval | 30-40% | 75-85% |
| Documentation | 50-60% | 82-90% |
| Relationship queries | 10-20% | 55-70% |
| End-to-end accuracy | 40-50% | 65-75% |

---

### 4.3 Claude Code Integration Layer

**Status**: ✅ SPECIFIED (v0.5 - Replaces Claude Harness)

> **Architecture Change**: The Claude Harness (v0.4) has been absorbed into Claude Code's native primitives. This section documents how AEF integrates with Claude Code rather than orchestrating agents directly.

#### 4.3.1 Integration Philosophy

AEF operates as a **value-add layer** on top of Claude Code, not a replacement. The integration follows three principles:

1. **Use Native Primitives**: Tasks, Skills, Subagents, Sessions — don't rebuild
2. **Hook, Don't Wrap**: Inject AEF value at key lifecycle points
3. **Enhance, Don't Compete**: Add capabilities Claude Code lacks

#### 4.3.2 Claude Code Primitives (Reference)

**Tasks** (Jan 2026):
- Dependencies between tasks stored in metadata
- Cross-session collaboration via `CLAUDE_CODE_TASK_LIST_ID`
- Broadcast updates to all sessions on same Task List
- File-based persistence in `~/.claude/tasks/`
- Works with AgentSDK (`claude -p`)

**Skills**:
- Modular task components: `SKILL.md` + optional resources
- Loaded on-demand (token-efficient)
- Organization-wide management for Team/Enterprise

**Subagents**:
- Native delegation to specialized Claude instances
- No AEF orchestration needed

**Sessions**:
- `--continue` (resume last), `--resume` (pick specific)
- Built-in persistence

#### 4.3.3 AEF Hook Points

AEF injects value at Claude Code lifecycle events:

```yaml
hooks:
  PreTaskStart:
    trigger: "Task transitions to in_progress"
    actions:
      - query_codex:
          extract_entities: true
          inject_context: true
      - load_relevant_skills:
          source: org_dna
          match: task_description
    
  PostTaskComplete:
    trigger: "Task transitions to completed"
    actions:
      - run_evaluation:
          rubrics: [correctness, consistency, completeness]
          on_fail: trigger_self_correct
      - prompt_contribution:
          condition: significance_score > threshold
          target: codex
    
  OnTaskFail:
    trigger: "Task marked failed OR error thrown"
    actions:
      - capture_audit_trail:
          include: [task_context, error, reasoning_trace]
      - trigger_self_correct:
          max_attempts: 3
          escalate_on_exhaust: true
    
  OnSessionStart:
    trigger: "Claude Code session begins"
    actions:
      - load_user_context:
          source: codex
          scope: [user, project]
      - inject_org_dna:
          format: skills
```

#### 4.3.4 Org DNA as Skills

Org DNA content is packaged as Claude Code Skills for native integration:

**Directory Structure:**
```
~/.claude/skills/
├── org-dna/
│   ├── SKILL.md           # Main behavioral guidance
│   ├── examples/
│   │   ├── good/          # Exemplary outputs
│   │   └── bad/           # Anti-patterns
│   ├── rubrics/
│   │   ├── code-review.md
│   │   └── design-doc.md
│   └── standards/
│       ├── coding.md
│       └── communication.md
├── team-overrides/
│   └── SKILL.md           # Team-specific adjustments
└── project-context/
    └── SKILL.md           # Project-specific guidance
```

**SKILL.md Format (Org DNA):**
```markdown
---
name: org-dna
description: Organizational standards and behavioral guidance
version: 1.0.0
triggers:
  - always  # Load for all tasks
---

# Organizational DNA

## Communication Standards
[content from Org DNA store]

## Code Quality Standards
[content from Org DNA store]

## Decision Principles
[content from Org DNA store]
```

#### 4.3.5 Role System (Preserved)

Roles remain an AEF concept, implemented via Skills that modify behavior:

| Role | Implementation | Effect |
|------|----------------|--------|
| **Architect** | `skills/roles/architect.md` | System-wide focus, governance emphasis |
| **Engineer** | `skills/roles/engineer.md` | Implementation focus, sprint scope |
| **Incident Responder** | `skills/roles/incident.md` | Urgency mode, minimal viable fixes |

**Role Skill Example:**
```markdown
---
name: role-architect
description: Architect role behavioral modifications
triggers:
  - keyword: "architect"
  - keyword: "system design"
  - keyword: "cross-team"
---

# Architect Role

## Focus Areas
- System-wide implications of changes
- Cross-service dependencies
- Governance and compliance
- Long-term maintainability

## Output Emphasis
- Architecture Decision Records (ADRs)
- System diagrams
- Risk assessments

## Context Prioritization
Request from Codex:
- Existing architecture documentation (required)
- Related ADRs (required)
- Service topology (optional)
```

#### 4.3.6 What Moved to Claude Code

| v0.4 AEF Component | v0.5 Location | Notes |
|--------------------|---------------|-------|
| Subagent orchestration | Claude Code native | Use `claude -p` with subagents |
| Task decomposition | Claude Code Tasks | Dependencies built-in |
| Cross-session state | Claude Code Tasks | Broadcast updates |
| Working memory (session) | Claude Code Sessions | `--continue`, `--resume` |
| Agent definitions | Claude Code Skills | Package as SKILL.md |

#### 4.3.7 What Remains in AEF

| Component | Rationale |
|-----------|-----------|
| **Codex** | Claude Code has no knowledge base |
| **CAL** | Context assembly requires orchestration logic |
| **Evaluation** | No native quality gates |
| **Self-Correct** | No native retry-with-feedback |
| **Contribution Manager** | Artifacts don't auto-flow to knowledge |
| **Audit Trail** | Tasks store state, not reasoning |
| **Role System** | Semantic layer over Skills |

#### 4.3.8 Migration Path from v0.4

For users of the v0.4 Claude Harness specification:

1. **Subagent Definitions** → Convert to Skills format
2. **Role Configs** → Convert to role Skills
3. **Working Memory** → Use `CLAUDE_CODE_TASK_LIST_ID` for project scoping
4. **Persistence Layer** → Audit Trail remains; Working Memory absorbed
5. **CAL Integration** → Unchanged; hooks into PreTaskStart

```bash
# Example: Project-scoped Claude Code session with AEF
export CLAUDE_CODE_TASK_LIST_ID=my-project
export AEF_CODEX_PROJECT=my-project
export AEF_ROLE=architect

claude --skills ~/.claude/skills/org-dna,~/.claude/skills/roles/architect
```
### 4.4 Experiment Service

**Status**: ✅ SPECIFIED

A separate async service for running prototype bake-offs and hypothesis testing. Separate because experiments can be long-running and should not block other work.

#### 4.4.1 Purpose

- Claude proposes multiple approaches to test
- Engineers submit competing implementations  
- Design questions require empirical answers

#### 4.4.2 Interaction Modes

| Mode | Behavior | Use When |
|------|----------|----------|
| **Fire and Forget** | Submit, continue work, receive results via webhook | Exploratory, not blocking |
| **Await with Timeout** | Pause THIS branch, other orchestrations continue | Decision depends on empirical answer |
| **Background with Check-in** | Continue provisionally, validate/revise on completion | Can progress with assumptions |

#### 4.4.3 Experiment Flow

```
Harness submits ExperimentRequest
    │
    ├── experiment_id, type
    ├── variants[] (code refs, configurations)
    ├── evaluation_criteria (metrics, test suite, timeout)
    └── notification (webhook | poll)
            │
            ▼
    ┌───────────────────────────────────────────────────┐
    │              Experiment Service                    │
    │                                                    │
    │  Variant A     Variant B     Variant C            │
    │  (container)   (container)   (container)          │
    │      │              │              │              │
    │      ▼              ▼              ▼              │
    │  ┌─────────────────────────────────────────────┐ │
    │  │          Telemetry Collection               │ │
    │  │  • Performance (latency, throughput)        │ │
    │  │  • Correctness (test pass/fail)             │ │
    │  │  • Code quality (complexity)                │ │
    │  └─────────────────────────────────────────────┘ │
    │                       │                          │
    │                       ▼                          │
    │           Comparison Report                      │
    │           + Recommendation                       │
    └───────────────────────────────────────────────────┘
            │
            ▼
    ExperimentResult → Harness
            │
            ├── Auto-ingest experiment record → Codex
            └── Prompt user for extracted learnings
```

#### 4.4.4 Results Handling

- **Experiment records**: Auto-ingest to Codex (automatic tier)
- **Extracted learnings**: Prompt user before Codex ingestion (prompted tier)

---

### 4.5 Observability Layer

#### 4.4.1 Trace Model

OpenTelemetry-compatible with AEF extensions:

```typescript
interface AgentTrace extends OpenTelemetrySpan {
  trace_id: string;
  span_id: string;
  parent_span_id?: string;
  
  aef: {
    // Context lineage
    context_manifest_id: string;
    context_sources: SourceReference[];
    
    // Grounding verification
    grounding_check: GroundingResult;
    hallucination_flags: HallucinationFlag[];
    
    // Agent identity
    agent_type: 'orchestrator' | 'subagent' | 'evaluator';
    agent_instance_id: string;
    
    // Task lineage
    task_id: string;
    subtask_id?: string;
  };
}
```

#### 4.4.2 Grounding Verification

```typescript
interface GroundingResult {
  verified: boolean;
  confidence: number;
  source_citations: Citation[];
  ungrounded_claims: UngroundedClaim[];
}

interface ClaimAnalysis {
  claim_text: string;
  claim_type: 'factual' | 'procedural' | 'opinion' | 'generated';
  grounded: boolean;
  source_citation?: {
    codex_document_id: string;
    chunk_id: string;
    similarity_score: number;
  };
  ungrounded_reason?: 'no_source' | 'contradicts_source' | 'hallucinated';
}
```

---

## 5. Component Specification Status

> **v0.5 Update**: Component status revised to reflect integration with Claude Code primitives.

### Fully Specified (Ready to Implement)

| Component | Status | Notes |
|-----------|--------|-------|
| Agent Registry | ✅ Ready | CRUD service with defined schema |
| Cost Controller | ✅ Ready | Standard cloud patterns apply |
| Prompt Registry | ✅ Ready | Git-like versioning for prompts |
| AFS Structure | ✅ Ready | Namespace defined |
| Context Manifest | ✅ Ready | Schema defined |
| Trace Model | ✅ Ready | OTEL-compatible schema |

### Partially Specified (Need Design Decisions)

| Component | Status | Blocking Questions |
|-----------|--------|-------------------|
| Query Router | 🟡 Partial | Intent classifier implementation |
| Safety Filters | 🟡 Partial | Which techniques to use |
| Workflow Engine | 🟡 Partial | DSL design for workflows |

### Underspecified (Require Deep Dives)

| Component | Status | Section |
|-----------|--------|---------|
| Learning Loop | ⚠️ **Needs Deep Dive** | [6.2](#62-learning--improvement-loop) |
| HITL Integration | ⚠️ **Needs Deep Dive** | [6.5](#65-human-in-the-loop-integration) |

### Recently Specified

| Component | Status | Section |
|-----------|--------|---------|
| Evaluation Framework | ✅ **Specified** | [6.1](#61-evaluation-framework) |
| Organizational DNA System | ✅ **Specified** | [6.3](#63-organizational-dna-system) |
| Prompt Composition | ✅ **Specified** | [6.4](#64-prompt-composition-specification) |
| Claude Code Integration | ✅ **Specified (v0.5)** | [4.3](#43-claude-code-integration-layer) |
| Experiment Service | ✅ **Specified** | [4.4](#44-experiment-service) |

### Deprecated/Absorbed (v0.5)

| Component | Status | Absorbed By |
|-----------|--------|-------------|
| Claude Harness | ❌ **Absorbed** | Claude Code Tasks, Subagents, Sessions |
| Subagent Model | ❌ **Absorbed** | Claude Code native subagents |
| Working Memory | ❌ **Absorbed** | Claude Code Tasks + Sessions |


---

## 6. Deep Dive Areas

### 6.1 Evaluation Framework

**Status**: âœ… SPECIFIED  
**Priority**: CRITICAL  
**Owner**: Central Shared Systems Team

#### Overview

The evaluation framework operates at two levels:
1. **Agent/Prompt Eval** â€” Development-time validation of agent changes
2. **Runtime Eval** â€” Per-output validation before PR creation

#### Quality Criteria

**High Priority (must excel):**
| Criterion | Evaluation Method |
|-----------|-------------------|
| Correctness | Execute generated tests + provided test suite |
| Error Handling | Static analysis + LLM-as-judge with rubric |
| Testability | Complexity heuristics + LLM-as-judge |
| Security | SAST tools (Semgrep, Bandit) + secrets scanning |
| Performance | Static analysis for anti-patterns + benchmarks |
| Readability | LLM-as-judge with rubric + human sampling |

**Medium Priority (should meet):**
| Criterion | Evaluation Method |
|-----------|-------------------|
| Follows Patterns | Codebase similarity scoring + LLM-as-judge |
| Style Compliance | Linters (ESLint, Ruff, etc.) |
| Minimal Scope | Diff analysis + LLM-as-judge |
| Documentation | Presence checks + LLM-as-judge for quality |
| Backward Compatibility | API diff tools + integration tests |

**Instant Reject:** Any security violation (hardcoded credentials, etc.)

#### Non-Determinism Handling

- **Runs per eval case:** 3 (configurable)
- **Zero-tolerance criteria:** Correctness, Security â€” ALL runs must pass
- **Standard criteria:** Majority (2 of 3) must pass

#### Two-Layer Runtime Evaluation

**Layer 1: Continuous (Per-Commit)**
| Check | Tool | Behavior |
|-------|------|----------|
| Security scan | Semgrep | Advisory (tracked) |
| Secrets detection | Gitleaks | Advisory (tracked) |
| Syntax check | Language-native (tsc, go build, etc.) | Advisory (tracked) |
| Linter | Repo-configured | Advisory (tracked) |
| Docstring presence | Custom check | Advisory (tracked) |

- **Gate behavior:** Advisory â€” agent sees warnings, can continue
- **Unresolved issues:** Automatically surfaced on PR

**Layer 2: Story Complete (Pre-PR)**
| Check | Method | Required |
|-------|--------|----------|
| Correctness | Execute tests | Yes |
| Security (full) | SAST full scan | Yes |
| Error handling | LLM-as-judge | Yes |
| Testability | LLM-as-judge | Yes |
| Performance | Static analysis | Yes |
| Backward compatibility | API diff + integration tests | Yes |
| Readability | LLM-as-judge | No (advisory) |
| Follows patterns | LLM-as-judge + similarity | No (advisory) |
| Minimal scope | LLM-as-judge | No (advisory) |
| Documentation | LLM-as-judge | No (advisory) |

**Self-Correct Loop:**
```
Story Complete Eval
       â”‚
       â–¼
   Pass? â”€â”€â”€Yesâ”€â”€â”€â–º Create PR (clean)
       â”‚
      No
       â”‚
       â–¼
  Attempts < 3? â”€â”€â”€Noâ”€â”€â”€â–º Create PR with EXPLICIT FLAGS
       â”‚                   - Unresolved issues listed
      Yes                  - Failure history included
       â”‚                   - Human reviewer must advise
       â–¼
  Agent Self-Corrects
       â”‚
       â””â”€â”€â–º (retry eval)
```

#### Agent/Prompt Evaluation (Development-Time)

**Purpose:** Validate agent/prompt changes before deployment

**Protocol:**
1. Run full eval suite (N=3 per case for variance)
2. Compare to baseline metrics
3. Flag if any category drops >5%
4. Human review required for flagged regressions
5. Canary deployment (10% traffic) before full rollout

**Latency/Cost Targets:**
| Scenario | Latency | Cost |
|----------|---------|------|
| Dev testing prompt change | Not critical | Should be cheap |
| Full regression before deploy | Not critical | Thorough over cheap |

#### Evaluation Dataset Structure

```
eval_suite/
â”œâ”€â”€ factual/
â”‚   â””â”€â”€ cases.jsonl           # {input, expected_output, tolerance}
â”œâ”€â”€ code_generation/
â”‚   â””â”€â”€ cases/
â”‚       â”œâ”€â”€ case_001/
â”‚       â”‚   â”œâ”€â”€ prompt.md
â”‚       â”‚   â”œâ”€â”€ test_suite.py     # Must pass these
â”‚       â”‚   â””â”€â”€ constraints.yaml   # Style, complexity
â”œâ”€â”€ code_review/
â”‚   â””â”€â”€ cases/
â”‚       â”œâ”€â”€ case_001/
â”‚       â”‚   â”œâ”€â”€ diff.patch
â”‚       â”‚   â”œâ”€â”€ expected_issues.yaml
â”‚       â”‚   â””â”€â”€ rubric.yaml
â””â”€â”€ adversarial/
    â””â”€â”€ prompt_injection_attempts.jsonl
```

#### Ownership Model

**Hybrid approach:**
- **Central Shared Systems Team:** Owns framework, rubrics, cross-cutting evals (security, style)
- **Domain Teams:** Own domain-specific eval cases (as adoption grows)

#### Configuration Schema

```yaml
evaluation_config:
  version: "1.0"
  
  agent_eval:
    runs_per_case: 3
    pass_criteria:
      zero_tolerance:
        - correctness
        - security
      majority_pass:
        - error_handling
        - testability
        - performance
        - readability
        - follows_patterns
        - style_compliance
        - minimal_scope
        - documentation
        - backward_compatibility
    
  runtime_eval:
    layers:
      continuous:
        enabled: true
        gate_behavior: "advisory"
        unresolved_issues: "surface_on_pr"
        checks:
          - name: security_scan
            tool: semgrep
            block_on_fail: false
          - name: secrets_detection
            tool: gitleaks
            block_on_fail: false
          - name: syntax_check
            tool: language_native
            block_on_fail: false
          - name: linter
            tool: repo_configured
            block_on_fail: false
          - name: docstring_presence
            tool: custom_check
            block_on_fail: false
            
      story_complete:
        enabled: true
        gate_behavior: "self_correct_loop"
        self_correct:
          max_attempts: 3
          on_exhausted: "pr_with_flags"
        pr_flags:
          include_failure_details: true
          include_attempt_history: true
          require_human_decision: true
        checks:
          - name: correctness
            method: execute_tests
            required: true
          - name: security_full
            method: sast_full
            required: true
          - name: error_handling
            method: llm_judge
            rubric: error_handling_v1
            required: true
          - name: testability
            method: llm_judge
            rubric: testability_v1
            required: true
          - name: performance
            method: static_analysis
            required: true
          - name: readability
            method: llm_judge
            rubric: readability_v1
            required: false
          - name: follows_patterns
            method: llm_judge
            rubric: patterns_v1
            context_source: repo_patterns
            required: false
          - name: minimal_scope
            method: llm_judge
            rubric: scope_v1
            required: false
          - name: documentation
            method: llm_judge
            rubric: documentation_v1
            required: false
          - name: backward_compatibility
            method: api_diff
            required: true

  overrides:
    - match:
        repo: "payments-*"
      config:
        runtime_eval.layers.story_complete.checks.backward_compatibility.required: true
        
    - match:
        agent_type: "prototype_explorer"
      config:
        runtime_eval.layers.story_complete.gate_behavior: "flag_for_review"
```

#### Open Items for Implementation

- [ ] Define LLM-as-judge rubrics (error_handling_v1, readability_v1, etc.)
- [ ] Select specific SAST tools per language
- [ ] Build initial eval dataset for Agent Eval
- [ ] Define "repo_patterns" extraction for Follows Patterns check
- [ ] Build self-correct loop orchestration

---

### 6.2 Learning & Improvement Loop

**Status**: âš ï¸ UNDERSPECIFIED  
**Priority**: HIGH  
**Blocking**: No mechanism for agents to improve over time

#### Honest Assessment

What I proposed:
```
Failures â†’ Pattern Detection â†’ Automatic Prompt Fixes
```

What's realistic:
```
Failures â†’ Human Reviews â†’ Hypothesizes Cause â†’ 
Crafts Change â†’ Tests â†’ Discovers Breakage â†’ Iterates
```

#### Automation Levels

| Capability | Automation | Notes |
|------------|------------|-------|
| Failure detection | High | Track errors, low scores, corrections |
| Failure clustering | Medium | Group similar failures (noisy) |
| Root cause ID | Low | Requires human judgment |
| Fix generation | Very Low | Prompt engineering is still art |
| Fix validation | Medium | Can run evals |

#### Realistic Pipeline

```typescript
interface ImprovementPipeline {
  // Automated: Collect and cluster failures
  failure_aggregator: {
    collect_low_confidence_outputs(): FailedOutput[];
    collect_human_corrections(): Correction[];
    collect_negative_feedback(): Feedback[];
    cluster_by_similarity(): FailureCluster[];
  };
  
  // Semi-automated: Surface patterns for humans
  pattern_surfacer: {
    rank_clusters_by_frequency(): RankedCluster[];
    extract_common_characteristics(): Pattern[];
    generate_failure_report(): Report;
  };
  
  // Manual: Human prompt engineers
  improvement_workflow: {
    assign_cluster_to_engineer(cluster: FailureCluster): Assignment;
    track_hypothesis(assignment: Assignment, hypothesis: string): void;
    track_proposed_fix(assignment: Assignment, diff: PromptDiff): void;
    trigger_eval_run(diff: PromptDiff): EvalResult;
    approve_or_reject(result: EvalResult): Decision;
  };
  
  // Automated: Deployment
  rollout: {
    canary_deploy(version: string, percent: number): void;
    monitor_canary(deployment: Deployment): Metrics;
    promote_or_rollback(metrics: Metrics): Decision;
  };
}
```

#### Open Questions â€” NEEDS DECISION

- [ ] Who are the "prompt engineers" in your org?
- [ ] What's the SLA for addressing failure clusters?
- [ ] How do we prioritize which failures to fix?
- [ ] What tooling do prompt engineers need?

---

### 6.3 Organizational DNA System

**Status**: âœ… SPECIFIED  
**Priority**: CRITICAL  
**Owner**: Prompt Composer (NOT CAL)

#### Design Principle: Separation of Concerns

| Concern | Owner | Selection Logic | Purpose |
|---------|-------|-----------------|---------|
| "How should I behave?" | Prompt Composer + Org DNA Store | Metadata filtering by task type | Behavioral context |
| "What do I need to know?" | CAL + Codex | Semantic relevance to task content | Task context |

**Key insight:** Org DNA examples are selected by task TYPE (code review â†’ code review examples), not semantic similarity to task CONTENT. This is fundamentally different from CAL's knowledge retrieval.

#### Architecture

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                        PROMPT COMPOSER                                   â”‚
â”‚                   (Owns behavioral context)                              â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                          â”‚
â”‚  Retrieves from Org DNA Store:                                          â”‚
â”‚  â”œâ”€â”€ examples/      Selected by: task_type + language + tags           â”‚
â”‚  â”œâ”€â”€ guidance/      Selected by: task_type (exact match)               â”‚
â”‚  â””â”€â”€ rubrics/       Selected by: task_type (exact match)               â”‚
â”‚                                                                          â”‚
â”‚  Selection logic: Metadata filtering, NOT semantic search              â”‚
â”‚  Token budget: Fixed allocation (~800-1000 tokens)                     â”‚
â”‚                                                                          â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                                    â”‚
                                    â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                              CAL                                         â”‚
â”‚                     (Owns task context)                                  â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                          â”‚
â”‚  Retrieves from Codex:                                                  â”‚
â”‚  â”œâ”€â”€ code_store/    Relevant code patterns for THIS repo/task          â”‚
â”‚  â”œâ”€â”€ architecture/  Relevant ADRs for THIS decision                    â”‚
â”‚  â”œâ”€â”€ incidents/     Relevant playbooks for THIS issue                  â”‚
â”‚  â””â”€â”€ team_norms/    Relevant standards for THIS code                   â”‚
â”‚                                                                          â”‚
â”‚  Selection logic: Semantic relevance to task content                   â”‚
â”‚  Token budget: Variable, fills remaining space                         â”‚
â”‚                                                                          â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

#### Progressive Complexity (Tier Model)

No explicit "tier" setting. Configure what you care about; everything else uses defaults.

```yaml
# org_dna.yaml

# Tier 3: Full rubric (I have strong opinions)
error_handling:
  rubric:
    criteria:
      - id: explicit_errors
        weight: 0.4
        levels:
          1: "Errors swallowed"
          5: "Comprehensive Result<T,E> usage"
    pass_threshold: 4.0

# Tier 2: Just guidance
testing:
  guidance: "Prefer integration tests. Unit tests only for complex logic."

# Tier 2: Just examples
security:
  examples:
    - path: "examples/security/good_input_validation.md"

# Tier 1: Not specified â†’ built-in defaults
# readability: (uses AEF defaults)
# communication: (uses AEF defaults)
```

**Resolution logic:**
```
For each category:
  1. Rubric defined?       â†’ Use rubric (Tier 3)
  2. Guidance or examples? â†’ Enhance default prompt (Tier 2)
  3. Nothing defined?      â†’ Use built-in prompt (Tier 1)
```

#### Layered Defaults with Override

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚  LAYER 3: TEAM/PROJECT OVERRIDES (Most Specific)                        â”‚
â”‚  - Team-specific patterns, repo-level configuration                     â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚  LAYER 2: ORGANIZATION CUSTOMS                                          â”‚
â”‚  - Company coding standards, org-wide architecture decisions            â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚  LAYER 1: AEF DEFAULTS (Ships with Framework)                           â”‚
â”‚  - Industry best practices, language conventions, universal security    â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜

Resolution: Layer 3 â†’ Layer 2 â†’ Layer 1 (most specific wins)
Default behavior: MERGE (customizations layer on top)
Explicit replace: Per-category opt-in when full control needed
```

**Merge example:**
```yaml
# Layer 1 (AEF default)
error_handling:
  - "Use explicit error types"
  - "Always wrap errors with context"

# Layer 2 (org) - ADDS to defaults
error_handling:
  - "Use Result<T,E> pattern"
  
# Result: All three rules apply
```

**Replace example:**
```yaml
# Layer 2 (org) - REPLACES defaults for this category
security:
  _strategy: "replace"
  values:
    - "All secrets via Vault only"
    - "mTLS between all services"
```

#### Org DNA Store Structure

Simple metadata-indexed store (NOT Codex, no vector embeddings):

```
org_dna_store/
â”œâ”€â”€ examples/
â”‚   â”œâ”€â”€ code_review/
â”‚   â”‚   â”œâ”€â”€ pr_1234.md       # tagged: typescript, error-handling
â”‚   â”‚   â””â”€â”€ pr_2345.md       # tagged: python, testing
â”‚   â””â”€â”€ incident_response/
â”‚       â””â”€â”€ inc_001.md       # tagged: database, outage
â”œâ”€â”€ guidance/
â”‚   â”œâ”€â”€ code_review.md
â”‚   â””â”€â”€ incident_response.md
â””â”€â”€ rubrics/
    â””â”€â”€ code_review_v1.yaml
```

**Query interface:**
```typescript
interface OrgDnaStore {
  getExamples(params: {
    category: string;
    language?: string;
    tags?: string[];
    quality?: 'excellent' | 'acceptable' | 'poor';
    limit?: number;
  }): Example[];
  
  getGuidance(category: string): string | null;
  
  getRubric(category: string): Rubric | null;
}
```

#### Example Capture Workflow

**Lightweight capture:**
```bash
/capture-example "PR #1234 is an excellent code review"
```

**Automated processing:**
```
User Input
    â”‚
    â–¼
Agent Fetches Content (GitHub MCP)
    â”‚
    â–¼
Agent Analyzes & Generates Metadata
  â€¢ Detects language
  â€¢ Infers tags
  â€¢ Extracts exemplary parts
  â€¢ Writes "Why This Is Excellent"
    â”‚
    â–¼
Agent Outputs Structured File
  â†’ org_dna/examples/code_review/pr_1234.md
    â”‚
    â–¼
(Optional) Human Reviews
  [Accept] [Edit] [Reject]
```

**Example file format (auto-generated):**
```markdown
---
id: pr_1234_code_review
quality: excellent
category: code_review
language: typescript
tags: [error-handling, api-design]
source: github:PR#1234
captured_by: alice@company.com
captured_date: 2025-01-15
auto_generated: true
---

## Context
PR #1234: Refactored payment service error handling

## Exemplary Content
> The error wrapping here loses the original stack trace...

## Why This Is Excellent
- Specific and actionable
- Explains the "why"
- Constructive tone
```

**Review configuration:**
```yaml
org_dna:
  capture:
    review_required: false  # Default: auto-commit
    # Set to true for teams wanting approval workflow
```

#### Content Priority

| Priority | Content Type | Purpose |
|----------|--------------|---------|
| 1 | **Rubrics** | Required for Evaluation Framework (LLM-as-judge) |
| 2 | **Examples** | Highest impact on agent behavior |
| 3 | **Rules** | Actionable, partially automatable |
| 4 | **Anti-examples** | Valuable after positive examples exist |
| 5 | **Principles** | Good documentation, low behavior impact |

**AEF ships with:**
- Default rubrics for all categories
- Default rules per language (TypeScript, Python, Go)
- Default principles

**Users add:**
- Examples (captured from real work)
- Custom guidance
- Override rubrics (Tier 3) as needed

#### Example Selection at Runtime

**Prompt Composer queries Org DNA Store:**
```typescript
// For task: Review TypeScript PR with error handling
const examples = orgDnaStore.getExamples({
  category: "code_review",
  language: "typescript",
  tags: ["error-handling"],
  quality: "excellent",
  limit: 2
});

const guidance = orgDnaStore.getGuidance("code_review");
const rubric = orgDnaStore.getRubric("code_review");
```

**Selection logic:**
1. Filter by category (exact match)
2. Filter by language (match or "any")
3. Rank by tag overlap
4. Select top N (default: 2)
5. Optionally include 1 anti-example for high-risk tasks

#### Integration with Prompt Composition

```
Task: Review TypeScript PR for error handling
                    â”‚
                    â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ PROMPT COMPOSER                                                          â”‚
â”‚                                                                          â”‚
â”‚ 1. Get agent type: code_reviewer                                        â”‚
â”‚ 2. Query Org DNA Store:                                                 â”‚
â”‚    - getExamples({ category: "code_review", language: "typescript" })   â”‚
â”‚    - getGuidance("code_review")                                         â”‚
â”‚    - getRubric("code_review")                                           â”‚
â”‚ 3. Assemble behavioral context (fixed budget: ~1000 tokens)             â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                    â”‚
                    â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ CAL CONTEXT CONSTRUCTOR                                                  â”‚
â”‚                                                                          â”‚
â”‚ 1. Query Codex for task-relevant knowledge:                             â”‚
â”‚    - Code patterns in payments-service repo                             â”‚
â”‚    - Team coding standards                                              â”‚
â”‚    - Recent similar PRs                                                 â”‚
â”‚ 2. Rank by semantic relevance to THIS PR's content                      â”‚
â”‚ 3. Fill remaining token budget                                          â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                    â”‚
                    â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ FINAL PROMPT                                                             â”‚
â”‚                                                                          â”‚
â”‚ [1. System Foundation]                                                   â”‚
â”‚ [2. Agent Persona: Code Reviewer]                                       â”‚
â”‚ [3. Org DNA: 2 examples + guidance]       â† From Prompt Composer        â”‚
â”‚ [4. Task Context: repo patterns]          â† From CAL                    â”‚
â”‚ [5. Task: Review this PR]                                               â”‚
â”‚ [6. Guardrails]                                                         â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

#### Open Items for Implementation

- [ ] Build Org DNA Store with metadata indexing
- [ ] Implement `/capture-example` command with MCP integrations
- [ ] Create AEF default rubrics for all evaluation categories
- [ ] Create AEF default rules for TypeScript, Python, Go
- [ ] Build Prompt Composer integration with Org DNA Store

---

### 6.4 Prompt Composition Specification

**Status**: âœ… SPECIFIED  
**Priority**: CRITICAL  
**Owner**: Prompt Composer

#### Overview

The Prompt Composer assembles the final prompt from multiple sources. It serves as the coordinator, calculating budgets and directing CAL and Org DNA Store on what to retrieve.

**Key Principle:** Prompt Composer owns budget allocation. It tells CAL and Org DNA Store what they have to work with. They retrieve within those constraints. Assembly is concatenation â€” no compression in the happy path.

#### Composition Order

Based on LLM attention patterns (primacy/recency effects) and instruction-following research:

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ PROMPT STRUCTURE                                                             â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 1. SYSTEM FOUNDATION                                    ~200 tokens    â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Model identity and role                                              â”‚ â”‚
â”‚  â”‚ â€¢ Universal safety constraints                                         â”‚ â”‚
â”‚  â”‚ â€¢ Output format requirements (JSON, markdown, etc.)                    â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ WHY FIRST: Primacy effect ensures foundational constraints attended.  â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 2. AGENT PERSONA                                        ~300 tokens    â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Role definition ("You are a code reviewer for...")                   â”‚ â”‚
â”‚  â”‚ â€¢ Capabilities and boundaries                                          â”‚ â”‚
â”‚  â”‚ â€¢ Tone and communication style                                         â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ WHY SECOND: Establishes identity before behavioral examples.          â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 3. ORGANIZATIONAL DNA                                   ~800 tokens    â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Guidance (plain text rules/principles)               ~200 tokens     â”‚ â”‚
â”‚  â”‚ â€¢ Few-shot examples (1-2 positive)                     ~500 tokens     â”‚ â”‚
â”‚  â”‚ â€¢ Anti-example (optional, for high-risk)               ~100 tokens     â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ Source: Org DNA Store (via Prompt Composer)                           â”‚ â”‚
â”‚  â”‚ WHY THIRD: Examples work best after identity, before task dilutes.    â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 4. TASK CONTEXT                                         ~variable      â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Required content (PR diff, user files, etc.)         MUST FIT       â”‚ â”‚
â”‚  â”‚ â€¢ Retrieved knowledge from Codex                       FILLS REST     â”‚ â”‚
â”‚  â”‚ â€¢ Conversation history (if multi-turn)                 BUDGET: 25%    â”‚ â”‚
â”‚  â”‚ â€¢ Tool definitions (if tools enabled)                  IF NEEDED      â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ Source: CAL Context Constructor (given budget by Prompt Composer)     â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 5. TASK INSTRUCTION                                     ~300 tokens    â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Specific task description                                            â”‚ â”‚
â”‚  â”‚ â€¢ User input/request                                                   â”‚ â”‚
â”‚  â”‚ â€¢ Expected output format and success criteria                          â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ WHY FIFTH: Recency effect on task specifics.                          â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â”‚  â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â” â”‚
â”‚  â”‚ 6. GUARDRAILS REMINDER                                  ~100 tokens    â”‚ â”‚
â”‚  â”‚â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”‚ â”‚
â”‚  â”‚ â€¢ Critical constraints restated                                        â”‚ â”‚
â”‚  â”‚ â€¢ "Remember: Never do X. Always do Y."                                â”‚ â”‚
â”‚  â”‚                                                                        â”‚ â”‚
â”‚  â”‚ WHY LAST: Recency bias ensures constraints strongly attended.         â”‚ â”‚
â”‚  â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜ â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

#### Budget Calculation Flow

Prompt Composer calculates budgets BEFORE retrieval:

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                        BUDGET CALCULATION FLOW                               â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  STEP 1: Measure required content                                            â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  required_tokens = (                                                         â”‚
â”‚      user_input             # The question/request                          â”‚
â”‚    + attached_content       # PR diff, uploaded files, code to review       â”‚
â”‚    + system_foundation      # 200 (always needed)                           â”‚
â”‚    + guardrails             # 100 (always needed)                           â”‚
â”‚  )                                                                           â”‚
â”‚                                                                              â”‚
â”‚  STEP 2: Hard feasibility check                                              â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  if required_tokens > model_limit:                                          â”‚
â”‚      â†’ Return error: INFEASIBLE                                             â”‚
â”‚      â†’ Caller must reduce content size                                      â”‚
â”‚      â†’ Prompt Composer does NOT suggest how (not its job)                   â”‚
â”‚                                                                              â”‚
â”‚  STEP 3: Calculate enhancement budget                                        â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  available = model_limit - required_tokens                                  â”‚
â”‚  enhancement_budget = min(available, soft_cap_for_task_type)                â”‚
â”‚                                                                              â”‚
â”‚  STEP 4: Distribute enhancement budget                                       â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  agent_persona = 300                                                        â”‚
â”‚  org_dna = 800                                                              â”‚
â”‚  task_instruction = 300                                                     â”‚
â”‚                                                                              â”‚
â”‚  if conversation_history:                                                    â”‚
â”‚      history_budget = (enhancement_budget - 1400) * 0.25                    â”‚
â”‚      cal_budget = (enhancement_budget - 1400) - history_budget              â”‚
â”‚  else:                                                                       â”‚
â”‚      cal_budget = enhancement_budget - 1400                                 â”‚
â”‚                                                                              â”‚
â”‚  STEP 5: Call CAL with budget                                                â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  CAL receives: { task, token_budget: cal_budget }                           â”‚
â”‚  CAL returns: context â‰¤ cal_budget (no compression needed)                  â”‚
â”‚                                                                              â”‚
â”‚  STEP 6: Assemble and return with metrics                                    â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚                                                                              â”‚
â”‚  Return composed prompt + metrics for observability                         â”‚
â”‚  Do NOT warn, predict quality, or recommend decomposition                   â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

#### Design Principle: Minimal Responsibility

**What Prompt Composer DOES:**
- Measure token counts
- Hard fail on impossibilities (exceeds model limit)
- Calculate and distribute budgets
- Assemble prompt sections in correct order
- Report metrics for observability

**What Prompt Composer does NOT do:**
- Predict quality outcomes
- Warn about soft limits
- Recommend decomposition strategies
- Make orchestration decisions

**Why:** Quality concerns are handled by downstream systems designed for that purpose:

| Concern | Handled By |
|---------|------------|
| Important content prioritization | CAL smart curation |
| Hallucination detection | Context Evaluator |
| Quality failures | Self-correct loop (3 attempts) |
| Edge cases | Human review |
| Pattern detection | Observability (tune based on real data) |

This separation keeps Prompt Composer simple and testable while ensuring quality concerns are addressed by specialized components.

#### Content Categories

| Category | Examples | Compressible? |
|----------|----------|---------------|
| **Required** | PR diff, user's code, uploaded files, specific question | NO â€” can't do the task without it |
| **Behavioral** | Org DNA examples, guidance, persona | Partially â€” helps quality but task still possible |
| **Enrichment** | Codex patterns, similar code, standards | Yes â€” improves quality but not essential |

#### Soft Caps (Attention Degradation Prevention)

Even with 200K available, attention degrades. Recommended limits:

| Task Type | Soft Cap | Rationale |
|-----------|----------|-----------|
| Code review | 20K tokens | Need full diff + surrounding code |
| Code generation | 15K tokens | Relevant patterns + specs |
| Q&A / Factual | 8K tokens | Focused retrieval |
| Incident response | 20K tokens | Logs + playbooks + history |
| Default | 15K tokens | Balanced |

#### Conflict Resolution Rules

```
PRIORITY ORDER (highest to lowest):
â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

1. SAFETY CONSTRAINTS (always win)
   â€¢ Guardrails section overrides everything
   â€¢ Cannot be overridden by task instructions

2. TASK-SPECIFIC INSTRUCTIONS
   â€¢ Section 5 overrides general persona
   â€¢ User's specific request takes precedence

3. ORG DNA EXAMPLES
   â€¢ Demonstrated behavior in examples
   â€¢ When examples conflict with persona, examples win

4. AGENT PERSONA
   â€¢ Default behavior when not overridden

5. SYSTEM FOUNDATION
   â€¢ Foundational but general
   â€¢ Most likely to be overridden by specifics
```

#### Multi-Turn Conversation Handling

```yaml
conversation:
  history_budget_percent: 25    # % of enhancement budget
  verbatim_turns: 3             # Keep last N turns as-is
  summarize_earlier: true       # Summarize turns before that
```

History is placed within Task Context section.

#### CAL Interface

CAL accepts budget from Prompt Composer:

```typescript
interface CALContextRequest {
  task: TaskDescription;
  token_budget: number;           // â† Prompt Composer provides this
  required_content?: string;      // Already measured, passed through
  
  // Optional hints
  repo?: string;
  file_paths?: string[];
  topics?: string[];
}

interface CALContextResponse {
  context: string;
  tokens_used: number;            // Always â‰¤ budget
  manifest: ContextManifest;
  
  // Observability
  budget_utilization: number;
  excluded_for_budget: number;
}
```

#### Prompt Composer Interface

```typescript
interface PromptComposer {
  compose(params: ComposeParams): ComposedPrompt;
}

interface ComposeParams {
  agent_type: string;
  task_type: string;
  task_instruction: string;
  user_input: string;
  required_content?: string;      // PR diff, files, etc.
  
  // For Org DNA selection
  language?: string;
  tags?: string[];
  risk_level?: 'low' | 'medium' | 'high';
  
  // Multi-turn
  conversation_history?: Message[];
}

interface ComposedPrompt {
  // If null, request was infeasible (exceeds model limit)
  prompt: string | null;
  
  // If infeasible, error details
  error?: {
    type: 'exceeds_model_limit';
    required_tokens: number;
    model_limit: number;
  };
  
  // Section breakdown (when successful)
  sections?: {
    system_foundation: { content: string; tokens: number };
    agent_persona: { content: string; tokens: number };
    org_dna: { 
      content: string; 
      tokens: number;
      examples_used: string[];
      guidance_used: string[];
    };
    task_context: { 
      content: string; 
      tokens: number;
      sources: ContextSource[];
    };
    task_instruction: { content: string; tokens: number };
    guardrails: { content: string; tokens: number };
  };
  
  // Always present for observability
  metrics: {
    required_tokens: number;
    enhancement_budget: number;
    total_tokens: number;
    model_limit: number;
    budget_utilization: number;   // total / model_limit
  };
  
  // No warnings, no recommendations â€” just metrics
}
```

#### Configuration Schema

```yaml
prompt_composition:
  version: "1.0"
  
  # Fixed section budgets
  budgets:
    system_foundation: 200
    agent_persona: 300
    org_dna: 800
    task_instruction: 300
    guardrails: 100
  
  # Soft caps for task context (prevents attention degradation)
  task_context_soft_caps:
    default: 15000
    by_task_type:
      code_review: 20000
      code_generation: 15000
      qa_factual: 8000
      incident_response: 20000
  
  # Multi-turn settings
  conversation:
    history_budget_percent: 25
    verbatim_turns: 3
    summarize_earlier: true
  
  # Conflict handling
  conflicts:
    detection: true
    block_on_conflict: false      # Warn, don't block
    priority_order:
      - guardrails
      - task_instruction
      - org_dna_examples
      - agent_persona
      - system_foundation
```

#### Component Responsibilities

```
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚                     CLEAR SEPARATION OF CONCERNS                             â”‚
â”œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¤
â”‚                                                                              â”‚
â”‚  PROMPT COMPOSER                                                             â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Measure required content tokens                                          â”‚
â”‚  âœ“ Hard fail if exceeds model limit                                         â”‚
â”‚  âœ“ Calculate and distribute budgets                                         â”‚
â”‚  âœ“ Coordinate retrieval (call CAL and Org DNA Store with budgets)           â”‚
â”‚  âœ“ Assemble sections in correct order                                       â”‚
â”‚  âœ“ Report metrics for observability                                         â”‚
â”‚  âœ— NOT: Warn about quality                                                  â”‚
â”‚  âœ— NOT: Recommend decomposition                                             â”‚
â”‚  âœ— NOT: Make orchestration decisions                                        â”‚
â”‚                                                                              â”‚
â”‚  CAL CONTEXT CONSTRUCTOR                                                     â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Receive budget from Prompt Composer                                      â”‚
â”‚  âœ“ Smart curation: prioritize important content                             â”‚
â”‚  âœ“ Auto-detect repo stack                                                   â”‚
â”‚  âœ“ Query Entity Graph for dependencies                                      â”‚
â”‚  âœ“ Retrieve from Codex within budget                                        â”‚
â”‚  âœ“ Return context + manifest                                                â”‚
â”‚  âœ— NOT: Decide what's "too big"                                             â”‚
â”‚                                                                              â”‚
â”‚  ORG DNA STORE                                                               â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Return examples/guidance matching task type                              â”‚
â”‚  âœ“ Filter by language and tags                                              â”‚
â”‚  âœ“ Stay within allocation (~800 tokens)                                     â”‚
â”‚                                                                              â”‚
â”‚  CLAUDECODE HARNESS (when used)                                              â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  âœ“ Decide if task needs decomposition                                       â”‚
â”‚  âœ“ Implement multi-pass workflows                                           â”‚
â”‚  âœ“ Call Prompt Composer for each subtask                                    â”‚
â”‚  âœ“ Synthesize results                                                       â”‚
â”‚                                                                              â”‚
â”‚  DOWNSTREAM QUALITY SYSTEMS                                                  â”‚
â”‚  â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€ â”‚
â”‚  â€¢ Evaluator: Grounding verification, hallucination detection               â”‚
â”‚  â€¢ Self-correct loop: Retry on flagged issues (3 attempts)                  â”‚
â”‚  â€¢ Human review: Final backstop                                             â”‚
â”‚  â€¢ Observability: Track patterns, inform tuning                             â”‚
â”‚                                                                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

#### Handling Large Required Content

**Prompt Composer's role is minimal:**

| Scenario | Prompt Composer Action |
|----------|----------------------|
| Required content exceeds model limit | Return error, caller must reduce scope |
| Required content fits but is large | Proceed normally, report metrics |
| Quality might suffer | NOT Prompt Composer's concern |

**Quality is handled by downstream systems:**

```
Large PR submitted
       â”‚
       â–¼
Prompt Composer: Fits? â†’ Yes â†’ Assemble, return metrics
       â”‚
       â–¼
CAL: Smart curation within budget (prioritize important sections)
       â”‚
       â–¼
LLM: Generate output
       â”‚
       â–¼
Evaluator: Check grounding, flag hallucinations
       â”‚
       â–¼
Self-correct loop: Retry if flagged (up to 3 attempts)
       â”‚
       â–¼
Human review: Final backstop if issues persist
       â”‚
       â–¼
Observability: Track context_size vs quality_score correlation
              â†’ Tune soft caps based on real data over time
```

**Why no warnings?**
- Users ignore them ("proceed anyway")
- We can't reliably predict quality degradation
- Downstream systems handle actual failures
- Observability surfaces real patterns for tuning

**When decomposition is needed:**
- Caller (not Prompt Composer) decides to use ClaudeCode Harness
- Harness implements triage â†’ deep-dive â†’ synthesis workflows
- This is orchestration concern, not composition concern

#### Open Items for Implementation

- [ ] Create System Foundation templates
- [ ] Create Agent Persona templates + compact variants
- [ ] Create Guardrails templates
- [ ] Implement conversation history summarization
- [ ] Build budget calculation logic
- [ ] Integrate with CAL and Org DNA Store

---

### 6.5 Human-in-the-Loop Integration

**Status**: âš ï¸ UNDERSPECIFIED  
**Priority**: HIGH  
**Blocking**: No structured human oversight

#### Approval Gate Configuration

```typescript
interface ApprovalGateConfig {
  gate_id: string;
  trigger_conditions: TriggerCondition[];
  escalation_path: EscalationPath;
  timeout_action: 'auto_approve' | 'auto_reject' | 'escalate';
  timeout_duration: Duration;
}

// Example configurations
const gates: ApprovalGateConfig[] = [
  {
    gate_id: 'code_execution',
    trigger_conditions: [
      { type: 'action', action: 'execute_code' },
      { type: 'risk_score', threshold: 0.7 }
    ],
    escalation_path: {
      primary: 'task_owner',
      fallback: 'team_lead',
      final: 'security_team'
    },
    timeout_action: 'auto_reject',
    timeout_duration: '4h'
  },
  {
    gate_id: 'external_communication',
    trigger_conditions: [
      { type: 'action', action: 'post_to_github' },
      { type: 'action', action: 'send_email' }
    ],
    escalation_path: { primary: 'task_owner' },
    timeout_action: 'auto_reject',
    timeout_duration: '24h'
  },
  {
    gate_id: 'low_confidence',
    trigger_conditions: [
      { type: 'confidence_score', threshold: 0.6 },
      { type: 'grounding_score', threshold: 0.5 }
    ],
    escalation_path: { primary: 'domain_expert_pool' },
    timeout_action: 'escalate',
    timeout_duration: '1h'
  }
];
```

#### Open Questions â€” NEEDS DECISION

- [ ] What UI for human review? (Web app? Slack? Email?)
- [ ] How to route to the right human? (Skills? Load balancing?)
- [ ] What info does reviewer see? (Output only? Full trace?)
- [ ] How is feedback captured? (Approve/reject? Edit? Rating?)
- [ ] What are the initial gates for your org?

---

## 7. Secondary Use Cases

### 7.1 Experiment Sandbox

Autonomous PoC/PoT execution environment.

**Workflow:**
1. Problem Definition: User provides questions + constraints
2. Hypothesis Generation: Agent generates testable hypotheses
3. Experiment Planning: Creates PoC plans with success criteria
4. Concurrent Execution: Run experiments in isolated VMs
5. Telemetry Collection: Capture metrics, results, errors
6. Synthesis: Analyze, generate insights, update Codex

### 7.2 GitHub PR Assistant

**Workflow:**
1. Checkout & Analyze: Clone branch, parse PR description
2. Test Coverage Analysis: Identify gaps, generate missing tests
3. Validation Gate: Run tests, reject on failure
4. Review Prioritization: Score changes, check standards via Codex
5. Post Summary: Focused review guide for human

### 7.3 Enterprise Dependency Mapper

**Capabilities:**
- Track package dependencies across codebases
- Map API/service relationships
- Change impact analysis (blast radius)
- Vulnerability monitoring (CVE feeds)
- Sync to Codex Entity Graph

---

## 8. Implementation Phases

> **v0.5 Update**: Phases revised to prioritize unique AEF value (Codex, Evaluation) while leveraging Claude Code primitives for orchestration.

### Phase 0: Deep Dives (Weeks 1-2) ✅ Partially Complete
- [x] Complete Evaluation Framework specification
- [x] Complete Org DNA System specification
- [x] Complete Prompt Composition specification
- [x] Complete Claude Code Integration specification (v0.5)
- [ ] Complete HITL Integration specification
- [ ] Define Learning Loop tooling requirements

### Phase 1: Foundation (Weeks 3-6) — REVISED
Focus: Build what Claude Code doesn't provide

| Component | Priority | Rationale |
|-----------|----------|-----------|
| **Codex MVP** | **HIGH** | No Claude Code equivalent; unique value |
| **Evaluation Framework** | **HIGH** | No quality gates in Claude Code |
| **Observability Foundation** | MEDIUM | OTEL, basic dashboard |
| **Agent Registry** | LOW | May leverage Claude Code subagent tracking |

Deliverables:
- [ ] Codex MVP (single domain store with hybrid search)
- [ ] Evaluation harness (rubric-based, self-correct loop)
- [ ] Basic audit trail capture
- [ ] Org DNA as Skills (first package)

### Phase 2: Integration & Quality (Weeks 7-10) — REVISED
Focus: Hook into Claude Code lifecycle

| Component | Priority | Rationale |
|-----------|----------|-----------|
| **Claude Code Hooks** | **HIGH** | PreTaskStart, PostTaskComplete, OnTaskFail |
| **Contribution Manager** | **HIGH** | Close the knowledge loop |
| **Codex Federation** | MEDIUM | Multi-domain routing |
| **CAL Implementation** | MEDIUM | Context assembly with Codex |

Deliverables:
- [ ] Hook implementations (env-based injection)
- [ ] Contribution Manager (prompted + automatic tiers)
- [ ] Codex Federation (all domain stores + router)
- [ ] CAL with Codex integration

### Phase 3: Human Integration (Weeks 11-14)
- [ ] HITL Engine (gates, escalation, feedback)
- [ ] Learning Loop tooling
- [ ] Safety Filters
- [ ] Cost Controller

### Phase 4: Polish & Scale (Weeks 15-18)
- [ ] Role Skills library (Architect, Engineer, IR)
- [ ] Org DNA management UI
- [ ] Advanced evaluation (LLM-as-judge)
- [ ] Cross-team knowledge federation

### Implementation Priority Matrix

| Component | Unique to AEF? | Claude Code Overlap | Priority |
|-----------|----------------|---------------------|----------|
| Codex | ✅ Yes | None | **P0** |
| Evaluation | ✅ Yes | None | **P0** |
| Contribution Manager | ✅ Yes | None | **P0** |
| Audit Trail | ✅ Partial | Tasks store state, not reasoning | **P1** |
| CAL | ✅ Yes | None | **P1** |
| Org DNA | ⚠️ Partial | Skills format, but AEF manages content | **P1** |
| Role System | ⚠️ Partial | Skills, but semantic layer | **P2** |
| ~~Orchestration~~ | ❌ No | Tasks, Subagents | ~~Defer~~ |
| ~~Working Memory~~ | ❌ No | Tasks + Sessions | ~~Defer~~ |


---

## 9. Open Questions

### Technical Decisions Needed

| Question | Options | Impact | Status |
|----------|---------|--------|--------|
| Vector DB | pgvector vs Qdrant vs Pinecone | Cost, scale, ops complexity | âœ… **Decided: Qdrant** |
| Graph DB | Neo4j vs Neptune vs Memgraph | Query language, hosting | âœ… **Decided: Qdrant (simple graph) or defer** |
| Workflow Engine | Build vs Temporal vs Prefect | Flexibility vs time-to-value | Open |
| Intent Classifier | Rule-based vs LLM vs hybrid | Accuracy vs latency vs cost | âœ… **Decided: Heuristic (rule-based)** |

### Organizational Decisions Needed

| Question | Owner | Deadline |
|----------|-------|----------|
| Who creates eval datasets? | | |
| Who curates Org DNA examples? | | |
| Who are prompt engineers? | | |
| What are initial HITL gates? | | |
| Multi-tenancy requirements? | | |

---

## 10. Decision Log

| Date | Decision | Rationale | Decided By |
|------|----------|-----------|------------|
| Jan 2025 | Federated Codex over monolithic | RAG performance with disparate content | Initial design |
| Jan 2025 | Few-shot examples as primary value encoding | More effective than declarative statements | Analysis |
| Jan 2025 | Org DNA Store separate from Codex | Different selection logic: behavioral (metadata) vs task knowledge (semantic) | Deep dive |
| Jan 2025 | Prompt Composer owns budget, not quality | Hard fail on impossibilities, report metrics, trust downstream systems for quality | Deep dive |
| Jan 2025 | No advisory warnings in Prompt Composer | Users ignore them; downstream systems (Evaluator, self-correct, human review) handle quality | Deep dive |
| Jan 2025 | CAL auto-detects repo context | Convention over configuration; Entity Graph for dependencies; optional config for edge cases | Deep dive |
| Jan 2025 | Progressive complexity for Org DNA | No tier setting; configure what you care about; everything else uses defaults | Deep dive |
| Jan 2025 | Layered runtime evaluation | Continuous (per-commit, advisory) + Story Complete (pre-PR, self-correct loop) | Deep dive |
| Jan 2025 | Self-correct loop with graceful degradation | 3 attempts, then PR with explicit flags for human decision | Deep dive |
| Jan 2025 | **Hybrid search (vector + BM25) for Codex** | +15-30% recall over pure vector; proven in production | Codex deep dive |
| Jan 2025 | **Voyage Code-3 for code embeddings** | 97.3% MRR vs 11.7% for generic embeddings | Codex deep dive |
| Jan 2025 | **Contextual retrieval with Claude Haiku** | -67% retrieval failures per Anthropic research | Codex deep dive |
| Jan 2025 | **AST-aware chunking for code** | Preserves semantic units (functions, classes) | Codex deep dive |
| Jan 2025 | **Multi-stage self-hosted reranking** | BGE-base â†’ BGE-v2-m3 â†’ Claude (conditional); high accuracy without vendor lock-in | Codex deep dive |
| Jan 2025 | **Code-derived Entity Graph (deterministic)** | Extract from imports, manifests, CODEOWNERS; no LLM extraction risk | Codex deep dive |
| Jan 2025 | **Qdrant for vector storage** | Open source, native hybrid search, good performance | Codex deep dive |
| Jan 2025 | **Event-driven freshness over CDC** | Git hooks, webhooks; simpler than full CDC, sufficient for update patterns | Codex deep dive |
| Jan 2025 | **Defer full GraphRAG to Phase 3** | High complexity, unclear ROI vs. code-derived graph | Codex deep dive |
| Jan 2025 | **Subagents are distinct specialized agents** | Context isolation and task optimization | Harness deep dive |
| Jan 2025 | **Documentation as output mode, not agent** | Cross-cutting concern across agents | Harness deep dive |
| Jan 2025 | **Incident Agent separate** | Distinct context (runbooks, alerts), urgency mode | Harness deep dive |
| Jan 2025 | **Roles emphasize, never restrict** | User agency; all agents available | Harness deep dive |
| Jan 2025 | **Harness persistence separate from AEF** | Keep Codex clean; working memory is ephemeral | Harness deep dive |
| Jan 2025 | **Harness extracts entities, CAL receives structured** | Single interpretation, clear audit trail | Harness deep dive |
| Jan 2025 | **Experiment Service is separate async** | Long-running experiments should not block | Harness deep dive |
| Jan 2025 | **Kanban via external tools (Jira, Linear)** | Integration not ownership | Harness deep dive |
| Jan 2025 | **Layered significance scoring** | Heuristics + user override + selective LLM | Harness deep dive |
| Jan 2025 | **Retrieval failure: warn and wait** | User may have context or guidance | Harness deep dive |
| Jan 2025 | **Audit trail as flight recorder** | Full traceability: failures, pivots, thinking | Harness deep dive |
| Jan 2025 | **CAL policies via YAML** | Declarative configuration | Harness deep dive |
| Jan 2025 | **Contribution two-tier (prompted/auto)** | Prevent Codex pollution; ensure quality | Harness deep dive |
| Jan 2025 | **Policy hierarchy (platform → team)** | Organizational standards with team flexibility | Harness deep dive |
| Jan 2025 | **Build on Claude Code Tasks, not replace** | Native dependencies, cross-session broadcast, file persistence | v0.5 architecture revision |
| Jan 2025 | **Org DNA implemented as Claude Code Skills** | Portable format, native loading, ecosystem compatibility | v0.5 architecture revision |
| Jan 2025 | **Absorb Claude Harness into Claude Code primitives** | Subagents, Tasks, Sessions provide orchestration | v0.5 architecture revision |
| Jan 2025 | **AEF as hook layer, not wrapper** | PreTaskStart, PostTaskComplete, OnTaskFail injection points | v0.5 architecture revision |
| Jan 2025 | **Roles preserved via Skills** | Semantic layer over behavioral Skills | v0.5 architecture revision |
| Jan 2025 | **Audit Trail remains (Tasks are state, not reasoning)** | Decision rationale not captured by Tasks | v0.5 architecture revision |
| Jan 2025 | **Codex standalone (no Claude Code equivalent)** | Knowledge retrieval is unique AEF value | v0.5 architecture revision |
| Jan 2025 | **Use CLAUDE_CODE_TASK_LIST_ID for project scoping** | Native cross-session state | v0.5 architecture revision |

---

## Appendix A: Glossary

| Term | Definition |
|------|------------|
| AFS | Agentic File System â€” unified namespace for context |
| AST | Abstract Syntax Tree â€” code structure representation |
| BM25 | Best Matching 25 â€” lexical search algorithm |
| CAL | Context Assembly Layer â€” context lifecycle management |
| Codex | Institutional knowledge retrieval system with hybrid search |
| Context Manifest | Record of what context was used and why |
| Contextual Retrieval | Prepending context to chunks before embedding |
| Cross-encoder | Model that jointly encodes query + document for reranking |
| Grounding | Verification that output is based on source material |
| HITL | Human-in-the-Loop |
| Hybrid Search | Combining vector similarity with lexical (BM25) search |
| NDCG | Normalized Discounted Cumulative Gain â€” ranking metric |
| Org DNA | Organizational values, examples, and behavioral norms |
| Reranking | Second-pass scoring to reorder search results |
| RRF | Reciprocal Rank Fusion â€” method to combine ranked lists |
| Subagent | Specialized agent with isolated context for specific tasks |
| Working Memory | Harness-local session state, separate from Codex |
| Flight Recorder | Audit trail capturing full orchestration decisions |
| Tasks | Claude Code native task tracking with dependencies and cross-session broadcast |
| Skills | Claude Code modular behavioral components (SKILL.md format) |
| Hook | AEF injection point into Claude Code lifecycle events |
| CLAUDE_CODE_TASK_LIST_ID | Environment variable for project-scoped task lists |

---

## Appendix B: References

1. "Everything is Context: Agentic File System Abstraction for Context Engineering" â€” arXiv:2512.05470v1
2. LangChain Context Engineering â€” https://blog.langchain.dev/context-engineering/
3. AIGNE Framework â€” https://github.com/AIGNE-io/aigne-framework
