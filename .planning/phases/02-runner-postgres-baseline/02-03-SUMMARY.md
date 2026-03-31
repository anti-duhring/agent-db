---
phase: 02-runner-postgres-baseline
plan: 03
subsystem: benchmark
tags: [benchmark, scenarios, golang.org/x/sync, errgroup, hdrhistogram]

# Dependency graph
requires:
  - phase: 02-runner-postgres-baseline/02-02
    provides: benchmark.Scenario interface, SeedResult, Runner engine
  - phase: 02-runner-postgres-baseline/02-01
    provides: PostgresRepository implementing ChatRepository
provides:
  - All five benchmark scenarios implementing benchmark.Scenario
  - WarmupSkipper optional interface for cold-start scenarios
  - Runner updated to respect WarmupSkipper before warmup loop
affects:
  - 03-dynamodb-turso-adapters
  - 04-cli-report

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Optional interface pattern (WarmupSkipper) for scenario-level runner control"
    - "errgroup.WithContext + SetLimit for bounded concurrent goroutine management"
    - "Compile-time interface checks via var _ benchmark.Scenario = (*T)(nil)"

key-files:
  created:
    - internal/benchmark/scenarios/append.go
    - internal/benchmark/scenarios/window.go
    - internal/benchmark/scenarios/list.go
    - internal/benchmark/scenarios/coldstart.go
    - internal/benchmark/scenarios/concurrent.go
  modified:
    - internal/benchmark/scenario.go
    - internal/benchmark/runner.go

key-decisions:
  - "WarmupSkipper implemented as optional interface (not a Scenario method) to keep the interface minimal and non-breaking for adapters that don't need it"
  - "ConcurrentScenario uses errgroup.SetLimit(concurrency) per plan spec — each Run call is one round of N concurrent writes, timing captured by runner's outer loop"
  - "golang.org/x/sync v0.19.0 was already present in go.mod; no new dependency required"

patterns-established:
  - "Scenario pattern: Setup stores IDs from SeedResult, Run calls exactly one repo method, Teardown is no-op"
  - "Compile-time interface check in every scenario file: var _ benchmark.Scenario = (*T)(nil)"

requirements-completed: [SCEN-01, SCEN-02, SCEN-03, SCEN-04, SCEN-05]

# Metrics
duration: 4min
completed: 2026-03-31
---

# Phase 02 Plan 03: Benchmark Scenarios Summary

**Five benchmark scenarios (AppendMessage, LoadSlidingWindow, ListConversations, ColdStartLoad, ConcurrentWrites) implementing the Scenario interface, with WarmupSkipper optional interface for cold-start latency measurement**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-31T23:38:00Z
- **Completed:** 2026-03-31T23:42:41Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Implemented all five scenarios satisfying benchmark.Scenario interface with compile-time checks
- Added WarmupSkipper optional interface so ColdStartLoad skips warmup without changing the core interface
- Updated Runner to check for WarmupSkipper before executing warmup iterations
- ConcurrentWrites uses errgroup.WithContext + SetLimit for bounded parallel AppendMessage calls

## Task Commits

Each task was committed atomically:

1. **Task 1: AppendMessage, LoadSlidingWindow, ListConversations** - `da795b6` (feat)
2. **Task 2: ColdStartLoad, ConcurrentWrites + WarmupSkipper** - `21128c1` (feat)

**Plan metadata:** (docs commit — added after summary)

## Files Created/Modified
- `internal/benchmark/scenarios/append.go` - AppendScenario (SCEN-01): single-message write latency
- `internal/benchmark/scenarios/window.go` - WindowScenario (SCEN-02): LoadWindow(n=20) with 200+ message preference
- `internal/benchmark/scenarios/list.go` - ListScenario (SCEN-03): ListConversations for seeded partner/user
- `internal/benchmark/scenarios/coldstart.go` - ColdStartScenario (SCEN-04): WarmupSkipper + LoadWindow(n=20)
- `internal/benchmark/scenarios/concurrent.go` - ConcurrentScenario (SCEN-05): errgroup N-goroutine AppendMessage
- `internal/benchmark/scenario.go` - Added WarmupSkipper optional interface
- `internal/benchmark/runner.go` - Runner checks WarmupSkipper before warmup loop

## Decisions Made
- WarmupSkipper implemented as optional interface pattern (not embedded in Scenario) — keeps Scenario minimal and non-breaking for future adapters
- ConcurrentScenario's Run spawns exactly `concurrency` goroutines per call; the runner's outer timing loop captures total-round latency for the histogram
- `golang.org/x/sync` was already present in go.mod (v0.19.0); no `go get` needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Worktree was behind main branch (02-02 commits not merged). Resolved by merging main before implementation.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All five benchmark scenarios are ready for use with the Runner and any ChatRepository backend
- Phase 03 can plug in DynamoDB and Turso repositories and run the same scenarios immediately
- No stubs — all scenarios are fully wired to their target repo methods
