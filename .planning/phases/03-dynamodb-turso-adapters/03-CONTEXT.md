# Phase 3: DynamoDB + Turso Adapters - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

All five benchmark scenarios run against DynamoDB and Turso, completing the three-way comparison with a correctly designed DynamoDB schema. This includes DynamoDB and Turso ChatRepository adapter implementations, local testing infrastructure (LocalStack for DynamoDB, Turso Cloud dev DB), and CLI wiring for multi-backend selection. No cost model, no comparison report, no new scenarios or runner changes.

</domain>

<decisions>
## Implementation Decisions

### DynamoDB table design
- **D-01:** Single-table design. One DynamoDB table (`chat_data`) with composite keys covering all access patterns. No GSI needed.
- **D-02:** Conversation listing items: PK=`USER#<partner_id>#<user_id>`, SK=`CONV#<updated_at>#<conv_id>`. Query PK with ScanIndexForward=false for ListConversations sorted by last activity.
- **D-03:** Message items: PK=`CONV#<conv_id>`, SK=`MSG#<created_at>#<msg_id>`. Query PK with ScanIndexForward=false, Limit=N for LoadWindow.
- **D-04:** AppendMessage uses TransactWriteItems for atomic consistency: (1) Put message item, (2) Delete old conversation SK, (3) Put new conversation SK with updated timestamp. Guarantees listing always reflects latest message timestamp.
- **D-05:** ConsistentRead=true for all read scenarios per roadmap success criteria. Warmup pass before timing begins.
- **D-06:** No Scan operations — all reads use Query per roadmap success criteria.

### Turso SDK and driver
- **D-07:** Use `libsql-client-go` (deprecated but functional, pure Go, no CGO). Connects via `libsql://` protocol to Turso Cloud.
- **D-08:** Raw `database/sql` for all queries — direct `sql.Query` + manual `rows.Scan` into domain types. No sqlx or other scanning libraries. Standard Go pattern, sufficient for 4 methods.
- **D-09:** Turso schema mirrors Postgres schema (two tables: conversations, messages) since Turso is SQLite-compatible. Same indexes adapted to SQLite syntax.

### Local testing strategy
- **D-10:** DynamoDB uses LocalStack via testcontainers-go — same pattern as Postgres. Spins up LocalStack container, creates table, runs tests, auto-cleans up.
- **D-11:** Turso uses a dedicated dev/staging database on Turso Cloud. Connection via environment variables (`TURSO_URL`, `TURSO_AUTH_TOKEN`). Real internet latency is the point — that's what the benchmark measures.
- **D-12:** Tests and benchmarks gracefully skip Turso if env vars are not set. No failure, just skip with informational message.

### CLI integration
- **D-13:** `--backend` flag accepts `postgres`, `dynamodb`, `turso`, comma-separated combinations, or `all`. Extends the existing flag validation in `main.go`.
- **D-14:** `--backend all` skips unavailable backends with a warning line and runs whatever's available. Results table shows "skipped" for unavailable backends. No failure.
- **D-15:** BackendMeta transport annotation for Turso: header note above Turso results showing `Transport: libsql:// (remote, internet)` plus a note that latency includes internet round-trip to Turso Cloud.
- **D-16:** Side-by-side output when running multiple backends — each backend gets its own results section with header metadata.

### Claude's Discretion
- DynamoDB attribute naming conventions (beyond PK/SK structure)
- Exact DynamoDB table provisioning settings (on-demand vs provisioned for benchmark)
- Turso schema DDL details (column types, index names)
- Error message wording and retry behavior
- Container configuration details (LocalStack version, resource limits)
- Exact CLI output formatting and spacing

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — IFACE-03 (DynamoDB and Turso implementations) defines acceptance criteria for this phase

### Stack decisions
- `CLAUDE.md` §Technology Stack — Locked driver versions: aws-sdk-go-v2/service/dynamodb v1.57.1, aws-sdk-go-v2/config, aws-sdk-go-v2/feature/dynamodb/attributevalue, libsql-client-go. Use expression builder for DynamoDB, not raw map construction.
- `CLAUDE.md` §Alternatives Considered — Why aws-sdk-go-v2 over v1, why libsql-client-go over go-libsql, why LocalStack over DynamoDB Local

### Project context
- `.planning/PROJECT.md` — Core value (data-backed evidence), constraints (Go, AWS), data model (partner_id/user_id scoping), Turso latency expectations
- `.planning/ROADMAP.md` §Phase 3 — Success criteria (no Scan ops, ConsistentRead, BackendMeta annotation, --backend all side-by-side)

### Prior phase context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Interface contract (D-01 through D-04), domain types (D-05 through D-08), project structure (D-13 through D-16)
- `.planning/phases/02-runner-postgres-baseline/02-CONTEXT.md` — Runner architecture (D-06 through D-09), testcontainers pattern (D-13 through D-15), CLI output format (D-10 through D-12)

### Existing code
- `internal/repository/repository.go` — ChatRepository interface (4 methods, locked)
- `internal/domain/types.go` — Conversation, Message, Role types (locked)
- `internal/benchmark/runner.go` — Runner orchestrator, SeedResult, seedRepository
- `internal/benchmark/scenario.go` — Scenario interface, WarmupSkipper
- `internal/benchmark/scenarios/` — Five scenario implementations (append, window, list, coldstart, concurrent)
- `internal/repository/postgres/postgres.go` — Reference adapter implementation pattern
- `main.go` — CLI entry point with flag parsing, container lifecycle, backend wiring

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ChatRepository` interface (`internal/repository/repository.go`): DynamoDB and Turso adapters must implement these 4 methods exactly
- Domain types (`internal/domain/types.go`): Adapters map these to/from DynamoDB items or SQL rows
- Runner + scenarios (`internal/benchmark/`): Already built — new adapters just plug in via ChatRepository interface
- Postgres adapter (`internal/repository/postgres/postgres.go`): Reference implementation pattern — schema embed, constructor with setup, prepared statements, Close method
- Testcontainers pattern: Postgres module usage in `main.go` — DynamoDB follows same lifecycle pattern with LocalStack module

### Established Patterns
- All repository methods: `context.Context` first arg, `(result, error)` return
- UUID types from `github.com/google/uuid` — DynamoDB stores as string attributes, Turso stores as TEXT
- `time.Time` for timestamps — DynamoDB stores as ISO8601 string or Unix epoch, Turso stores as TEXT
- Schema via `//go:embed` — Postgres uses embedded SQL migration file
- Compile-time interface check: `var _ repository.ChatRepository = (*XRepository)(nil)`

### Integration Points
- `internal/repository/dynamodb/` — New package, sibling to `postgres/` and `memory/`
- `internal/repository/turso/` — New package, sibling to others
- `main.go` — Extends backend switch to create DynamoDB/Turso repos, manage container lifecycle
- `go.mod` — Adds aws-sdk-go-v2 modules, libsql-client-go, testcontainers-go/localstack

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

*Phase: 03-dynamodb-turso-adapters*
*Context gathered: 2026-03-31*
