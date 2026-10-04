# Trellis Check Review — fixed-tag recovery

## Findings (fixed)

- No additional implementation defect was found during the independent
  recovery review, so no product-code fix was required.

## Findings (not fixed)

- The recovery executor has not yet run through the complete GitHub Actions
  gate from the integrated `main` commit. That remote run remains required
  before dispatching publication.
- GHCR push/digest read-back, package visibility and anonymous pull remain
  `unrun`. After a successful private first publication, making the package
  public is still a manual administrator action. The existing Git tag `2.0.1`
  must remain at `dea2306c32f27e648d9438cbc8b67cec6151b6cc`.

## Verification

- Lint: pass — Actionlint v1.7.7 checked `docker-image.yml`, `ci.yml`, and
  `quality-gate.yml` with no findings (`shellcheck` is unavailable and was
  explicitly disabled).
- TypeCheck: not applicable — changed runtime tooling is JavaScript/YAML.
  `node --check` passed for `verify-image-source-run.mjs`,
  `check-ghcr-tag-absent.mjs`, and `validate-summaries.mjs`.
- Tests: pass — independent publication/Release focused suites: 45 passed,
  0 failed, 0 skipped. Implementer evidence records the full Node suite as
  238 passed, 1 Windows-only skip, 0 failed before the final small format seam;
  the affected focused suites passed afterward.
- Live source verifier: pass — the new verifier independently accepted real
  Docker image run `37173559882`, attempt `1`, for tag `2.0.1` and candidate
  `dea2306c32f27e648d9438cbc8b67cec6151b6cc`.
- Source summaries: pass — task evidence records all eight downloaded real
  summaries passing the shared schema and explicit source-execution
  provenance validator.
- Protected files: pass — `Dockerfile`, `release.yml`, and
  `quality-gate.yml` have no recovery diff.
- Whitespace: pass — `git diff --check` reported only line-ending conversion
  warnings.

## Full-scope conclusion

The recovery design keeps the candidate and executor identities separate:

- Dispatch is restricted to `refs/heads/main` and accepts only a strict
  numeric tag plus explicit candidate SHA, source run ID and attempt.
- The remote tag is fetched and peeled to the candidate before work and again
  immediately before the sole push; candidate ancestry against refreshed
  `origin/main` remains required. Push and recovery share one target-tag
  concurrency lock, and registry presence still blocks overwrite.
- GitHub API verification binds the original repository, active workflow,
  push event, tag, SHA and latest explicit attempt. It requires validate,
  seven quality jobs and summary to succeed, plus candidate build/smoke to
  succeed and the original push step to fail.
- All eight downloaded summaries are checked through the shared strict schema
  validator and must match the original workflow, ref, commit, run ID and
  attempt. Current executor quality runs separately through the unchanged full
  reusable gate.
- The candidate checkout supplies package version, Docker build context,
  revision, epoch and smoke. The trusted executor checkout supplies the
  repaired GHCR helper. First-package creation accepts only the two documented
  structured 404 pairs under explicit policy; auth, identity, JSON, network,
  redirect and existing-tag states remain fail closed.
- Default permissions remain `contents: read`; only validate adds
  `actions: read`, and only publish adds `packages: write`. The protected
  Release path remains `v*.*.*`; image publishing remains numeric-only.

No local merge blocker remains. The recovery change is **ready to integrate**;
the next gates are the complete executor CI and then the already-authorized
fixed-tag dispatch, without moving or recreating `2.0.1`.
