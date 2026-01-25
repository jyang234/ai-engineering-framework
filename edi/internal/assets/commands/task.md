---
name: task
aliases:
  - tasks
description: Manage task-based workflows with RECALL enrichment
---

# /task [task-id | description]

## No Arguments: Show Task Status

Display current task list with status and RECALL annotation summaries.

```
Tasks: 3 completed, 2 in progress, 4 pending

In Progress:
- task-abc123-4: Implement payment retry logic
  RECALL: 2 patterns, 2 failures, 1 decision

- task-abc123-5: Write payment integration tests
  Blocked by: task-abc123-4

Ready to Start:
- task-abc123-6: Implement refund service
- task-abc123-7: Add payment webhooks
```

Do NOT load full RECALL content at this stage. Show summary counts only.

## With Task ID: Pick Up Task

When picking up a specific task (e.g., `/task task-abc123-4`):

1. Load task annotation from `.edi/tasks/{task-id}.yaml`

2. Display stored RECALL context:
   ```
   Picking up: Implement payment retry logic

   RECALL Context (from task creation):
   - P-008: Exponential backoff with jitter
   - P-041: Circuit breaker pattern
   - F-023: Memory leak with unbounded retry queue
   - ADR-031: Payment service architecture

   Inherited from parent tasks:
   - From task-abc123-2: "Use Stripe as payment provider"
   - From task-abc123-3: "Idempotency keys use UUIDv7"
   ```

3. Begin work with full context

## With Description: Create New Tasks

When given a description (e.g., `/task Implement billing system with Stripe`):

1. Break work into tasks with dependencies

2. For each task, query RECALL:
   ```
   recall_search({query: "[task description]", types: ["pattern", "failure", "decision"]})
   ```

3. Create annotation file in `.edi/tasks/` for each task

4. Log to flight recorder:
   ```
   flight_recorder_log({
     type: "task_annotation",
     content: "Created task: [description]",
     metadata: {
       task_id: "[id]",
       recall_items: ["P-008", "F-023", ...]
     }
   })
   ```

5. Show task graph and ask to proceed

## During Task Execution

Log significant decisions:
```
flight_recorder_log({
  type: "decision",
  content: "[what you decided]",
  rationale: "[why]",
  metadata: {
    task_id: "[current task]",
    propagate: true
  }
})
```

## On Task Completion

1. Log completion to flight recorder

2. If significant decisions were made, prompt:
   ```
   Task completed: Implement payment retry logic

   Decisions made:
   [1] Exponential backoff with jitter, max 5 retries
   [2] Circuit breaker opens after 3 consecutive failures

   Capture to RECALL?
   [A] All as pattern
   [1-2] Select specific
   [S] Skip
   ```

3. Update task annotation with execution context
