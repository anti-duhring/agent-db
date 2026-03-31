# Technology Stack

**Project:** Agent DB — Chat Storage Benchmark
**Researched:** 2026-03-31
**Research mode:** Ecosystem

---

## Recommended Stack

### Language Runtime

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.26 (stable as of 2026-02-10) | Language runtime | Matches team stack; 1.26 is current stable with 1.24/1.25 in supported window |

Use `go 1.26` in `go.mod`. The AWS SDK v2 minimum is Go 1.23, pgx v5.9 minimum is Go 1.25 — so Go 1.26 satisfies all dependencies cleanly.

---

### Database Drivers

| Driver | Version | Backend | Why |
|--------|---------|---------|-----|
| `github.com/jackc/pgx/v5` | v5.9.1 (2026-03-22) | Postgres | Native pgx protocol, no C dependencies, highest throughput of any Go Postgres driver; includes `pgxpool` for connection pooling |
| `github.com/aws/aws-sdk-go-v2/service/dynamodb` | v1.57.1 (2026-03-26) | DynamoDB | Official AWS SDK v2; v1 (old SDK) is maintenance-only and should not be used for new code |
| `github.com/aws/aws-sdk-go-v2/config` | (same module, latest) | DynamoDB auth | Credential chain loading — picks up env vars, instance profiles, and local AWS config automatically |
| `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` | (same module) | DynamoDB | Struct marshaling via `dynamodbav` struct tags; eliminates manual `AttributeValue` construction |
| `github.com/tursodatabase/libsql-client-go/libsql` | v0.0.0-20251219 | Turso | Remote-only driver, no CGO required; connects via `libsql://[DB].turso.io?authToken=[TOKEN]`; implements `database/sql` interface |

**Confidence:** HIGH (pgx, AWS SDK) | MEDIUM (Turso driver, noted below)

**Turso driver caveat:** `libsql-client-go` is deprecated upstream in favor of `go-libsql`, but `go-libsql` requires `CGO_ENABLED=1` and ships precompiled C libraries (linux amd64/arm64, darwin amd64/arm64 only). For a benchmark harness that runs in CI and needs simple builds, the `libsql-client-go/libsql` remote-only driver is the pragmatic choice — it is pure Go, still functional for remote Turso Cloud connections, and sufficient for latency measurement. If embedded replica semantics were needed (they are not here), `go-libsql` would be mandatory.

#### Postgres: use pgxpool, not pgx directly

```go
import "github.com/jackc/pgx/v5/pgxpool"

pool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_DSN"))
```

The benchmark runs concurrent scenarios. `pgxpool` manages a pool of underlying `*pgx.Conn` connections automatically. The project context notes this is a long-running service deployment model — pgxpool is correct here. Use `database/sql` only if ORM compatibility is needed; for raw benchmark control, native pgx is preferred.

#### DynamoDB: use expression builder, not raw map construction

```go
import (
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
)
```

The expression builder prevents injection-class bugs in key condition expressions and is the AWS-recommended pattern as of SDK v2.

---

### Latency Measurement

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/HdrHistogram/hdrhistogram-go` | v1.2.0 (2025-11-09) | p50/p95/p99 per scenario | HDR Histogram maintains O(1) recording cost and fixed memory regardless of sample count; purpose-built for latency distribution tracking |

**Do not use `testing.B` for this harness.** `testing.B` reports ops/sec and ns/op averages — it does not capture percentile distributions. The requirement is p50/p95/p99 per scenario per backend. HDR Histogram records every sample and answers `ValueAtPercentiles([]float64{50, 95, 99})` in a single call.

**Confidence:** HIGH — library is v1 stable, maintained by the official HdrHistogram org, API is well-defined.

Usage pattern:

```go
import hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"

// Create histogram: min 1µs, max 30s, 3 significant digits
h := hdrhistogram.New(1, 30_000_000_000, 3) // nanoseconds

start := time.Now()
err := repo.AppendMessage(ctx, msg)
h.RecordValue(time.Since(start).Nanoseconds())

pcts := h.ValueAtPercentiles([]float64{50, 95, 99})
// pcts[50.0], pcts[95.0], pcts[99.0]
```

---

### Synthetic Data Generation

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/brianvoe/gofakeit/v7` | v7.14.1 (2026-03-03) | Chat message content, user IDs, conversation metadata | Seeded, reproducible, 310+ generation functions including `UUID()`, `Sentence()`, `Word()`; supports deterministic seeds for benchmark reproducibility |

**Do not use `go-faker/faker`.** It is struct-tag-driven and adds friction for generating free-form text content. `gofakeit` has a simpler imperative API that maps directly to the "small/medium/large message profile" patterns needed here.

Seed for reproducibility:

```go
import "github.com/brianvoe/gofakeit/v7"

faker := gofakeit.New(42) // fixed seed = deterministic across runs
content := faker.Sentence(20)
userID := faker.UUID()
```

**Confidence:** HIGH — v7 is stable and actively maintained (v7.14.1 published 2026-03-03).

---

### Local Testing Infrastructure

| Tool | Version | Purpose | Why |
|------|---------|---------|-----|
| `github.com/testcontainers/testcontainers-go/modules/postgres` | latest | Postgres integration testing | Spins up real Postgres in Docker; no mocking needed; cleans up automatically |
| `github.com/testcontainers/testcontainers-go/modules/localstack` | latest | DynamoDB local testing | LocalStack emulates DynamoDB free tier; standard pattern for Go + DynamoDB integration tests |
| Turso Cloud (dev database) | n/a | Turso benchmark target | No local emulator for Turso; use a dedicated dev/staging database on Turso Cloud. Latency measured over real internet is the point. |

**Confidence:** HIGH for testcontainers (well-established in Go ecosystem) | MEDIUM for LocalStack (DynamoDB Local is alternative; either works)

---

### Report Output

| Approach | Version | Purpose | Why |
|----------|---------|---------|-----|
| `encoding/json` (stdlib) | Go 1.26 stdlib | Structured JSON output | No dependency needed; for a benchmark harness producing a single report file, stdlib JSON is sufficient and has zero overhead risk |
| `text/tabwriter` (stdlib) | Go 1.26 stdlib | Human-readable terminal table | Already in stdlib; formats p50/p95/p99 tables cleanly without a dependency |

**Do not add a third-party JSON library (jsoniter, sonic, etc.) for report output.** Performance of JSON serialization is irrelevant for a once-per-run report write. Adding dependencies for no gain violates Go's dependency hygiene norms.

For CI/artifact consumption, write a `.json` file. For human review, write to stdout with `tabwriter`. The CLI can support both simultaneously.

---

### CLI

| Approach | Version | Purpose | Why |
|----------|---------|---------|-----|
| `flag` (stdlib) | Go 1.26 stdlib | CLI flag parsing | This is a single-binary benchmark tool with ~5 flags (backend selector, iterations, concurrency, output path, seed). `flag` is sufficient. `cobra` adds 1000+ lines of dep graph for no benefit here. |

---

## Full Dependency Summary

```bash
# Core drivers
go get github.com/jackc/pgx/v5@v5.9.1
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/dynamodb@v1.57.1
go get github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue
go get github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression
go get github.com/tursodatabase/libsql-client-go/libsql

# Measurement
go get github.com/HdrHistogram/hdrhistogram-go@v1.2.0

# Data generation
go get github.com/brianvoe/gofakeit/v7

# Test infrastructure
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/testcontainers/testcontainers-go/modules/localstack
```

Stdlib only for: CLI (`flag`), report output (`encoding/json`, `text/tabwriter`), concurrency (`sync`, `context`).

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Postgres driver | pgx/v5 native | `database/sql` + pgx adapter | The adapter adds overhead and loses pgx-specific batch/copy features; for benchmarking you want the raw driver numbers |
| Postgres driver | pgx/v5 | `lib/pq` | `lib/pq` is in maintenance mode; pgx v5 is the Go community's de facto Postgres driver |
| DynamoDB driver | aws-sdk-go-v2 | aws-sdk-go (v1) | v1 is maintenance-only since 2023; new code must use v2 |
| Turso driver | libsql-client-go | go-libsql | `go-libsql` requires CGO + precompiled C libs; adds build complexity for a benchmark harness; remote-only use case doesn't need embedded replica features |
| Latency tracking | HdrHistogram | `testing.B` | `testing.B` reports averages only, not percentile distributions; wrong tool for p50/p95/p99 requirements |
| Latency tracking | HdrHistogram | prometheus histograms | Prometheus is for runtime metrics emission, not offline benchmark recording; wrong abstraction |
| Data generation | gofakeit/v7 | go-faker/faker | go-faker is struct-tag-driven — adds noise for procedural conversation generation; gofakeit has simpler imperative API |
| CLI | stdlib `flag` | cobra | cobra adds ~10 transitive dependencies and ~1000 lines of framework code for a 5-flag CLI; unjustified |
| Report output | stdlib `encoding/json` | jsoniter | jsoniter is faster but JSON serialization performance is irrelevant for a once-per-run report write |
| Local DynamoDB | LocalStack via testcontainers | DynamoDB Local (official AWS docker image) | Both are valid; LocalStack integrates cleanly with testcontainers-go which the Postgres tests will already use |

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| pgx v5.9.1 | HIGH | Verified on pkg.go.dev and official CHANGELOG |
| aws-sdk-go-v2 DynamoDB v1.57.1 | HIGH | Verified on pkg.go.dev, published 2026-03-26 |
| Turso libsql-client-go | MEDIUM | Package is deprecated upstream but still functional; go-libsql is preferred upstream but requires CGO; pragmatic choice for pure-Go build |
| HdrHistogram v1.2.0 | HIGH | Verified on pkg.go.dev and GitHub; maintained by official HdrHistogram org |
| gofakeit v7.14.1 | HIGH | Verified on pkg.go.dev, published 2026-03-03 |
| testcontainers-go | HIGH | Well-established, official Go support module |
| stdlib-only for CLI and reports | HIGH | No external libraries needed; confirmed by scope review |

---

## Sources

- [pgx GitHub — CHANGELOG](https://github.com/jackc/pgx/blob/master/CHANGELOG.md)
- [pgx pkg.go.dev](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [aws-sdk-go-v2 DynamoDB pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb)
- [attributevalue package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue)
- [Turso Go Quickstart — official docs](https://docs.turso.tech/sdk/go/quickstart)
- [go-libsql GitHub](https://github.com/tursodatabase/go-libsql)
- [libsql-client-go pkg.go.dev — deprecation notice](https://pkg.go.dev/github.com/tursodatabase/libsql-client-go)
- [HdrHistogram Go pkg.go.dev](https://pkg.go.dev/github.com/HdrHistogram/hdrhistogram-go)
- [gofakeit v7 pkg.go.dev](https://pkg.go.dev/github.com/brianvoe/gofakeit/v7)
- [testcontainers-go Postgres module](https://golang.testcontainers.org/modules/postgres/)
- [testcontainers-go LocalStack module](https://golang.testcontainers.org/modules/localstack/)
- [Go release history](https://go.dev/doc/devel/release)
