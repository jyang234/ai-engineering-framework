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
# Experiment 3B: Paired tasks for repeat failure prevention
# =============================================================================

run_3b_pair() {
    local source="$1"
    local target="$2"
    local pair_id="$3"
    local attempt="$4"

    echo "  Pair: $source → $target ($pair_id)"

    local model_flag=""
    if [[ -n "$MODEL" ]]; then
        model_flag="--model $MODEL"
    fi

    # Run target under baseline (no RECALL)
    local run_id_base="3B-baseline-${target}-${attempt}"
    "$LOG_DIR/aef-eval" run \
        --experiment "3B" \
        --condition "baseline" \
        --strategy "$STRATEGY" \
        --task-dir "$TASK_DIR" \
        --log-dir "$LOG_DIR" \
        --db "$DB_PATH" \
        --attempt "$attempt" \
        $model_flag \
        || echo "    Warning: baseline run failed for $target"

    # Run target under aef-full (RECALL seeded with source's pitfall knowledge)
    "$LOG_DIR/aef-eval" run \
        --experiment "3B" \
        --condition "aef-full" \
        --strategy "${3B_STRATEGY:-agent}" \
        --task-dir "$TASK_DIR" \
        --log-dir "$LOG_DIR" \
        --db "$DB_PATH" \
        --attempt "$attempt" \
        --skill-dir "$SKILL_DIR" \
        $model_flag \
        || echo "    Warning: aef-full run failed for $target"
}

run_experiment_3b() {
    echo ""
    echo "==> Running Experiment 3B: Repeat Failure Prevention"
    echo "    $ATTEMPTS attempt(s) per pair"
    echo ""

    # 5 pairs from pairs.yaml (hardcoded for reliability)
    local -a PAIRS=(
        "worker-pool:pipeline:goroutine-lifecycle"
        "batch-processor:stream-aggregator:flush-on-shutdown"
        "lru-cache:request-coalescer:concurrent-map-access"
        "circuit-breaker:task-scheduler:state-machine-sync"
        "retry-backoff:rate-limiter:context-aware-wait"
    )

    for attempt in $(seq 1 "$ATTEMPTS"); do
        echo "--- 3B Attempt $attempt/$ATTEMPTS ---"
        for pair in "${PAIRS[@]}"; do
            IFS=':' read -r source target pair_id <<< "$pair"
            run_3b_pair "$source" "$target" "$pair_id" "$attempt"
        done
    done
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

    case "$EXPERIMENT" in
        3A)
            # Run conditions
            if [[ -n "$CONDITION" ]]; then
                run_condition "$CONDITION"
            else
                run_condition "baseline"
                run_condition "aef-minimal"
                run_condition "aef-full"
            fi
            ;;
        3B)
            run_experiment_3b
            ;;
        all)
            # Run both experiments
            EXPERIMENT="3A"
            if [[ -n "$CONDITION" ]]; then
                run_condition "$CONDITION"
            else
                run_condition "baseline"
                run_condition "aef-minimal"
                run_condition "aef-full"
            fi
            EXPERIMENT="3B"
            run_experiment_3b
            ;;
        *)
            # Default: run as 3A
            if [[ -n "$CONDITION" ]]; then
                run_condition "$CONDITION"
            else
                run_condition "baseline"
                run_condition "aef-minimal"
                run_condition "aef-full"
            fi
            ;;
    esac

    # Generate report
    generate_report

    # If we ran both, also generate the 3B report
    if [[ "$EXPERIMENT" == "all" ]]; then
        EXPERIMENT="3B"
        generate_report
    fi

    echo ""
    echo "============================================================"
    echo "  Evaluation complete!"
    echo ""
    echo "  Experiments available:"
    echo "    ./run-eval.sh --experiment 3A              # Defect rate comparison (15 tasks x 3 conditions)"
    echo "    ./run-eval.sh --experiment 3B              # Repeat failure prevention (5 pairs x 2 conditions)"
    echo "    ./run-eval.sh --experiment all             # Both experiments"
    echo "    ./run-eval.sh --experiment 3A --quick      # Quick smoke test (1 attempt)"
    echo ""
    echo "  To re-generate reports:"
    echo "    $0 --report-only"
    echo ""
    echo "  To view all runs:"
    echo "    $LOG_DIR/aef-eval list --runs --db $DB_PATH"
    echo "============================================================"
}

main
