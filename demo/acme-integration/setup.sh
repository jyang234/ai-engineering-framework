#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== Acme Integration Demo Setup ==="
echo ""

# ── Prerequisites ─────────────────────────────────────────────────────────

if ! command -v edi &>/dev/null; then
    echo "ERROR: 'edi' not found in PATH. Install EDI first."
    echo "  cd edi && make build && make install"
    exit 1
fi

if ! command -v claude &>/dev/null; then
    echo "WARNING: 'claude' not found in PATH. EDI launch will fail."
    echo "  Install Claude Code CLI first."
fi

if ! command -v jq &>/dev/null; then
    echo "WARNING: 'jq' not found. verify.sh requires jq."
    echo "  brew install jq (macOS) or apt install jq (Linux)"
fi

# ── Generate project files ────────────────────────────────────────────────

mkdir -p src docs

# go.work scoped to this demo (shadows the parent workspace)
cat > go.work << 'GOWORKEOF'
go 1.22

use ./src
GOWORKEOF
echo "✓ Generated go.work"

# src/main.go — scaffold with TODOs
cat > src/main.go << 'SRCEOF'
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

// Widget represents our internal widget format.
// Maps from Acme's AcmeWidget schema (see docs/data-contract-v1.yaml).
type Widget struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int    `json:"price"`    // cents USD
	Category string `json:"category"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// TODO(US-004): Implement health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "not_implemented"})
	})

	// TODO(US-001): Implement OAuth token client for Acme API authentication
	// See ADR-001 and docs/data-contract-v1.yaml for token endpoint spec

	// TODO(US-002): Implement GET /widgets — fetch catalog from Acme API
	// Must map AcmeWidget fields to our Widget struct
	mux.HandleFunc("GET /widgets", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})

	// TODO(US-003): Implement GET /widgets/{id} — single widget lookup
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})

	addr := fmt.Sprintf(":%s", port)
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
SRCEOF
echo "✓ Generated src/main.go"

# docs/adr-001-auth-method.md
cat > docs/adr-001-auth-method.md << 'EOF'
# ADR-001: Authentication Method for Acme API

## Status
Accepted

## Context
We need to authenticate with the Acme Widgets partner API. Two options were evaluated:

1. **API Keys** — Simple static key in Authorization header
2. **OAuth 2.0 Client Credentials** — Token-based with automatic rotation

## Decision
We chose **OAuth 2.0 Client Credentials** flow.

## Rationale
- Acme's API requires OAuth 2.0 as of their v3 API (API keys deprecated)
- Tokens auto-expire, reducing blast radius of credential leaks
- Client credentials flow is appropriate for server-to-server communication
- Token caching reduces latency on subsequent requests

## Consequences
- Must implement token caching and refresh logic
- Need to store client_id and client_secret securely (env vars)
- Token endpoint: `https://auth.acme-widgets.example.com/oauth/token`
- Tokens expire after 3600 seconds
EOF
echo "✓ Generated docs/adr-001-auth-method.md"

# docs/adr-002-data-format.md
cat > docs/adr-002-data-format.md << 'EOF'
# ADR-002: Data Format for Acme Integration

## Status
Accepted

## Context
The Acme Widgets API supports JSON responses. We considered whether to use protobuf internally for our service-to-service communication.

## Decision
We chose **JSON** for both external (Acme API) and internal communication.

## Rationale
- Acme API only serves JSON — protobuf would require a translation layer with no partner-side benefit
- Our internal consumers (web dashboard, mobile app) already expect JSON
- Simpler debugging — human-readable payloads in logs and traces
- Schema validation via OpenAPI spec is sufficient for our needs

## Consequences
- Slightly larger payload sizes vs protobuf (acceptable for our volume)
- Schema evolution managed through OpenAPI versioning
- Field mapping from Acme's naming conventions to ours handled in the integration layer
EOF
echo "✓ Generated docs/adr-002-data-format.md"

# docs/data-contract-v1.yaml
cat > docs/data-contract-v1.yaml << 'EOF'
openapi: "3.0.3"
info:
  title: Acme Widgets API (Partner Contract)
  version: "3.1.0"
  description: |
    Data contract for the Acme Widgets partner API.
    Base URL: https://api.acme-widgets.example.com/v3

paths:
  /widgets:
    get:
      summary: List all widgets
      operationId: listWidgets
      parameters:
        - name: category
          in: query
          schema:
            type: string
        - name: limit
          in: query
          schema:
            type: integer
            default: 50
            maximum: 200
      responses:
        "200":
          description: Widget catalog
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: "#/components/schemas/AcmeWidget"
                  pagination:
                    $ref: "#/components/schemas/Pagination"

  /widgets/{widget_id}:
    get:
      summary: Get widget by ID
      operationId: getWidget
      parameters:
        - name: widget_id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Widget details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AcmeWidget"
        "404":
          description: Widget not found

  /oauth/token:
    post:
      summary: Get access token
      operationId: getToken
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              properties:
                grant_type:
                  type: string
                  enum: [client_credentials]
                client_id:
                  type: string
                client_secret:
                  type: string
      responses:
        "200":
          description: Token response
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/TokenResponse"

components:
  schemas:
    AcmeWidget:
      type: object
      properties:
        widget_id:
          type: string
          description: Acme's internal widget identifier
        widget_name:
          type: string
        unit_price_cents:
          type: integer
          description: Price in cents (USD)
        widget_category:
          type: string
          enum: [hardware, software, accessories, services]
        created_at:
          type: string
          format: date-time
        is_active:
          type: boolean

    Pagination:
      type: object
      properties:
        total:
          type: integer
        offset:
          type: integer
        limit:
          type: integer

    TokenResponse:
      type: object
      properties:
        access_token:
          type: string
        token_type:
          type: string
        expires_in:
          type: integer
EOF
echo "✓ Generated docs/data-contract-v1.yaml"

# docs/meeting-notes-kickoff.md
cat > docs/meeting-notes-kickoff.md << 'EOF'
# Kickoff Meeting — Acme Widgets Integration

**Date:** 2026-01-15
**Attendees:** Alice (PM), Bob (Backend), Carol (Security), Dave (Partner Eng)

## Agenda
1. Partner API overview
2. Authentication approach
3. Timeline and milestones

## Notes

### Partner API
- Dave confirmed Acme's v3 API is stable and fully documented
- Rate limit: 100 requests/minute per client (soft limit, they'll raise if needed)
- Widget catalog has ~500 items, updates daily at 02:00 UTC

### Key Decisions

**Decision: Use OAuth 2.0 for authentication.** Carol noted that API keys have been deprecated by Acme as of Q4 2025. OAuth client credentials is the only supported method going forward. Token lifetime is 1 hour. We should implement refresh-ahead logic to avoid request failures during token rotation.

**Decision: Cache widget catalog with 5-minute TTL.** Given the catalog only updates daily, aggressive caching is safe. Bob suggested starting with in-memory cache and upgrading to Redis if we need cross-instance consistency.

### Action Items
- [ ] Bob: Scaffold Go service with OAuth client
- [ ] Carol: Review OAuth token storage approach
- [ ] Dave: Send us sandbox credentials by EOW
EOF
echo "✓ Generated docs/meeting-notes-kickoff.md"

# docs/PRD.json
cat > docs/PRD.json << 'EOF'
{
  "project": "acme-integration",
  "description": "HTTP service integrating with the Acme Widgets partner API",
  "userStories": [
    {
      "id": "US-001",
      "title": "OAuth token client",
      "description": "Implement an OAuth 2.0 client credentials token fetcher for authenticating with the Acme API. Token should be cached and refreshed before expiry.",
      "criteria": [
        "Function fetchToken(ctx, clientID, clientSecret, tokenURL) returns access token",
        "Token is cached in memory and reused until 60s before expiry",
        "Returns descriptive error on auth failure",
        "Unit test with httptest mock server"
      ],
      "passes": false
    },
    {
      "id": "US-002",
      "title": "Widget catalog endpoint",
      "description": "Create GET /widgets endpoint that fetches the widget catalog from Acme API and returns it in our internal format.",
      "criteria": [
        "GET /widgets returns JSON array of widgets",
        "Each widget has id, name, price, category fields",
        "Maps Acme API response to internal schema (see data-contract-v1.yaml)",
        "Returns 502 if Acme API is unreachable",
        "Integration test with mock Acme API"
      ],
      "passes": false,
      "depends_on": ["US-001"]
    },
    {
      "id": "US-003",
      "title": "Single widget lookup",
      "description": "Create GET /widgets/:id endpoint that fetches a single widget by ID from Acme API.",
      "criteria": [
        "GET /widgets/:id returns single widget JSON",
        "Returns 404 if widget not found at Acme",
        "Returns 502 if Acme API is unreachable",
        "Unit test covering happy path and error cases"
      ],
      "passes": false,
      "depends_on": ["US-002"]
    },
    {
      "id": "US-004",
      "title": "Health and readiness",
      "description": "Add health check and readiness endpoints.",
      "criteria": [
        "GET /health returns 200 with {\"status\":\"ok\"}",
        "GET /ready checks Acme API connectivity and returns 200 or 503",
        "Both endpoints return within 5 seconds"
      ],
      "passes": false
    }
  ]
}
EOF
echo "✓ Generated docs/PRD.json"

# ── Initialize git repo ───────────────────────────────────────────────────

if [ ! -d .git ]; then
    git init
    git add -A
    git commit -m "Initial demo scaffold"
    echo "✓ Git repository initialized"
else
    echo "✓ Git repository already exists"
fi

# ── Initialize EDI project ────────────────────────────────────────────────

if [ ! -f .edi/config.yaml ]; then
    if [ -f .edi/profile.md ]; then
        cp .edi/profile.md /tmp/acme-demo-profile.md
    fi
    edi init --force
    if [ -f /tmp/acme-demo-profile.md ]; then
        cp /tmp/acme-demo-profile.md .edi/profile.md
        rm -f /tmp/acme-demo-profile.md
    fi

    # Use a project-local Codex DB so the demo is isolated from global RECALL
    cat > .edi/config.yaml << 'CFGEOF'
# EDI Project Configuration
version: "1"

project:
  name: "acme-integration"

recall:
  enabled: true
  backend: codex

codex:
  metadata_db: .edi/codex.db
CFGEOF
    echo "✓ EDI project initialized (custom profile preserved, local Codex DB)"
else
    echo "✓ EDI project already initialized"
fi

# ── Runtime directories ───────────────────────────────────────────────────

mkdir -p .demo
if [ ! -f .demo/results.json ]; then
    echo "[]" > .demo/results.json
    echo "✓ results.json initialized"
fi

# ── Seed RECALL ───────────────────────────────────────────────────────────

echo ""
echo "Seeding RECALL with project knowledge..."

seed_recall() {
    local type="$1" title="$2" content="$3" tags="$4"
    echo "  → $title"
    edi recall add --type "$type" --title "$title" --content "$content" --tags "$tags" --if-not-exists 2>/dev/null || {
        echo "    WARNING: Failed to seed '$title' (RECALL may not be configured)"
    }
}

seed_recall "decision" \
    "ADR-001: OAuth 2.0 for Acme API authentication" \
    "Chose OAuth 2.0 client credentials over API keys. Acme deprecated API keys in Q4 2025. Tokens expire after 3600s, must implement refresh-ahead logic. Token endpoint: auth.acme-widgets.example.com/oauth/token" \
    "auth,oauth,acme,adr"

seed_recall "decision" \
    "ADR-002: JSON data format for Acme integration" \
    "Chose JSON over protobuf. Acme API only serves JSON. Internal consumers expect JSON. Schema validation via OpenAPI. Field mapping from Acme naming (widget_id, widget_name, unit_price_cents) to internal format (id, name, price)." \
    "data-format,json,acme,adr"

seed_recall "context" \
    "Kickoff meeting: Acme integration decisions" \
    "Key decisions from kickoff: 1) Use OAuth 2.0 (API keys deprecated). 2) Cache widget catalog with 5-min TTL (catalog updates daily at 02:00 UTC). Rate limit: 100 req/min. ~500 widgets in catalog. In-memory cache for MVP, Redis upgrade path." \
    "meeting,kickoff,acme,cache"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Run verification: ./verify.sh 01-init && ./verify.sh 02-recall-seed"
echo "  2. Start EDI: edi"
echo "  3. Inside session: search RECALL for 'auth decisions'"
echo "  4. Follow the walkthrough in README.md"
