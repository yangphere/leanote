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

## 3. Contracts

- Git tag `X.Y.Z` publishes `ghcr.io/yangphere/leanote:X.Y.Z` and updates
  `ghcr.io/yangphere/leanote:latest` to the same manifest digest, on
  `linux/amd64`. First publication uses Git tag and image tag `2.0.1`.
  Use `scripts/version.mjs` as the sole version/tag rule; registry absence
  checks and push must use the same unprefixed image tag.
  Version tags are immutable. Latest is a mutable alias updated only after
  version read-back succeeds; do not generate other short-version aliases.
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
  Copy that exact registry digest only to latest, preserving bytes; do not
  rebuild or repush the immutable version. Reject unknown operations or
  irrelevant digest inputs. Both paths retain candidate/source evidence gates.
- The workflow lock is `docker-image-latest` for all versions/entries, with cancellation
  disabled. Sharing the protected Release workflow's whole-run lock would
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
  explicit policy `ALLOW_INITIAL_PACKAGE_CREATE=true`. The function defaults
  to disallowing first-package creation.
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
  manifest/config, then copy the archive to latest and verify the same identities.
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

## 5. Good / Base / Bad Cases

- Good: a fresh main tag passes all gates, the exact candidate passes smoke,
  absence is confirmed, and version/latest raw manifest read-back matches the
  one-build OCI artifact. Existing version promotion verifies/pulls/smokes
  the expected digest and writes only latest.
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
  PDF smoke environment for both operation paths.
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
Correct: validate and smoke immutable 2.0.1@digest, then copy that digest to latest.

Wrong: check registry v2.0.1, then push registry 2.0.1.
Correct: accept canonical Git tag 2.0.1 and check/push registry 2.0.1.
```
