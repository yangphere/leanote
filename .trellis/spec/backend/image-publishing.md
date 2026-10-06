# Versioned GHCR Image Publishing

## 1. Scope / Trigger

Use this contract when changing `.github/workflows/docker-image.yml` or its
registry preflight. ADR-0005 permits image-only delivery before the protected
tarball/GitHub Release gates. It does not authorize production deployment.

## 2. Signatures

```text
push tag X.Y.Z -> validate -> quality-gate.yml -> publish
main workflow_dispatch(operation, tag, expected_commit,
  source_run_id, source_run_attempt, expected_registry_digest?, expected_config_digest?)
  -> validate candidate/source evidence -> quality-gate.yml -> publish candidate
RELEASE_TAG=2.0.1 node scripts/check-version.mjs --image-tag
node scripts/version.mjs --package-tag <X.Y.Z-or-vX.Y.Z>
node scripts/check-ghcr-tag-absent.mjs
node scripts/check-latest-promotion.mjs --has-latest <tags.json> <repository>
node scripts/check-latest-promotion.mjs --should-promote <tags.json> <config.json> <repository> <candidate-version>
scripts/container-smoke.sh <candidate-image>
skopeo copy --preserve-digests oci-archive:<one-build-archive> docker://<version-or-latest>
IMAGE_MANIFEST_PATH=<raw-json-file> EXPECTED_MANIFEST_DIGEST=sha256:<hex>
  EXPECTED_CONFIG_DIGEST=sha256:<hex> node scripts/verify-image-manifest.mjs
```

`checkGhcrTagAbsent({ image, tag, actor, token,
allowInitialPackageCreate = false, fetchImpl = fetch })` resolves to
`{ initialPackage: boolean }` only after confirmed registry absence; otherwise
it rejects. The CLI reports a stage-specific error and exits nonzero.

`verifyImageManifest({ manifestBytes, expectedManifestDigest,
expectedConfigDigest? })` returns `{ manifestDigest, configDigest }` only for
exact-byte SHA256, supported single-platform schema-2 image media type and
matching config identity. `verifyImageManifestFile` adds a nonempty 1 MiB file
limit. Its CLI prints the config digest and exits nonzero on any mismatch.

`compareImageVersions(left, right)` validates both through `assertImageTagFormat`
and compares numeric components with BigInt, returning -1/0/1.
`shouldPromoteLatest({ candidateVersion, listing, expectedRepository, latestConfig })`
returns a boolean using the validated Skopeo listing and OCI version label.
The file CLI requires nonempty regular JSON files at most 1 MiB, prints
true/false for normal decisions and exits nonzero on errors. A missing latest
tag confirmed by a valid listing requires no config file; an existing latest
tag requires readable config and a valid version label.

## 3. Contracts

- Git tag `X.Y.Z` publishes `ghcr.io/yangphere/leanote:X.Y.Z` and, when it
  advances the current version, updates `ghcr.io/yangphere/leanote:latest` to the same manifest digest, on
  `linux/amd64`. First publication uses Git tag and image tag `2.0.1`.
  Use `scripts/version.mjs` as the sole version/tag rule; registry absence
  checks and push must use the same unprefixed image tag.
  Version tags are immutable. Latest identifies the highest successfully
  promoted strict `X.Y.Z`, updated only after version read-back succeeds.
  Skip latest writes for older/equal versions; do not generate other short-version aliases.
- GitHub selects the workflow from the event's commit SHA/ref; tag push uses
  the tagged commit's workflow. These guard/queue/package policies do not
  retrofit old workflow files. Do not backfill version tags on commits lacking
  this hardening. New release tags must include the fixes, and recovery must
  run from a main executor containing them. Enforcing historical workflows
  needs separately reviewed remote tag rules or a trusted fixed entry point;
  do not claim this local change provides that enforcement.
- `assertImageTag(tag, version)` accepts only strict unprefixed `X.Y.Z` equal
  to the package version. `check-version.mjs --image-tag` selects that rule;
  its default and `assertReleaseTag` retain the protected `vX.Y.Z` contract.
  Workflow glob filtering is followed by strict validation; it is not the
  validation boundary.
- The reusable gate's `sh/package.sh` accepts either strict image or protected
  release tag matching the project version in real tag contexts. Branch names
  are never release tags. Keep the protected release CLI/metadata validators
  strict; select the dual packaging contract explicitly with
  `version.mjs --package-tag`. `assertPackageTag` dispatches to the existing
  strict image or release rule, rather than defining another version parser.
- `RELEASE_TAG` matches the package/lock version; the remotely peeled tag
  equals `CANDIDATE_SHA`, and that SHA is an ancestor of refreshed `origin/main`.
  Recheck tag and ancestry immediately before publication; reject forced tags.
- Push candidates use `github.sha`. Recovery dispatch is restricted to main;
  it uses the explicit original `expected_commit`, never the executor SHA.
  Preserve an existing tag rather than moving it to repair publication tooling.
  Verify the specified original Docker image push run/attempt through GitHub
  API: repository, workflow path/name, tag, candidate SHA and attempt must match.
  Required quality jobs and summary succeeded; original publish build/smoke
  succeeded and push step failed, even though overall run conclusion is failure.
  Reject attempt drift. Download its summaries and reuse the shared schema
  validator with explicit source execution provenance, including summary.
  Executor quality-gate success alone is not candidate quality evidence.
- Dispatch operation `recover_version` rebuilds a confirmed absent version.
  `update_latest` instead requires explicit expected registry/config digests,
  validates the existing version manifest, pulls its exact digest, verifies
  config Id, Linux/amd64, version, revision and source labels, and smokes it.
  Apply the same latest version guard as fresh publication, then copy that
  exact registry digest only to latest when it advances the alias, preserving bytes; do not
  rebuild or repush the immutable version. Reject unknown operations or
  irrelevant digest inputs. Both paths retain candidate/source evidence gates.
- The workflow lock is `docker-image-latest` for all versions/entries, with
  `cancel-in-progress: false` and `queue: max`. Up to 100 runs can wait; default
  single-pending replacement cancellation is disabled, but queue overflow can
  still cancel new runs. Waiting order is not semantic version order. Re-run
  canceled work only after checking immutable-version presence; published
  versions require guarded latest recovery, never a duplicate version push.
  Sharing the protected Release workflow's whole-run lock would
  block this path while its unprovisioned delivery runner waits.
- Default permissions are `contents: read`; source verification adds
  `actions: read` only in validate; only publish adds
  `packages: write`. Registry credentials are `GITHUB_ACTOR` and `GH_TOKEN`
  from `GITHUB_TOKEN`. Never print credentials or raw authorization responses.
- Skopeo runs as pinned `quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0`
  (tested 1.22.3). Mount Docker's GHCR auth config read-only and pass explicit
  authfile options. Ubuntu 22.04's tested Skopeo 1.4.1 lacks
  `--preserve-digests`; do not install that incompatible package or drop the
  preservation requirement to make it pass.
- The CLI uses `GHCR_IMAGE`, `RELEASE_TAG`, `GITHUB_ACTOR`, `GH_TOKEN`, and the
  optional explicit policy `ALLOW_INITIAL_PACKAGE_CREATE=true`. The function
  defaults to disallowing first-package creation. First publication is already
  complete: the ordinary workflow must not enable that option. Package absence
  is an error requiring separate authorization, never an automatic re-create.
- Before either latest copy, use the pinned Skopeo tool to read a successful
  package `list-tags` result and bind `Repository` to `IMAGE_REPOSITORY`.
  If `Tags` contains latest, read its raw OCI config and require
  `config.Labels["org.opencontainers.image.version"]` to be strict `X.Y.Z`.
  Share the version-format rule and compare each numeric component without
  floating-point precision loss. Only greater candidates promote; older/equal
  candidates skip with a clear message. A valid listing without latest permits
  initialization. Failed requests, bad listing/config, missing labels or
  malformed versions are errors, never evidence that latest is absent.
- Registry queries use authenticated GHCR token scope for the exact image,
  reject redirects, bound JSON responses to 64 KiB, and time out requests.
  For an existing package, require `MANIFEST_UNKNOWN` and a successful
  identity-bound tag listing. Explicit first-package creation requires
  structured 404 `NAME_UNKNOWN` from listing and either `NAME_UNKNOWN` or
  `MANIFEST_UNKNOWN` from the exact manifest endpoint; reject any
  supplied repository identity that conflicts with the requested image.
- Build once in publish with `VERSION`, `REVISION`, `SOURCE_DATE_EPOCH` and
  `OCI_CREATED`, without provenance/SBOM. A single Buildx invocation exports
  both the loaded smoke candidate and its OCI archive. Before smoke, hash the
  archive's raw manifest and compare it to Buildx `containerimage.digest`;
  bind its config descriptor to Buildx `containerimage.config.digest` and the
  loaded image Id. Smoke that exact loaded candidate. Publish the same archive
  with `skopeo copy --preserve-digests` to the version, verify exact raw registry
  manifest/config, then apply the latest guard and, if permitted, copy the
  archive to latest and verify the same identities.
  A Docker image/config Id is not a manifest digest. Do not use Docker Engine
  push for this artifact: load/push reserializes the manifest and changes its
  digest, even when `oci-mediatypes=false`; config stays stable. This boundary
  was reproduced against a real local registry, not inferred from mocks.
  Candidate checkout supplies build/smoke inputs; executor checkout supplies
  the repaired registry helper. All revision/epoch metadata binds to candidate.
- The first package is private. Public visibility and anonymous pull are
  separate manual evidence. The protected path retains its `v*.*.*` trigger
  and `vX.Y.Z` image tags; an unprefixed tag does not trigger it. All seven
  reusable quality jobs and summary remain required. The protected handoff
  `release-inputs` remains limited to `v` tags and is skipped for image-only
  delivery. The two paths do not share a registry immutability lock.

## 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Version mismatch, forced/moved tag, SHA outside main | Stop before push |
| Prefixed, malformed, or package-mismatched image tag | Stop before registry requests |
| Quality gate or candidate smoke failure | Publish job blocked / no push |
| Manifest exists, or listing contains the target tag | Reject overwrite |
| Missing manifest plus confirmed existing package listing | Allow immutable version publication |
| Two confirmed unknown-name replies with explicit creation policy | Allow first-package push |
| Manifest `MANIFEST_UNKNOWN` and listing `NAME_UNKNOWN`, both structured 404 with explicit creation policy | Allow first-package push |
| Recovery source identity/attempt/gate/summary mismatch or non-main dispatch | Stop before publish |
| Missing creation policy, conflicting identity, one-sided absence | Reject |
| Missing credentials, auth/permission/rate-limit error, invalid/oversized JSON, redirect, timeout/network error | Reject; never infer absence |
| Archive digest/config or loaded candidate Id mismatch | Stop before smoke or remote writes |
| Registry exact-byte digest/config differs from OCI artifact | Fail; do not update latest or retry immutable version writes |
| Latest copy/read-back failure after version success | Version remains published; retry only explicit latest promotion after verification |
| Promotion expected digest/config/platform/version/revision/source mismatch | Stop before latest write |
| Candidate version older than/equal to latest | Skip latest copy/read-back, retain current alias |
| Valid bound package listing confirms no latest | Permit alias initialization after candidate validation |
| Latest listing/config/label unreadable or malformed | Fail; no latest copy and no absence fallback |
| Ordinary workflow sees confirmed absent package | Reject initial package creation |
| Multiple pending runs, queue below 100 | Retain waiting runs with queue:max |
| Queue has 100 pending runs | New run can be canceled; check publication state before recovery |

## 5. Good / Base / Bad Cases

- Good: a fresh main tag passes all gates, the exact candidate passes smoke,
  absence is confirmed, and version/latest raw manifest read-back matches the
  one-build OCI artifact when the version advances latest. Existing version
  promotion verifies/pulls/smokes the expected digest and applies the same
  version guard before writing only latest. Older publications preserve latest.
- Base: injected `fetchImpl` proves all preflight branches without credentials
  or remote writes. Static checks verify job ordering and action pins.
- Bad: infer absence from any failed `imagetools inspect`, compare an image ID
  to a manifest digest, or infer public pullability from workflow success.

## 6. Tests Required

- `tests/js/docker-image-workflow.test.js`: numeric push/main recovery triggers, minimal
  permissions, full reusable gate, fresh source checks, smoke-before-push,
  exact tag and build-manifest comparison; exercise registry failure branches
  and recovery source execution/summary provenance and candidate separation.
  Cover raw-byte/config mismatch, one build/two exports, byte-preserving copy,
  latest ordering/shared lock, no version write during promotion, and required
  PDF smoke environment for both operation paths. Cover numeric comparison,
  equal/older skips, multi-digit/large components, missing latest initialization,
  invalid listing/config/version rejection, both guards before latest copy,
  queue:max with cancellation disabled and ordinary first-package rejection.
- `tests/js/release-contract.test.js`: packaging under numeric/v tag contexts
  succeeds; malformed or mismatched tags fail, while branch names never enter
  tag validation. The protected release version contract remains prefixed.
- Run focused tests, `npm test`, workflow YAML/Actionlint checks and
  `git diff --check`. Record tool versions and skipped external validators.
- Real tag push, GitHub Actions execution, initial package creation, digest
  read-back, public visibility and anonymous pull remain `unrun` until executed.
  Fixtures and command stubs cannot satisfy those gates.

## 7. Wrong vs Correct

```text
Wrong: registry inspection failed, therefore the tag is absent; push it.
Correct: accept only the explicit authenticated absence cases above.

Wrong: compare Buildx --load digest to Docker Engine push manifest digest.
Correct: preserve the one-build OCI archive manifest through registry copy;
         compare its exact raw digest, and bind loaded image Id to config digest.

Wrong: overwrite existing 2.0.1 just to add latest, or rebuild latest separately.
Correct: validate and smoke immutable 2.0.1@digest, then apply the version guard;
         copy that digest only when it advances or initializes latest.

Wrong: check registry v2.0.1, then push registry 2.0.1.
Correct: accept canonical Git tag 2.0.1 and check/push registry 2.0.1.

Wrong: backfill 1.9.9 and unconditionally overwrite latest 2.0.1.
Correct: publish missing 1.9.9, but skip latest because its numeric version is newer.

Wrong: cancel-in-progress:false means every pending run is kept.
Correct: set queue:max; document its 100-pending limit and state-aware recovery.

Wrong: leave first-package creation enabled after successful initial publication.
Correct: keep the normal workflow on the helper's default false policy.

Wrong: merge the guard into main, then tag an old unpatched commit and expect protection.
Correct: tag only commits containing the hardening; recovery uses repaired main.
```

## Scenario: Production and dev Compose consumers

### 1. Scope / Trigger

Use this contract when changing `docker-compose.yml`, its explicit dev
override, environment examples or deployment instructions.

### 2. Signatures

```text
docker compose config --quiet
docker compose pull
docker compose up -d
docker compose -f docker-compose.yml -f docker-compose.dev.yml config --quiet
docker compose -f docker-compose.yml -f docker-compose.dev.yml build leanote
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

The production application also validates `site.url` from the container
configuration before binding the listener or opening MongoDB:

```go
cfg, err := httpserver.ValidateProductionConfig("/etc/leanote/app.conf")
```

### 3. Contracts

- Base Compose uses `ghcr.io/yangphere/leanote:${LEANOTE_IMAGE_TAG:?LEANOTE_IMAGE_TAG must be set}`
  without build. Select a published strict unprefixed `X.Y.Z`; no implicit
  version, `v` prefix, latest fallback or repository override.
- Dev override contains only `services.leanote.image=leanote:local` and build
  context/Dockerfile/`VERSION=${LEANOTE_VERSION:?LEANOTE_VERSION must be set}`.
  `LEANOTE_VERSION=0.0.0` denotes a local build; production does not consume it.
- Compose interpolates each file before merging. Both combinations require
  `LEANOTE_IMAGE_TAG`, even though dev ultimately uses the local image.
  Compose's required-value expression rejects missing/empty values; it does
  not validate semantic version syntax. Publication owns strict version rules.
- `LEANOTE_SITE_URL` is required in the base service environment and is
  expanded by `conf/app.conf-docker` as `[prod] site.url`. It must be the
  browser-facing `http`/`https` origin, with an optional port and no path,
  query, fragment, user info, or surrounding whitespace. This value drives
  absolute image/attachment/API/mail links and the default blog host.
- The same origin determines production session-cookie `Secure`: HTTPS means
  HTTPS-only login cookies; bypassing the proxy with HTTP cannot retain login.
  Do not add `LEANOTE_COOKIE_SECURE` or `cookie.secure` to production config.
- `LEANOTE_SESSION_EXPIRES` is optional. Base Compose passes
  `${LEANOTE_SESSION_EXPIRES:-168h}` and Docker config declares
  `session.expires=${LEANOTE_SESSION_EXPIRES}`. The application validates a Go
  duration in `[5m, 8760h]` (no `d` unit), defaulting absent/blank input to `168h`.
  Dev Compose inherits this env mapping. The duration is absolute from the
  last session write; recreate the container after changing it, and retain
  existing cookies' embedded expiry until reissue.
- Shared services, networks, named volumes, Linux/amd64 and required runtime
  environment fields exist only in the base file. Gotenberg retains its pinned
  digest, hardening arguments and internal-only PDF network.
- Production still requires the repository's
  `mongodb_backup/leanote_install_data` seed directory. Do not describe this
  layout as a standalone single-file deployment.
- Production upgrades pull the selected version then recreate leanote; dev
  source updates build then recreate with both explicit `-f` arguments.
  Preserve data volumes and do not edit an operator's credentials for tests.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Published image tag, runtime env, no dev version | Base renders a GHCR image with no build |
| Both files, image tag and dev version | Local image and VERSION build argument |
| Missing/empty image tag, either combination | Required-value interpolation failure |
| Missing dev version, both files | Required-value interpolation failure |
| Missing/empty `LEANOTE_SITE_URL` | Required-value interpolation failure with `LEANOTE_SITE_URL must be set` |
| Invalid production `site.url` or non-target placeholder | Redacted `ConfigError` (`CONFIG_SITE_URL_INVALID` or `CONFIG_SOURCE_CONFLICT`); exit 78 before listener/database |
| Session expiry unset/empty in either Compose combination | container env `LEANOTE_SESSION_EXPIRES=168h` |
| Custom session expiry in either combination | supplied duration passed unchanged; application enforces bounds |
| Invalid/out-of-range production session expiry | redacted `CONFIG_SESSION_EXPIRES_INVALID`; startup exits 78 |
| Registry pull failure | Surface the failure; do not build or fall back to latest |

### 5. Good / Base / Bad Cases

- Good: production selects a published `2.0.1`; dev explicitly combines the
  two files while sharing one service topology.
- Good: `.env` sets `LEANOTE_SITE_URL=https://note.example.com`; the container
  expands the same value and blog/default links use `note.example.com`.
- Base: fixture-only configuration rendering verifies image/build separation.
- Bad: automatically load dev overrides, duplicate Mongo/PDF/volume settings,
  assume dev avoids required base-file interpolation, or leave `site.url`
  empty so the app guesses its public host.

### 6. Tests Required

- Release-contract assertions cover base image/required variable/no build,
  and dev image/build-only scope. Review the environment template and
  documentation for distinct production/dev commands.
- Assert the `.env.example`, Compose interpolation, and Docker config all
  carry `LEANOTE_SITE_URL`; unit-test valid/invalid origins, placeholder
  source errors, and redacted `ConfigError{Code,Key}` values.
- Assert expiry's optional `:-168h` interpolation, Docker placeholder and
  `.env.example` default, and absence of a production cookie-security override.
  Render prod/dev with expiry unset, empty and custom to verify env inheritance.
- Windows release-contract checks normalize source CRLF before multiline
  matching or running extracted shell snippets. Put Git's `usr/bin` on the
  verification process PATH for shell-dependent tests; retain the assertions.
- Actual Compose rendering must verify both combinations, missing/empty image
  tag rejection, missing dev version rejection, and production independence
  from the dev version. Also render with a non-sensitive site URL and verify
  missing/empty `LEANOTE_SITE_URL` is rejected. Use non-sensitive fixture
  values.
- Real registry pull and container health/persistence evidence remain separate
  from static tests. Run runtime checks in a separate project with new volumes
  and port; do not replace the current stack or remove its volumes.

### 7. Wrong vs Correct

```text
Wrong: docker compose build leanote                  # base has no build
Correct: docker compose pull leanote                # production upgrade
Correct: docker compose -f docker-compose.yml -f docker-compose.dev.yml build leanote

Wrong: dev image override means LEANOTE_IMAGE_TAG can be omitted.
Correct: supply LEANOTE_IMAGE_TAG for base interpolation; dev uses leanote:local.

Wrong: site.url=http://127.0.0.1:9000 in a reverse-proxied deployment.
Correct: set LEANOTE_SITE_URL to the public origin and recreate the leanote
container so generated absolute links and blog host matching use that origin.

Wrong: LEANOTE_SESSION_EXPIRES=7d or making the Compose variable required.
Correct: accept optional LEANOTE_SESSION_EXPIRES=168h; reject malformed or
out-of-range values at the application's production configuration boundary.
```
