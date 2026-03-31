# Domain Pitfalls

**Domain:** Database benchmark harness — Postgres vs DynamoDB vs Turso for LLM chat storage
**Researched:** 2026-03-31
**Confidence:** HIGH (most pitfalls verified from multiple official/primary sources)

---

## Critical Pitfalls

Mistakes that produce meaningless numbers or require a rewrite to fix.

---

### Pitfall 1: Coordinated Omission — Measuring "Easy" Requests Only

**What goes wrong:** When the benchmark sends the next request only after the previous one completes (sequential issue-and-wait), slow responses self-select out of the timing window. A 2-second stall causes the harness to not issue the next request until the stall resolves — so only fast requests accumulate in the sample. P99 looks great; the user experience is terrible.

**Why it happens:** Most naive `time.Now()` / `time.Since()` loops in Go measure individual request latency but ignore queueing delay. This is the "coordinated omission" problem described by Gil Tene (HDRHistogram/wrk2 author) — the load generator and the system under test silently coordinate to hide tail latency.

**Consequences:** P95/P99 values for DynamoDB's on-demand capacity ramp-up or Turso's cross-internet calls can look 5-10x better than production reality. The comparison report will be wrong.

**Prevention:**
- For concurrent-write scenarios, use a fixed-rate goroutine dispatcher (ticker-based), not a sequential loop
- Measure start-to-complete latency from the scheduled issue time, not the actual send time
- Use HDRHistogram (Go port: `github.com/HdrHistogram/hdrhistogram-go`) for accurate percentile capture
- Alternatively: for this benchmark's scale (dozens of users), document that measurements are sequential-issue and label results accordingly, so the limitation is explicit rather than hidden

**Detection:** P99 latency that is only 2-3x P50 across all backends is a warning sign — real tail latency under any contention is usually higher. If P99 for Turso remote looks "reasonable," something is wrong.

**Phase:** Address during scenario measurement implementation (concurrent-writes scenario in particular).

---

### Pitfall 2: DynamoDB On-Demand Cold Start Throttling Poisons Results

**What goes wrong:** A freshly created DynamoDB on-demand table starts with a warm throughput of 4,000 WCU / 12,000 RCU. If the benchmark fires requests before the table has warmed up — or if sequential data loading triggers "split for heat" partition splitting mid-load — the first N measurements are throttled and artificially inflate latency.

**Why it happens:** DynamoDB on-demand capacity is not unlimited from the start. It takes several minutes for the service to detect heat and split partitions. There is no explicit notification that a split occurred — performance just improves suddenly. This means the first portion of a benchmark run captures throttling behavior, not steady-state behavior.

**Consequences:** DynamoDB write latency looks terrible in early rounds, then normalizes. A short benchmark (< 5 minutes) will blend throttled and steady-state measurements. A long benchmark will show falsely improving latency over time. Either way the numbers are wrong.

**Prevention:**
- Run a mandatory warm-up pass of at least 2-3 minutes before any timing begins — send representative read/write traffic at full load
- Use provisioned capacity for the benchmark (not on-demand) if you want consistent, deterministic throughput
- Load seed data sequentially (not randomly) to avoid premature partition splits that make the query phase artificially faster than a cold production table
- Document which capacity mode was used in the report — it materially changes cost projections

**Detection:** Latency that measurably improves across the first 60-120 seconds of benchmark execution. Plot latency over time, not just aggregate histograms.

**Phase:** Infrastructure setup phase (table provisioning decisions) and data seeding phase.

---

### Pitfall 3: Turso Uses HTTP/WebSocket — Not Comparable Wire Protocol to Postgres pgx

**What goes wrong:** Postgres via pgx uses the PostgreSQL binary wire protocol over a persistent TCP connection in the same VPC. Turso's Go SDK (`go-libsql`) communicates over HTTP (the "hrana" protocol) or WebSocket to Turso's remote servers outside AWS. These are not equivalent transport layers. Measuring them identically as "database latency" conflates network topology differences with database performance differences.

**Why it happens:** The benchmark project correctly notes "Turso is expected to show worse latency (edge-SQLite called from single-region AWS over internet)" — but without labeling this clearly in the harness code and output, readers will compare raw numbers without understanding why.

**Consequences:** Turso will appear 3-10x slower than Postgres for single operations. This is accurate but misleading if the report does not clearly distinguish "transport-layer penalty" from "database engine performance." The team may incorrectly conclude Turso is a poor database rather than a poor fit for single-region AWS.

**Prevention:**
- Add a `BackendMeta` struct to benchmark output that records transport type, endpoint location, and network topology for each backend
- Report Turso results with an explicit annotation: "remote HTTP/WebSocket to Turso Cloud from us-east-1; internet egress penalty included"
- Do NOT use Turso embedded replicas for this benchmark — embedded replicas give microsecond reads and would make the comparison nonsensical for the team's actual deployment scenario (the service is not embedded)
- Separately report connection establishment cost (per-backend) vs per-query cost

**Detection:** If Turso P50 reads are < 5ms, you accidentally enabled an embedded replica or local mode.

**Phase:** Interface design phase (BackendMeta must be built in from the start; retrofitting it is hard).

---

### Pitfall 4: DynamoDB Access Pattern Mismatch — Scan Instead of Query

**What goes wrong:** DynamoDB is only fast when you Query using the partition key. If the data model uses `user_id` as partition key but the "list conversations" scenario does a Scan with a FilterExpression instead of a Query, DynamoDB reads every partition in the table and discards non-matching items. The benchmark will make DynamoDB look catastrophically slow for a pattern it handles well when designed correctly.

**Why it happens:** DynamoDB's key-value model requires up-front access pattern design. The natural reflex (especially from SQL backgrounds) is to filter by attribute, which becomes a Scan. For the chat use case: conversations are scoped by `(partner_id, user_id)` — the correct model uses a composite partition key, not a filter.

**Consequences:** "List conversations for user" shows DynamoDB at 100-500ms; the actual Query-based design would be 1-5ms. The report would incorrectly condemn DynamoDB for a self-inflicted schema mistake.

**Prevention:**
- Design the DynamoDB schema before writing any benchmark code. The recommended pattern for chat: PK = `PARTNER#<partner_id>#USER#<user_id>`, SK = `CONV#<created_at ISO8601>` (for conversations), and a separate item type for messages
- Never use Scan in any benchmark scenario — it is not a valid access pattern for this use case
- Have someone with DynamoDB experience review the schema before the benchmark runs
- Document the schema design rationale in the comparison report

**Detection:** "ListConversations" DynamoDB latency exceeds 50ms on small datasets. Query on the primary key should be 1-5ms for < 100 items.

**Phase:** DynamoDB schema design (must happen before any DynamoDB implementation work).

---

### Pitfall 5: DynamoDB Strongly Consistent vs Eventually Consistent Read Confusion

**What goes wrong:** DynamoDB defaults to eventually consistent reads, which cost 0.5 RCU per 4KB (versus 1 RCU for strongly consistent). A benchmark that uses eventually consistent reads for DynamoDB while Postgres provides serializable reads by default is not a fair comparison for a chat application that needs to see its own writes.

**Why it happens:** The Go AWS SDK's `QueryInput` does not require setting `ConsistentRead` — it defaults to false (eventually consistent). Developers assume "read" means "read the latest value." For message history reads where the user just appended a message and immediately loads the sliding window, eventually consistent reads can return stale data.

**Consequences:** DynamoDB latency looks artificially low (cheaper consistency); cost projections are understated; the comparison is not apples-to-apples; production use of eventually consistent reads causes user-visible bugs (user appends message, reloads, message is missing).

**Prevention:**
- For "load sliding window" and "load conversation" scenarios, always set `ConsistentRead: aws.Bool(true)` in QueryInput
- Document the consistency level used in benchmark output
- For the cost projection model, use the strongly consistent RCU pricing, not the default

**Detection:** Read latency for DynamoDB is suspiciously lower than expected, or load-after-write tests occasionally return incomplete result sets.

**Phase:** DynamoDB implementation (set as a constant in the client constructor, not per-call).

---

## Moderate Pitfalls

Mistakes that skew numbers but don't fully invalidate them.

---

### Pitfall 6: Not Warming the Postgres Connection Pool Before Measuring

**What goes wrong:** pgxpool establishes connections lazily. The first N requests after creating the pool will incur TCP + TLS + Postgres authentication overhead (20-100ms each on RDS). If the benchmark starts measuring immediately after creating the pool, cold-connection latency contaminates the steady-state numbers.

**Prevention:**
- Call `pool.Ping(ctx)` or run a dummy query loop to establish `MaxConns` connections before the timer starts
- Use `b.ResetTimer()` (Go benchmark) or an explicit warm-up window after pool creation
- Set `pgxpool.Config.MinConns` to a non-zero value to pre-establish connections at pool creation

**Detection:** First 5-10 iterations show 10-50x higher latency than subsequent iterations.

**Phase:** Benchmark harness setup (pool initialization helper).

---

### Pitfall 7: Including Go benchmark.testing Setup Time in Measurements

**What goes wrong:** Go's `testing.B` timing begins at function start. Any data generation, client initialization, or context creation inside the measurement loop inflates results. This is especially harmful for DynamoDB where marshaling structs with `attributevalue.MarshalMap` has measurable overhead.

**Prevention:**
- Call `b.ResetTimer()` after all per-benchmark setup
- Use `b.StopTimer()` / `b.StartTimer()` to bracket pure I/O calls if iteration-level setup is unavoidable
- Pre-generate all synthetic messages/conversations outside the timing loop; pass them by reference
- For this project's use of `time.Now()` / `time.Since()` custom harness (not testing.B): establish a clear contract that only the database call itself is timed, not marshaling or result processing

**Detection:** "Append message" latency is similar across all backends even though DynamoDB marshaling overhead should be a real factor — means you're measuring total operation time fairly and that's probably fine, but make sure it is intentional.

**Phase:** Benchmark harness design (timing contract must be documented in the interface).

---

### Pitfall 8: DynamoDB Cost Model Uses Wrong Unit Prices

**What goes wrong:** On-demand vs provisioned capacity have meaningfully different per-request costs. On-demand charges per request unit (RRU/WRU — not per second), while provisioned charges per provisioned capacity unit per hour regardless of utilization. Mixing pricing models in cost projections produces nonsense numbers. Additionally, DynamoDB item sizes round up to the nearest 1KB for writes and 4KB for reads — a 200-byte message costs 1 full WCU, not 0.2 WCU.

**Prevention:**
- For the "dozens to hundreds of users" scale, on-demand pricing is almost certainly cheaper and more appropriate — use it for both benchmarking and cost modeling
- Always round item sizes up to DynamoDB billing boundaries in the cost model (1KB for writes, 4KB for reads)
- Include storage costs ($0.25/GB/month) in projections — chat message history accumulates
- Annotate cost model with the capacity mode and region used

**Detection:** Projected DynamoDB costs that look lower than Postgres RDS at small scale are suspicious — DynamoDB minimum cost exists even at zero load.

**Phase:** Cost projection model implementation.

---

### Pitfall 9: Turso go-libsql SDK Is in Beta — Behavioral Surprises

**What goes wrong:** The `go-libsql` package is marked beta. The older `libsql-client-go` package uses HTTP per-request (stateless), while `go-libsql` uses a native libsql connection. Using the wrong SDK or mixing them produces inconsistent results and the wrong latency profile.

**Prevention:**
- Use `go-libsql` (the newer package: `github.com/tursodatabase/go-libsql`) for remote connections, not `libsql-client-go`
- Do NOT use embedded replica mode for this benchmark (see Pitfall 3)
- Pin the SDK version and document it — beta packages change behavior between minor versions
- Test that connection reuse actually works: Turso remote connections should be persistent WebSocket, not per-query HTTP if using the native driver

**Detection:** Turso latency varies wildly between runs (50ms to 500ms) suggesting connection establishment per query rather than reuse.

**Phase:** Turso client implementation.

---

### Pitfall 10: Single-Run Numbers Without Statistical Validation

**What goes wrong:** A single benchmark run producing a single P99 number is not statistically meaningful. Cloud environments have "noisy neighbor" interference, JIT-style query plan caching, and transient network jitter. A 30-second run showing DynamoDB at 8ms P99 could be 15ms P99 on the next run.

**Prevention:**
- Run each scenario at least 3 times; report min/median/max P99 across runs, not just one number
- Use `benchstat` (Go's official tool) if using `testing.B`, or report standard deviation in the custom harness
- Run during low-traffic times if using shared cloud resources
- Note: for this project's scale ("dozens of users"), variance will be high relative to signal — be explicit about sample size limitations in the report

**Detection:** Re-running the benchmark produces results that differ by > 20% for the same backend and scenario.

**Phase:** Harness execution and reporting (add a `--runs N` flag and aggregate stats from the start).

---

## Minor Pitfalls

---

### Pitfall 11: Postgres Prepared Statements Enabled by Default in pgx

**What goes wrong:** pgx automatically prepares and caches statements by default, giving Postgres an unfair advantage over a fresh connection baseline. In production with pgxpool in a long-running service, this is realistic behavior — but it means "first query" and "subsequent query" latencies for Postgres differ significantly.

**Prevention:** Accept this as the correct production behavior for a long-running service. Document that the benchmark uses pgx with statement caching enabled (the production behavior). Do not disable it to "level the playing field" — that would make the benchmark less realistic.

**Phase:** Postgres implementation (no code change needed; just document).

---

### Pitfall 12: DynamoDB Item Size for Chat Messages

**What goes wrong:** DynamoDB has a 400KB hard limit per item. A naïve schema that stores all messages in a conversation as a single item (common in tutorial examples using document-style storage) will fail when a conversation exceeds ~200-300 messages depending on message size.

**Prevention:**
- Store each message as a separate DynamoDB item (not embedded in a conversation item)
- The schema must support `Query` to retrieve messages for a conversation, ordered by timestamp (sort key)
- The sliding window benchmark scenario needs pagination via `LastEvaluatedKey` if results exceed 1MB

**Detection:** DynamoDB writes fail with `ValidationException: Item size has exceeded the maximum allowed size` during data seeding with large conversation profiles.

**Phase:** DynamoDB schema design.

---

### Pitfall 13: Turso Expected to Lose — Confirmation Bias in Report Framing

**What goes wrong:** The project context explicitly states Turso is "expected to show worse latency." This creates a risk of confirmation bias: interpreting ambiguous results as confirming the hypothesis, or not investigating why Turso's numbers look the way they do.

**Prevention:**
- Report all numbers neutrally. If Turso's write latency is competitive (writes go to the Turso primary which may be in a nearby region), say so.
- The report should explain the architectural reason for any result, not just assert it was expected
- If Turso performs surprisingly well on a scenario, investigate rather than assume error

**Phase:** Report writing.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| DynamoDB schema design | Scan instead of Query (Pitfall 4); wrong partition key design | Design schema against access patterns explicitly before any code |
| DynamoDB implementation | Eventually consistent reads as default (Pitfall 5) | Set ConsistentRead=true in client constructor |
| Turso client implementation | Wrong SDK or embedded replica mode (Pitfall 9, 3) | Use go-libsql, disable embedded replica, pin version |
| Data seeding | DynamoDB on-demand cold start throttling (Pitfall 2); item size limit (Pitfall 12) | Sequential seed with warm-up delay; one item per message |
| Benchmark harness timing | Coordinated omission (Pitfall 1); setup overhead in timings (Pitfall 7) | Ticker-based concurrency; b.ResetTimer discipline |
| Pool/connection setup | Postgres pool cold start (Pitfall 6) | MinConns + explicit warm-up ping before timer starts |
| Concurrent-write scenario | Coordinated omission most acute here (Pitfall 1) | Fixed-rate dispatcher, not sequential issue-and-wait |
| Cost projection model | Wrong DynamoDB pricing units (Pitfall 8) | On-demand pricing, 1KB/4KB billing boundaries |
| Results aggregation | Single-run variance (Pitfall 10) | --runs flag, report min/median/max |
| Report writing | Confirmation bias on Turso (Pitfall 13); transport vs engine conflation (Pitfall 3) | BackendMeta in output; neutral framing |

---

## Sources

- [Fair Benchmarking Considered Difficult: Common Pitfalls](https://mytherin.github.io/papers/2018-dbtest.pdf) — Muehleisen & Raasveldt (VLDB group)
- [Aerospike: Best Practices for Database Benchmarking](https://aerospike.com/blog/best-practices-for-database-benchmarking/) — comprehensive pitfall list
- [hackmysql.com: Benchmarking](https://hackmysql.com/eng/benchmarking/) — 8 specific ways benchmarks are invalidated
- [ScyllaDB: On Coordinated Omission](https://www.scylladb.com/2021/04/22/on-coordinated-omission/) — coordinated omission mechanics
- [AWS: Scaling DynamoDB — partitions, hot keys, split for heat (Part 1)](https://aws.amazon.com/blogs/database/part-1-scaling-dynamodb-how-partitions-hot-keys-and-split-for-heat-impact-performance/) — DynamoDB warm throughput and partition behavior
- [AWS DynamoDB: Understanding warm throughput scenarios](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/warm-throughput-scenarios.html) — on-demand cold start limits
- [AWS DynamoDB: Read Consistency](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.ReadConsistency.html) — strongly vs eventually consistent reads
- [DynamoDB: The Three Limits You Need to Know (Alex DeBrie)](https://www.alexdebrie.com/posts/dynamodb-limits/) — item size, query page size, partition limits
- [AWS: Best Practices for Partition Key Design](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/bp-partition-key-design.html) — access pattern design
- [Turso: Embedded Replicas Introduction](https://docs.turso.tech/features/embedded-replicas/introduction) — remote vs embedded replica behavior
- [tursodatabase/go-libsql GitHub](https://github.com/tursodatabase/go-libsql) — beta status, SDK choice
- [Turso: Bring Your Own SDK / HTTP API](https://turso.tech/blog/bring-your-own-sdk-with-tursos-http-api-ff4ccbed) — hrana protocol
- [jackc/pgx: pgxpool documentation](https://pkg.go.dev/github.com/jackc/pgx/v4/pgxpool) — connection pool lazy initialization
- [How to Write Accurate Benchmarks in Go (P99 CONF)](https://www.p99conf.io/2023/08/16/how-to-write-accurate-benchmarks-in-go/) — b.ResetTimer, setup overhead
- [Go blog: More predictable benchmarking with testing.B.Loop](https://go.dev/blog/testing-b-loop) — Go 1.24+ benchmark loop behavior
- [cybertec-postgresql.com: Network Latency Makes a Big Difference](https://www.cybertec-postgresql.com/en/postgresql-network-latency-does-make-a-big-difference/) — VPC co-location importance
- [cloudonaut.io: DynamoDB Pitfall — Limited Throughput Due to Hot Partitions](https://cloudonaut.io/dynamodb-pitfall-limited-throughput-due-to-hot-partitions/) — hot partition throttling
