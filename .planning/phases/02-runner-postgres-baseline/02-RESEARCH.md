# Phase 2: Runner + Postgres Baseline - Research

**Researched:** 2026-03-31
**Domain:** Go benchmark runner, pgx/v5 Postgres adapter, HdrHistogram latency measurement, testcontainers-go, errgroup concurrency
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** Two normalized tables: `conversations` and `messages` with FK relationship. Maps 1:1 to domain types.
**D-02:** Index on `(partner_id, user_id, updated_at DESC)` for ListConversations. Index on `(conversation_id, created_at DESC)` for LoadWindow.
**D-03:** Schema managed as embedded SQL files via `//go:embed` in `internal/repository/postgres/migrations/001_create_tables.sql`.
**D-04:** Prepared statements for all benchmark queries — prepared once at adapter init, reused during iterations.
**D-05:** LoadWindow uses `ORDER BY created_at DESC LIMIT N` with reverse in Go.
**D-06:** Scenario interface pattern with registry. Each scenario implements `Scenario` interface (Name, Setup, Run, Teardown). Runner iterates registered scenarios.
**D-07:** Package layout: `internal/benchmark/scenario.go` (interface), `runner.go` (orchestrator), `results.go` (Result types + HdrHistogram), `scenarios/` directory with one file per scenario (append.go, window.go, list.go, coldstart.go, concurrent.go).
**D-08:** Separate warmup pass — N warmup iterations run first (discarded), then M measured iterations recorded to HdrHistogram. Both configurable via `--warmup` and `--iterations` flags.
**D-09:** ConcurrentWrites (SCEN-05) uses `golang.org/x/sync/errgroup` with configurable N goroutines. Each goroutine records to shared HdrHistogram (thread-safe). N from `--concurrency` flag.
**D-10:** Results grouped by scenario — one section per scenario showing p50/p95/p99 columns. Header shows backend, profile, iteration count, and seed.
**D-11:** `--dry-run` outputs checklist with pass/fail per step (connection, schema, seed data, sample query).
**D-12:** Adaptive latency units — microseconds when < 1ms, milliseconds otherwise. HdrHistogram records in microseconds internally.
**D-13:** Testcontainers for everything — both integration tests and benchmark runs spin up a fresh Postgres container.
**D-14:** Fresh container per run — each benchmark run gets a new Postgres container with schema applied from scratch.
**D-15:** Shared seed data per run — data generated once at run start, all scenarios run against the same dataset.

### Claude's Discretion

- Exact Scenario interface method signatures beyond Name/Setup/Run/Teardown
- HdrHistogram configuration (min/max/precision values)
- Testcontainer configuration (Postgres version, container resource limits)
- Error message wording and exit codes
- Internal runner state management
- Exact CLI flag parsing implementation details

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IFACE-03 | Postgres implementation of ChatRepository | pgx/v5 pgxpool + prepared statements via AfterConnect; UUID and time.Time scan natively |
| SCEN-01 | AppendMessage — single message insert, measures write latency | pgxpool.Exec with prepared INSERT; HdrHistogram.RecordValue per iteration |
| SCEN-02 | LoadSlidingWindow — last N=20 messages, keyset pagination | D-05: ORDER BY created_at DESC LIMIT N + reverse in Go; prepared SELECT |
| SCEN-03 | ListConversations — all convs for (partner_id, user_id) sorted by last activity | Composite index D-02; prepared SELECT ORDER BY updated_at DESC |
| SCEN-04 | ColdStartLoad — first window read after fresh connection (no warmup) | Requires acquiring/releasing a dedicated pool connection per iteration; no statement cache warm |
| SCEN-05 | ConcurrentWrites — N goroutines appending in parallel | errgroup.WithContext + SetLimit(N); external mutex on shared HdrHistogram (not thread-safe) |
| METR-01 | p50/p95/p99 per scenario per backend | HdrHistogram.ValueAtPercentile(50/95/99); records in microseconds |
| METR-02 | Warmup excluded from measurement | Two-pass loop: warmup iterations first (no RecordValue), then measured iterations |
| METR-03 | --iterations N flag | stdlib flag.IntVar; passed to runner as RunConfig |
| METR-04 | Clean schema/data per run | D-14: fresh testcontainer per run; schema from embedded SQL via WithInitScripts or Exec after connect |
| CLI-01 | --dry-run mode | Checklist output: connect, schema check, seed insert, sample query |
| CLI-02 | --backend flag | flag.String; "postgres" only for this phase |
| CLI-03 | --scenario flag | flag.String; comma-separated or "all"; maps to scenario registry lookup |
| CLI-04 | --profile flag | flag.String; "small"/"medium"/"large"; maps to generator.Small/Medium/Large |
</phase_requirements>

---

## Summary

Phase 2 builds three interlocking pieces: a Postgres ChatRepository adapter using pgx/v5 with prepared statements, a benchmark runner engine that orchestrates warmup + measured iterations across registered scenarios, and a CLI that wires everything together via stdlib `flag`. All work runs inside a fresh Postgres testcontainer on every invocation — no external Postgres required.

The most critical API decision is how prepared statements work with pgxpool. pgxpool.Pool has no `Prepare` method — prepared statements must be registered per-connection via the `AfterConnect` callback in `pgxpool.Config`. The pool then calls this callback for each new connection, ensuring every connection in the pool has the statements pre-parsed. Benchmark queries reference statements by name string. This is the only correct pattern for prepared statements with a pool.

HdrHistogram is not thread-safe. SCEN-05 (ConcurrentWrites) uses multiple goroutines, so either each goroutine owns a private histogram that gets merged at the end, or a sync.Mutex wraps every RecordValue call. The per-goroutine + merge approach is cleaner and avoids lock contention in the measurement path.

**Primary recommendation:** Implement the Postgres adapter first (D-01 through D-05), then the runner skeleton (D-06 through D-08), then the five scenarios against the adapter, then the CLI (D-10 through D-12).

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5/pgxpool` | v5.9.1 (2026-03-22) | Postgres connection pool | Locked in CLAUDE.md; pgxpool not pgx directly |
| `github.com/HdrHistogram/hdrhistogram-go` | v1.2.0 (2025-11-09) | p50/p95/p99 latency recording | Locked in CLAUDE.md; O(1) record cost, fixed memory |
| `golang.org/x/sync/errgroup` | v0.20.0 (2026-02-23) | Concurrent goroutine management for SCEN-05 | Locked via D-09 |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | v0.41.0 (2026-03-10) | Postgres container for benchmark + tests | Locked via D-13/D-14 |
| `encoding/embed` (stdlib) | Go 1.26 | Embed SQL migration files | Used for D-03 |
| `flag` (stdlib) | Go 1.26 | CLI flag parsing | Locked in CLAUDE.md; 5 flags, cobra unjustified |
| `text/tabwriter` (stdlib) | Go 1.26 | Formatted results table output | Locked in CLAUDE.md |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/uuid` | v1.6.0 | UUID generation for Postgres rows | Already in go.mod; pgx scans UUID natively |
| `github.com/brianvoe/gofakeit/v7` | v7.14.1 | Generator already available | Seed data for benchmark run |
| `sync` (stdlib) | Go 1.26 | Mutex for HdrHistogram in SCEN-05 | Needed if per-goroutine histograms not used |

**Installation (new dependencies to add):**
```bash
go get github.com/jackc/pgx/v5@v5.9.1
go get github.com/HdrHistogram/hdrhistogram-go@v1.2.0
go get golang.org/x/sync@v0.20.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.41.0
```

**Version verification (confirmed against Go module proxy 2026-03-31):**
- pgx/v5 v5.9.1 — published 2026-03-22 (current)
- hdrhistogram-go v1.2.0 — published 2025-11-09 (current)
- golang.org/x/sync v0.20.0 — published 2026-02-23 (current)
- testcontainers-go v0.41.0 — published 2026-03-10 (current)

---

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── repository/
│   ├── repository.go          # ChatRepository interface (existing, locked)
│   ├── memory/                # In-memory adapter (existing)
│   └── postgres/
│       ├── postgres.go        # PostgresRepository struct + New() + interface impl
│       └── migrations/
│           └── 001_create_tables.sql   # Embedded via //go:embed
internal/benchmark/
├── scenario.go                # Scenario interface
├── runner.go                  # Runner struct, RunConfig, orchestration loop
├── results.go                 # ScenarioResult, HdrHistogram wrapper
└── scenarios/
    ├── append.go              # SCEN-01: AppendMessage
    ├── window.go              # SCEN-02: LoadSlidingWindow
    ├── list.go                # SCEN-03: ListConversations
    ├── coldstart.go           # SCEN-04: ColdStartLoad
    └── concurrent.go          # SCEN-05: ConcurrentWrites
main.go                        # CLI entry point (currently bare stub)
```

### Pattern 1: pgxpool with Prepared Statements via AfterConnect

**What:** pgxpool.Pool has no Prepare method. Prepared statements must be registered on each connection via the AfterConnect callback.

**When to use:** All four ChatRepository methods — prepare once per connection at pool init, reference by name string in queries.

```go
// Source: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Config
config, err := pgxpool.ParseConfig(connString)
if err != nil {
    return nil, err
}
config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
    stmts := []struct{ name, sql string }{
        {"create_conversation", "INSERT INTO conversations ..."},
        {"append_message", "INSERT INTO messages ..."},
        {"load_window", "SELECT ... ORDER BY created_at DESC LIMIT $2"},
        {"list_conversations", "SELECT ... WHERE partner_id=$1 AND user_id=$2 ORDER BY updated_at DESC"},
    }
    for _, s := range stmts {
        if _, err := conn.Prepare(ctx, s.name, s.sql); err != nil {
            return fmt.Errorf("prepare %s: %w", s.name, err)
        }
    }
    return nil
}
pool, err := pgxpool.NewWithConfig(ctx, config)
```

### Pattern 2: Embedded SQL Schema

**What:** `//go:embed` directive includes the SQL file in the binary. Schema applied at adapter init by executing the embedded SQL.

```go
// Source: https://pkg.go.dev/embed
//go:embed migrations/001_create_tables.sql
var schema string

func applySchema(ctx context.Context, pool *pgxpool.Pool) error {
    _, err := pool.Exec(ctx, schema)
    return err
}
```

**Testcontainers alternative:** Pass the SQL file via `postgres.WithInitScripts(path)` which places it in `/docker-entrypoint-initdb.d` and runs it automatically before the container is ready. However, this requires a real file path, not an embedded string. For benchmark runs that use a fresh container programmatically, calling `applySchema` after `pool.Ping` is simpler and works with the embedded string.

### Pattern 3: LoadWindow — DESC Query + Go Reverse

**What:** D-05 mandates `ORDER BY created_at DESC LIMIT N` (which uses the index efficiently) then reverse the slice in Go to return oldest-first.

```go
// Prepared statement returns newest-first
rows, err := pool.Query(ctx, "load_window", conversationID, n)
if err != nil {
    return nil, err
}
msgs, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Message])
if err != nil {
    return nil, err
}
// Reverse in-place to return oldest-first (matches ChatRepository contract)
for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
    msgs[i], msgs[j] = msgs[j], msgs[i]
}
return msgs, nil
```

### Pattern 4: Scenario Interface and Runner Loop

**What:** D-06 registry pattern — each scenario self-registers, runner iterates the list.

```go
// internal/benchmark/scenario.go
type Scenario interface {
    Name() string
    Setup(ctx context.Context, repo repository.ChatRepository, data generator.GeneratedData) error
    Run(ctx context.Context, repo repository.ChatRepository) error
    Teardown(ctx context.Context) error
}
```

Runner loop structure:
1. For each scenario: call Setup once
2. Warmup loop: call Run N times, discard results
3. Measured loop: call Run M times, record `time.Since(start)` in microseconds to HdrHistogram
4. Call Teardown once
5. Emit ScenarioResult with p50/p95/p99 from histogram

### Pattern 5: HdrHistogram Configuration

**What:** Histogram configured once per scenario. Records in microseconds.

```go
// Source: https://pkg.go.dev/github.com/HdrHistogram/hdrhistogram-go
// lowestDiscernibleValue=1µs, highestTrackableValue=30s (30,000,000µs), 3 significant digits
h := hdrhistogram.New(1, 30_000_000, 3)

start := time.Now()
// ... benchmark operation ...
elapsed := time.Since(start).Microseconds()
if err := h.RecordValue(elapsed); err != nil {
    // value out of range — widen highestTrackableValue if this occurs
}

p50 := h.ValueAtPercentile(50.0)   // in microseconds
p95 := h.ValueAtPercentile(95.0)
p99 := h.ValueAtPercentile(99.0)
```

**Recommended config:** `New(1, 30_000_000, 3)` — tracks 1µs to 30s, 3 digits precision (±0.1% error), ~35KB memory per histogram.

### Pattern 6: SCEN-05 ConcurrentWrites — Per-Goroutine Histograms

**What:** HdrHistogram is not thread-safe. Each goroutine owns a private histogram. Merge after all goroutines complete.

```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(concurrency)

histograms := make([]*hdrhistogram.Histogram, concurrency)
for i := 0; i < concurrency; i++ {
    i := i
    histograms[i] = hdrhistogram.New(1, 30_000_000, 3)
    g.Go(func() error {
        for iter := 0; iter < iterationsPerGoroutine; iter++ {
            start := time.Now()
            err := repo.AppendMessage(ctx, convID, role, content)
            elapsed := time.Since(start).Microseconds()
            if err != nil {
                return err
            }
            histograms[i].RecordValue(elapsed)
        }
        return nil
    })
}
if err := g.Wait(); err != nil {
    return nil, err
}
// Merge all per-goroutine histograms into one
merged := hdrhistogram.New(1, 30_000_000, 3)
for _, h := range histograms {
    merged.Merge(h)
}
```

### Pattern 7: Testcontainer Lifecycle for Benchmark Runs

**What:** D-13/D-14 require a fresh container per run. Container starts before any benchmark work, terminates after.

```go
// Source: https://golang.testcontainers.org/modules/postgres/
ctx := context.Background()
ctr, err := postgres.Run(ctx,
    "postgres:16-alpine",
    postgres.WithDatabase("agentdb"),
    postgres.WithUsername("bench"),
    postgres.WithPassword("bench"),
    postgres.BasicWaitStrategies(),
)
if err != nil {
    return err
}
defer testcontainers.TerminateContainer(ctr)

connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
if err != nil {
    return err
}
// connStr passed to pgxpool.New + AfterConnect prepared statements
```

### Pattern 8: Adaptive Latency Units (D-12)

**What:** Display microseconds when p50 < 1000µs, milliseconds otherwise.

```go
func formatLatency(microseconds int64) string {
    if microseconds < 1000 {
        return fmt.Sprintf("%dµs", microseconds)
    }
    return fmt.Sprintf("%.2fms", float64(microseconds)/1000.0)
}
```

### Anti-Patterns to Avoid

- **Using pgx.Conn directly instead of pgxpool:** A single connection serializes all benchmark operations. CLAUDE.md explicitly forbids this.
- **Using pgx.QueryModeExec to disable the statement cache:** The auto-cache in pgx works per-connection. D-04 requires explicit prepared statements via AfterConnect for deterministic behavior.
- **Calling RunContainer instead of Run:** `postgres.RunContainer` is deprecated in testcontainers-go v0.41.0. Use `postgres.Run`.
- **Using a global/shared HdrHistogram with concurrent goroutines:** HdrHistogram is not documented as thread-safe. Per-goroutine histograms + merge is correct.
- **`ORDER BY created_at ASC LIMIT N` for LoadWindow:** This does not use the DESC index on `(conversation_id, created_at DESC)` efficiently. Use DESC + reverse.
- **Recording nanoseconds in HdrHistogram:** RecordValue uses int64; nanoseconds overflow the 30-second max. Always convert to microseconds: `time.Since(start).Microseconds()`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Percentile histograms | Custom bucket tracking | `hdrhistogram.New(1, 30_000_000, 3)` | HDR avoids bucket boundary artifacts; O(1) record; merge is built-in |
| Concurrent goroutine error aggregation | Manual WaitGroup + channel | `errgroup.WithContext` + `SetLimit(N)` | First error propagated automatically; context cancelation built in |
| Postgres container lifecycle | Docker SDK directly | `testcontainers-go postgres.Run` | Wait strategies, init scripts, connection string extraction all handled |
| SQL type marshaling (UUID, time.Time) | Manual `[]byte` conversion | pgx native scanning | pgx registers ~70 PostgreSQL types including UUID and TIMESTAMPTZ |

**Key insight:** The measurement infrastructure (histogram, concurrency, containers) has more edge cases than the storage code. Using proven libraries here keeps the benchmark credible.

---

## Common Pitfalls

### Pitfall 1: pgxpool + Prepared Statements — AfterConnect Required

**What goes wrong:** Developer calls `pool.Prepare(...)` — this method does not exist on `*pgxpool.Pool`. The code fails to compile or the developer falls back to unprepared queries, which violates D-04.

**Why it happens:** pgxpool documentation is not immediately clear that prepared statements are a per-connection concern.

**How to avoid:** Use `config.AfterConnect` callback exclusively. Prepare all four query statements there. Verify with a compile-time interface check (`var _ repository.ChatRepository = (*PostgresRepository)(nil)`).

**Warning signs:** Any call to `pool.Prepare` in code is a compilation error.

### Pitfall 2: HdrHistogram RecordValue Error on Out-of-Range Values

**What goes wrong:** If a benchmark operation takes longer than `highestTrackableValue` (e.g., container startup latency leaks into first iteration), `RecordValue` returns an error and the sample is lost.

**Why it happens:** The histogram silently drops values outside [lowestDiscernibleValue, highestTrackableValue].

**How to avoid:** Set `highestTrackableValue` to 30 seconds (30_000_000 µs) — far above any realistic Postgres latency. Log and count RecordValue errors in the runner; if any occur, emit a warning. The warmup pass (D-08) absorbs container startup latency before measurement begins.

**Warning signs:** `h.TotalCount()` after measurement is less than `--iterations` value.

### Pitfall 3: SCEN-04 ColdStart Does Not Use the Pool's Connection Cache

**What goes wrong:** The pool maintains idle connections. Acquiring a connection from the pool for SCEN-04 may reuse an already-warmed connection, defeating the cold-start intent.

**Why it happens:** pgxpool keeps connections alive between acquisitions.

**How to avoid:** For SCEN-04, configure a pool with `MaxConns: 1`, `MaxConnIdleTime: 1ms` (or close the pool and create a fresh one per iteration). Alternatively, track this as a "best-effort cold start" and document the methodology — the pool will have drained if the test runs after a period of inactivity. Document the approach clearly in the scenario's Setup comment.

**Warning signs:** SCEN-04 p50 latency is indistinguishable from SCEN-02 (window read) — suggests connection reuse.

### Pitfall 4: testcontainers-go — No Wait Strategy Causes Flaky Starts

**What goes wrong:** Container starts but Postgres is not ready yet. First connection attempt fails.

**Why it happens:** Docker container "running" state does not mean Postgres is accepting connections.

**How to avoid:** Always include `postgres.BasicWaitStrategies()` in the `postgres.Run` call. This waits for the Postgres log line confirming readiness.

**Warning signs:** Intermittent `connection refused` errors on the first pool connection.

### Pitfall 5: Embedded SQL + `//go:embed` Path Resolution

**What goes wrong:** `//go:embed migrations/001_create_tables.sql` resolves relative to the `.go` file's package directory, not the working directory. If the embed directive is in a different package than expected, the path will fail to compile.

**Why it happens:** Go embed paths are resolved at compile time relative to the package directory containing the `//go:embed` comment.

**How to avoid:** Place the embed directive in `internal/repository/postgres/postgres.go`. The `migrations/` directory must be at `internal/repository/postgres/migrations/`. The embed variable must be in the same package as the directive.

**Warning signs:** `go build` error: `pattern migrations/001_create_tables.sql: no matching files found`.

### Pitfall 6: ConcurrentWrites — Loop Variable Capture (Pre-Go 1.22)

**What goes wrong:** Goroutine closures capture the loop variable by reference, so all goroutines see the same (final) index value.

**Why it happens:** Classic Go loop variable capture bug.

**How to avoid:** The go.mod is set to `go 1.26`, which uses Go 1.22+ loop variable semantics where loop variables are per-iteration. This pitfall does not apply to this project, but confirm `go 1.26` in go.mod before assuming it.

**Warning signs:** All goroutines write to `histograms[N-1]` instead of their assigned index.

---

## Code Examples

### Schema SQL (D-01, D-02, D-03)
```sql
-- Source: D-01, D-02 decisions from CONTEXT.md
CREATE TABLE IF NOT EXISTS conversations (
    id         UUID PRIMARY KEY,
    partner_id UUID NOT NULL,
    user_id    UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversations_list
    ON conversations (partner_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
    id              UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    token_count     INT  NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_window
    ON messages (conversation_id, created_at DESC);
```

### Postgres Adapter Constructor
```go
// Source: pgxpool.Config AfterConnect pattern
// https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Config
type PostgresRepository struct {
    pool *pgxpool.Pool
}

var _ repository.ChatRepository = (*PostgresRepository)(nil)

func New(ctx context.Context, connString string) (*PostgresRepository, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
        _, err := conn.Prepare(ctx, "insert_conversation",
            `INSERT INTO conversations (id, partner_id, user_id, created_at, updated_at)
             VALUES ($1, $2, $3, $4, $5)
             RETURNING id, partner_id, user_id, created_at, updated_at`)
        if err != nil {
            return err
        }
        _, err = conn.Prepare(ctx, "insert_message",
            `INSERT INTO messages (id, conversation_id, role, content, token_count, created_at)
             VALUES ($1, $2, $3, $4, $5, $6)
             RETURNING id, conversation_id, role, content, token_count, created_at`)
        if err != nil {
            return err
        }
        _, err = conn.Prepare(ctx, "load_window",
            `SELECT id, conversation_id, role, content, token_count, created_at
             FROM messages
             WHERE conversation_id = $1
             ORDER BY created_at DESC
             LIMIT $2`)
        if err != nil {
            return err
        }
        _, err = conn.Prepare(ctx, "list_conversations",
            `SELECT id, partner_id, user_id, created_at, updated_at
             FROM conversations
             WHERE partner_id = $1 AND user_id = $2
             ORDER BY updated_at DESC`)
        return err
    }
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("new pool: %w", err)
    }
    if err := applySchema(ctx, pool); err != nil {
        pool.Close()
        return nil, fmt.Errorf("apply schema: %w", err)
    }
    return &PostgresRepository{pool: pool}, nil
}
```

### AppendMessage with UpdatedAt Touch
```go
// AppendMessage inserts a message and updates conversations.updated_at.
// Two statements: one INSERT into messages, one UPDATE on conversations.
// Both use prepared statements.
func (r *PostgresRepository) AppendMessage(
    ctx context.Context, conversationID uuid.UUID, role domain.Role, content string,
) (domain.Message, error) {
    id := uuid.New()
    now := time.Now().UTC()
    tokenCount := len(content) / 4

    var msg domain.Message
    err := r.pool.QueryRow(ctx, "insert_message",
        id, conversationID, string(role), content, tokenCount, now,
    ).Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.TokenCount, &msg.CreatedAt)
    if err != nil {
        return domain.Message{}, fmt.Errorf("insert message: %w", err)
    }
    // Touch updated_at on the parent conversation (benchmark measures full write path)
    _, err = r.pool.Exec(ctx,
        "UPDATE conversations SET updated_at = $1 WHERE id = $2",
        now, conversationID)
    if err != nil {
        return domain.Message{}, fmt.Errorf("update conversation: %w", err)
    }
    return msg, nil
}
```

**Note:** The UPDATE on conversations is not a prepared statement in the example above. For D-04 compliance, add an `"update_conversation_updated_at"` prepared statement in AfterConnect.

### Runner Configuration and Loop
```go
// internal/benchmark/runner.go
type RunConfig struct {
    Warmup      int
    Iterations  int
    Concurrency int
    Profile     generator.Profile
    Seed        int64
}

type Runner struct {
    repo      repository.ChatRepository
    scenarios []Scenario
    config    RunConfig
}

func (r *Runner) Run(ctx context.Context) ([]ScenarioResult, error) {
    gen := generator.New(r.config.Seed)
    partnerID, userID := uuid.New(), uuid.New()
    data := gen.Generate(partnerID, userID, r.config.Profile)

    // Insert seed data into repository
    if err := seedRepository(ctx, r.repo, data); err != nil {
        return nil, fmt.Errorf("seed data: %w", err)
    }

    var results []ScenarioResult
    for _, sc := range r.scenarios {
        if err := sc.Setup(ctx, r.repo, data); err != nil {
            return nil, fmt.Errorf("scenario %s setup: %w", sc.Name(), err)
        }

        h := hdrhistogram.New(1, 30_000_000, 3)

        // Warmup (discarded)
        for i := 0; i < r.config.Warmup; i++ {
            _ = sc.Run(ctx, r.repo) // errors logged but not fatal in warmup
        }

        // Measured iterations
        for i := 0; i < r.config.Iterations; i++ {
            start := time.Now()
            if err := sc.Run(ctx, r.repo); err != nil {
                return nil, fmt.Errorf("scenario %s iteration %d: %w", sc.Name(), i, err)
            }
            _ = h.RecordValue(time.Since(start).Microseconds())
        }

        _ = sc.Teardown(ctx)

        results = append(results, ScenarioResult{
            ScenarioName: sc.Name(),
            P50:          h.ValueAtPercentile(50.0),
            P95:          h.ValueAtPercentile(95.0),
            P99:          h.ValueAtPercentile(99.0),
            TotalCount:   h.TotalCount(),
        })
    }
    return results, nil
}
```

### CLI main.go Skeleton
```go
// main.go
func main() {
    backend  := flag.String("backend",   "postgres", "backend to benchmark (postgres)")
    scenario := flag.String("scenario",  "all",      "scenario(s) to run (all,append,window,list,coldstart,concurrent)")
    profile  := flag.String("profile",   "medium",   "data profile (small,medium,large)")
    iters    := flag.Int("iterations",   100,        "measured iteration count per scenario")
    warmup   := flag.Int("warmup",       10,         "warmup iteration count (discarded)")
    conc     := flag.Int("concurrency",  10,         "goroutine count for concurrent scenario")
    seed     := flag.Int64("seed",       42,         "RNG seed for deterministic data")
    dryRun   := flag.Bool("dry-run",     false,      "verify connectivity without running benchmarks")
    flag.Parse()
    // ...
}
```

### Output Table with tabwriter
```go
// Source: text/tabwriter stdlib
w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
fmt.Fprintf(w, "SCENARIO\tP50\tP95\tP99\tCOUNT\n")
for _, r := range results {
    fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
        r.ScenarioName,
        formatLatency(r.P50),
        formatLatency(r.P95),
        formatLatency(r.P99),
        r.TotalCount,
    )
}
w.Flush()
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `testcontainers.RunContainer(ctx, opts...)` | `postgres.Run(ctx, img, opts...)` | testcontainers-go v0.20.0+ | Old function is deprecated in v0.41.0; use `postgres.Run` |
| aws-sdk-go v1 | aws-sdk-go-v2 | 2023 (v1 maintenance-only) | Irrelevant this phase; noted for Phase 3 |
| pgx automatic statement cache | Explicit `AfterConnect` prepare | Recommended for benchmarks since pgx v4 | Cache can evict; explicit prepare guarantees statements always available |

**Deprecated/outdated:**
- `postgres.RunContainer(ctx, opts...)`: deprecated in testcontainers-go, use `postgres.Run(ctx, img, opts...)`
- `pgx.Connect` (single connection): do not use for benchmarks; use pgxpool

---

## Open Questions

1. **SCEN-04 ColdStart — true cold start vs. pool warm connection**
   - What we know: pgxpool reuses idle connections; "cold start" intent is first read with no cached state
   - What's unclear: Whether planner intends "fresh TCP connection" or "fresh statement cache"; the decision (D-04) uses persistent prepared statements, which are compatible with warmup
   - Recommendation: Define SCEN-04 as "acquire connection, disable statement cache for that acquire, execute one LoadWindow, release"; document methodology. Alternatively, keep the pool but close+reopen the pool for each SCEN-04 iteration (slower but unambiguous). Planner should clarify.

2. **AppendMessage — single INSERT or INSERT + UPDATE transaction**
   - What we know: memory adapter updates `UpdatedAt` on AppendMessage; benchmark measures "full write path"
   - What's unclear: D-04 says "prepared statements for all benchmark queries" — does the UPDATE on conversations count as a separate benchmark query or an implementation detail?
   - Recommendation: Include the UPDATE in the same logical operation but not wrapped in a transaction (avoid transaction overhead in the benchmark). Prepare both statements.

3. **Seed data insertion — inside timed path or setup only**
   - What we know: D-15 says "data generated once at run start"; scenarios run against the same dataset
   - What's unclear: Whether seedRepository uses the Postgres adapter's ChatRepository methods (slow, goes through prepared statements) or a bulk COPY approach
   - Recommendation: Use bulk COPY via pgx's CopyFrom API for seed data insertion (much faster at 500-5000 messages per run). This keeps seed time short and does not inflate benchmark p99. The benchmark itself uses the 4 ChatRepository methods.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go runtime | All code | Yes | go1.26.0 linux/amd64 | — |
| Docker | testcontainers-go | Yes | 28.2.2 | — |
| PostgreSQL client (psql) | Manual debugging only | Yes | 16.13 | — |
| Internet (Go module proxy) | `go get` for new deps | Yes | — | Vendor mode if needed |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None. All required tools are present.

---

## Project Constraints (from CLAUDE.md)

These directives are mandatory and cannot be overridden by research findings:

- **Language:** Go only. No polyglot additions.
- **Postgres driver:** `github.com/jackc/pgx/v5/pgxpool` — use pgxpool, not pgx.Conn directly.
- **DynamoDB driver:** `aws-sdk-go-v2` only (v1 forbidden). Not in scope for this phase.
- **Turso driver:** `libsql-client-go` (no CGO). Not in scope for this phase.
- **Latency measurement:** `hdrhistogram-go` only. `testing.B` and prometheus histograms are explicitly excluded.
- **Data generation:** `gofakeit/v7` only. Never call global gofakeit functions — always use seeded instance.
- **CLI:** stdlib `flag` only. `cobra` is explicitly excluded.
- **Report output:** stdlib `encoding/json` and `text/tabwriter` only.
- **Local testing:** testcontainers-go for Postgres. No mocking of database layer.
- **Go version:** go.mod directive `go 1.26`; code should compile on 1.25.4 (no 1.26-specific syntax).
- **GSD workflow:** All file changes must go through a GSD command (`/gsd:execute-phase`).

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/jackc/pgx/v5/pgxpool` — pgxpool.Config, AfterConnect, Pool methods
- `pkg.go.dev/github.com/HdrHistogram/hdrhistogram-go` — New(), RecordValue(), ValueAtPercentile(), thread-safety
- `pkg.go.dev/golang.org/x/sync/errgroup` — WithContext, Group.Go, Group.Wait, SetLimit
- `golang.testcontainers.org/modules/postgres/` — postgres.Run, BasicWaitStrategies, ConnectionString
- Go module proxy `proxy.golang.org` — version verification for all four new dependencies

### Secondary (MEDIUM confidence)
- pgx v5 documentation on prepared statements and QueryExecMode — cross-verified with pgxpool source

### Tertiary (LOW confidence)
- SCEN-04 cold start approach — no official guidance; derived from pgxpool behavior documentation

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified on Go module proxy 2026-03-31
- Architecture: HIGH — patterns derived from official pkg.go.dev docs + locked CONTEXT.md decisions
- Pitfalls: HIGH — pgxpool/prepared statement pitfall verified from API docs; others from reasoning about the stack
- Environment availability: HIGH — verified by direct command invocation

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable libraries; testcontainers-go moves fast but API is stable)
