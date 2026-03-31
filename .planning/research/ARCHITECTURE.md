# Architecture Patterns

**Domain:** Go database benchmark harness (Postgres vs DynamoDB vs Turso)
**Researched:** 2026-03-31

---

## Recommended Architecture

A layered benchmark harness with a strict separation between the domain contract (interface), backend implementations (adapters), benchmark execution engine, measurement collection, and output/reporting.

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Entry Point                       │
│              (flag parsing, backend selection)               │
└────────────────────────┬────────────────────────────────────┘
                         │ orchestrates
┌────────────────────────▼────────────────────────────────────┐
│                   Benchmark Runner                           │
│   (scenario loop, warm-up, worker pool, timing capture)      │
└────────┬────────────────┬───────────────────────────────────┘
         │ submits ops    │ collects samples
┌────────▼──────────┐   ┌▼──────────────────────────────────┐
│  Scenario Suite   │   │     Metrics Collector              │
│  (5 op types)     │   │  (per-scenario histogram → p50/    │
│                   │   │   p95/p99, min/max, stddev)        │
└────────┬──────────┘   └────────────────────────────────────┘
         │ calls
┌────────▼──────────────────────────────────────────────────┐
│              ChatRepository Interface                       │
│         (single contract, all backends satisfy it)          │
└────────┬──────────────┬──────────────────────────────────┘
         │              │                           │
┌────────▼──────┐  ┌────▼──────────┐  ┌────────────▼──────┐
│   Postgres    │  │   DynamoDB    │  │      Turso        │
│   Adapter     │  │   Adapter     │  │     Adapter        │
│  (pgx/sql)   │  │  (aws-sdk-v2) │  │  (libsql-go)      │
└───────────────┘  └───────────────┘  └────────────────────┘
         │              │                           │
     RDS Postgres    DynamoDB              Turso HTTP endpoint
     (AWS VPC)      (AWS native)          (outside VPC)

┌─────────────────────────────────────────────────────────────┐
│                  Data Generator                              │
│  (synthetic conversations, small/medium/large profiles)      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                  Report Writer                               │
│    (JSON + human-readable table, cost projection model)      │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Boundaries

| Component | Responsibility | Inputs | Outputs | Communicates With |
|-----------|---------------|--------|---------|-------------------|
| CLI Entry | Parse flags, select backends to run, wire dependencies | os.Args | config struct | Benchmark Runner |
| Benchmark Runner | Execute scenarios, manage warm-up, control concurrency, collect timing | config, ChatRepository | raw samples | Scenario Suite, Metrics Collector |
| Scenario Suite | Define the 5 benchmark scenarios as callable functions | ChatRepository, seed data | []Operation | Benchmark Runner |
| ChatRepository Interface | Contract all backends must satisfy | (none — interface definition) | (none — interface definition) | Adapters |
| Postgres Adapter | Translate ChatRepository calls to SQL via pgx | context, domain types | domain types, error | Postgres on RDS |
| DynamoDB Adapter | Translate ChatRepository calls to DynamoDB API calls | context, domain types | domain types, error | DynamoDB service |
| Turso Adapter | Translate ChatRepository calls to libsql over HTTP | context, domain types | domain types, error | Turso HTTP endpoint |
| Metrics Collector | Accumulate per-operation latency samples, compute percentiles | time.Duration samples | ScenarioResult (p50/p95/p99, min, max, count) | Benchmark Runner, Report Writer |
| Data Generator | Produce deterministic synthetic conversations at three sizes | profile enum (small/medium/large) | SeedData struct | Benchmark Runner (setup phase) |
| Report Writer | Format results as JSON and human-readable table, project costs | []BackendResult | stdout + optional file | CLI Entry |

---

## Data Flow

```
CLI flags
  └─► config{backends, scenarios, concurrency, iterations, warmup_iters}
        └─► Data Generator: generate deterministic seed data once
              └─► for each backend:
                    └─► Benchmark Runner:
                          1. SETUP: call repo.Setup(ctx, seedData)
                          2. WARM-UP: run N iterations, discard timing
                          3. MEASURE: run M iterations with timing
                             ├─► Scenario Suite: build operation
                             ├─► time op execution via ChatRepository
                             └─► Metrics Collector: record sample
                          4. TEARDOWN: call repo.Teardown(ctx)
                    └─► Metrics Collector: compute ScenarioResult
              └─► Report Writer:
                    ├─► latency table (backend × scenario)
                    ├─► cost projection (per-backend model)
                    └─► recommendation summary
```

Key flow rules:
- Seed data is generated **once** and reused across all backends (identical payloads, fair comparison).
- The warm-up phase runs before measurement to eliminate cold-start JIT/connection effects.
- The Metrics Collector holds raw samples (not rolled-up), computing percentiles at the end of a scenario run, not inline.
- Cost projection is a **separate post-processing step** on ScenarioResults — it does not touch the live backends.

---

## Component Details

### ChatRepository Interface

This is the single most important architectural decision. All backends must implement one interface; the benchmark runner never imports a concrete adapter directly.

```go
type ChatRepository interface {
    // Core operations (benchmark scenarios)
    AppendMessage(ctx context.Context, msg Message) error
    LoadWindow(ctx context.Context, convID string, limit int) ([]Message, error)
    ListConversations(ctx context.Context, partnerID, userID string) ([]Conversation, error)
    LoadConversation(ctx context.Context, convID string) ([]Message, error)

    // Lifecycle (called outside timing loop)
    Setup(ctx context.Context, seed SeedData) error
    Teardown(ctx context.Context) error
    Name() string
}
```

The interface must be defined in an internal package that no adapter imports. Adapters import the interface package; the runner imports both. This enforces one-way dependency.

### Scenario Suite

Five scenarios map to the five benchmark operations in PROJECT.md:

| Scenario | Operation | What It Measures |
|----------|-----------|-----------------|
| AppendMessage | Single message insert | Write latency, append pattern |
| LoadSlidingWindow | Latest N messages per conversation | Read latency, recency query |
| ListConversations | All conversations for a (partner, user) | List/scan latency |
| ColdStartLoad | Load full conversation after no-cache | Cold-read latency |
| ConcurrentWrites | Parallel AppendMessage from N goroutines | Contention, throughput |

Scenarios are functions that take a `ChatRepository` and a `*rand.Rand` seeded deterministically. They must not include setup logic.

### Benchmark Runner — Concurrency Model

For serial scenarios (1–4): run a single goroutine, M iterations, record each duration.

For the concurrent scenario (5): use a worker pool of N goroutines (configurable via CLI, default 10). Fan-out work over a job channel. Collect durations back via a results channel. Do not use `sync.WaitGroup` + shared slice (race condition risk) — use a channel of `time.Duration`.

```
jobCh ──► [worker 1] ──┐
          [worker 2] ──┤──► resultCh ──► Metrics Collector
          [worker N] ──┘
```

### Metrics Collector

Do not use Go's built-in `testing.B` — it is unsuitable for custom harnesses with multiple backends. Instead, collect raw `[]time.Duration` samples and compute percentiles at the end.

Implementation approach:
- Collect all samples into a `[]time.Duration` slice per scenario.
- Sort the slice (O(n log n)) after all iterations complete.
- Compute p50, p95, p99 as `sorted[n*0.50]`, `sorted[n*0.95]`, `sorted[n*0.99]`.
- Also record min, max, mean, stddev, and iteration count.

Consider `github.com/jamiealquiza/tachymeter` as a drop-in if the math-heavy percentile code is not worth owning. It provides p50/p95/p99/p999 plus histogram output from a simple API.

### Data Generator

Produces three synthetic data profiles. Profiles must be deterministic (seeded PRNG) so every run of the harness against any backend uses identical data.

| Profile | Conversations | Messages per Conv | Message Body Size |
|---------|---------------|-------------------|-------------------|
| Small | 10 | 5 | ~100 bytes |
| Medium | 100 | 50 | ~500 bytes |
| Large | 500 | 200 | ~2 KB |

Each profile produces a `SeedData` struct containing pre-generated `[]Conversation` and `[]Message` with stable IDs. Adapters consume this struct during `Setup()`.

### Cost Projection Model

A separate non-benchmarked module. Input: `ScenarioResult` (operation counts, latency). Output: estimated monthly cost at configured scale.

Per-backend cost formulas:
- **Postgres/RDS**: Fixed instance cost + storage (not per-query). Express as hourly rate at chosen instance class.
- **DynamoDB**: WCU/RCU consumption per operation type × projected operation volume. Use on-demand pricing model.
- **Turso**: Per-database monthly cost + per-row-read pricing (Turso's billing model).

Cost model assumptions (scale, ops/day) must be configurable via CLI flags or a config file, not hardcoded.

---

## Patterns to Follow

### Pattern 1: Interface-First, Adapter-Second

Define `ChatRepository` in an internal package first. Write a mock/in-memory implementation to validate the interface before touching any real database. Build adapters against the finalized interface.

This prevents the common mistake of designing the interface around what a specific database makes easy (usually Postgres SQL, which would make DynamoDB's adapter unnatural).

### Pattern 2: Explicit Setup/Teardown Outside the Timing Loop

The timing loop must only measure the operation itself. Connection establishment, schema migration, index creation, and seeding happen in `Setup()`, which is called before the warmup phase and not timed.

```go
// CORRECT
repo.Setup(ctx, seed)      // not timed
warmup(repo)               // not recorded
measure(repo, collector)   // timed
repo.Teardown(ctx)         // not timed
```

### Pattern 3: Single Seed Data, All Backends

Generate `SeedData` once before the backend loop. Pass the same struct to every adapter's `Setup()`. This ensures latency differences reflect the database, not data variation.

### Pattern 4: Warm-Up Before Measurement

Run at least 100 warm-up iterations per scenario before recording any samples. This eliminates:
- TCP connection setup
- TLS handshake overhead
- Query plan caching (Postgres)
- SDK client initialization (DynamoDB, Turso)

The warm-up iteration count should be configurable (default: 100).

### Pattern 5: Separate Report Building from Data Collection

The Metrics Collector owns raw samples. The Report Writer owns formatting. These must not be mixed. The Runner calls `collector.Compute()` once after all iterations; the runner never formats output itself.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Interface Leaking Database Idioms

**What:** Putting SQL-specific types (`*sql.Tx`), DynamoDB-specific types (`dynamodb.AttributeValue`), or Turso-specific types in the `ChatRepository` interface.
**Why bad:** Forces all three adapters to expose an alien type. Makes the interface fragile. Testing becomes impossible without real databases.
**Instead:** Interface uses only domain types (`Message`, `Conversation`, `ConversationID`). Adapters handle all translation internally.

### Anti-Pattern 2: Timing Connection Lifecycle

**What:** Starting the timer before calling `repo.Connect()` or including the first request's TCP handshake in samples.
**Why bad:** Turso is over the internet — its first-connection cost dwarfs per-operation cost and distorts the comparison.
**Instead:** Connections are established in `Setup()`. Warm-up absorbs any remaining SDK initialization before the measurement window.

### Anti-Pattern 3: Using `testing.B` for Multi-Backend Harness

**What:** Embedding benchmark logic inside Go's `testing.B` infrastructure.
**Why bad:** `testing.B` is designed for single-function microbenchmarks, not multi-backend scenario orchestration. The runner logic, warm-up control, and output format become constrained by the test framework.
**Instead:** Build a standalone CLI with custom timing. Use `go test -bench` only for microbenchmarks within individual adapter packages (e.g., verifying query plan reuse), not for the comparative harness.

### Anti-Pattern 4: Per-Operation Database Connections

**What:** Opening a new connection per benchmarked operation (like a Lambda-style handler).
**Why bad:** The project explicitly targets a long-running service model. Benchmarking connection-per-op would test connection pooling overhead, not storage operation latency.
**Instead:** One connection (or connection pool, for Postgres) established in `Setup()`, reused across all operations in the scenario. For DynamoDB and Turso, reuse the SDK client across iterations.

### Anti-Pattern 5: Hardcoded Scale Assumptions in Cost Model

**What:** Baking in "1000 requests/day" as a constant in the cost projection code.
**Why bad:** Makes the cost model useless when scale assumptions change during review.
**Instead:** Cost projection inputs (daily operations, data retention days, message size) come from CLI flags or a projection config file. The cost model is a pure function of those inputs and the measured ScenarioResults.

---

## Build Order (Phase Dependencies)

The components have a strict dependency graph. Build in this order:

```
1. Domain types          (Message, Conversation, ConversationID, SeedData)
   └─ no dependencies

2. ChatRepository interface
   └─ depends on: domain types

3. Data Generator
   └─ depends on: domain types

4. In-Memory Adapter (optional, for interface validation)
   └─ depends on: interface, domain types

5. Metrics Collector
   └─ depends on: (none — pure math on []time.Duration)

6. Benchmark Runner (skeleton, no real backends yet)
   └─ depends on: interface, metrics collector, data generator

7. Postgres Adapter
   └─ depends on: interface, domain types
   └─ blocks: nothing else, parallelizable with 8 and 9

8. DynamoDB Adapter
   └─ depends on: interface, domain types
   └─ parallelizable with 7 and 9

9. Turso Adapter
   └─ depends on: interface, domain types
   └─ parallelizable with 7 and 8

10. Cost Projection Model
    └─ depends on: ScenarioResult types from metrics collector

11. Report Writer
    └─ depends on: metrics collector output, cost projection model

12. CLI Entry Point
    └─ depends on: all of the above, wires them together
```

Steps 7, 8, and 9 are parallelizable in terms of implementation order — they share no code beyond the interface. The interface (step 2) is the single hard gate. Nothing else can be finalized until the interface is stable.

---

## Scalability Considerations

This is a benchmark harness, not a production service. "Scale" here means benchmark scale (number of iterations, concurrency level, data profile size) rather than user traffic.

| Concern | Approach |
|---------|----------|
| Memory for raw samples | 10,000 iterations × 8 bytes (time.Duration) = 80 KB per scenario. Negligible. |
| Concurrency in runner | Worker pool with configurable N (default: 10). Not unbounded goroutines. |
| DynamoDB throttling | Use on-demand capacity mode during benchmarks. Add retry backoff in adapter — but record the retry-inclusive latency, not pre-retry. |
| Turso network variability | Run 3 full passes and report per-pass results, not a single merged set, so network jitter is visible. |
| Postgres connection pool | pgxpool with pool_max_conns=20 (conservative for an RDS instance); configure via env var. |

---

## Sources

- [go_db_bench — jackc's PostgreSQL driver benchmark](https://github.com/jackc/go_db_bench): Real-world example of multi-driver comparison structure
- [Repository Pattern in Go — Three Dots Labs](https://threedots.tech/post/repository-pattern-in-go/): Interface-per-backend adapter pattern with multiple implementations
- [Adapter Pattern in Go — Bitfield Consulting](https://bitfieldconsulting.com/posts/adapter): Grouping database-specific code behind an interface boundary
- [Tachymeter — jamiealquiza/tachymeter](https://github.com/jamiealquiza/tachymeter): Go library for p50/p95/p99 percentile computation from raw samples
- [Database Benchmarking Best Practices — Aerospike](https://aerospike.com/blog/best-practices-for-database-benchmarking/): Warm-up, fairness, reproducibility methodology
- [Worker Pool Pattern — goperf.dev](https://goperf.dev/01-common-patterns/worker-pool/): Bounded goroutine pool for concurrent benchmark scenarios
- [Postgres vs DynamoDB — testdriven.io](https://testdriven.io/blog/postgres-vs-dynamodb/): Access pattern differences that inform adapter design
- [gobenchdata](https://pkg.go.dev/go.bobheadxi.dev/gobenchdata): Structured JSON output from Go benchmark runs (reference for report format)
