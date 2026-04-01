# Agent DB Benchmark — TL;DR

**Bottom line: Use Postgres.**

## Latency (p50, medium profile, 100 iterations)

| Scenario | Postgres | DynamoDB | Turso |
|----------|----------|----------|-------|
| AppendMessage | **277us** | 14.95ms | 793.60ms |
| LoadSlidingWindow | **149us** | 10.60ms | 188.93ms |
| ListConversations | **151us** | 8.32ms | 187.52ms |
| ColdStartLoad | **157us** | 9.20ms | 169.22ms |
| ConcurrentWrites | **1.92ms** | 111.49ms | 4829.18ms |

Postgres is **50-60x faster** than DynamoDB and **600-2500x faster** than Turso on every scenario.

## Monthly Cost (100 users, 50 convos/user, 200 msgs/day)

| Backend | Total/mo |
|---------|----------|
| Turso | $29.00 |
| **Postgres** | **$102.76** |
| DynamoDB | $243.28 |

Postgres is cheapest at this scale among production-grade options. Turso is cheaper but not viable (see latency).

## Operational Score (out of 25)

| Backend | Score | Highlights |
|---------|-------|------------|
| **Postgres** | **23/25** | Best SDK, best migrations, best local dev |
| Turso | 18/25 | Standard sql.DB, but no local emulator |
| DynamoDB | 16/25 | Verbose SDK, no schema migrations, LocalStack adequate |

## Why Postgres

1. **Already in the stack** — team runs RDS for svc-accounts-payable, knows Ent ORM
2. **Fastest by far** — sub-millisecond p50 on all read scenarios
3. **Predictable cost** — flat RDS instance billing, no per-request surprises
4. **Best DX** — testcontainers, SQL DDL, pgx is idiomatic Go

## When to reconsider DynamoDB

If scale goes from hundreds to millions of users and you want zero-ops auto-scaling.

## Why not Turso

Edge-SQLite called from a single AWS region over internet. The ~200ms read latency is architectural (network round-trip), not tunable. Not a fit for server-side workloads in AWS.

---

*Full report: [REPORT.md](REPORT.md) | Seed: 42 | Profile: medium | 2026-04-01*
