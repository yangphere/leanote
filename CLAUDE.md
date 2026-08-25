# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Leanote server + web app: a Go 1.15 / **Revel 1.0** monolith on **MongoDB** (`gopkg.in/mgo.v2`), serving
server-rendered templates plus a jQuery/TinyMCE frontend. The same binary also serves the `/api/*`
JSON API consumed by the separate desktop, iOS and Android clients, so API responses and the USN sync
model are external contracts — changing them breaks shipped clients.

## Commands

Requires a local MongoDB and the Revel CLI (`go install github.com/revel/cmd/revel@v1.0.3`).

```bash
# First-time data (creates the leanote DB with the admin user + demo notes)
mongorestore -h localhost -d leanote --dir ./mongodb_backup/leanote_install_data/

# Run dev server on :9000 (wraps `revel run -a .`, watch mode)
cd sh && sh run.sh

# Production tarball -> sh/leanote.tar.gz
cd sh && sh package.sh

# Build without the revel CLI: generate app/routes/routes.go + app/tmp/main.go (both gitignored)
cd app/cmd && sh gen_tmp.sh
go build -o leanote github.com/leanote/leanote/app/tmp
./leanote -importPath=github.com/leanote/leanote -runMode=dev -port=9000

# Go tests (app/tests) - REQUIRE a running Mongo, even with -run: auth_test.go's
# package-level init() calls db.Init and panics with "no reachable servers" otherwise
go test ./app/tests/...
go test ./app/tests/ -run TestReg -v      # single test

# JS tests (node:test, no deps needed) - currently 10 passing
npm test
node --test tests/js/paste-plugin.test.js
```

`tests/apptest.go` is a Revel testrunner suite; the testrunner module is commented out in
`conf/app.conf`, so it does not run — `app/tests/*_test.go` is the live Go test location.

Frontend assets are built by `Gulpfile.js` (`gulp concat`, `plugins`, `minifycss`, `i18n`,
`devToProHtml`). That toolchain is gulp 3.x and does not install/run on modern Node, so recent fixes
have instead been applied to *both* the readable and the pre-built/minified assets by hand — see the
gotchas below.

## Architecture

### Request pipeline

`app/init.go` installs the filter chain and all `revel.TemplateFuncs`, and registers the single
`OnAppStart` hook. Two filters are swapped out for Leanote's own:

- `app/lea/route/Route.go` `RouterFilter` replaces `revel.RouterFilter`. It pings Mongo
  (`db.CheckMongoSessionLost`) on every non-static request and rewrites the controller name by URL
  prefix: `/api/x/y` → `ApiX.Y`, `/member/x/y` → `MemberX.Y`. Combined with the catch-all routes at
  the bottom of `conf/routes` (`* /:controller/:action`), most new actions need **no** routes entry —
  add explicit routes only for path params or pjax URLs.
- `app/lea/i18n` `I18nFilter` replaces `revel.I18nFilter`.

Startup order in `OnAppStart` is load-bearing: `db.Init` → `InitEmail` → `InitVd` →
`service.InitService` → `controllers/admin/member.InitService` → `ConfigS.InitGlobalConfigs` →
`api.InitService`.

### Layers

- **`app/controllers/`** — four packages: root (web), `admin/`, `member/`, `api/`. Each has an
  `init.go` doing the same three things: copy service singletons into package-level vars, declare a
  `commonUrl` whitelist of controller/action pairs that skip auth, and register `AuthInterceptor` via
  `revel.InterceptFunc` per controller struct. Controllers embed `BaseController` (the `api` package
  embeds it **by value**, not as a pointer — see the comment in `ApiBaseController.go`).
- **`app/service/`** — all business logic. Services are singletons created in `service/init.go`
  `InitService()`; both a `NoteS` (exported) and `noteService` (package-private) alias exist for every
  service so services can call each other.
- **`app/db/Mgo.go`** — there is no repository/DAO layer. It exposes `*mgo.Collection` globals
  (`db.Notes`, `db.Users`, …) plus generic helpers, and services query them directly.
- **`app/info/`** — bson-tagged models *and* the JSON envelopes `Re` / `ApiRe`.
- **`app/lea/`** — utilities, usually dot-imported (`. "github.com/leanote/leanote/app/lea"`).
- **`app/cmd/`** — a trimmed vendored copy of `revel/cmd` that only runs source generation, so
  `routes.go`/`main.go` can be produced without the real Revel tool. `app/cmd/README.md` lists
  exactly which files were modified; keep it in sync if you touch that package.

### Auth

Web: Revel cookie session, `Session["UserId"]`. API: `?token=xxx` (falling back to the session id)
resolved by `SessionService` in `api/init.go`'s `AuthInterceptor`, which stores `_userId` / `_token`
in the session for `ApiBaseContrller.getUserId()`. Data-ownership is enforced *in the query*, not by a
middleware — hence the `…ByIdAndUserId` family in `db/Mgo.go`. Any new query that reads or writes
user-owned documents must include `UserId`.

### Client sync (USN)

Notes, notebooks and tags carry a per-user monotonic `Usn`, bumped via `UserService.IncrUsn`. Clients
pull deltas with `GetSyncNotes(userId, afterUsn, maxEntry)`. **A mutation that does not bump `Usn`
is invisible to the desktop/mobile clients.**

### Blog rendering

Blogs deliberately bypass Revel's template engine: `app/lea/blog/Template.go` builds its own
`html/template` set at startup and clones it per request when a user theme is active. Built-in themes
live in `public/blog/themes/{default,elegant,nav_fixed}`; uploaded themes in
`public/upload/<digest>/<userId>/themes/<themeId>` (see `ThemeService`). The `Preview.*` routes mirror
`Blog.*` and render with verbose template errors — that is the theme-debugging path.

### Config, i18n, uploads

- Config is split: `conf/app.conf` (tracked — `app.conf-default` is the template) for infrastructure,
  and the Mongo `configs` collection via `ConfigService`/the admin UI for site behaviour.
- User-facing strings live in `messages/<locale>/*.conf` (7 locales). Server side reads them through
  `app/lea/i18n` (`msg`, `leaMsg`, `rawMsg` template funcs); client side needs the gulp `i18n` task,
  which scrapes `getMsg('key')` out of JS/HTML into `public/js/i18n/*.js` and the TinyMCE lang files.
- Uploads: note images/attachments go to `files/<GetRandomFilePath(userId, guid)>/{images,attachs}`
  (gitignored, served through `File.Output*` / `Attach.Download` / `api/file/*`); avatars and themes
  go under `public/upload/`.

### Frontend layout

`app/views/<controller>/<action>.html` are Revel templates. `public/` holds the SPA-ish editor
(`public/js/app/*`, `public/js/plugins/*`), the markdown editor (`public/md`, requirejs), the vendored
TinyMCE 4.1.9 fork with Leanote plugins (`leaui_image`, `leaui_mindmap`, `leanote_nav`,
`leanote_code`), and the blog/admin/member/album bundles.

## Gotchas

- **`app/views/note/note-dev.html` is the source of truth** for the editor page; `note.html` is
  generated from it by `gulp devToProHtml` (strips `<!-- dev -->` blocks, swaps in `*.min.js`). Edit
  the dev file, then mirror the change into `note.html`.
- **A TinyMCE behaviour fix must be applied to every bundled copy**: `plugins/paste/classes/*.js`,
  `plugins/paste/plugin.js`, `plugin.min.js`, `tinymce.full.js`, `tinymce.full.min.js`.
  `tests/js/paste-plugin.test.js` loads all five and will fail if one is missed. Same shape applies to
  other `public/js/*.min.js` bundles.
- Adding a service means three edits: the struct, `service/init.go` (var + `InitService`), and the
  `init.go` of each controller package that uses it.
- Comments, docs (`app/controllers/api/API*.md`) and much of the naming are Chinese; match the
  surrounding language rather than translating.
- `Re.Msg` is an i18n *key*, not text. `BaseController.RenderRe` translates it and supports
  `key-arg1-arg2` for interpolation.

## Trellis

This repo is Trellis-managed (`AGENTS.md`, `.trellis/`). Non-trivial work goes through the phase
workflow in `.trellis/workflow.md`: ask for task-creation consent, write planning artifacts, then
`task.py start` before implementing. Note that `.trellis/spec/{backend,frontend}/*` are still
unedited scaffolding templates (and describe a React/ORM stack that does not exist here) — do not
treat them as this project's conventions.
