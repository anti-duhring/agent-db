# Agent DB — Chat Storage Benchmark

## What This Is

A benchmark harness in Go that evaluates three database candidates (Postgres, DynamoDB, Turso) for storing LLM chat conversations. The harness runs identical scenarios against each backend, measures performance, and produces a comparison report with latency metrics and cost projections. The output is both a CLI benchmark tool and a written recommendation document for team review.

## Core Value

Produce data-backed evidence that the team can use to choose the right storage engine for user-scoped LLM chat conversations — not opinions, not guesses, measured numbers.

## Requirements

### Validated

- [x] Common ChatRepository interface defined with four methods — Validated in Phase 01: Foundation
- [x] In-memory reference implementation of ChatRepository — Validated in Phase 01: Foundation
- [x] Synthetic conversation data generator (small/medium/large profiles) — Validated in Phase 01: Foundation
- [x] Postgres ChatRepository implementation with pgx/v5 — Validated in Phase 02: Runner + Postgres Baseline
- [x] Benchmark scenarios (append, window, list, coldstart, concurrent) — Validated in Phase 02: Runner + Postgres Baseline
- [x] Latency measurement with HdrHistogram (p50/p95/p99) — Validated in Phase 02: Runner + Postgres Baseline
- [x] CLI that runs benchmarks and outputs structured results — Validated in Phase 02: Runner + Postgres Baseline
- [x] DynamoDB ChatRepository implementation with aws-sdk-go-v2 (single-table design) — Validated in Phase 03: DynamoDB + Turso Adapters
- [x] Turso ChatRepository implementation with libsql-client-go (database/sql) — Validated in Phase 03: DynamoDB + Turso Adapters
- [x] Multi-backend CLI dispatch (--backend all) with side-by-side output — Validated in Phase 03: DynamoDB + Turso Adapters

### Active

(None — all requirements validated)

### Recently Validated

- [x] Cost projection model per backend at projected scale — Validated in Phase 04: Cost Model + Report
- [x] Operational complexity assessment (code complexity, gotchas, connection management) — Validated in Phase 04: Cost Model + Report
- [x] Written comparison report with recommendation — Validated in Phase 04: Cost Model + Report

### Out of Scope

- Production-ready chat service — this is a benchmark harness, not a deployable service
- LLM integration or prompt building — we're testing storage, not the AI layer
- Authentication, authorization, or API layer — no HTTP/gRPC transport needed
- Redis as primary storage candidate — wrong data class for persistent conversations (could be a cache layer later)
- Real conversation data — synthetic data only for the POC
- Edge/global distribution testing — single-region AWS is the target deployment

## Context

- The team is building an LLM chat feature for their product. Conversations are persistent threads scoped per user_id within a partner_id
- Users have one active conversation and can access/resume older ones
- The LLM context will use a sliding window approach (last N messages), not full conversation history
- Existing services (e.g., svc-accounts-payable) use Go with DDD/CQRS, Ent ORM, Postgres on RDS, gRPC, deployed on k8s
- The chat service will be a long-running service (not Lambda), which eliminates Postgres connection pooling concerns
- Scale expectation: dozens of users initially, scaling to hundreds. This POC will help gather volume/usage metrics
- DynamoDB and Turso are included to demonstrate alternatives were evaluated, even though Postgres is the presumptive winner given existing infra and scale
- Turso is expected to show worse latency (edge-SQLite called from single-region AWS over internet) — this is a valid data point

## Constraints

- **Language**: Go — matches the team's existing stack and expertise
- **Cloud**: AWS — all infra runs on AWS; Postgres is RDS, DynamoDB is native
- **Turso**: External dependency outside AWS VPC — latency penalty expected and acceptable for evaluation purposes
- **Data model**: Conversations scoped by (partner_id, user_id) with time-ordered messages per conversation
- **Benchmark fairness**: Same interface, same data, same scenarios against all three backends

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Postgres as baseline | Team already operates RDS, knows Ent ORM, no new infra needed | -- Pending |
| DynamoDB as alternative | Native AWS, purpose-built for key-value append patterns, textbook chat use case | -- Pending |
| Turso as alternative | Demonstrates edge-SQLite was evaluated; expected to underperform in single-region | -- Pending |
| Sliding window over full context | Full history loading is expensive (tokens + cost), persistent threads grow unbounded | -- Pending |
| Benchmark harness over mini-service | Isolates storage evaluation from HTTP/framework noise, faster to build, reproducible | -- Pending |
| Long-running service (not Lambda) | Matches existing deployment model, eliminates Postgres connection pooling pain | -- Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-01 after Phase 04 completion — all milestone phases complete*
