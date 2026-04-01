# Phase 4: Cost Model + Report - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 04-cost-model-report
**Areas discussed:** Cost projection model, Operational scorecard, Recommendation report, JSON output + metadata

---

## Cost Projection Model

### Pricing data source

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded defaults + override flags | Embed current AWS/Turso pricing as defaults. Add override flags. Simplest, reproducible, sufficient for POC. | ✓ |
| Config file with pricing tables | External JSON/YAML with per-service pricing tiers. More flexible but adds file management overhead. | |
| Live API pricing lookup | Query AWS Pricing API and Turso pricing at runtime. Most accurate but fragile. | |

**User's choice:** Hardcoded defaults + override flags
**Notes:** None

### Projected scale

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable via --scale flag | Default 100 users x 50 convos x 200 msgs/day. Override with --scale-users, --scale-convos, --scale-msgs-per-day. | ✓ |
| Fixed three-tier projection | Show costs at small/medium/large (50/500/5000 users) — always all three. | |
| Match benchmark profile | Project costs based on the --profile flag. Simpler but conflates benchmark sizing with production scale. | |

**User's choice:** Configurable via --scale flag
**Notes:** None

### Cost dimensions

| Option | Description | Selected |
|--------|-------------|----------|
| Compute + storage + I/O | DynamoDB: WCU/RCU + storage. RDS: instance + storage. Turso: plan + reads/writes. | ✓ |
| Compute + storage + I/O + data transfer | Same plus cross-AZ/internet data transfer. Often negligible for chat storage. | |
| Total monthly cost only | Single $/month number per backend. Hides which component dominates. | |

**User's choice:** Compute + storage + I/O
**Notes:** None

### Cost output placement

| Option | Description | Selected |
|--------|-------------|----------|
| Cost table after latency results | Append cost projection table below latency results. Scale assumptions in header. | ✓ |
| Separate --cost flag | Only run cost projections when --cost is passed. | |
| Integrated into per-backend section | Each backend's section includes both latency and cost together. | |

**User's choice:** Cost table after latency results
**Notes:** None

---

## Operational Scorecard

### Scoring method

| Option | Description | Selected |
|--------|-------------|----------|
| 1-5 numeric scale per dimension | Rate each backend 1-5 on 5 dimensions. Hardcoded from implementation experience. | ✓ |
| Narrative paragraphs per dimension | Prose descriptions. Richer but harder to compare at a glance. | |
| Traffic light (green/yellow/red) | Three-level rating. Simpler but loses nuance. | |

**User's choice:** 1-5 numeric scale per dimension
**Notes:** None

### Placement

| Option | Description | Selected |
|--------|-------------|----------|
| CLI table + report section | Compact table in CLI output + expanded narrative in written report. | ✓ |
| Report only | Scorecard only in written report. | |
| CLI only | Scorecard only in terminal. | |

**User's choice:** CLI table + report section
**Notes:** None

### Score source

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded assessments | Scores baked into code based on Phases 1-3 experience. Honest, fast. | ✓ |
| Semi-automated with code metrics | Derive some scores from code (LOC, error paths). More defensible but complex. | |

**User's choice:** Hardcoded assessments
**Notes:** None

---

## Recommendation Report

### Format

| Option | Description | Selected |
|--------|-------------|----------|
| Generated Markdown file | CLI writes REPORT.md after benchmark. Reviewable in GitHub. | ✓ |
| Stdout-only extended output | Everything prints to terminal. No file. | |
| Both Markdown + terminal summary | Full report to file AND condensed terminal summary. | |

**User's choice:** Generated Markdown file
**Notes:** None

### Framing

| Option | Description | Selected |
|--------|-------------|----------|
| Data-first with explicit recommendation | Present data neutrally, then state clear recommendation with rationale. | ✓ |
| Neutral comparison only | Present trade-offs without picking a winner. | |
| Scored ranking | Weighted composite score per backend. | |

**User's choice:** Data-first with explicit recommendation
**Notes:** None

### Turso framing

| Option | Description | Selected |
|--------|-------------|----------|
| Architectural explanation section | Dedicated section explaining edge-SQLite from single-region AWS over internet. | ✓ |
| Inline footnotes per table | Footnote on each Turso latency number. | |
| You decide | Claude picks best approach. | |

**User's choice:** Architectural explanation section
**Notes:** None

---

## JSON Output + Metadata

### JSON structure

| Option | Description | Selected |
|--------|-------------|----------|
| Flat results + metadata envelope | Top-level metadata, results array, cost_projections, scorecard as sibling keys. | ✓ |
| Nested by backend | Top-level keyed by backend name. | |
| You decide | Claude picks most ergonomic structure. | |

**User's choice:** Flat results + metadata envelope
**Notes:** None

### Build info capture

| Option | Description | Selected |
|--------|-------------|----------|
| Go build info at runtime | runtime/debug.ReadBuildInfo() for Go version and VCS revision. | ✓ |
| Build-time ldflags | Inject via -ldflags at build time. | |
| You decide | Claude picks best approach. | |

**User's choice:** Go build info at runtime
**Notes:** None

### JSON destination

| Option | Description | Selected |
|--------|-------------|----------|
| Stdout JSON, suppresses tables | --output json writes to stdout, no human-readable tables. Pipe-friendly. | ✓ |
| Write JSON file alongside report | --output json writes to file, tables still print. | |
| You decide | Claude picks most ergonomic approach. | |

**User's choice:** Stdout JSON, suppresses tables
**Notes:** None

---

## Claude's Discretion

- Exact JSON field names and nesting beyond top-level structure
- Report section ordering and Markdown formatting
- Scorecard narrative wording
- Cost calculation formulas and rounding
- Report filename default
- --report and --output json interaction

## Deferred Ideas

None — discussion stayed within phase scope
