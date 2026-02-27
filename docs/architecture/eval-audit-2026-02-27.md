# Evaluation System Audit

> Date: 2026-02-27
> Scope: Full audit of test coverage across EDI, codex, and the eval infrastructure
> Question: "Are the tools built in a way that allows the craftsman to use them effectively?"

---

## Executive Summary

AEF has two kinds of tests:

1. **Component/integration tests** that verify the actual tools work (EDI launch, RECALL search, MCP protocol, config loading). These exist, mostly pass, and prove the tools are well-built.

2. **An L3 eval corpus** (8,295 lines of infrastructure + 5,165 lines of task corpus) that tests whether Claude writes better code with pitfall knowledge in its prompt. This answers a prompting question, not a tools question, and is overbuilt for its purpose.

The real gaps are not in the eval corpus — they're in untested production components: the MCP server has zero tests, embeddings have zero tests, hooks have zero tests, and the add→search round-trip across sessions is never verified end-to-end.

---

## What's Tested and Working

### Tier 1: Real E2E tests that prove the tools work

| Component | Test Location | What It Proves | Dependencies |
|---|---|---|---|
| RECALL retrieval quality | `codex/eval/harness_test.go` (TestE2E) | Hybrid search returns relevant results from 30-doc PayFlow corpus. Recall@10 ≥ 0.3, MRR ≥ 0.2. | Ollama (local) |
| LLM judge filtering | `codex/eval/judge_test.go` (TestJudge) | Judge improves precision ≥ 0.15 over raw retrieval | Anthropic API |
| RRF fusion algorithm | `codex/internal/core/fusion_test.go` (7 tests) | Items in both vector + keyword lists rank highest. Score formula correct. | None |
| IR metrics math | `codex/eval/metrics_test.go` (5 tests) | Recall@K, Precision@K, nDCG, MRR compute correctly | None |
| Judge metrics math | `codex/eval/judge_metrics_test.go` (18 tests) | Precision, recall, F1, false filtering rate compute correctly | None |
| SQLite storage | `codex/internal/storage/` (28 tests) | Items persist, vectors search by cosine similarity, dimensions validated | SQLite |

**Benchmark results** (from actual runs):
- nomic-embed-text (local): Recall@5: 0.83, nDCG@10: 0.78, MRR: 0.81
- voyage-code-3 (API): Recall@5: 0.86, nDCG@10: 0.79, MRR: 0.82

### Tier 2: EDI unit/integration tests — solid foundation

| Package | Tests | What's Verified |
|---|---|---|
| `edi/internal/launch/` | 13 tests | Slash command install (SHA256 dedup), MCP config generation (V0/codex), stale session detection |
| `edi/internal/cli/` | 26 tests | Context building (with/without RECALL), project init, doctor checks, ralph loop |
| `edi/internal/config/` | 8 tests | Config defaults, YAML serialization, language detection |
| `edi/internal/tasks/` | 20 tests | Manifest CRUD, task sync on launch/hook, session cleanup, legacy migration |
| `edi/internal/briefing/` | 10 tests | Briefing generation, history save/load, flight recorder file I/O |
| `edi/internal/agents/` | 6 tests | Agent .md parsing (YAML frontmatter + body), skill language filtering |
| `edi/internal/recall/` | 6 tests | V0 SQLite storage, MCP protocol messages, FTS5 search, multi-scope |
| `edi/internal/integration/` | 9 tests | EDI init E2E, RECALL server startup/shutdown, context building, briefing render |
| `edi/internal/codex/` | 5 tests | Binary detection, source detection |

**Total EDI**: 108 test functions, 5,024 lines of test code across 20 test files.

**Known failures**: `TestParseSkillFile` has 3 failing sub-tests (content trimming bug in skill parsing).

### Tier 3: Codex search engine — tested but heavily mocked

| Component | Tests | What's Verified | What's Mocked |
|---|---|---|---|
| Search engine | 13 tests | Result sorting, type/scope filtering, score thresholds, limit enforcement | Embedder, vector store, keyword searcher |
| Indexer | 20 tests | Content type detection, title extraction, document chunking, type routing | Embedder, vector store, chunker |
| Migration | 20 tests | Type mapping, stats tracking, DB validation, item parsing | SearchEngine (actual migration skipped) |

These tests prove the **logic** is correct but not that the **integrations** work. The mocks hide real-world issues (Ollama timeouts, FTS5 query edge cases, embedding dimension mismatches).

---

## What's Not Tested (Real Gaps)

### Critical: Zero test coverage

| Component | Location | Advertised Value | Impact |
|---|---|---|---|
| **MCP server** | `codex/internal/mcp/` | JSON-RPC protocol interface between Claude Code and RECALL | Claude Code talks to RECALL exclusively through this. No test for request handling, tool dispatch, error codes, parameter validation, or size limits. |
| **Embeddings** | `codex/internal/embedding/` | Vector embeddings for semantic search | Always mocked in tests. Ollama connectivity, dimension handling, error recovery untested. |
| **Chunking** | `codex/internal/chunking/` | Code/doc splitting for indexing | Always mocked. Real chunking behavior untested. |
| **Reranking** | `codex/internal/reranking/` | Cross-encoder precision improvement | Zero tests. |
| **Hooks** | (no test file) | "Enforce mechanical standards" — gofumpt on Write/Edit, protect-files | Zero tests that hooks fire, produce correct output, or interact with Claude Code correctly. |

### Significant: Partial coverage hiding integration gaps

| Gap | What's Missing | Why It Matters |
|---|---|---|
| **Add→search round-trip** | No test that `recall_add` stores an item and `recall_search` finds it across sessions | This is the core RECALL value proposition — knowledge persists and is retrievable. PayFlow tests this within one session but never across sessions. |
| **Real FTS5 keyword search** | Engine tests mock the keyword searcher | The "hybrid" in hybrid search is never tested end-to-end except in the PayFlow harness. |
| **Agent discovery** | `Load(name)` and `ListAgents()` untested | Project-override-global resolution never verified. |
| **8/15 EDI CLI commands** | config, history, recall, agent, sync, version, flight-log, root launch | Command execution paths not validated. |
| **`/end` workflow** | No test for knowledge curation, status.md update, history write | The session capture pipeline — the mechanism for knowledge accumulation — is never tested. |

---

## What's Overbuilt (L3 Eval Corpus Infrastructure)

### The numbers

| Component | Source Lines | Test Lines | Test Functions |
|---|---|---|---|
| `report_l3.go` | 443 | 366 | 16 |
| `scorer.go` | 590 | 778 | 35 |
| `condition.go` | 216 | 662 | 29 |
| `runner_agent.go` | 553 | 432 | 25 |
| `runner_pipe.go` | 401 | 505 | 30 |
| `agent_tools.go` | 505 | 642 | 34 |
| `results.go` | 440 | 546 | 14 |
| `stats.go` | 398 | 509 | 34 |
| `mcpclient.go` | 309 | — | — |
| **Subtotal (infra)** | **3,855** | **4,440** | **217** |
| 15-task corpus | ~5,165 | — | — |
| **Total L3** | **~9,020** | **4,440** | **217** |

### What this infrastructure does

It builds a synthetic agent that mimics Claude Code, runs it through 15 Go coding tasks under 3 conditions (baseline / skills-only / skills+RECALL), grades the output, stores results in SQLite, runs statistical tests (Mann-Whitney U, Bootstrap CI, Wilcoxon, Spearman), and generates evidence reports validating 7 claims.

### What it actually tests

Whether the Claude model produces better code when pitfall knowledge is included in its prompt. The synthetic agent is not Claude Code, hooks don't fire, RECALL items are pre-seeded (not organically accumulated), and the search engine is never queried by the agent's own search formulation.

### What it would take to answer the same question without this infrastructure

A script that runs `claude -p` on each task twice — with and without pitfall text appended — then runs `go test -race` and greps for anti-patterns. ~200 lines of bash.

### The 217 test functions

These test the infrastructure's own internals: "does the scorer correctly compute a weighted score?" "does the condition configurator produce the right tool list?" "does the results DB insert and query correctly?" They do not test whether EDI or codex work. They test whether the eval framework's mocks behave as expected.

---

## Recommendations

### What to fix (actual tool quality)

**Priority 1 — Fill the zero-coverage gaps:**

1. **MCP server tests** (`codex/internal/mcp/server_test.go`): Test all 5 tool handlers, JSON-RPC protocol, error codes, parameter validation, environment variable injection.

2. **Add→search round-trip test**: Add an item via `recall_add`, then `recall_search` with a relevant query, assert the item appears in results. This is the one test that proves RECALL's core value loop.

3. **Hook execution test**: Configure a gofumpt hook, trigger a Write event, verify the file was formatted. This proves hooks work as advertised.

4. **Fix `TestParseSkillFile`**: 3 sub-tests failing due to content trimming. Skills with language filtering may not parse correctly.

**Priority 2 — Strengthen integration coverage:**

5. **Real embedding test**: Call Ollama with a known query, verify the embedding has correct dimensions and is deterministic.

6. **`/end` workflow test**: Simulate the curation flow — write to `/memories/`, call `recall_add`, verify items persist, verify `status.md` updated.

7. **Untested CLI commands**: Add basic tests for the 8 untested EDI commands.

8. **Search engine integration test**: Run a query through real FTS5 + real embeddings (not mocks) with the PayFlow corpus.

### What to do with the L3 infrastructure

**Option A: Keep it, relabel it.** It's a valid prompting effectiveness study. Call it that. Don't claim it proves the tools work.

**Option B: Reduce it.** Strip the synthetic agent, statistical suite, and results DB. Keep the 15 task corpus (it's well-crafted). Replace the infrastructure with a simple `claude -p` runner that checks test pass rate and anti-pattern presence. ~200 lines replaces ~8,300.

**Option C: Repurpose it.** Use the synthetic agent runner as a test harness for the real MCP server. Instead of testing "does knowledge improve code?", test "does the search engine return the right items when an agent formulates its own queries?" This would be a genuine test of the search tool's usability.

### What the test suite should look like

```
Tests that prove the tools work:
├── codex/internal/mcp/server_test.go        ← NEW: MCP protocol + all 5 tools
├── codex/internal/core/engine_test.go        ← EXISTS: needs real embedding integration
├── codex/internal/core/fusion_test.go        ← EXISTS: solid
├── codex/internal/storage/*_test.go          ← EXISTS: solid
├── codex/eval/harness_test.go                ← EXISTS: PayFlow E2E (Recall, nDCG, MRR)
├── codex/eval/judge_test.go                  ← EXISTS: LLM judge quality
├── codex/eval/metrics_test.go                ← EXISTS: IR math
├── codex/eval/roundtrip_test.go              ← NEW: add→search across sessions
├── edi/internal/launch/*_test.go             ← EXISTS: config, commands, MCP, recovery
├── edi/internal/cli/*_test.go                ← EXISTS: needs 8 more command tests
├── edi/internal/recall/*_test.go             ← EXISTS: server + integration
├── edi/internal/tasks/*_test.go              ← EXISTS: manifest + sync
├── edi/internal/agents/*_test.go             ← EXISTS: needs fix + discovery tests
├── edi/internal/integration/*_test.go        ← EXISTS: E2E session lifecycle
└── edi/hooks_test.go                         ← NEW: hook execution verification

Tests that prove prompting helps (optional, relabel):
└── codex/eval/corpus/                        ← EXISTS: 15 tasks, keep or simplify
```

---

## Appendix: Test Execution

```bash
# EDI tests (1 failure in skill parsing)
cd edi && go test -race -tags "fts5" ./...

# Codex core tests (pass)
cd codex && go test -race -tags "fts5" ./internal/core/... ./internal/storage/...

# Codex eval L1 tests (need Ollama)
cd codex && go test -tags "fts5 evalintegration" ./eval/... -run TestE2E -timeout 15m

# Codex eval L2 tests (need Ollama + Anthropic API key)
cd codex && go test -tags "fts5 evalintegration" ./eval/... -run TestJudge -timeout 30m

# Codex eval L3 tests (test own infrastructure mocks only)
cd codex && go test -tags "fts5" ./eval/... -timeout 5m

# Packages with ZERO test coverage
# codex/internal/mcp/       — MCP server
# codex/internal/embedding/  — Ollama embeddings
# codex/internal/chunking/   — Code/doc chunking
# codex/internal/reranking/  — Cross-encoder reranking
```
