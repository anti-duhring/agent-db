---
phase: 03-dynamodb-turso-adapters
plan: 02
subsystem: database
tags: [turso, libsql, sqlite, database-sql, chat-repository]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: ChatRepository interface, domain types (Conversation, Message, Role)
  - phase: 02-runner-postgres-baseline
    provides: Postgres adapter as structural reference pattern
provides:
  - TursoRepository implementing ChatRepository via database/sql + libsql driver
  - SQLite-compatible schema (conversations + messages tables with TEXT UUIDs/timestamps)
  - Integration tests that skip gracefully when TURSO_URL/TURSO_AUTH_TOKEN not set
affects: [03-03-cli-wiring, 04-report]

# Tech tracking
tech-stack:
  added:
    - github.com/tursodatabase/libsql-client-go v0.0.0-20251219100830-236aa1ff8acc
    - github.com/coder/websocket v1.8.12 (transitive)
    - github.com/antlr4-go/antlr/v4 v4.13.0 (transitive)
    - golang.org/x/exp (transitive)
  patterns:
    - database/sql with libsql driver registration via blank import
    - SQLite TEXT type for UUIDs and RFC3339Nano timestamps (parse on scan)
    - DESC ORDER + in-place reverse for oldest-first LoadWindow (mirrors Postgres pattern)
    - External test package (turso_test) with package-qualified references
    - Env-var skip guard in test helper for optional external services

key-files:
  created:
    - internal/repository/turso/turso.go
    - internal/repository/turso/turso_test.go
    - internal/repository/turso/migrations/001_create_tables.sql
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Turso uses database/sql with libsql driver (sql.Open(\"libsql\", ...)) per D-07, D-08"
  - "SQLite schema uses TEXT for all UUIDs and timestamps (RFC3339Nano string), INTEGER not INT, no TIMESTAMPTZ"
  - "Schema applied at constructor time with multi-statement fallback (split by semicolon) for libsql compatibility"
  - "Tests use unique UUIDs per test to avoid cross-test interference on shared Turso Cloud database"
  - "Close() returns error (unlike Postgres Close() which returns nothing) — consistent with database/sql interface"

patterns-established:
  - "Turso pattern: sql.Open(\"libsql\", url+\"?authToken=\"+token) for connection string construction"
  - "UUID storage: always .String() on write, uuid.Parse() on scan — no native UUID type in SQLite"
  - "Timestamp storage: time.RFC3339Nano on write, time.Parse(time.RFC3339Nano, ...) on scan"
  - "Empty result: initialize msgs via append (nil); return []domain.Message{} if nil after loop"

requirements-completed: [IFACE-03]

# Metrics
duration: 15min
completed: 2026-04-01
---

# Phase 03 Plan 02: Turso Adapter Summary

**Turso ChatRepository adapter using database/sql + libsql-client-go with SQLite-compatible schema and graceful env-var skip tests**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-04-01T00:53:00Z
- **Completed:** 2026-04-01T01:08:29Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- TursoRepository implementing all 4 ChatRepository methods via database/sql with libsql driver
- SQLite-compatible DDL (TEXT for UUIDs/timestamps, INTEGER, no TIMESTAMPTZ) with matching indexes
- 6 integration tests covering all methods and empty-result edge cases, skipping gracefully without env vars

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Turso ChatRepository adapter with SQLite schema** - `fc69b67` (feat)
2. **Task 2: Turso integration tests with env-var skip** - `5ac1d5f` (test)

## Files Created/Modified
- `internal/repository/turso/turso.go` - TursoRepository with all 4 ChatRepository methods, compile-time interface check, schema embed
- `internal/repository/turso/migrations/001_create_tables.sql` - SQLite DDL for conversations and messages tables with indexes
- `internal/repository/turso/turso_test.go` - 6 integration tests with env-var skip guard
- `go.mod` - Added libsql-client-go dependency
- `go.sum` - Updated checksums

## Decisions Made
- Used `sql.Open("libsql", url+"?authToken="+token)` for connection string construction per D-07
- Schema applied at constructor time with single-exec attempt plus per-statement fallback for libsql compatibility
- `Close()` returns `error` (vs Postgres `Close()` which returns nothing) — follows `database/sql` `*sql.DB.Close()` signature
- Tests use unique `uuid.New()` per test for partnerID/userID to avoid cross-test interference on shared cloud DB

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
**External service requires manual configuration for integration tests to run.**

To run Turso tests (they skip without these):
```
export TURSO_URL="libsql://<your-db>.turso.io"
export TURSO_AUTH_TOKEN="<your-token>"
go test ./internal/repository/turso/ -v -count=1 -timeout 60s
```

Create a Turso dev database: `turso db create agent-db-bench`

## Next Phase Readiness
- Turso adapter complete and compiles against ChatRepository interface
- Ready for CLI wiring (Plan 03) — `--backend turso` flag and BackendMeta annotation
- Turso latency tests will show expected internet round-trip penalty vs in-VPC Postgres/DynamoDB

---
*Phase: 03-dynamodb-turso-adapters*
*Completed: 2026-04-01*
