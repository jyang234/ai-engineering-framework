# Sandbox Implementation Plan

**Version**: 1.0  
**Created**: January 25, 2026  
**Target**: Claude Code execution  
**Estimated Duration**: 5 weeks

---

## Executive Summary

Sandbox provides **deterministic infrastructure for controlled experimentation** — disposable Docker environments where Claude can run integration-level behavioral verification with full observability.

### Key Principle

**Sandbox is a lab bench, not a scientist.** It provides the *where* and *how*. Claude provides the *what*. Zero intelligence in Sandbox.

### Core Capabilities

| Capability | Description |
|------------|-------------|
| Experiment execution | Disposable multi-service Docker environments |
| Fault injection | Network (tc/iptables), connection (Toxiproxy), application |
| Full telemetry | Logs, metrics, traces via OpenTelemetry |
| Assertions | Rich verification with evidence collection |
| Artifact lifecycle | Purpose-driven retention with auto-cleanup |

---

## Project Structure

```
sandbox/
├── cmd/
│   └── sandbox/
│       └── main.go              # API server entry point
├── internal/
│   ├── api/                     # HTTP API
│   │   ├── server.go
│   │   └── handlers.go
│   ├── experiment/              # Experiment lifecycle
│   │   ├── runner.go
│   │   ├── executor.go
│   │   └── setup.go
│   ├── compose/                 # Docker Compose management
│   │   ├── generator.go
│   │   └── client.go
│   ├── telemetry/               # Observability
│   │   ├── collector.go
│   │   └── otel.go
│   ├── fault/                   # Fault injection
│   │   ├── network.go
│   │   └── toxiproxy.go
│   ├── artifact/                # Artifact management
│   │   ├── storage.go
│   │   └── cleanup.go
│   └── assertion/               # Assertion engine
│       ├── engine.go
│       └── matchers.go
├── pkg/types/                   # Shared types
│   ├── experiment.go
│   └── result.go
├── templates/                   # Base configs
│   └── otel-config.yaml
├── go.mod
└── Makefile
```

---

## Timeline

| Phase | Week | Deliverables |
|-------|------|--------------|
| 1. Core Infrastructure | 1 | API server, Docker Compose generation |
| 2. Experiment Execution | 2 | Full lifecycle: setup → execute → teardown |
| 3. Fault Injection | 3 | Network faults, Toxiproxy integration |
| 4. Telemetry & Assertions | 4 | OTEL collection, assertion engine |
| 5. Artifacts & Polish | 5 | Lifecycle management, cleanup, MCP tools |

---

## Phase 1: Core Infrastructure (Week 1)

### 1.1 API Server

```go
// internal/api/server.go
package api

import "github.com/gin-gonic/gin"

type Server struct {
    router *gin.Engine
    runner *experiment.Runner
}

func NewServer(runner *experiment.Runner) *Server {
    router := gin.Default()
    s := &Server{router: router, runner: runner}
    
    router.POST("/experiments", s.CreateExperiment)
    router.GET("/experiments/:id", s.GetExperiment)
    router.DELETE("/experiments/:id", s.CancelExperiment)
    router.GET("/artifacts/:id", s.GetArtifact)
    router.GET("/health", s.Health)
    
    return s
}
```

### 1.2 Experiment Schema

```go
// pkg/types/experiment.go
type ExperimentRequest struct {
    Name        string      `json:"name"`
    Environment Environment `json:"environment"`
    Setup       []Step      `json:"setup"`
    Steps       []Step      `json:"steps"`
    Teardown    []Step      `json:"teardown"`
    Assertions  []Assertion `json:"assertions"`
    Timeout     Duration    `json:"timeout"`
    Lifecycle   Lifecycle   `json:"lifecycle"`
}

type Service struct {
    Name      string            `json:"name"`
    Image     string            `json:"image"`
    Mount     *MountConfig      `json:"mount"`  // Code mounting
    Env       map[string]string `json:"env"`
    Ports     []PortMapping     `json:"ports"`
    DependsOn []string          `json:"depends_on"`
}

type Step struct {
    Name    string       `json:"name"`
    Exec    *ExecAction  `json:"exec"`
    HTTP    *HTTPAction  `json:"http"`
    Wait    *WaitAction  `json:"wait"`
    Fault   *FaultAction `json:"fault"`
    Capture *CaptureAction `json:"capture"`
}
```

### 1.3 Docker Compose Generator

```go
// internal/compose/generator.go
func Generate(exp *ExperimentRequest) (*ComposeFile, error) {
    compose := &ComposeFile{
        Version:  "3.8",
        Services: make(map[string]ComposeService),
        Networks: map[string]Network{"sandbox": {Driver: "bridge"}},
    }
    
    // Add telemetry services
    addOTELCollector(compose)
    addPrometheus(compose)
    
    // Add user services
    for _, svc := range exp.Environment.Services {
        compose.Services[svc.Name] = convertService(&svc)
    }
    
    // Add Toxiproxy if fault injection needed
    if hasFaults(exp) {
        addToxiproxy(compose)
    }
    
    return compose, nil
}
```

### 1.4 Validation Checkpoint

- [ ] API server starts on port 8081
- [ ] POST /experiments accepts spec
- [ ] Docker Compose generated correctly
- [ ] Services start via compose

---

## Phase 2: Experiment Execution (Week 2)

### 2.1 Experiment Runner

```go
// internal/experiment/runner.go
type Runner struct {
    experiments sync.Map
    compose     *compose.Client
    telemetry   *telemetry.Collector
}

func (r *Runner) Start(ctx context.Context, req *ExperimentRequest) (*Experiment, error) {
    exp := &Experiment{
        ID:      uuid.New().String(),
        Request: req,
        Status:  StatusPending,
    }
    
    r.experiments.Store(exp.ID, exp)
    go r.run(ctx, exp)
    
    return exp, nil
}

func (r *Runner) run(ctx context.Context, exp *Experiment) {
    // Setup: generate compose, start services, wait healthy
    exp.Status = StatusSetup
    if err := r.setup(ctx, exp); err != nil {
        exp.Status = StatusFailed
        return
    }
    
    // Execute: run steps, capture results
    exp.Status = StatusRunning
    result, err := r.execute(ctx, exp)
    
    // Teardown: always cleanup
    exp.Status = StatusTeardown
    r.teardown(ctx, exp)
    
    // Complete
    exp.Status = StatusCompleted
    exp.Result = result
}
```

### 2.2 Step Executor

```go
// internal/experiment/executor.go
func (r *Runner) executeStep(ctx context.Context, exp *Experiment, step *Step) (interface{}, error) {
    switch {
    case step.Exec != nil:
        return r.compose.Exec(ctx, exp.Project, step.Exec.Service, step.Exec.Command)
    case step.HTTP != nil:
        return r.executeHTTP(ctx, step.HTTP)
    case step.Wait != nil:
        time.Sleep(step.Wait.Duration)
        return nil, nil
    case step.Fault != nil:
        return r.executeFault(ctx, exp, step.Fault)
    }
    return nil, nil
}
```

### 2.3 Validation Checkpoint

- [ ] Services start and pass healthchecks
- [ ] Exec steps run in containers
- [ ] HTTP steps execute requests
- [ ] Wait steps pause correctly
- [ ] Teardown cleans up all resources

---

## Phase 3: Fault Injection (Week 3)

### 3.1 Network Faults (tc/iptables)

```go
// internal/fault/network.go
type NetworkFaultInjector struct {
    compose *compose.Client
}

func (n *NetworkFaultInjector) ApplyLatency(ctx context.Context, project, service string, ms int) error {
    cmd := []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", 
                    "delay", fmt.Sprintf("%dms", ms)}
    _, err := n.compose.Exec(ctx, project, service, cmd, nil)
    return err
}

func (n *NetworkFaultInjector) ApplyPacketLoss(ctx context.Context, project, service string, percent float64) error {
    cmd := []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem",
                    "loss", fmt.Sprintf("%.2f%%", percent)}
    _, err := n.compose.Exec(ctx, project, service, cmd, nil)
    return err
}

func (n *NetworkFaultInjector) ApplyPartition(ctx context.Context, project, service, from string) error {
    cmd := []string{"iptables", "-A", "INPUT", "-s", from, "-j", "DROP"}
    _, err := n.compose.Exec(ctx, project, service, cmd, nil)
    return err
}
```

### 3.2 Toxiproxy Faults

```go
// internal/fault/toxiproxy.go
func (t *ToxiproxyInjector) ApplyTimeout(ctx context.Context, proxy string, ms int) error {
    p, _ := t.client.Proxy(proxy)
    _, err := p.AddToxic("timeout", "timeout", "downstream", 1.0, 
        toxiproxy.Attributes{"timeout": ms})
    return err
}

func (t *ToxiproxyInjector) ApplyBandwidth(ctx context.Context, proxy string, kbps int) error {
    p, _ := t.client.Proxy(proxy)
    _, err := p.AddToxic("bandwidth", "bandwidth", "downstream", 1.0,
        toxiproxy.Attributes{"rate": kbps * 1024})
    return err
}
```

### 3.3 Validation Checkpoint

- [ ] Network latency injection works
- [ ] Packet loss works
- [ ] Network partition works
- [ ] Toxiproxy connection faults work
- [ ] Faults removed on teardown

---

## Phase 4: Telemetry & Assertions (Week 4)

### 4.1 OTEL Configuration

```yaml
# templates/otel-config.yaml
receivers:
  otlp:
    protocols:
      grpc: {endpoint: "0.0.0.0:4317"}

processors:
  batch: {timeout: 1s}

exporters:
  file/logs: {path: /var/log/sandbox/logs.jsonl}
  file/traces: {path: /var/log/sandbox/traces.jsonl}
  prometheus: {endpoint: "0.0.0.0:8889"}

service:
  pipelines:
    logs: {receivers: [otlp], processors: [batch], exporters: [file/logs]}
    traces: {receivers: [otlp], processors: [batch], exporters: [file/traces]}
    metrics: {receivers: [otlp], processors: [batch], exporters: [prometheus]}
```

### 4.2 Assertion Engine

```go
// internal/assertion/engine.go
type Engine struct {
    telemetry *telemetry.Collector
}

func (e *Engine) Run(ctx context.Context, assertions []Assertion, captures map[string]interface{}) []AssertionResult {
    results := make([]AssertionResult, len(assertions))
    for i, a := range assertions {
        results[i] = e.runAssertion(ctx, &a, captures)
    }
    return results
}

func (e *Engine) runAssertion(ctx context.Context, a *Assertion, captures map[string]interface{}) AssertionResult {
    switch a.Type {
    case "state_check":
        return e.checkState(a, captures)
    case "log_contains":
        return e.checkLogContains(ctx, a)
    case "log_absent":
        return e.checkLogAbsent(ctx, a)
    case "metric_threshold":
        return e.checkMetricThreshold(ctx, a)
    case "trace_analysis":
        return e.checkTraceAnalysis(ctx, a)
    case "exit_code":
        return e.checkExitCode(a, captures)
    }
    return AssertionResult{Status: "error"}
}
```

### 4.3 Validation Checkpoint

- [ ] OTEL collector receives logs
- [ ] Traces captured with spans
- [ ] Metrics queryable
- [ ] All assertion types work
- [ ] Evidence attached to failures

---

## Phase 5: Artifacts & Polish (Week 5)

### 5.1 Artifact Lifecycle

```go
// internal/artifact/storage.go
type Lifecycle struct {
    Purpose     string // debug, pr, evidence
    LinkedPR    string
    LinkedCodex string
    RetainDays  int
}

// Retention defaults
// debug: 7 days
// pr: 30 days (or until PR closed)
// evidence: 365 days (Codex-linked)

func (s *Storage) Store(ctx context.Context, expID, artifactType string, reader io.Reader, lifecycle Lifecycle) (*Artifact, error) {
    // Calculate expiry based on purpose
    var retainDays int
    switch lifecycle.Purpose {
    case "debug": retainDays = 7
    case "pr": retainDays = 30
    case "evidence": retainDays = 365
    }
    
    // Store with metadata
    // ...
}
```

### 5.2 Cleanup Job

```go
// internal/artifact/cleanup.go
func (c *CleanupJob) run(ctx context.Context) {
    expired, _ := c.storage.FindExpired(ctx)
    for _, artifact := range expired {
        // Don't delete Codex-linked evidence
        if artifact.Lifecycle.LinkedCodex != "" {
            continue
        }
        os.Remove(artifact.Path)
        c.storage.Delete(ctx, artifact.ID)
    }
}
```

### 5.3 Validation Checkpoint

- [ ] Artifacts stored with lifecycle
- [ ] Cleanup deletes expired
- [ ] Evidence persists when Codex-linked
- [ ] Full workflow works end-to-end

---

## Dependencies

```go
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/google/uuid v1.6.0
    github.com/Shopify/toxiproxy/v2 v2.8.0
    github.com/tidwall/gjson v1.17.0
    gopkg.in/yaml.v3 v3.0.1
)
```

**System**: Docker, Docker Compose, tc, iptables

---

## Environment Variables

```bash
SANDBOX_DATA_PATH=/var/lib/sandbox
SANDBOX_API_PORT=8081
SANDBOX_CLEANUP_INTERVAL=1h
SANDBOX_DEFAULT_TIMEOUT=5m
```
