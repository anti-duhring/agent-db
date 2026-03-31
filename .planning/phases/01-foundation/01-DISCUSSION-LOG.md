# Phase 1: Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-31
**Phase:** 01-foundation
**Areas discussed:** Interface contract design, Domain type details, Data generator design, Project structure

---

## Interface contract design

### Context and error handling

| Option | Description | Selected |
|--------|-------------|----------|
| context.Context + error returns | Every method takes ctx as first arg, returns (result, error). Standard Go pattern. | ✓ |
| context.Context + custom error types | Same but with ChatRepoError codes (ErrNotFound, ErrConflict). | |
| Minimal — errors only, no context | Simpler signatures but not idiomatic for DB-backed code. | |

**User's choice:** context.Context + error returns
**Notes:** Standard Go pattern, allows timeout/cancellation per-call.

### LoadWindow return type

| Option | Description | Selected |
|--------|-------------|----------|
| []Message slice | Simple, idiomatic. For N=20 trivially small. | ✓ |
| Paginated result struct | WindowResult{Messages, HasMore, NextCursor}. More flexible but complex. | |
| You decide | Claude picks. | |

**User's choice:** []Message slice

### ListConversations pagination

| Option | Description | Selected |
|--------|-------------|----------|
| Return all | Full slice sorted by last activity. Per-user counts small. | ✓ |
| Cursor-based pagination | More production-realistic but marginal benchmark value. | |
| You decide | Claude picks. | |

**User's choice:** Return all

### In-memory stub scope

| Option | Description | Selected |
|--------|-------------|----------|
| Full working implementation | Maps/slices, passes all contracts. Reference impl for Phase 2. | ✓ |
| Compile-only stub | Returns nil/empty. Can't be used for real tests. | |
| You decide | Claude picks. | |

**User's choice:** Full working implementation

---

## Domain type details

### ID types

| Option | Description | Selected |
|--------|-------------|----------|
| string UUIDs | Simple, portable, no import needed. | |
| uuid.UUID (google/uuid) | Typed UUID values with validation at type level. | ✓ |
| You decide | Claude picks. | |

**User's choice:** uuid.UUID (google/uuid)
**Notes:** User preferred type safety over simplicity.

### token_count type

| Option | Description | Selected |
|--------|-------------|----------|
| int | Standard Go int (64-bit). Simple and idiomatic. | ✓ |
| int32 | Explicit 32-bit. Sufficient but unusual in Go APIs. | |
| You decide | Claude picks. | |

**User's choice:** int

### Message.role type

| Option | Description | Selected |
|--------|-------------|----------|
| Typed const (string enum) | type Role string with const RoleUser, RoleAssistant. | ✓ |
| Plain string | Just string field. No compile-time safety. | |
| You decide | Claude picks. | |

**User's choice:** Typed const (string enum)

### Timestamp type

| Option | Description | Selected |
|--------|-------------|----------|
| time.Time | Standard Go time. All drivers handle natively. | ✓ |
| int64 Unix millis | Raw timestamps. Less idiomatic. | |
| You decide | Claude picks. | |

**User's choice:** time.Time

---

## Data generator design

### Seed system

| Option | Description | Selected |
|--------|-------------|----------|
| Single global seed | One seed controls all generation. Fully deterministic. | ✓ |
| Per-profile seeds | Each profile gets derived sub-seed. More complex. | |
| You decide | Claude picks. | |

**User's choice:** Single global seed

### Content style

| Option | Description | Selected |
|--------|-------------|----------|
| Fake chat-like sentences | gofakeit Sentence()/Paragraph(). Variable 100-2000 chars. | ✓ |
| Lorem ipsum | Classic placeholder. Predictable but unrealistic. | |
| Template-based | Message templates with slots. More realistic but more code. | |

**User's choice:** Fake chat-like sentences

### Conversation counts

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed per profile | Small: 5x10, Medium: 10x500, Large: 10x5000. | ✓ |
| Configurable count | Caller specifies count. More flexible. | |
| You decide | Claude picks. | |

**User's choice:** Fixed per profile

### token_count generation

| Option | Description | Selected |
|--------|-------------|----------|
| Estimate from content length | len(content)/4 approximation. Cheap, realistic-looking. | ✓ |
| Random realistic values | Random int in 25-500 range. No content correlation. | |
| Leave zero | Always 0. Simplest but field never exercised. | |

**User's choice:** Estimate from content length

---

## Project structure

### Module layout

| Option | Description | Selected |
|--------|-------------|----------|
| internal/ packages | domain/, repository/, repository/memory/, generator/. main.go in root. | ✓ |
| Flat internal/ with files | Single internal/ package with separate .go files. | |
| cmd/ + internal/ | cmd/agent-db/main.go + internal/. Overkill for single binary. | |

**User's choice:** internal/ packages

### Adapter location (future phases)

| Option | Description | Selected |
|--------|-------------|----------|
| internal/repository/{backend}/ | Sibling packages under repository/. Interface in parent. | ✓ |
| Top-level adapter packages | internal/postgres/, internal/dynamodb/, etc. | |
| You decide | Claude picks. | |

**User's choice:** internal/repository/{backend}/

### Test location

| Option | Description | Selected |
|--------|-------------|----------|
| Same package _test.go files | Tests next to code. Standard Go convention. | ✓ |
| Separate test package | Black-box testing with _test suffix. Stricter. | |
| You decide | Claude picks. | |

**User's choice:** Same package _test.go files

### Module path

| Option | Description | Selected |
|--------|-------------|----------|
| github.com/alternative-payments/agent-db | Team's GitHub org convention. | |
| github.com/anti-duhring/agent-db | Current repo owner. Personal POC. | ✓ |
| You decide | Claude picks. | |

**User's choice:** github.com/anti-duhring/agent-db

---

## Claude's Discretion

- Exact method signatures beyond contract decisions
- Internal data structures for in-memory implementation
- Generator helper functions and organization
- Error message wording

## Deferred Ideas

None — discussion stayed within phase scope
