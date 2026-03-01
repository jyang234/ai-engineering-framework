# Spec: FTS5 Search Quality Improvements

## Problem

RECALL's keyword search treats title, content, and tag matches equally.
A query for "PostgreSQL vs DynamoDB" scores a title match the same as a
passing mention buried in content, and the noise word "vs" inflates
false-positive rates by matching every document that contains it.

These are the two lowest-hanging improvements available in the search
pipeline today. Both changes are confined to a single method
(`MetadataStore.KeywordSearch`) and are fully covered by the existing
eval framework.

## Changes

### 1. FTS5 BM25 Column Weighting

**File:** `codex/internal/storage/metadata.go` — `KeywordSearch()`

**Current query (line 469-477):**
```sql
SELECT i.id, i.type, i.title, i.content, i.tags, i.scope,
       -rank AS score
FROM items_fts f
JOIN items i ON i.rowid = f.rowid
WHERE items_fts MATCH ?
ORDER BY rank
LIMIT ?
```

`-rank` uses default BM25 weights (1.0 for all columns). The FTS5 table
columns are ordered `title, content, tags` (defined at line 179-183).

**New query:**
```sql
SELECT i.id, i.type, i.title, i.content, i.tags, i.scope,
       -bm25(items_fts, 5.0, 1.0, 3.0) AS score
FROM items_fts f
JOIN items i ON i.rowid = f.rowid
WHERE items_fts MATCH ?
ORDER BY bm25(items_fts, 5.0, 1.0, 3.0)
LIMIT ?
```

**Column weights:**
| Column  | Weight | Rationale |
|---------|--------|-----------|
| title   | 5.0    | Title is the strongest relevance signal. A direct title match means the document is *about* the query topic, not just mentioning it. |
| content | 1.0    | Baseline. Content matches are still valuable but should not outrank title matches. |
| tags    | 3.0    | Tags are curated metadata chosen specifically for findability. Higher than content but lower than title because tags are short (one hit is very meaningful but also very sparse). |

**Why these specific numbers:** FTS5 BM25 column weights are relative
multipliers. The ratio matters, not the absolute values. 5:1:3 means a
title match is 5x more important than a content match. These are
standard IR starting points (title boost 3-10x, tag boost 2-5x). The
eval framework can tune them empirically.

**Risk:** Near zero. BM25 column weighting is a documented FTS5
feature. It changes scoring, not matching — the same documents are
found, just ranked differently.

### 2. Stop Word Filtering

**File:** `codex/internal/storage/metadata.go` — `KeywordSearch()`

**Current behavior (line 455-467):**
```go
raw := strings.Fields(query)
var terms []string
for _, w := range raw {
    for _, part := range strings.Split(w, "-") {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        escaped := strings.ReplaceAll(part, `"`, `""`)
        terms = append(terms, `"`+escaped+`"`)
    }
}
sanitized := strings.Join(terms, " OR ")
```

The query "PostgreSQL vs DynamoDB" becomes `"PostgreSQL" OR "vs" OR "DynamoDB"`.
The word "vs" appears in many documents and adds noise to results.

**New behavior:** Add a stop word check inside the inner loop:

```go
raw := strings.Fields(query)
var terms []string
for _, w := range raw {
    for _, part := range strings.Split(w, "-") {
        part = strings.TrimSpace(part)
        if part == "" || isStopWord(part) {
            continue
        }
        escaped := strings.ReplaceAll(part, `"`, `""`)
        terms = append(terms, `"`+escaped+`"`)
    }
}
sanitized := strings.Join(terms, " OR ")
```

**Stop word list:**

```go
var ftsStopWords = map[string]bool{
    "a": true, "an": true, "and": true, "are": true, "as": true,
    "at": true, "be": true, "by": true, "do": true, "for": true,
    "from": true, "has": true, "how": true, "i": true, "in": true,
    "is": true, "it": true, "its": true, "not": true, "of": true,
    "on": true, "or": true, "so": true, "that": true, "the": true,
    "this": true, "to": true, "vs": true, "was": true, "we": true,
    "what": true, "when": true, "which": true, "who": true,
    "will": true, "with": true,
}

func isStopWord(word string) bool {
    return ftsStopWords[strings.ToLower(word)]
}
```

This is a small, conservative list (~35 words) covering:
- English articles/prepositions: a, an, the, in, on, at, of, for, by, from, to, with, as
- Common verbs: is, are, was, be, do, has, will
- Pronouns: i, it, its, we, who, this, that, which
- Conjunctions: and, or, not, so, when
- Domain-specific noise: vs, how, what

**Edge case — all words are stop words:** If filtering removes every
term, fall back to the original unfiltered query. This prevents empty
MATCH expressions:

```go
if len(terms) == 0 {
    // All terms were stop words; use original terms unfiltered
    for _, w := range raw {
        escaped := strings.ReplaceAll(w, `"`, `""`)
        terms = append(terms, `"`+escaped+`"`)
    }
}
```

**Risk:** Near zero. Stop words are filtered before the FTS query is
constructed. The worst case is a marginal recall loss for queries where
a stop word is genuinely meaningful (e.g., searching for the Go keyword
"for"). The fallback behavior (use unfiltered query when all terms are
stop words) prevents catastrophic failures.

## Files Changed

| File | Change |
|------|--------|
| `codex/internal/storage/metadata.go` | BM25 weights in SQL query; stop word list + filter in `KeywordSearch()` |
| `codex/internal/storage/metadata_test.go` | New test cases for column weighting and stop word filtering |

## Test Plan

### New Unit Tests (metadata_test.go)

**Test: Title matches rank higher than content matches**
```
Given: Two items — one with "authentication" in title, one with
       "authentication" in content only
When:  KeywordSearch("authentication")
Then:  Title-match item ranks first
```

**Test: Tag matches rank higher than content matches**
```
Given: Two items — one with "circuit-breaker" in tags, one with
       "circuit breaker" only in content
When:  KeywordSearch("circuit breaker")
Then:  Tag-match item ranks first
```

**Test: Stop words are filtered from queries**
```
Given: Two items — one about PostgreSQL, one about DynamoDB
When:  KeywordSearch("PostgreSQL vs DynamoDB")
Then:  Both items returned (not every document containing "vs")
```

**Test: All-stop-word query falls back to unfiltered**
```
Given: Items in database
When:  KeywordSearch("is it a")
Then:  No error, returns results (not empty MATCH)
```

**Test: Stop word filtering preserves existing behavior for non-stop queries**
```
Given: Item with "JWT Authentication Pattern" in title
When:  KeywordSearch("authentication")
Then:  Item found (no regression)
```

### Existing Tests (must still pass)

- `TestMetadataStore_KeywordSearch_Basic` — basic term matching
- `TestMetadataStore_KeywordSearch_NoResults` — missing terms
- `TestMetadataStore_KeywordSearch_SpecialChars` — FTS operator safety

### Eval Framework Validation

After implementation, run the retrieval eval to measure impact:
```bash
go test -tags "fts5" -run TestE2E ./codex/eval -v
```

Expected improvements:
- **Keyword queries** (q-08 through q-14): Higher precision from title/tag boosting
- **Hybrid queries** (q-15 through q-20): Better keyword signal feeding into RRF
- **Semantic queries** (q-01 through q-07): Neutral (these are vector-dominated)

## Not In Scope

- **Query-adaptive RRF weights**: Requires eval data to tune. Do after
  we have baseline numbers from this change.
- **Contextual enrichment**: Operates on a code path (`Indexer`) not
  used by the primary `recall_add` flow.
- **Reranking**: Separate concern, higher complexity.
