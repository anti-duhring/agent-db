---
phase: 04-cost-model-report
plan: "02"
subsystem: cli
tags: [go, cli, report, cost-model, json, markdown, benchmark]

# Dependency graph
requires:
  - phase: 04-01
    provides: internal/report package with PrintJSON, PrintCostTable, PrintScorecardTable, GenerateMarkdown, WriteReport, CollectMetadata, ComputeProjections

provides:
  - CLI flags for output format (--output), report path (--report), scale (--scale-users, --scale-convos, --scale-msgs-per-day), and pricing (--rds-instance-type, --dynamodb-mode)
  - Result accumulation across all backends before any output
  - JSON output mode via --output json (report.PrintJSON to stdout)
  - Markdown report file generation via --report flag (report.GenerateMarkdown + WriteReport)
  - Human-readable cost and scorecard tables appended after latency results

affects: [final benchmark output, written recommendation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Backend run functions return (BackendMeta, []ScenarioResult, error) — no printing inside helpers"
    - "Results accumulated in []report.BackendResults before any output dispatch"
    - "--output and --report are orthogonal flags (both can be set simultaneously)"

key-files:
  created: []
  modified:
    - main.go

key-decisions:
  - "runPostgres/runDynamoDB/runTurso return results instead of printing — enables post-loop report dispatch"
  - "dynamoDBMode flag suppressed with _ (only on-demand supported, defaults are correct)"
  - "JSON output replaces all human-readable tables when --output json is set (D-13)"
  - "--report is orthogonal to --output: Markdown report can be written alongside JSON or table output"

patterns-established:
  - "Accumulate-then-report: collect all results first, then dispatch output — avoids interleaved output in multi-backend mode"

requirements-completed: [OUT-01, OUT-02, OUT-03, OUT-04, OUT-05, OUT-06]

# Metrics
duration: 1min
completed: 2026-03-31
---

# Phase 04 Plan 02: CLI Report Wiring Summary

**report package wired into main.go: --output json/table, --report REPORT.md, scale and pricing flags, result accumulation across all backends**

## Performance

- **Duration:** 1 min
- **Started:** 2026-03-31T17:48:29Z
- **Completed:** 2026-03-31T17:49:39Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Changed runPostgres/runDynamoDB/runTurso to return (BackendMeta, []ScenarioResult, error) instead of printing results directly
- Added 7 new CLI flags: --output, --report, --scale-users, --scale-convos, --scale-msgs-per-day, --rds-instance-type, --dynamodb-mode
- Results from all backends are accumulated in []report.BackendResults before any output is generated
- JSON output (--output json) dispatches to report.PrintJSON, suppressing human-readable tables
- Human-readable mode prints per-backend latency tables then report.PrintCostTable + report.PrintScorecardTable
- Markdown report (--report path) writes via report.GenerateMarkdown + report.WriteReport, orthogonal to --output

## Task Commits

Each task was committed atomically:

1. **Task 1: Add CLI flags and result accumulation to main.go** - `16bfb8c` (feat)

**Plan metadata:** see final commit

## Files Created/Modified

- `/home/anti-duhring/alt/pocs/agent-db/main.go` - CLI flag wiring, result accumulation, report output dispatch

## Decisions Made

- runPostgres/runDynamoDB/runTurso return results instead of printing — accumulate-then-report pattern enables clean post-loop output dispatch
- dynamoDBMode flag accepted but suppressed with `_ =` since only "on-demand" is supported; defaults are already correct
- --output json and --report are orthogonal per D-08 and D-13: both can be set at once

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 04 complete: cost model, report package, and CLI wiring are all done
- The benchmark now produces JSON output, human-readable tables with cost projections, and Markdown recommendation reports
- All OUT-* requirements satisfied
- Ready for final project review and recommendation document generation

---
*Phase: 04-cost-model-report*
*Completed: 2026-03-31*
