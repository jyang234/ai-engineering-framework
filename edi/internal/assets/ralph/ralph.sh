#!/bin/bash
# ralph.sh — Autonomous task execution with human escalation
#
# Executes well-defined tasks from PRD.json. Escalates when stuck or
# when spec issues are discovered. No RECALL dependency — context
# should be baked into the spec during planning.
#
# Usage: ./ralph.sh
#
# Environment:
#   MAX_ITERATIONS    Maximum loop iterations (default: 50)
#   STUCK_THRESHOLD   Consecutive errors before auto-escalate (default: 3)

set -euo pipefail

#─────────────────────────────────────────────────────────────────────────────
# Configuration
#─────────────────────────────────────────────────────────────────────────────

MAX_ITERATIONS=${MAX_ITERATIONS:-50}
STUCK_THRESHOLD=${STUCK_THRESHOLD:-3}
COMPLETION_PROMISE="<promise>DONE</promise>"

#─────────────────────────────────────────────────────────────────────────────
# State
#─────────────────────────────────────────────────────────────────────────────

iteration=0
consecutive_failures=0
last_error=""

#─────────────────────────────────────────────────────────────────────────────
# Logging
#─────────────────────────────────────────────────────────────────────────────

log() {
    echo "[$(date +%H:%M:%S)] $*"
}

log_header() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "$*"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

#─────────────────────────────────────────────────────────────────────────────
# Task Management
#─────────────────────────────────────────────────────────────────────────────

get_next_task_id() {
    jq -r '
        .userStories as $all |
        .userStories[] |
        select(.passes == false and .skipped != true) |
        select(
            (.depends_on // []) | 
            all(. as $dep | $all[] | select(.id == $dep) | .passes == true)
        ) |
        .id
    ' PRD.json 2>/dev/null | head -1
}

get_task_details() {
    local task_id=$1
    jq -r --arg id "$task_id" '
        .userStories[] | select(.id == $id) |
        "## \(.id): \(.title)\n\n\(.description)\n\n### Acceptance Criteria\n\(.criteria | map("- " + .) | join("\n"))"
    ' PRD.json
}

mark_task_complete() {
    local task_id=$1
    jq --arg id "$task_id" \
        '(.userStories[] | select(.id == $id)).passes = true' \
        PRD.json > PRD.json.tmp
    mv PRD.json.tmp PRD.json
    log "Marked $task_id complete"
}

mark_task_skipped() {
    local task_id=$1
    jq --arg id "$task_id" \
        '(.userStories[] | select(.id == $id)).skipped = true' \
        PRD.json > PRD.json.tmp
    mv PRD.json.tmp PRD.json
    log "Marked $task_id skipped"
}

get_progress_summary() {
    local total=$(jq '.userStories | length' PRD.json)
    local complete=$(jq '[.userStories[] | select(.passes == true)] | length' PRD.json)
    local skipped=$(jq '[.userStories[] | select(.skipped == true)] | length' PRD.json)
    local remaining=$((total - complete - skipped))
    echo "$complete/$total complete ($remaining remaining, $skipped skipped)"
}

#─────────────────────────────────────────────────────────────────────────────
# Prompt Building
#─────────────────────────────────────────────────────────────────────────────

build_prompt() {
    local task_id=$1
    local task_details=$2
    local human_input=""
    
    # Check for human input from previous escalation
    if [ -f .ralph/human-input.txt ]; then
        human_input=$(cat .ralph/human-input.txt)
        rm .ralph/human-input.txt
        
        if [ "$human_input" = "SKIP" ]; then
            echo "SKIP_TASK"
            return
        fi
    fi
    
    # Build prompt
    {
        echo "# Current Task"
        echo ""
        echo "$task_details"
        echo ""
        
        # Add human guidance if present
        if [ -n "$human_input" ]; then
            echo "## Human Guidance"
            echo ""
            echo "You previously escalated. The human responded:"
            echo ""
            echo "$human_input"
            echo ""
            echo "---"
            echo ""
        fi
        
        # Add instructions
        cat PROMPT.md
        
    } > .ralph/prompt.md

    echo "OK"
}

#─────────────────────────────────────────────────────────────────────────────
# Human Escalation
#─────────────────────────────────────────────────────────────────────────────

escalate_to_human() {
    local output_file=$1
    
    log_header "🚨 ESCALATION REQUIRED"
    echo ""
    
    # Extract and display escalation block
    sed -n '/<escalate/,/<\/escalate>/p' "$output_file"
    
    echo ""
    echo "─────────────────────────────────────────────────────────────────"
    echo ""
    echo "Options:"
    echo "  [1-9]  Select numbered option from above"
    echo "  [c]    Provide custom guidance"
    echo "  [s]    Skip this task"
    echo "  [r]    Retry without guidance"
    echo "  [a]    Abort loop"
    echo ""
    read -p "Choice: " choice
    
    case $choice in
        [1-9])
            echo "Proceed with option $choice." > .ralph/human-input.txt
            log "Selected option $choice"
            ;;
        c)
            echo ""
            echo "Enter guidance (empty line to finish):"
            guidance=""
            while IFS= read -r line; do
                [ -z "$line" ] && break
                guidance+="$line"$'\n'
            done
            echo "$guidance" > .ralph/human-input.txt
            log "Provided custom guidance"
            ;;
        s)
            echo "SKIP" > .ralph/human-input.txt
            log "Chose to skip"
            ;;
        r)
            log "Chose to retry"
            ;;
        a)
            log "Aborted"
            exit 0
            ;;
        *)
            echo "$choice" > .ralph/human-input.txt
            log "Input: $choice"
            ;;
    esac
}

auto_escalate_stuck() {
    local output_file=$1
    local error=$2
    local count=$3
    
    log "Auto-escalating: same error $count times"
    
    cat >> "$output_file" << EOF

<escalate type="stuck">
<summary>Auto-escalated: same error $count consecutive times</summary>
<context>
Error: $error

The loop encountered this error $count times in a row.
</context>
<options>
1. Provide guidance on fixing this error
2. Skip this task
3. Abort the loop
</options>
<question>How should we proceed?</question>
</escalate>
EOF
    
    escalate_to_human "$output_file"
}

#─────────────────────────────────────────────────────────────────────────────
# Output Analysis
#─────────────────────────────────────────────────────────────────────────────

check_for_escalation() {
    grep -q "<escalate" "$1"
}

check_for_loop_done() {
    grep -q "$COMPLETION_PROMISE" "$1"
}

check_for_task_complete() {
    local output_file=$1
    local task_id=$2
    
    # Look for explicit completion statements
    # Be specific to avoid false positives like "couldn't complete"
    grep -qiE "(^|\s)task\s+${task_id}\s+(is\s+)?complete" "$output_file" || \
    grep -qiE "(^|\s)${task_id}:?\s+(is\s+)?(done|complete|finished)" "$output_file" || \
    grep -qiE "completed\s+task\s+${task_id}" "$output_file" || \
    grep -qiE "all\s+acceptance\s+criteria\s+(are\s+)?met" "$output_file"
}

extract_error() {
    # Extract error patterns, being careful to get meaningful ones
    grep -oE "(Error|ERROR|Exception|EXCEPTION|Failed|FAILED|Traceback)[:\s][^.]{10,80}" "$1" 2>/dev/null | head -1 || echo ""
}

#─────────────────────────────────────────────────────────────────────────────
# Main Loop
#─────────────────────────────────────────────────────────────────────────────

preflight_check() {
    local errors=0
    
    if [ ! -f PRD.json ]; then
        echo "Error: PRD.json not found"
        errors=$((errors + 1))
    fi
    
    if [ ! -f PROMPT.md ]; then
        echo "Error: PROMPT.md not found"
        errors=$((errors + 1))
    fi
    
    if ! command -v claude &> /dev/null; then
        echo "Error: claude CLI not found"
        errors=$((errors + 1))
    fi
    
    if ! command -v jq &> /dev/null; then
        echo "Error: jq not found"
        errors=$((errors + 1))
    fi
    
    if [ $errors -gt 0 ]; then
        exit 1
    fi
}

main() {
    preflight_check
    
    log_header "🚀 Ralph Loop Starting"
    log "Max iterations: $MAX_ITERATIONS"
    log "Stuck threshold: $STUCK_THRESHOLD"
    log "Progress: $(get_progress_summary)"
    
    mkdir -p .ralph
    
    while [ $iteration -lt $MAX_ITERATIONS ]; do
        iteration=$((iteration + 1))
        
        # Get next task
        task_id=$(get_next_task_id)
        
        if [ -z "$task_id" ]; then
            log_header "✅ All Tasks Complete"
            log "Final: $(get_progress_summary)"
            exit 0
        fi
        
        log_header "📍 Iteration $iteration: $task_id"
        log "Progress: $(get_progress_summary)"
        
        # Build prompt
        task_details=$(get_task_details "$task_id")
        build_result=$(build_prompt "$task_id" "$task_details")
        
        if [ "$build_result" = "SKIP_TASK" ]; then
            mark_task_skipped "$task_id"
            consecutive_failures=0
            continue
        fi
        
        # Run Claude
        output_file=".ralph/output_${iteration}.txt"
        log "Running Claude..."
        
        if ! cat .ralph/prompt.md | claude -p 2>&1 | tee "$output_file"; then
            log "Warning: Claude exited with error"
        fi
        
        # Check for loop completion
        if check_for_loop_done "$output_file"; then
            log_header "🎉 All Done"
            log "Final: $(get_progress_summary)"
            git add -A && git commit -m "Ralph: all tasks complete" 2>/dev/null || true
            exit 0
        fi
        
        # Check for escalation
        if check_for_escalation "$output_file"; then
            log "Claude escalated"
            escalate_to_human "$output_file"
            consecutive_failures=0
            continue
        fi
        
        # Check for task completion
        if check_for_task_complete "$output_file" "$task_id"; then
            mark_task_complete "$task_id"
            git add -A && git commit -m "Ralph: complete $task_id" 2>/dev/null || true
            consecutive_failures=0
            last_error=""
            continue
        fi
        
        # Check for repeated errors
        current_error=$(extract_error "$output_file")
        
        if [ -n "$current_error" ]; then
            if [ "$current_error" = "$last_error" ]; then
                consecutive_failures=$((consecutive_failures + 1))
                log "Repeated error ($consecutive_failures/$STUCK_THRESHOLD)"
                
                if [ $consecutive_failures -ge $STUCK_THRESHOLD ]; then
                    auto_escalate_stuck "$output_file" "$current_error" $consecutive_failures
                    consecutive_failures=0
                    continue
                fi
            else
                consecutive_failures=1
                last_error="$current_error"
                log "Error: ${current_error:0:60}..."
            fi
        else
            consecutive_failures=0
        fi
        
        # Commit progress
        git add -A && git commit -m "Ralph: progress on $task_id" 2>/dev/null || true
        
        sleep 1
    done
    
    log_header "⚠️ Max Iterations Reached"
    log "Stopped after $MAX_ITERATIONS iterations"
    log "Final: $(get_progress_summary)"
    exit 1
}

main "$@"
