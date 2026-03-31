# Phase 2: Runner + Postgres Baseline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 02-runner-postgres-baseline
**Areas discussed:** Postgres schema design, Runner architecture, CLI output format, Environment isolation

---

## Postgres Schema Design

### Table structure
| Option | Description | Selected |
|--------|-------------|----------|
| Two tables, normalized | conversations + messages tables, FK relationship. Standard relational approach. | ✓ |
| Single table, denormalized | Everything in one table with conversation metadata repeated per message. | |
| You decide | Claude picks the schema design during planning. | |

**User's choice:** Two tables, normalized
**Notes:** Maps 1:1 to domain model. Standard approach.

### Migration approach
| Option | Description | Selected |
|--------|-------------|----------|
| Embedded SQL files | SQL files in migrations/, loaded via Go embed. | ✓ |
| Inline Go strings | Schema DDL as const strings in Go file. | |
| You decide | Claude picks the migration approach. | |

**User's choice:** Embedded SQL files
**Notes:** Clear separation, easy to review.

### Prepared statements
| Option | Description | Selected |
|--------|-------------|----------|
| Prepared statements | Prepare queries once at adapter init, reuse during iterations. | ✓ |
| Inline queries each time | Pass SQL strings directly in each method call. | |
| You decide | Claude picks based on fairest benchmark numbers. | |

**User's choice:** Prepared statements
**Notes:** Better reflects real-world usage, gives Postgres fair shot.

### LoadWindow pagination
| Option | Description | Selected |
|--------|-------------|----------|
| ORDER BY created_at DESC LIMIT N | Simple reverse-chronological with LIMIT, reverse in Go. | ✓ |
| Subquery with row_number() | Window function approach. | |
| You decide | Claude picks the query approach. | |

**User's choice:** ORDER BY created_at DESC LIMIT N
**Notes:** Index-efficient, no cursor state needed.

---

## Runner Architecture

### Scenario organization
| Option | Description | Selected |
|--------|-------------|----------|
| Scenario interface + registry | Each scenario implements Scenario interface. Clean extension point. | ✓ |
| Flat functions in runner | All scenario logic in a single runner.go. | |
| You decide | Claude picks the runner architecture. | |

**User's choice:** Scenario interface + registry
**Notes:** Selected with preview of package layout.

### Warmup handling
| Option | Description | Selected |
|--------|-------------|----------|
| Separate warmup pass | N warmup iterations first (discarded), then M measured iterations. | ✓ |
| Interleaved with discard flag | All iterations in one loop, first N excluded from stats. | |
| You decide | Claude picks the warmup strategy. | |

**User's choice:** Separate warmup pass
**Notes:** Cleaner separation, easier to reason about.

### Concurrency management (SCEN-05)
| Option | Description | Selected |
|--------|-------------|----------|
| errgroup with configurable N | golang.org/x/sync/errgroup, shared HdrHistogram. | ✓ |
| Raw goroutines + WaitGroup | Manual goroutine management with sync.WaitGroup. | |
| You decide | Claude picks the concurrency approach. | |

**User's choice:** errgroup with configurable N
**Notes:** Better error handling, configurable via --concurrency flag.

---

## CLI Output Format

### Results table layout
| Option | Description | Selected |
|--------|-------------|----------|
| Grouped by scenario | One section per scenario with p50/p95/p99 columns. | ✓ |
| Matrix table | Single dense table with backends as columns. | |
| You decide | Claude picks the output format. | |

**User's choice:** Grouped by scenario
**Notes:** Natural reading order for comparing backends within a scenario.

### Dry-run output
| Option | Description | Selected |
|--------|-------------|----------|
| Checklist with pass/fail | Step-by-step verification output with [OK]/[FAIL]. | ✓ |
| Silent success, verbose failure | No output on success, only errors. | |
| You decide | Claude picks the dry-run output style. | |

**User's choice:** Checklist with pass/fail
**Notes:** More reassuring visual feedback.

### Latency units
| Option | Description | Selected |
|--------|-------------|----------|
| Adaptive units | Show us when < 1ms, ms otherwise. | ✓ |
| Always milliseconds | Consistent units, simpler formatting. | |
| You decide | Claude picks the unit formatting. | |

**User's choice:** Adaptive units
**Notes:** No precision loss for fast operations.

---

## Environment Isolation

### Local Postgres provisioning
| Option | Description | Selected |
|--------|-------------|----------|
| Testcontainers in tests only | Tests use testcontainers, benchmarks need external Postgres. | |
| Testcontainers for everything | Both tests and benchmark runs spin up containers. | ✓ |
| You decide | Claude picks the local infra approach. | |

**User's choice:** Testcontainers for everything
**Notes:** Fully self-contained, no external dependency needed.

### Clean state strategy
| Option | Description | Selected |
|--------|-------------|----------|
| Fresh container per run | New Postgres container per benchmark run. | ✓ |
| Truncate tables between runs | Reuse one container, TRUNCATE between runs. | |
| You decide | Claude picks the isolation strategy. | |

**User's choice:** Fresh container per run
**Notes:** Guarantees zero state leakage.

### Data seeding
| Option | Description | Selected |
|--------|-------------|----------|
| Shared seed data per run | Generated once, all scenarios use same data. | ✓ |
| Per-scenario seeding | Each scenario gets fresh tailored data. | |
| You decide | Claude picks the data seeding strategy. | |

**User's choice:** Shared seed data per run
**Notes:** Consistent baseline, matches real-world access patterns.

---

## Claude's Discretion

- Exact Scenario interface method signatures
- HdrHistogram configuration values
- Testcontainer configuration details
- Error message wording and exit codes
- Internal runner state management

## Deferred Ideas

None — discussion stayed within phase scope.
