# Repository Guidelines

Leanote is a Go 1.26 MongoDB monolith using the first-party `net/http` stack.
`app/httpserver` owns the request pipeline and registry; `app/controllers`
contains explicit web, API, admin and member adapters; `app/service` and
`app/application` own business and content contracts. Templates are under
`app/views`, browser assets under `public`, translations under `messages`, and
routes/configuration under `conf`.

## Build, test and local development

```bash
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go vet ./...
go test ./app/httpserver ./app/controllers/... ./app/service ./cmd/leanote
go run ./cmd/leanote -runMode dev
go test ./app/tests/...                 # requires MongoDB and leanote_test
npm ci && npm run build && npm test
```

The HTTP harness builds `./cmd/leanote` and starts `-runMode test`; it does not
generate a second runtime. Test mode must select `leanote_test`. Production
configuration is explicit and never falls back to repository configuration.

## Style and naming

Run `gofmt` on changed Go files. Use lower-case package names, PascalCase
exported identifiers and camelCase locals. Do not hand-edit generated frontend
assets: `app/views/note/note-dev.html` and
`scripts/build/manifest.mjs` are the sources of truth.

## Tests and evidence

Go tests use `testing` and `*_test.go`; JavaScript tests use `node:test`.
Every behavior fix needs a focused regression test where a deterministic test
seam exists. Build/vet checks do not replace Mongo, real HTTP, browser,
cross-process, failpoint or corpus evidence; leave those gates explicitly
unrun until executed.

Downloads and backups must stream bounded readers to the response. Resolve
stored `files/`, `upload/` and `public/upload/` paths only through
`app/application/content.ParseStoredPath`; do not add a second path parser.

## Security and boundaries

Do not commit credentials or generated user data. Validate external paths and
command inputs at the boundary. Keep authorization, USN/receipt state and
content lifecycle rules in services; adapters only bind parameters and map
results. Anonymous published image/attachment reads are an intentional public
whitelist and still pass through service authorization.

<!-- TRELLIS:START -->
# Trellis Instructions

This project is managed by Trellis. Read `.trellis/workflow.md`, the active
task's PRD/design, and the layer-specific documents under `.trellis/spec/`
before changing code. Record implementation and validation evidence in the
active task. Prefer the repository Trellis commands when available.

<!-- TRELLIS:END -->
