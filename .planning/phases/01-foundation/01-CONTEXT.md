# Phase 1: Foundation - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

The harness skeleton compiles with a stable ChatRepository interface, domain types (Conversation, Message), a deterministic seed-based data generator at three size profiles (small/medium/large), and a fully working in-memory stub implementation. No database adapters, no benchmark runner, no CLI.

</domain>

<decisions>
## Implementation Decisions

### Interface contract design
- **D-01:** Every ChatRepository method takes `context.Context` as first argument and returns `(result, error)`. Standard Go error handling, no custom error types.
- **D-02:** `LoadWindow` returns `[]Message` slice (not paginated result struct). For N=20 messages this is trivially small.
- **D-03:** `ListConversations` returns all conversations for a (partner_id, user_id) as a slice, sorted by last activity. No pagination — per-user conversation counts are small.
- **D-04:** In-memory adapter is a full working implementation (maps/slices), not a compile-only stub. Serves as reference implementation and enables Phase 2 benchmark runner testing without DB dependencies.

### Domain type details
- **D-05:** All IDs (conversation_id, message_id, partner_id, user_id) use `uuid.UUID` from `github.com/google/uuid`. Typed UUIDs with validation at the type level.
- **D-06:** `token_count` on Message is `int` (standard Go int, 64-bit on modern platforms).
- **D-07:** `Message.role` uses a typed string constant: `type Role string` with `const RoleUser Role = "user"` and `const RoleAssistant Role = "assistant"`.
- **D-08:** Timestamps (`created_at`, `updated_at`) use `time.Time`. All three DB drivers handle time.Time natively.

### Data generator design
- **D-09:** Single global seed controls all generation. Same seed = identical output across invocations. CLI will pass `--seed N`.
- **D-10:** Message content uses gofakeit `Sentence()` / `Paragraph()` for fake chat-like sentences. Variable length 100-2000 chars per DATA-03.
- **D-11:** Fixed conversation counts per profile: Small (5 conversations x 10 msgs), Medium (10 x 500 msgs), Large (10 x 5000 msgs).
- **D-12:** `token_count` estimated from content length as `len(content) / 4` (rough chars-to-tokens ratio). No real tokenizer needed.

### Project structure
- **D-13:** Go module path: `github.com/anti-duhring/agent-db`
- **D-14:** Layout: `internal/` packages — `internal/domain/` (types), `internal/repository/` (interface), `internal/repository/memory/` (in-memory impl), `internal/generator/` (data gen). `main.go` in root.
- **D-15:** Database adapters in later phases live under `internal/repository/{backend}/` — postgres/, dynamodb/, turso/ as siblings.
- **D-16:** Tests use same-package `_test.go` files next to source code. Standard Go convention.

### Claude's Discretion
- Exact method signatures beyond the contract decisions above (parameter ordering, naming)
- Internal data structures for the in-memory implementation (map keys, mutex strategy)
- Generator helper functions and internal organization
- Error message wording

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — IFACE-01, IFACE-02, DATA-01, DATA-02, DATA-03 define the acceptance criteria for this phase

### Stack decisions
- `CLAUDE.md` §Technology Stack — Locked driver versions, library choices, and alternatives considered. Particularly: pgx/v5, aws-sdk-go-v2, libsql-client-go, HdrHistogram, gofakeit/v7

### Project context
- `.planning/PROJECT.md` — Core value, constraints, data model (partner_id/user_id scoping), and key decisions
- `.planning/ROADMAP.md` §Phase 1 — Success criteria and dependency chain

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project, no existing code

### Established Patterns
- None yet — this phase establishes the foundational patterns

### Integration Points
- `ChatRepository` interface in `internal/repository/` will be the integration point for all Phase 2-3 adapters
- Domain types in `internal/domain/` will be imported by every package
- Generator in `internal/generator/` will be used by the benchmark runner in Phase 2

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

*Phase: 01-foundation*
*Context gathered: 2026-03-31*
