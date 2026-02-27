#!/usr/bin/env bash
#
# run-eval.sh — Turn-key AEF evaluation runner
#
# Runs the baseline eval (Experiment 3A) across all three conditions
# (baseline, aef-minimal, aef-full) and generates a comparison report.
#
# Usage:
#   ./run-eval.sh                      # Run full eval (all conditions, 3 attempts)
#   ./run-eval.sh --quick              # Quick mode (1 attempt per condition)
#   ./run-eval.sh --condition baseline  # Run only one condition
#   ./run-eval.sh --strategy agent     # Use agent mode (Strategy C1) instead of pipe
#   ./run-eval.sh --report-only        # Skip runs, just generate report from existing data
#
# Prerequisites:
#   - ANTHROPIC_API_KEY environment variable set
#   - 'claude' CLI installed and configured (for pipe strategy)
#   - Go 1.22+ with CGO enabled (for building aef-eval)
#   - golangci-lint installed (for scoring)
#
set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CODEX_DIR="$REPO_ROOT/codex"
TASK_DIR="$SCRIPT_DIR/tasks"
SKILL_DIR="$REPO_ROOT/edi/internal/assets/skills"
LOG_DIR="$SCRIPT_DIR/eval-logs"
DB_PATH="$LOG_DIR/results.db"
EXPERIMENT="3A"
STRATEGY="pipe"
ATTEMPTS=3
CONDITION=""
REPORT_ONLY=false
MODEL=""

# =============================================================================
# Argument parsing
# =============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --quick)
            ATTEMPTS=1
            shift
            ;;
        --condition)
            CONDITION="$2"
            shift 2
            ;;
        --strategy)
            STRATEGY="$2"
            shift 2
            ;;
        --attempts)
            ATTEMPTS="$2"
            shift 2
            ;;
        --experiment)
            EXPERIMENT="$2"
            shift 2
            ;;
        --model)
            MODEL="$2"
            shift 2
            ;;
        --report-only)
            REPORT_ONLY=true
            shift
            ;;
        --log-dir)
            LOG_DIR="$2"
            DB_PATH="$LOG_DIR/results.db"
            shift 2
            ;;
        --help|-h)
            sed -n '2,/^$/s/^# //p' "$0"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# =============================================================================
# Validation
# =============================================================================

check_prereqs() {
    local missing=()

    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        missing+=("ANTHROPIC_API_KEY environment variable")
    fi

    if [[ "$STRATEGY" == "pipe" ]] && ! command -v claude &>/dev/null; then
        missing+=("'claude' CLI (install from https://docs.anthropic.com/claude-code)")
    fi

    if ! command -v go &>/dev/null; then
        missing+=("Go 1.22+ (https://go.dev/dl/)")
    fi

    if ! command -v golangci-lint &>/dev/null; then
        echo "Warning: golangci-lint not found — lint scoring will be skipped"
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        echo "ERROR: Missing prerequisites:"
        for m in "${missing[@]}"; do
            echo "  - $m"
        done
        exit 1
    fi
}

# =============================================================================
# Build
# =============================================================================

build_eval() {
    echo "==> Building aef-eval..."
    cd "$CODEX_DIR"
    go build -tags "fts5" -o "$LOG_DIR/aef-eval" ./cmd/aef-eval/
    echo "    Built: $LOG_DIR/aef-eval"
}

# =============================================================================
# Run conditions
# =============================================================================

run_condition() {
    local condition="$1"
    local attempt

    echo ""
    echo "==> Running condition: $condition ($ATTEMPTS attempt(s))"
    echo "    Strategy: $STRATEGY | Experiment: $EXPERIMENT | Tasks: $TASK_DIR"
    echo ""

    local model_flag=""
    if [[ -n "$MODEL" ]]; then
        model_flag="--model $MODEL"
    fi

    local skill_flag=""
    if [[ "$condition" != "baseline" ]]; then
        skill_flag="--skill-dir $SKILL_DIR"
    fi

    for attempt in $(seq 1 "$ATTEMPTS"); do
        echo "--- Attempt $attempt/$ATTEMPTS ---"
        "$LOG_DIR/aef-eval" run \
            --experiment "$EXPERIMENT" \
            --condition "$condition" \
            --strategy "$STRATEGY" \
            --task-dir "$TASK_DIR" \
            --log-dir "$LOG_DIR" \
            --db "$DB_PATH" \
            --attempt "$attempt" \
            $skill_flag \
            $model_flag \
            || echo "    Warning: some tasks may have failed (continuing)"
    done
}

# =============================================================================
# Report
# =============================================================================

generate_report() {
    echo ""
    echo "==> Generating report..."
    echo ""

    "$LOG_DIR/aef-eval" report \
        --experiment "$EXPERIMENT" \
        --db "$DB_PATH" \
        --format text

    echo ""
    echo "==> Generating JSON report..."
    "$LOG_DIR/aef-eval" report \
        --experiment "$EXPERIMENT" \
        --db "$DB_PATH" \
        --format json > "$LOG_DIR/report-${EXPERIMENT}.json"

    echo "    Text report: printed above"
    echo "    JSON report: $LOG_DIR/report-${EXPERIMENT}.json"
    echo "    Results DB:  $DB_PATH"
}

# =============================================================================
# Main
# =============================================================================

main() {
    echo "============================================================"
    echo "  AEF Evaluation Runner"
    echo "  Experiment: $EXPERIMENT"
    echo "  Strategy:   $STRATEGY"
    echo "  Attempts:   $ATTEMPTS"
    echo "  Tasks:      $TASK_DIR"
    echo "============================================================"

    if [[ "$REPORT_ONLY" == "true" ]]; then
        generate_report
        exit 0
    fi

    check_prereqs

    mkdir -p "$LOG_DIR"

    build_eval

    # List discovered tasks
    echo ""
    echo "==> Discovered tasks:"
    "$LOG_DIR/aef-eval" list --tasks --task-dir "$TASK_DIR"

    # Run conditions
    if [[ -n "$CONDITION" ]]; then
        # Run single condition
        run_condition "$CONDITION"
    else
        # Run all three conditions for a complete comparison
        run_condition "baseline"
        run_condition "aef-minimal"
        run_condition "aef-full"
    fi

    # Generate report
    generate_report

    echo ""
    echo "============================================================"
    echo "  Evaluation complete!"
    echo ""
    echo "  To re-generate reports:"
    echo "    $0 --report-only"
    echo ""
    echo "  To run additional attempts:"
    echo "    $0 --condition baseline --attempts 2"
    echo ""
    echo "  To view all runs:"
    echo "    $LOG_DIR/aef-eval list --runs --db $DB_PATH"
    echo "============================================================"
}

main
