Last updated: 2026-02-26

## Current Milestone
EDI v0 complete, Codex v1 substantially complete, evaluation framework infrastructure ready. Iterating on code quality and preparing for experiment data collection.

## Completed
- EDI CLI with init, launch, config, recall, history, agent, doctor, sync, ralph, version commands
- RECALL v0 MCP server with FTS5 search
- 4 core agents (coder, architect, reviewer, incident) + 7 subagents
- 9 slash commands: /plan, /build, /review, /review-plan, /incident, /task, /ralph, /end, /end-recovery
- Briefing generation from profile/history/tasks/status
- Task annotations system
- Stale session detection + /end-recovery command
- Enriched RECALL items: auto-injected session/git metadata + structured content templates
- 7 skills: edi-core, retrieval-judge, coding, testing, scaffolding-tests, refactoring-planning, plan-review (all embedded + installed)
- `edi sync` command for non-destructive asset updates
- `make sync` and `make reinstall` Makefile targets
- Auto memory integration (MEMORY.md co-management with Claude Code)
- Ralph autonomous loop (bash script, PRD-driven, escalation protocol)
- Codex v1: hybrid search engine (vector KNN + FTS5 BM25 + RRF fusion)
- Codex v1: MCP server with 5 tools (recall_search, recall_get, recall_add, recall_feedback, flight_recorder_log)
- Codex v1: AST chunking via Tree-sitter (Go, Python, TypeScript, JavaScript, TSX, JSX)
- Codex v1: markdown chunking, web UI (Gin), admin CLI, v0-to-v1 migration
- Codex v1: nomic-embed-text embeddings via Ollama (768-dim, asymmetric prefixes)
- Evaluation framework Level 1: EvalHarness (8-phase MCP + retrieval quality)
- Evaluation framework Level 2-4: PipeRunner, AgentRunner, Scorer (tests + lint + LLM judge)
- Evaluation framework: statistical analysis (Mann-Whitney U, Bootstrap CI, Wilcoxon, Spearman)
- Evaluation framework: L3 report generation (condition comparisons, claim validation, scorecards)
- Evaluation framework: `aef-eval` CLI (run, score, report, list)
- Evaluation framework: ResultsDB for structured run storage
- Evaluation framework: condition system (baseline, aef-minimal, aef-full)
- Effective Go violations fixed across edi/ and codex/ directories
- Documentation updated: README, deep dive, CLAUDE.md

## Stubs (Not Functional)
- Reranking layer (BGE model integration via Hugot — stub returns passthrough scores)
- Contextual chunking (Claude Haiku enrichment — stub falls back to basic chunking)

## Next Steps
- Collect experiment data: run aef-eval across task corpus to produce baseline vs. AEF comparison results
- Author task corpus for controlled experiments (YAML task specs + pitfall definitions)
- Validate statistical pipeline with real experiment data
- Add more comprehensive unit and integration tests
- Investigate reranking implementation (ONNX model loading)
