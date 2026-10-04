# Validation

## Initial candidate automated checks (historical)

| Check | Result | Evidence |
|---|---|---|
| Red test before implementation | passed | `node --test tests/js/docker-image-workflow.test.js`: 0 passed, 7 failed because the workflow and helper did not exist |
| Focused contract tests after review fixes | passed | `node --test tests/js/docker-image-workflow.test.js`: 10 passed, 0 failed |
| Main-session final publication/release regressions | passed | With Git Bash on the test process PATH: `node --test tests/js/docker-image-workflow.test.js tests/js/release-contract.test.js`: 37 passed, 0 failed, 0 skipped |
| Full Node suite before review fixes | passed | With `C:\Program Files\Git\bin` added to the test process PATH: `npm test`: 230 tests, 229 passed, 0 failed, 1 platform skip; duration 261537 ms. The final focused suites cover the subsequent local review fixes; the expensive existing build suite was not repeated. |
| Initial full Node run | environment failure, superseded | Without Git Bash on PATH: 228 passed, 1 failed, 1 skipped; the existing `release-contract.test.js` failed only with `spawnSync sh ENOENT`. The corrected run above passed that test. |
| Workflow parser/lint | passed | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= .github/workflows/docker-image.yml`; actionlint v1.7.7, no findings |
| Standalone PyYAML parse | unavailable | Local Python has no `yaml` module; actionlint performed the workflow YAML parse. No dependency was installed for this duplicate check. |
| Diff whitespace | passed | `git diff --check`; only existing line-ending conversion warnings were printed |
| Trellis context | passed | `python ./.trellis/scripts/task.py validate .trellis/tasks/10-04-ghcr-tag-image-publish`: implement 6 entries, check 5 entries |
| Node syntax | passed | `node --check` on the helper and new test file |
| Required existing files preserved | passed | `git diff --exit-code -- Dockerfile .github/workflows/release.yml .github/workflows/quality-gate.yml` |

`shellcheck` is not installed, so actionlint was run with `-shellcheck=`. The
focused Node tests exercise registry status/JSON/size/identity/network/timeout
failure classification through injected `fetchImpl`; they do not contact GHCR.

Independent Trellis review and four local fixes are recorded in
[check-review.md](check-review.md). The main session verified the final 37-test
publication/release suites and Actionlint result. No in-scope defect remains.

## Spec sync and handoff

- Added the seven-section executable infrastructure contract at
  `.trellis/spec/backend/image-publishing.md`, linked the backend index and
  curated both task context manifests.
- Local implementation and quality checks are complete. Task status remains
  `in_progress`; no commit or archive was requested or performed.
- Existing `CONTEXT.md` changes and the Compose planning task were preserved.

## Initial candidate external/runtime evidence (superseded version)

| Gate | Status | Notes |
|---|---|---|
| Real `v1.0.0` tag push | `unrun` | No tag was created or pushed by this task |
| GitHub Actions `docker-image.yml` execution | `unrun` | Requires the user-authorized remote tag push |
| Real GHCR first-package `NAME_UNKNOWN` response compatibility | `unrun` | Static/helper tests fail closed on unknown shapes; actual GHCR response is not inferred |
| Real image build and `container-smoke.sh` candidate run | `unrun` | The workflow was not executed locally or remotely |
| GHCR push and manifest digest read-back | `unrun` | No remote write was attempted |
| Package visibility changed to public | `unrun` | Manual, irreversible GitHub setting after first publication |
| Anonymous `docker pull ghcr.io/yangphere/leanote:v1.0.0` | `unrun` | Must be performed after public visibility is confirmed |

## Source verification

GitHub documentation was read on 2026-10-04:

- <https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry>
- <https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility>

It supports `GITHUB_TOKEN` publication from a repository workflow and states
that first publication creates a private package. It does not document a
stable `NAME_UNKNOWN.detail.name` field, so the helper treats that field as
optional but rejects it when present and conflicting.

## Revised main / 2.0.1 candidate

The latest user instruction selects Git tag `2.0.1` and unprefixed registry
tag `2.0.1`, with `main` as source/default branch. No `v1.0.0` was published.
The earlier local checks do not establish that this revised candidate passes.
Four actual remote-CI blockers are being repaired as documented in
`remote-ci-preflight.md`; independent review, main CI, tag publication and
registry read-back evidence will be recorded here and in `publication.md`.

| Revised check | Result | Evidence |
|---|---|---|
| TinyMCE focused manifest regression | passed | 1/1; output remains `public/tinymce/...`, URL is `/tinymce/...` |
| Manifest-driven build | passed | `npm run build` exit 0, no tracked generated-output drift |
| Complete build-pipeline test file | passed | 37 tests, 36 passed, 0 failed, 1 Windows skip |
| Local Chromium build smoke | unrun | Existing-service and E2E credential variables absent; main CI will exercise it |
| Numeric image / protected release regressions | passed | 39/39 (`docker-image-workflow.test.js` + `release-contract.test.js`), Git Bash on PATH |
| CI-like provenance negative regressions | passed | `GITHUB_WORKFLOW=CI` ambient environment, both intended negative cases pass (2/2) |
| Real focused Mongo/HTTP harness | passed | `GOTOOLCHAIN=local go test ./app/tests/harness -run 'TestGoldenWebOwnershipControllers\|TestWebAdminMemberAndControllerSmoke' -count=1 -timeout 60s`: exit 0, 14.469s, 2 executed / 0 skipped. Fixture restore and native server requests execute; managed fixture cleanup explains absent container afterward. Exact Mongo provisioning mode was not captured. |
| Numeric and protected tag modes | passed | Image `2.0.1` accepted; prefixed/mismatched image tags rejected; protected mode retains `vX.Y.Z` |
| Shared package tag boundary | passed | 41/41 publication + Release focused tests; a real `refs/tags/2.0.1` `sh/package.sh` run with a fake Go compiler produced `leanote-v2.0.1-linux-amd64.tar.gz`; `v2.0.1` remains accepted and malformed/mismatched tags fail |
| Revised workflow Actionlint | passed | v1.7.7, YAML parsed, shellcheck unavailable and explicitly disabled |

Local runtime versions: Go 1.27.1 (Windows/amd64), Node 24.21.0, npm 11.19.0.
GitHub CI will supply the pinned Linux toolchains and Chromium runtime evidence.

## First real main CI

[37172547373](https://github.com/yangphere/leanote/actions/runs/37172547373)
at `dc323d9e`: six primary jobs passed (Node, Mongo, Go 1.26.7/1.27.0,
package smoke, container smoke). Chromium failed because the canonical
TinyMCE `index.html` URL still redirects (301); summary correctly failed.
No publication tag was created. A follow-up uses canonical directory URLs
for index resources, retaining disk outputs and the direct-200 smoke gate.

Follow-up verification: explicit `/tinymce/plugins/leaui_image/index.html`
returns 301 with `Location: ./`; the manifest now emits the directory URL.
Both `/tinymce/plugins/leaui_image/` and
`/tinymce/plugins/leaui_mindmap/mindmap/` return direct 200 from the existing
local service. Focused manifest regression passed 1/1; `npm run build` passed
without tracked generated-output drift. Full Chromium evidence awaits the
next real main CI.

## Passing main candidate and real tag

- [Main CI 37173341933](https://github.com/yangphere/leanote/actions/runs/37173341933)
  at `dea2306c32f27e648d9438cbc8b67cec6151b6cc`: seven primary jobs and
  summary **passed**, including real Chromium; protected `release-inputs`
  correctly skipped.
- Git tag `2.0.1` creation/push **passed**; remotely peeled tag matches the
  reviewed main candidate. The old `v1.0.0` and `v2.0.1` proposals were not
  created or pushed.
- [Docker image 37173559882](https://github.com/yangphere/leanote/actions/runs/37173559882)
  finished with publication failure before push. Seven quality jobs, summary,
  immutable image build and candidate smoke passed. Manifest returned
  structured `MANIFEST_UNKNOWN`; listing returned 404 but its body was not
  parsed by the old helper. Do not infer its exact error code from status alone.

## Fixed-tag recovery

Preserve `2.0.1` at `dea2306c32f27e648d9438cbc8b67cec6151b6cc`.
Recovery from main binds that candidate and source run `37173559882` attempt
`1`; executor and source-candidate quality evidence are checked separately.
Local recovery tests/review, executor main CI and actual dispatch publication
are pending. GHCR push/read-back, public visibility and anonymous pull remain
`unrun`; successful original build/smoke is not publication evidence.

Recovery local evidence (2026-10-04):

- Implementer full Node suite: 239 tests, 238 passed, 1 Windows skip, 0 failed
  (227s), before the final small version-format seam; affected focused suites
  were then rerun successfully.
- Main-session final focused publication/Release suites: 45/45 passed, 0 skips,
  6.3s; Git Bash on PATH. Node syntax and Actionlint v1.7.7 passed in implement
  validation; shellcheck remains unavailable.
- Main-session live read-only GitHub API verification of source run
  `37173559882`, attempt `1`, passed. Eight real downloaded source summaries
  passed shared schema/source-execution validation.
- Final independent recovery review passed: 45/45 focused tests, Actionlint,
  Node syntax, live source verifier, Trellis validation and diff checks. No
  local merge blocker; see `check-review-recovery.md`. Remote executor
  CI `37175199730` subsequently passed all seven jobs and summary at
  `30bf11451d96db9b0b4c178870a0d9b1918b0605`. Recovery dispatch
  `37175408129` is in progress; GHCR/public/anonymous evidence is pending.

## Real publication and digest boundary

| Gate | Result | Evidence |
|---|---|---|
| Recovery source API/artifacts | passed | Real Actions validate bound original run/attempt; downloaded and validated 8 summaries |
| Recovery executor quality | passed | Seven primary jobs and summary in `37175408129` |
| Actual candidate build/smoke | passed | Publish job `111357415300` succeeded before push |
| First package preflight and docker push | passed | Explicit creation accepted, docker push returned `2.0.1` registry digest |
| Workflow Buildx-manifest comparison | failed | Export digest `1550df70...` differs from Docker-pushed `0b67446e...`; run overall failure |
| Independent registry bytes/header/push digest | passed | Anonymous exact-byte SHA256 is `0b67446e...`, matching content-digest header and push output |
| Exact candidate config binding | passed | Registry config descriptor and pulled Id equal exported config `99b11b45...`; platform/version/revision labels match |
| Anonymous real pull | passed | Empty isolated Docker config; 11 layers downloaded and checksum verified; RepoDigest matches |
| Package publicly accessible | passed | Anonymous registry and pull succeeded; agent did not change visibility, actor/time unknown |

The published manifest is Docker v2. Future manifest verification remains to
be corrected after proving the build/export/load/push serialization boundary.
The immutable `2.0.1` Git/image tag must remain unchanged; its latest alias
is separately authorized by the user's subsequent request.

## Latest and byte-preserving transport validation

- User requested latest in addition to the immutable version. Implementation
  adds one-build OCI archive publication and explicit existing-version promotion;
  the version remains unchanged.
- Real local registry reproduced BuildKit load/Engine push manifest
  reserialization with stable config. Forcing Docker media types did not fix it.
- A single Buildx invocation dual-exported loaded candidate and OCI archive.
  Loaded Id matched archive/metadata config; pinned Skopeo 1.22.3 copy to both
  version/latest preserved raw manifest SHA, registry header and config.
- `inspect --raw` redirected file retained exact bytes and digest. Ubuntu 22.04
  Skopeo 1.4.1 actually rejected `--preserve-digests`; workflow uses the tested
  full-digest-pinned container instead. See `digest-diagnosis.md`.
- Latest focused/lint checks and final independent review passed; see
  `check-review-latest.md`. Main-session publication/Release regressions passed
  47/47 with no skips. Trellis validation and scoped diff checks passed.
  Integrated main CI and actual GHCR latest promotion subsequently passed
  as recorded below.

## Final real latest evidence

| Gate | Result | Evidence |
|---|---|---|
| Reviewed integration | passed | Work `a4e402d3`, main merge `5e4452a8`; identical reviewed/merged content tree `fed03fdc333ce9a557fd8aa840d503fb9a4393e0` |
| Main executor CI | passed | [37177660541](https://github.com/yangphere/leanote/actions/runs/37177660541): seven primary jobs and summary, including actual Chromium and smoke |
| Latest operation | passed | [37177847779](https://github.com/yangphere/leanote/actions/runs/37177847779), main dispatch attempt 1; overall success |
| Original candidate/source gates | passed | Validate verified original numeric push run `37173559882`, attempt 1 and downloaded summary provenance; executor quality also passed separately |
| Exact existing version smoke | passed | Publish job `111364690701`: digest pull, config/platform/version/revision/source validation and full candidate smoke succeeded |
| No immutable version write | passed | Publish Buildx build and version push steps skipped; one registry-to-registry copy wrote only latest |
| Runner copy tool/raw read-back | passed | Pinned Skopeo 1.22.3 confirmed; latest raw manifest/config matches expected existing-version digests |
| Anonymous registry bytes | passed | Both tags return 200 and 2622 bytes; raw SHA/header `sha256:0b67446ea183a69aea3a35ede6dc187d85b5bb1e3e031ecd9cc0612c02a46c9a`, config `sha256:99b11b4586b2ea0549b2264bd80bc736fd57216ea2b0a7a4e7cca5661b9ee6a8` |
| Anonymous actual pulls | passed | Empty isolated Docker auth configuration; latest and 2.0.1 pulled successfully, same Id/RepoDigest and original metadata; cached verified layers reused |
| Git/default branch preservation | passed | Remote annotated 2.0.1 object/peeled SHA unchanged; default main confirmed after latest publication |
| Future fresh-version GHCR dual-export branch | unrun | Actual dual-export and digest-preserving version/latest transfer passed against a real local registry; current GHCR run exercised existing-version promotion only |

Earlier failed Actions remain failed historical evidence. The task stays active;
no archive or production deployment was requested. User edits in `CONTEXT.md`
and the Compose planning task remain excluded from all publication commits.
