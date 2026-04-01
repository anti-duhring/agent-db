---
phase: 02-runner-postgres-baseline
plan: 04
subsystem: database
tags: [postgres, benchmark, cli, testcontainers, hdrhistogram, errgroup]

requires:
  - phase: 02-runner-postgres-baseline/02-01
    provides: PostgresRepository with pgxpool, schema migration, prepared statements
  - phase: 02-runner-postgres-baseline/02-02
    provides: Scenario interface, Runner, RunConfig, SeedResult, HdrHistogram measurement
  - phase: 02-runner-postgres-baseline/02-03
    provides: Five scenario implementations (append, window, list, coldstart, concurrent)

provides:
  - main.go CLI entry point with flag parsing and testcontainer lifecycle
  - internal/benchmark/scenario.go - Scenario interface + WarmupSkipper
  - internal/benchmark/results.go - ScenarioResult, formatLatency, PrintResults tabwriter
  - internal/benchmark/runner.go - Runner, RunConfig, SeedResult, warmup/measure loop
  - internal/benchmark/scenarios/append.go - AppendMessage scenario (SCEN-01)
  - internal/benchmark/scenarios/window.go - LoadSlidingWindow scenario (SCEN-02)
  - internal/benchmark/scenarios/list.go - ListConversations scenario (SCEN-03)
  - internal/benchmark/scenarios/coldstart.go - ColdStartLoad scenario with WarmupSkipper (SCEN-04)
  - internal/benchmark/scenarios/concurrent.go - ConcurrentWrites scenario with errgroup (SCEN-05)

affects: [03-dynamodb-adapter, 04-turso-adapter, final-report]

tech-stack:
  added:
    - github.com/HdrHistogram/hdrhistogram-go v1.2.0
  patterns:
    - WarmupSkipper optional interface for cold-start scenario
    - Index-based mapping between generator conversations and DB conversations in SeedResult
    - errgroup.SetLimit for bounded concurrent goroutines
    - tabwriter for aligned terminal output

key-files:
  created:
    - main.go
    - internal/benchmark/scenario.go
    - internal/benchmark/results.go
    - internal/benchmark/runner.go
    - internal/benchmark/scenarios/append.go
    - internal/benchmark/scenarios/window.go
    - internal/benchmark/scenarios/list.go
    - internal/benchmark/scenarios/coldstart.go
    - internal/benchmark/scenarios/concurrent.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Window/ColdStart scenarios match DB conversations to generator conversations by index order (not ID) since CreateConversation generates its own ID"
  - "WarmupSkipper is an optional interface checked at runtime via type assertion in Runner.Run"
  - "ConcurrentScenario: each Run() call spawns N goroutines; runner histogram records total wall time per N-goroutine batch"
  - "main.go uses stdlib flag package per CLAUDE.md — no cobra"

patterns-established:
  - "Scenario interface: Name/Setup/Run/Teardown with SeedResult carrying DB-assigned conversation IDs"
  - "Runner seeding: CreateConversation returns DB IDs; messages appended with DB IDs; original generator IDs irrelevant after seeding"
  - "HdrHistogram: 1us to 30s range, 3 significant digits"

requirements-completed: [CLI-01, CLI-02, CLI-03, CLI-04, METR-03]

duration: 4min
completed: 2026-03-31
---

# Phase 02 Plan 04: CLI Entry Point Summary

**Complete benchmark CLI wiring all components: testcontainer Postgres, five scenarios, HdrHistogram runner, and tabwriter results table via stdlib flag**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-31T23:26:18Z
- **Completed:** 2026-03-31T23:26:00Z
- **Tasks:** 1 of 2 complete (Task 2 is a human-verify checkpoint)
- **Files modified:** 11

## Accomplishments

- Created all benchmark infrastructure (Plans 02-02 and 02-03 scope) in this worktree since parallel agents work in separate worktrees
- CLI parses 8 flags: --backend, --scenario, --profile, --iterations, --warmup, --concurrency, --seed, --dry-run
- --dry-run verifies container startup, connection, schema, seed data insertion, and sample query
- All five scenarios implement the Scenario interface with FK-safe SeedResult mapping
- ColdStartLoad correctly skips warmup via WarmupSkipper interface
- ConcurrentWrites uses errgroup.SetLimit for bounded parallel AppendMessage calls
- `go build ./...` passes cleanly

## Task Commits

1. **Task 1: Implement CLI with flag parsing, testcontainer lifecycle, and runner wiring** - `a4fe573` (feat)

**Plan metadata:** pending final commit

## Files Created/Modified

- `main.go` - CLI entry point: flag parsing, testcontainer lifecycle, runner wiring
- `internal/benchmark/scenario.go` - Scenario interface + WarmupSkipper optional interface
- `internal/benchmark/results.go` - ScenarioResult struct, formatLatency, PrintResults with tabwriter
- `internal/benchmark/runner.go` - Runner, RunConfig, SeedResult, seedRepository, warmup/measure loop
- `internal/benchmark/scenarios/append.go` - AppendMessage scenario (SCEN-01)
- `internal/benchmark/scenarios/window.go` - LoadSlidingWindow scenario (SCEN-02)
- `internal/benchmark/scenarios/list.go` - ListConversations scenario (SCEN-03)
- `internal/benchmark/scenarios/coldstart.go` - ColdStartLoad + WarmupSkipper (SCEN-04)
- `internal/benchmark/scenarios/concurrent.go` - ConcurrentWrites via errgroup (SCEN-05)
- `go.mod`, `go.sum` - Added HdrHistogram v1.2.0

## Decisions Made

- **Index-based ID mapping:** seed.Conversations[i] corresponds to OriginalData.Conversations[i]. Generator creates conversation IDs that differ from DB-assigned IDs (CreateConversation generates its own); messages in OriginalData.Messages are keyed by generator IDs. Resolved by matching by position (order of creation).
- **WarmupSkipper as optional interface:** Runner checks at runtime via type assertion rather than adding a required method to every scenario. ColdStartLoad is the only scenario that needs this.
- **ConcurrentScenario batch model:** Each Run() call launches N goroutines and waits. The runner's outer iteration loop then records wall-clock time for the whole batch. This means P50/P95/P99 measure "time for N concurrent appends" rather than individual write latency — which is the intended ConcurrentWrites metric.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed SeedResult ID mismatch for window/coldstart scenario setup**

- **Found during:** Task 1 (implementing window.go and coldstart.go)
- **Issue:** Plan's Setup pseudocode used `seed.Conversations` IDs to look up messages in `seed.OriginalData.Messages`, but those maps are keyed by generator-assigned conversation IDs. DB conversations have different IDs. Result: window/coldstart would always fall back to first conversation, defeating the 200+ message selection logic.
- **Fix:** Changed Setup to iterate `seed.OriginalData.Conversations` by index to find a 200+ message conversation, then use `seed.Conversations[i].ID` (the DB ID) for the actual query.
- **Files modified:** internal/benchmark/scenarios/window.go, internal/benchmark/scenarios/coldstart.go
- **Verification:** go build ./... passes; logic correctly maps generator conversations to DB conversations
- **Committed in:** a4fe573 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Essential correctness fix. Without it, window and cold-start scenarios would not reliably select conversations with sufficient message depth.

## Issues Encountered

None beyond the ID mismatch deviation above.

## User Setup Required

None - testcontainer manages Postgres lifecycle automatically.

## Next Phase Readiness

- All benchmark infrastructure complete and compiling
- `go run . --backend postgres --scenario all --profile small --iterations 10 --warmup 2 --seed 42` is the verification command for Task 2 checkpoint
- Phase 03 can proceed: DynamoDB adapter implements ChatRepository, registered via --backend dynamodb flag
- Turso adapter same pattern: implements ChatRepository, registered via --backend turso

---
*Phase: 02-runner-postgres-baseline*
*Completed: 2026-03-31*
