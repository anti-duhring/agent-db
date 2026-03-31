# Roadmap: Agent DB — Chat Storage Benchmark

## Overview

Build a Go benchmark harness that produces data-backed evidence for choosing a database backend for LLM chat storage. The work has two hard gates — the ChatRepository interface must be stable before any adapter is written, and the DynamoDB schema must be designed before DynamoDB code begins — then fans out into three adapter implementations before converging on a cost model and final recommendation report.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - ChatRepository interface, domain types, data generator, and in-memory stub (completed 2026-03-31)
- [ ] **Phase 2: Runner + Postgres Baseline** - Benchmark runner, metrics collector, CLI, and first working end-to-end backend
- [ ] **Phase 3: DynamoDB + Turso Adapters** - Remaining two adapter implementations completing the three-way comparison
- [ ] **Phase 4: Cost Model + Report** - Post-processing cost projections, operational complexity scorecard, and final written recommendation

## Phase Details

### Phase 1: Foundation
**Goal**: The harness skeleton compiles with a stable interface, domain types, and deterministic seed data at three size profiles
**Depends on**: Nothing (first phase)
**Requirements**: IFACE-01, IFACE-02, DATA-01, DATA-02, DATA-03
**Success Criteria** (what must be TRUE):
  1. `ChatRepository` interface with CreateConversation, AppendMessage, LoadWindow, ListConversations compiles and is importable by adapter packages
  2. Conversation and Message domain types exist with all specified fields and are used exclusively (no database-specific types leak into the interface)
  3. Data generator produces deterministic output: identical seed produces identical conversation and message content across invocations
  4. All three size profiles (small/10 msgs, medium/500 msgs, large/5000 msgs) are generated with realistic variable-length content and correct role alternation
**Plans**: 3 plans
Plans:
- [x] 01-01-PLAN.md — Go module init, domain types, ChatRepository interface
- [x] 01-02-PLAN.md — In-memory ChatRepository adapter with tests
- [x] 01-03-PLAN.md — Seeded data generator with three profiles

### Phase 2: Runner + Postgres Baseline
**Goal**: All five benchmark scenarios produce valid p50/p95/p99 latency numbers against a real Postgres backend, and the CLI controls them
**Depends on**: Phase 1
**Requirements**: IFACE-03 (Postgres only), SCEN-01, SCEN-02, SCEN-03, SCEN-04, SCEN-05, METR-01, METR-02, METR-03, METR-04, CLI-01, CLI-02, CLI-03, CLI-04
**Success Criteria** (what must be TRUE):
  1. Running `go run . --backend postgres --scenario all --profile medium` produces a results table with p50/p95/p99 for each of the five scenarios
  2. Warmup iterations are excluded from measurement output (configurable via flag, verified by count)
  3. `--iterations N` flag changes the sample count reflected in results
  4. `--dry-run` verifies Postgres connectivity and schema setup without executing benchmark iterations
  5. `--backend`, `--scenario`, and `--profile` flags correctly scope which work executes
**Plans**: 4 plans
Plans:
- [x] 02-01-PLAN.md — Postgres ChatRepository adapter with pgx/v5, schema, and integration tests
- [ ] 02-02-PLAN.md — Benchmark runner engine (Scenario interface, Runner, Results, HdrHistogram)
- [ ] 02-03-PLAN.md — Five benchmark scenarios (append, window, list, coldstart, concurrent)
- [ ] 02-04-PLAN.md — CLI entry point wiring flags, testcontainer, and runner

### Phase 3: DynamoDB + Turso Adapters
**Goal**: All five scenarios run against DynamoDB and Turso, completing the three-way comparison with a correctly designed DynamoDB schema
**Depends on**: Phase 2
**Requirements**: IFACE-03 (DynamoDB and Turso implementations)
**Success Criteria** (what must be TRUE):
  1. Running `--backend dynamodb --scenario all` produces valid p50/p95/p99 for all five scenarios without any Scan operations (all reads use Query)
  2. Running `--backend turso --scenario all` produces valid p50/p95/p99 for all five scenarios with BackendMeta transport annotation in output
  3. Running `--backend all` executes all three backends against the same seed data and produces a side-by-side results table
  4. DynamoDB adapter uses ConsistentRead=true for all read scenarios and completes a warmup pass before timing begins
**Plans**: TBD

### Phase 4: Cost Model + Report
**Goal**: A complete comparison report exists with cost projections, operational complexity scores, and a written recommendation
**Depends on**: Phase 3
**Requirements**: OUT-01, OUT-02, OUT-03, OUT-04, OUT-05, OUT-06
**Success Criteria** (what must be TRUE):
  1. Results table renders per-backend per-scenario with p50/p95/p99 columns in human-readable form
  2. `--output json` produces machine-readable JSON including run metadata (timestamp, Git SHA, Go version, backend configs)
  3. Cost projection output shows DynamoDB RCU/WCU costs, RDS instance cost, and Turso pricing at a configurable projected scale
  4. Operational complexity scorecard covers SDK ergonomics, connection management, error handling, schema migration, and local dev story for each backend
  5. Written recommendation document exists with neutral framing and explicit architectural explanation for Turso latency results
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete   | 2026-03-31 |
| 2. Runner + Postgres Baseline | 1/4 | In Progress|  |
| 3. DynamoDB + Turso Adapters | 0/? | Not started | - |
| 4. Cost Model + Report | 0/? | Not started | - |
