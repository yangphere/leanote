# Database Guidelines

> MongoDB access patterns in the Go backend.

---

## Overview

- Driver: `go.mongodb.org/mongo-driver/v2`. Connection happens only in `app/db/mongo_client.go` `dialMongo`, which always applies `lea.CodecRegistry` + `BSONOptions{DefaultDocumentM: true}` (blog theme templates decode untyped documents as `bson.M`; dropping these options breaks them).
- Timeouts come from config keys `db.connectTimeoutMs` / `db.operationTimeoutMs` (defaults 10s/15s); an invalid value is a startup fatal (`timeoutConfigValue`), never a silent fallback.
- `db.InitWithError(url, dbname)` in `app/db/Mgo.go` is the only init entry: it dials and pings before assigning the package client. A nil client keeps `/healthz` at 503 — never fake readiness.
- Shared MongoDB helpers must fail closed when the package client has not been initialized. Return an explicit error before dereferencing the client; do not silently return an empty result or create an implicit connection.
- Read-only test-support queries (for example, `e2e_runs` identity markers) use the current application database session and must propagate connection and query errors to the caller.

## Collections and data rules

- Collection wrappers are declared at the bottom of `app/db/Mgo.go` (`Notebooks`, `Notes`, `NoteContents`, `NoteContentHistories`, `ShareNotes`, ...). Controllers never touch `client.Database(...)` directly — go through `app/service`.
- `app/domain.ObjectID` is the canonical ID type for domain models; it serializes as a plain BSON ObjectId only through the `app/db` adapter, and its zero value JSON/Hex is `""`. `lea.ObjectID` is a migration-only alias and must not be reintroduced into `app/info`.
- Update semantics: `splitUpdateKind` splits operator-style vs replacement-style updates; `UpdateAll` does not split. The archived revel-migration design (§6.1 supersession note) is the authority.
- USN counters and note-content history invariants are enforced in `app/service`; the `leanote_test` fixture (`mongodb_backup/leanote_install_data`, exactly 2 users — the harness verifies this count after every restore) is the test baseline.

## Test environments (three-mode contract)

`app/tests/harness/environment.go` `ResolveMongoTestMode` selects exactly one mode per process — mixing sources fails closed:

1. `LEANOTE_REQUIRE_MONGO=1` ⇒ service-backed: consume `mongodb://127.0.0.1:27017/leanote_test` (or a `LEANOTE_TEST_MONGO_URL` override whose database name must be exactly `leanote_test`); **zero docker calls**; per-test host `mongorestore --drop`.
2. Unset ⇒ self-provisioned: `MongoEnvironment.Up()`/`Down()` own the `leanote-test-mongo` container per test.
3. The e2e supervisor (`app/tests/harness/cmd/e2e/main.go`) is always self-provisioned and asserts port 27017 is free before starting; any service declaration (`LEANOTE_REQUIRE_MONGO`/`LEANOTE_TEST_MONGO_URL`) is rejected.

The database name is fixed to `leanote_test` in every mode. Do not add a second fixed port or a probe-based fallback.

## Production config

The packaged binary only accepts `/etc/leanote/app.conf` (mode 0440, regular file — `app/httpserver/production_config.go`) with the exact placeholders `db.urlEnv=${MONGODB_URL}` and `app.secret=${LEANOTE_APP_SECRET}`; hosts must not be localhost/loopback literals (CI uses a hosts alias like `mongo-smoke.internal`).

## Migrations

There is no migration framework. Schema changes ship as code changes plus fixture updates (`mongodb_backup/leanote_install_data` is restored by tests and CI); compatibility is protected by the Golden replay suite (`LEANOTE_GOLDEN=replay`) and the archived per-task contract tests.

## Common Mistakes

- Initializing a second Mongo client or dialing from a controller — breaks the nil-client fail-closed path and the three-mode test contract.
- Masking a database error as empty data; the caller must receive the error (see `app/httpserver` handlers and `error-handling.md`).
- Unit tests must keep covering the uninitialized-client path (non-nil error asserted) and identity tests must distinguish a missing/invalid marker from a database error — both fail closed, neither exposes credentials.

## Scenario: Multi-step initialization with transaction fallback

### 1. Scope / Trigger

- Trigger: a service operation creates a primary record plus required side effects such as default notebooks, share rows, copied note metadata, action tokens, or outbox events.
- Ownership: `app/db.ExecuteUserInitialization` owns the transaction/compensation runner; the service owns step construction and any cleanup inside one step that performs multiple writes.

### 2. Signatures

```go
type InitializationStep struct {
    Name       string
    Apply      func(context.Context) error
    Compensate func(context.Context) error
}

func ExecuteUserInitialization(ctx context.Context, plan UserInitializationPlan, transaction TransactionRunner) (UserInitializationResult, error)
```

### 3. Contracts

- A step is considered applied only after its `Apply` returns `nil`; the generic compensation loop cannot see partial writes made inside a failed `Apply`.
- Multi-write steps must either split into one write per step or clean their own already-applied writes before returning the failure.
- Outbox enqueue is part of the durable success boundary. A failed enqueue returns `ErrSideEffect`; a partial initialization returns `ErrPartialWrite`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Transaction succeeds | `Committed=true`, no compensation |
| Transaction unsupported before writes | compensation mode may apply idempotent steps |
| Step N fails after prior steps completed | prior completed steps are compensated in reverse order |
| Step N performs write A, then write B fails | step N cleans write A itself or is split so write A has its own `Compensate` |
| Outbox enqueue fails | return `ErrSideEffect`, do not report user registration success |
| Compensation cleanup fails | return `ErrPartialWrite` with observable failure context |

### 5. Good / Base / Bad Cases

- Good: registration `shared_resources` deletes any `has_share_notes` / `share_*` rows it wrote if a later share insert in the same step fails.
- Base: a single insert step can rely on its `Compensate` because it either finishes or fails before being marked applied.
- Bad: looping over many copied notes inside one `Apply` and returning on the second failure while leaving the first copied note behind.

### 6. Tests Required

- Contract tests for transaction success, transaction-unsupported fallback, partial failure compensation, outbox `ErrSideEffect`, and transient transaction errors.
- Service tests for every multi-write step proving an in-step failure removes the writes completed earlier in the same step.
- Persistence tests must keep real Mongo transaction and standalone fallback behavior separate; do not treat focused mocks as live Mongo evidence.

### 7. Wrong vs Correct

#### Wrong

```go
Apply: func(ctx context.Context) error {
    for _, item := range items {
        if err := insert(ctx, item); err != nil {
            return err // earlier inserts in this same step are invisible to generic compensation
        }
    }
    return nil
}
```

#### Correct

```go
Apply: func(ctx context.Context) error {
    applied := make([]Item, 0, len(items))
    for _, item := range items {
        if err := insert(ctx, item); err != nil {
            _ = remove(ctx, applied)
            return err
        }
        applied = append(applied, item)
    }
    return nil
}
```

## Scenario: Framework-neutral ObjectID and dynamic JSON boundaries

### 1. Scope / Trigger

- Trigger: domain models must be usable without Revel or the Mongo driver while existing Mongo documents and API JSON remain byte-compatible.
- Ownership: `app/domain` owns zero/parse/hex/JSON behavior; `app/db` owns BSON conversion and legacy string reads; `app/lea` only carries the temporary compatibility alias.

### 2. Signatures

```go
func ParseObjectID(value string) (ObjectID, error)
func (id ObjectID) Hex() string
func (id ObjectID) IsZero() bool
func (id ObjectID) MarshalJSON() ([]byte, error)
func (id *ObjectID) UnmarshalJSON(data []byte) error
```

### 3. Contracts

- `ParseObjectID` accepts `""` as zero and exactly 24 hexadecimal characters (case-insensitive); `Hex` emits lowercase or `""` for zero.
- JSON encoding emits `""` for zero and lowercase 24-character hex otherwise. JSON `null` decodes to zero; numbers, objects, arrays and malformed strings fail.
- BSON encoding is configured through the shared `CodecRegistry` at the database boundary. Decoding accepts BSON ObjectId, legacy empty string and legacy 24-character hex; other BSON types/strings fail.
- `Re.List`/`Item` and `Theme.Info` remain JSON-compatible dynamic values. Their `MarshalJSON` methods validate once at the boundary and preserve nil (`null`) versus empty collection (`[]`/`{}`) shape.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| empty ObjectID text or JSON `null` | zero value, no error |
| valid 24-character hex | parsed ID; JSON output is lowercase |
| malformed ID / JSON number/object/array | `ErrInvalidObjectID`-wrapped error |
| BSON ObjectId / legacy empty or hex string | decoded ID |
| unknown BSON type or invalid legacy string | explicit decoder error; never zero fallback |
| non-JSON dynamic value (function, channel, NaN, cycle) | marshal fails with `Re.List`, `Re.Item` or `Theme.Info` field context |

### 5. Good / Base / Bad Cases

- Good: `app/info` imports only standard library plus `app/domain`; `app/db` configures `CodecRegistry` on every driver client/encoder.
- Base: migration callers may use the `lea.ObjectID` alias while downstream packages are being migrated.
- Bad: adding a BSON method or Mongo import to `app/domain`, using `bson.Marshal` without the shared registry, or silently converting invalid dynamic values to `null`.

### 6. Tests Required

- Domain unit tests assert zero/hex/JSON/null/invalid-ID behavior and dynamic-value field-context errors without Mongo or Revel dependencies.
- Database adapter tests assert BSON ObjectId encoding, legacy empty/hex reads, unknown-type failures, and operator-update classification using the shared registry.
- Dependency checks assert both `go list -deps ./app/info` and `go list -deps -test ./app/info` contain neither Revel nor the Mongo driver.

### 7. Wrong vs Correct

#### Wrong

```go
type ObjectID [12]byte // with BSON methods in app/domain
```

#### Correct

```go
// app/domain: JSON/hex only
type ObjectID [12]byte

// app/db: configure the adapter registry for Mongo operations
encoder.SetRegistry(CodecRegistry)
```
