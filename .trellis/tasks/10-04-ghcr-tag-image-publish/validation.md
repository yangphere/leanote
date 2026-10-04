# Validation

## Automated checks

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

## External/runtime evidence

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
