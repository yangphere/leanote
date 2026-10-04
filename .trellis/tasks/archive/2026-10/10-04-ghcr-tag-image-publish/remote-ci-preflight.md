# Remote CI Preflight

## Scope

- Read-only review of GitHub Actions run
  [37088848935](https://github.com/yangphere/leanote/actions/runs/37088848935),
  commit `a5fc3cc9bc6e40c8fd3dda8b667243e37058bc52`, completed 2026-10-03.
- Compared against local candidate `e1b18995bc4d88a28507db75674e715beb9c8d94`.
- No remote workflow was rerun or changed. No branch, tag, package or repository
  setting was written.

## Conclusion

The complete quality gate is not ready for a `v1.0.0` publication. The latest
remote run had three independent primary failures; `summary` then failed as the
expected consequence of those upstream results. The local candidate does not
contain fixes for the three failing areas, so adding `docker-image.yml` alone
does not make the reusable quality gate green.

Go 1.26.7, Go 1.27.0, container smoke and package smoke passed. The failures are
not caused by unavailable pinned toolchains or job time budgets.

## Blocking findings

### 1. Node contract fixtures inherit the real workflow name

- Remote failure: `quality-gate / node-build`, step `Discover and run Node
  tests`.
- Failing tests:
  - `release artifact validation rejects unknown metadata schema versions`
  - `release artifact validation binds build metadata to the tarball bytes`
- Root cause: the fixtures are generated with workflow `Quality gate`, but the
  validator subprocess environment in
  `tests/js/release-contract.test.js:55` and
  `tests/js/release-contract.test.js:109` spreads `process.env` without
  overriding `GITHUB_WORKFLOW`. In GitHub Actions it therefore receives `CI`
  and exits first at `scripts/validate-release-artifact.mjs:46` with
  `release inputs workflow mismatch`; neither intended negative path is
  reached.
- Current-candidate status: still blocked. Reproduced locally by setting the
  run's GitHub variables (`GITHUB_WORKFLOW=CI`): both tests fail with the same
  mismatch. Setting `GITHUB_WORKFLOW=Quality gate` makes the two focused tests
  pass (2 passed, 0 failed).
- Narrow fix: in both subprocess fixture environments, explicitly set
  `GITHUB_WORKFLOW: 'Quality gate'` along with the already pinned run ID and
  attempt. Add or retain a CI-environment regression invocation so ambient
  GitHub provenance cannot intercept the asserted error path.

### 2. Mongo golden contradicts the migrated controller compatibility mapping

- Remote failure: `quality-gate / mongo-8_0`, step `Discover and run
  integration tests`, `TestGoldenWebOwnershipControllers`.
- Actual response for the demo recipient's unauthorized notebook lookup:
  `200 application/json` with body `null`; golden expects `404 text/plain`.
- Root cause: `app/controllers/httpserver_publishing.go:311-316` deliberately
  maps `service.ErrShareResource` to legacy `null` JSON without exposing notes.
  The original golden also expected `200 null`; commit `a2f146d9` changed only
  `app/tests/golden/web/share_listShareNotes_demo.json:1` to 404, while the
  later standard-library adapter kept the explicit compatibility mapping.
- Current-candidate status: still blocked. The harness, controller mapping and
  golden are unchanged from the failing remote run.
- Narrow fix: preserve the fail-closed service authorization and the explicit
  legacy adapter response, then restore this golden to `200`, JSON content
  type, body `null`. Add a controller/service assertion that unauthorized
  access returns no note data so the compatibility envelope is not mistaken
  for authorization success.

### 3. The preview smoke applies an HTML assertion to an intentional text 404

- Remote failure: `quality-gate / mongo-8_0`,
  `TestWebAdminMemberAndControllerSmoke`.
- Root cause: `app/tests/harness/smoke_test.go:53` calls `assertHTMLPage` for
  `/preview` with expected status 404. The helper at lines 109-123 always
  requires `<html`, but `PreviewHTTPServer.dispatch` returns the standard plain
  text 404 when no `themeId` is supplied (`app/controllers/httpserver_publishing.go:185-191`).
- Current-candidate status: still blocked; these files are unchanged from the
  failing run.
- Narrow fix: assert the intentional 404 status and plain response separately,
  or supply a valid preview theme and expect a real 200 HTML page. Do not weaken
  `assertHTMLPage` for successful page routes.

### 4. Build manifest publishes a filesystem URL for TinyMCE index files

- Remote failure: `quality-gate / chromium-e2e`, step `Discover and run
  Chromium E2E`.
- Actual result: `/public/tinymce/plugins/leaui_image/index.html` returned 301,
  while the resource-closure check requires a direct 200.
- Root cause: `scripts/build/manifest.mjs:121-128` derives every asset URL as
  `/<output>`, producing `/public/tinymce/...`. The application's canonical
  route is `/tinymce/*filepath` (`conf/routes:126`), and browser/business tests
  already use `/tinymce/plugins/leaui_image/index.html`. The filesystem-style
  `/public/.../index.html` request encounters the static handler's canonical
  index redirect, which the no-redirect closure check correctly exposes.
- Current-candidate status: still blocked. The manifest and build-smoke loop at
  `tests/e2e/build/build-resource-smoke.spec.mjs:117-123` are unchanged from the
  failing run.
- Narrow fix: map `public/tinymce/...` manifest outputs to canonical
  `/tinymce/...` URLs while retaining their output paths. Add a manifest
  contract asserting no TinyMCE runtime URL starts with `/public/tinymce/`,
  regenerate the affected build fixtures/assets through `npm run build`, and
  rerun the Chromium build smoke.

`quality-gate / summary` is not a separate root cause. It correctly rejected
the failed `chromium-e2e` summary after all seven quality summaries were
downloaded.

## Toolchain and budget evidence

- `go-1_26_7`: passed, 469 tests discovered/executed.
- `go-1_27_0`: passed, 469 tests discovered/executed.
- `container-smoke`: passed with `/healthz` 200.
- `package-smoke`: passed with `/healthz` 200.
- Node setup successfully installed 24.20.0; the failing Node suite completed
  in about 31 seconds.
- Playwright 1.62.1 and Chromium installed successfully; the failing E2E test
  reached its assertion in about 28 seconds.
- MongoDB 8.0.29 restored 498 documents with zero restore failures; the test
  failures occurred after startup and fixture restore.
- The `summary` job used the hosted action runtime/default Node and emitted
  Node-20 deprecation warnings for pinned actions being forced to Node 24.
  Those warnings did not cause this run's failures. They should be handled by
  future action-pin maintenance, not by weakening this release gate.

## Branch prerequisite

The requested source/default branch is now `main`, but the remote currently has
only `dev` and `master`; `refs/heads/main` is absent. Any ancestry fetch against
`origin/main` will fail until the reviewed candidate is integrated into a real
remote `main` branch and repository/default-branch coordination is complete.
This is independent of the four quality-gate code/test blockers above.

## Required verification before publication

1. Run the two release-contract negative tests under CI-like provenance.
2. Run the focused Mongo harness tests against the restored MongoDB 8.0 fixture.
3. Run the Chromium build-resource smoke against the native test server.
4. Run the full reusable quality gate and require every primary job plus
   `summary` to pass before creating or pushing `v1.0.0`.
