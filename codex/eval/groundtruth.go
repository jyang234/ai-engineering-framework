package eval

// PayFlowQueries returns 20 ground truth queries for evaluating the PayFlow
// test collection. Queries are split across three categories: semantic (meaning-
// based retrieval), keyword (term-matching retrieval), and hybrid-advantage
// (queries where combining both signals improves results).
func PayFlowQueries() []TestQuery {
	return []TestQuery{
		// ── semantic (7) ────────────────────────────────────────────────
		{
			ID:       "q-01",
			Query:    "How does the system ensure payment operations are not duplicated?",
			Category: "semantic",
			RelevantIDs: []string{
				"adr-003", "pattern-001", "api-001",
			},
		},
		{
			ID:       "q-02",
			Query:    "What happens when an external payment provider goes down?",
			Category: "semantic",
			RelevantIDs: []string{
				"adr-004", "pattern-002", "arch-002",
			},
		},
		{
			ID:       "q-03",
			Query:    "How are payment state transitions tracked over time?",
			Category: "semantic",
			RelevantIDs: []string{
				"adr-001", "pattern-003", "arch-002",
			},
		},
		{
			ID:       "q-04",
			Query:    "What approach was taken for handling sensitive card data?",
			Category: "semantic",
			RelevantIDs: []string{
				"adr-005", "arch-001",
			},
		},
		{
			ID:       "q-05",
			Query:    "How does the system handle payments in different currencies?",
			Category: "semantic",
			RelevantIDs: []string{
				"arch-005", "design-001",
			},
		},
		{
			ID:       "q-06",
			Query:    "What was the strategy for breaking up long-running payment workflows?",
			Category: "semantic",
			RelevantIDs: []string{
				"pattern-004", "arch-002",
			},
		},
		{
			ID:       "q-07",
			Query:    "How does the system notify merchants about payment events?",
			Category: "semantic",
			RelevantIDs: []string{
				"design-002", "api-004",
			},
		},

		// ── keyword (7) ─────────────────────────────────────────────────
		{
			ID:       "q-08",
			Query:    "PostgreSQL vs DynamoDB",
			Category: "keyword",
			RelevantIDs: []string{
				"adr-002",
			},
		},
		{
			ID:       "q-09",
			Query:    "circuit breaker threshold",
			Category: "keyword",
			RelevantIDs: []string{
				"adr-004", "pattern-002",
			},
		},
		{
			ID:       "q-10",
			Query:    "PCI compliance tokenization",
			Category: "keyword",
			RelevantIDs: []string{
				"adr-005", "meeting-002",
			},
		},
		{
			ID:       "q-11",
			Query:    "webhook delivery retry",
			Category: "keyword",
			RelevantIDs: []string{
				"design-002", "api-004",
			},
		},
		{
			ID:       "q-12",
			Query:    "GraphQL REST merchant API",
			Category: "keyword",
			RelevantIDs: []string{
				"adr-006", "design-003",
			},
		},
		{
			ID:       "q-13",
			Query:    "Black Friday performance",
			Category: "keyword",
			RelevantIDs: []string{
				"meeting-003",
			},
		},
		{
			ID:       "q-14",
			Query:    "settlement reconciliation",
			Category: "keyword",
			RelevantIDs: []string{
				"arch-003",
			},
		},

		// ── hybrid-advantage (6) ────────────────────────────────────────
		{
			ID:       "q-15",
			Query:    "idempotency key implementation for payment creation",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"adr-003", "pattern-001", "api-001", "design-001",
			},
		},
		{
			ID:       "q-16",
			Query:    "fraud detection machine learning pipeline",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"arch-004", "meeting-005",
			},
		},
		{
			ID:       "q-17",
			Query:    "refund processing error handling",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"design-004", "api-003", "meeting-004",
			},
		},
		{
			ID:       "q-18",
			Query:    "event sourcing aggregate state reconstruction",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"adr-001", "pattern-003",
			},
		},
		{
			ID:       "q-19",
			Query:    "merchant onboarding authentication flow",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"design-003", "api-005",
			},
		},
		{
			ID:       "q-20",
			Query:    "payment status tracking webhook notifications",
			Category: "hybrid-advantage",
			RelevantIDs: []string{
				"design-005", "design-002", "api-002", "api-004",
			},
		},
	}
}
