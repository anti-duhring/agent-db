# Phase 3: DynamoDB + Turso Adapters - Research

**Researched:** 2026-03-31
**Domain:** DynamoDB single-table design (aws-sdk-go-v2), Turso/libsql adapter (libsql-client-go), LocalStack via testcontainers-go, multi-backend CLI wiring
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**DynamoDB table design:**
- D-01: Single-table design. One DynamoDB table (`chat_data`) with composite keys covering all access patterns. No GSI needed.
- D-02: Conversation listing items: PK=`USER#<partner_id>#<user_id>`, SK=`CONV#<updated_at>#<conv_id>`. Query PK with ScanIndexForward=false for ListConversations sorted by last activity.
- D-03: Message items: PK=`CONV#<conv_id>`, SK=`MSG#<created_at>#<msg_id>`. Query PK with ScanIndexForward=false, Limit=N for LoadWindow.
- D-04: AppendMessage uses TransactWriteItems for atomic consistency: (1) Put message item, (2) Delete old conversation SK, (3) Put new conversation SK with updated timestamp. Guarantees listing always reflects latest message timestamp.
- D-05: ConsistentRead=true for all read scenarios per roadmap success criteria. Warmup pass before timing begins.
- D-06: No Scan operations — all reads use Query per roadmap success criteria.

**Turso SDK and driver:**
- D-07: Use `libsql-client-go` (deprecated but functional, pure Go, no CGO). Connects via `libsql://` protocol to Turso Cloud.
- D-08: Raw `database/sql` for all queries — direct `sql.Query` + manual `rows.Scan` into domain types. No sqlx or other scanning libraries.
- D-09: Turso schema mirrors Postgres schema (two tables: conversations, messages) since Turso is SQLite-compatible. Same indexes adapted to SQLite syntax.

**Local testing strategy:**
- D-10: DynamoDB uses LocalStack via testcontainers-go — same pattern as Postgres. Spins up LocalStack container, creates table, runs tests, auto-cleans up.
- D-11: Turso uses a dedicated dev/staging database on Turso Cloud. Connection via environment variables (`TURSO_URL`, `TURSO_AUTH_TOKEN`). Real internet latency is the point.
- D-12: Tests and benchmarks gracefully skip Turso if env vars are not set. No failure, just skip with informational message.

**CLI integration:**
- D-13: `--backend` flag accepts `postgres`, `dynamodb`, `turso`, comma-separated combinations, or `all`. Extends the existing flag validation in `main.go`.
- D-14: `--backend all` skips unavailable backends with a warning line and runs whatever's available. Results table shows "skipped" for unavailable backends. No failure.
- D-15: BackendMeta transport annotation for Turso: header note above Turso results showing `Transport: libsql:// (remote, internet)` plus a note that latency includes internet round-trip to Turso Cloud.
- D-16: Side-by-side output when running multiple backends — each backend gets its own results section with header metadata.

### Claude's Discretion
- DynamoDB attribute naming conventions (beyond PK/SK structure)
- Exact DynamoDB table provisioning settings (on-demand vs provisioned for benchmark)
- Turso schema DDL details (column types, index names)
- Error message wording and retry behavior
- Container configuration details (LocalStack version, resource limits)
- Exact CLI output formatting and spacing

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IFACE-03 | Three implementations of ChatRepository: Postgres, DynamoDB, Turso | DynamoDB adapter uses aws-sdk-go-v2 + expression builder + TransactWriteItems. Turso adapter uses libsql-client-go + database/sql. Both implement the 4-method ChatRepository interface exactly. |
</phase_requirements>

---

## Summary

Phase 3 completes the three-way comparison by implementing the DynamoDB and Turso `ChatRepository` adapters. All technical decisions have been locked in CONTEXT.md. The research focus is on (1) the exact aws-sdk-go-v2 API patterns for the locked DynamoDB schema, (2) the libsql-client-go connection and query patterns for Turso, (3) the LocalStack testcontainers-go setup for DynamoDB integration tests, and (4) multi-backend CLI wiring patterns consistent with the existing Phase 2 code.

The primary complexity is DynamoDB's `AppendMessage` — it requires a three-item `TransactWriteItems` call to atomically put the message, delete the stale conversation sort key, and put the new conversation sort key with the updated timestamp. All other operations are straightforward `Query` calls using the expression builder. The Turso adapter is structurally identical to the Postgres adapter but uses `database/sql` with the libsql driver and SQLite-compatible DDL.

The locked schema uses ISO8601 timestamps in sort keys (e.g., `CONV#2026-01-15T10:30:00Z#<uuid>`), which preserves lexicographic sort order in DynamoDB — ISO8601 is both human-readable in the AWS console and correctly sortable without any special handling.

**Primary recommendation:** Implement DynamoDB adapter first (higher complexity, needs TransactWriteItems + LocalStack setup), then Turso (structurally simple, mirrors Postgres). Wire CLI backend dispatch last after both adapters pass their integration tests.

---

## Standard Stack

### Core — New Dependencies for Phase 3

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/aws/aws-sdk-go-v2/service/dynamodb` | v1.57.1 | DynamoDB client, Query, TransactWriteItems | Official AWS SDK v2; v1 is maintenance-only |
| `github.com/aws/aws-sdk-go-v2/config` | latest (same module) | AWS credential chain loading | Picks up env vars, instance profiles, local config |
| `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` | latest (same module) | Struct marshaling via `dynamodbav` tags | Eliminates manual `AttributeValue` construction |
| `github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression` | v1.8.37 | Type-safe query/key condition builder | Handles reserved-word aliasing automatically |
| `github.com/tursodatabase/libsql-client-go/libsql` | v0.0.0-20251219 | Turso remote driver (pure Go, no CGO) | CGO-free build; implements database/sql interface |
| `github.com/testcontainers/testcontainers-go/modules/localstack` | v0.41.0 | LocalStack container for DynamoDB tests | Same testcontainers version as Postgres module already in project |

### Already Present (no changes needed)

| Library | Version | Purpose |
|---------|---------|---------|
| `github.com/testcontainers/testcontainers-go` | v0.41.0 | Container lifecycle management |
| `github.com/google/uuid` | v1.6.0 | UUID generation (used in all repos) |
| `github.com/HdrHistogram/hdrhistogram-go` | v1.2.0 | Latency histogram (runner) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `libsql-client-go` | `go-libsql` | go-libsql requires CGO + precompiled C libs; adds build complexity; pure-Go build is the correct choice for a benchmark harness |
| expression builder | raw `map[string]types.AttributeValue` | Raw map construction is error-prone and misses reserved-word escaping; expression builder handles it |
| LocalStack | `amazon/dynamodb-local` official image | Both work; LocalStack integrates with testcontainers-go module already established in project; testcontainers-go/modules/dynamodb also exists as an alternative |
| `attributevalue.MarshalMap` | Manual `AttributeValueMemberS` construction | Manual is verbose; MarshalMap with `dynamodbav` tags is cleaner and consistent with project style |

**Installation:**
```bash
go get github.com/aws/aws-sdk-go-v2/config \
       github.com/aws/aws-sdk-go-v2/service/dynamodb \
       github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue \
       github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression \
       github.com/tursodatabase/libsql-client-go/libsql \
       github.com/testcontainers/testcontainers-go/modules/localstack@v0.41.0
```

**Version verification:** aws-sdk-go-v2/service/dynamodb v1.57.1 verified on pkg.go.dev (published 2026-03-26). expression package v1.8.37 verified on pkg.go.dev (published 2026-03-26). libsql-client-go v0.0.0-20251219100830-236aa1ff8acc verified on pkg.go.dev (published 2025-12-19). All three AWS SDK packages are in the same module; `go get` pulls consistent versions automatically.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/repository/
├── repository.go              # ChatRepository interface (locked, do not touch)
├── memory/                    # Phase 1 in-memory adapter (reference)
├── postgres/                  # Phase 2 adapter (reference pattern)
│   ├── postgres.go
│   ├── postgres_test.go
│   └── migrations/
│       └── 001_create_tables.sql
├── dynamodb/                  # NEW: Phase 3
│   ├── dynamodb.go            # DynamoDBRepository struct + 4 methods
│   └── dynamodb_test.go       # Integration tests via LocalStack
└── turso/                     # NEW: Phase 3
    ├── turso.go               # TursoRepository struct + 4 methods
    ├── turso_test.go          # Integration tests (skip if env vars absent)
    └── migrations/
        └── 001_create_tables.sql  # SQLite-compatible DDL
```

The `main.go` backend switch gains two new cases. The Postgres container lifecycle code becomes a pattern that both backends follow.

---

### Pattern 1: DynamoDB Adapter Constructor

The DynamoDB adapter constructor creates the AWS client, creates the `chat_data` table if it doesn't exist, then waits for the table to become ACTIVE using the built-in `TableExistsWaiter`. Use `PAY_PER_REQUEST` billing (on-demand) — correct for a benchmark that runs variable load and removes the need to estimate provisioned capacity.

```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb + official AWS code examples
package dynamodb

import (
    "context"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/anti-duhring/agent-db/internal/repository"
)

const tableName = "chat_data"

var _ repository.ChatRepository = (*DynamoDBRepository)(nil)

type DynamoDBRepository struct {
    client *dynamodb.Client
    table  string
}

func New(ctx context.Context, endpoint string) (*DynamoDBRepository, error) {
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
        // For LocalStack: inject static credentials
    )
    if err != nil {
        return nil, err
    }

    client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
        if endpoint != "" {
            o.BaseEndpoint = aws.String(endpoint)
        }
    })

    repo := &DynamoDBRepository{client: client, table: tableName}
    if err := repo.ensureTable(ctx); err != nil {
        return nil, err
    }
    return repo, nil
}

func (r *DynamoDBRepository) ensureTable(ctx context.Context) error {
    _, err := r.client.CreateTable(ctx, &dynamodb.CreateTableInput{
        TableName:   aws.String(r.table),
        BillingMode: types.BillingModePayPerRequest,
        AttributeDefinitions: []types.AttributeDefinition{
            {AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
            {AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
        },
        KeySchema: []types.KeySchemaElement{
            {AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
            {AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
        },
    })
    // Ignore ResourceInUseException (table already exists)
    var riu *types.ResourceInUseException
    if err != nil && !errors.As(err, &riu) {
        return err
    }
    waiter := dynamodb.NewTableExistsWaiter(r.client)
    return waiter.Wait(ctx, &dynamodb.DescribeTableInput{
        TableName: aws.String(r.table),
    }, 5*time.Minute)
}
```

**Key insight:** `errors.As(err, &riu)` correctly unwrites the AWS SDK error wrapping — do not use string matching on error messages.

---

### Pattern 2: DynamoDB Key Encoding

All keys follow the locked schema (D-02, D-03). ISO8601 UTC timestamps are used in sort keys because they sort lexicographically in the same order as chronological order when formatted as RFC3339 (`time.RFC3339Nano`).

```go
// Source: CONTEXT.md D-02, D-03 + AWS DynamoDB sort key docs
const (
    pkUserPrefix = "USER#"
    pkConvPrefix = "CONV#"
    skConvPrefix = "CONV#"
    skMsgPrefix  = "MSG#"
)

// Conversation listing PK: USER#<partner_id>#<user_id>
func userPK(partnerID, userID uuid.UUID) string {
    return fmt.Sprintf("USER#%s#%s", partnerID, userID)
}

// Conversation listing SK: CONV#<updated_at_iso8601>#<conv_id>
func convSK(updatedAt time.Time, convID uuid.UUID) string {
    return fmt.Sprintf("CONV#%s#%s", updatedAt.UTC().Format(time.RFC3339Nano), convID)
}

// Message PK: CONV#<conv_id>
func convPK(convID uuid.UUID) string {
    return fmt.Sprintf("CONV#%s", convID)
}

// Message SK: MSG#<created_at_iso8601>#<msg_id>
func msgSK(createdAt time.Time, msgID uuid.UUID) string {
    return fmt.Sprintf("MSG#%s#%s", createdAt.UTC().Format(time.RFC3339Nano), msgID)
}
```

---

### Pattern 3: DynamoDB AppendMessage (TransactWriteItems)

This is the most complex operation. It must atomically: (1) put the message item, (2) delete the stale conversation listing item (old SK), (3) put the new conversation listing item (new SK with updated timestamp). The old SK must be tracked in the conversation item so it can be deleted — store `old_sk` as an attribute on the conversation listing item, or derive it by storing `updated_at` separately.

**Practical approach:** Store `updated_at` as a plain attribute on the conversation item. Before AppendMessage, load the current `updated_at` to compute the old SK for the delete. But this adds a read per write. A better approach: embed the full SK value as an attribute (`conv_sk`) in the conversation item itself so the delete key can be reconstructed without a read.

```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb (TransactWriteItems)
func (r *DynamoDBRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role domain.Role, content string) (domain.Message, error) {
    id := uuid.New()
    now := time.Now().UTC()
    tc := len(content) / 4

    // Step 1: Load the current conversation item to get old_sk for deletion.
    // Store the current SK as a plain attribute "conv_sk" when conversation is created.
    // This avoids a separate GetItem — the conv item has its own SK embedded.
    oldSK, err := r.getConvCurrentSK(ctx, conversationID)
    if err != nil {
        return domain.Message{}, err
    }
    userPKVal, err := r.getConvUserPK(ctx, conversationID)
    if err != nil {
        return domain.Message{}, err
    }

    newSK := convSK(now, conversationID)

    msgItem, _ := attributevalue.MarshalMap(msgRecord{
        PK:             convPK(conversationID),
        SK:             msgSK(now, id),
        MessageID:      id.String(),
        ConversationID: conversationID.String(),
        Role:           string(role),
        Content:        content,
        TokenCount:     tc,
        CreatedAt:      now.Format(time.RFC3339Nano),
    })

    newConvItem, _ := attributevalue.MarshalMap(convRecord{
        PK:        userPKVal,
        SK:        newSK,
        ConvSK:    newSK,  // embedded for future AppendMessage lookups
        ConvID:    conversationID.String(),
        UpdatedAt: now.Format(time.RFC3339Nano),
    })

    _, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
        TransactItems: []types.TransactWriteItem{
            {Put: &types.Put{TableName: aws.String(r.table), Item: msgItem}},
            {Delete: &types.Delete{
                TableName: aws.String(r.table),
                Key: map[string]types.AttributeValue{
                    "PK": &types.AttributeValueMemberS{Value: userPKVal},
                    "SK": &types.AttributeValueMemberS{Value: oldSK},
                },
            }},
            {Put: &types.Put{TableName: aws.String(r.table), Item: newConvItem}},
        },
    })
    // ...
}
```

**Important gotcha:** The conversation item (for listing) and the message item live in the SAME table but have different PKs. The conversation listing item's PK is `USER#...` and the message item's PK is `CONV#...`. Both items participate in the same TransactWriteItems call across two "virtual" partitions — this is valid and is the whole point of single-table design.

**Alternative — avoid the extra read:** Store a "conversation metadata" item at a fixed SK (e.g., `CONV#META`) with PK=`CONV#<conv_id>`. This item holds `user_pk`, `partner_id`, `user_id`, `created_at`, and `conv_listing_sk` (the current listing SK). AppendMessage reads this item once (GetItem, not Query), then issues the TransactWriteItems. This is a single extra round-trip per append but keeps the code correct without needing to know the old SK from the caller.

---

### Pattern 4: DynamoDB Query (ListConversations and LoadWindow)

```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression
// ListConversations: query USER# partition, descending SK order
keyCond := expression.Key("PK").Equal(expression.Value(userPK(partnerID, userID)))
expr, _ := expression.NewBuilder().WithKeyCondition(keyCond).Build()

result, err := r.client.Query(ctx, &dynamodb.QueryInput{
    TableName:                 aws.String(r.table),
    KeyConditionExpression:    expr.KeyCondition(),
    ExpressionAttributeNames:  expr.Names(),
    ExpressionAttributeValues: expr.Values(),
    ScanIndexForward:          aws.Bool(false), // DESC — most recent first
    ConsistentRead:            aws.Bool(true),  // D-05
})

// LoadWindow: query CONV# partition, descending SK, with Limit
keyCond2 := expression.Key("PK").Equal(expression.Value(convPK(conversationID)))
expr2, _ := expression.NewBuilder().WithKeyCondition(keyCond2).Build()

result2, err := r.client.Query(ctx, &dynamodb.QueryInput{
    TableName:                 aws.String(r.table),
    KeyConditionExpression:    expr2.KeyCondition(),
    ExpressionAttributeNames:  expr2.Names(),
    ExpressionAttributeValues: expr2.Values(),
    ScanIndexForward:          aws.Bool(false), // DESC
    Limit:                     aws.Int32(int32(n)),
    ConsistentRead:            aws.Bool(true),  // D-05
})
// Then reverse result slice in-place (same pattern as Postgres adapter)
```

**Pitfall:** The `Limit` parameter in DynamoDB is applied BEFORE the filter expression (if any). Since we use no filter expression here — only a key condition — `Limit=N` correctly returns the N most recent items. No extra filtering needed.

**Pitfall:** `ScanIndexForward: aws.Bool(false)` must use a pointer — the zero value `false` would be mistaken for "not set" if you use the non-pointer form. Always use `aws.Bool(...)`.

---

### Pattern 5: Turso Adapter

The Turso adapter is structurally identical to Postgres. The key differences: (1) use `sql.Open("libsql", url)` with the auth token embedded in the URL, (2) SQLite-compatible DDL (no `uuid` type — use `TEXT`, no `timestamptz` — use `TEXT` or `DATETIME`).

```go
// Source: docs.turso.tech/sdk/go/quickstart + pkg.go.dev/github.com/tursodatabase/libsql-client-go/libsql
package turso

import (
    "context"
    "database/sql"
    _ "embed"
    _ "github.com/tursodatabase/libsql-client-go/libsql"
    "github.com/anti-duhring/agent-db/internal/repository"
)

//go:embed migrations/001_create_tables.sql
var schema string

var _ repository.ChatRepository = (*TursoRepository)(nil)

type TursoRepository struct {
    db *sql.DB
}

// New opens a connection to the Turso remote database.
// url format: "libsql://[DATABASE].turso.io?authToken=[TOKEN]"
func New(ctx context.Context, url string) (*TursoRepository, error) {
    db, err := sql.Open("libsql", url)
    if err != nil {
        return nil, err
    }
    if err := db.PingContext(ctx); err != nil {
        db.Close()
        return nil, err
    }
    if _, err := db.ExecContext(ctx, schema); err != nil {
        db.Close()
        return nil, err
    }
    return &TursoRepository{db: db}, nil
}

func (r *TursoRepository) Close() error {
    return r.db.Close()
}
```

The four interface methods use `db.QueryContext` / `db.ExecContext` with manual `rows.Scan` — same pattern as the Postgres adapter but no prepared statements (libsql-client-go handles the protocol transparently).

---

### Pattern 6: LocalStack TestContainer for DynamoDB

```go
// Source: golang.testcontainers.org/modules/localstack/
package dynamodb_test

import (
    "context"
    "fmt"
    "testing"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/docker/go-connections/nat"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/localstack"
    dynrepo "github.com/anti-duhring/agent-db/internal/repository/dynamodb"
)

func setupTestRepo(t *testing.T) (*dynrepo.DynamoDBRepository, func()) {
    t.Helper()
    ctx := context.Background()

    ctr, err := localstack.Run(ctx, "localstack/localstack:3")
    if err != nil {
        t.Fatalf("failed to start localstack: %v", err)
    }

    mappedPort, err := ctr.MappedPort(ctx, nat.Port("4566/tcp"))
    if err != nil {
        testcontainers.TerminateContainer(ctr)
        t.Fatalf("failed to get mapped port: %v", err)
    }

    host, err := ctr.Host(ctx)
    if err != nil {
        testcontainers.TerminateContainer(ctr)
        t.Fatalf("failed to get host: %v", err)
    }

    endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

    // DynamoDBRepository.New accepts optional endpoint override for LocalStack.
    repo, err := dynrepo.New(ctx, endpoint)
    if err != nil {
        testcontainers.TerminateContainer(ctr)
        t.Fatalf("failed to create DynamoDBRepository: %v", err)
    }

    cleanup := func() {
        testcontainers.TerminateContainer(ctr)
    }
    return repo, cleanup
}
```

**Key detail:** The `dynrepo.New` constructor must accept an optional `endpoint` parameter (empty string for production, LocalStack URL for tests). Use `dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { if endpoint != "" { o.BaseEndpoint = aws.String(endpoint) } })`.

**LocalStack credentials:** LocalStack accepts any non-empty AWS credentials. Use `credentials.NewStaticCredentialsProvider("test", "test", "test")` in the test config.

---

### Pattern 7: Turso Test Skip Pattern

```go
// Source: CONTEXT.md D-12
func TestTursoCreateConversation(t *testing.T) {
    tursoURL := os.Getenv("TURSO_URL")
    tursoToken := os.Getenv("TURSO_AUTH_TOKEN")
    if tursoURL == "" || tursoToken == "" {
        t.Skip("TURSO_URL or TURSO_AUTH_TOKEN not set — skipping Turso integration test")
    }
    url := fmt.Sprintf("%s?authToken=%s", tursoURL, tursoToken)
    repo, err := turso.New(context.Background(), url)
    // ...
}
```

---

### Pattern 8: Multi-Backend CLI Wiring

The existing `main.go` has a hardcoded `if *backend != "postgres"` guard. Phase 3 replaces this with a backend dispatch function that creates the appropriate repository (or skips if unavailable), runs the benchmark, and collects results per backend.

```go
// Source: CONTEXT.md D-13, D-14, D-16 + existing main.go patterns
backends := parseBackends(*backend) // returns []string from comma-separated or "all"

var allResults []backendResult
for _, b := range backends {
    repo, cleanup, err := createBackend(ctx, b)
    if err != nil {
        fmt.Printf("WARNING: backend %s unavailable: %v — skipping\n", b, err)
        allResults = append(allResults, backendResult{backend: b, skipped: true})
        continue
    }
    defer cleanup()
    runner := benchmark.NewRunner(repo, selectedScenarios, config)
    results, err := runner.Run(ctx)
    // ...
    allResults = append(allResults, backendResult{backend: b, results: results})
}

for _, br := range allResults {
    if br.skipped {
        fmt.Printf("\nBackend: %s — SKIPPED (unavailable)\n", br.backend)
        continue
    }
    if br.backend == "turso" {
        fmt.Println("Transport: libsql:// (remote, internet) — latency includes internet round-trip to Turso Cloud")
    }
    benchmark.PrintResults(br.backend, prof.Name, *iters, *seed, br.results)
}
```

---

### Anti-Patterns to Avoid

- **Using Scan instead of Query:** DynamoDB Scan reads every item in the table. All reads must use `Query` with explicit PK. Verified: D-06 locks this requirement.
- **ConsistentRead default (false):** The default for DynamoDB Query is eventual consistency. Always pass `ConsistentRead: aws.Bool(true)` for read scenarios or the benchmark will measure eventual-consistency reads, which is not what the spec requires.
- **ScanIndexForward zero value:** `ScanIndexForward` is a `*bool`. Assigning `false` directly (not through `aws.Bool(false)`) will produce a nil pointer and the SDK will use the default (true = ascending). Always use `aws.Bool(false)`.
- **Forgetting to delete old conversation SK:** If AppendMessage only puts the new conversation listing item without deleting the old SK, ListConversations will return duplicate entries for the same conversation (one per AppendMessage call). The TransactWriteItems three-item pattern (D-04) prevents this.
- **sql.Open without Ping for Turso:** `sql.Open` does not establish a connection. Call `db.PingContext(ctx)` immediately to verify the connection is live before proceeding to schema setup.
- **Turso schema using Postgres syntax:** Turso is SQLite-compatible. `uuid` type does not exist — use `TEXT`. `timestamptz` does not exist — use `TEXT` or `DATETIME`. Serial/sequence does not exist — use `INTEGER PRIMARY KEY AUTOINCREMENT` only for synthetic keys (not needed here since all IDs are UUIDs).
- **Embedded auth token in logs:** The connection URL contains the auth token as a query parameter. Do not log the full URL. Log only the base URL without the token.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DynamoDB expression name/value aliasing | Custom string interpolation into expression strings | `expression` package from `aws-sdk-go-v2/feature/dynamodb/expression` | Reserved words (e.g., `name`, `role`, `status`) silently break queries without aliasing |
| Struct-to-AttributeValue conversion | Manual `&types.AttributeValueMemberS{Value: ...}` for every field | `attributevalue.MarshalMap` / `UnmarshalListOfMaps` with `dynamodbav` struct tags | Manual construction is 5-10x more verbose and error-prone |
| Table existence check on startup | Describe-then-create logic | `errors.As(err, &types.ResourceInUseException{})` after CreateTable | Same result with less code |
| Table readiness polling | Manual sleep loop | `dynamodb.NewTableExistsWaiter` | Built-in waiter handles exponential backoff correctly |
| LocalStack wait strategy | Custom health-check | `testcontainers-go/modules/localstack` Run (has built-in wait) | Module handles service readiness |

**Key insight:** The DynamoDB expression builder is not optional ergonomics — it is required correctness. Without it, attribute names like `role` (a DynamoDB reserved word) will cause `ValidationException` errors at runtime.

---

## Common Pitfalls

### Pitfall 1: DynamoDB Reserved Words as Attribute Names
**What goes wrong:** Attribute names like `role`, `name`, `status`, `content` are DynamoDB reserved words. Queries using them unescaped throw `ValidationException: Value provided in ExpressionAttributeNames must begin with '#'`.
**Why it happens:** DynamoDB has 573+ reserved words; common English words are often in the list.
**How to avoid:** Always use the expression builder (`expression.Name("role")`) — it automatically prefixes attribute names with `#` and adds them to `ExpressionAttributeNames`.
**Warning signs:** `ValidationException` containing "reserved keyword" in the error message.

### Pitfall 2: TransactWriteItems Item Limit
**What goes wrong:** TransactWriteItems has a hard limit of 100 items per transaction.
**Why it happens:** Phase 3 uses 3 items per AppendMessage — well within limits. But ConcurrentScenario spawns N goroutines each calling AppendMessage independently. Each AppendMessage is its own TransactWriteItems call (3 items). No batching of concurrent calls into a single TransactWriteItems.
**How to avoid:** N=10 and N=50 goroutines each making independent 3-item transactions. Total items per transaction never exceeds 3. No issue.
**Warning signs:** `TransactionCanceledException: Too many transact requests per request`.

### Pitfall 3: DynamoDB Provisioning Throttling
**What goes wrong:** If table is created with PROVISIONED mode at low RCU/WCU, the ConcurrentScenario causes throttling (`ProvisionedThroughputExceededException`).
**Why it happens:** Default provisioned capacity is 5 RCU / 5 WCU — easily exceeded by 50 concurrent goroutines.
**How to avoid:** Use `PAY_PER_REQUEST` billing (D-01 discretion area). No capacity planning needed. Research confirms this is the right choice for benchmark workloads with unpredictable concurrency.

### Pitfall 4: libsql-client-go Deprecation Warning at Build Time
**What goes wrong:** `go get` or `go build` may print a deprecation notice from the module (`Deprecated: use go-libsql instead`). This is a cosmetic warning, not a build failure.
**Why it happens:** The upstream maintainers prefer `go-libsql` but it requires CGO. `libsql-client-go` is still functional and maintained for remote-only use cases.
**How to avoid:** The deprecation is acknowledged in CLAUDE.md and CONTEXT.md D-07. Suppress or ignore the warning during development.
**Warning signs:** Build output contains "module is deprecated" but the binary still builds and runs correctly.

### Pitfall 5: Turso Schema Migration on Re-run
**What goes wrong:** Running `CREATE TABLE` on a Turso Cloud dev database that already has the schema causes an error and prevents the test/benchmark from running.
**Why it happens:** Unlike Postgres (fresh container each run) and DynamoDB (LocalStack fresh container), Turso Cloud is persistent.
**How to avoid:** Use `CREATE TABLE IF NOT EXISTS` in the Turso migrations/001_create_tables.sql. This is idempotent and matches the Postgres pattern.
**Warning signs:** `sql: error ... table already exists` on second run.

### Pitfall 6: LocalStack Container Cold Start
**What goes wrong:** LocalStack takes several seconds to initialize. The `localstack.Run` call returns before all services are ready, and the first `CreateTable` call fails with a connection error.
**Why it happens:** LocalStack loads services lazily. The testcontainers-go module includes a built-in wait strategy.
**How to avoid:** Use `localstack.Run(ctx, "localstack/localstack:3")` — the module's default wait strategy handles this. Do not add custom sleep. The `TableExistsWaiter` in `ensureTable` also provides a second safety net.

### Pitfall 7: GetItem Required to Resolve Old Conversation SK
**What goes wrong:** AppendMessage needs the old `conv_listing_sk` to delete the stale conversation item. If this value is not stored somewhere queryable, the only alternative is a Query to find it — which adds latency noise to the AppendMessage benchmark.
**Why it happens:** DynamoDB delete requires the full primary key (PK + SK). The old SK contains the old `updated_at` timestamp which is unknown at AppendMessage time.
**How to avoid:** Store a conversation metadata item at fixed SK `CONV#META` with PK=`CONV#<conv_id>`. This item holds `partner_id`, `user_id` (to reconstruct `user_pk`), `created_at`, `updated_at`, and `listing_sk` (the current conversation listing SK). AppendMessage does one GetItem on this metadata item, then issues TransactWriteItems with 4 items: (1) Put message, (2) Delete old listing item, (3) Put new listing item, (4) Update metadata item with new `updated_at` and `listing_sk`. This keeps the entire operation atomic.

---

## DynamoDB Item Schema (Resolved)

Based on the locked decisions and pitfall analysis, the complete item shapes are:

**Conversation Metadata Item** (for AppendMessage SK lookup):
```
PK:         CONV#<conv_id>
SK:         CONV#META
partner_id: <uuid string>
user_id:    <uuid string>
created_at: <ISO8601>
updated_at: <ISO8601>
listing_sk: <current CONV#<updated_at>#<conv_id> value>
```

**Conversation Listing Item** (for ListConversations):
```
PK:        USER#<partner_id>#<user_id>
SK:        CONV#<updated_at>#<conv_id>
conv_id:   <uuid string>
```

**Message Item** (for LoadWindow):
```
PK:              CONV#<conv_id>
SK:              MSG#<created_at>#<msg_id>
message_id:      <uuid string>
conversation_id: <conv_id string>
role:            user | assistant
content:         <string>
token_count:     <int as number string>
created_at:      <ISO8601>
```

**AppendMessage TransactWriteItems (4 items):**
1. Put: message item
2. Delete: old conversation listing item (PK=USER#..., SK=old listing_sk)
3. Put: new conversation listing item (PK=USER#..., SK=new listing_sk)
4. Update: conversation metadata item (PK=CONV#<conv_id>, SK=CONV#META) → set updated_at, listing_sk

**CreateConversation items written (2 items — no TransactWriteItems needed):**
1. PutItem: conversation metadata item (CONV#<conv_id> / CONV#META)
2. PutItem: conversation listing item (USER#... / CONV#<created_at>#<conv_id>)

These can be two independent PutItem calls since there is no stale SK to delete on creation.

---

## Turso Schema DDL (Resolved)

SQLite-compatible schema mirroring Postgres:

```sql
-- migrations/001_create_tables.sql (turso)
CREATE TABLE IF NOT EXISTS conversations (
    id         TEXT NOT NULL PRIMARY KEY,
    partner_id TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversations_user
    ON conversations (partner_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT NOT NULL PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT NOT NULL,
    token_count     INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_window
    ON messages (conversation_id, created_at DESC);
```

SQLite stores timestamps as ISO8601 `TEXT`. The `ORDER BY created_at DESC LIMIT N` pattern works correctly because ISO8601 sorts lexicographically in chronological order. No `RETURNING` clause needed — Turso adapter constructs the return value from the inserted data (same as Postgres adapter).

---

## Code Examples

### DynamoDB attributevalue struct definition

```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue
type msgRecord struct {
    PK             string `dynamodbav:"PK"`
    SK             string `dynamodbav:"SK"`
    MessageID      string `dynamodbav:"message_id"`
    ConversationID string `dynamodbav:"conversation_id"`
    Role           string `dynamodbav:"role"`
    Content        string `dynamodbav:"content"`
    TokenCount     int    `dynamodbav:"token_count"`
    CreatedAt      string `dynamodbav:"created_at"`
}

type convMetaRecord struct {
    PK        string `dynamodbav:"PK"`
    SK        string `dynamodbav:"SK"`
    PartnerID string `dynamodbav:"partner_id"`
    UserID    string `dynamodbav:"user_id"`
    CreatedAt string `dynamodbav:"created_at"`
    UpdatedAt string `dynamodbav:"updated_at"`
    ListingSK string `dynamodbav:"listing_sk"`
}

type convListRecord struct {
    PK     string `dynamodbav:"PK"`
    SK     string `dynamodbav:"SK"`
    ConvID string `dynamodbav:"conv_id"`
}
```

### Unmarshal query results

```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue
var records []msgRecord
if err := attributevalue.UnmarshalListOfMaps(result.Items, &records); err != nil {
    return nil, err
}
```

### Turso ListConversations query

```go
// Source: CONTEXT.md D-08, D-09 — raw database/sql, same pattern as Postgres
rows, err := r.db.QueryContext(ctx,
    `SELECT id, partner_id, user_id, created_at, updated_at
     FROM conversations
     WHERE partner_id = ? AND user_id = ?
     ORDER BY updated_at DESC`,
    partnerID.String(), userID.String(),
)
```

SQLite uses `?` placeholders (not `$1`). The libsql driver follows the standard `database/sql` interface.

---

## Runtime State Inventory

This is a greenfield implementation (new packages, new dependencies). No rename or migration involved.

**Stored data:** None — DynamoDB LocalStack is ephemeral (test only). Turso Cloud dev DB is persistent but empty until benchmarks run. No pre-existing data to migrate.
**Live service config:** None — LocalStack is managed by testcontainers-go. Turso Cloud dev DB connection is via env vars.
**OS-registered state:** None.
**Secrets/env vars:** `TURSO_URL` and `TURSO_AUTH_TOKEN` are new env vars. They are optional — tests skip if absent.
**Build artifacts:** New packages require `go get` to update go.mod and go.sum. No compiled binaries to clean up.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | LocalStack (DynamoDB tests) | Yes | 28.2.2 | — |
| Go | All compilation | Yes | 1.26.0 | — |
| `TURSO_URL` env var | Turso integration tests | Not checked (CI/local) | — | Tests skip gracefully (D-12) |
| `TURSO_AUTH_TOKEN` env var | Turso integration tests | Not checked | — | Tests skip gracefully (D-12) |
| AWS credentials env vars | DynamoDB production (not LocalStack) | Not required for tests | — | LocalStack uses static fake creds |

**Missing dependencies with no fallback:** None — Docker is present and LocalStack test container will work.

**Missing dependencies with fallback:** Turso env vars. No env vars = tests skip with informational message. Benchmark will also skip Turso backend if env vars absent per D-14.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| aws-sdk-go v1 | aws-sdk-go-v2 (locked in CLAUDE.md) | 2023 (v1 to maintenance-only) | Must use v2; v1 API is completely different |
| DynamoDB raw map construction | expression builder + attributevalue | Since SDK v2 GA | Eliminates reserved-word errors, struct-tag marshaling |
| `sql.Open("libsql", url?authToken=...)` | Same (no change) | Stable | libsql-client-go uses standard sql.Open with token in URL |

**Deprecated/outdated:**
- `aws-sdk-go` (v1): maintenance-only since 2023; do not use for new code
- `lib/pq` Postgres driver: maintenance-only; not applicable to this phase but noted for context
- `libsql-client-go`: deprecated upstream in favor of `go-libsql`; functional for remote-only use; CGO requirement of go-libsql makes it unsuitable for this project (locked in CLAUDE.md)

---

## Open Questions

1. **Conversation metadata GetItem latency impact on AppendMessage benchmark**
   - What we know: AppendMessage requires a GetItem (to read `listing_sk`) before TransactWriteItems. This adds one extra round-trip to LocalStack/DynamoDB per AppendMessage call.
   - What's unclear: Whether to eliminate this by passing the old SK as a parameter to AppendMessage (interface change — NOT allowed per locked interface) or accept the extra round-trip as an inherent DynamoDB cost that the benchmark should measure.
   - Recommendation: Accept the extra round-trip. It is a real cost of this DynamoDB schema design and measuring it is part of the benchmark's value. Do not change the ChatRepository interface (IFACE-03 locks it).

2. **LocalStack version selection**
   - What we know: `localstack/localstack:3` is a recent stable tag. The testcontainers-go module examples show version `1.4.0` in some docs but `3` is current.
   - What's unclear: Whether the latest `localstack/localstack:latest` or a pinned version is preferred.
   - Recommendation: Use `localstack/localstack:3` (major version tag). Pinning to a minor version (`:3.x.y`) is more reproducible but adds maintenance. Major version tag is a pragmatic choice for a benchmark harness.

---

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb) — QueryInput, TransactWriteItems, CreateTable structures
- [pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression) — KeyConditionBuilder, Builder, expression patterns
- [pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue) — MarshalMap, UnmarshalListOfMaps, dynamodbav struct tags
- [docs.turso.tech/sdk/go/quickstart](https://docs.turso.tech/sdk/go/quickstart) — libsql-client-go sql.Open pattern, authToken URL format
- [golang.testcontainers.org/modules/localstack](https://golang.testcontainers.org/modules/localstack/) — LocalStack Run, MappedPort, DynamoDB client setup
- [docs.aws.amazon.com/code-library/.../go_2_dynamodb_code_examples.html](https://docs.aws.amazon.com/code-library/latest/ug/go_2_dynamodb_code_examples.html) — CreateTable with PAY_PER_REQUEST, TableExistsWaiter
- CLAUDE.md §Technology Stack — locked driver versions and rationale
- `.planning/phases/03-dynamodb-turso-adapters/03-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)
- [aws.amazon.com/blogs/database/working-with-date-and-timestamp-data-types-in-amazon-dynamodb](https://aws.amazon.com/blogs/database/working-with-date-and-timestamp-data-types-in-amazon-dynamodb/) — ISO8601 sort key recommendation (verified: ISO8601 is sortable as UTF-8)
- [pkg.go.dev/github.com/tursodatabase/libsql-client-go](https://pkg.go.dev/github.com/tursodatabase/libsql-client-go) — deprecation status, NewConnector, WithAuthToken option

### Tertiary (LOW confidence)
- [guedes.hashnode.dev/integration-test-with-testcontainer-and-localstack](https://guedes.hashnode.dev/integration-test-with-testcontainer-and-localstack) — Community blog example for LocalStack + Go pattern (pattern verified by official docs)

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — all versions verified on pkg.go.dev against live registry; aws-sdk-go-v2 is the official AWS Go SDK; testcontainers-go v0.41.0 is already in project
- Architecture (DynamoDB schema): HIGH — locked in CONTEXT.md with explicit decisions D-01 through D-06; patterns verified against official AWS SDK docs
- Architecture (Turso): HIGH — sql.Open pattern verified against official Turso docs; SQLite-compatible DDL is established practice
- LocalStack setup: MEDIUM-HIGH — official testcontainers-go module docs confirmed; DynamoDB-specific example derived from S3 pattern (same API)
- Pitfalls: HIGH — reserved word issue is documented by AWS; TransactWriteItems limit from official docs; libsql deprecation from pkg.go.dev

**Research date:** 2026-03-31
**Valid until:** 2026-04-30 (aws-sdk-go-v2 releases frequently but API is stable; libsql-client-go is pre-v1 but no breaking changes expected for remote-only use)
