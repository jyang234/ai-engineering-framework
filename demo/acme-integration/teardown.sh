#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Tearing down Acme Integration Demo ==="

# ── Demo git repo ─────────────────────────────────────────────────────────
rm -rf .git
echo "✓ Removed demo .git"

# ── Runtime / generated directories ───────────────────────────────────────
rm -rf .demo .ralph .claude .scaffold
echo "✓ Removed .demo/, .ralph/, .claude/, .scaffold/"

# ── EDI generated files (preserve profile.md only) ───────────────────────
rm -f .edi/config.yaml .edi/status.md
rm -rf .edi/history .edi/tasks .edi/recall .edi/cache
echo "✓ Reset .edi/ (profile.md preserved)"

# ── Claude Code / MCP generated files ────────────────────────────────────
rm -f .mcp.json
echo "✓ Removed .mcp.json"

# ── src/ — remove all generated code ─────────────────────────────────────
rm -rf src/*
echo "✓ Emptied src/"

# ── docs/PRD.json — Ralph modifies task states; remove the copy ──────────
rm -f docs/PRD.json
rm -f docs/adr-*.md docs/data-contract-*.yaml docs/meeting-notes-*.md
echo "✓ Removed docs/PRD.json"

# ── Root-level generated files ────────────────────────────────────────────
rm -f PRD.json .gitignore go.work go.work.sum
echo "✓ Removed PRD.json, .gitignore, go.work"

# ── Demo RECALL items (project-local Codex DB) ──────────────────────────
rm -f .edi/codex.db .edi/codex.db-wal .edi/codex.db-shm
echo "✓ Removed demo Codex DB"

echo ""
echo "=== Teardown Complete ==="
echo ""
echo "Remaining files are the static demo scaffold."
echo "Run ./setup.sh to start fresh."
