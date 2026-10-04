# Versioned GHCR Image Publishing

## 1. Scope / Trigger

Use this contract when changing `.github/workflows/docker-image.yml` or its
registry preflight. ADR-0005 permits image-only delivery before the protected
tarball/GitHub Release gates. It does not authorize production deployment.

## 2. Signatures

```text
push tag vX.Y.Z -> validate -> quality-gate.yml -> publish
node scripts/check-version.mjs
node scripts/check-ghcr-tag-absent.mjs
scripts/container-smoke.sh <candidate-image>
```

`checkGhcrTagAbsent({ image, tag, actor, token,
allowInitialPackageCreate = false, fetchImpl = fetch })` resolves to
`{ initialPackage: boolean }` only after confirmed registry absence; otherwise
it rejects. The CLI reports a stage-specific error and exits nonzero.

## 3. Contracts

- Publish only `ghcr.io/yangphere/leanote:vX.Y.Z`, on `linux/amd64`.
  Use `scripts/version.mjs` as the sole version/tag rule.
- `RELEASE_TAG` matches the package/lock version; the remotely peeled tag
  equals `GITHUB_SHA`, and that SHA is an ancestor of refreshed `origin/master`.
  Recheck tag and ancestry immediately before publication; reject forced tags.
- The workflow lock is `docker-image-${{ github.ref }}`, with cancellation
  disabled. Sharing the protected Release workflow's whole-run lock would
  block this path while its unprovisioned delivery runner waits.
- Default permissions are `contents: read`; only publish adds
  `packages: write`. Registry credentials are `GITHUB_ACTOR` and `GH_TOKEN`
  from `GITHUB_TOKEN`. Never print credentials or raw authorization responses.
- The CLI uses `GHCR_IMAGE`, `RELEASE_TAG`, `GITHUB_ACTOR`, `GH_TOKEN`, and the
  explicit policy `ALLOW_INITIAL_PACKAGE_CREATE=true`. The function defaults
  to disallowing first-package creation.
- Registry queries use authenticated GHCR token scope for the exact image,
  reject redirects, bound JSON responses to 64 KiB, and time out requests.
  For an existing package, require `MANIFEST_UNKNOWN` and a successful
  identity-bound tag listing. First-package creation requires structured
  `NAME_UNKNOWN` from both exact manifest and listing endpoints; reject any
  supplied repository identity that conflicts with the requested image.
- Build once in publish with `VERSION`, `REVISION`, `SOURCE_DATE_EPOCH` and
  `OCI_CREATED`, without provenance/SBOM. Smoke this exact candidate before
  the only push. Compare registry manifest digest with Buildx metadata's
  `containerimage.digest`; a Docker image/config ID is a different identity.
- The first package is private. Public visibility and anonymous pull are
  separate manual evidence. Before enabling protected Release publication,
  disable image-only publication and select a fresh tag; the two workflows
  do not provide a shared atomic publication lock.

## 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Version mismatch, forced/moved tag, SHA outside master | Stop before push |
| Quality gate or candidate smoke failure | Publish job blocked / no push |
| Manifest exists, or listing contains the target tag | Reject overwrite |
| Missing manifest plus confirmed existing package listing | Allow one push |
| Two confirmed unknown-name replies with explicit creation policy | Allow first-package push |
| Missing creation policy, conflicting identity, one-sided absence | Reject |
| Missing credentials, auth/permission/rate-limit error, invalid/oversized JSON, redirect, timeout/network error | Reject; never infer absence |
| Registry digest differs from build manifest digest | Fail after push; publication is unconfirmed |

## 5. Good / Base / Bad Cases

- Good: a fresh master tag passes all gates, the exact candidate passes smoke,
  absence is confirmed, and registry manifest read-back matches.
- Base: injected `fetchImpl` proves all preflight branches without credentials
  or remote writes. Static checks verify job ordering and action pins.
- Bad: infer absence from any failed `imagetools inspect`, compare an image ID
  to a manifest digest, or infer public pullability from workflow success.

## 6. Tests Required

- `tests/js/docker-image-workflow.test.js`: tag-only trigger, minimal
  permissions, full reusable gate, fresh source checks, smoke-before-push,
  exact tag and build-manifest comparison; exercise registry failure branches.
- Run focused tests, `npm test`, workflow YAML/Actionlint checks and
  `git diff --check`. Record tool versions and skipped external validators.
- Real tag push, GitHub Actions execution, initial package creation, digest
  read-back, public visibility and anonymous pull remain `unrun` until executed.
  Fixtures and command stubs cannot satisfy those gates.

## 7. Wrong vs Correct

```text
Wrong: registry inspection failed, therefore the tag is absent; push it.
Correct: accept only the explicit authenticated absence cases above.

Wrong: local Docker image Id equals the registry manifest digest.
Correct: Buildx containerimage.digest equals the registry manifest digest.
```
