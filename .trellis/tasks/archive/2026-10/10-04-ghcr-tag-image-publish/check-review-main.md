# Trellis Check Review — main / 2.0.1 candidate

## Findings (fixed)

### High — numeric image tags failed the reusable package quality job

- File: `sh/package.sh`, `scripts/version.mjs`,
  `tests/js/release-contract.test.js`
- Issue: `quality-gate.yml` runs `sh/package.sh` for every caller. In a
  `refs/tags/2.0.1` image workflow run, the package script passed `2.0.1` to
  the default `version.mjs <tag>` CLI, whose protected Release contract
  correctly accepts only `vX.Y.Z`. The `package-smoke` job would therefore
  fail before building its package, blocking every numeric image release.
- Fix: add the explicit `assertPackageTag` / `--package-tag` boundary for the
  shared package script. It dispatches to the existing strict numeric image
  or prefixed protected Release validator. The default positional CLI remains
  Release-only. Added a real `refs/tags/2.0.1` package-script regression with
  a fake Go compiler, plus accepted `v2.0.1` and rejected malformed/mismatched
  tag cases. The executable publishing spec now records this boundary.

## Findings (not fixed)

- No remaining local implementation or specification defect was found in the
  revised task scope.
- Real `main` GitHub CI, the `2.0.1` tag push, GHCR first-package responses,
  image push/read-back, package visibility, and anonymous pull remain
  `unrun`. These require the authorized remote steps and cannot be inferred
  from local contract tests.
- Local Chromium E2E remains `unrun`; the required runtime/credential context
  is absent locally. The reusable remote quality gate remains the acceptance
  boundary.

## Verification

- Lint: pass — Actionlint v1.7.7 parsed `docker-image.yml`, `ci.yml`, and
  `quality-gate.yml` with no findings (`shellcheck` unavailable and disabled).
- TypeCheck: not applicable — changed runtime code is JavaScript and POSIX
  shell; focused Node execution and Actionlint cover their syntax/contracts.
- Tests: pass — publication + Release focused suites: 41 passed, 0 failed;
  the new real numeric-tag package regression passed. CI-like ambient
  provenance regressions: 2 passed, 0 failed. Focused Mongo/HTTP harness: 2
  passed, 0 skipped in 14.576s.
- Build pipeline: pass — recorded task evidence is 37 total, 36 passed and 1
  Windows-only skip; the manifest-driven build produced no tracked output
  drift.
- Trellis validation: pass — `implement.jsonl` 6 entries and `check.jsonl` 6
  entries.
- Whitespace: pass — `git diff --check` reported only line-ending conversion
  warnings.

## Scope conclusion

The numeric `2.0.1` workflow now reaches the complete reusable quality gate,
including package smoke, without weakening the protected `vX.Y.Z` Release
contract. The seven quality jobs and summary remain intact; `release-inputs`
is still limited to `v` tags. Both publication stages refresh and validate
the remote tag and `origin/main`; GHCR absence checks remain fail-closed; the
candidate is smoked before the sole push and the pushed manifest digest is
checked afterward. The candidate is ready for the real `main` CI gate.

## Follow-up review — canonical TinyMCE index URLs

### Findings (fixed)

- No additional defect was found in the follow-up patch. The patch addresses
  the concrete Chromium failure without changing the production handler:
  manifest entries whose output ends in `/index.html` now expose the
  containing directory URL with a trailing slash. Files remain at their
  original `public/tinymce/.../index.html` output paths.

### Findings (not fixed)

- The follow-up has not yet run in GitHub Chromium. The next real `main` CI
  must prove all seven primary quality jobs and `summary` pass together.
- The `2.0.1` publication tag and all GHCR/public/anonymous-pull evidence
  remain `unrun`; no tag or registry write was performed during review.

### Verification

- Focused manifest test: pass — 1 passed, 0 failed. It verifies every TinyMCE
  asset keeps a `public/tinymce/...` output, uses a canonical `/tinymce/...`
  runtime URL, and maps index files to directory URLs. The two discovered
  mappings are `/tinymce/plugins/leaui_image/` and
  `/tinymce/plugins/leaui_mindmap/mindmap/`.
- Build: pass — `npm run build` exited 0 and introduced no additional tracked
  output changes.
- Live route evidence: recorded local requests returned 301 with `Location:
  ./` for explicit `leaui_image/index.html`, while both manifest directory
  URLs returned direct 200. This matches Go's canonical index behavior and
  the no-redirect Chromium contract.
- Remote evidence: GitHub run `37172547373` was independently queried. At
  `dc323d9ef3b4f744735fb4665ab41d7009403e58`, Node, Mongo, Go 1.26.7, Go
  1.27.0, package smoke and container smoke passed; Chromium failed and
  `summary` correctly failed. `release-inputs` was skipped.
- Workflow isolation: pass — this follow-up does not modify any workflow.
  `docker-image.yml` still accepts only numeric tags, calls the full reusable
  gate, checks `origin/main`, and confines `packages: write` to publish;
  `release.yml` still accepts only `v*.*.*`; all seven quality jobs plus
  `summary` remain present and `release-inputs` remains `v`-only.
- Trellis validation and `git diff --check`: pass; only existing line-ending
  conversion warnings were reported.

No new local publication blocker remains. The candidate is ready for commit,
merge to `main`, and the next complete remote quality gate.
