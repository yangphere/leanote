# CLAUDE.md

Leanote is a Go 1.26 monolith backed by MongoDB. The running HTTP stack is the
first-party `net/http` implementation under `app/httpserver`; the old web
runtime and generated controller entrypoints have been removed. The server
renders the existing templates and serves the `/api/*` contract used by the
desktop, iOS and Android clients, so response shapes and USN sync behavior are
published interfaces.

## Commands

Use the repository module graph and the native entrypoint:

```bash
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go vet ./...
go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote
go run ./cmd/leanote -runMode dev
go test ./app/tests/...                 # needs MongoDB and the leanote_test fixture
npm ci && npm run build && npm test
```

The harness builds `./cmd/leanote` and starts it with `-runMode test`; it does
not generate a second server. `conf/app.conf` supplies the dev/test sections,
and test mode must use the `leanote_test` database. Production uses the
explicit deployment config and does not fall back to repository config.

## Architecture

- `app/httpserver/` owns configuration, routes, registry, request binding,
  sessions, middleware, templates and response writers.
- `app/controllers/` contains explicit HTTP adapters for the web, API, admin
  and member surfaces. Each action is registered by name with its method set
  and BEFORE hooks; catch-all dispatch never reflects over methods.
- `app/service/` owns business rules and MongoDB access. HTTP adapters call
  services and map typed results; they do not duplicate USN, receipt,
  authorization or content lifecycle state machines.
- `app/application/content/` is the single logical-path resolver. Stored
  `files/`, `upload/` and `public/upload/` paths must pass through it before
  mapping to validated content roots.
- `app/views/`, `public/`, `messages/` and `conf/` retain the existing page,
  asset, translation and route contracts.

## Request and identity rules

The request flow is recovery/gzip, health/readiness, static handling, route
matching, session and locale resolution, registry lookup, BEFORE hooks, action,
session commit, and result writing. `HEAD` follows `GET`; actions with an
explicit method matrix return 405 with `Allow` for an in-registry mismatch.

Web requests use the session principal. API requests accept the published
query/form token contract and write `_token`/`_userId` session state only after
successful authentication. User-owned queries must include the authenticated
owner. Anonymous access to published blog images and attachments is intentional
and is authorized by the content service.

`httpserver.Params` preserves legacy presence, repeated values, nested keys and
Atob boolean vocabulary. Use strict integer/ObjectID readers only where the
action contract marks a field as required; keep zero/default behavior for
legacy-compatible optional fields.

## Blog, themes and content

Built-in themes are selected from `public/blog/themes/{default,elegant,nav_fixed}`
according to the user's style. Uploaded themes live below the configured
public upload root and are resolved with containment checks. Preview responses
may expose detailed template errors for theme debugging; public blog responses
use stable generic 500 text and log details server-side.

Attachment, image, archive and backup downloads use bounded services and stream
readers to the HTTP response. Do not use `io.ReadAll` to materialize a large
download in an adapter, and always sanitize `Content-Disposition` filenames.

## Configuration and generated assets

`ProductionConfig` is the sole startup handoff for Mongo identity, content
roots, temporary storage and backup root. Admin and member adapters consume
typed configuration and the loaded global snapshot; an unavailable snapshot is
not an empty successful configuration.

`app/views/note/note-dev.html` and `scripts/build/manifest.mjs` are the sources
of truth for generated frontend assets. Edit sources and run the Node 24 build;
do not hand-edit generated bundles.

## Verification and Trellis

For code changes run focused tests first, then `gofmt -l app cmd`, build, vet,
and `git diff --check`. Mongo replay, real listener/browser, failpoint and
container evidence remains explicitly `unrun` until executed. Read
`.trellis/workflow.md` and the active task's PRD/design/spec before changing a
layer, and record evidence in the task materials when behavior or contracts
change.
