# Sandbox Architecture Specification

**Status**: Draft  
**Created**: January 24, 2026  
**Last Updated**: January 24, 2026  
**Version**: 0.1

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Core Identity](#2-core-identity)
3. [Architecture Overview](#3-architecture-overview)
4. [Core Concepts](#4-core-concepts)
5. [Experiment Types](#5-experiment-types)
6. [Fault Injection](#6-fault-injection)
7. [Telemetry & Observability](#7-telemetry--observability)
8. [API Design](#8-api-design)
9. [Concurrency & Queuing](#9-concurrency--queuing)
10. [Artifact Lifecycle](#10-artifact-lifecycle)
11. [Deployment & Operations](#11-deployment--operations)
12. [Integration Points](#12-integration-points)
13. [Scale Evolution Guide](#13-scale-evolution-guide)
14. [Decision Log](#14-decision-log)

---

## 1. Executive Summary

### What is Sandbox?

**Sandbox is deterministic infrastructure for controlled experimentation.** It provides disposable simulation environments where Claude can run integration-level behavioral verification with full observability.

Sandbox is a lab bench, not a scientist. It provides the *where* and *how* of execution. Claude provides the *what* (code, data, scenarios). This separation is critical — Sandbox has zero intelligence, zero decision-making.

### Core Value Proposition

| Problem | Sandbox Solution |
|---------|------------------|
| Need to verify code behavior before commit | Controlled experiment execution with rich feedback |
| CI/CD is too slow for iterative development | Fast, disposable environments for rapid feedback loops |
| Can't safely test failure scenarios | Multi-strategy fault injection (network, kernel, app) |
| Hard to debug what went wrong | Full telemetry (logs, metrics, traces) on every run |
| Test artifacts accumulate endlessly | Purpose-driven lifecycle with automatic cleanup |

### Design Principles

| Principle | Implication |
|-----------|-------------|
| **Deterministic infrastructure** | Zero intelligence in Sandbox; Claude provides all decisions |
| **Disposable by default** | Every experiment is ephemeral; no persistent state |
| **Rich observability** | Full telemetry (logs, metrics, traces) on every run |
| **Mount, don't build** | Language base images + mounted code; avoid image sprawl |
| **Multi-strategy fault injection** | Network, kernel, and application-level faults |
| **Artifact lifecycle tied to purpose** | PR artifacts expire; Codex-linked artifacts persist |

### Scope Boundaries

```
┌─────────────────────────────────────────────────────────────────────┐
│                        SCOPE BOUNDARIES                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   Local Dev          CI/CD Pipeline           Sandbox               │
│   ──────────         ──────────────           ───────               │
│   Unit tests         Unit tests               Integration tests     │
│   Fast iteration     Automated gates          Behavioral scenarios  │
│   Developer box      Build server             Simulated topology    │
│                                               Full observability    │
│                                               Disposable            │
│                                                                     │
│   ◄──── Speed ────►  ◄──── Gates ────►        ◄──── Fidelity ────►  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Core Identity

### What Sandbox IS

| Responsibility | Description |
|----------------|-------------|
| **Container orchestration** | Spin up/tear down Docker environments |
| **Input injection** | Mount code, data, configs into containers |
| **Execution control** | Start, monitor, timeout, terminate |
| **Telemetry collection** | Aggregate logs, metrics, traces from sidecars |
| **Result packaging** | Structured output (pass/fail, measurements, artifacts) |
| **Isolation guarantee** | No cross-experiment contamination, no host escape |
| **Fault injection** | Network, kernel, and application-level fault simulation |

### What Sandbox is NOT

| Not This | Why |
|----------|-----|
| **Test framework** | Doesn't define tests — runs whatever Claude provides |
| **AI/LLM component** | Zero intelligence, purely mechanical |
| **CI/CD system** | No pipelines, triggers, or deployment — just execution |
| **Codex** | Doesn't store knowledge — produces inputs for Codex |
| **Evaluator** | Doesn't judge pass/fail — reports measurements, Claude interprets |

The last point is subtle but important: Sandbox reports "test X returned exit code 1" or "latency p99 = 450ms". The *interpretation* (is 450ms acceptable?) happens in Claude or the integration layer based on the success criteria Claude provided.

### Responsibility Separation

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLAUDE (Intelligence)                       │
│                                                                     │
│   Generates:                                                        │
│   • App code (greenfield or modifications)                          │
│   • Test scenarios (inputs, edge cases)                             │
│   • Environment configs (env vars, feature flags)                   │
│   • Success criteria (what to measure, thresholds)                  │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼ Experiment Definition
┌─────────────────────────────────────────────────────────────────────┐
│                      SANDBOX (Deterministic Infra)                  │
│                                                                     │
│   Executes:                                                         │
│   • Spin up containers (app + sidecars)                             │
│   • Inject provided code/data/config                                │
│   • Run to completion or timeout                                    │
│   • Collect telemetry (logs, metrics, traces)                       │
│   • Report structured results                                       │
│   • Tear down (no state leakage)                                    │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼ Experiment Results
┌─────────────────────────────────────────────────────────────────────┐
│                     INTEGRATION LAYER (Routing)                     │
│                                                                     │
│   Routes results to:                                                │
│   • Codex (Evidence creation/update)                                │
│   • LLM Judge (failure attribution)                                 │
│   • Flight Recorder (telemetry archive)                             │
│   • UI (human visibility)                                           │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. Architecture Overview

### Layer Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 5: UI/Dashboard                                              │
│  ─────────────────────                                              │
│  Human visibility: experiment results, escalation queue,            │
│  knowledge browser. Pure read + HITL actions.                       │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 4: Integration                                               │
│  ────────────────────                                               │
│  Routes results → Codex, LLM Judge, Flight Recorder.                │
│  Applies success criteria. Triggers downstream workflows.           │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 3: Telemetry Harness                                         │
│  ─────────────────────────                                          │
│  OTEL collector, log aggregator, metrics scraper.                   │
│  Runs as sidecars. Streams to central store during experiment.      │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 2: Experiment Orchestrator                                   │
│  ───────────────────────────────                                    │
│  Reads experiment definition. Builds container graph.               │
│  Manages lifecycle (create → inject → run → collect → destroy).     │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 1: Container Runtime                                         │
│  ───────────────────────────                                        │
│  Docker daemon. Network isolation. Volume mounts.                   │
│  Resource limits (CPU, memory, time).                               │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                       SANDBOX SERVICE                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  API Layer                                                  │    │
│  │  ──────────                                                 │    │
│  │  POST /experiments         Create & run experiment          │    │
│  │  GET  /experiments/{id}    Get status & results             │    │
│  │  DELETE /experiments/{id}  Force teardown                   │    │
│  │  GET  /experiments/{id}/artifacts/{type}  Stream artifacts  │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                      │
│                              ▼                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Experiment Controller                                      │    │
│  │  ───────────────────────                                    │    │
│  │  • Validates experiment definition                          │    │
│  │  • Manages experiment lifecycle (queued→running→complete)   │    │
│  │  • Enforces resource limits & timeouts                      │    │
│  │  • Coordinates components below                             │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                      │
│         ┌────────────────────┼────────────────────┐                 │
│         ▼                    ▼                    ▼                 │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐          │
│  │ Topology    │      │ Scenario    │      │ Telemetry   │          │
│  │ Manager     │      │ Executor    │      │ Collector   │          │
│  │             │      │             │      │             │          │
│  │ • Docker    │      │ • Phase     │      │ • OTEL      │          │
│  │   Compose   │      │   runner    │      │   collector │          │
│  │ • Network   │      │ • Fault     │      │ • Log       │          │
│  │   isolation │      │   injection │      │   aggregator│          │
│  │ • Volume    │      │ • HTTP      │      │ • Metrics   │          │
│  │   mounts    │      │   client    │      │   scraper   │          │
│  │ • Health    │      │ • Wait      │      │ • Artifact  │          │
│  │   checks    │      │   conditions│      │   storage   │          │
│  └─────────────┘      └─────────────┘      └─────────────┘          │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  Assertion Evaluator                                        │    │
│  │  ────────────────────                                       │    │
│  │  • Evaluates assertions against collected telemetry         │    │
│  │  • No interpretation — just "condition met?" true/false     │    │
│  │  • Attaches evidence (relevant logs, metrics, traces)       │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       DOCKER HOST                                   │
│                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │  Experiment Network (isolated per experiment)               │   │
│   │                                                             │   │
│   │   ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐              │   │
│   │   │ app │  │ db  │  │redis│  │otel │  │ log │              │   │
│   │   │     │  │     │  │     │  │coll │  │ agg │              │   │
│   │   └─────┘  └─────┘  └─────┘  └─────┘  └─────┘              │   │
│   │                                                             │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Summary

| Component | Responsibility | Technology |
|-----------|---------------|------------|
| **API Gateway** | HTTP interface, auth, rate limiting | FastAPI |
| **Experiment Controller** | Lifecycle management, timeout enforcement | Python async |
| **Queue Manager** | Multi-lane FIFO, assignment | Redis or in-memory |
| **Topology Manager** | Docker Compose generation, orchestration | Docker SDK |
| **Scenario Executor** | Phase runner, action dispatch | Python async |
| **Fault Injector** | Toxiproxy, tc/iptables, env injection | Mixed |
| **Telemetry Collector** | OTEL, logs, metrics aggregation | OTEL Collector + Loki + Prometheus |
| **Assertion Evaluator** | Condition evaluation, evidence extraction | Python |
| **Artifact Manager** | Storage, lifecycle, cleanup | S3-compatible + cron |

---

## 4. Core Concepts

### 4.1 Experiment

The unit of work. Claude defines it, Sandbox executes it.

```yaml
experiment:
  # Identity
  id: exp-${uuid}
  name: string
  type: regression | verification | exploration
  
  # Linkage (determines artifact retention)
  context:
    type: pr | poc | re_verification | ad_hoc
    pr_id: string?              # If PR-linked
    evidence_id: string?        # If Codex-linked
    session_id: string          # Always present
  
  # What to run
  topology: Topology
  scenario: Scenario
  assertions: Assertion[]
  
  # Execution bounds
  execution:
    timeout: duration           # Max wall-clock time
    resource_limits:
      cpu: string               # e.g., "2.0" (cores)
      memory: string            # e.g., "4Gi"
      network_bandwidth: string # e.g., "100Mbps"
```

### 4.2 Topology

The service graph for the experiment. Claude describes what's needed; Sandbox materializes it.

```yaml
topology:
  # Base language runtime (mounted code approach)
  runtime:
    language: python | node | go | java | rust
    version: string             # e.g., "3.11", "20", "1.21"
    
  # Code to mount
  code:
    source: git | local | generated
    ref: string?                # Git ref if source=git
    path: string                # Path to mount
    mount_point: /app           # Where in container
    
  # Additional services
  services:
    - name: string
      image: string
      ports: int[]
      env: map[string, string]
      volumes: Volume[]
      healthcheck: Healthcheck?
      depends_on: string[]
      secrets: string[]         # Secret names to inject
      
  # Test data
  fixtures:
    - name: string
      type: sql | json | csv | file
      source: inline | file | generated
      content: string | path
      target:
        service: string
        path: string            # Mount path or DB init
```

### 4.3 Scenario

What happens during the experiment. Claude generates this.

```yaml
scenario:
  name: string
  
  phases:
    - name: string
      actions: Action[]
      
  # Action types
  actions:
    # Setup actions
    - type: wait_healthy
      target: service_name
      timeout: duration
      
    - type: seed_data
      target: service_name
      fixture: fixture_name
      
    # Execution actions  
    - type: http_request
      target: service_name
      method: GET | POST | PUT | DELETE
      path: string
      headers: map[string, string]?
      body: any?
      capture_as: string?       # Variable name for response
      
    - type: grpc_call
      target: service_name
      service: string
      method: string
      request: any
      capture_as: string?
      
    - type: shell_exec
      target: service_name
      command: string
      capture_as: string?
      
    - type: wait
      duration: duration
      
    - type: wait_until
      condition: string         # Expression over captured vars
      timeout: duration
      poll_interval: duration
      
    # Fault injection actions
    - type: inject_fault
      strategy: network | kernel | application
      fault: FaultSpec
      duration: duration?       # If omitted, until phase ends
```

### 4.4 Assertions

What Claude expects to observe. Sandbox evaluates and reports.

```yaml
assertions:
  - name: string
    description: string?
    
    # Assertion types
    type: state_check | log_contains | log_absent | metric_threshold | 
          trace_analysis | response_check | exit_code
    
    # Type-specific config
    config:
      # state_check
      source: captured.variable_name
      path: "$.json.path"       # JSONPath
      condition: "== 'expected'"
      
      # log_contains / log_absent
      source: service_name.logs
      pattern: string | regex
      count: ">= 1" | "== 0"    # For contains vs absent
      window: duration?          # Time window to search
      
      # metric_threshold
      source: service_name.metrics
      metric: string            # Prometheus metric name
      labels: map[string, string]?
      condition: ">= 100"
      aggregation: last | avg | max | min | sum
      window: duration?
      
      # trace_analysis
      source: traces
      span_name: string
      attribute_filter: map[string, string]?
      condition: "count == 1" | "duration < 500ms"
      
      # response_check  
      source: captured.http_response
      status: 200
      body_path: "$.data.id"
      body_condition: "!= null"
      
      # exit_code
      source: service_name
      condition: "== 0"
```

### 4.5 Experiment Result

Rich outputs that give Claude actionable feedback:

```yaml
result:
  # Identity
  experiment_id: string
  
  # Status
  status: passed | failed | error | timeout
  started_at: timestamp
  completed_at: timestamp
  duration_ms: int
  
  # Summary
  summary:
    assertions_total: int
    assertions_passed: int
    assertions_failed: int
    
  # Detailed assertion results
  assertions:
    - name: string
      status: passed | failed | error
      
      # On failure: what we expected vs got
      expected: string?
      actual: string?
      
      # Evidence: relevant slice of telemetry
      evidence:
        log_snippet: string?      # Relevant log lines
        metric_values: any?       # Relevant metric data points
        trace_snippet: any?       # Relevant spans
        
  # Artifacts (retained per lifecycle policy)
  artifacts:
    logs:
      $service_name: url         # S3/blob URL
    traces: url                  # OTLP export
    metrics: url                 # Prometheus snapshot
    captures: map[string, any]   # Captured variables
    
  # Telemetry stats
  telemetry:
    spans_collected: int
    log_lines_collected: int
    metrics_scraped: int
    
  # Resource usage
  resources:
    peak_cpu: float
    peak_memory_mb: int
    network_io_mb: int
```

---

## 5. Experiment Types

### 5.1 Regression (PR Context)

Existing code modified → Run existing tests → Verify no regressions

```yaml
experiment:
  type: regression
  context:
    type: pr
    pr_id: "gh-123"
    
  inputs:
    code:
      source: git
      ref: feature/new-payment-flow
    tests:
      source: existing           # Project's test suite
      
  success_criteria:
    - metric: test_pass_rate
      threshold: ">= 0.90"
```

**Self-correct loop integration:**

```
┌──────────────────────────────────────────────────────────────────┐
│                     SELF-CORRECT LOOP                            │
│                                                                  │
│   iteration = 0                                                  │
│   max_iterations = 3  (configurable)                             │
│                                                                  │
│   while iteration < max_iterations:                              │
│       if iteration == 0:                                         │
│           code = generate_implementation(task)                   │
│       else:                                                      │
│           code = generate_fix(code, last_result)                 │
│                                                                  │
│       experiment = build_experiment(code, tests)                 │
│       result = sandbox.run(experiment)  ◄─────────────────┐      │
│                                                           │      │
│       if result.pass_rate >= 0.90:                        │      │
│           return Success(code)                            │      │
│                                                           │      │
│       last_result = result                                │      │
│       iteration += 1                                      │      │
│                                                           │      │
│   return Escalate(code, all_results)                      │      │
│                                                           │      │
└───────────────────────────────────────────────────────────┘      │
                                                                   │
           Sandbox executes each iteration ────────────────────────┘
```

### 5.2 Verification (Evidence Re-verification)

Existing claim → Run verification experiment → Confirm/update/deprecate

```yaml
experiment:
  type: verification
  context:
    type: re_verification
    evidence_id: "codex-ev-12345"
    
  inputs:
    claim: "OrderService handles 500 orders/min"
    
  topology:
    services:
      - name: app
        image: order-service:latest
      - name: load-generator
        image: locust:latest
        
  scenario:
    phases:
      - name: load_test
        actions:
          - type: shell_exec
            target: load-generator
            command: "locust -f /tests/order_load.py --headless -u 500 -r 50"
            
  assertions:
    - name: "Throughput meets claim"
      type: metric_threshold
      config:
        source: app.metrics
        metric: orders_processed_total
        condition: "rate(1m) >= 500"
        
    - name: "Error rate acceptable"
      type: metric_threshold
      config:
        source: app.metrics
        metric: order_errors_total
        condition: "rate(1m) < 5"
```

### 5.3 Exploration (POC/Greenfield)

Claude generates both code AND tests → Discover behavior

```yaml
experiment:
  type: exploration
  context:
    type: poc
    
  inputs:
    code:
      source: generated          # Claude-generated
    scenario:
      source: generated          # Claude-generated test scenario
      description: "Concurrent payment processing with network partition"
    test_data:
      source: generated          # Claude-generated fixtures
      
  # No strict pass/fail — exploratory
  assertions:
    - name: "Observe retry behavior"
      type: log_contains
      config:
        source: app.logs
        pattern: "retry attempt"
        # count not specified — just observe
```

---

## 6. Fault Injection

Three strategies, composable within a single experiment:

### 6.1 Network Faults (L7 Proxy)

Implemented via Toxiproxy sidecar.

```yaml
fault_injection:
  strategy: network
  faults:
    - type: latency
      target: 
        from: app
        to: database
      latency: 500ms
      jitter: 100ms
      
    - type: bandwidth
      target: { from: app, to: storage }
      rate: 1Mbps
      
    - type: packet_loss
      target: { from: app, to: cache }
      percent: 10
      
    - type: connection_reset
      target: { from: app, to: payment-gateway }
      probability: 0.5
      
    - type: partition
      between: [service_a, service_b]
      # Complete network isolation
```

### 6.2 Kernel Faults (tc/iptables)

Implemented via privileged sidecar with kernel access.

```yaml
fault_injection:
  strategy: kernel
  faults:
    - type: disk_latency
      target: app
      device: /dev/sda
      latency: 100ms
      
    - type: cpu_pressure
      target: app
      percent: 80
      
    - type: memory_pressure
      target: app
      percent: 90
      
    - type: io_error
      target: app
      device: /dev/sda
      error_rate: 0.01
```

### 6.3 Application Faults (Env Vars)

Implemented via environment variable injection at container start.

```yaml
fault_injection:
  strategy: application
  faults:
    - type: env_override
      target: app
      vars:
        SIMULATE_PAYMENT_FAILURE: "true"
        FAILURE_RATE: "0.5"
        CIRCUIT_BREAKER_DISABLED: "true"
        
    - type: feature_flag
      target: app
      flags:
        new_payment_flow: false
        legacy_mode: true
```

### Fault Injection Implementation

| Strategy | Implementation | Pros | Cons |
|----------|----------------|------|------|
| Network (L7) | Toxiproxy sidecar | Easy to configure, protocol-aware | Slight overhead |
| Kernel (L4/disk) | `tc` / `iptables` via privileged sidecar | More realistic, lower level | Linux-specific, requires privileges |
| Application | Environment variables | Simple, no infra needed | Requires app support |

---

## 7. Telemetry & Observability

### 7.1 Collection Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     EXPERIMENT NETWORK                              │
│                                                                     │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐                       │
│   │   app   │     │   db    │     │  redis  │                       │
│   │         │     │         │     │         │                       │
│   │ ┌─────┐ │     │ ┌─────┐ │     │ ┌─────┐ │                       │
│   │ │OTEL │ │     │ │logs │ │     │ │logs │ │                       │
│   │ │agent│ │     │ │only │ │     │ │only │ │                       │
│   │ └──┬──┘ │     │ └──┬──┘ │     │ └──┬──┘ │                       │
│   └────┼────┘     └────┼────┘     └────┼────┘                       │
│        │               │               │                            │
│        └───────────────┼───────────────┘                            │
│                        ▼                                            │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │                    TELEMETRY SIDECARS                       │   │
│   │                                                             │   │
│   │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │   │
│   │  │    OTEL     │  │    Loki     │  │ Prometheus  │          │   │
│   │  │  Collector  │  │   (logs)    │  │  (metrics)  │          │   │
│   │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘          │   │
│   │         │                │                │                 │   │
│   └─────────┼────────────────┼────────────────┼─────────────────┘   │
│             │                │                │                     │
└─────────────┼────────────────┼────────────────┼─────────────────────┘
              │                │                │
              ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     ARTIFACT STORAGE                                │
│                                                                     │
│   s3://sandbox-artifacts/{experiment_id}/                           │
│   ├── traces.otlp                                                   │
│   ├── logs/                                                         │
│   │   ├── app.jsonl                                                 │
│   │   ├── db.jsonl                                                  │
│   │   └── redis.jsonl                                               │
│   └── metrics.prom                                                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 7.2 Telemetry Types

| Type | Format | Source | Collection |
|------|--------|--------|------------|
| **Traces** | OTLP | OTEL SDK in app | OTEL Collector |
| **Logs** | JSON lines | stdout/stderr | Loki or direct capture |
| **Metrics** | Prometheus | /metrics endpoint | Prometheus scrape |

### 7.3 Evidence Extraction

When assertions fail, Sandbox extracts relevant telemetry slices:

```yaml
evidence_extraction:
  on_assertion_failure:
    log_contains:
      # Extract window around expected pattern
      strategy: context_window
      before: 10 lines
      after: 5 lines
      # Or if pattern not found, last N lines
      fallback: last_50_lines
      
    metric_threshold:
      # Extract metric values during experiment
      strategy: time_series
      resolution: 1s
      include_labels: true
      
    trace_analysis:
      # Extract relevant spans
      strategy: span_tree
      root: matching_span
      include_children: true
      include_siblings: false
```

---

## 8. API Design

### 8.1 Endpoints

```yaml
api:
  base: /api/v1/sandbox
  
  endpoints:
    # Create and run experiment
    - method: POST
      path: /experiments
      request: ExperimentDefinition
      response: 
        id: string
        status: queued | running
        queue_position: int?
        estimated_start: timestamp?
      
    # Get experiment status/result
    - method: GET
      path: /experiments/{id}
      response: ExperimentResult | ExperimentStatus
      
    # Stream logs in real-time (while running)
    - method: GET
      path: /experiments/{id}/logs/stream
      response: text/event-stream
      query:
        services: string[]?     # Filter by service
        
    # Get artifact
    - method: GET
      path: /experiments/{id}/artifacts/{type}
      types: [logs, traces, metrics, captures]
      response: 
        url: string             # Pre-signed download URL
        expires_at: timestamp
        
    # Force teardown (if stuck/hung)
    - method: DELETE
      path: /experiments/{id}
      response:
        status: terminated
        
    # Export artifacts (user-initiated retention)
    - method: POST
      path: /experiments/{id}/export
      request:
        artifacts: string[]     # Which to export
        destination: local | s3 | codex
      response:
        export_id: string
        status: pending | complete
        urls: map[string, string]?
        
    # Queue status
    - method: GET
      path: /queue
      response:
        lanes:
          - id: int
            running: string?    # Experiment ID
            queued: int
        total_queued: int
        estimated_wait: duration
        
    # Health check
    - method: GET
      path: /health
      response:
        status: healthy | degraded | unhealthy
        docker: connected | disconnected
        storage: connected | disconnected
```

### 8.2 Webhook Events (Optional)

For async integrations:

```yaml
webhooks:
  events:
    - experiment.queued
    - experiment.started
    - experiment.completed
    - experiment.failed
    - experiment.timeout
    
  payload:
    event: string
    experiment_id: string
    timestamp: timestamp
    data: ExperimentResult?     # On completed/failed
```

---

## 9. Concurrency & Queuing

### 9.1 Multi-Lane Queue

```yaml
concurrency:
  # Multi-lane queue
  lanes:
    count: 5                    # Configurable 3-5
    assignment: round_robin     # Or: least_loaded
    
  # Per-lane behavior
  lane:
    max_concurrent: 1           # One experiment per lane
    queue_depth: 10             # Max queued per lane
    queue_behavior: fifo
    
  # Backpressure
  when_full:
    action: reject              # Or: wait
    error: "sandbox_queue_full"
    retry_after: 30s
```

### 9.2 Queue Visualization

```
┌─────────────────────────────────────────────────────────────────────┐
│                        QUEUE MANAGER                                │
│                                                                     │
│   Lane 1    Lane 2    Lane 3    Lane 4    Lane 5                    │
│   ┌─────┐   ┌─────┐   ┌─────┐   ┌─────┐   ┌─────┐                   │
│   │ exp │   │ exp │   │ exp │   │     │   │ exp │  ◄── Running      │
│   ├─────┤   ├─────┤   ├─────┤   ├─────┤   ├─────┤                   │
│   │ exp │   │     │   │ exp │   │     │   │     │  ◄── Queued       │
│   │ exp │   │     │   │     │   │     │   │     │                   │
│   └─────┘   └─────┘   └─────┘   └─────┘   └─────┘                   │
│                                                                     │
│   Incoming experiments assigned round-robin to lanes                │
│   Each lane processes its queue FIFO                                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 9.3 Future: Priority Queues

Not implemented in v0.1, but designed for:

```yaml
# Future consideration
priority:
  lanes:
    high: 2                     # Self-correct iterations
    normal: 2                   # Standard experiments
    low: 1                      # Exploration/POC
    
  assignment:
    self_correct_iteration: high
    re_verification: normal
    pr_regression: normal
    exploration: low
```

---

## 10. Artifact Lifecycle

### 10.1 Retention Policies

```yaml
artifact_lifecycle:
  policies:
    # PR-linked experiments
    pr:
      retention: 7d
      trigger_cleanup: pr_merged | pr_closed
      grace_period: 3d          # After trigger
      
    # POC/Exploration experiments
    poc:
      retention: 7d
      on_adr_created: 
        link_to: codex
        # User can flag specific artifacts for retention
      user_export:
        enabled: true
        formats: [tar.gz, json]
        
    # Evidence re-verification
    re_verification:
      retention: indefinite     # While evidence exists
      linked_to: evidence_id
      on_evidence_deprecated: archive_30d
      
    # Ad-hoc experiments
    ad_hoc:
      retention: 24h
      user_export:
        enabled: true
```

### 10.2 Storage Tiers

```yaml
storage:
  hot: 
    location: s3://sandbox-artifacts/hot/
    retention: 7d
    access: immediate
    
  cold:
    location: s3://sandbox-artifacts/cold/
    retention: 90d
    access: minutes
    
  archive:
    location: s3://sandbox-artifacts-archive/
    retention: 365d
    access: hours
```

### 10.3 Cleanup Process

```yaml
cleanup:
  schedule: daily at 02:00 UTC
  
  process:
    1. Scan experiments past retention
    2. Check for Codex links (skip if linked)
    3. Check for user export flags (skip if flagged)
    4. Move to cold/archive or delete
    5. Update experiment record (artifacts_purged: true)
```

---

## 11. Deployment & Operations

### 11.1 Authentication

**Baseline: Session-scoped tokens issued by EDI**

```yaml
authentication:
  method: bearer_token
  issuer: edi_session_manager
  lifetime: 1h                    # Matches typical session length
  scope: experiment:create,experiment:read,experiment:delete
  binding: session_id             # Token tied to EDI session
  
  implementation:
    format: JWT (RS256)
    signing_key: EDI's private key (stored in secret manager)
    validation: Sandbox has EDI's public key
    claims:
      sub: user_id
      session: session_id
      project: project_id
      exp: timestamp
      scope: [experiment:*]
```

**Flow:**

```
1. User starts EDI session
2. EDI generates session token with sandbox scope
3. Token passed to Sandbox API on each request
4. Sandbox validates signature + expiry
5. Token expires with session (or earlier)
```

**Why this baseline:**

| Property | Benefit |
|----------|---------|
| No external auth service | Day 1 simplicity |
| Session-scoped | Automatic expiry, no token management |
| JWT | Stateless validation, no round-trip |
| Project-scoped | Natural isolation |

**Scale path:**

| Trigger | Evolution |
|---------|-----------|
| Multiple EDI instances | Shared signing key (secret manager) or JWKS endpoint |
| Enterprise SSO | OIDC integration; EDI becomes relying party |
| Fine-grained permissions | Add roles (admin, developer, viewer) to token claims |
| Audit requirements | Add token ID claim, log all API calls with token metadata |

### 11.2 Multi-tenancy

**Baseline: Soft isolation with namespaced resources**

```yaml
multi_tenancy:
  isolation_level: namespace
  
  # Network isolation
  network:
    strategy: per_experiment_network
    naming: sandbox-{project_id}-{experiment_id}
    # Each experiment gets isolated Docker network
    # No cross-experiment communication possible
    
  # Resource isolation  
  resources:
    strategy: cgroups_limits
    per_experiment:
      cpu: 2 cores max
      memory: 4Gi max
      pids: 100 max              # Fork bomb protection
      disk_io: 50MB/s max
    per_project:
      concurrent_experiments: 3   # Fairness across projects
      
  # Storage isolation
  storage:
    strategy: prefixed_paths
    pattern: /sandbox-data/{project_id}/{experiment_id}/
    cleanup: per_lifecycle_policy
    
  # Container naming
  containers:
    naming: {project_id}-{experiment_id}-{service_name}
    labels:
      sandbox.project: {project_id}
      sandbox.experiment: {experiment_id}
      sandbox.session: {session_id}
```

**Why this baseline:**

| Property | Benefit |
|----------|---------|
| Single Docker host | Simple ops, no orchestrator needed |
| Network namespacing | Experiments can't interfere |
| Resource limits | One runaway can't starve others |
| Per-project limits | Fairness without complexity |

**Scale path:**

| Trigger | Evolution |
|---------|-----------|
| Resource contention | Multiple Docker hosts with experiment routing |
| Compliance/security | Hard isolation via Firecracker microVMs or dedicated hosts |
| Global scale | Kubernetes with namespace-per-project, NetworkPolicies |
| Cost optimization | Spot instances for experiment hosts, auto-scaling pool |

```
┌─────────────────────────────────────────────────────────────────────┐
│                     ISOLATION EVOLUTION                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   Day 1              Scale               Enterprise                 │
│   ─────              ─────               ──────────                 │
│                                                                     │
│   ┌─────────┐        ┌─────────┐         ┌─────────┐               │
│   │ Docker  │        │ Docker  │         │ K8s     │               │
│   │ Host    │        │ Hosts   │         │ Cluster │               │
│   │         │        │ (pool)  │         │         │               │
│   │ ┌─────┐ │        │ ┌─────┐ │         │ ┌─────┐ │               │
│   │ │net-1│ │        │ │host1│ │         │ │ns-A │ │               │
│   │ ├─────┤ │   →    │ ├─────┤ │    →    │ ├─────┤ │               │
│   │ │net-2│ │        │ │host2│ │         │ │ns-B │ │               │
│   │ ├─────┤ │        │ ├─────┤ │         │ ├─────┤ │               │
│   │ │net-3│ │        │ │host3│ │         │ │ns-C │ │               │
│   │ └─────┘ │        │ └─────┘ │         │ └─────┘ │               │
│   └─────────┘        └─────────┘         └─────────┘               │
│                                                                     │
│   Namespace          Host-level          Namespace +                │
│   isolation          isolation           NetworkPolicy +            │
│                                          ResourceQuota              │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 11.3 Image Registry

**Baseline: Local cache with pull-through to public registries**

```yaml
image_registry:
  strategy: pull_through_cache
  
  # Base images (language runtimes)
  base_images:
    source: docker.io
    cache: local_daemon         # Docker's built-in cache
    pull_policy: if_not_present
    pinned_versions:            # Explicit version control
      python: ["3.10", "3.11", "3.12"]
      node: ["18", "20", "22"]
      go: ["1.21", "1.22"]
      java: ["17", "21"]
      rust: ["1.75", "1.76"]
      
  # Infrastructure images (sidecars)
  infra_images:
    source: docker.io
    cache: local_daemon
    images:
      - toxiproxy:2.5
      - otel/opentelemetry-collector:0.90
      - grafana/loki:2.9
      - prom/prometheus:2.48
      - postgres:15
      - redis:7
      - localstack/localstack:3  # AWS service mocking
      
  # Project code: never built into images
  project_code:
    strategy: volume_mount
    # Code mounted at runtime, not baked into images
    # Avoids image sprawl, faster iteration
    
  # Pre-warming (optional optimization)
  pre_warm:
    on_sandbox_start:
      - python:3.11
      - node:20
      - postgres:15
      - redis:7
    strategy: background_pull
```

**Why this baseline:**

| Property | Benefit |
|----------|---------|
| No registry infrastructure | Nothing to manage |
| Docker's native caching | Handles most needs |
| Volume mounting | Avoids per-PR image builds |
| Pinned versions | Reproducibility without registry |

**Scale path:**

| Trigger | Evolution |
|---------|-----------|
| Slow pulls from Docker Hub | Deploy registry mirror (Harbor, registry:2) |
| Private base images needed | Private registry with pull-through |
| Air-gapped environment | Full mirror of required images |
| Image scanning requirements | Harbor + Trivy, scan on pull |
| Multi-region | Regional registry mirrors |

### 11.4 Secrets Handling

**Baseline: Encrypted environment variables with experiment-scoped lifetime**

```yaml
secrets_handling:
  storage:
    method: encrypted_file
    location: /sandbox-secrets/{project_id}/secrets.enc
    encryption: AES-256-GCM
    key_source: SANDBOX_MASTER_KEY env var
    
  # Secret definition (by user/EDI)
  secret_definition:
    scope: project              # Secrets are per-project
    format:
      name: string
      value: encrypted_string
      allowed_experiments: 
        - type: [pr, poc, verification]
      expires_at: timestamp?
      
  # Injection into experiments
  injection:
    method: environment_variable
    timing: container_start     # Injected at runtime, not in image
    visibility: 
      in_logs: redacted         # Auto-redact in log output
      in_traces: redacted
      in_results: never         # Never in API responses
      
  # Lifecycle
  lifecycle:
    experiment_end: 
      action: env_cleared       # Container destroyed = secrets gone
    project_secret_rotation:
      method: manual
      notification: 30d_before_expiry
```

**Secret flow:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        SECRET LIFECYCLE                             │
│                                                                     │
│   1. User/EDI defines secret                                        │
│      POST /projects/{id}/secrets                                    │
│      { name: "STRIPE_TEST_KEY", value: "sk_test_..." }              │
│                         │                                           │
│                         ▼                                           │
│   2. Sandbox encrypts and stores                                    │
│      /sandbox-secrets/proj-123/secrets.enc                          │
│                         │                                           │
│                         ▼                                           │
│   3. Experiment requests secret                                     │
│      topology.services.app.secrets: [STRIPE_TEST_KEY]               │
│                         │                                           │
│                         ▼                                           │
│   4. Injected at container start                                    │
│      docker run -e STRIPE_TEST_KEY=sk_test_...                      │
│      (decrypted in memory, never written to disk)                   │
│                         │                                           │
│                         ▼                                           │
│   5. Container destroyed = secret gone                              │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Why this baseline:**

| Property | Benefit |
|----------|---------|
| No Vault/HSM infrastructure | Day 1 simplicity |
| Encrypted at rest | Safe on disk |
| Env var injection | Standard pattern, works with any app |
| Auto-redaction | Prevents accidental exposure |
| Experiment-scoped | Automatic cleanup |

**Scale path:**

| Trigger | Evolution |
|---------|-----------|
| Compliance (SOC2, etc.) | HashiCorp Vault or cloud KMS |
| Dynamic secrets needed | Vault dynamic secrets (DB creds, cloud tokens) |
| Secret rotation | Vault lease-based rotation |
| Audit trail | Vault audit log, every secret access logged |
| Multi-team | Vault policies per team/project |

---

## 12. Integration Points

### 12.1 EDI → Sandbox

```yaml
edi_to_sandbox:
  trigger: experiment_request
  
  authentication:
    method: bearer_token
    token_source: edi_session
    
  payload: ExperimentDefinition
  
  response:
    sync: experiment_id + queue_position
    async: webhook on completion (optional)
```

### 12.2 Sandbox → Integration Layer

```yaml
sandbox_to_integration:
  on_complete:
    emit: experiment_result
    to: integration_layer
    
  integration_routing:
    # Evidence re-verification
    - condition: context.type == "re_verification"
      action: update_evidence
      target: codex
      payload:
        evidence_id: context.evidence_id
        status: result.status
        new_measurement: result.assertions[0].actual
        
    # Failure attribution
    - condition: result.status == "failed"
      action: trigger_judge
      target: llm_judge
      payload:
        result: ExperimentResult
        context: ExperimentContext
        
    # PR status update
    - condition: context.type == "pr"
      action: update_pr_status
      target: github | gitlab
      payload:
        pr_id: context.pr_id
        status: result.status
        summary: result.summary
        
    # Always: archive telemetry
    - always: true
      action: archive_telemetry
      target: flight_recorder
```

### 12.3 MCP Server Interface

For Claude Code integration:

```yaml
mcp_server:
  name: sandbox
  
  tools:
    - name: sandbox_run_experiment
      description: "Run a behavioral experiment in Sandbox"
      parameters:
        experiment: ExperimentDefinition
      returns:
        experiment_id: string
        status: queued | running
        
    - name: sandbox_get_result
      description: "Get experiment result"
      parameters:
        experiment_id: string
      returns:
        result: ExperimentResult
        
    - name: sandbox_stream_logs
      description: "Stream logs from running experiment"
      parameters:
        experiment_id: string
        services: string[]?
      returns:
        stream: AsyncIterator[LogLine]
        
    - name: sandbox_queue_status
      description: "Check queue status"
      parameters: {}
      returns:
        total_queued: int
        estimated_wait: duration
```

---

## 13. Scale Evolution Guide

### 13.1 When to Scale

| Trigger | Indicator | Action |
|---------|-----------|--------|
| Queue wait > 5 min average | Users complaining about delays | Add lanes or Docker hosts |
| CPU/memory contention | Experiments timing out | Increase host resources or add hosts |
| Docker Hub rate limits | 429 errors on pulls | Deploy local registry mirror |
| Audit requirements | Compliance review | Add Vault, detailed logging |
| Multi-region teams | Latency complaints | Regional Sandbox instances |

### 13.2 Evolution Paths

```
┌─────────────────────────────────────────────────────────────────────┐
│                     SCALE EVOLUTION PATHS                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   COMPUTE                                                           │
│   ───────                                                           │
│   Single Host → Multiple Hosts → Kubernetes → Multi-region          │
│                                                                     │
│   ISOLATION                                                         │
│   ─────────                                                         │
│   Namespace → Host-level → Firecracker → Dedicated Clusters         │
│                                                                     │
│   IMAGES                                                            │
│   ──────                                                            │
│   Docker Hub → Local Mirror → Private Registry → Multi-region       │
│                                                                     │
│   SECRETS                                                           │
│   ───────                                                           │
│   Encrypted File → Vault → Vault + Dynamic Secrets → HSM            │
│                                                                     │
│   AUTH                                                              │
│   ────                                                              │
│   EDI JWT → Shared JWKS → OIDC/SSO → mTLS + RBAC                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 13.3 Kubernetes Migration (When Needed)

```yaml
# Future: Kubernetes deployment
kubernetes:
  namespace_per_project: true
  
  experiment_pod:
    spec:
      containers:
        - name: app
          # ... from topology
        - name: otel-collector
          # ... sidecar
      securityContext:
        runAsNonRoot: true
        readOnlyRootFilesystem: true
        
  network_policy:
    # Isolate experiment pods
    podSelector:
      matchLabels:
        sandbox.experiment: "*"
    ingress: []                 # No ingress from other pods
    egress:
      - to:
          - podSelector:
              matchLabels:
                sandbox.experiment: ${SAME_EXPERIMENT_ID}
                
  resource_quota:
    # Per-project limits
    hard:
      requests.cpu: "10"
      requests.memory: "20Gi"
      pods: "20"
```

---

## 14. Decision Log

| Decision | Date | Rationale |
|----------|------|-----------|
| Deterministic infrastructure (no AI in Sandbox) | Jan 24, 2026 | Clear separation of concerns; Claude decides, Sandbox executes |
| Docker containers, not VMs | Jan 24, 2026 | Fast startup, good isolation, familiar tooling |
| Volume mount code, don't build images | Jan 24, 2026 | Avoid image sprawl, faster iteration, consistency across experiment types |
| Multi-strategy fault injection | Jan 24, 2026 | Different scenarios need different fault types |
| Rich experiment results (full logs/traces) | Jan 24, 2026 | Claude needs actionable feedback for self-correction |
| Artifact lifecycle tied to purpose | Jan 24, 2026 | PR artifacts expire; Codex-linked persist |
| Multi-lane FIFO queue (5 lanes) | Jan 24, 2026 | Simple fairness without priority complexity |
| 3 self-correct iterations default | Jan 24, 2026 | Balances thoroughness with cost/time |
| JWT auth from EDI | Jan 24, 2026 | No external auth infra needed day 1 |
| Namespace isolation baseline | Jan 24, 2026 | Single host simplicity with clear scale path |
| Local Docker cache, no private registry | Jan 24, 2026 | No infra to manage; scale when needed |
| Encrypted file secrets | Jan 24, 2026 | Secure enough for day 1; Vault when compliance requires |

---

## Appendix A: Example Experiment

Complete example of a regression experiment for a payment service PR:

```yaml
experiment:
  id: exp-2026-01-24-payment-retry
  name: "Verify payment retry logic with network partition"
  type: regression
  
  context:
    type: pr
    pr_id: "gh-456"
    session_id: "edi-session-abc123"
    
  topology:
    runtime:
      language: python
      version: "3.11"
      
    code:
      source: git
      ref: feature/payment-retry
      path: /src
      mount_point: /app
      
    services:
      - name: db
        image: postgres:15
        ports: [5432]
        env:
          POSTGRES_DB: payments_test
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        healthcheck:
          test: pg_isready
          interval: 5s
          retries: 5
          
      - name: redis
        image: redis:7
        ports: [6379]
        
      - name: payment-gateway-mock
        image: wiremock/wiremock:3
        ports: [8080]
        volumes:
          - source: fixtures/wiremock
            target: /home/wiremock/mappings
            
    fixtures:
      - name: seed_orders
        type: sql
        source: file
        content: fixtures/orders.sql
        target:
          service: db
          path: /docker-entrypoint-initdb.d/
          
  scenario:
    name: "Payment retry under network partition"
    
    phases:
      - name: setup
        actions:
          - type: wait_healthy
            target: db
            timeout: 30s
          - type: wait_healthy
            target: redis
            timeout: 10s
          - type: wait_healthy
            target: payment-gateway-mock
            timeout: 10s
          - type: shell_exec
            target: app
            command: "python -m pytest --collect-only"  # Verify tests found
            
      - name: baseline
        actions:
          - type: http_request
            target: app
            method: POST
            path: /api/payments
            body:
              order_id: "ord-001"
              amount: 100.00
            capture_as: baseline_response
            
      - name: inject_partition
        actions:
          - type: inject_fault
            strategy: network
            fault:
              type: partition
              between: [app, payment-gateway-mock]
            duration: 10s
            
          - type: http_request
            target: app
            method: POST
            path: /api/payments
            body:
              order_id: "ord-002"
              amount: 200.00
            capture_as: partition_response
            
          - type: wait
            duration: 15s  # Let retry logic run
            
      - name: verify_recovery
        actions:
          - type: http_request
            target: app
            method: GET
            path: /api/orders/ord-002
            capture_as: final_state
            
  assertions:
    - name: "Baseline payment succeeds"
      type: response_check
      config:
        source: captured.baseline_response
        status: 200
        body_path: "$.status"
        body_condition: "== 'completed'"
        
    - name: "Partition payment eventually succeeds"
      type: state_check
      config:
        source: captured.final_state
        path: "$.payment_status"
        condition: "== 'completed'"
        
    - name: "Retry attempts logged"
      type: log_contains
      config:
        source: app.logs
        pattern: "payment retry attempt"
        count: ">= 2"
        
    - name: "Circuit breaker activated"
      type: metric_threshold
      config:
        source: app.metrics
        metric: circuit_breaker_state
        labels:
          service: payment-gateway
        condition: "max == 1"  # Was opened at some point
        
    - name: "No duplicate charges"
      type: trace_analysis
      config:
        source: traces
        span_name: "payment.charge"
        attribute_filter:
          order_id: "ord-002"
        condition: "count == 1"
        
  execution:
    timeout: 300s
    resource_limits:
      cpu: "2.0"
      memory: "4Gi"
```

---

## Appendix B: Related Documents

- [AEF Architecture Specification v0.5](./aef-architecture-specification-v0_5.md)
- [EDI Specification Plan](./edi-specification-plan.md)
- [EDI Learning Architecture Spec](./edi-learning-architecture-spec.md)
- [Codex Architecture Deep Dive](./codex-architecture-deep-dive.md)

---

## Appendix C: Glossary

| Term | Definition |
|------|------------|
| **Experiment** | Unit of work in Sandbox; includes topology, scenario, and assertions |
| **Topology** | Service graph for an experiment (app + dependencies + sidecars) |
| **Scenario** | Sequence of phases and actions to execute |
| **Assertion** | Expected condition to verify; evaluated against telemetry |
| **Fault Injection** | Intentional introduction of failures (network, kernel, application) |
| **Lane** | Independent execution queue slot in the multi-lane queue |
| **Evidence** | Codex knowledge type; Sandbox experiments can create/verify Evidence |
| **Flight Recorder** | Telemetry archive for forensics and learning |
