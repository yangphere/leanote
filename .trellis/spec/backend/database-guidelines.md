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

## Scenario: Durable standalone mutation receipts

### 1. Scope / Trigger

- Trigger: a multi-step mutation can commit a database write while the caller loses the result, or a subsequent verification read is temporarily unavailable.
- Ownership: `app/application/notes` owns the receipt state machine; `app/db/workspace_operation_store.go` persists its owner-scoped, version-fenced receipts. Services construct immutable plans and do not create a second receipt from changed current generations.

### 2. Signatures

```go
func NewClientOperationIdentity(kind string, ownerID domain.ObjectID, clientOperationID string) (string, error)
func ExecuteStandalone(ctx context.Context, plan MutationPlan, store OperationStore, now time.Time) (MutationResult, error)
```

### 3. Contracts

- An optional caller `OperationId` is an owner-and-kind-scoped stable receipt key. The complete canonical request, including that ID, remains in `InputDigest`; reusing the same ID with different input conflicts.
- A caller-supplied `OperationId` still creates and commits a receipt when the canonical command is a no-op. It consumes no business USN, but freezes the original result so the same input replays and different input conflicts. Only legacy calls without `OperationId` may return a no-op without receipt state.
- A receipt freezes before/desired state and assigned per-step USNs on first execution. Retries address that receipt and resume from its frozen state; they never derive a replacement key from generations observed after a partial write.
- A committed required-write receipt is not proof that a repairable projection completed. Wrapper flows such as copy/shared-copy must re-enter the single create/repair runner or verify the separate projection receipt before reporting retry success.
- Stable shared-copy freezes the ordered source image/attachment identity manifest in the root receipt before destination creation. A retry consumes that manifest and the committed destination content snapshot; it must not re-enumerate later source assets into the original operation.
- Legacy callers without an `OperationId` retain the existing wire contract and receive no unknown-result retry guarantee.
- If `Apply` returns an error and `Verify` cannot determine whether it applied, persist `repair_pending` with `CurrentStep`, before state, desired state, and assigned USNs intact. A retry must verify first and must not blindly repeat the apply.
- Only terminal receipts are redacted. `repair_pending` is recoverable and must not be redacted as a failed operation.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Same owner/kind/OperationId and same canonical input | return/recover the original receipt |
| Same owner/kind/OperationId and different input | conflict before business writes |
| Caller OperationId with a no-op command | commit a zero-write receipt and freeze the original result; do not allocate USN |
| Required receipt committed but projection receipt missing/pending | resume or verify the projection; do not report clean success |
| Shared-copy source gains an image/attachment after the root receipt begins | retry only the frozen manifest; do not copy the later asset under the old `OperationId` |
| Apply error, Verify confirms desired state | record the step and continue/commit |
| Apply error, Verify returns unavailable/error | `repair_pending` plus `partial_write`; preserve recovery state |
| Retry of `repair_pending` | verify frozen current step before any replay |
| Committed receipt no longer matches current resources | fail closed; do not replay an old mutation |

### 5. Good / Base / Bad Cases

- Good: a notebook reorder retry supplies the original `OperationId`, loads its receipt, and observes the already-assigned per-notebook USNs without allocating again.
- Base: an old notebook caller omits the ID and retains its prior non-retry-safe behavior.
- Bad: a copy wrapper sees the destination row plus a committed create receipt and returns before the separate tag/count/image projection receipt is committed.
- Bad: a shared-copy retry enumerates the current source attachments after the root receipt committed and silently adds newly discovered assets to the old operation.
- Bad: a client-generated no-op returns before `Begin`, allowing the same `OperationId` to be reused later with a different command.
- Bad: creating an ID from the request for lookup, then creating a second ID from the request plus current generations for storage; the retry can never find the stored receipt.

### 6. Tests Required

- Unit test production-style terminal redaction semantics: an Apply+Verify error leaves a `repair_pending` receipt whose next call only verifies and commits without another Apply.
- Service tests assert same `OperationId` has a stable receipt ID/digest, changed input conflicts, and before-state retains original generations.
- Service tests cover caller-generated no-op replay/conflict without a USN allocation, plus copy/shared-copy retries where the primary create receipt committed but the projection receipt must still be repaired.
- Service tests prove stable shared-copy persists/consumes the frozen image/attachment manifest and excludes assets added to the source after the first attempt; legacy no-`OperationId` copy retains its historical ordering and USN behavior.
- Mongo-backed tests, when the configured fixture is available, cover response-loss replay and partial resume without duplicate step USNs or writes. If unavailable, skip explicitly rather than treating the unit test as cross-process evidence.

### 7. Wrong vs Correct

#### Wrong

```go
if verifyErr != nil {
    return failReceipt(ctx, result, receipt, store, step.Name, verifyErr)
}
```

#### Correct

```go
if verifyErr != nil {
    return pendingStep(ctx, result, receipt, store, step.Name, verifyErr)
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
