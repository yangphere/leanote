# Latest publication review

## Findings (fixed)

- File: `tests/js/docker-image-workflow.test.js`
- Issue: The workflow regression tests did not prove that the `update_latest` recovery path avoids rebuilding or rewriting the immutable version tag, nor that promotion occurs only after the final Git tag and `origin/main` recheck. The manifest verifier tests also covered config binding but not raw manifest digest mismatch or rejection of an OCI index.
- Fix: Added assertions that the latest-only branch contains no Buildx build, OCI archive, or version-tag absence check; performs exactly one Skopeo copy; and copies `latest` only after the source recheck. Added verifier regressions for a mismatched raw manifest digest and an OCI index presented where a single-platform manifest is required.

## Findings (not fixed)

- The repaired workflow has not yet run end to end on a GitHub-hosted executor. Local workflow tests, Actionlint, and the implementation-stage registry exercise cover the deterministic logic, but do not substitute for the real Actions environment.
- The real `ghcr.io/yangphere/leanote:latest` promotion and its registry read-back have not run. This is intentionally left to the main flow after integration; this review performed no remote writes.
- Public visibility changes remain an owner action. The main flow has already verified anonymous manifest access and a real anonymous pull of `2.0.1`; the package is publicly accessible. This review did not change visibility. Anonymous `latest` access still requires verification after promotion.

## Verification

- Focused tests: pass — `node --test tests/js/docker-image-workflow.test.js` (17 passed, 0 failed, 0 skipped).
- Lint: pass — Actionlint v1.7.7 accepted `.github/workflows/docker-image.yml`, `.github/workflows/ci.yml`, and `.github/workflows/quality-gate.yml`.
- Syntax/import: pass — `node --check scripts/verify-image-manifest.mjs` and `node --check tests/js/docker-image-workflow.test.js`.
- Trellis validation: pass — task metadata and both six-entry implementation/check JSONL manifests validate.
- Diff hygiene: pass — `git diff --check` reports no whitespace errors (only the existing Git line-ending notice).
- Registry behavior: pass at the implementation-stage local registry gate — one dual-export candidate was copied to version and `latest` with Skopeo `--preserve-digests`; raw manifest bytes, manifest SHA, config digest, headers, and OCI metadata matched on read-back.
- Prior broad suite: implementation handoff records 238 passed and 1 Windows-specific skip; the recovery review records 45/45 focused checks passing.

Conclusion: ready to integrate, with no local blocker. The main flow should run the complete GitHub CI and then dispatch the guarded `update_latest` operation using the verified immutable version manifest/config digests.
