# Phase 1: Foundation - Research

**Researched:** 2026-03-31
**Domain:** Go module initialization, interface design, domain types, seeded data generation
**Confidence:** HIGH

## Summary

Phase 1 is a pure Go code phase with no external service dependencies. It establishes the module skeleton, domain types, ChatRepository interface, an in-memory reference implementation, and a seeded data generator. All library choices are already locked in CLAUDE.md — this phase's research focuses on verifying APIs, confirming correct usage patterns, and documenting Go-specific pitfalls for the specific constructs being built.

The two libraries with meaningful API surface to verify are `github.com/google/uuid` (v1.6.0, current) and `github.com/brianvoe/gofakeit/v7` (v7.14.1, current). Both verified against the Go module proxy and pkg.go.dev. The Go version on the development machine is 1.25.4, while CLAUDE.md specifies 1.26 as the target — the go.mod directive should specify `go 1.26` per CLAUDE.md, which is valid since 1.25.4 can still compile and test code targeting 1.26 unless 1.26-specific language features are used.

The in-memory adapter (D-04) requires careful mutex strategy to be safe for the concurrent benchmark runner Phase 2 will build. A `sync.RWMutex` with read-locking for queries and write-locking for mutations is the correct pattern. Skipping this leads to data races in Phase 2 testing.

**Primary recommendation:** Initialize the Go module first (`go mod init github.com/anti-duhring/agent-db`), then define domain types, then the interface, then the in-memory adapter, then the generator — in dependency order. Run `go vet ./...` and `go build ./...` as the compile gate at the end.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Every ChatRepository method takes `context.Context` as first argument and returns `(result, error)`. Standard Go error handling, no custom error types.
- **D-02:** `LoadWindow` returns `[]Message` slice (not paginated result struct). For N=20 messages this is trivially small.
- **D-03:** `ListConversations` returns all conversations for a (partner_id, user_id) as a slice, sorted by last activity. No pagination — per-user conversation counts are small.
- **D-04:** In-memory adapter is a full working implementation (maps/slices), not a compile-only stub. Serves as reference implementation and enables Phase 2 benchmark runner testing without DB dependencies.
- **D-05:** All IDs use `uuid.UUID` from `github.com/google/uuid`. Typed UUIDs with validation at the type level.
- **D-06:** `token_count` on Message is `int` (standard Go int, 64-bit on modern platforms).
- **D-07:** `Message.role` uses a typed string constant: `type Role string` with `const RoleUser Role = "user"` and `const RoleAssistant Role = "assistant"`.
- **D-08:** Timestamps use `time.Time`. All three DB drivers handle time.Time natively.
- **D-09:** Single global seed controls all generation. Same seed = identical output across invocations.
- **D-10:** Message content uses gofakeit `Sentence()` / `Paragraph()` for fake chat-like sentences. Variable length 100-2000 chars per DATA-03.
- **D-11:** Fixed conversation counts per profile: Small (5 conversations x 10 msgs), Medium (10 x 500 msgs), Large (10 x 5000 msgs).
- **D-12:** `token_count` estimated from content length as `len(content) / 4`.
- **D-13:** Go module path: `github.com/anti-duhring/agent-db`
- **D-14:** Layout: `internal/domain/`, `internal/repository/`, `internal/repository/memory/`, `internal/generator/`. `main.go` in root.
- **D-15:** Database adapters in later phases: `internal/repository/{postgres,dynamodb,turso}/`
- **D-16:** Tests use same-package `_test.go` files next to source code.

### Claude's Discretion

- Exact method signatures beyond the contract decisions above (parameter ordering, naming)
- Internal data structures for the in-memory implementation (map keys, mutex strategy)
- Generator helper functions and internal organization
- Error message wording

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IFACE-01 | Common `ChatRepository` interface with methods: CreateConversation, AppendMessage, LoadWindow, ListConversations | Go interface design patterns; context.Context as first arg (D-01); verified uuid.UUID type |
| IFACE-02 | Domain types: Conversation (id, partner_id, user_id, created_at, updated_at), Message (id, conversation_id, role, content, token_count, created_at) | uuid.UUID v1.6.0 API verified; time.Time native support confirmed; Role typed string pattern (D-07) |
| DATA-01 | Synthetic conversation data generator with deterministic seeded RNG | gofakeit/v7 v7.14.1 `New(seed int64) *Faker` constructor verified; same seed produces identical output |
| DATA-02 | Three data size profiles: small (10 messages), medium (500 messages), large (5,000 messages) | Fixed counts from D-11: small=5x10, medium=10x500, large=10x5000 |
| DATA-03 | Realistic message content with variable sizes (100-2000 chars) and role alternation (user/assistant) | gofakeit `Sentence(wordCount int)` and `Paragraph(paragraphCount int)` verified; word count tuning to hit char range; Role alternation via index parity |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/google/uuid` | v1.6.0 | Typed UUID values for all IDs | Locked in D-05; standard Go UUID library; UUID [16]byte type is map-key-safe and comparable |
| `github.com/brianvoe/gofakeit/v7` | v7.14.1 | Seeded synthetic data generation | Locked in CLAUDE.md; 310+ generation functions; `New(seed)` produces reproducible sequences |

### Supporting (Phase 1 only)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sync` (stdlib) | Go stdlib | RWMutex for in-memory adapter | Any shared state accessed from multiple goroutines |
| `time` (stdlib) | Go stdlib | time.Time for all timestamps | All created_at, updated_at fields |
| `context` (stdlib) | Go stdlib | context.Context in all interface methods | Required by D-01 for every ChatRepository method |
| `sort` (stdlib) | Go stdlib | Sorting conversations by last activity in ListConversations | Required by D-03 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `uuid.UUID` ([16]byte) | `string` UUIDs | String is simpler but loses type safety — D-05 explicitly chose typed UUIDs |
| `gofakeit.New(seed)` | `math/rand` with custom generation | gofakeit provides realistic sentences with a single API call; hand-rolling loses time to no benefit |
| `sync.RWMutex` | `sync.Mutex` | RWMutex allows concurrent reads — important because Phase 2 benchmarks will read and write concurrently |

**Installation:**
```bash
go mod init github.com/anti-duhring/agent-db
go get github.com/google/uuid@v1.6.0
go get github.com/brianvoe/gofakeit/v7@v7.14.1
```

**Version verification:** Both versions verified against Go module proxy (proxy.golang.org) on 2026-03-31:
- `github.com/google/uuid` — v1.6.0 published 2024-01-23, confirmed current as of proxy listing
- `github.com/brianvoe/gofakeit/v7` — v7.14.1 published 2026-03-03, confirmed latest in proxy list

## Architecture Patterns

### Recommended Project Structure
```
github.com/anti-duhring/agent-db/
├── main.go                          # Entry point (minimal in Phase 1)
├── go.mod
├── go.sum
└── internal/
    ├── domain/
    │   └── types.go                 # Conversation, Message, Role types
    ├── repository/
    │   └── repository.go            # ChatRepository interface
    ├── repository/memory/
    │   └── memory.go                # In-memory implementation
    │   └── memory_test.go           # Reference implementation tests
    └── generator/
        └── generator.go             # Seeded data generator
        └── generator_test.go        # Determinism and profile shape tests
```

### Pattern 1: ChatRepository Interface with Context

**What:** Define the interface in its own package, separate from implementations. All methods take `context.Context` first, return `(result, error)`.

**When to use:** This is the only correct pattern per D-01/D-02/D-03.

**Example:**
```go
// Source: D-01, D-02, D-03 from CONTEXT.md; Go context convention from golang.org/blog/context
package repository

import (
    "context"
    "github.com/anti-duhring/agent-db/internal/domain"
    "github.com/google/uuid"
)

type ChatRepository interface {
    CreateConversation(ctx context.Context, partnerID, userID uuid.UUID) (domain.Conversation, error)
    AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error)
    LoadWindow(ctx context.Context, conversationID uuid.UUID, n int) ([]domain.Message, error)
    ListConversations(ctx context.Context, partnerID, userID uuid.UUID) ([]domain.Conversation, error)
}
```

### Pattern 2: Typed Domain Types

**What:** All domain types in `internal/domain/`, imported by every other package. No database-specific types leak into this package.

**When to use:** Enforces IFACE-02 — domain types are the only currency across the system.

**Example:**
```go
// Source: D-05 through D-08 from CONTEXT.md
package domain

import (
    "time"
    "github.com/google/uuid"
)

type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Conversation struct {
    ID        uuid.UUID
    PartnerID uuid.UUID
    UserID    uuid.UUID
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Message struct {
    ID             uuid.UUID
    ConversationID uuid.UUID
    Role           domain.Role
    Content        string
    TokenCount     int
    CreatedAt      time.Time
}
```

### Pattern 3: In-Memory Adapter with RWMutex

**What:** Maps keyed by uuid.UUID for O(1) lookup; `sync.RWMutex` protecting all state; messages stored as `[]domain.Message` per conversation ID.

**When to use:** Full working implementation per D-04 — not a stub. Phase 2 benchmarks use this as the reference backend.

**Example:**
```go
// Source: Claude's discretion (D-14, D-16); sync package stdlib
package memory

import (
    "context"
    "fmt"
    "sort"
    "sync"
    "time"

    "github.com/anti-duhring/agent-db/internal/domain"
    "github.com/anti-duhring/agent-db/internal/repository"
    "github.com/google/uuid"
)

type MemoryRepository struct {
    mu            sync.RWMutex
    conversations map[uuid.UUID]domain.Conversation
    messages      map[uuid.UUID][]domain.Message // keyed by conversationID
}

func New() *MemoryRepository {
    return &MemoryRepository{
        conversations: make(map[uuid.UUID]domain.Conversation),
        messages:      make(map[uuid.UUID][]domain.Message),
    }
}

// Verify interface compliance at compile time
var _ repository.ChatRepository = (*MemoryRepository)(nil)
```

### Pattern 4: Seeded Gofakeit Generator

**What:** Construct a `*gofakeit.Faker` with `gofakeit.New(seed)`. All generation calls on that single instance. Wrap in a generator struct that holds the faker and profile config.

**When to use:** Required by D-09 — single seed controls all output, reproducible across invocations.

**Example:**
```go
// Source: gofakeit/v7 pkg.go.dev; D-09 through D-12 from CONTEXT.md
package generator

import (
    "time"

    "github.com/brianvoe/gofakeit/v7"
    "github.com/anti-duhring/agent-db/internal/domain"
    "github.com/google/uuid"
)

type Profile struct {
    Conversations int
    Messages      int // per conversation
}

var (
    Small  = Profile{Conversations: 5,  Messages: 10}
    Medium = Profile{Conversations: 10, Messages: 500}
    Large  = Profile{Conversations: 10, Messages: 5000}
)

type Generator struct {
    faker *gofakeit.Faker
}

func New(seed int64) *Generator {
    return &Generator{faker: gofakeit.New(seed)}
}

func (g *Generator) Generate(partnerID, userID uuid.UUID, profile Profile) []domain.Conversation {
    // ... implementation
}

// generateContent returns a string in the 100-2000 char range
// Strategy: alternate between Sentence(12) (~80 chars) and Paragraph(2) (~400 chars)
// with a random choice per message
func (g *Generator) generateContent() string {
    // Use faker.IntRange or a threshold to vary length
    // Sentence(wordCount int) string — ~6 chars per word average
    // Paragraph(paragraphCount int) string — longer output
    content := g.faker.Paragraph(1)
    // enforce 100 char minimum by appending if needed
    for len(content) < 100 {
        content += " " + g.faker.Sentence(10)
    }
    // cap at 2000 chars
    if len(content) > 2000 {
        content = content[:2000]
    }
    return content
}

func tokenCount(content string) int {
    return len(content) / 4
}
```

### Pattern 5: Compile-Time Interface Check

**What:** `var _ repository.ChatRepository = (*MemoryRepository)(nil)` as a package-level blank identifier assignment.

**When to use:** Every concrete implementation package. Causes a compile error — not a runtime panic — if the implementation drifts from the interface.

**Example:**
```go
// Source: idiomatic Go; enforces IFACE-01 compliance at compile time
var _ repository.ChatRepository = (*MemoryRepository)(nil)
```

### Anti-Patterns to Avoid

- **Exporting database-specific types from the interface package:** Violates IFACE-02. `LoadWindow` must return `[]domain.Message`, not a driver-specific result.
- **Using string instead of uuid.UUID for IDs:** Loses type safety that prevents passing a userID where a conversationID is expected. D-05 requires typed UUIDs.
- **Calling gofakeit global functions (not the seeded instance):** `gofakeit.Sentence(10)` uses a global non-deterministic source. Must use `faker.Sentence(10)` on the seeded instance.
- **No mutex on in-memory adapter:** Causes data races under concurrent Phase 2 benchmark goroutines. Use RWMutex from the start.
- **Storing messages in a single flat slice:** Makes LoadWindow O(n) over all messages. Store messages per conversationID as `map[uuid.UUID][]domain.Message`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation | Custom random ID generator | `uuid.New()` from `github.com/google/uuid` | RFC 4122 compliance, collision guarantees, type safety |
| Realistic sentence generation | String concatenation of random words | `faker.Sentence(wordCount int)` | gofakeit handles word selection, capitalization, punctuation |
| Paragraph generation for long content | Loop of Sentence() calls | `faker.Paragraph(paragraphCount int)` | Produces natural paragraph structure in one call |
| Token count estimation | Real tokenizer | `len(content) / 4` | D-12 explicitly chose this approximation; no tokenizer dependency needed |

**Key insight:** For a benchmark harness, the fake data just needs to be realistic in size and structure — not linguistically perfect. gofakeit's built-in functions are more than sufficient and eliminate all hand-rolled text generation.

## Common Pitfalls

### Pitfall 1: Global Gofakeit vs Seeded Instance

**What goes wrong:** Developer calls package-level `gofakeit.Sentence(10)` instead of `faker.Sentence(10)` on the seeded instance. Output is non-deterministic — DATA-01 violated.

**Why it happens:** gofakeit provides both global functions and instance methods with identical names. The global functions auto-seed with time.Now().

**How to avoid:** Always construct `faker := gofakeit.New(seed)` and call methods only on `faker`. Never import and call `gofakeit.Sentence(...)` directly. Consider a lint rule or code review checklist.

**Warning signs:** Tests that pass sometimes and fail other times; two runs with identical seed produce different output.

### Pitfall 2: Missing Interface Compliance Verification

**What goes wrong:** `MemoryRepository` compiles, but is missing a method from `ChatRepository`. Discovered only when Phase 2 tries to assign it to a variable of type `ChatRepository`.

**Why it happens:** Go interfaces are satisfied implicitly — the compiler won't tell you until you actually use it as the interface type.

**How to avoid:** Add `var _ repository.ChatRepository = (*MemoryRepository)(nil)` to memory.go. This causes a compile error immediately if any method is missing.

**Warning signs:** Compiles fine until concrete assignment in another package.

### Pitfall 3: Role Type Misuse

**What goes wrong:** Code passes arbitrary string literals as `domain.Role` instead of `RoleUser` or `RoleAssistant`. Type safety is undermined.

**Why it happens:** `type Role string` allows string literals to be assigned to Role values without explicit conversion in some contexts.

**How to avoid:** Only use `domain.RoleUser` and `domain.RoleAssistant` constants. In generator role alternation, use index parity: `if i%2 == 0 { role = domain.RoleUser } else { role = domain.RoleAssistant }`.

**Warning signs:** Role values in generated data that don't match the two constants.

### Pitfall 4: Go Version Mismatch

**What goes wrong:** `go.mod` specifies `go 1.26` but the installed Go is 1.25.4. If any 1.26-specific language features are accidentally used, builds will fail.

**Why it happens:** CLAUDE.md targets Go 1.26 but the dev machine has 1.25.4 (verified).

**How to avoid:** Write only Go 1.25-compatible code in Phase 1. Avoid any language features introduced in 1.26. The `go 1.26` directive in go.mod sets the minimum toolchain version but does not prevent 1.25.4 from compiling the code as long as no 1.26-specific constructs are used.

**Warning signs:** Compiler errors referencing unknown syntax when building with go1.25.4.

### Pitfall 5: ListConversations Sort Order

**What goes wrong:** ListConversations returns conversations in map iteration order (random in Go) instead of sorted by last activity. D-03 requires sorted output.

**Why it happens:** Map iteration in Go is deliberately randomized. Iterating `conversations` map and appending to a slice produces non-deterministic order.

**How to avoid:** After building the result slice, always call `sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })`.

**Warning signs:** ListConversations tests that check ordering fail non-deterministically.

## Code Examples

Verified patterns from official sources:

### Module Initialization
```bash
# Source: go.dev/doc/modules/gomod-ref
cd /path/to/agent-db
go mod init github.com/anti-duhring/agent-db
go get github.com/google/uuid@v1.6.0
go get github.com/brianvoe/gofakeit/v7@v7.14.1
```

### UUID Generation
```go
// Source: pkg.go.dev/github.com/google/uuid
import "github.com/google/uuid"

id := uuid.New()          // returns uuid.UUID ([16]byte), panics on failure
id, err := uuid.NewRandom() // returns (uuid.UUID, error)
```

### Seeded Faker Construction
```go
// Source: pkg.go.dev/github.com/brianvoe/gofakeit/v7
import "github.com/brianvoe/gofakeit/v7"

faker := gofakeit.New(42)              // seed=42, returns *Faker
sentence := faker.Sentence(12)        // ~12-word sentence, returns string
paragraph := faker.Paragraph(2)      // 2-paragraph block, returns string
```

### Content Length Control
```go
// Source: gofakeit/v7 API + D-10, D-03 from CONTEXT.md
// Strategy: use Paragraph(1) as base (~200-400 chars), pad if under 100, truncate at 2000
func (g *Generator) generateContent() string {
    content := g.faker.Paragraph(1)
    for len(content) < 100 {
        content += " " + g.faker.Sentence(10)
    }
    if len(content) > 2000 {
        content = content[:2000]
    }
    return content
}
```

### Role Alternation
```go
// Source: D-07, D-03 from CONTEXT.md; standard Go index parity pattern
for i := 0; i < profile.Messages; i++ {
    role := domain.RoleUser
    if i%2 != 0 {
        role = domain.RoleAssistant
    }
    // ...
}
```

### Compile-Time Interface Check
```go
// Source: Idiomatic Go; enforces IFACE-01 at compile time
var _ repository.ChatRepository = (*MemoryRepository)(nil)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `lib/pq` Postgres driver | `pgx/v5` | ~2020 | lib/pq is maintenance-only; irrelevant for Phase 1 but noted for Phase 2 |
| `aws-sdk-go` (v1) | `aws-sdk-go-v2` | 2023 | v1 is maintenance-only; irrelevant for Phase 1 |
| Global gofakeit functions | Seeded `*Faker` instance | Always best practice | Determinism required by DATA-01 |

**Deprecated/outdated:**
- `go-libsql` (CGO-based Turso driver): Noted in STATE.md for Phase 3 re-evaluation. Not relevant to Phase 1.

## Open Questions

1. **gofakeit Paragraph() exact character output range**
   - What we know: `Paragraph(1)` returns one paragraph; average output is roughly 200-600 chars based on gofakeit internals (unverified exact range).
   - What's unclear: Whether `Paragraph(1)` can return under 100 chars in edge cases, and how often the truncation at 2000 chars would fire.
   - Recommendation: The defensive `generateContent()` pattern (pad if < 100, truncate if > 2000) handles any edge case regardless of exact output range. Implement defensively.

2. **Go 1.26 vs 1.25.4 on dev machine**
   - What we know: CLAUDE.md targets Go 1.26; dev machine has 1.25.4 (verified 2026-03-31).
   - What's unclear: Whether Go 1.26 introduced any language features Phase 1 code might accidentally rely on.
   - Recommendation: Set `go 1.26` in go.mod per CLAUDE.md. Write code that compiles on 1.25.4 as the immediate verification target. No 1.26-specific syntax constructs are needed for this phase.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go code | Yes | 1.25.4 | — |
| `go mod` | Module initialization | Yes | 1.25.4 | — |
| `github.com/google/uuid` | Domain types (D-05) | Downloadable | v1.6.0 | — |
| `github.com/brianvoe/gofakeit/v7` | Generator (D-09/D-10) | Downloadable | v7.14.1 | — |
| Network (go get) | Fetching dependencies | Assumed yes | — | Pre-cache with `GONOSUMCHECK` if offline |

**Missing dependencies with no fallback:** None — this is a pure code phase.

**Note on Go version:** Dev machine has 1.25.4; CLAUDE.md targets 1.26. Set `go 1.26` in go.mod. All Phase 1 constructs (interfaces, structs, stdlib packages) are compatible with 1.25.4. No upgrade required to build and test Phase 1.

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/google/uuid` — UUID type definition, New(), NewRandom(), Parse() signatures
- `proxy.golang.org/github.com/google/uuid/@v/list` — v1.6.0 confirmed current (2024-01-23)
- `proxy.golang.org/github.com/brianvoe/gofakeit/v7/@v/v7.14.1.info` — v7.14.1 published 2026-03-03
- `pkg.go.dev/github.com/brianvoe/gofakeit/v7#New` — `New(seed int64) *Faker` confirmed
- `pkg.go.dev/github.com/brianvoe/gofakeit/v7#Faker.Sentence` — `Sentence(wordCount int) string` confirmed
- `pkg.go.dev/github.com/brianvoe/gofakeit/v7#Faker.Paragraph` — `Paragraph(paragraphCount int) string` confirmed
- `golang.org/blog/context` — context.Context as first parameter is the standard Go convention
- `go.dev/doc/modules/gomod-ref` — go.mod module directive format
- CLAUDE.md §Technology Stack — all library version choices locked and pre-researched
- CONTEXT.md D-01 through D-16 — all implementation decisions locked by prior discussion

### Secondary (MEDIUM confidence)
- `go env GOVERSION` output: Go 1.25.4 confirmed on dev machine (2026-03-31)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified against Go module proxy and pkg.go.dev on 2026-03-31
- Architecture: HIGH — patterns are idiomatic Go; all decisions locked in CONTEXT.md; no architectural ambiguity remains
- Pitfalls: HIGH — gofakeit global/instance distinction is a documented library behavior; interface compliance check and mutex patterns are standard Go

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (stable libraries; gofakeit and uuid release infrequently)
