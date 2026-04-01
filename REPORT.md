# Agent DB Benchmark Report

**Generated:** 2026-04-01T03:36:00Z

| Field | Value |
|-------|-------|
| Go Version | go1.26.0 |
| Git SHA | unknown |
| Seed | 42 |
| Profile | medium |
| Iterations | 100 |

## Latency Results

### Backend: postgres

**Transport:** pgx/v5 (local container)

| Scenario | P50 | P95 | P99 | Count |
|----------|-----|-----|-----|-------|
| AppendMessage | 277us | 559us | 794us | 100 |
| LoadSlidingWindow | 149us | 342us | 488us | 100 |
| ListConversations | 151us | 187us | 203us | 100 |
| ColdStartLoad | 157us | 252us | 510us | 100 |
| ConcurrentWrites | 1.92ms | 2.59ms | 2.81ms | 100 |

### Backend: dynamodb

**Transport:** aws-sdk-go-v2 (LocalStack)

| Scenario | P50 | P95 | P99 | Count |
|----------|-----|-----|-----|-------|
| AppendMessage | 14.95ms | 18.16ms | 20.22ms | 100 |
| LoadSlidingWindow | 10.60ms | 12.86ms | 14.23ms | 100 |
| ListConversations | 8.32ms | 9.75ms | 11.09ms | 100 |
| ColdStartLoad | 9.20ms | 10.77ms | 11.95ms | 100 |
| ConcurrentWrites | 111.49ms | 136.83ms | 151.94ms | 100 |

### Backend: turso

**Transport:** libsql:// (remote, internet)

**Note:** Latency includes internet round-trip to Turso Cloud

| Scenario | P50 | P95 | P99 | Count |
|----------|-----|-----|-----|-------|
| AppendMessage | 793.60ms | 1164.29ms | 1339.39ms | 100 |
| LoadSlidingWindow | 188.93ms | 210.56ms | 253.31ms | 100 |
| ListConversations | 187.52ms | 211.84ms | 245.12ms | 100 |
| ColdStartLoad | 169.22ms | 208.00ms | 226.69ms | 100 |
| ConcurrentWrites | 4829.18ms | 7348.22ms | 8208.38ms | 100 |

## Cost Projections

**Scale assumptions:** 100 users x 50 conversations/user x 200 messages/day

| Backend | Instance/Plan | Compute | Storage | I/O | Total/mo |
|---------|---------------|---------|---------|-----|----------|
| dynamodb | on-demand | $0.00 | $175.78 | $67.50 | $243.28 |
| postgres | db.t4g.micro | $21.90 | $80.86 | $0.00 | $102.76 |
| turso | scaler | $29.00 | $0.00 | $0.00 | $29.00 |

**dynamodb notes:** AppendMessage uses TransactWriteItems (4 items x 2 WRU = 8 WRU/msg). Reads estimated at 2x daily writes (LoadWindow + ListConversations). Storage estimated at 2KB/msg, 12 months accumulation.

**postgres notes:** RDS on-demand instance cost + gp3 storage. Storage estimated at 2KB/msg, 12 months accumulation. No per-request I/O billing with gp3.

**turso notes:** Turso bills per scanned row. Row writes estimated at 3/msg (INSERT + UPDATE listing + UPDATE meta). Row reads estimated at 20/query (LoadWindow 20-row scan + ListConversations 2x multiplier). Storage included in plan cost.

## Operational Complexity Scorecard

Scores are 1-5 where 1 = worst and 5 = best, based on Phase 1-3 implementation experience.

| Dimension | Postgres | DynamoDB | Turso |
|-----------|----------|----------|-------|
| SDK Ergonomics | 5/5 | 3/5 | 4/5 |
| Connection Management | 4/5 | 5/5 | 4/5 |
| Error Handling | 4/5 | 3/5 | 4/5 |
| Schema Migration | 5/5 | 2/5 | 4/5 |
| Local Dev Story | 5/5 | 3/5 | 2/5 |

### Dimension Details

**SDK Ergonomics** (Postgres: 5/5, DynamoDB: 3/5, Turso: 4/5)

pgx idiomatic Go; DynamoDB expression builder verbose; Turso is standard sql.DB

**Connection Management** (Postgres: 4/5, DynamoDB: 5/5, Turso: 4/5)

pgxpool requires pre-schema setup; DynamoDB stateless SDK; Turso sql.DB standard

**Error Handling** (Postgres: 4/5, DynamoDB: 3/5, Turso: 4/5)

Postgres error codes clear; DynamoDB has service+marshaling errors layered; Turso standard sql errors

**Schema Migration** (Postgres: 5/5, DynamoDB: 2/5, Turso: 4/5)

Postgres standard SQL DDL; DynamoDB no schema migrations; Turso SQL works but SQLite constraints

**Local Dev Story** (Postgres: 5/5, DynamoDB: 3/5, Turso: 2/5)

Postgres testcontainers trivial; DynamoDB LocalStack adequate; Turso requires real internet

## Turso Latency: Architectural Context

Turso is an edge-SQLite database. In this benchmark, it was called from a single AWS region over the public internet. The observed latency premium compared to Postgres (local container) and DynamoDB (LocalStack) reflects the network round-trip to Turso Cloud, not a product performance limitation. In a production edge deployment where Turso replicas are co-located with users, read latency would be significantly lower.

## Recommendation

Based on latency, cost, and operational fit, we recommend **Postgres** as the primary storage backend for user-scoped LLM chat conversations.

**Rationale:**

- **Latency:** Postgres delivers the lowest p99 latency for all benchmark scenarios. For a long-running service on AWS with RDS, connection pooling eliminates cold-start overhead.
- **Cost:** RDS db.t4g.micro provides predictable monthly billing with no per-request surprises. DynamoDB on-demand pricing scales linearly with writes, which can be unpredictable during traffic spikes.
- **Operations:** Postgres scores highest in the operational complexity scorecard (23/25 total). Standard SQL DDL, testcontainers for local dev, and the team's existing Ent ORM expertise reduce risk.
- **Fit:** The team already operates RDS for existing services (svc-accounts-payable). No new infrastructure, no new operational runbooks, and existing team expertise apply directly.

**When DynamoDB might make sense instead:**

- Scale exceeds hundreds of users to millions, where DynamoDB's auto-scaling eliminates instance sizing decisions.
- Zero-management preference: if the team wants to eliminate all database operational overhead, DynamoDB's serverless model removes patching, failover configuration, and connection management.
- The access pattern becomes pure key-value (no ad-hoc queries), which removes the expressiveness advantage of SQL.

**Turso** is not recommended for this use case. The architectural context section above explains why the observed latency penalty is expected and structural, not fixable by tuning.
