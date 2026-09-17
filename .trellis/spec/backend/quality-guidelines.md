# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Go 1.26 monolith, standard `testing`, no linter beyond `gofmt`/`go vet` — the gates are the CI quality jobs (node-build/chromium-e2e/mongo-8_0 + go-1_26_7/go-1_27_0) and the contract tests under `tests/js/`. Quality = green CI on the real boundaries plus focused regression cases per fix (AGENTS.md testing rules).

## Forbidden Patterns

- Revel imports: `rg 'github.com/revel|revel\.' app go.mod sh conf` must stay zero hits (go.sum pongo2 hash is the recorded exception).
- Probe-based or fallback environment selection (Mongo mode, toolchain) — fail closed instead (see `database-guidelines.md`).
- Log-and-return at every layer; masking db/service errors as empty success (`error-handling.md`).
- Hand-editing generated assets: anything produced by `scripts/build/manifest.mjs` (`public/js/app.min.js`, `public/tinymce/**` bundles, `app/views/note/note.html`) — regenerate via `npm run build`.
- CI shell scripts using PCRE-only regex syntax with `grep -E` (e.g. `(?:...)` — GNU grep ERE never matches it; the B-E6 root cause).

## Required Patterns

- `gofmt` before commit; run `go vet ./app/tests/harness/...` for harness changes; `go build ./...` must pass.
- Fake-injection for environment-dependent tests: function fields (`run`, `now`, `sleep`, `lookPath`, `ping`, `verifyFixture`) on the environment struct with nil-guard defaults — see `app/tests/harness/environment.go` and its tests.
- Windows-safe shell: `set -eu` scripts guard expansions (`${GITHUB_REF:-}`); MSYS path pitfalls handled with `MSYS_NO_PATHCONV` where needed.
- Failure diagnostics preserved: smoke scripts dump app log tail / response headers / docker logs on failure (original-cause requirement).

## Testing Requirements

- Every fix ships a focused regression case (AGENTS.md). Real server boundary for HTTP work — never call a controller directly.
- Mongo-backed tests run under the three-mode harness; `LEANOTE_GOLDEN=replay go test -p 1 ./app/tests/... -count=1 -timeout 30m` stays read-only; only `LEANOTE_GOLDEN=record` writes.
- A test that needs an uninitialized global Mongo collection saves it, assigns `nil`, and restores it with `t.Cleanup`; it must not infer that earlier same-package tests did not initialize `db.Notes` or another collection. Tests that mutate package-global collections must not call `t.Parallel`.
- Node contract suite `npm test` covers build closure, i18n scanner, release/summary/browser-evidence contracts — extend it when touching `scripts/` tooling.

---

## Code Review Checklist

<!-- What reviewers should check -->

- For HTTP regression work, exercise the real server boundary; do not call a
  controller directly.
- Keep `LEANOTE_GOLDEN=replay` read-only and fail on a missing or mismatched
  snapshot. Only an explicit `LEANOTE_GOLDEN=record` may write snapshots.
- Run the legacy Revel generator with a Go toolchain of at least 1.26.7. The
  harness resolves `go` from PATH by default and fails closed below that floor;
  `LEANOTE_TEST_GO` is an optional explicit override, and every generation or
  build subprocess runs with `GOTOOLCHAIN=local` (no automatic toolchain
  downloads).
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
- Generated Revel server entrypoints use the default `go` on PATH, enforced to be at least 1.26.7 (fail closed); `LEANOTE_TEST_GO` is an optional explicit override that bypasses the floor check

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
| Missing, older, or unreadable default `go` for server generation | explicit failure before any generation; `LEANOTE_TEST_GO` overrides |
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

## Scenario: Go 1.26 Travis Revel CLI

### 1. Scope / Trigger

When a Travis job running Go 1.26+ invokes `sh/run.sh` or `sh/package.sh`, the
Revel executable must be built from Leanote's main module graph. A versioned
`go install github.com/revel/cmd/revel@v1.0.3` instead resolves Revel's frozen
2020 `x/tools` dependency and panics during type checking (evidenced 2026-08-26;
since the Revel 1.1 upgrade the isolated `go install ...@v1.1.2` graph happens
to build, but the module-graph build with metadata assertion stays canonical).

### 2. Signatures

```sh
export PATH="$PATH:$HOME/gopath/bin"
export GOTOOLCHAIN=local
go build -o "$HOME/gopath/bin/revel" github.com/revel/cmd/revel
go version -m "$HOME/gopath/bin/revel" | grep -E 'github\.com/revel/cmd[[:space:]]+v1\.1\.2'
go version -m "$HOME/gopath/bin/revel" | grep -E 'golang.org/x/tools[[:space:]]+v0\.49\.0'
```

### 3. Contracts

- The executable path is `$HOME/gopath/bin/revel`, the same PATH entry used by
  `sh/run.sh` and `sh/package.sh`.
- The main `go.mod` selects `github.com/revel/cmd v1.1.2` (Revel runtime
  v1.1.0 since the 2026-08-28 C-a upgrade) and `golang.org/x/tools v0.49.0`;
  the binary metadata checks prove that selected dependency graph reached the
  executable.
- `GOTOOLCHAIN=local` prohibits the CLI build from silently downloading a
  different Go toolchain.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Module-aware build fails | Travis install fails with the original non-zero exit |
| Metadata lacks x/tools v0.49.0 | `grep` fails and scripts do not start |
| `revel version` fails | Travis install fails before Mongo restore or smoke requests |
| `revel run` or `revel package` fails | Keep the command failure; do not fall back to stock install |

### 5. Good / Base / Bad Cases

- Good: build the CLI from the checked-out Leanote module, inspect its build
  metadata, then let both shell entrypoints resolve that binary through PATH.
- Base: run `revel version` after metadata validation.
- Bad: append `@v1.0.3` to `go install`, use a separate temporary module, or
  ignore a CLI failure and continue to curl the server.

### 6. Tests Required

- Build `github.com/revel/cmd/revel` with `GOTOOLCHAIN=local` from the repository
  root and assert `go version -m` contains `golang.org/x/tools v0.49.0`.
- Run `revel version`; Linux entrypoint validation must exercise the same binary
  with `sh/run.sh` and `sh/package.sh`.

### 7. Wrong vs Correct

```text
Wrong:   go install github.com/revel/cmd/revel@v1.0.3
Correct: go build -o "$HOME/gopath/bin/revel" github.com/revel/cmd/revel
```

## Scenario: Content roots, deletion recovery, and self-contained PDF export

### 1. Scope / Trigger

Use this contract for filesystem-backed content, a deletion that must survive
process loss, or a note-to-PDF boundary. `app/application/content` owns logical
paths, manifest state, and renderer input validation; `app/service/contentfs`
owns absolute paths and durable filesystem operations; controllers only map the
authenticated request and the application result. This prevents a controller
or a renderer subprocess from becoming an alternate filesystem/network owner.

### 2. Signatures

```go
roots, err := contentfs.ValidateContentRoots(contentfs.ContentRootsConfig{ /* pairs + temporary + served roots */ })
manifest, err := content.NewDeleteManifest(identity, source, now)
err = store.CompareAndSwap(ctx, lookupKey, version, digest, next)
document, err := content.SerializeSelfContainedPDF(content.PDFDocumentRequest{ /* authorized resources */ })
err = service.InitContentRuntime(service.ContentRoots{ /* explicit pairs + temporary + served roots */ })
result, err := repair.FinalizePreNote(ctx, identity)
```

- `ContentRootsConfig` supplies private and public `DurableRootConfig` pairs,
  a temporary root, and every HTTP-served root.
- `DeleteIdentity.LookupKey` is derived from action/owner/kind/asset; only an
  explicit operation ID, or legacy generation plus content digest, can produce
  its `OperationKey`.
- `PDFDocumentRequest.Resources` is an already-authorized map of bounded image
  bytes. It has no URL fetcher, callback URL, or credential field.
- `PreNoteAssetIdentity` is the action/owner/record-owner/parent-note/kind/asset
  lookup for an API multipart asset before the parent note has a committed
  generation. Its generated `CreateIdentity` uses `Generation: 0` and
  `PreNote: true`.

### 3. Contracts

- Roots must already be absolute, accessible directories after symlink
  canonicalization. Data, quarantine, temporary, and served roots cannot
  overlap; each data/quarantine pair must support an atomic rename on one
  filesystem; quarantine cannot be HTTP-served. Failure prevents
  `cmd/leanote` from listening.
- A deletion writes a generic content manifest before irreversible work. State
  progresses only `prepared -> quarantined -> metadata_mutated -> terminal`;
  every transition increments `Version` and recomputes `StateDigest`. The
  terminal record clears source, quarantine, and content digest. It is not a
  notes receipt and must not own USN or history.
- Manifests are staged at `0600`, published without replacement or atomically
  replaced under an owner-scoped lease, directory-synced, and read with a 1 MiB
  limit. Terminal records are retained for seven days before GC.
- The serializer strips user-controlled active elements, event/style/form
  attributes, and unapproved external references. It may emit only the pinned
  completion script and `data:image/...` values created from validated,
  authorized resources; validate again immediately before process execution.
- A pre-note manifest remains active after exact row and byte verification.
  A parent-note lookup under the same owner decides it: an existing parent may
  call `FinalizePreNote`; an absent parent must use the ordinary owner-scoped
  delete manifest/quarantine/metadata-delete/purge flow before
  `DiscardPreNote`. Generic create recovery scans non-pre-notes only; the API
  action scans `api_note_asset_upload` pre-notes with its own bounded budget.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing, relative, inaccessible, overlapping, served, or cross-filesystem root | initialization fails closed; no listener fallback |
| Missing operation ID with missing legacy generation/digest | `ErrorValidation` (`legacy_delete_identity_incomplete`) |
| Canceled manifest operation | `ErrorTimeout`; do not report a successful delete |
| Existing lease, stale CAS, malformed/oversized manifest, or root ambiguity | typed `ErrorConflict`; recovery must not guess |
| Unsupported no-replace/replace/sync filesystem primitive | typed `ErrorUnsupportedFS` or `ErrorUnknownResult`; caller verifies before retry |
| Empty/oversized/invalid PDF resource or invalid image bytes | typed resource/media error; no renderer start |
| User HTML containing script, SVG, refresh, external URL, or active attribute | remove untrusted content or reject the final document; never fetch it |
| Pre-note manifest whose parent exists but row/file digest does not verify | conflict/dependency; do not terminalize or delete |
| Pre-note manifest whose parent is absent | delete lifecycle then discarded terminal; row/projection reference blocks cleanup |
| Unknown pre-note action in startup scan | leave untouched; it must not consume the API action scan budget |

### 5. Good / Base / Bad Cases

- Good: initialize both durable pairs before HTTP registration; create and CAS
  a manifest around quarantine/metadata repair; render only a validated,
  self-contained document through direct argv and stdin.
- Good: publish an API asset as pre-note, verify it only after the matching
  parent note exists, and run a separate action-filtered startup repair for an
  abandoned parent.
- Base: a terminal manifest contains recovery identity and timestamp but no
  path or content digest; a note with no approved embedded resource still
  exports after sanitization.
- Bad: call `os.Remove` from a controller or service before durable recovery
  state exists; recover from a corrupt manifest by deleting a guessed path; let
  `wkhtmltopdf` resolve `http`, `file`, CSS, SVG, or callback resources.
- Bad: let generic create recovery terminalize every exact pre-note row, or
  let a pre-note-only backlog consume the ordinary create-repair scan budget.

### 6. Tests Required

- Root tests cover canonicalization, all overlap directions, served-root
  exclusion, same-filesystem probe failure, and startup error propagation.
- Manifest tests cover explicit and legacy identities, every valid transition,
  terminal scrubbing, stale CAS, held lease, cancellation, oversized/trailing
  JSON, root mismatch, sync failure, and seven-day terminal GC.
- PDF tests cover Markdown and HTML serialization, resource MIME/size/decode
  checks, stripping `script`/`svg`/`background`/`xlink:href`, final-document
  validation, direct-argv timeout/cancel, stderr/output bounds, PDF magic, and
  cleanup. Linux delivery additionally proves local-file denial and zero
  outbound traffic with real `wkhtmltopdf`.
- Pre-note tests cover committed-parent finalization, absent-parent delete then
  discard, row/file conflict refusal, and independent generic/API scan limits.

### 7. Wrong vs Correct

```go
// Wrong: controller-owned deletion has no restart-safe recovery boundary.
_ = os.Remove(path)
_ = db.Delete(assetID)

// Correct: generic recovery state is durable before the irreversible path.
manifest, err := content.NewDeleteManifest(identity, source, now)
if err == nil {
    err = manifestStore.Create(ctx, manifest)
}
```

```go
// Wrong: generic recovery cannot know whether the parent note later committed.
_ = repair.RecoverAbandoned(ctx, limit) // includes pre-notes

// Correct: parent decision and action-scoped budget remain explicit.
result, err := recoverAPINotePreNotes(ctx, scanner, workflow, limit)
```
