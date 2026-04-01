# Phase 4: Cost Model + Report - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

A complete comparison report exists with cost projections, operational complexity scores, and a written recommendation. This includes: per-backend per-scenario results tables (human-readable and JSON), run metadata, cost projection model at configurable scale, operational complexity scorecard, and a generated Markdown recommendation document. No new scenarios, no new backends, no changes to the benchmark runner or adapters.

</domain>

<decisions>
## Implementation Decisions

### Cost projection model
- **D-01:** Pricing data hardcoded as defaults with override flags. Embed current AWS/Turso pricing. Add `--rds-instance-type`, `--dynamodb-mode` (on-demand/provisioned), `--scale-factor` flags to override. Reproducible and sufficient for POC.
- **D-02:** Projected scale configurable via `--scale-users`, `--scale-convos`, `--scale-msgs-per-day` flags. Default: 100 users x 50 conversations x 200 messages/day.
- **D-03:** Cost dimensions: compute + storage + I/O per backend. DynamoDB: WCU/RCU on-demand costs + storage GB. RDS: instance type monthly + storage GB. Turso: plan tier + row reads/writes.
- **D-04:** Cost projection table appended after latency results in CLI output. Scale assumptions printed in the header.

### Operational complexity scorecard
- **D-05:** 1-5 numeric scale per dimension for each backend. Dimensions: SDK ergonomics, connection management, error handling, schema migration, local dev story.
- **D-06:** Scores hardcoded based on implementation experience from Phases 1-3. E.g., Postgres 5/5 on schema migration (standard SQL), DynamoDB 2/5 (no DDL, manual table design).
- **D-07:** Scorecard appears both as a compact CLI table (after cost table) and as an expanded narrative section in the written report.

### Recommendation report
- **D-08:** Generated Markdown file written by CLI after benchmark completes. Default path or `--report path`. Contains latency tables, cost projections, scorecard, and recommendation narrative. Reviewable in GitHub.
- **D-09:** Data-first framing with explicit recommendation. Present all data neutrally, then state a clear recommendation with rationale: "Based on latency, cost, and operational fit, we recommend X because..."
- **D-10:** Dedicated architectural explanation section for Turso latency. Explains edge-SQLite called from single-region AWS over internet — latency penalty is expected and architectural, not a product flaw.

### JSON output + metadata
- **D-11:** Flat results + metadata envelope. Top-level object with `metadata` (timestamp, git_sha, go_version, backend_configs, seed, profile, iterations), `results` array (per-backend, per-scenario latency), `cost_projections`, and `scorecard` as sibling top-level keys.
- **D-12:** Build info captured via `runtime/debug.ReadBuildInfo()` for Go version and VCS revision. Zero config, works with standard `go build`.
- **D-13:** `--output json` writes JSON to stdout, suppresses human-readable tables. Pipe-friendly: `go run . --output json | jq .results`.

### Claude's Discretion
- Exact JSON field names and nesting beyond the top-level structure
- Report section ordering and Markdown formatting details
- Scorecard narrative wording in the report
- Cost calculation formulas and rounding
- Report filename default (e.g., `REPORT.md` vs `benchmark-report.md`)
- How `--report` flag interacts with `--output json` (both? mutually exclusive?)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — OUT-01 through OUT-06 define acceptance criteria for this phase

### Stack decisions
- `CLAUDE.md` §Technology Stack — `encoding/json` (stdlib) for JSON output, `text/tabwriter` (stdlib) for terminal tables. No external dependencies needed for reporting.
- `CLAUDE.md` §CLI — stdlib `flag` for CLI flag parsing. No cobra.

### Project context
- `.planning/PROJECT.md` — Core value (data-backed evidence), constraints, Turso latency expectations, scale context (dozens to hundreds of users)
- `.planning/ROADMAP.md` §Phase 4 — Success criteria (5 items), dependency on Phase 3, requirements list

### Prior phase context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Domain types, project structure
- `.planning/phases/02-runner-postgres-baseline/02-CONTEXT.md` — Runner architecture, CLI output format (D-10 through D-12), testcontainers pattern
- `.planning/phases/03-dynamodb-turso-adapters/03-CONTEXT.md` — BackendMeta struct (D-15/D-16), side-by-side output, multi-backend CLI dispatch

### Existing code
- `internal/benchmark/results.go` — `ScenarioResult`, `BackendMeta`, `PrintResults()`, `formatLatency()` — extend for cost/scorecard output
- `main.go` — CLI entry point with flag parsing, backend dispatch — extend with new flags and report generation

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `PrintResults()` (`internal/benchmark/results.go`): Current output function using tabwriter. Cost and scorecard tables can follow the same tabwriter pattern.
- `BackendMeta` struct: Already carries name, transport, note per backend. Can be extended or used as-is for report metadata.
- `ScenarioResult` struct: Holds p50/p95/p99/count per scenario. JSON serialization and report generation consume these directly.
- `formatLatency()`: Adaptive µs/ms formatting. Reuse in report generation.

### Established Patterns
- stdlib `flag` for all CLI flags — new flags (--output, --report, --scale-*, --rds-instance-type, --dynamodb-mode) follow same pattern
- `tabwriter` for aligned terminal tables — scorecard and cost tables use same approach
- Per-backend helper functions in main.go (runPostgres, runDynamoDB, runTurso) — report generation aggregates results from all backends

### Integration Points
- `main.go` — Collects results from all backend runs, then passes to report/cost/json generation
- New package likely: `internal/report/` — report generation, cost model, scorecard, JSON output
- `go.mod` — No new external dependencies expected (stdlib json, tabwriter, runtime/debug)

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

*Phase: 04-cost-model-report*
*Context gathered: 2026-03-31*
