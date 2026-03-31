# Feature Landscape: Database Benchmark Harness

**Domain:** Go CLI benchmark harness — Postgres vs DynamoDB vs Turso for chat conversation storage
**Researched:** 2026-03-31
**Overall confidence:** HIGH (well-established domain with strong reference material)

---

## Table Stakes

Features the benchmark must have or the results are meaningless — a benchmark missing any of these cannot be trusted.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Common `ChatRepository` interface | Ensures same code paths hit all backends; without this you're testing implementations not databases | Low | Define interface first; all scenarios call through it |
| Latency percentiles: p50, p95, p99 | Mean hides tail behavior; p99 is where users feel pain; standard in any serious benchmark | Medium | Use HDR histogram or `hdrhistogram-go`; naive sort is fine at <100k samples |
| Append message scenario | Core write path: insert one message into an existing conversation | Low | Models the dominant write operation for chat |
| Load sliding window scenario | Core read path: fetch last N messages for LLM context | Low | Models every LLM turn; keyset pagination, not OFFSET |
| List conversations scenario | Secondary read: list all conversations for a user | Low | Models the inbox/history view; exercises index/GSI differently |
| Concurrent write scenario | Tests backend behavior under parallel load | Medium | Use `errgroup` or goroutine pool; reveal lock contention or throttling |
| Cold start load scenario | Isolates first-query latency vs warm-cache latency | Medium | Clear connection pool and sleep before measuring; critical for Turso over internet |
| Iteration count control | Results must be reproducible; N must be configurable | Low | Flag: `--iterations N`; default to a value that runs >5s per scenario |
| Warmup phase (excluded from measurement) | First iterations hit cold caches; including them poisons results | Low | Run N warmup iterations before starting the timer |
| Per-backend per-scenario output | Results table showing each backend × each scenario | Low | Output should be structured (JSON) and human-readable (table) |
| Deterministic synthetic data | Same data must be seeded identically across all backends | Low | Fixed random seed; generate before benchmark loop, not during |
| Data size profiles (small/medium/large) | Different message sizes expose different behavior (DynamoDB item size limits, etc.) | Low | Small: 100 chars, Medium: 500 chars, Large: 2000 chars per message |
| Environment isolation | Each run starts from a known schema/data state | Medium | Truncate or re-seed between runs; DynamoDB needs explicit cleanup |

---

## Differentiators

Features that make the benchmark credible and the recommendation convincing to stakeholders.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Cost projection model | The team needs TCO, not just latency; DynamoDB RCU/WCU costs scale with access patterns | High | Model: $0.25/million reads, $1.25/million writes for DynamoDB on-demand; RDS instance cost amortized over estimated queries; Turso per-row-read pricing |
| Operational complexity scorecard | Code complexity, gotchas, SDK ergonomics matter as much as raw latency | Medium | Qualitative rubric: connection management, error types, schema migration, local dev story |
| Coefficient of variation reporting | Low CV (target <5%) proves the numbers are stable, not noise | Medium | CV = stddev/mean; flag scenarios with high CV as unreliable |
| Latency histogram output | Shows distribution shape, not just percentiles; bimodal distributions indicate caching effects | Medium | ASCII histogram or JSON bucket data; sysbench-style |
| Backend-specific configuration notes | Documents what knobs were set (RDS instance class, DynamoDB capacity mode, Turso region) | Low | Ensures results are reproducible in the reader's environment |
| Scenario timing breakdown | Wall time per scenario, not just per-op — shows relative scenario weight | Low | Print scenario summary: total wall time, ops/sec, p50/p95/p99 |
| Concurrent writer contention metric | Not just "did it work" but how much degradation under load vs single-writer baseline | Medium | Compare single-goroutine baseline vs N-goroutine concurrency run; report % degradation |
| `--dry-run` mode | Verifies connectivity and schema setup without running benchmarks; useful in CI | Low | Connects, inserts one row, reads it back, deletes it, exits 0 |
| JSON output mode | Machine-readable results enable diff between runs, CI assertions, and chart generation | Low | `--output json` flag; structured result envelope with run metadata |
| Run metadata in output | Timestamps, Git SHA, backend config, Go version baked into output | Low | Ensures results are traceable; critical when sharing with team |

---

## Anti-Features

Things to explicitly NOT build. Each represents a trap that would bloat scope or undermine the benchmark's purpose.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| HTTP/gRPC transport layer | Adds network noise that obscures DB latency; not part of what's being measured | Call `ChatRepository` methods directly from benchmark harness |
| Authentication / JWT middleware | Same reason — irrelevant to storage evaluation | Hard-code a test `partner_id` and `user_id` in synthetic data |
| Production-safe connection pool tuning | This is a benchmark, not an ops runbook; over-tuned pools skew results | Use idiomatic defaults; document what was used |
| Real conversation data / PII | Compliance risk, unnecessary complexity, harder to share results | Generate realistic-looking synthetic data with fixed seed |
| Streaming / SSE / WebSocket layer | LLM streaming is an application concern, not a storage concern | Measure storage read latency only; streaming is additive on top |
| Redis cache layer evaluation | Wrong scope — cache is a complement to storage, not a candidate replacement | Note in output that caching was not evaluated; it could reduce all three backends equally |
| Multi-region / edge distribution testing | Single-region AWS is the target; edge testing inflates Turso's normal-case latency even further | Test from single region; note Turso's edge value prop is outside this scope |
| Automated schema migration tooling | Not needed for a benchmark harness; schema is bootstrapped once per run | `CREATE TABLE IF NOT EXISTS` in setup; Turso/libSQL handles DDL same as SQLite |
| Dashboard / real-time monitoring UI | Over-engineering for a POC; results are a one-time report | Output JSON to file; read with `jq` or paste into a spreadsheet |
| Chaos engineering / failure injection | Valuable for production evaluation, out of scope for storage selection POC | Note in report that failure modes were not tested; flag as future work |
| ORM benchmarking | The benchmark should test the storage engine, not ORM overhead | Use raw SQL for Postgres (or minimal Ent usage matching prod), raw DynamoDB SDK, libsql directly |
| Automated cloud provisioning (Terraform) | Heavy infra tooling for a POC; manual setup with documented steps is sufficient | Document required AWS resources in README; assume they pre-exist |

---

## Feature Dependencies

```
Synthetic data generator
  --> All scenarios (data must exist before any scenario runs)

Common ChatRepository interface
  --> All three backend implementations
  --> All benchmark scenarios

Append message scenario
  --> List conversations scenario (needs conversations to exist)
  --> Load sliding window scenario (needs messages to exist)

Cold start load scenario
  --> Append message scenario (needs data seeded first)
  --> Warmup phase (must be explicitly skipped or cleared)

Concurrent write scenario
  --> Append message scenario (parallel version of the same operation)

Latency percentile measurement (p50/p95/p99)
  --> All scenarios (wraps each scenario's timing loop)

Cost projection model
  --> Latency measurement (needs ops/sec to calculate RCU/WCU consumption)
  --> Scenario results (needs reads vs writes breakdown per scenario)

JSON output mode
  --> All scenarios (aggregate after all scenarios complete)
  --> Run metadata (included in output envelope)
```

---

## MVP Recommendation

Build in this order to get valid results as quickly as possible:

1. `ChatRepository` interface + Postgres implementation — baseline established, nothing else matters until this works
2. Synthetic data generator with seeded RNG — all scenarios depend on this
3. Latency measurement harness (p50/p95/p99) — core output; instrument this before adding scenarios
4. Append message + load sliding window scenarios — covers the two highest-frequency operations in production
5. DynamoDB implementation — adds first alternative; Postgres vs DynamoDB is the primary comparison
6. List conversations + cold start + concurrent write scenarios — completes the scenario matrix
7. Turso implementation — third candidate; expected to underperform, confirm and quantify
8. Cost projection model — converts raw latency into the TCO comparison the team actually needs
9. Operational complexity scorecard — qualitative layer; write after all three implementations are complete
10. JSON output + run metadata — makes results shareable without manual transcription

**Defer:**
- Coefficient of variation and histogram output (add if p50/p95/p99 spread looks suspicious)
- `--dry-run` mode (add if integration with CI is desired after POC is accepted)

---

## Domain-Specific Metrics for Chat Storage

These metrics matter specifically for this use case and should inform scenario design:

| Metric | Why It Matters | How to Measure |
|--------|---------------|----------------|
| Append latency (single write) | Every LLM turn produces one or two writes (user + assistant message) | Single-row insert time per backend |
| Sliding window read latency | Every LLM turn reads the last N messages before calling the model | `SELECT ... ORDER BY created_at DESC LIMIT N` with keyset pagination |
| Conversation list latency | Users navigate between conversations; slow list = bad UX | `SELECT ... WHERE user_id = ? ORDER BY updated_at DESC` |
| Cold start overhead | First query after connection establishment; relevant for Turso (internet RTT) and new DB connections | Measure first query after fresh connection vs warmed connection |
| Write throughput under concurrency | Dozens-to-hundreds of concurrent users sending messages simultaneously | N goroutines each appending messages; measure degradation vs single-writer |
| DynamoDB RCU/WCU consumption per scenario | Item size and access pattern determine cost; 4KB read unit and 1KB write unit boundaries matter | Count RCUs/WCUs per operation; multiply by projected message volume and size |
| Postgres connection overhead | Long-running service eliminates cold pool cost, but connection limits still apply at scale | Measure with realistic pool size; document pool configuration used |

---

## Scenario Definitions (Precise)

| Scenario | Setup | Operation | Measurement |
|----------|-------|-----------|-------------|
| `AppendMessage` | Existing conversation with M messages | Insert one message row | Latency of single insert, p50/p95/p99 |
| `LoadSlidingWindow` | Conversation with 200+ messages | Fetch last N=20 messages ordered by time | Latency of keyset-paginated read |
| `ListConversations` | User with K=10 conversations | Fetch all conversations for user_id ordered by updated_at | Latency of user-scoped list query |
| `ColdStartLoad` | Connection pool cleared, no warmup | First sliding window read after fresh connection | First-query latency (not averaged into warmup) |
| `ConcurrentWrites` | N goroutines (N=10, 50) each appending | Parallel insert storm | Throughput (ops/sec) and p99 tail latency |

---

## Sources

- [Best practices for database benchmarking — Aerospike](https://aerospike.com/blog/best-practices-for-database-benchmarking/)
- [10 principles of proper database benchmarking — db-benchmarks.com](https://db-benchmarks.com/principles-of-proper-benchmarking/)
- [Benchmarking in Go — Better Stack](https://betterstack.com/community/guides/scaling-go/golang-benchmarking/)
- [Go testing.B.ReportMetric — official pkg.go.dev](https://pkg.go.dev/testing)
- [More predictable benchmarking with testing.B.Loop — Go Blog](https://go.dev/blog/testing-b-loop)
- [DynamoDB vs PostgreSQL — Better Stack](https://betterstack.com/community/guides/databases/postgresql-vs-dynamodb/)
- [DynamoDB Pricing — awsfundamentals](https://awsfundamentals.com/blog/amazon-dynamodb-pricing-explained)
- [How to think about DynamoDB costs — Alex DeBrie](https://www.alexdebrie.com/posts/dynamodb-costs/)
- [Turso Go quickstart — official docs](https://docs.turso.tech/sdk/go/quickstart)
- [Turso libsql-client-go — GitHub](https://github.com/tursodatabase/libsql-client-go)
- [GSI is Not a Silver Bullet — DynamoDB access patterns](https://medium.com/@akshaynawale/gsi-is-not-a-silver-bullet-why-adding-a-new-access-pattern-to-your-dynamodb-table-can-hurt-b68320b8ffbb)
