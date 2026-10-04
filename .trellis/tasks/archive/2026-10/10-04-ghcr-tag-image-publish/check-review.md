# Trellis Check Review

## Findings (fixed)

### Medium — refresh the release source immediately before the write

- File: `.github/workflows/docker-image.yml`
- Issue: the final remote tag peel and refreshed `origin/master` ancestry check
  ran before the authenticated registry preflight. Each registry request has a
  bounded five-minute timeout, so the source decision could be stale by the
  time `docker push` started.
- Fix: authenticate first, complete the registry absence preflight, then fetch
  and verify the remote tag and `origin/master` immediately before the only
  push. The focused workflow test now asserts this order.

### Medium — query every manifest form that can occupy an immutable tag

- File: `scripts/check-ghcr-tag-absent.mjs`
- Issue: the exact-tag request advertised only single-image OCI and Docker
  manifest media types. An existing OCI index or Docker manifest list must also
  count as an occupied tag; omitting those media types weakens the immutable
  absence check through content negotiation.
- Fix: add OCI index and Docker manifest-list media types to `Accept`, with a
  regression assertion on the actual manifest request.

### Low — keep streamed response failures staged and redacted

- File: `scripts/check-ghcr-tag-absent.mjs`
- Issue: a transport failure while iterating a registry response body escaped
  as the raw lower-level exception, unlike request and JSON failures.
- Fix: map stream-read failures to `GHCR <stage> response read failure` without
  exposing the underlying message; preserve the distinct 64 KiB budget error.
  A focused regression injects a failing async body stream.

### Low — enforce all external action pins in the task contract test

- File: `tests/js/docker-image-workflow.test.js`
- Issue: the test checked the Buildx action pin but did not enforce the task's
  full-SHA rule for checkout and Node setup actions.
- Fix: enumerate every external `uses:` entry and require a 40-character
  lowercase hexadecimal commit SHA.

## Findings (not fixed)

- No remaining implementation or documentation defect was found within the
  task scope.
- Real tag push, GitHub Actions execution, actual GHCR first-package response,
  candidate image build/smoke, registry push/read-back, public visibility and
  anonymous pull remain `unrun`. They require the separately authorized remote
  publication steps and cannot be replaced by local contract tests.
- The lightweight and protected Release workflows still have no cross-workflow
  atomic lock by design. ADR-0005 and the delivery guide accurately require
  disabling the lightweight path and choosing a fresh tag before protected
  Release publication.

## Verification

- Lint: pass — Actionlint v1.7.7 with `-shellcheck=` reported no findings.
  ShellCheck is unavailable.
- TypeCheck: not applicable to the changed JavaScript/YAML files; `node --check`
  passed for the helper and focused test.
- Tests: pass — focused Node contract suite: 10 passed, 0 failed.
- Full tests: the implementer-recorded corrected `npm test` run passed before
  review fixes (230 total, 229 passed, 0 failed, 1 platform skip). It was not
  repeated because the review changes are fully exercised by the focused suite.
- Whitespace: pass — `git diff --check` reported only existing line-ending
  conversion warnings.

## Consistency review

- The workflow remains tag-only, uses the independent
  `docker-image-${{ github.ref }}` lock, grants `packages: write` only to
  publish, reuses the complete quality gate, builds and smokes the exact image
  before one push, and compares manifest digest identities.
- Existing-package `MANIFEST_UNKNOWN` and explicit first-package
  `NAME_UNKNOWN` handling remain fail-closed. Missing, optional and conflicting
  `detail.name`, malformed/oversized JSON, permission/rate-limit states,
  redirects, timeouts, network failures and body-stream failures are covered by
  implementation checks and focused tests.
- ADR-0005, ADR-0004's qualification, the delivery guide, task PRD/design and
  `.trellis/spec/backend/image-publishing.md` agree on scope and evidence limits.
