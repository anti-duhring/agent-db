# Phase 2: Runner + Postgres Baseline - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

All five benchmark scenarios produce valid p50/p95/p99 latency numbers against a real Postgres backend, and the CLI controls them. This includes the benchmark runner engine, Postgres ChatRepository adapter, metrics collection via HdrHistogram, and CLI flags for backend/scenario/profile/iterations selection. No DynamoDB, no Turso, no cost model, no JSON output, no comparison report.

</domain>

<decisions>
## Implementation Decisions

### Postgres schema design
- **D-01:** Two normalized tables: `conversations` and `messages` with FK relationship. Maps 1:1 to domain types.
- **D-02:** Index on `(partner_id, user_id, updated_at DESC)` for ListConversations. Index on `(conversation_id, created_at DESC)` for LoadWindow.
- **D-03:** Schema managed as embedded SQL files via `//go:embed` in `internal/repository/postgres/migrations/001_create_tables.sql`.
- **D-04:** Prepared statements for all benchmark queries — prepared once at adapter init, reused during iterations. Gives Postgres fair shot at best performance.
- **D-05:** LoadWindow uses `ORDER BY created_at DESC LIMIT N` with reverse in Go. Simple, index-efficient, no window functions needed.

### Runner architecture
- **D-06:** Scenario interface pattern with registry. Each scenario implements `Scenario` interface (Name, Setup, Run, Teardown). Runner iterates registered scenarios.
- **D-07:** Package layout: `internal/benchmark/scenario.go` (interface), `runner.go` (orchestrator), `results.go` (Result types + HdrHistogram), `scenarios/` directory with one file per scenario (append.go, window.go, list.go, coldstart.go, concurrent.go).
- **D-08:** Separate warmup pass — N warmup iterations run first (discarded), then M measured iterations recorded to HdrHistogram. Both configurable via `--warmup` and `--iterations` flags.
- **D-09:** ConcurrentWrites (SCEN-05) uses `golang.org/x/sync/errgroup` with configurable N goroutines. Each goroutine records to shared HdrHistogram (thread-safe). N from `--concurrency` flag.

### CLI output format
- **D-10:** Results grouped by scenario — one section per scenario showing p50/p95/p99 columns. Header shows backend, profile, iteration count, and seed.
- **D-11:** `--dry-run` outputs checklist with pass/fail per step (connection, schema, seed data, sample query).
- **D-12:** Adaptive latency units — microseconds when < 1ms, milliseconds otherwise. HdrHistogram records in microseconds internally.

### Environment isolation
- **D-13:** Testcontainers for everything — both integration tests and benchmark runs spin up a fresh Postgres container. Fully self-contained, no external Postgres dependency.
- **D-14:** Fresh container per run — each benchmark run gets a new Postgres container with schema applied from scratch. Guarantees zero state leakage between runs (METR-04).
- **D-15:** Shared seed data per run — data generated once at run start from the global seed, inserted into Postgres, all scenarios run against the same dataset. Consistent baseline across scenarios.

### Claude's Discretion
- Exact Scenario interface method signatures beyond Name/Setup/Run/Teardown
- HdrHistogram configuration (min/max/precision values)
- Testcontainer configuration (Postgres version, container resource limits)
- Error message wording and exit codes
- Internal runner state management
- Exact CLI flag parsing implementation details

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — IFACE-03 (Postgres), SCEN-01 through SCEN-05, METR-01 through METR-04, CLI-01 through CLI-04 define acceptance criteria for this phase

### Stack decisions
- `CLAUDE.md` §Technology Stack — Locked driver versions: pgx/v5.9.1, HdrHistogram v1.2.0, testcontainers-go/postgres. Use pgxpool not pgx directly. Use expression builder patterns.
- `CLAUDE.md` §Alternatives Considered — Why pgx over lib/pq, why HdrHistogram over testing.B, why stdlib flag over cobra

### Project context
- `.planning/PROJECT.md` — Core value (data-backed evidence), constraints (Go, AWS, benchmark fairness), data model (partner_id/user_id scoping)
- `.planning/ROADMAP.md` §Phase 2 — Success criteria, dependency on Phase 1, requirements list

### Prior phase context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Interface contract (D-01 through D-04), domain types (D-05 through D-08), generator design (D-09 through D-12), project structure (D-13 through D-16)

### Existing code
- `internal/repository/repository.go` — ChatRepository interface (4 methods, locked)
- `internal/domain/types.go` — Conversation, Message, Role types (locked)
- `internal/generator/generator.go` — Seeded data generator (3 profiles)
- `internal/repository/memory/memory.go` — Reference implementation

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ChatRepository` interface (`internal/repository/repository.go`): Postgres adapter must implement these 4 methods exactly
- Domain types (`internal/domain/types.go`): Conversation, Message, Role — Postgres adapter maps these to/from SQL rows
- Data generator (`internal/generator/generator.go`): Produces seeded test data at 3 profiles — runner uses this to populate Postgres before benchmarking
- In-memory adapter (`internal/repository/memory/memory.go`): Reference implementation for testing runner logic without a database

### Established Patterns
- All repository methods: `context.Context` first arg, `(result, error)` return — Postgres adapter follows same pattern
- UUID types from `github.com/google/uuid` — Postgres uses `UUID` column type, pgx handles marshaling natively
- `time.Time` for timestamps — pgx maps to `TIMESTAMPTZ` natively

### Integration Points
- `internal/repository/postgres/` — New package, sibling to `memory/`
- `internal/benchmark/` — New package for runner engine, consumes ChatRepository interface
- `main.go` — Currently bare, will become CLI entry point with flag parsing
- `go.mod` — Will add pgx/v5, hdrhistogram-go, testcontainers-go, errgroup dependencies

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-runner-postgres-baseline*
*Context gathered: 2026-03-31*
