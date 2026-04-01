---
phase: 03-dynamodb-turso-adapters
plan: 01
subsystem: database
tags: [dynamodb, aws-sdk-go-v2, localstack, testcontainers, single-table-design]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: ChatRepository interface, domain types (Conversation, Message, Role)
  - phase: 02-runner-postgres-baseline
    provides: Postgres adapter as reference pattern for constructor, Close, method signatures
provides:
  - DynamoDBRepository implementing ChatRepository (internal/repository/dynamodb/dynamodb.go)
  - 7 integration tests passing against LocalStack 3.8 (internal/repository/dynamodb/dynamodb_test.go)
affects: [03-02-turso-adapter, 03-03-cli-wiring, 04-report]

# Tech tracking
tech-stack:
  added:
    - github.com/aws/aws-sdk-go-v2 v1.41.5
    - github.com/aws/aws-sdk-go-v2/config v1.32.13
    - github.com/aws/aws-sdk-go-v2/credentials v1.19.13
    - github.com/aws/aws-sdk-go-v2/service/dynamodb v1.57.1
    - github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.20.37
    - github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression v1.8.37
    - github.com/testcontainers/testcontainers-go/modules/localstack v0.41.0
  patterns:
    - Single-table DynamoDB with USER# and CONV# partition key prefixes
    - convSK time-sortable SK for conversation listing (CONV#<RFC3339Nano>#<uuid>)
    - TransactWriteItems for atomic multi-item writes (4-item AppendMessage)
    - GetItem + TransactWriteItems sequence for read-modify-write on AppendMessage
    - attributevalue.MarshalMap/UnmarshalMap with dynamodbav struct tags
    - expression.NewBuilder() for KeyConditionExpression construction
    - ScanIndexForward=false + in-place reverse for oldest-first window queries
    - LocalStack 3.8 (pinned, not latest) to avoid license auth requirement

key-files:
  created:
    - internal/repository/dynamodb/dynamodb.go
    - internal/repository/dynamodb/dynamodb_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Used localstack/localstack:3.8 instead of :latest — latest image (2026.x) requires LOCALSTACK_AUTH_TOKEN; 3.8 is the last OSS version"
  - "4-item TransactWriteItems in AppendMessage: message Put + old listing Delete + new listing Put + meta Put — atomic SK rotation per D-04"
  - "convMetaRecord stores user_pk and conv_listing_sk to avoid GSI for SK rotation lookups in AppendMessage"
  - "LoadWindow returns make([]domain.Message, 0, len(result.Items)) not nil for empty result"

patterns-established:
  - "DynamoDB key encoding: userPK(), convSK(), convPK(), msgSK() functions with format constants"
  - "Record types use dynamodbav struct tags, all time.Time stored as RFC3339Nano strings"
  - "New(ctx, endpoint) constructor — empty endpoint uses AWS credential chain, non-empty uses static creds + endpoint override for LocalStack"
  - "ensureTable() ignores ResourceInUseException (idempotent table creation)"

requirements-completed: [IFACE-03]

# Metrics
duration: 36min
completed: 2026-04-01
---

# Phase 03 Plan 01: DynamoDB ChatRepository Adapter Summary

**DynamoDB single-table adapter with TransactWriteItems SK rotation and 7 passing LocalStack integration tests**

## Performance

- **Duration:** 36 min
- **Started:** 2026-04-01T01:02:19Z
- **Completed:** 2026-04-01T01:08:17Z
- **Tasks:** 2
- **Files modified:** 4 (dynamodb.go, dynamodb_test.go, go.mod, go.sum)

## Accomplishments

- Implemented full DynamoDB ChatRepository adapter with single-table design (D-01 through D-06)
- AppendMessage uses 4-item TransactWriteItems: message Put, old listing Delete, new listing Put, updated meta Put — atomic SK rotation ensures ListConversations always reflects latest activity
- All 7 integration tests pass against LocalStack 3.8 (pinned to last OSS version before auth requirement)

## Task Commits

1. **Task 1: Implement DynamoDB ChatRepository adapter** - `e74a461` (feat)
2. **Task 2: DynamoDB integration tests via LocalStack** - `d168f03` (test)

## Files Created/Modified

- `internal/repository/dynamodb/dynamodb.go` — DynamoDB adapter implementing ChatRepository with single-table design
- `internal/repository/dynamodb/dynamodb_test.go` — 7 integration tests via LocalStack 3.8
- `go.mod` — Added aws-sdk-go-v2 modules and localstack testcontainers module
- `go.sum` — Updated checksums

## Decisions Made

- Pinned LocalStack to `3.8` instead of `latest` — the latest image (2026.3.x) now requires a `LOCALSTACK_AUTH_TOKEN` license. Version 3.8 is the last publicly available OSS version and works without credentials.
- `convMetaRecord` stores both `user_pk` and `conv_listing_sk` fields so AppendMessage can do a single GetItem to find the old listing SK without a GSI or additional query.
- Used `make([]domain.Message, 0, ...)` for LoadWindow return to guarantee non-nil empty slice when no messages exist.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Switched LocalStack image from :latest to :3.8**
- **Found during:** Task 2 (DynamoDB integration tests)
- **Issue:** `localstack/localstack:latest` now exits with code 55 requiring LOCALSTACK_AUTH_TOKEN; all 7 tests failed to start the container
- **Fix:** Changed image tag from `latest` to `3.8` (last OSS version without auth requirement)
- **Files modified:** internal/repository/dynamodb/dynamodb_test.go
- **Verification:** All 7 tests pass with localstack:3.8
- **Committed in:** d168f03 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** LocalStack image pin was necessary for tests to run at all. No scope creep.

## Issues Encountered

- LocalStack `latest` tag now requires paid account. Pinned to 3.8 as the last OSS version — this is a stable choice since 3.x series is well-tested and DynamoDB emulation is complete.

## User Setup Required

None — DynamoDB tests use LocalStack (Docker); no AWS credentials or external accounts needed for running tests.

## Next Phase Readiness

- DynamoDB adapter is fully implemented and tested — ready for CLI wiring in plan 03
- Pattern established: LocalStack 3.8 image for DynamoDB testing (Turso plan can reference this)
- No blockers for proceeding to plan 02 (Turso adapter)

## Self-Check: PASSED

- FOUND: internal/repository/dynamodb/dynamodb.go
- FOUND: internal/repository/dynamodb/dynamodb_test.go
- FOUND: .planning/phases/03-dynamodb-turso-adapters/03-01-SUMMARY.md
- FOUND commit: e74a461 (feat: DynamoDB adapter)
- FOUND commit: d168f03 (test: DynamoDB integration tests)

---
*Phase: 03-dynamodb-turso-adapters*
*Completed: 2026-04-01*
