# Project Profile

## Overview

Acme Integration — a Go HTTP service that integrates with the Acme Widgets partner API. This service acts as a middleware layer between our internal systems and Acme's external API, handling authentication, data transformation, and error handling.

## Architecture

- Single Go binary HTTP server
- OAuth 2.0 client credentials flow for Acme API authentication (see ADR-001)
- JSON request/response format (see ADR-002)
- In-memory caching with TTL for widget catalog data
- Structured logging with slog

## Tech Stack

- **Go 1.22+**
- **net/http** — standard library HTTP server
- **slog** — structured logging
- **encoding/json** — JSON serialization

## Conventions

- Standard Go project layout (`cmd/`, `internal/`)
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Table-driven tests
- Context propagation through all function signatures

## Key Decisions

1. **OAuth over API keys** — Partner requires OAuth 2.0; API keys were considered but rejected for security reasons (ADR-001)
2. **JSON over protobuf** — Simpler integration, partner API is JSON-only (ADR-002)
3. **In-memory cache** — Acceptable for MVP; Redis upgrade path exists

## Getting Started

```bash
cd demo/acme-integration
./setup.sh
edi
```
