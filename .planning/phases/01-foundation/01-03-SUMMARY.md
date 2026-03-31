---
phase: 01-foundation
plan: "03"
subsystem: testing
tags: [gofakeit, uuid, generator, synthetic-data, deterministic, benchmark]

# Dependency graph
requires:
  - phase: 01-01
    provides: domain types (Conversation, Message, Role) in internal/domain/types.go
provides:
  - Seeded deterministic data generator with three benchmark profiles (small/medium/large)
  - GeneratedData struct holding conversations and messages keyed by conversation ID
  - Profile constants matching D-11 requirements
affects:
  - 02-benchmark-runner (uses generator to produce test data for benchmark scenarios)
  - 02-postgres-adapter (uses generator data for integration tests)
  - 03-dynamodb-adapter (uses generator data for integration tests)
  - 03-turso-adapter (uses generator data for integration tests)

# Tech tracking
tech-stack:
  added:
    - github.com/brianvoe/gofakeit/v7 v7.14.1 (seeded synthetic data generation)
  patterns:
    - "Seeded faker pattern: always use instance methods (g.faker.X), never package-level gofakeit.X functions"
    - "Deterministic UUID generation via 16 Uint8() byte loop with version/variant bit setting"
    - "Profile-driven generation: Profile struct drives conversation and message counts"

key-files:
  created:
    - internal/generator/generator.go
    - internal/generator/generator_test.go
  modified:
    - go.mod (added gofakeit/v7)
    - go.sum

key-decisions:
  - "gofakeit v7 uses uint64 seed (not int64); cast int64 seed to uint64 in New() constructor"
  - "gofakeit v7 uses math/rand/v2.Source which does not implement io.Reader; UUID generation uses 16 Uint8() calls with manual version/variant bit setting"
  - "Deterministic base time (2026-01-01 UTC) used for all timestamps so they are reproducible from seed"

patterns-established:
  - "Pattern: Generator wraps a single *gofakeit.Faker — no global faker calls anywhere"
  - "Pattern: GeneratedData returns conversations slice + messages map keyed by uuid.UUID"
  - "Pattern: Content length enforced via pad-then-truncate loop (< 100 pad, > 2000 truncate)"

requirements-completed: [DATA-01, DATA-02, DATA-03]

# Metrics
duration: 2min
completed: "2026-03-31"
---

# Phase 01 Plan 03: Seeded Data Generator Summary

**Deterministic synthetic chat generator using gofakeit/v7 seeded instance, producing small/medium/large benchmark profiles with role-alternating messages between 100-2000 chars and len/4 token counts**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-31T22:04:29Z
- **Completed:** 2026-03-31T22:06:31Z
- **Tasks:** 1 (TDD: test + impl)
- **Files modified:** 4

## Accomplishments

- Seeded `Generator` struct using `gofakeit.New(uint64(seed))` — same seed always produces identical output
- Three benchmark profiles: Small(5 conversations x 10 messages), Medium(10x500), Large(10x5000)
- Content generation pads to 100 chars minimum and truncates at 2000 chars via seeded Paragraph/Sentence calls
- Role alternation enforced: index%2==0 → RoleUser, else → RoleAssistant
- TokenCount computed as `len(content) / 4` per D-12
- 10 tests covering determinism, profile counts, content length, role alternation, token count, UUID non-zero, partner/user ID propagation

## Task Commits

1. **Task 1 (RED): failing tests** - `8da2130` (test)
2. **Task 1 (GREEN): generator implementation** - `8668a01` (feat)

## Files Created/Modified

- `internal/generator/generator.go` - Generator struct, New(), Generate(), generateContent(), newUUID(), Profile constants
- `internal/generator/generator_test.go` - 10 tests proving determinism, profile shapes, content constraints, role alternation
- `go.mod` - Added gofakeit/v7 v7.14.1
- `go.sum` - Dependency checksums

## Decisions Made

- gofakeit v7 changed seed type to `uint64` (not `int64`); the public API takes `int64` for ergonomics and casts internally
- gofakeit v7 uses `math/rand/v2.Source` which does NOT implement `io.Reader`, so `uuid.NewRandomFromReader(g.faker.Rand)` fails; fallback to 16 `Uint8()` calls with manual RFC-4122 version/variant bits
- All timestamps use a fixed base time (2026-01-01 UTC) plus deterministic offsets so they are reproducible without seeding time

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] gofakeit v7 seed type is uint64, not int64**
- **Found during:** Task 1 (GREEN — first compile)
- **Issue:** `gofakeit.New(seed)` where seed is `int64` caused compile error: "cannot use seed (variable of type int64) as uint64"
- **Fix:** Changed constructor to `gofakeit.New(uint64(seed))`; kept public `New(seed int64)` signature for ergonomics
- **Files modified:** internal/generator/generator.go
- **Verification:** go build passes, all 10 tests pass
- **Committed in:** 8668a01 (feat commit)

**2. [Rule 1 - Bug] gofakeit v7 Rand field does not implement io.Reader**
- **Found during:** Task 1 (GREEN — first compile)
- **Issue:** `uuid.NewRandomFromReader(g.faker.Rand)` caused compile error: "math/rand/v2.Source does not implement io.Reader (missing method Read)"
- **Fix:** Replaced with 16-byte loop using `g.faker.Uint8()` and manual version/variant bit setting (RFC-4122 UUID v4 format)
- **Files modified:** internal/generator/generator.go
- **Verification:** UUIDs are non-zero, unique, deterministic; TestNonZeroUUIDs and TestDeterminism pass
- **Committed in:** 8668a01 (feat commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — API mismatch bugs between plan spec and actual gofakeit v7 library)
**Impact on plan:** Both fixes were necessary for correct compilation. No scope changes.

## Issues Encountered

- gofakeit v7 API differs from what plan assumed: seed type is uint64, Rand field is math/rand/v2.Source not io.Reader. Fixed inline per Rule 1.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Generator is ready for use by Phase 2 benchmark runner
- `generator.New(seed).Generate(partnerID, userID, generator.Small/Medium/Large)` is the entry point
- GeneratedData.Conversations and GeneratedData.Messages[convID] provide all data needed for benchmark scenarios

---
*Phase: 01-foundation*
*Completed: 2026-03-31*
