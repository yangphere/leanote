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
- `lea.ObjectID` is the canonical ID type; it serializes as a plain BSON ObjectId, and its zero value JSON/Hex is `""`.
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
