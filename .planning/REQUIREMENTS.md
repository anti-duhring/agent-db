# Requirements: Agent DB — Chat Storage Benchmark

**Defined:** 2026-03-31
**Core Value:** Produce data-backed evidence for choosing the right storage engine for user-scoped LLM chat conversations

## v1 Requirements

Requirements for the benchmark harness. Each maps to roadmap phases.

### Interface & Domain

- [x] **IFACE-01**: Common `ChatRepository` interface with methods: CreateConversation, AppendMessage, LoadWindow, ListConversations
- [x] **IFACE-02**: Domain types: Conversation (id, partner_id, user_id, created_at, updated_at), Message (id, conversation_id, role, content, token_count, created_at)
- [x] **IFACE-03**: Three implementations of ChatRepository: Postgres, DynamoDB, Turso

### Data Generation

- [x] **DATA-01**: Synthetic conversation data generator with deterministic seeded RNG
- [x] **DATA-02**: Three data size profiles: small (10 messages), medium (500 messages), large (5,000 messages)
- [x] **DATA-03**: Realistic message content with variable sizes (100-2000 chars) and role alternation (user/assistant)

### Benchmark Scenarios

- [ ] **SCEN-01**: AppendMessage — single message insert into existing conversation, measures write latency
- [ ] **SCEN-02**: LoadSlidingWindow — fetch last N=20 messages from conversation with 200+ messages, keyset pagination
- [ ] **SCEN-03**: ListConversations — list all conversations for a (partner_id, user_id) sorted by last activity
- [ ] **SCEN-04**: ColdStartLoad — first sliding window read after fresh connection (no warmup)
- [ ] **SCEN-05**: ConcurrentWrites — N goroutines (10, 50) appending messages in parallel

### Measurement & Metrics

- [x] **METR-01**: Latency percentiles per scenario per backend: p50, p95, p99
- [x] **METR-02**: Warmup phase (configurable iterations) excluded from measurement
- [ ] **METR-03**: Configurable iteration count via `--iterations N` flag
- [x] **METR-04**: Environment isolation — clean schema/data state per run

### Output & Reporting

- [ ] **OUT-01**: Per-backend per-scenario results table (human-readable)
- [ ] **OUT-02**: JSON output mode (`--output json`) for machine-readable results
- [ ] **OUT-03**: Run metadata in output: timestamps, Git SHA, Go version, backend configs
- [ ] **OUT-04**: Cost projection model: DynamoDB RCU/WCU at projected scale, RDS instance cost, Turso pricing
- [ ] **OUT-05**: Operational complexity scorecard: SDK ergonomics, connection management, error handling, schema migration, local dev story
- [ ] **OUT-06**: Written comparison report with final recommendation

### CLI

- [ ] **CLI-01**: `--dry-run` mode that verifies connectivity and schema setup without running benchmarks
- [ ] **CLI-02**: Backend selection flag (`--backend postgres,dynamodb,turso` or `--backend all`)
- [ ] **CLI-03**: Scenario selection flag (`--scenario append,window` or `--scenario all`)
- [ ] **CLI-04**: Data profile flag (`--profile small,medium,large`)

## v2 Requirements

Deferred to future work if POC validates the approach.

### Advanced Metrics

- **ADV-01**: Coefficient of variation reporting (flag scenarios with CV > 5%)
- **ADV-02**: Latency histogram output (ASCII or JSON bucket data)
- **ADV-03**: Concurrent write contention metric (% degradation vs single-writer baseline)

### CI Integration

- **CI-01**: Exit code based on latency thresholds (fail if p99 > Xms)
- **CI-02**: Result diff between runs (detect performance regressions)

## Out of Scope

| Feature | Reason |
|---------|--------|
| HTTP/gRPC transport layer | Adds network noise; benchmark calls ChatRepository directly |
| Authentication / authorization | Irrelevant to storage evaluation |
| Redis cache layer evaluation | Cache complements storage, not a replacement candidate |
| Real conversation data / PII | Compliance risk; synthetic data sufficient |
| Multi-region / edge testing | Single-region AWS is the deployment target |
| ORM benchmarking (Ent) | Testing storage engines, not ORM overhead |
| Automated cloud provisioning | Manual setup with documented steps sufficient for POC |
| Dashboard / monitoring UI | Over-engineering; JSON output + jq is enough |
| Streaming / SSE / WebSocket | Application concern, not storage concern |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| IFACE-01 | Phase 1 | Complete |
| IFACE-02 | Phase 1 | Complete |
| IFACE-03 | Phase 3 | Complete |
| DATA-01 | Phase 1 | Complete |
| DATA-02 | Phase 1 | Complete |
| DATA-03 | Phase 1 | Complete |
| SCEN-01 | Phase 2 | Pending |
| SCEN-02 | Phase 2 | Pending |
| SCEN-03 | Phase 2 | Pending |
| SCEN-04 | Phase 2 | Pending |
| SCEN-05 | Phase 2 | Pending |
| METR-01 | Phase 2 | Complete |
| METR-02 | Phase 2 | Complete |
| METR-03 | Phase 2 | Pending |
| METR-04 | Phase 2 | Complete |
| OUT-01 | Phase 4 | Pending |
| OUT-02 | Phase 4 | Pending |
| OUT-03 | Phase 4 | Pending |
| OUT-04 | Phase 4 | Pending |
| OUT-05 | Phase 4 | Pending |
| OUT-06 | Phase 4 | Pending |
| CLI-01 | Phase 2 | Pending |
| CLI-02 | Phase 2 | Pending |
| CLI-03 | Phase 2 | Pending |
| CLI-04 | Phase 2 | Pending |

**Coverage:**
- v1 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0

---
*Requirements defined: 2026-03-31*
*Last updated: 2026-03-31 after roadmap creation*
