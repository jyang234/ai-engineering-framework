---
name: ralph
description: Guided PRD authoring for Ralph autonomous execution
user_invocable: true
---

# /ralph — PRD Authoring for Ralph

You are now in **PRD authoring mode**. Your goal is to produce a complete, Ralph-ready `PRD.json` file through a structured interview process.

Ralph executes tasks autonomously with fresh context windows — each task must be self-contained and fully specified. Your job is to front-load all decisions so Ralph never has to guess.

## Workflow

### Phase 1: Discovery

Interview the user to understand:

1. **What's the goal?** What are we building or changing?
2. **What are the deliverables?** Concrete outputs, not aspirations.
3. **What's the scope boundary?** What is explicitly out of scope?

### Phase 2: Story Breakdown

Break the work into independent user stories:

1. Each story should be completable in a single Claude session (one context window)
2. Define specific, verifiable acceptance criteria for each story — not "works correctly" but "GET /users returns 200 with JSON array containing id, name, email fields"
3. Identify dependencies between stories — which must complete before others can start?
4. Order stories so dependencies flow forward (no circular refs)

### Phase 3: Front-Load Decisions

For each story, ensure the description contains ALL context Ralph will need:

- Architecture decisions that affect implementation
- Technology choices and constraints
- File paths and naming conventions to follow
- Patterns to follow (query RECALL: `recall_search({query: "[relevant pattern]", types: ["pattern", "decision"]})`)
- Known gotchas to avoid (query RECALL: `recall_search({query: "[potential pitfalls]", types: ["failure"]})`)
- Error handling expectations
- Test requirements

**Bake decisions into story descriptions.** Ralph cannot query RECALL or ask clarifying questions at runtime.

### Phase 4: Quality Check

Before writing the PRD, validate:

- [ ] Every story has specific, verifiable acceptance criteria
- [ ] Descriptions are detailed enough for a fresh context window to implement without guessing
- [ ] Dependencies form a DAG (no circular references)
- [ ] No story requires dynamic context retrieval at runtime
- [ ] Stories are ordered so early stories don't depend on later ones
- [ ] Scope is realistic for autonomous execution

### Phase 5: Write PRD.json

Write the PRD file to the current directory:

```json
{
  "project": "project-name",
  "description": "What this project/change does",
  "userStories": [
    {
      "id": "US-001",
      "title": "Story title",
      "description": "Detailed description with all context needed",
      "criteria": [
        "Specific verifiable criterion 1",
        "Specific verifiable criterion 2"
      ],
      "passes": false,
      "depends_on": []
    }
  ]
}
```

### Phase 6: Hand Off

After writing PRD.json, tell the user:

```
PRD.json written with N tasks.

To execute with Ralph:
  1. End this session: /end
  2. Run: edi ralph

Or in a separate terminal (keep this session running):
  edi ralph
```

## Guidelines

- Ask questions — don't assume. Ambiguity in the PRD becomes bugs in execution.
- Push back on vague criteria. "Works correctly" is not verifiable.
- Keep stories small. If a story feels like it needs multiple context windows, split it.
- Use RECALL to find relevant patterns and past decisions. Bake findings into story descriptions.
- The user is the domain expert. You are the spec-writing expert. Collaborate.
