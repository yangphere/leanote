# Real GHCR publication

## Authorization and target

- On 2026-10-04 the user requested: “帮我真实在GHCR发布”. This authorizes
  the planned real image publication, including its scoped commit, remote
  candidate integration and the selected version tag push. No production deployment is
  included.
- Latest user-selected target: Git tag `2.0.1`, image `ghcr.io/yangphere/leanote:2.0.1`, `linux/amd64`.
- The user subsequently requested a `latest` image alias in addition to the
  build version. This authorizes updating latest to the verified 2.0.1 digest,
  and doing the same after future successful version publications. Preserve
  immutable version tags; no new version or Git tag is needed for this alias.
- `gh` is authenticated as repository administrator `yangphere`.
- Initial package/lock version was `1.0.0`; the latest user instruction requires `2.0.1`. Recheck remote `2.0.1` before tag creation.
- Remote master starts at `5bc6bd439b55c88f2aeb42477cc99dd38fc860be`.
  It has one unique merge commit; the current dev candidate has 227 unique
  commits. Integrate the candidate while retaining master history.
- The user's existing `CONTEXT.md` edit and Compose task are excluded from the
  publication commit.

## Target branch update

On 2026-10-04 the user changed the integration and publishing source to `main`
and explicitly requested `main` as the GitHub default branch. Remote `main`
does not yet exist. Create it while retaining existing master history, merge
the reviewed dev candidate into it, update CI and source checks to `main`, and
publish the selected `2.0.1` from that fixed candidate. The earlier master target is
superseded; no master push or tag push had occurred before this correction.

The native managed worktree at
`C:\Users\rog\.codex\worktrees\ghcr-v1-0-0\leanote` isolates integration from
the user's dirty dev checkout. Initial merge-tree preflight was conflict-free
and its content tree exactly matched implementation commit `e1b18995`.

The worktree directory retains its initial name; no `v1.0.0` tag was created
or published. On 2026-10-04 the user replaced that version with `v2.0.1` and
specified that the registry tag must omit the `v` prefix.

The user subsequently removed the `v` from the Git tag as well: both tags are
`2.0.1`, so this publication avoids the existing protected `v*.*.*` Release
trigger. No superseded tag was created or pushed.

## Initial execution state (historical)

Preparing a reviewed candidate. Real Actions execution, GHCR push/digest
read-back and anonymous pull are not yet verified. Any public-visibility change
must be recorded separately; initial GHCR package visibility defaults to private.

Recent repository CI runs failed. Read their real job diagnostics before the
publication tag; do not bypass the full reusable quality gate.

## Main integration and remote execution

- Work commit: `41ffaebd6a5f7ae81c4c3e20daf2ed6622009ea0`.
- Main merge: `dc323d9ef3b4f744735fb4665ab41d7009403e58`, preserving
  master `5bc6bd43` as first parent and reviewed dev `41ffaebd` as second.
- Main merge tree `31bac3ab3ead2bfa896ef82761dfd6d21d984ae2` equals the
  reviewed dev tree. No conflicts or extra content changes were introduced.
  A whitespace scan against historical master reports pre-existing archive
  logs/source whitespace; the scoped work diff passes and the merge has zero
  diff against the reviewed dev content. Historical whitespace was preserved.
- Pushed explicitly as `main:main`, tracking `origin/main`. GitHub default
  branch and local `origin/HEAD` are confirmed as `main`; master is retained.
- Main CI: [37172547373](https://github.com/yangphere/leanote/actions/runs/37172547373),
  candidate `dc323d9e`, failed only Chromium and its dependent summary.
  Node, Mongo, both Go versions, package smoke and container smoke passed.
  Chromium now requests `/tinymce/plugins/leaui_image/index.html`, but Go's
  static handler still returns its canonical index redirect (301). The
  manifest must use the corresponding directory URL for direct 200. This
  follow-up is being repaired before any tag creation.
- Existing user `CONTEXT.md` / Compose task were excluded and remain intact.

## Reviewed follow-up and version tag

- Follow-up work commit `9cca58c0dc549882186f355c5d2aca4ebe00cdfb`; main
  merge candidate `dea2306c32f27e648d9438cbc8b67cec6151b6cc`.
- [Main CI 37173341933](https://github.com/yangphere/leanote/actions/runs/37173341933)
  passed all seven primary jobs and summary. The protected handoff was skipped
  by its original v-tag rule. Real Chromium now passed.
- Created and pushed annotated Git tag `2.0.1` without force. Tag object
  `887a424375187212c6ee31b4fc904361cfac58e8` peels to the confirmed main
  candidate `dea2306c32f27e648d9438cbc8b67cec6151b6cc`.
- Only [Docker image 37173559882](https://github.com/yangphere/leanote/actions/runs/37173559882)
  was triggered by this numeric tag. Seven quality jobs, summary, image build
  and candidate smoke passed. Registry preflight failed before docker push:
  manifest returned structured `MANIFEST_UNKNOWN`, then listing returned 404.
  The old helper did not parse/print the listing error code, so `NAME_UNKNOWN`
  is not yet an observed remote result. No push/digest read-back is confirmed.

## Recovery without moving the version tag

- Preserve remote tag `2.0.1` and its peeled candidate SHA exactly.
- Repair initial-package classification and introduce a main dispatch that
  validates the original candidate evidence before rebuilding that candidate.
- Fixed recovery inputs: `tag=2.0.1`,
  `expected_commit=dea2306c32f27e648d9438cbc8b67cec6151b6cc`,
  `source_run_id=37173559882`, `source_run_attempt=1`.
- Source run's overall failure is accepted only with successful identity-bound
  primary quality jobs, summary and build/smoke; source summaries are also
  schema/provenance checked. Executor quality remains required separately.
- Recovery implementation and independent full-scope review passed. Integrated
  executor CI, dispatch and GHCR push/read-back are pending. Package public
  visibility and anonymous pull remain `unrun`.

## Integrated recovery executor

- Reviewed work commit `213b3bc8a6a09a5a259a1ef49ab121c3753b9089`.
- Main merge `30bf11451d96db9b0b4c178870a0d9b1918b0605` was conflict-free;
  its tree `971310cc09b81aab00585f3cebadb9dcb208f2b9` exactly matches reviewed
  dev content. Scoped merge diff whitespace check passed.
- Pushed `main:main`; remote default remains main. Remote `2.0.1` tag object
  and peeled candidate remain unchanged.
- [Executor CI 37175199730](https://github.com/yangphere/leanote/actions/runs/37175199730)
  passed all seven primary jobs and summary; protected handoff skipped under
  the original rule. Real Chromium, package and container smoke passed.
- Main SHA and version tag were rechecked immediately before dispatch.
  [Recovery 37175408129](https://github.com/yangphere/leanote/actions/runs/37175408129)
  was dispatched from reviewed main with the fixed recovery inputs above.
  Publication is in progress; no registry write/read-back is yet confirmed.

## Published image and independent read-back

- Recovery run `37175408129` passed validate, all seven quality jobs, summary,
  candidate build and smoke. Job `111357415300` accepted explicit initial
  package absence, then **actually pushed** `ghcr.io/yangphere/leanote:2.0.1`.
- Docker push printed registry manifest digest
  `sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`.
  The subsequent test failed because Buildx export manifest digest was
  `sha256:1550df70ffaf933b1c6c5c7b87313654d5e2fc2e44ef34a64e48faec2b45a026`.
  This run's overall conclusion remains failure; do not relabel it success or
  retry writing the existing tag. The digest boundary is under investigation.
- Independent anonymous registry read hashed the exact 2622-byte manifest.
  Hash, `Docker-Content-Digest` header, Docker push digest and anonymous pull
  RepoDigest all equal `sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`.
- Remote Docker v2 manifest config digest and anonymously pulled image Id both
  equal the exact Buildx-exported candidate config:
  `sha256:99b11b4586b2ea0549b2264bd80bc736fd57216ea2b0a7a4e7cca5661b9ee6a8`.
  Its config includes the rootfs identity. Image metadata confirms Linux/amd64,
  version `2.0.1`, revision `dea2306c32f27e648d9438cbc8b67cec6151b6cc`,
  and source `https://github.com/yangphere/leanote`.
- `docker --config <empty isolated directory> pull
  ghcr.io/yangphere/leanote:2.0.1` succeeded, downloaded and checksum-verified
  all 11 layers. Public accessibility and anonymous pull are verified. The
  agent did not change package visibility; who/when made it public is unknown.
- Existing version tag and remote image are preserved. Correct future
  build/load/push manifest verification before finishing implementation.

## Latest executor integration

- Work commit `a4e402d3` adds byte-preserving OCI archive transport and
  the latest-only promotion operation. Final independent review passed.
- Main merge `5e4452a8ef3d288b02807dc4af6b8f853c66d9f7` is conflict-free;
  content tree `fed03fdc333ce9a557fd8aa840d503fb9a4393e0` equals the
  reviewed dev tree. Scoped merge diff checks passed.
- Pushed explicitly as `main:main`. Default branch remains main; remote
  Git tag object `887a424375187212c6ee31b4fc904361cfac58e8` and its peeled
  candidate `dea2306c32f27e648d9438cbc8b67cec6151b6cc` remain unchanged.
- [Latest executor CI 37177660541](https://github.com/yangphere/leanote/actions/runs/37177660541)
  passed all seven primary jobs and summary, including real Chromium,
  package smoke and container smoke. Protected handoff skipped as intended.

## Successful latest promotion and anonymous verification

- [Latest promotion 37177847779](https://github.com/yangphere/leanote/actions/runs/37177847779)
  attempt `1`, `workflow_dispatch` from main executor `5e4452a8`, completed
  with overall **success**. Validate, all seven quality jobs, summary and
  publish passed. Original source evidence remains run `37173559882` attempt `1`.
- Exact dispatch inputs: `operation=update_latest`, `tag=2.0.1`,
  `expected_commit=dea2306c32f27e648d9438cbc8b67cec6151b6cc`,
  `expected_registry_digest=sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`,
  `expected_config_digest=sha256:99b11b4586b2ea0549b2264bd80bc736fd57216ea2b0a7a4e7cca5661b9ee6a8`,
  `source_run_id=37173559882`, `source_run_attempt=1`.
- Publish job `111364690701` verified and pulled the existing registry digest,
  checked metadata, ran the complete candidate smoke, and copied that digest
  only to latest. Candidate Buildx build and immutable version push steps
  were **skipped**. Pinned Skopeo 1.22.3 was confirmed on the real runner;
  latest manifest/config read-back passed. Job completed at `2026-10-04T04:49:28Z`.
- Independent anonymous HTTP verification at `2026-10-04T04:50:21Z` returned
  200 for both `2.0.1` and `latest`. Both raw manifests are 2622 bytes;
  exact-byte SHA256 and `Docker-Content-Digest` headers equal the manifest
  digest above. Both config descriptors equal the expected config above.
- Actual anonymous Docker pulls of both tags succeeded with an isolated
  configuration directory containing no `config.json` or registry credentials.
  Previously validated layers were reused; latest was newly resolved from GHCR.
  Both tags resolve to the same pulled Id/RepoDigest, Linux/amd64, version `2.0.1`,
  original revision `dea2306c` and repository source label.
- After promotion, remote Git tag object and peeled SHA remain unchanged;
  GitHub default remains main. No package visibility change or deployment was
  performed. The earlier recovery `37175408129` remains a failed run despite
  its real version push; this successful alias operation does not rewrite it.
- Fresh-version dual-export publication was verified against a real local
  registry; a new-version GHCR execution of that branch remains `unrun`.
  No additional version/tag was created merely to exercise it.

## Local closeout authorization

- At the user's cleanup request, the managed GHCR publication worktree was
  verified clean and equal to remote main `d525be93`, then archived with the
  native worktree tool. Only the primary dev workspace remains registered;
  main is retained and no user changes were removed.
- On 2026-10-04 the user requested local commit and Trellis task archive.
  Existing code/publication work is already committed through `bcc35812`;
  local archive and journal exclude `CONTEXT.md` and
  `10-04-compose-prod-dev-split`, and perform no additional remote push.
