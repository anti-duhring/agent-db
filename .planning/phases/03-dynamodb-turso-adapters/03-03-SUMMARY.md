---
phase: 03-dynamodb-turso-adapters
plan: "03"
subsystem: database
tags: [dynamodb, turso, localstack, libsql, cli, multi-backend, benchmark]

# Dependency graph
requires:
  - phase: 03-dynamodb-turso-adapters/03-01
    provides: DynamoDB ChatRepository adapter with New(ctx, endpoint) and Close()
  - phase: 03-dynamodb-turso-adapters/03-02
    provides: Turso ChatRepository adapter with New(ctx, url, authToken) and Close()
provides:
  - Multi-backend CLI dispatch via --backend flag (postgres, dynamodb, turso, all, comma-separated)
  - BackendMeta struct in results.go with Name, Transport, Note fields
  - Per-backend helper functions (runPostgres, runDynamoDB, runTurso) for clean isolation
  - Side-by-side benchmark output with per-backend transport headers
  - --backend all mode with graceful Turso skip when env vars missing
  - Dry-run support for all three backends
affects: [04-cost-model-report]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Multi-backend loop: --backend flag parsed into []string, each backend executed via dedicated helper function"
    - "BackendMeta struct replaces bare string in PrintResults for structured transport metadata"
    - "Env-var-gated skip: Turso skipped with warning in --backend all mode when TURSO_URL/TURSO_AUTH_TOKEN missing"

key-files:
  created: []
  modified:
    - main.go
    - internal/benchmark/results.go

key-decisions:
  - "BackendMeta struct added to results.go with Name, Transport, Note fields — PrintResults signature now takes BackendMeta instead of bare string per D-15"
  - "Turso skipped with warning (not error) in --backend all mode when env vars missing per D-14"
  - "Per-backend helper functions (runPostgres/runDynamoDB/runTurso) extract lifecycle per backend for readability"
  - "LocalStack pinned to 3.8 (not :latest) continued from Plan 01 — 2026.x requires paid auth token"

patterns-established:
  - "Backend dispatch: parse --backend into []string, iterate, call runXxx(ctx, ...) helper per backend"
  - "BackendMeta propagation: each runXxx creates its own BackendMeta and passes to PrintResults"

requirements-completed: [IFACE-03]

# Metrics
duration: ~20min
completed: 2026-03-31
---

# Phase 03 Plan 03: CLI Multi-Backend Wiring Summary

**Multi-backend CLI wiring that lets `--backend all` run Postgres, DynamoDB, and Turso sequentially with side-by-side results headers showing per-backend transport metadata**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-03-31T22:00:00Z
- **Completed:** 2026-03-31T22:16:25Z
- **Tasks:** 2 (1 auto + 1 human-verify)
- **Files modified:** 2

## Accomplishments

- Extended `--backend` flag to accept `postgres`, `dynamodb`, `turso`, `all`, and comma-separated combinations (e.g., `postgres,dynamodb`)
- Added `BackendMeta` struct to `internal/benchmark/results.go` and updated `PrintResults` signature to show transport and optional note per backend
- Wired DynamoDB (via LocalStack 3.8) and Turso (via env-var-gated cloud connection) as first-class benchmark targets alongside Postgres
- `--backend all` gracefully skips Turso with a warning when `TURSO_URL`/`TURSO_AUTH_TOKEN` are not set (per D-14)
- Dry-run support implemented for all three backends

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire multi-backend CLI dispatch and BackendMeta output** - `f1c7cec` (feat)
2. **Task 2: Verify multi-backend benchmark execution** - Human-approved checkpoint (no code commit)

## Files Created/Modified

- `main.go` — Extended CLI with multi-backend dispatch, runPostgres/runDynamoDB/runTurso helpers, Turso skip logic, dry-run per backend (+345/-65 lines)
- `internal/benchmark/results.go` — Added BackendMeta struct, updated PrintResults to accept BackendMeta instead of bare string

## Decisions Made

- BackendMeta struct carries `Name`, `Transport`, and `Note` fields; `PrintResults` signature changed from `(backend string, ...)` to `(meta BackendMeta, ...)` for structured output per D-15
- Per-backend helper functions (`runPostgres`, `runDynamoDB`, `runTurso`) extract container lifecycle and repo creation — main() becomes a loop over []string backends
- Turso emits a warning and continues in `--backend all` mode when env vars are absent; explicit `--backend turso` with missing env vars is a hard error per D-14

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

For Turso backend (`--backend turso` or `--backend all` with Turso included):
- `TURSO_URL` — libsql:// URL for your Turso database
- `TURSO_AUTH_TOKEN` — Turso auth token

Without these, `--backend all` skips Turso with a warning. `--backend turso` exits with an error.

## Next Phase Readiness

- Three-way comparison harness is complete: `go run . --backend all --scenario all --profile medium` runs all three backends and produces side-by-side output
- Phase 4 (Cost Model + Report) can consume benchmark output; `--output json` flag work is scoped to Phase 4
- No blockers for Phase 4

## Self-Check: PASSED

- SUMMARY.md: FOUND at `.planning/phases/03-dynamodb-turso-adapters/03-03-SUMMARY.md`
- Task 1 commit f1c7cec: FOUND in git log

---
*Phase: 03-dynamodb-turso-adapters*
*Completed: 2026-03-31*
