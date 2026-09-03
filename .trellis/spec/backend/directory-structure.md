# Directory Structure

> Where different file types go in the Go backend.

---

## Overview

Leanote is a Go 1.26 monolith that migrated off the Revel runtime onto a first-party HTTP stack (ADRs in `docs/adr/`). Revel-era paths for controllers/views are kept; the server kernel lives in `app/httpserver/`.

## Directory Layout

| Path | Role | Real examples |
|------|------|---------------|
| `cmd/leanote/` | Production entrypoint: flag parsing, canonical config validation, route/session/static wiring, server run | `cmd/leanote/main.go`, `main_test.go` |
| `app/httpserver/` | HTTP kernel: registry, routing, sessions, gzip, healthz, production config | `registry.go` (`/healthz` handled before route matching), `production_config.go`, `middleware.go` |
| `app/controllers/` | Web + admin + member controllers (Revel-era paths) | `app/controllers/Note.go`, `app/controllers/admin/` |
| `app/api/` | `/api/*` JSON controllers | `app/api/` package |
| `app/service/` | Business logic between controllers and db | `app/service/Note.go` |
| `app/db/` | MongoDB access: collection wrappers + codecs | `Mgo.go` (`InitWithError`), `mongo_client.go` (`dialMongo`, `Ping`) |
| `app/info/` | Struct models mapped to BSON | `app/info/Note.go` |
| `app/lea/` | Shared runtime helpers (ObjectID, codec registry) | `app/lea/` |
| `app/tests/` | Go tests (standard `testing`; Mongo-backed via harness) | `auth_test.go`, `harness/` |
| `app/tests/harness/` | E2E supervisor + three-mode Mongo environment for Playwright | `environment.go`, `cmd/e2e/main.go` |
| `app/views/` | Go text/template views; `app/views/note/note-dev.html` is the editable source of generated `note.html` | `app/views/note/` |
| `conf/` | `conf/routes` table (packaged into the runtime) + `app.conf-default` | `conf/routes` |
| `sh/` | POSIX sh packaging (`set -eu`) | `sh/package.sh` |
| `scripts/` | Node 24 build/release/CI tooling | `scripts/build/manifest.mjs` |

## Module Organization

- New HTTP handler: implement the action in `app/controllers` (web/admin/member) or `app/api`, add the route to `conf/routes`, and ensure registration happens via `controllers.RegisterHTTP(registry, runMode, cfg)` wiring in `cmd/leanote/main.go`.
- Server-kernel concerns (routing, sessions, config validation, health, gzip) belong in `app/httpserver/` — never inside controllers.
- Database access goes through `app/db` wrappers; services compose them; controllers never dial Mongo directly.
- Utilities shared across layers go in `app/lea/`; do not create a second helper package for the same concern.

## Naming Conventions

- Packages are lowercase single words matching their directory (`httpserver`, `controllers`, `service`, `db`, `info`, `lea`).
- Exported identifiers PascalCase, locals camelCase; file names match the primary type (`Note.go` for `NoteService`-style aggregates).
- Tests are `*_test.go` with `TestXxx`; the harness's fake-injection fields follow the `run`/`now`/`sleep`/`ping` pattern in `app/tests/harness/environment.go`.

## Examples

- Route registration end-to-end: `conf/routes` → `httpserver.ParseRoutes` → `controllers.RegisterHTTP` in `cmd/leanote/main.go`.
- A controller module with service + db split: `app/controllers/Note.go` → `app/service/Note.go` → `app/db/Mgo.go` collections.
- Constraint to keep: `rg 'github.com/revel|revel\.' app go.mod sh conf` must stay zero hits (the go.sum pongo2 hash is the recorded exception).
