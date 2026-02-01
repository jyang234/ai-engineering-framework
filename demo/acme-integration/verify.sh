#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RESULTS_FILE=".demo/results.json"
CHECKPOINTS_DIR="checkpoints"

# Ensure jq is available
if ! command -v jq &>/dev/null; then
    echo "ERROR: jq is required. Install with: brew install jq"
    exit 1
fi

# Ensure results file exists
mkdir -p .demo
[ -f "$RESULTS_FILE" ] || echo "[]" > "$RESULTS_FILE"

# ─── Helpers ───────────────────────────────────────────────────────────────

timestamp() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

log_result() {
    local step="$1" check="$2" expected="$3" actual="$4" status="$5" notes="${6:-}"
    local entry
    entry=$(jq -n \
        --arg step "$step" \
        --arg check "$check" \
        --arg expected "$expected" \
        --arg actual "$actual" \
        --arg status "$status" \
        --arg notes "$notes" \
        --arg ts "$(timestamp)" \
        '{step: $step, check: $check, expected: $expected, actual: $actual, status: $status, notes: $notes, timestamp: $ts}')

    # Append to results array
    local tmp
    jq --argjson entry "$entry" '. += [$entry]' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp"
    mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"

    if [ "$status" = "pass" ]; then
        echo "  ✓ $check"
    else
        echo "  ✗ $check"
        echo "    expected: $expected"
        echo "    actual:   $actual"
        [ -n "$notes" ] && echo "    notes:    $notes"
    fi
}

# ─── Check implementations ────────────────────────────────────────────────

check_file_exists() {
    local step="$1" path="$2"
    if [ -e "$path" ]; then
        log_result "$step" "file_exists:$path" "exists" "exists" "pass"
    else
        log_result "$step" "file_exists:$path" "exists" "missing" "fail"
    fi
}

check_dir_not_empty() {
    local step="$1" path="$2"
    if [ -d "$path" ] && [ "$(ls -A "$path" 2>/dev/null)" ]; then
        log_result "$step" "dir_not_empty:$path" "not empty" "has contents" "pass"
    else
        log_result "$step" "dir_not_empty:$path" "not empty" "empty or missing" "fail"
    fi
}

check_file_modified() {
    local step="$1" path="$2"
    if git diff --name-only HEAD~1 2>/dev/null | grep -qF "$path"; then
        log_result "$step" "file_modified:$path" "modified" "modified in last commit" "pass"
    elif git diff --name-only 2>/dev/null | grep -qF "$path"; then
        log_result "$step" "file_modified:$path" "modified" "has unstaged changes" "pass"
    else
        log_result "$step" "file_modified:$path" "modified" "no changes detected" "fail"
    fi
}

check_recall_contains() {
    local step="$1" query="$2" min_results="${3:-1}" expect_title="${4:-}"
    local output
    output=$(edi recall search "$query" 2>/dev/null || echo "")
    local count
    count=$(echo "$output" | grep -c "^" || echo "0")
    if [ -n "$expect_title" ]; then
        if echo "$output" | grep -qF "$expect_title"; then
            log_result "$step" "recall_contains:$query (title=$expect_title)" ">= $min_results results with title" "found" "pass"
        else
            log_result "$step" "recall_contains:$query (title=$expect_title)" "title match" "not found" "fail"
        fi
        return
    fi
    if [ "$count" -ge "$min_results" ]; then
        log_result "$step" "recall_contains:$query" ">= $min_results results" "$count results" "pass"
    else
        log_result "$step" "recall_contains:$query" ">= $min_results results" "$count results" "fail"
    fi
}

check_git_commits_since() {
    local step="$1" min="$2"
    # Count commits since the initial demo commit
    local count
    count=$(git rev-list --count HEAD 2>/dev/null || echo "0")
    # Subtract 1 for the initial setup commit
    count=$((count - 1))
    [ "$count" -lt 0 ] && count=0
    if [ "$count" -ge "$min" ]; then
        log_result "$step" "git_commits_since:$min" ">= $min new commits" "$count commits" "pass"
    else
        log_result "$step" "git_commits_since:$min" ">= $min new commits" "$count commits" "fail"
    fi
}

check_prd_tasks_complete() {
    local step="$1" prd_path="$2" min_complete="$3"
    if [ ! -f "$prd_path" ]; then
        log_result "$step" "prd_tasks_complete" ">= $min_complete" "PRD not found" "fail"
        return
    fi
    local complete
    complete=$(jq '[.userStories[] | select(.passes == true)] | length' "$prd_path")
    if [ "$complete" -ge "$min_complete" ]; then
        log_result "$step" "prd_tasks_complete:$min_complete" ">= $min_complete complete" "$complete complete" "pass"
    else
        log_result "$step" "prd_tasks_complete:$min_complete" ">= $min_complete complete" "$complete complete" "fail"
    fi
}

check_llm_judge() {
    local step="$1" prompt="$2" expected="$3"
    if ! command -v claude &>/dev/null; then
        log_result "$step" "llm_judge" "$expected" "claude CLI not available" "skip" "Install Claude Code to enable LLM judge checks"
        return
    fi

    echo "  ⏳ Running LLM judge: $prompt"
    local ruling
    local schema='{"type":"object","properties":{"ruling":{"type":"string","enum":["pass","fail"]},"reasoning":{"type":"string"},"evidence":{"type":"string"}},"required":["ruling","reasoning"]}'
    ruling=$(claude -p "You are a verification judge for the Acme Integration demo (step: $step).

Question: $prompt

Expected: $expected

Evaluate based on the project files and recent git history in this directory. Be generous — if there's reasonable evidence the criterion was met, rule pass." \
        --output-format json --json-schema "$schema" 2>/dev/null \
        || echo '{"result":"","cost_usd":0}')

    # --output-format json returns envelope with .structured_output containing the schema result
    local inner
    inner=$(echo "$ruling" | jq -c '.structured_output // empty' 2>/dev/null)
    if [ -n "$inner" ] && [ "$inner" != "null" ]; then
        ruling="$inner"
    fi
    [ -z "$ruling" ] && ruling='{"ruling":"error","reasoning":"no JSON in response"}'

    local result
    result=$(echo "$ruling" | jq -r '.ruling' 2>/dev/null || echo "error")
    local reasoning
    reasoning=$(echo "$ruling" | jq -r '.reasoning' 2>/dev/null || echo "parse error")

    if [ "$result" = "pass" ]; then
        log_result "$step" "llm_judge:${prompt:0:50}..." "$expected" "LLM ruled pass: $reasoning" "llm_pass"
        echo "    LLM ruled PASS: $reasoning"
        read -r -p "    Confirm? [Y/n/notes] " confirm
        case "$confirm" in
            n|N) log_result "$step" "llm_judge_override" "human override" "human overrode to fail" "overridden_fail" ;;
            ""|y|Y) ;; # already logged as llm_pass
            *) log_result "$step" "llm_judge_note" "human note" "$confirm" "llm_pass" "$confirm" ;;
        esac
    else
        log_result "$step" "llm_judge:${prompt:0:50}..." "$expected" "LLM ruled $result: $reasoning" "llm_fail"
        echo "    LLM ruled FAIL: $reasoning"
        read -r -p "    Override to pass? [y/N/notes] " confirm
        case "$confirm" in
            y|Y) log_result "$step" "llm_judge_override" "human override" "human overrode to pass" "overridden_pass" ;;
            *) ;; # already logged as llm_fail
        esac
    fi
}

# ─── Run checks for a step ────────────────────────────────────────────────

run_step() {
    local step="$1"
    local checkpoint_file="$CHECKPOINTS_DIR/${step}.json"

    if [ ! -f "$checkpoint_file" ]; then
        echo "ERROR: No checkpoint file for step '$step'"
        echo "Available steps:"
        ls "$CHECKPOINTS_DIR"/*.json 2>/dev/null | sed 's|.*/||;s|\.json||'
        exit 1
    fi

    local desc
    desc=$(jq -r '.description' "$checkpoint_file")
    echo ""
    echo "── Step: $step — $desc ──"

    local checks
    checks=$(jq -c '.checks[]' "$checkpoint_file")

    while IFS= read -r check; do
        local type
        type=$(echo "$check" | jq -r '.type')

        case "$type" in
            file_exists)
                check_file_exists "$step" "$(echo "$check" | jq -r '.path')"
                ;;
            dir_not_empty)
                check_dir_not_empty "$step" "$(echo "$check" | jq -r '.path')"
                ;;
            file_modified)
                check_file_modified "$step" "$(echo "$check" | jq -r '.path')"
                ;;
            recall_contains)
                check_recall_contains "$step" \
                    "$(echo "$check" | jq -r '.query')" \
                    "$(echo "$check" | jq -r '.min_results // 1')"
                ;;
            git_commits_since)
                check_git_commits_since "$step" "$(echo "$check" | jq -r '.min')"
                ;;
            prd_tasks_complete)
                check_prd_tasks_complete "$step" \
                    "$(echo "$check" | jq -r '.prd_path')" \
                    "$(echo "$check" | jq -r '.min_complete')"
                ;;
            llm_judge)
                check_llm_judge "$step" \
                    "$(echo "$check" | jq -r '.prompt')" \
                    "$(echo "$check" | jq -r '.expected')"
                ;;
            *)
                echo "  ? Unknown check type: $type"
                ;;
        esac
    done <<< "$checks"
}

# ─── Summary generation ───────────────────────────────────────────────────

generate_summary() {
    echo ""
    echo "=== Generating RESULTS.md ==="

    local total pass fail skip
    total=$(jq 'length' "$RESULTS_FILE")
    pass=$(jq '[.[] | select(.status == "pass" or .status == "llm_pass" or .status == "overridden_pass")] | length' "$RESULTS_FILE")
    fail=$(jq '[.[] | select(.status == "fail" or .status == "llm_fail" or .status == "overridden_fail")] | length' "$RESULTS_FILE")
    skip=$(jq '[.[] | select(.status == "skip")] | length' "$RESULTS_FILE")

    cat > .demo/RESULTS.md << EOF
# Demo Results

**Run date:** $(date -u +"%Y-%m-%d %H:%M UTC")
**Total checks:** $total | **Passed:** $pass | **Failed:** $fail | **Skipped:** $skip

## Results by Step

EOF

    # Group by step
    jq -r '[.[] | .step] | unique | .[]' "$RESULTS_FILE" | while IFS= read -r step; do
        echo "### $step" >> .demo/RESULTS.md
        echo "" >> .demo/RESULTS.md
        echo "| Check | Status | Details |" >> .demo/RESULTS.md
        echo "|-------|--------|---------|" >> .demo/RESULTS.md

        jq -r --arg step "$step" '.[] | select(.step == $step) | "| \(.check) | \(.status) | \(.actual) |"' "$RESULTS_FILE" >> .demo/RESULTS.md
        echo "" >> .demo/RESULTS.md
    done

    # Failures section
    local failures
    failures=$(jq '[.[] | select(.status == "fail" or .status == "llm_fail" or .status == "overridden_fail")]' "$RESULTS_FILE")
    if [ "$(echo "$failures" | jq 'length')" -gt 0 ]; then
        echo "## Failures (Bug Backlog)" >> .demo/RESULTS.md
        echo "" >> .demo/RESULTS.md
        echo "$failures" | jq -r '.[] | "- **\(.step)/\(.check)**: expected \(.expected), got \(.actual)\(.notes | if . != "" then " — " + . else "" end)"' >> .demo/RESULTS.md
        echo "" >> .demo/RESULTS.md
    fi

    echo "✓ Written to .demo/RESULTS.md"
    echo ""
    echo "Summary: $pass/$total passed, $fail failed, $skip skipped"
}

# ─── Main ─────────────────────────────────────────────────────────────────

case "${1:-}" in
    summary)
        generate_summary
        ;;
    "")
        echo "Usage: ./verify.sh <step-number> | summary"
        echo ""
        echo "Steps:"
        ls "$CHECKPOINTS_DIR"/*.json 2>/dev/null | sed 's|.*/||;s|\.json||' | sort
        echo ""
        echo "Run './verify.sh summary' to generate RESULTS.md"
        ;;
    *)
        run_step "$1"
        ;;
esac
