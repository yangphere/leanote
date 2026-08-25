# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

- For HTTP regression work, exercise the real server boundary; do not call a
  controller directly.
- Keep `LEANOTE_GOLDEN=replay` read-only and fail on a missing or mismatched
  snapshot. Only an explicit `LEANOTE_GOLDEN=record` may write snapshots.
- Run the legacy Revel generator with the explicitly selected Go 1.20.14
  executable through `LEANOTE_TEST_GO`; newer Go versions can panic in the
  pinned `x/tools` type checker.
- Use MongoDB 5.0 for the `mgo.v2` baseline fixture. Restore the fixture before
  integration tests and remove the named container afterward.
- Treat `Content-Type` and `Location` as the only comparable HTTP headers for
  JSON responses; reject headers outside the documented comparison/exclusion
  sets. Binary snapshots compare non-empty body presence and stable headers,
  not machine-dependent bytes.
- Keep baseline changes out of production packages: `app/` changes are limited
  to `app/tests/`, plus the test run-mode section in `conf/app.conf` and the
  explicitly tracked regression workflow.

## HTTP Baseline Contract

### 1. Scope / Trigger

The contract applies when adding or updating the legacy HTTP Golden, USN, smoke,
or Mongo fixture harness under `app/tests/harness`.

### 2. Signatures

- `go run ./app/tests/harness/cmd/env up|down`
- `LEANOTE_GOLDEN=record|replay go test -p 1 ./app/tests/... -count=1 -timeout 30m`
- `LEANOTE_TEST_GO=<Go 1.20.14 executable>` for generated Revel server entrypoints

### 3. Contracts

- `up` starts `mongo:5.0` as `leanote-test-mongo`, restores into
  `leanote_test`, and verifies two fixture users; any setup failure after
  container creation must remove the named container before returning.
- Replay reads `app/tests/golden/**/*.json` and never creates or rewrites files.
- Record stores normalized request/response snapshots; dynamic ObjectId and
  timestamp replacement is field-scoped and preserves JSON key order.
- The test server binds only to loopback (`http.addr=127.0.0.1`), listens on
  fixed port `28017`, and uses the `[test]` config section with
  `site.url=http://127.0.0.1:28017` so Windows does not expose the generated
  test executable to public/private network firewall prompts.
- The configuration guard parses global and `[test]` values with section
  precedence, removes inline `#`/`;` comments, expands `${VAR}` values, and
  rejects empty or unresolved `db.url`/`db.urlEnv` values. Any active URL must
  be a single line and resolve to the isolated `leanote_test` database through
  both the supported Mongo URL parser and the legacy `db.Init` path-segment
  fallback before the server starts. If the URL has no database segment,
  mirror `db.Init`'s fallback to the already-validated `[test] db.dbname`;
  unknown options may only be accepted when the legacy path is still isolated.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing/invalid `LEANOTE_GOLDEN` | replay by default; invalid value fails |
| Missing or mismatched replay file | test failure; no write |
| Missing `LEANOTE_TEST_GO` for server generation | explicit failure |
| Port 28017 occupied | explicit failure; no random fallback |
| Unknown response header | normalization failure |
| Missing ExportPdf golden or unavailable wkhtmltopdf in replay | explicit skip with a message to run the Linux record job; record mode fails |
| Windows `ExportPdf` | the same tool/golden guard skips; Linux record job owns the first golden |
| Test config missing, test server address is not loopback, a database URL uses continuation/newline syntax, or database URL points outside `leanote_test` | explicit failure before server startup |
| Legacy `TestAuth` without MongoDB in CI | `LEANOTE_REQUIRE_MONGO=1` makes the independent auth step fail |
| Binary response has JSON Content-Type | record/replay assertion fails |

### 5. Good / Base / Bad Cases

- Good: restore Mongo 5.0, run real HTTP requests, replay unchanged snapshots.
- Base: run pure normalization/store tests without Mongo.
- Bad: call controllers directly, auto-record during replay, or silently choose
  another Go/Mongo/port configuration.

### 6. Tests Required

- Unit tests assert normalization, header closure, record/replay write
  protection, multipart requests, configuration isolation, and fixed-port/toolchain guards.
- Integration tests assert the 29 distributable API actions, failure envelopes,
  USN mutation pairs/boundaries, seven ownership-sensitive web controllers,
  admin/member JSON smoke, and page status/HTML markers.

### 7. Wrong vs Correct

```text
Wrong: LEANOTE_GOLDEN is unset and a missing snapshot is generated.
Correct: unset means replay; a missing snapshot fails and asks for explicit record.
```
