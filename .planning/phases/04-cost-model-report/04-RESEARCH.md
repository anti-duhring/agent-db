# Phase 4: Cost Model + Report - Research

**Researched:** 2026-03-31
**Domain:** Go report generation, cost modeling, JSON output, CLI flag extension
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Pricing data hardcoded as defaults with override flags. Embed current AWS/Turso pricing. Add `--rds-instance-type`, `--dynamodb-mode` (on-demand/provisioned), `--scale-factor` flags to override. Reproducible and sufficient for POC.
- **D-02:** Projected scale configurable via `--scale-users`, `--scale-convos`, `--scale-msgs-per-day` flags. Default: 100 users x 50 conversations x 200 messages/day.
- **D-03:** Cost dimensions: compute + storage + I/O per backend. DynamoDB: WCU/RCU on-demand costs + storage GB. RDS: instance type monthly + storage GB. Turso: plan tier + row reads/writes.
- **D-04:** Cost projection table appended after latency results in CLI output. Scale assumptions printed in the header.
- **D-05:** 1-5 numeric scale per dimension for each backend. Dimensions: SDK ergonomics, connection management, error handling, schema migration, local dev story.
- **D-06:** Scores hardcoded based on implementation experience from Phases 1-3. E.g., Postgres 5/5 on schema migration (standard SQL), DynamoDB 2/5 (no DDL, manual table design).
- **D-07:** Scorecard appears both as a compact CLI table (after cost table) and as an expanded narrative section in the written report.
- **D-08:** Generated Markdown file written by CLI after benchmark completes. Default path or `--report path`. Contains latency tables, cost projections, scorecard, and recommendation narrative. Reviewable in GitHub.
- **D-09:** Data-first framing with explicit recommendation. Present all data neutrally, then state a clear recommendation with rationale: "Based on latency, cost, and operational fit, we recommend X because..."
- **D-10:** Dedicated architectural explanation section for Turso latency. Explains edge-SQLite called from single-region AWS over internet — latency penalty is expected and architectural, not a product flaw.
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

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OUT-01 | Per-backend per-scenario results table (human-readable) | `tabwriter` pattern already established in `PrintResults()`; phase extends it for cost/scorecard |
| OUT-02 | JSON output mode (`--output json`) for machine-readable results | `encoding/json` (stdlib); `--output` flag added via `flag` package; D-11/D-13 define structure |
| OUT-03 | Run metadata in output: timestamps, Git SHA, Go version, backend configs | `runtime/debug.ReadBuildInfo()` provides `GoVersion` + `vcs.revision` from `Settings`; `time.Now()` for timestamp |
| OUT-04 | Cost projection model: DynamoDB RCU/WCU at projected scale, RDS instance cost, Turso pricing | Hardcoded defaults from current pricing; formulas documented in this research |
| OUT-05 | Operational complexity scorecard: SDK ergonomics, connection management, error handling, schema migration, local dev story | Hardcoded 1-5 scores per D-06; tabwriter table + Markdown narrative |
| OUT-06 | Written comparison report with final recommendation | `os.WriteFile()` writing a Markdown string; `--report` flag controls path |
</phase_requirements>

## Summary

Phase 4 is a pure output and reporting layer built entirely on Go stdlib. No new external dependencies are introduced. The three output artifacts — human-readable terminal tables, a JSON envelope, and a Markdown report file — all rely on packages already present in the project: `encoding/json`, `text/tabwriter`, `os`, `time`, and `runtime/debug`.

The cost model is a deterministic calculation over hardcoded pricing defaults. All three backends require different cost dimensions: DynamoDB uses per-request pricing (WRU/RRU per million), RDS uses hourly instance cost × 730 hours/month plus storage, and Turso uses a tiered plan model. Translating benchmark scenarios to projected-scale request counts is the core formula work, and the defaults from CONTEXT.md decisions (100 users × 50 convos × 200 msgs/day) drive those calculations.

The operational complexity scorecard is hardcoded data, not computed — the phase's implementation experience from Phases 1–3 informs the 1–5 scores per dimension. The recommendation Markdown report assembles all three artifact sections into a structured document using plain string formatting and `os.WriteFile()`.

**Primary recommendation:** Create `internal/report/` as a single package containing cost model, scorecard, JSON marshaling, Markdown generation, and tabwriter output — then wire the new `--output` and `--report` flags in `main.go` after all backend results are collected.

## Standard Stack

### Core (all stdlib — no new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | Go 1.26 stdlib | JSON output envelope (D-11) | No dependency needed; CLAUDE.md §Report Output locks this |
| `text/tabwriter` | Go 1.26 stdlib | Human-readable cost and scorecard tables | Already used in `PrintResults()`; CLAUDE.md §Report Output locks this |
| `os` | Go 1.26 stdlib | Write Markdown report file to disk | Standard file I/O |
| `time` | Go 1.26 stdlib | Run timestamp in metadata | Standard |
| `runtime/debug` | Go 1.26 stdlib | Capture Go version and VCS revision for metadata | ReadBuildInfo() provides both; no linker flags needed |
| `flag` | Go 1.26 stdlib | New CLI flags: --output, --report, --scale-*, --rds-instance-type, --dynamodb-mode | CLAUDE.md §CLI locks stdlib flag |
| `fmt` + `strings` | Go 1.26 stdlib | Markdown string assembly | No template engine needed for a single report format |

**No new `go get` commands required.** The go.mod file already contains every package needed for this phase.

## Architecture Patterns

### Recommended Project Structure

```
internal/
└── report/
    ├── cost.go         # CostConfig, CostProjection, ComputeProjections()
    ├── scorecard.go    # ScorecardEntry, Scorecard constant (hardcoded scores)
    ├── metadata.go     # RunMetadata, CollectMetadata() — calls runtime/debug
    ├── json.go         # BenchmarkReport struct, MarshalJSON output
    ├── table.go        # PrintCostTable(), PrintScorecardTable() using tabwriter
    └── markdown.go     # GenerateMarkdown() — assembles full report string
main.go                 # Add --output, --report, --scale-* flags; call report package after all backends complete
```

This mirrors the existing `internal/benchmark/` package boundary. All report concerns stay inside `internal/report/`, keeping `main.go` as a thin orchestrator.

### Pattern 1: Collecting All Backend Results Before Reporting

The current `main.go` runs each backend in a for-loop and calls `PrintResults()` immediately per backend. Phase 4 must accumulate results from all backends before generating cost projections and the combined report.

**What:** Collect `[]ScenarioResult` per backend into a slice of `BackendResults`, then pass the full slice to `report.Generate*()` after the loop exits.

**Example:**
```go
// Source: main.go integration pattern
type BackendResults struct {
    Meta    benchmark.BackendMeta
    Results []benchmark.ScenarioResult
}

var allResults []BackendResults
// ... inside the for-loop over selectedBackends:
allResults = append(allResults, BackendResults{Meta: meta, Results: results})

// After the loop:
if *output == "json" {
    report.PrintJSON(os.Stdout, allResults, meta, scaleConfig, *seed, *iters, *profile)
} else {
    // Print existing tables + cost + scorecard
    report.PrintCostTable(os.Stdout, scaleConfig)
    report.PrintScorecardTable(os.Stdout)
}
if *reportPath != "" {
    md := report.GenerateMarkdown(allResults, scaleConfig, *seed, *iters, *profile)
    os.WriteFile(*reportPath, []byte(md), 0644)
}
```

### Pattern 2: ReadBuildInfo for Run Metadata (D-12)

`runtime/debug.ReadBuildInfo()` returns a `*BuildInfo` struct with `GoVersion` and a `Settings []BuildSetting` slice. The VCS revision is in `Settings` under key `"vcs.revision"`. This works with standard `go build` — no `-ldflags` injection required.

```go
// Source: https://pkg.go.dev/runtime/debug
import "runtime/debug"

func collectBuildMeta() (goVersion, gitSHA string) {
    info, ok := debug.ReadBuildInfo()
    if !ok {
        return "unknown", "unknown"
    }
    goVersion = info.GoVersion
    for _, s := range info.Settings {
        if s.Key == "vcs.revision" {
            gitSHA = s.Value
        }
    }
    if gitSHA == "" {
        gitSHA = "unknown" // dev builds without VCS info
    }
    return
}
```

**Confidence:** HIGH — verified against pkg.go.dev docs. VCS keys: `vcs.revision` (commit hash), `vcs.time` (RFC3339 commit time), `vcs.modified` (bool, true if working tree dirty).

### Pattern 3: JSON Output Envelope (D-11)

The flat envelope structure per D-11:

```go
// Source: CONTEXT.md D-11 + encoding/json stdlib
type BenchmarkReport struct {
    Metadata        RunMetadata              `json:"metadata"`
    Results         []BackendResultJSON      `json:"results"`
    CostProjections []BackendCostProjection  `json:"cost_projections"`
    Scorecard       []ScorecardEntry         `json:"scorecard"`
}

type RunMetadata struct {
    Timestamp      string            `json:"timestamp"`       // time.Now().UTC().Format(time.RFC3339)
    GitSHA         string            `json:"git_sha"`
    GoVersion      string            `json:"go_version"`
    BackendConfigs []BackendConfig   `json:"backend_configs"`
    Seed           int64             `json:"seed"`
    Profile        string            `json:"profile"`
    Iterations     int               `json:"iterations"`
}
```

`json.NewEncoder(os.Stdout).Encode(report)` produces a newline-terminated JSON object suitable for piping to `jq`.

### Pattern 4: Cost Projection Formula

**DynamoDB on-demand cost calculation:**

Scale inputs: `users`, `convosPerUser`, `msgsPerDay`
- Daily messages written = `users × convosPerUser × msgsPerDay` (each message is ~1 WRU for a small message body, or 2-4 WRU for a large message — use average message size / 1KB, ceil)
- Daily messages read (LoadWindow, 20 msgs × estimated queries) = read operations × 4KB / 4KB per RRU = ~1 RRU per 4KB
- Monthly WRU cost = `(daily_writes × 30) / 1_000_000 × $0.25`
- Monthly RRU cost = `(daily_reads × 30) / 1_000_000 × $0.125`
- Monthly storage cost = `estimated_total_GB × $0.25`

**RDS cost calculation:**
- Instance cost = `hours_per_month × hourly_rate` (730 hours/month standard)
- Storage cost = `estimated_storage_GB × $0.115` (gp3, us-east-1)

**Turso cost calculation:**
- Determine which plan tier covers the projected row reads/writes
- Report plan name, monthly cost, and whether projected usage fits within plan limits or triggers overage

### Pattern 5: Markdown Report Generation

Use `fmt.Fprintf` into a `strings.Builder` — no template engine needed.

```go
func GenerateMarkdown(allResults []BackendResults, scale ScaleConfig, ...) string {
    var b strings.Builder
    fmt.Fprintf(&b, "# Agent DB Benchmark Report\n\n")
    fmt.Fprintf(&b, "**Generated:** %s\n", time.Now().UTC().Format(time.RFC3339))
    // ... sections
    return b.String()
}
```

### Anti-Patterns to Avoid

- **Printing per-backend as you go in JSON mode:** JSON output must be a single well-formed object, not a stream of per-backend objects. Collect all results first.
- **Calling `os.Exit(1)` before writing the report file:** If report generation is wired after the benchmark loop, make sure error handling doesn't `os.Exit` before `os.WriteFile` runs.
- **Using `text/template` for the Markdown report:** The report has a fixed structure; a `strings.Builder` with `fmt.Fprintf` is simpler and avoids template parsing complexity for a single output format.
- **Embedding pricing in `main.go`:** Cost constants belong in `internal/report/cost.go` to keep `main.go` thin.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON serialization | Custom JSON string builder | `encoding/json` | Handles escaping, quoting, struct tags correctly |
| Aligned terminal tables | `fmt.Sprintf` with manual padding | `text/tabwriter` | Already used; handles variable-width columns cleanly |
| Build version capture | `-ldflags` injection or `init()` git exec | `runtime/debug.ReadBuildInfo()` | Zero config, no build script changes required |
| File writing | Custom buffered writer | `os.WriteFile()` | Single call, handles create/truncate/close atomically |

**Key insight:** This phase is pure assembly of already-known data into output formats. Every formatting concern has a stdlib solution. No custom serialization, no template engine, no third-party report library.

## Common Pitfalls

### Pitfall 1: ReadBuildInfo Returns "unknown" in Dev Builds
**What goes wrong:** `debug.ReadBuildInfo()` returns `ok=false` or empty `vcs.revision` when the binary is built with `go run .` outside a git repo, or when the working tree has no git history.
**Why it happens:** VCS stamping requires `go build` (not `go run`) and a git repo with at least one commit. The `-buildvcs=false` flag also suppresses it.
**How to avoid:** Always check `ok` from `ReadBuildInfo()` and check for empty `vcs.revision` in Settings. Fall back to `"unknown"` gracefully. Do not treat missing VCS info as an error.
**Warning signs:** `git_sha: "unknown"` in output during development is expected and acceptable.

### Pitfall 2: --output json and --report Both Writing to stdout
**What goes wrong:** If `--report` defaults write to stdout and `--output json` also writes to stdout, the JSON becomes malformed.
**Why it happens:** Not thinking through the flag interaction (marked as Claude's Discretion in D context).
**How to avoid:** Recommended resolution: `--report` always writes to a file path (required argument); `--output json` writes to stdout. They are orthogonal — both can be set simultaneously without conflict. When `--output json` is set, suppress all human-readable table output; the report file still gets written if `--report` is set.
**Warning signs:** Attempting to `jq` parse output that contains both JSON and Markdown.

### Pitfall 3: Cost Model Using Wrong Message-to-Request Mapping
**What goes wrong:** Conflating one message AppendMessage with one WRU, when the DynamoDB AppendMessage implementation uses a `TransactWriteItems` with 4 items (message + old listing delete + new listing + meta update) — each item in a transaction counts as 2 WRUs in on-demand mode.
**Why it happens:** The cost model looks at scenarios without accounting for how the adapter actually executes them.
**How to avoid:** The cost comment in the model should note "AppendMessage uses TransactWriteItems (4 items) = 8 WRUs per message append in on-demand mode." Use a `dynamoDBWRUPerAppend = 8` constant.
**Warning signs:** DynamoDB projected cost is suspiciously low compared to expected transactional write overhead.

### Pitfall 4: tabwriter Flushing
**What goes wrong:** `tabwriter.Writer` output is buffered — if `Flush()` is not called, the table is never written.
**Why it happens:** Easy to forget when copy-pasting `tabwriter` boilerplate.
**How to avoid:** Always `defer w.Flush()` immediately after `tabwriter.NewWriter(...)`. The existing `PrintResults()` does call `w.Flush()` — new table functions must do the same.

### Pitfall 5: Turso Row-Read Estimation Mismatch
**What goes wrong:** Turso counts row reads as individual SQLite rows scanned, not logical query operations. A `LoadWindow` fetching 20 rows = 20 row reads, but a table scan for `ListConversations` could read many more rows than returned.
**Why it happens:** Turso's billing unit is the scanned row, not the returned row.
**How to avoid:** Document the Turso row-read multiplier in cost model comments. Use a conservative multiplier for `ListConversations` (e.g., assume scans 2× the returned row count).

## Code Examples

### ReadBuildInfo for Metadata
```go
// Source: https://pkg.go.dev/runtime/debug#BuildInfo
import "runtime/debug"

type RunMetadata struct {
    Timestamp  string `json:"timestamp"`
    GitSHA     string `json:"git_sha"`
    GoVersion  string `json:"go_version"`
    Seed       int64  `json:"seed"`
    Profile    string `json:"profile"`
    Iterations int    `json:"iterations"`
}

func CollectMetadata(seed int64, profile string, iterations int) RunMetadata {
    goVer, gitSHA := "unknown", "unknown"
    if info, ok := debug.ReadBuildInfo(); ok {
        goVer = info.GoVersion
        for _, s := range info.Settings {
            if s.Key == "vcs.revision" {
                gitSHA = s.Value
            }
        }
    }
    return RunMetadata{
        Timestamp:  time.Now().UTC().Format(time.RFC3339),
        GitSHA:     gitSHA,
        GoVersion:  goVer,
        Seed:       seed,
        Profile:    profile,
        Iterations: iterations,
    }
}
```

### JSON Envelope Output (D-11, D-13)
```go
// Source: encoding/json stdlib + CONTEXT.md D-11
func PrintJSON(w io.Writer, report BenchmarkReport) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(report)
}
```

### tabwriter Scorecard Table
```go
// Source: text/tabwriter stdlib — mirrors existing PrintResults() pattern
func PrintScorecardTable(w io.Writer) {
    tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight)
    defer tw.Flush()
    fmt.Fprintln(tw, "DIMENSION\tPOSTGRES\tDYNAMODB\tTURSO\t")
    fmt.Fprintln(tw, "---------\t--------\t--------\t-----\t")
    for _, row := range hardcodedScorecard {
        fmt.Fprintf(tw, "%s\t%d/5\t%d/5\t%d/5\t\n",
            row.Dimension, row.Postgres, row.DynamoDB, row.Turso)
    }
}
```

### Writing the Markdown Report
```go
// Source: os + strings stdlib
func WriteReport(path string, content string) error {
    return os.WriteFile(path, []byte(content), 0644)
}
```

## Pricing Data (Hardcoded Defaults)

These are the values to embed in `internal/report/cost.go`. Confidence levels reflect source quality.

| Backend | Dimension | Default Value | Source | Confidence |
|---------|-----------|--------------|--------|------------|
| DynamoDB | Write Request Unit (on-demand) | $0.25 per million WRU | AWS docs via web search | MEDIUM — verify at aws.amazon.com/dynamodb/pricing/on-demand/ |
| DynamoDB | Read Request Unit (on-demand) | $0.125 per million RRU | AWS docs via web search | MEDIUM — verify at aws.amazon.com/dynamodb/pricing/on-demand/ |
| DynamoDB | Storage (Standard) | $0.25 per GB-month | AWS docs via web search | MEDIUM — verify at aws.amazon.com/dynamodb/pricing/ |
| DynamoDB | AppendMessage WRU multiplier | 8 WRU (TransactWriteItems 4 items × 2) | Phase 3 implementation (D-04) | HIGH |
| RDS (db.t4g.micro) | Instance hourly (on-demand, us-east-1) | $0.030/hr ($21.90/mo) | Web search (economize.cloud verified) | MEDIUM |
| RDS (db.t4g.small) | Instance hourly (on-demand, us-east-1) | $0.068/hr ($49.64/mo) | Web search (economize.cloud verified) | MEDIUM |
| RDS | Storage (gp3) | $0.115 per GB-month | AWS RDS pricing page | MEDIUM |
| Turso | Starter plan | $0/mo (free tier: 500M row reads, 10M row writes/mo) | turso.tech/pricing via web search | LOW — pricing tiers change frequently |
| Turso | Scaler plan | $29/mo (100B row reads, 100M row writes/mo) | turso.tech/pricing via web search | LOW — verify before finalizing |
| Turso | Overage row reads | $1.00 per billion | turso.tech/pricing via web search | LOW — verify before finalizing |
| Turso | Overage row writes | $1.00 per million | turso.tech/pricing via web search | LOW — verify before finalizing |

**IMPORTANT:** All pricing defaults should have a comment in `cost.go` noting the date verified and the URL. This makes it trivial to update when pricing changes.

## Operational Complexity Scorecard (Hardcoded Values)

These scores reflect Phase 1-3 implementation experience. They are inputs for the planner to encode as constants.

| Dimension | Postgres | DynamoDB | Turso | Rationale |
|-----------|----------|----------|-------|-----------|
| SDK ergonomics | 5 | 3 | 4 | pgx idiomatic Go; DynamoDB expression builder verbose; Turso is standard sql.DB |
| Connection management | 4 | 5 | 4 | pgxpool requires pre-schema setup; DynamoDB stateless SDK; Turso sql.DB standard |
| Error handling | 4 | 3 | 4 | Postgres error codes clear; DynamoDB has service errors + marshaling errors layered; Turso standard sql errors |
| Schema migration | 5 | 2 | 4 | Postgres standard SQL DDL; DynamoDB no schema migrations, only table-level; Turso SQL migrations work but SQLite constraints |
| Local dev story | 5 | 3 | 2 | Postgres testcontainers trivial; DynamoDB LocalStack adequate; Turso requires real internet (no local emulator) |

These values are Claude's judgment based on Phase 1-3 implementation. The planner should encode these as a `var hardcodedScorecard = []ScorecardRow{...}` constant block in `scorecard.go`. They are not computed — the implementation team can adjust before shipping the report.

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| `-ldflags "-X main.Version=..."` build injection | `runtime/debug.ReadBuildInfo()` | No build script changes; works with `go build` and `go install` |
| `ioutil.WriteFile()` | `os.WriteFile()` | `ioutil` deprecated since Go 1.16; use `os.WriteFile()` directly |
| `json.MarshalIndent` + `os.Stdout.Write` | `json.NewEncoder(w).Encode()` | Encoder streams to writer; no intermediate byte slice allocation |

**Deprecated/outdated:**
- `io/ioutil`: Deprecated in Go 1.16, all functions moved to `os` and `io`. Do not use `ioutil.WriteFile` — use `os.WriteFile`.

## Environment Availability

Step 2.6: SKIPPED — Phase 4 is a pure code/output change. All required runtime environments (Go 1.26, go.mod dependencies) are already in place from Phases 1-3. No new external dependencies are introduced.

## Open Questions

1. **--report flag interaction with --output json**
   - What we know: D-13 says `--output json` writes to stdout and suppresses human-readable tables. D-08 says `--report path` writes a Markdown file.
   - What's unclear: Whether `--report` generates a report even when `--output json` is active.
   - Recommendation: Treat them as orthogonal. `--output json` controls stdout format; `--report` controls whether a Markdown file is written to disk. Both can be set simultaneously without conflict. This is the simplest, most composable design and is within Claude's Discretion per CONTEXT.md.

2. **Default report filename**
   - What we know: D-08 says "default path or `--report path`."
   - What's unclear: The exact default filename.
   - Recommendation: `REPORT.md` in the current working directory. Short, obvious, renders in GitHub. Within Claude's Discretion per CONTEXT.md.

3. **Turso pricing accuracy**
   - What we know: Pricing found via web search, but Turso has changed their plans multiple times (the search results reference at least 3 different plan structures).
   - What's unclear: Whether the "Starter" free tier or "Scaler $29" are the current plan names and limits as of March 2026.
   - Recommendation: Verify directly at turso.tech/pricing before hardcoding. Add a `// Verified: YYYY-MM-DD` comment in cost.go. Plan for the values to be wrong; the override flag (`--scale-factor`) provides an escape hatch.

4. **DynamoDB on-demand pricing post-November 2024 reduction**
   - What we know: AWS reduced DynamoDB on-demand pricing in November 2024. Web search shows $0.25/million WRU and $0.125/million RRU — different from an older figure of $0.78/million WRU that appeared in one source.
   - What's unclear: Which figure is current for us-east-1.
   - Recommendation: Use $0.25/million WRU and $0.125/million RRU as the defaults (these appear in multiple 2025 sources and align with the "November 2024 reduction" timeline). Add the verification URL as a comment.

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/runtime/debug` — BuildInfo struct, Settings keys (vcs.revision, vcs.time, vcs.modified), GoVersion field
- Go stdlib docs — `encoding/json`, `text/tabwriter`, `os.WriteFile`
- Project CONTEXT.md (04-CONTEXT.md) — all decisions locked by user

### Secondary (MEDIUM confidence)
- [Amazon DynamoDB pricing (on-demand)](https://aws.amazon.com/dynamodb/pricing/on-demand/) — WRU/RRU pricing and storage
- [economize.cloud — db.t4g.micro](https://www.economize.cloud/resources/aws/pricing/rds/db.t4g.micro/) — $21.90/mo RDS t4g.micro
- [economize.cloud — db.t4g.small](https://www.economize.cloud/resources/aws/pricing/rds/db.t4g.small/) — $51.10/mo RDS t4g.small
- [Amazon RDS for PostgreSQL Pricing](https://aws.amazon.com/rds/postgresql/pricing/) — gp3 storage $0.115/GB-month
- [Dynobase — DynamoDB pricing](https://dynobase.dev/dynamodb-pricing-calculator/) — confirmed $0.25/million WRU post-Nov 2024
- [cloudburn.io — DynamoDB pricing guide 2025](https://cloudburn.io/blog/amazon-dynamodb-pricing) — storage $0.25/GB-month confirmed

### Tertiary (LOW confidence — verify before hardcoding)
- [Turso pricing page](https://turso.tech/pricing) — plan tiers, overage rates; pricing has changed multiple times; verify current structure before encoding
- [Turso docs — usage and billing](https://docs.turso.tech/help/usage-and-billing) — billing unit definitions (row reads = rows scanned)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib; no new dependencies to research
- Architecture: HIGH — patterns derived from existing codebase (`PrintResults`, `BackendMeta`, `RunConfig`) plus stdlib idioms
- Pricing defaults: MEDIUM — web-searched values for AWS; LOW for Turso (verify before hardcoding)
- Pitfalls: HIGH — based on actual implementation patterns in Phases 1-3 and known Go stdlib behavior

**Research date:** 2026-03-31
**Valid until:** Pricing defaults should be re-verified if > 30 days old before final report generation. Code patterns valid indefinitely (stdlib).
