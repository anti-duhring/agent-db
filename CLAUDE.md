<!-- GSD:project-start source:PROJECT.md -->
## Project

**Agent DB — Chat Storage Benchmark**

A benchmark harness in Go that evaluates three database candidates (Postgres, DynamoDB, Turso) for storing LLM chat conversations. The harness runs identical scenarios against each backend, measures performance, and produces a comparison report with latency metrics and cost projections. The output is both a CLI benchmark tool and a written recommendation document for team review.

**Core Value:** Produce data-backed evidence that the team can use to choose the right storage engine for user-scoped LLM chat conversations — not opinions, not guesses, measured numbers.

### Constraints

- **Language**: Go — matches the team's existing stack and expertise
- **Cloud**: AWS — all infra runs on AWS; Postgres is RDS, DynamoDB is native
- **Turso**: External dependency outside AWS VPC — latency penalty expected and acceptable for evaluation purposes
- **Data model**: Conversations scoped by (partner_id, user_id) with time-ordered messages per conversation
- **Benchmark fairness**: Same interface, same data, same scenarios against all three backends
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### Language Runtime
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.26 (stable as of 2026-02-10) | Language runtime | Matches team stack; 1.26 is current stable with 1.24/1.25 in supported window |
### Database Drivers
| Driver | Version | Backend | Why |
|--------|---------|---------|-----|
| `github.com/jackc/pgx/v5` | v5.9.1 (2026-03-22) | Postgres | Native pgx protocol, no C dependencies, highest throughput of any Go Postgres driver; includes `pgxpool` for connection pooling |
| `github.com/aws/aws-sdk-go-v2/service/dynamodb` | v1.57.1 (2026-03-26) | DynamoDB | Official AWS SDK v2; v1 (old SDK) is maintenance-only and should not be used for new code |
| `github.com/aws/aws-sdk-go-v2/config` | (same module, latest) | DynamoDB auth | Credential chain loading — picks up env vars, instance profiles, and local AWS config automatically |
| `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` | (same module) | DynamoDB | Struct marshaling via `dynamodbav` struct tags; eliminates manual `AttributeValue` construction |
| `github.com/tursodatabase/libsql-client-go/libsql` | v0.0.0-20251219 | Turso | Remote-only driver, no CGO required; connects via `libsql://[DB].turso.io?authToken=[TOKEN]`; implements `database/sql` interface |
#### Postgres: use pgxpool, not pgx directly
#### DynamoDB: use expression builder, not raw map construction
### Latency Measurement
| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/HdrHistogram/hdrhistogram-go` | v1.2.0 (2025-11-09) | p50/p95/p99 per scenario | HDR Histogram maintains O(1) recording cost and fixed memory regardless of sample count; purpose-built for latency distribution tracking |
### Synthetic Data Generation
| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/brianvoe/gofakeit/v7` | v7.14.1 (2026-03-03) | Chat message content, user IDs, conversation metadata | Seeded, reproducible, 310+ generation functions including `UUID()`, `Sentence()`, `Word()`; supports deterministic seeds for benchmark reproducibility |
### Local Testing Infrastructure
| Tool | Version | Purpose | Why |
|------|---------|---------|-----|
| `github.com/testcontainers/testcontainers-go/modules/postgres` | latest | Postgres integration testing | Spins up real Postgres in Docker; no mocking needed; cleans up automatically |
| `github.com/testcontainers/testcontainers-go/modules/localstack` | latest | DynamoDB local testing | LocalStack emulates DynamoDB free tier; standard pattern for Go + DynamoDB integration tests |
| Turso Cloud (dev database) | n/a | Turso benchmark target | No local emulator for Turso; use a dedicated dev/staging database on Turso Cloud. Latency measured over real internet is the point. |
### Report Output
| Approach | Version | Purpose | Why |
|----------|---------|---------|-----|
| `encoding/json` (stdlib) | Go 1.26 stdlib | Structured JSON output | No dependency needed; for a benchmark harness producing a single report file, stdlib JSON is sufficient and has zero overhead risk |
| `text/tabwriter` (stdlib) | Go 1.26 stdlib | Human-readable terminal table | Already in stdlib; formats p50/p95/p99 tables cleanly without a dependency |
### CLI
| Approach | Version | Purpose | Why |
|----------|---------|---------|-----|
| `flag` (stdlib) | Go 1.26 stdlib | CLI flag parsing | This is a single-binary benchmark tool with ~5 flags (backend selector, iterations, concurrency, output path, seed). `flag` is sufficient. `cobra` adds 1000+ lines of dep graph for no benefit here. |
## Full Dependency Summary
# Core drivers
# Measurement
# Data generation
# Test infrastructure
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
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
