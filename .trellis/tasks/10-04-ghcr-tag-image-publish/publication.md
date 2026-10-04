# Real GHCR publication

## Authorization and target

- On 2026-10-04 the user requested: “帮我真实在GHCR发布”. This authorizes
  the planned real image publication, including its scoped commit, remote
  candidate integration and the selected version tag push. No production deployment is
  included.
- Latest user-selected target: Git tag `2.0.1`, image `ghcr.io/yangphere/leanote:2.0.1`, `linux/amd64`.
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

## Execution state

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
