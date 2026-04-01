# Phase 3: DynamoDB + Turso Adapters - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 03-dynamodb-turso-adapters
**Areas discussed:** DynamoDB table design, Turso SDK choice, Local testing strategy, CLI integration

---

## DynamoDB table design

### Table structure

| Option | Description | Selected |
|--------|-------------|----------|
| Single-table design | One table, composite keys. PK/SK covers all access patterns. No GSI needed. Textbook DynamoDB pattern for chat. | ✓ |
| Two-table design | Separate conversations and messages tables mirroring Postgres schema. Simpler to reason about but less idiomatic. May need GSI. | |

**User's choice:** Single-table design
**Notes:** None

### AppendMessage consistency

| Option | Description | Selected |
|--------|-------------|----------|
| TransactWriteItems | Atomic transaction: insert message + delete old CONV SK + put new CONV SK. Guarantees consistency. ~2x WCU cost. | ✓ |
| Separate writes (eventual) | Non-atomic put + delete + put. Brief inconsistency window. Simpler, faster. | |
| GSI with updated_at attribute | Store updated_at as attribute, use GSI for sorting. Avoids delete+put dance but adds GSI cost. | |

**User's choice:** TransactWriteItems
**Notes:** None

---

## Turso SDK choice

### SDK selection

| Option | Description | Selected |
|--------|-------------|----------|
| libsql-client-go | Deprecated but functional. Pure Go, no CGO. database/sql interface. Remote-only. Matches CLAUDE.md recommendation. | ✓ |
| go-libsql | Active development. Requires CGO + precompiled C libs. Supports embedded replicas. More build complexity. | |

**User's choice:** libsql-client-go
**Notes:** None

### Query approach

| Option | Description | Selected |
|--------|-------------|----------|
| Raw database/sql | Direct sql.Query + manual rows.Scan into domain types. No extra dependencies. Standard Go pattern. | ✓ |
| sqlx for scanning | jmoiron/sqlx for struct scanning convenience. Adds dependency but reduces boilerplate. | |

**User's choice:** Raw database/sql
**Notes:** None

---

## Local testing strategy

### DynamoDB local testing

| Option | Description | Selected |
|--------|-------------|----------|
| LocalStack via testcontainers | Same testcontainers pattern as Postgres. Spins up LocalStack container. Consistent with Phase 2. | ✓ |
| DynamoDB Local (AWS Docker image) | Official AWS image. Valid but doesn't integrate with testcontainers-go as cleanly. | |

**User's choice:** LocalStack via testcontainers
**Notes:** None

### Turso testing

| Option | Description | Selected |
|--------|-------------|----------|
| Turso Cloud dev DB | Dedicated dev database on Turso Cloud. Env vars for connection. Real internet latency is the point. Skip if env vars not set. | ✓ |
| SQLite local fallback | Local SQLite for unit tests, Turso Cloud for benchmarks. Two-tier approach. Adds sqlite3 CGO dependency. | |

**User's choice:** Turso Cloud dev DB
**Notes:** None

---

## CLI integration

### Backend availability handling

| Option | Description | Selected |
|--------|-------------|----------|
| Skip unavailable, warn | --backend all runs whatever's available. Skip Turso with warning if env vars not set. Results show "skipped". | ✓ |
| Fail fast if incomplete | --backend all requires all three. Fail with error if any can't connect. | |
| Comma-separated only | Remove 'all' shortcut. Always specify explicitly. | |

**User's choice:** Skip unavailable, warn
**Notes:** None

### BackendMeta annotation

| Option | Description | Selected |
|--------|-------------|----------|
| Header note in results | Metadata line above Turso results: "Transport: libsql:// (remote, internet)". Makes latency context clear. | ✓ |
| Per-row annotation | Transport column in results table for all backends. Shows "local" or "remote". | |

**User's choice:** Header note in results
**Notes:** None

---

## Claude's Discretion

- DynamoDB attribute naming conventions
- Exact table provisioning settings
- Turso schema DDL details
- Error message wording
- Container configuration details
- Exact CLI output formatting

## Deferred Ideas

None — discussion stayed within phase scope
