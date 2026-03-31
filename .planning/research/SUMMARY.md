# Project Research Summary

**Project:** Agent DB — Chat Storage Benchmark
**Domain:** Go CLI benchmark harness for database comparison
**Researched:** 2026-03-31
**Confidence:** HIGH

## Executive Summary

Agent DB is a Go benchmark harness that runs identical workloads against three database backends — Postgres (AWS RDS), DynamoDB, and Turso (edge SQLite over HTTP) — to produce data-backed evidence for a storage decision for LLM chat conversations. The project is scoped as a benchmark tool, not a deployable service: no HTTP layer, no auth, no ORM. Experts build this class of tool by defining a single backend-agnostic interface first, then building adapters behind it, so the benchmark scenarios never vary between backends and the comparison is strictly fair. The central architectural discipline is keeping setup, warm-up, and measurement in separate phases so only pure storage I/O is timed.

The recommended stack is Go 1.26, pgx/v5 with pgxpool for Postgres, aws-sdk-go-v2 for DynamoDB, and libsql-client-go for Turso (pure Go, no CGO — the right tradeoff for a benchmark harness). Latency measurement uses HDR Histogram rather than `testing.B`, which only reports averages and cannot produce p50/p95/p99 distributions. Synthetic data comes from gofakeit/v7 with a fixed seed for reproducibility. Report output uses stdlib only (encoding/json, text/tabwriter). The full dependency footprint is intentionally minimal.

The biggest risks are methodological, not technical. Three pitfalls can invalidate the entire comparison and must be designed against from the start: (1) coordinated omission in concurrent-write timing, (2) DynamoDB on-demand cold-start throttling poisoning early measurements, and (3) DynamoDB access pattern mistakes (Scan instead of Query) that make DynamoDB look slow for a use case it handles well when designed correctly. A fourth cross-cutting concern is that Turso's latency penalty reflects transport topology (HTTP over internet from AWS), not database engine quality — the report must label this clearly or results will be misread.

---

## Key Findings

### Recommended Stack

Go 1.26 satisfies all driver minimum requirements (pgx v5.9 requires Go 1.25; AWS SDK v2 requires Go 1.23). The stack is intentionally lean: three third-party database drivers, one measurement library, one data generation library, one test infrastructure library (testcontainers-go), and stdlib for everything else. `cobra` and `jsoniter` are explicitly not recommended — the scope does not justify the dependency overhead.

**Core technologies:**

- `github.com/jackc/pgx/v5` (v5.9.1) — Postgres driver — native pgx protocol, no C deps, highest throughput; pgxpool required for concurrent scenarios
- `github.com/aws/aws-sdk-go-v2/service/dynamodb` (v1.57.1) — DynamoDB — official v2 SDK; use expression builder, not raw map construction
- `github.com/tursodatabase/libsql-client-go/libsql` — Turso — deprecated upstream but pure Go (no CGO); the correct choice for a benchmark that runs in CI and targets remote-only Turso Cloud
- `github.com/HdrHistogram/hdrhistogram-go` (v1.2.0) — latency measurement — O(1) recording, accurate p50/p95/p99; do NOT use `testing.B` for this
- `github.com/brianvoe/gofakeit/v7` (v7.14.1) — synthetic data — seeded, deterministic, imperative API suited to procedural conversation generation
- `github.com/testcontainers/testcontainers-go` — integration tests — real Postgres in Docker; LocalStack for DynamoDB; no Turso local emulator (use real Turso Cloud dev DB)
- `flag` (stdlib) — CLI — 5 flags; cobra is unjustified
- `encoding/json`, `text/tabwriter` (stdlib) — report output — no third-party JSON library needed

### Expected Features

**Must have (table stakes) — results are meaningless without these:**

- `ChatRepository` interface — single contract across all backends; define before writing any adapter code
- p50/p95/p99 latency per scenario per backend — mean hides tail behavior; this is the primary output
- AppendMessage scenario — dominant write operation; models every LLM turn
- LoadSlidingWindow scenario — dominant read operation; models every LLM context fetch (keyset pagination, not OFFSET)
- ListConversations scenario — secondary read; exercises GSI/index design
- ConcurrentWrites scenario (N=10, 50 goroutines) — reveals lock contention and DynamoDB throttling
- ColdStartLoad scenario — isolates first-query latency; critical for Turso over internet
- Warmup phase (excluded from measurement) — mandatory; eliminates cold JIT/connection effects
- Deterministic synthetic data (fixed seed) — same payloads across all backends; fair comparison
- Data size profiles (small ~100B, medium ~500B, large ~2KB) — exposes DynamoDB billing boundary behavior
- Iteration count and concurrency configurable via CLI flags
- Per-run JSON output with run metadata (timestamp, Go version, backend config, Git SHA)

**Should have (differentiators — make results convincing):**

- Cost projection model — DynamoDB RCU/WCU costs at projected scale; this is the TCO comparison the team actually needs
- Operational complexity scorecard — qualitative assessment of connection management, error types, local dev story
- Coefficient of variation (CV) reporting — CV > 5% flags unreliable scenarios
- Backend metadata in output (BackendMeta struct) — transport type, endpoint, topology; Turso must be annotated explicitly
- `--runs N` flag with min/median/max P99 across runs — single-run numbers are not statistically meaningful
- Concurrent writer degradation metric — % degradation vs single-goroutine baseline

**Defer (v2+):**

- Coefficient of variation histogram (add if spread looks suspicious)
- `--dry-run` mode (add if CI integration is desired post-POC)
- Chaos / failure injection testing
- Dashboard / real-time UI

### Architecture Approach

The harness is layered: CLI entry point → Benchmark Runner → Scenario Suite + Metrics Collector → ChatRepository interface → three adapters (Postgres, DynamoDB, Turso) → real backends. Data Generator and Report Writer are independent vertical slices that plug into the Runner. The strict rule is that the Runner imports the interface, never a concrete adapter. Setup, warm-up, and measurement are distinct phases; only the measurement phase touches the Metrics Collector. Cost projection is a post-processing step on ScenarioResults and never calls live backends.

**Major components:**

1. **ChatRepository interface** — single contract; defined in an internal package adapters cannot import
2. **Benchmark Runner** — orchestrates setup/warmup/measure/teardown lifecycle; manages worker pool for concurrent scenarios
3. **Scenario Suite** — five named scenarios as functions taking a ChatRepository and seeded PRNG; no setup logic inside
4. **Metrics Collector** — accumulates raw `[]time.Duration` samples per scenario; computes p50/p95/p99/min/max/stddev after run completes
5. **Data Generator** — deterministic SeedData struct at three size profiles; generated once, reused across all backends
6. **Postgres Adapter** — pgxpool, native pgx v5, prepared statement caching enabled (matches production behavior)
7. **DynamoDB Adapter** — aws-sdk-go-v2, expression builder, ConsistentRead=true for all reads, one item per message
8. **Turso Adapter** — libsql-client-go, remote-only, no embedded replica, persistent connection reuse
9. **Cost Projection Model** — pure function on ScenarioResults; inputs (scale, ops/day) from CLI flags
10. **Report Writer** — JSON to file + tabwriter to stdout; includes BackendMeta and run metadata

### Critical Pitfalls

1. **Coordinated omission in concurrent writes** — sequential issue-and-wait hides tail latency 5-10x; use a fixed-rate ticker-based dispatcher for ConcurrentWrites; measure from scheduled issue time, not actual send time
2. **DynamoDB on-demand cold-start throttling** — new on-demand tables start at 4,000 WCU / 12,000 RCU warm throughput; run a mandatory 2-3 minute warm-up pass at full load before timing begins; consider provisioned capacity for deterministic throughput
3. **DynamoDB Scan instead of Query** — "list conversations" implemented as a Scan makes DynamoDB appear 20-100x slower than a correctly designed Query; design the schema (composite PK: `PARTNER#<id>#USER#<id>`, SK: `CONV#<created_at>`) before writing any DynamoDB code
4. **DynamoDB eventually consistent reads as default** — SDK defaults to eventually consistent; always set `ConsistentRead: aws.Bool(true)` for read scenarios; cost model must use strongly consistent RCU pricing
5. **Turso transport conflation** — Turso communicates over HTTP/WebSocket to a server outside the AWS VPC; raw latency numbers will be 3-10x Postgres; BackendMeta must annotate transport type and endpoint location, or results will be misread as database engine performance

---

## Implications for Roadmap

Based on combined research, the build order has hard gates: the ChatRepository interface must be stable before any adapter is written; DynamoDB schema must be designed before DynamoDB implementation begins; all three adapters can be built in parallel once the interface is finalized.

### Phase 1: Foundation — Domain Types, Interface, and Data Generator

**Rationale:** Everything depends on the interface. Building it first prevents the common mistake of letting a specific database's idioms shape the contract. The in-memory adapter validates the interface before touching any real backend. Data Generator must exist before any scenario can run.

**Delivers:** Compilable harness skeleton; validated interface; deterministic seed data at three profiles

**Addresses:** ChatRepository interface (table stake #1), deterministic synthetic data (table stake), data size profiles

**Avoids:** Interface leaking database idioms (Architecture anti-pattern 1); designing interface around Postgres SQL ergonomics that makes DynamoDB adapter unnatural

**Research flag:** Standard patterns — well-documented repository/adapter pattern; no additional research needed

---

### Phase 2: Benchmark Runner and Metrics Collector

**Rationale:** The runner and metrics collector are backend-agnostic; building them before adapters means the timing harness is correct before any real I/O is introduced. Worker pool and HDR Histogram must be wired here.

**Delivers:** Working measurement harness with p50/p95/p99 output; concurrent worker pool; warm-up/measure/teardown lifecycle

**Uses:** hdrhistogram-go v1.2.0; errgroup or channel-based worker pool

**Implements:** Benchmark Runner, Metrics Collector, Scenario Suite (stubs)

**Avoids:** Coordinated omission (Pitfall 1) — fixed-rate dispatcher built in from the start; setup overhead in timings (Pitfall 7); using `testing.B` for multi-backend harness (Architecture anti-pattern 3)

**Research flag:** Standard patterns for the runner skeleton; ConcurrentWrites scenario timing (coordinated omission) deserves careful review against wrk2/HDRHistogram documentation

---

### Phase 3: Postgres Adapter and Baseline Measurements

**Rationale:** Postgres is the team's existing technology and the presumptive winner. Establishing the baseline first gives a reference point for comparing later backends. Any harness bugs show up here before touching unfamiliar backends.

**Delivers:** Working end-to-end benchmark against real Postgres; all five scenarios producing valid p50/p95/p99 numbers; testcontainers-go integration tests

**Uses:** pgx/v5 v5.9.1, pgxpool, testcontainers-go postgres module

**Avoids:** Postgres pool cold start (Pitfall 6) — MinConns + explicit warm-up ping; prepared statement caching documented (Pitfall 11 — accept as correct production behavior)

**Research flag:** Standard patterns — pgx and testcontainers are well-documented

---

### Phase 4: DynamoDB Schema Design and Adapter

**Rationale:** DynamoDB requires up-front schema design against access patterns before any code. This is a hard constraint — writing code before the schema is wrong is the most common DynamoDB mistake and would invalidate benchmark results. Schema review should happen at the start of this phase.

**Delivers:** DynamoDB adapter passing all five scenarios; schema documented in report; LocalStack integration tests

**Uses:** aws-sdk-go-v2 v1.57.1, attributevalue, expression builder; testcontainers-go localstack module

**Avoids:** Scan instead of Query (Pitfall 4) — schema must use composite PK for conversations; eventually consistent reads (Pitfall 5) — ConsistentRead=true set in constructor; DynamoDB item size limit for chat messages (Pitfall 12) — one item per message; on-demand cold-start throttling (Pitfall 2) — mandatory warm-up pass

**Research flag:** DynamoDB schema design for this specific access pattern (conversations scoped by partner_id+user_id, messages ordered by time) warrants phase-level research to validate the key design before implementation

---

### Phase 5: Turso Adapter

**Rationale:** Turso is the most uncertain backend (deprecated SDK, beta successor, HTTP transport outside VPC). Build it last so harness bugs and methodology issues are resolved before introducing the most variable element.

**Delivers:** Turso adapter passing all five scenarios; BackendMeta transport annotation in output; connection reuse validated

**Uses:** libsql-client-go (pure Go, remote-only); real Turso Cloud dev database

**Avoids:** Wrong SDK or embedded replica mode (Pitfall 9, 3) — libsql-client-go, no embedded replica; transport conflation (Pitfall 3) — BackendMeta in output with explicit annotation; connection-per-query (per-operation connections anti-pattern)

**Research flag:** Turso SDK choice and connection behavior (WebSocket vs HTTP per-request) should be validated against current Turso docs at phase start — the SDK landscape has changed recently

---

### Phase 6: Cost Projection Model and Final Report

**Rationale:** Cost model is a post-processing step on ScenarioResults; it cannot be built until all three adapters produce valid results. The operational complexity scorecard is also written here, after hands-on experience with all three adapters.

**Delivers:** Cost projection per backend at configurable scale; operational complexity scorecard; final JSON + tabwriter report with all backends; recommendation document

**Uses:** stdlib encoding/json, text/tabwriter; no new dependencies

**Avoids:** Wrong DynamoDB pricing units (Pitfall 8) — on-demand pricing, 1KB/4KB billing boundaries, storage costs included; hardcoded scale assumptions (Architecture anti-pattern 5) — cost inputs from CLI flags; single-run variance (Pitfall 10) — --runs N flag, min/median/max P99; confirmation bias on Turso (Pitfall 13) — neutral framing, explain architectural reasons for results

**Research flag:** Standard patterns — cost model math is straightforward once DynamoDB pricing boundaries are known (research covered this)

---

### Phase Ordering Rationale

- Interface (Phase 1) is the single hard gate — no adapter can be finalized until it is stable
- Runner and Metrics (Phase 2) before adapters — ensures timing correctness is validated in isolation before real I/O is introduced
- Postgres first (Phase 3) — team's home turf; harness bugs surface cheaply before unfamiliar backends
- DynamoDB schema design upfront (Phase 4) — cannot be skipped or deferred; a wrong schema invalidates the benchmark entirely
- Turso last (Phase 5) — most uncertain; all methodology issues should be resolved first
- Cost model and report last (Phase 6) — pure post-processing; requires all adapter data to be valid

---

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 4 (DynamoDB):** The composite key schema for `(partner_id, user_id)` scoped conversations with time-ordered messages needs explicit design validation. The access patterns (append, sliding window, list by user) must all be served by Query, not Scan. A schema mistake here invalidates the entire DynamoDB comparison.
- **Phase 5 (Turso):** The libsql-client-go vs go-libsql SDK situation should be re-evaluated at phase start. Upstream has deprecated libsql-client-go in favor of go-libsql (which requires CGO). Connection behavior (WebSocket persistent vs HTTP per-request) needs confirmation against current Turso docs.

Phases with standard patterns (skip research-phase):

- **Phase 1 (Foundation):** Repository/adapter pattern is well-documented in Go ecosystem
- **Phase 2 (Runner/Metrics):** Worker pool and HDR Histogram patterns are well-established
- **Phase 3 (Postgres):** pgx and testcontainers are mature with extensive documentation
- **Phase 6 (Report):** stdlib output; DynamoDB pricing math is covered in PITFALLS.md

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All libraries verified on pkg.go.dev with recent publish dates; one MEDIUM exception for Turso libsql-client-go (deprecated upstream but functional) |
| Features | HIGH | Well-established domain; scenario definitions are precise and grounded in production chat access patterns |
| Architecture | HIGH | Interface-adapter pattern is canonical in Go; build order derived from hard dependency graph, not opinion |
| Pitfalls | HIGH | Pitfalls sourced from primary vendor documentation (AWS, Turso), academic benchmarking papers, and Go ecosystem references; critical pitfalls verified from multiple independent sources |

**Overall confidence:** HIGH

### Gaps to Address

- **Turso SDK deprecation:** libsql-client-go is the pragmatic choice now but its long-term viability is unclear. If go-libsql CGO requirements become acceptable (e.g., the project gains a Linux CI environment with CGO support), migration should be evaluated. Flag for Phase 5 planning.
- **DynamoDB warm throughput:** The 2-3 minute warm-up recommendation is based on AWS documentation for on-demand tables. If provisioned capacity is used instead, this gap disappears and the benchmark becomes more deterministic. The capacity mode decision should be explicit in Phase 4 planning.
- **Coordinated omission for this scale:** The project context says "dozens of users." At that scale, sequential-issue benchmarking may be acceptable if labeled clearly. The decision of whether to implement a true fixed-rate dispatcher vs. sequential-issue with honest labeling should be made explicitly in Phase 2.
- **Turso region proximity:** Turso Cloud region assignment affects absolute latency numbers significantly. The benchmark output must record which Turso region was used and its geographic relationship to the AWS region. Not yet specified in the project.

---

## Sources

### Primary (HIGH confidence)

- [pgx GitHub CHANGELOG](https://github.com/jackc/pgx/blob/master/CHANGELOG.md) — v5.9.1 verification
- [aws-sdk-go-v2 DynamoDB pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb) — v1.57.1 verification
- [HdrHistogram Go pkg.go.dev](https://pkg.go.dev/github.com/HdrHistogram/hdrhistogram-go) — v1.2.0 verification
- [gofakeit v7 pkg.go.dev](https://pkg.go.dev/github.com/brianvoe/gofakeit/v7) — v7.14.1 verification
- [AWS DynamoDB: Understanding warm throughput scenarios](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/warm-throughput-scenarios.html) — cold start throttling
- [AWS DynamoDB: Read Consistency](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.ReadConsistency.html) — ConsistentRead flag behavior
- [AWS: Best Practices for Partition Key Design](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/bp-partition-key-design.html) — schema design
- [AWS: Scaling DynamoDB — partitions and split for heat](https://aws.amazon.com/blogs/database/part-1-scaling-dynamodb-how-partitions-hot-keys-and-split-for-heat-impact-performance/) — partition behavior
- [Turso Go Quickstart](https://docs.turso.tech/sdk/go/quickstart) — SDK choice and connection model
- [Fair Benchmarking Considered Difficult (VLDB)](https://mytherin.github.io/papers/2018-dbtest.pdf) — coordinated omission, methodology
- [ScyllaDB: On Coordinated Omission](https://www.scylladb.com/2021/04/22/on-coordinated-omission/) — coordinated omission mechanics

### Secondary (MEDIUM confidence)

- [libsql-client-go pkg.go.dev deprecation notice](https://pkg.go.dev/github.com/tursodatabase/libsql-client-go) — SDK status
- [tursodatabase/go-libsql GitHub](https://github.com/tursodatabase/go-libsql) — CGO requirements for successor SDK
- [Alex DeBrie: DynamoDB costs](https://www.alexdebrie.com/posts/dynamodb-costs/) — pricing model validation
- [Alex DeBrie: DynamoDB limits](https://www.alexdebrie.com/posts/dynamodb-limits/) — 400KB item limit, query page size
- [Three Dots Labs: Repository Pattern in Go](https://threedots.tech/post/repository-pattern-in-go/) — interface-adapter architecture
- [Aerospike: Best Practices for Database Benchmarking](https://aerospike.com/blog/best-practices-for-database-benchmarking/) — warm-up, fairness, reproducibility

### Tertiary (LOW confidence)

- [cybertec-postgresql.com: Network Latency](https://www.cybertec-postgresql.com/en/postgresql-network-latency-does-make-a-big-difference/) — VPC co-location importance (supports Turso penalty hypothesis)

---

*Research completed: 2026-03-31*
*Ready for roadmap: yes*
