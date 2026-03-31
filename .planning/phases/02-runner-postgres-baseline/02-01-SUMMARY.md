---
phase: 02-runner-postgres-baseline
plan: "01"
subsystem: database
tags: [postgres, pgx, pgxpool, testcontainers, integration-testing, prepared-statements]

requires:
  - phase: 01-foundation
    provides: ChatRepository interface, domain types (Conversation, Message, Role)

provides:
  - PostgresRepository struct implementing all 4 ChatRepository methods
  - SQL schema with conversations and messages tables plus 2 performance indexes
  - 5 prepared statements registered via AfterConnect on each pool connection
  - Integration test suite (7 tests) against real Postgres via testcontainers-go

affects:
  - phase 02-runner-postgres-baseline (plans 02+)
  - phase 03 (DynamoDB and Turso adapters — same interface pattern to follow)

tech-stack:
  added:
    - github.com/jackc/pgx/v5 v5.9.1
    - github.com/jackc/pgx/v5/pgxpool v5.9.1
    - github.com/testcontainers/testcontainers-go v0.41.0
    - github.com/testcontainers/testcontainers-go/modules/postgres v0.41.0
  patterns:
    - Schema applied via direct pgx.Connect before pool creation (AfterConnect ordering fix)
    - AfterConnect callback registers named prepared statements on every pool connection
    - LoadWindow uses DESC query + in-place slice reverse for oldest-first output
    - //go:embed for SQL schema embedding (no runtime file I/O)
    - Integration tests use setupTestRepo helper with t.Cleanup for container teardown

key-files:
  created:
    - internal/repository/postgres/postgres.go
    - internal/repository/postgres/migrations/001_create_tables.sql
    - internal/repository/postgres/postgres_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Schema must be applied via a separate pgx.Connect before pool creation — AfterConnect fires when pool acquires first connection, so tables must exist before statements are prepared"
  - "LoadWindow uses DESC LIMIT query + Go in-place reverse: single efficient DB roundtrip, oldest-first output at application layer"
  - "ListConversations initializes result as []domain.Conversation{} not nil — matches interface contract from Phase 01 in-memory implementation"

patterns-established:
  - "Schema-before-pool: apply DDL via direct connection before creating pgxpool, then register prepared statements in AfterConnect"
  - "Manual row scanning over pgx.RowToStructByPos: Role field requires string→domain.Role cast, manual scan is explicit and safe"

requirements-completed: [IFACE-03, METR-04]

duration: 4min
completed: "2026-03-31"
---

# Phase 02 Plan 01: PostgresRepository Summary

**PostgresRepository implementing all 4 ChatRepository methods via pgx/v5 pool with prepared statements, embedded SQL schema, and 7 integration tests against real Postgres containers**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-31T22:59:58Z
- **Completed:** 2026-03-31T23:04:21Z
- **Tasks:** 1 (TDD: 2 commits — test + feat)
- **Files modified:** 5

## Accomplishments

- PostgresRepository with compile-time ChatRepository interface check (`var _ repository.ChatRepository = (*PostgresRepository)(nil)`)
- SQL schema with conversations and messages tables, 2 covering indexes (idx_conversations_list, idx_messages_window)
- pgxpool connection pool with 5 prepared statements registered via AfterConnect callback
- All 4 ChatRepository methods implemented with correct behavior (LoadWindow oldest-first, ListConversations empty-not-nil)
- 7 integration tests all passing against real postgres:16-alpine containers via testcontainers-go

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests + SQL schema** - `b81d8f4` (test)
2. **Task 1 (GREEN): PostgresRepository implementation** - `96de707` (feat)

_Note: TDD task split into test commit then implementation commit._

## Files Created/Modified

- `internal/repository/postgres/postgres.go` - PostgresRepository struct with New(), Close(), and 4 ChatRepository methods
- `internal/repository/postgres/migrations/001_create_tables.sql` - DDL for conversations and messages tables with 2 indexes
- `internal/repository/postgres/postgres_test.go` - 7 integration tests using testcontainers-go
- `go.mod` - Added pgx/v5 and testcontainers-go dependencies
- `go.sum` - Updated checksums

## Decisions Made

- **Schema-before-pool ordering:** AfterConnect fires when the pool acquires its first connection. If the pool is created before the schema is applied, AfterConnect tries to prepare statements against non-existent tables and fails. Fixed by applying DDL via a direct `pgx.Connect` call before `pgxpool.NewWithConfig`. This is the correct pattern for pgxpool with AfterConnect prepared statements.

- **Manual row scanning over pgx.RowToStructByPos:** The `domain.Role` field is a named `string` type (`type Role string`). `RowToStructByPos` does not handle this conversion automatically. Manual `rows.Scan` with explicit `string` → `domain.Role` cast is used instead — explicit, safe, and zero overhead.

- **LoadWindow reverse strategy:** Query orders by `created_at DESC LIMIT n` (uses the idx_messages_window index efficiently), then reverses the slice in-place in Go to produce oldest-first output. This is a single DB roundtrip with O(n) in-memory reverse, n ≤ window size.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Schema-before-pool ordering fix**
- **Found during:** Task 1 (GREEN — first test run)
- **Issue:** `pgxpool.NewWithConfig` acquires a connection immediately on creation, triggering `AfterConnect` which prepares statements against tables that don't exist yet. All 7 tests failed with `ERROR: relation "conversations" does not exist`
- **Fix:** Added a `pgx.Connect` / `pool.Exec(schema)` / `schemaConn.Close` sequence before `pgxpool.NewWithConfig`. AfterConnect now fires after tables exist.
- **Files modified:** `internal/repository/postgres/postgres.go`
- **Verification:** All 7 integration tests pass after fix
- **Committed in:** `96de707` (feat commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug, initialization ordering)
**Impact on plan:** Fix was necessary for correct operation. No scope creep. Implementation matches plan intent exactly.

## Issues Encountered

- pgxpool AfterConnect ordering: pool eagerly acquires a connection on creation, so AfterConnect fires before schema is applied. Resolved by separating schema application (direct connection) from pool creation. This is a well-known pgxpool pattern — plan could have specified it explicitly but it's a minor ordering detail.

## User Setup Required

None - no external service configuration required. Tests use testcontainers-go which manages Docker containers automatically.

## Next Phase Readiness

- PostgresRepository is complete and tested — ready for the benchmark runner (plan 02-02) to consume
- The `New(ctx, connString) (*PostgresRepository, error)` + `Close()` pattern is the adapter lifecycle interface all three backends (Postgres, DynamoDB, Turso) will follow
- No blockers for plans 02-02+

## Self-Check: PASSED

- FOUND: internal/repository/postgres/postgres.go
- FOUND: internal/repository/postgres/migrations/001_create_tables.sql
- FOUND: internal/repository/postgres/postgres_test.go
- FOUND commit: b81d8f4 (test — TDD RED)
- FOUND commit: 96de707 (feat — TDD GREEN)

---
*Phase: 02-runner-postgres-baseline*
*Completed: 2026-03-31*
