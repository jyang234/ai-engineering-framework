# AEF Production Readiness Assessment

> **Date**: March 6, 2026
> **Scope**: Viability for high-quality engineering on a small team (2-5 engineers)
> **Focus areas**: RECALL features, spec-first workflow, overall maturity

---

## Verdict

**Genuinely viable for a small team that uses Claude Code as its primary AI coding tool.** The core retrieval loop is production-quality. Known limitations are well-documented and appropriate for the project's stage.

---

## Strengths

### 1. Hybrid Search Engine (Production-Quality)

The search pipeline in `codex/internal/core/engine.go` implements a 10-step retrieval process:

1. Embed query (graceful degradation if unavailable)
2. Vector cosine similarity search
3. FTS5 BM25 keyword search
4. 2-way RRF fusion (vector + keywords)
5. Metadata hydration
6. Type/scope filtering
7. Optional reranking
8. Score threshold cutoff
9. Result limiting
10. Degradation marking

Key engineering details:
- Compensating transactions on write (rollback metadata if vector store fails)
- Graceful fallback to keyword-only when embeddings are unavailable
- Configurable score thresholds and candidate limits
- Clean interface-based dependency injection (`SearchEngineDeps`) for testability

### 2. Spec-First Discipline

- 45+ architecture and implementation docs in `docs/`
- Git history shows specs written before code (e.g., `Add spec for FTS5 search quality improvements` → `Improve FTS5 search quality`)
- Gap analysis document (`edi-implementation-gaps-analysis.md`) honestly tracks resolved vs. open items
- Architecture Decision Records for key choices (storage, embedding models, guard policies)

### 3. Test Coverage

- 47 test files across 155 Go source files (~30% test file ratio)
- Tests at every layer: storage, core engine, MCP server, eval harness, chunking, config, guard policies, CLI, integration
- Eval framework (`codex/eval/`) includes ground-truth test data, LLM judge metrics, and MCP round-trip tests
- Integration tests for CLI and session lifecycle

### 4. Pragmatic Architecture

| Decision | Rationale |
|----------|-----------|
| Go single binary | No container orchestration needed |
| SQLite for everything | No Postgres/Redis to manage |
| Local Ollama embeddings | No API key dependency for core features |
| `syscall.Exec` launch | EDI configures then gets out of the way |
| Clean module separation | EDI = session harness, Codex = knowledge backend |

### 5. Agent/Skill System

- 4 agents (coder, architect, reviewer, incident)
- 8 skills (coding, testing, golang-idioms, plan-review, etc.)
- 9 slash commands (/plan, /build, /review, /incident, /task, /end, /ralph, /review-plan, /end-recovery)
- 7 subagents (debugger, doc-writer, implementer, researcher, reviewer, test-writer, web-researcher)
- All version-controlled markdown with YAML frontmatter, embedded via `go:embed`

---

## Limitations

### 1. Stubbed Features

Per `docs/aef-components.md`:
- **Multi-stage reranking** (BGE models via ONNX): stub falls back to original ordering
- **Contextual retrieval** (Claude Haiku enrichment): stub returns error, not implemented

Impact: Low. The FTS5 + vector + RRF pipeline delivers good results without these.

### 2. Ollama Dependency

Codex v1 requires local Ollama with `nomic-embed-text` for vector search. Mitigation: search gracefully degrades to FTS5-only. The v0 backend (pure FTS5) works with zero external dependencies.

### 3. Planned Components Not Yet Implemented

- **Learning** (knowledge quality assurance): planned
- **Sandbox** (disposable dev environments): planned

### 4. No CI/CD Pipeline

No `.github/workflows/` found. Tests must be run locally. Acceptable for a small team but means no automated quality gate.

### 5. Single-User Design

SQLite-based storage without multi-user conflict resolution. Fine for solo use or a team with separate project knowledge bases.

---

## Recommendations

### Ideal For

- 1-3 person teams heavily using Claude Code
- Teams that value accumulating project knowledge across sessions
- Teams practicing spec-first development
- Go-comfortable teams that can extend the framework

### Less Suitable For

- Teams needing concurrent multi-user knowledge bases
- Remote-only environments without local Ollama (though v0 FTS5 still works)
- Teams expecting zero-maintenance tooling

### The Spec-First Workflow

The `/plan` → `/review-plan` → `/build` → `/review` pipeline, combined with RECALL search enriching task context, creates a workflow where architectural decisions get recorded and retrieved in future sessions. For a small team without a dedicated architect, this is high-value — it acts as institutional memory.

---

## Summary

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Core retrieval | Strong | Hybrid search with graceful degradation |
| Code quality | Strong | Clean Go, proper error handling, interface-based DI |
| Test coverage | Good | 30% test file ratio, multi-layer coverage |
| Documentation | Strong | Spec-first with honest gap tracking |
| Deployment | Good | Single binary + SQLite, minimal ops burden |
| Agent system | Good | Well-structured, version-controlled prompts |
| Maturity | Early-production | Core works; some stubs and planned features remain |
| Team fit | Small team (1-5) | Single-user SQLite, no CI/CD, requires Go familiarity |
