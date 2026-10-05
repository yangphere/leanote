# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

Go 1.26 monolith, standard `testing`, no linter beyond `gofmt`/`go vet` — the gates are the CI quality jobs (node-build/chromium-e2e/mongo-8_0 + go-1_26_7/go-1_27_0) and the contract tests under `tests/js/`. Quality = green CI on the real boundaries plus focused regression cases per fix (AGENTS.md testing rules).

## Forbidden Patterns

- Legacy runtime imports: `rg -n -i 'github\.com/revel|revel\.' app cmd go.mod go.sum sh conf scripts .github Dockerfile` must stay zero hits. Historical notes under `docs/` and `.trellis/` are outside this runtime scan.
- Probe-based or fallback environment selection (Mongo mode, toolchain) — fail closed instead (see `database-guidelines.md`).
- Log-and-return at every layer; masking db/service errors as empty success (`error-handling.md`).
- Hand-editing generated assets: anything produced by `scripts/build/manifest.mjs` (`public/js/app.min.js`, `public/tinymce/**` bundles, `app/views/note/note.html`) — regenerate via `npm run build`.
- CI shell scripts using PCRE-only regex syntax with `grep -E` (e.g. `(?:...)` — GNU grep ERE never matches it; the B-E6 root cause).

## Required Patterns

- `gofmt` before commit; run `go vet ./app/tests/harness/...` for harness changes; `go build ./...` must pass.
- Fake-injection for environment-dependent tests: function fields (`run`, `now`, `sleep`, `lookPath`, `ping`, `verifyFixture`) on the environment struct with nil-guard defaults — see `app/tests/harness/environment.go` and its tests.
- Windows-safe shell: `set -eu` scripts guard expansions (`${GITHUB_REF:-}`); MSYS path pitfalls handled with `MSYS_NO_PATHCONV` where needed.
- Failure diagnostics preserved: smoke scripts dump app log tail / response headers / docker logs on failure (original-cause requirement).
- Release fixture subprocesses pin their fixture `GITHUB_WORKFLOW` as well as
  run ID/attempt; an ambient CI workflow must not intercept a negative test
  before its intended assertion. For HTTP goldens, preserve explicit legacy
  response envelopes and assert authorization separately. Missing-theme
  `/preview` returns plain text 404; only successful page responses use HTML
  assertions.

## Scenario: Note mutation and blog-publication parameter binding

### 1. Scope / Trigger

Use this contract when a note HTTP action receives one or more note IDs from
the legacy jQuery client. jQuery encodes `noteIds` arrays as `noteIds[]`,
while older callers may send repeated `noteIds`, indexed fields, or one
`noteId` for blog publication.

### 2. Signatures

```go
func noteParameterStrings(params *httpserver.Params, name string) []string
func allNotesToBlog(noteIDs []string, publish func(string) bool) bool
```

`NoteService.ToBlog(userID, noteID, isBlog, isTop)` remains the sole owner of
publication and authorization decisions.

### 3. Contracts

- `noteParameterStrings(params, "noteIds")` accepts repeated `noteIds`,
  `noteIds[]`, and contiguous `noteIds[0]`, `noteIds[1]`, ... fields, in that
  order of precedence.
- `Note.SetNote2Blog` prefers the list forms and falls back to a non-empty
  singular `noteId` for legacy clients. It returns the raw JSON boolean
  `true` only when every target confirms through `ToBlog`.
- An empty target list returns `false`; each target is attempted in request
  order even after an earlier target fails. `isBlog`, `isTop`, owner identity,
  and service-layer receipt/USN rules are passed through unchanged.
- Delete, move, copy, and shared-copy actions use the same list binder;
  share and PDF actions continue to consume singular `noteId`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| `noteIds[]` or repeated `noteIds` supplied | all values reach the mutation/publication service |
| Indexed list supplied | contiguous values are read in index order |
| No list and non-empty `noteId` on `SetNote2Blog` | one-item publication attempt |
| Empty list and no singular fallback | raw JSON `false`; service is not called |
| Invalid, unauthorized, or failed target | continue remaining targets; final result is `false` |
| Every target confirms | raw JSON `true` |

### 5. Good / Base / Bad Cases

- Good: bind `noteIds[]` once and pass the resulting slice to the existing
  service method; aggregate publication results without duplicating service
  rules.
- Base: a legacy request with one `noteId` still publishes one note.
- Bad: call `Params.Strings("noteIds")` directly for a jQuery array, or stop
  after the first failed target and report a partial success.

### 6. Tests Required

- Parameter-binding tests assert repeated, bracketed, and indexed encodings.
- Publication aggregation tests assert empty input, all-success input, the
  false result for partial failure, and invocation of targets after failure.
- Keep Mongo, real HTTP, browser, and PDF-tool evidence separate from these
  deterministic controller-boundary tests.

### 7. Wrong vs Correct

```go
// Wrong: jQuery's noteIds[] values bind as an empty list.
ids := c.Params.Strings("noteIds")

// Correct: accept the deployed encodings through the shared binder.
ids := noteParameterStrings(c.Params, "noteIds")
ok := allNotesToBlog(ids, func(id string) bool {
	return noteService.ToBlog(userID, id, isBlog, isTop)
})
```

## Testing Requirements

- Every fix ships a focused regression case (AGENTS.md). Real server boundary for HTTP work — never call a controller directly.
- Route parser tests must check segment length before indexing extracted segments; static file routes bypass controller BEFORE hooks and need separate assertions.
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
- Build the native `cmd/leanote -runMode test` entrypoint with a Go toolchain of
  at least 1.26.7. The harness resolves `go` from PATH by default and fails
  closed below that floor; `LEANOTE_TEST_GO` is an optional explicit override,
  and every build subprocess runs with `GOTOOLCHAIN=local` (no automatic
  toolchain downloads).
- Use the pinned MongoDB 8.0 `leanote-test-mongo` fixture for self-provisioned
  replay. Restore it into `leanote_test` before integration tests and remove
  the named container afterward; service-backed mode must never invoke Docker.
- Treat `Content-Type` and `Location` as the only comparable HTTP headers for
  JSON responses; reject headers outside the documented comparison/exclusion
  sets. Binary snapshots compare non-empty body presence and stable headers,
  not machine-dependent bytes.
- Keep baseline changes out of production packages: `app/` changes are limited
  to `app/tests/`, plus the test run-mode section in `conf/app.conf` and the
  explicitly tracked regression workflow.

## HTTP Baseline Contract

### 1. Scope / Trigger

The contract applies when adding or updating the first-party HTTP Golden, USN,
smoke, or Mongo fixture harness under `app/tests/harness`.

### 2. Signatures

- `go run ./app/tests/harness/cmd/env up|down`
- `LEANOTE_GOLDEN=record|replay go test -p 1 ./app/tests/... -count=1 -timeout 30m`
- Native `cmd/leanote -runMode test` builds use the default `go` on PATH,
  enforced to be at least 1.26.7 (fail closed); `LEANOTE_TEST_GO` is an
  optional explicit override that bypasses the floor check.

### 3. Contracts

- `up` starts the pinned `mongo:8.0` image as `leanote-test-mongo`, restores into
  `leanote_test`, and verifies two fixture users; any setup failure after
  container creation must remove the named container before returning.
- Replay reads `app/tests/golden/**/*.json` and never creates or rewrites files.
- Record stores normalized request/response snapshots; dynamic ObjectId and
  timestamp replacement is field-scoped and preserves JSON key order.
- The native test server binds only to loopback (`http.addr=127.0.0.1`),
  listens on fixed port `28017`, and uses the `[test]` config section with
  `site.url=http://127.0.0.1:28017` so Windows does not expose the test
  executable to public/private network firewall prompts. A smoke test that
  expects readiness must provision the Mongo fixture before starting the
  process; without it, non-static requests correctly remain `503 not_ready`.
- Self-provisioned replay is single-owner per Docker daemon/workspace: do not
  run two harness processes concurrently because the fixed `leanote-test-mongo`
  container name and host ports `27017`/`28017` are shared. Use serialized
  runs (or service-backed/isolated resources) when parallel validation is
  required.
- `httpserver.Params.FormFile` must run the same bounded form parser as the
  scalar parameter readers before delegating to `http.Request.FormFile`; an
  upload action that accesses the file first must not bypass the request body
  limit or lose ordinary multipart fields.
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
| Missing, older, or unreadable default `go` for the native server build | explicit failure before any build; `LEANOTE_TEST_GO` overrides |
| Port 28017 occupied | explicit failure; no random fallback |
| Unknown response header | normalization failure |
| Missing ExportPdf golden or unavailable wkhtmltopdf in replay | explicit skip with a message to run the Linux record job; record mode fails |
| Windows `ExportPdf` | the same tool/golden guard skips; Linux record job owns the first golden |
| Test config missing, test server address is not loopback, a database URL uses continuation/newline syntax, or database URL points outside `leanote_test` | explicit failure before server startup |
| Legacy `TestAuth` without MongoDB in CI | `LEANOTE_REQUIRE_MONGO=1` makes the independent auth step fail |
| Binary response has JSON Content-Type | record/replay assertion fails |

### 5. Good / Base / Bad Cases

- Good: restore Mongo 8.0, run the native server, and replay unchanged snapshots.
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

## Scenario: Go 1.26 native entrypoint build

### 1. Scope / Trigger

When CI or the Mongo harness builds the test server, it must compile the
first-party `cmd/leanote` entrypoint with the repository module graph. The
native harness owns this build and never generates a second runtime entrypoint.

### 2. Signatures

```sh
export GOTOOLCHAIN=local
go version
go build -o "$RUNNER_TEMP/leanote" ./cmd/leanote
"$RUNNER_TEMP/leanote" -runMode=test -conf ./conf/app.conf
```

### 3. Contracts

- The native harness resolves the default `go` from PATH and rejects versions
  below 1.26.7 before building; `LEANOTE_TEST_GO` is an explicit override.
- `GOTOOLCHAIN=local` prohibits the build from silently downloading another
  toolchain.
- `sh/run.sh` invokes `go run ./cmd/leanote -runMode dev`; the test harness
  builds the same `cmd/leanote` package and starts it with `-runMode test`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Native build fails | The harness fails before Mongo replay or smoke requests |
| Default Go is missing, old, or unreadable | Fail closed before any build; `LEANOTE_TEST_GO` is the only explicit override |
| `cmd/leanote -runMode test` exits before readiness | Preserve the exit and include the child log; do not fall back to another entrypoint |
| Production config is supplied to test mode | Reject the source/database mismatch instead of silently falling back |

### 5. Good / Base / Bad Cases

- Good: build `./cmd/leanote` from the checked-out module with
  `GOTOOLCHAIN=local`, then run the test mode against the isolated fixture.
- Base: run the native build/toolchain contract tests without Mongo.
- Bad: generate an alternate runtime entrypoint, download a different
  toolchain implicitly, or ignore a native child-process failure.

### 6. Tests Required

- Run the harness toolchain floor and native build tests.
- Run `go build ./...` and the real Mongo/HTTP harness; Linux delivery may
  additionally exercise `sh/run.sh` and `sh/package.sh`.

### 7. Wrong vs Correct

```text
Wrong:   generate a legacy runtime entrypoint or run `go install` for a second framework
Correct: go build -o "$RUNNER_TEMP/leanote" ./cmd/leanote
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
- Renderer selection (`pdf.renderer`, `pdf.gotenberg.url`) is parsed once by
  `httpserver.parsePDFRendererConfig` for prod/dev/test and handed over inside
  `ProductionConfig`; omitted means the `wkhtmltopdf` process backend, an unknown
  value or a missing/invalid Gotenberg URL fails startup, and only deployment
  config (never the admin UI or request parameters) can choose Gotenberg.
- Gotenberg backend tests run against `httptest.Server`/`RoundTripper`. The
  real-container gate (`TestGotenbergBackendRealOutputPassesPDFRenderer`, enabled
  by `LEANOTE_GOTENBERG_URL`) must push Chromium output through
  `application.PDFRenderer.Render`; `qpdf --check` or a `%PDF-` prefix alone is
  not evidence. Docker delivery pins Gotenberg by multi-arch index digest, keeps
  it on an `internal` network with no host port (egress must fail), and uses
  `--chromium-deny-list=^(?!file:///tmp/|data:).*` so inlined `data:` resources
  render while any other URL is blocked.
- Docker prod disables the `demo` account; a container smoke that logs in as the
  fixture user must use a config without the Docker admin bootstrap, and the
  fixture note's remote image host is dead, so seed self-contained content first.
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

## Scenario: First-party HTTP startup and production configuration

### 1. Scope / Trigger

Use this contract when wiring `cmd/leanote` or adding an HTTP adapter that
consumes production configuration. The interface layer validates deployment
inputs once, creates a credential-free runtime handoff, and only then binds
content storage, database access, and the listener.

### 2. Signatures

```go
cfg, err := httpserver.ValidateProductionConfig("/etc/leanote/app.conf")
runtime, err := httpserver.ValidateProductionRuntimeConfig(cfg, publicStaticRoot)
registry.RegisterMethods("ApiAuth", "Register", []string{"POST"}, befores, handler)
```

`cmd/leanote` supplies `publicStaticRoot` from its resolved application base;
the executable-inference wrapper is not the application entrypoint and must
not replace that explicit handoff.

Production configuration must provide `db.dbname`,
`db.urlEnv=${MONGODB_URL}`, `app.secret=${LEANOTE_APP_SECRET}`,
`content.private.data`, `content.private.quarantine`,
`content.public.data`, `content.public.quarantine`, `content.temporary`,
and `admin.backup.root`. It must also provide an explicit `[prod] site.url`
whose value is either a literal `http`/`https` origin or exactly
`${LEANOTE_SITE_URL}`. The origin may include a valid port, but no path, query,
fragment, user info, or surrounding whitespace; its host must pass
`domain.CanonicalizeBlogHost`. `ProductionConfig` exposes only the address,
shutdown timeout, database identity/digest, credential-provider reference,
validated content roots, and backup root; it never carries raw credentials or
the source `Config`.

### 3. Contracts

- Production reads exactly `/etc/leanote/app.conf` in `prod` mode. Mongo and
  the app secret come from `MONGODB_URL` and `LEANOTE_APP_SECRET`.
- All six content/backup paths are absolute existing directories. Content
  pairs are canonicalized and pass `contentfs.ValidateContentRoots`; quarantine
  roots are not HTTP served, roots do not overlap, and backup is outside all
  content/static roots. `/upload/*` and `/public/upload/*` serve the validated
  public data root.
- The production app installs the locale resolver (configured cookie, first
  `Accept-Language`, then default), session reader/writer, and `ViewArgs` keys
  `currentLocale` and `locale` before an action runs.
- Explicit routes keep their route-table method behavior. Catch-all identity
  actions with an `AllowedMethods` list return `405` and an `Allow` header;
  `HEAD` follows `GET`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Non-canonical/missing/unreadable production config | stable `ConfigError`; process exits 78 before listener or database bind |
| Missing `[prod] site.url` | `CONFIG_KEY_INVALID` with key `site.url` |
| Missing/empty `LEANOTE_SITE_URL` for `${LEANOTE_SITE_URL}` | `CONFIG_VALUE_MISSING` / `CONFIG_VALUE_EMPTY` with key `LEANOTE_SITE_URL` |
| Invalid site origin or non-target `${...}` placeholder | `CONFIG_SITE_URL_INVALID` / `CONFIG_SOURCE_CONFLICT`; error text contains no URL value |
| Missing/relative root | `CONFIG_CONTENT_ROOT_MISSING` or `CONFIG_CONTENT_ROOT_RELATIVE` |
| Unwritable or cross-device data/quarantine pair | `CONFIG_CONTENT_ROOT_UNWRITABLE` or `CONFIG_CONTENT_ROOT_CROSS_DEVICE` |
| Quarantine under a served root / any root overlap | `CONFIG_CONTENT_ROOT_PUBLIC` or `CONFIG_CONTENT_ROOT_OVERLAP` |
| Catch-all identity method outside its matrix | `405` with deterministic `Allow`; do not invoke the action |
| Explicit route method mismatch | `404` from route matching, preserving the observed route contract |

### 5. Good / Base / Bad Cases

- Good: validate and canonicalize roots once, pass the resulting typed values
  to `service.InitContentRuntime`, then construct the `httpserver.App`.
- Base: a unit test supplies temporary directories and a loopback static root
  to `ValidateProductionRuntimeConfig` and asserts the complete handoff.
- Bad: derive `/app/files` or `/app/public/upload` from the executable,
  forward `*Config` to admin code, or silently fall back to another Mongo
  URL/root when validation fails.

### 6. Tests Required

- Unit tests assert every stable root/config error code, canonical identity and
  digest, locale precedence, `ViewArgs`, session commit behavior, static
  upload routing, the site URL/source/error matrix, and the identity 405/`Allow`
  matrix.
- Run `go build ./...`, focused `go test`, `go vet`, `gofmt -l`, task context
  validation, and `git diff --check` before committing.
- Real listener requests, Mongo/Golden replay, process-level exit 78, and
  container volume/non-root checks remain separate evidence; focused tests do
  not promote those gates.

### 7. Wrong vs Correct

```go
// Wrong: application code derives a legacy path and receives raw config.
roots := filepath.Join(appBase, "files")
admin.Configure(cfg)

// Correct: validate at the boundary and pass only typed, credential-free data.
runtime, err := httpserver.ValidateProductionRuntimeConfig(cfg, publicRoot)
if err != nil {
	return err
}
service.InitContentRuntime(runtime.ContentRoots)
```

## Scenario: First-party content download adapters

### 1. Scope / Trigger

Use this contract when a standard-library adapter exposes an attachment,
archive, image, or PDF through `Content-Disposition`.

### 2. Signatures

```go
func DownloadDisposition(kind, name string) string
type Result interface { Apply(http.ResponseWriter, *http.Request) }
```

### 3. Contracts

- Services own authorization and readable streams; adapters stream them with
  `io.Copy` and close them after the response. A service may stage a validated
  asset in its private temporary root, but the HTTP adapter must not load the
  complete download into a byte slice.
- `DownloadDisposition` strips path components, quotes, and control
  characters before constructing `filename="..."`.
- D-H6 dependency failures use HTTP 500 with the legacy empty `Page`/slice
  body; legacy attachment listing errors keep their 200 `Re{Msg:"error"}`
  envelope.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing readable file/image | legacy empty text (or `No Such File` for API attachment) |
| Stream read failure | same failure body; never report a successful binary |
| Unsafe display name | sanitized basename in `Content-Disposition` |
| `GetImages`/`GetAlbums` dependency failure | 500 plus empty legacy shape |

### 5. Good / Base / Bad Cases

- Good: call the content service, set the shared disposition sanitizer, copy the
  reader directly to the response, and close it on every path.
- Base: a valid download keeps its content type, body, and attachment/inline
  disposition.
- Bad: concatenate a database display name directly into a header, call
  `io.ReadAll` for a large download, or return a partial body as success after
  a stream error.

### 6. Tests Required

- Unit-test path, quote, CR/LF, and control-character filename sanitization.
- Contract-test 500 empty-body shapes for image/album dependency failures and
  the attachment legacy error envelope.
- Keep real HTTP/Mongo stream and Golden replay as separate evidence; in-process
  tests do not promote those gates.

### 7. Wrong vs Correct

```go
// Wrong: a user-controlled display name reaches a response header.
w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)

// Correct: the shared HTTP boundary sanitizes it once.
w.Header().Set("Content-Disposition", httpserver.DownloadDisposition("attachment", name))
```

## Scenario: Docker blog host and share-modal HTTP contracts

### 1. Scope / Trigger

Use this contract when a Docker production configuration or the authenticated
blog/share adapters changes. These paths are cross-layer boundaries: the
configured site URL selects the blog host, while share-info actions return a
Bootstrap modal fragment consumed by the browser.

### 2. Signatures

```go
func (s *ShareHTTPServer) dispatch(c *httpserver.Context) httpserver.Result
func renderShareUserInfo(c *httpserver.Context, resourceID string, isNote bool,
    users []info.ShareUserInfo) httpserver.Result
```

Docker must provide `site.url` in `conf/app.conf-docker`; share-info routes are
`GET /share/listNoteShareUserInfo?noteId=<id>` and
`GET /share/listNotebookShareUserInfo?notebookId=<id>`.

### 3. Contracts

- `site.url` is an explicit, non-empty deployment value used by blog host
  canonicalization; Docker does not infer it from an empty config.
- A valid share resource ID produces HTML containing `.modal-dialog`,
  `.modal-content`, `#shareNotebookTable`, and the existing share controls.
- The note/notebook distinction is preserved for permission and delete actions;
  the adapter passes `isNote` and the canonical resource ID to the view.
- Anonymous requests still run the session and authentication befores and are
  redirected to `/login`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Missing Docker `site.url` | fail blog-domain resolution instead of serving an ambiguous host |
| Invalid share resource ID | HTTP 400 `invalid share resource` |
| Valid ID with no shared users | HTTP 200 HTML modal with an empty list and add row |
| Anonymous share request | HTTP 302 to `/login`; handler is not called |

### 5. Good / Base / Bad Cases

- Good: resolve the configured blog domain once and render the shared modal
  template through the native HTTP boundary.
- Base: a focused controller test invokes both share-info actions with a valid
  and invalid ID and checks status, content type, and modal fragments.
- Bad: return validation JSON for a remote modal request, infer a blog host from
  an empty Docker setting, or duplicate note/notebook templates.

### 6. Tests Required

- Test authentication redirect, invalid-ID 400, HTML modal shape, and escaped
  user email data for both note and notebook actions.
- Run focused Go tests, `go vet`, `go build ./...`, and a real container
  `/healthz` plus blog/share browser check. Unit tests do not replace Mongo,
  HTTP, or browser evidence.

### 7. Wrong vs Correct

```go
// Wrong: remote modal callers receive JSON validation output.
return c.RenderJSON(service.ListNoteShareUserInfo(...))

// Correct: both actions share one escaped HTML modal contract.
return renderShareUserInfo(c, c.Params.String("noteId"), true, users)
```
