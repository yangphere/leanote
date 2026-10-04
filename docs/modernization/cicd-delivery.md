# CI/CD delivery

Leanote has two tag-triggered delivery paths. The lightweight image workflow
publishes only a Linux/amd64 GHCR image after the complete quality gate and an
exact-candidate container smoke. The protected release workflow publishes the
tarball and GitHub Release only after its additional browser, delivery and
approval gates. In the lightweight path, a strict Git tag `X.Y.Z` maps to
`ghcr.io/yangphere/leanote:X.Y.Z` and its `latest` alias; neither path deploys production.

## Lightweight GHCR image delivery

`.github/workflows/docker-image.yml` responds to pushed unprefixed `X.Y.Z` tags
and explicit recovery dispatches from main. It
rejects forced updates, requires the peeled tag commit to be an ancestor of
`origin/main`, reuses `quality-gate.yml`, and builds one `linux/amd64`
candidate with the same version, revision, epoch and OCI-created inputs as the
protected release. One Buildx invocation exports both the loaded smoke image
and an OCI archive. Before smoke, the archive's exact manifest hash/config
must match Buildx metadata, and config must match loaded image Id. Skopeo
copies the same archive with `--preserve-digests` to the immutable version;
raw registry manifest/config read-back must match. It then updates `latest`
from that archive and verifies the same identities. There are no other
short-version aliases. Docker Engine load/push can reserialize manifests;
its push digest cannot be compared directly to the Buildx export digest.

The workflow uses `GITHUB_TOKEN` with `packages: write` only in the publish job.
GitHub's [Container registry documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
states that a repository workflow can publish an associated package with
`GITHUB_TOKEN`, and that the first publication creates a private package. The
Dockerfile's `org.opencontainers.image.source` label identifies this repository.
No empty package must be created in advance. Initial creation is nevertheless
an explicit workflow policy: authenticated queries to the exact
`yangphere/leanote` tag-list endpoint must return structured 404 `NAME_UNKNOWN`,
and the manifest endpoint structured 404 `NAME_UNKNOWN` or `MANIFEST_UNKNOWN`;
a returned `detail.name` must also match that image.
Existing packages require `MANIFEST_UNKNOWN` plus a successful, identity-bound
tag listing. Authentication, authorization, transport, malformed JSON and all
other registry states block the push.

Recovery preserves the original tag. Dispatch supplies `tag`, `expected_commit`,
`source_run_id`, and `source_run_attempt`. The main executor verifies the
original Docker image push run's repository/workflow/tag/SHA/attempt, successful
seven quality jobs and summary, and candidate build/smoke. It downloads original
summaries and uses the shared schema validator with explicit source provenance.
The original run may have an overall failure from the publish step; executor
quality success is not a substitute for candidate evidence. Executor quality
still runs in full. Build, revision, epoch and smoke use the original candidate;
registry preflight uses the repaired executor helper. Push and recovery share
the shared `docker-image-latest` lock across all versions. Version tags are
never overwritten; latest is a mutable alias for the successful version.

Dispatch `operation=recover_version` handles an absent version.
`operation=update_latest` handles an already published version without
rebuilding or repushing it. It requires `expected_registry_digest` and
`expected_config_digest`, verifies the version's raw manifest, pulls the exact
digest, checks config Id/platform/version/revision/source and runs candidate
smoke. Only then does it copy that registry digest to latest and verify exact
raw manifest/config equality. If latest fails after version success, recover
only the alias; multi-tag publication is not atomic.

After the first successful run, an administrator must open the `leanote`
package settings and change visibility to public. GitHub documents that new
packages are private by default and that changing a package to public is not
reversible. Public visibility must then be verified by an anonymous
`docker pull ghcr.io/yangphere/leanote:2.0.1`; workflow success alone is not
anonymous-pull evidence.

Also pull `ghcr.io/yangphere/leanote:latest` anonymously and confirm its
RepoDigest equals the version tag's digest.

`docker-image.yml` uses unprefixed version tags; `release.yml` retains its
`v*.*.*` trigger and `vX.Y.Z` image tags. Pushing `2.0.1` therefore triggers only
image delivery. The reusable quality gate still runs all seven primary jobs
and summary; its protected `release-inputs` handoff remains limited to `v` tags.
Image-only delivery does not authorize a protected GitHub Release, which still
requires every additional protected gate.

First publication checklist:

- [ ] Merge the reviewed `dev` commit into `main`.
- [ ] Confirm `docker-image.yml` is the only intended publishing path for this tag.
- [ ] Set package/lock version to `2.0.1`, then create and push `2.0.1` on the selected `main` commit.
- [ ] Confirm the workflow's registry digest read-back succeeds.
- [ ] Change the new `leanote` package visibility to public in GitHub package settings.
- [ ] From an unauthenticated client, pull `ghcr.io/yangphere/leanote:2.0.1` and record the digest.

## Protected delivery gates and authorization

The tag workflow requires validate, quality-gate, browser-evidence and
delivery-evidence before publish. The delivery artifact is separate from
the five-file release handoff and two-file browser handoff:
leanote-delivery-evidence-v1 contains only catalog.json and
delivery-evidence.json. Its catalog binds upstream acceptance documents,
the 38 notes action IDs, unchecked presentation operations and extended
environment/failure scenarios by source hashes. Counts alone are not proof
that the underlying business assertions ran.

Provision a self-hosted runner with the protected-delivery label and the
delivery-validation environment. Its LEANOTE_DELIVERY_RUNNER_CONFIG
environment variable names an absolute protected JSON file:

~~~json
{
  "executable": "/opt/leanote-delivery/bin/verified-runner",
  "sha256": "<reviewed executable SHA-256>",
  "args": [],
  "timeout_ms": 7200000
}
~~~

The executable and all its dependencies must be reviewed and immutable; a
hash of a generic interpreter alone does not bind scripts supplied in args.
The workflow serializes this repository's delivery runs under a single
protected-delivery-resources concurrency group because the native fixture
uses fixed ports/container names. Provision an exclusive runner pool; a
repository lock does not serialize unrelated repositories on the same host.
The wrapper invokes it with absolute --catalog and --result paths in a
private temporary directory, in a clean candidate checkout. The runner must
execute every catalog scenario, perform its process/container/fixture cleanup,
and return {cleanup_status, records}. Each record supplies the exact scenario
ID, discovered/executed/passed/failed/skipped counts, exit code, cleanup state,
assertion identifiers, environment identifier and fixture hash. Missing,
duplicate, skipped, zero-execution or unclean records block publication.
Raw process output is never an artifact. delivery-failure-<run>-<attempt>
is a separate diagnostic artifact with sanitized stage/exit/cleanup data;
it cannot satisfy a release gate. If the process is killed before confirming
cleanup, cleanup remains unknown. Removing evidence files does not prove
fixture cleanup.

**Implementation boundary:** the wrapper, catalog and gate are implemented;
the complete protected scenario executable and its 38-action HTTP/DB,
kill/restart/failpoint, SMTP/backup, Linux/PDF and browser mappings are not
provided by this repository change. No synthetic runner may stand in for
them. Until reviewed adapters and isolated resources are provisioned, the
extended gate is blocked and a release must not be attempted.

Configure a protected release environment with required reviewers and
deployment branch/tag restrictions. Reviewers must approve the executor
revision (especially recovery dispatch branches) and the target candidate.
Set its RELEASE_APPROVED_IDENTITY variable to the explicitly authorized
yangphere/leanote@vX.Y.Z:<full-candidate-commit>. Scripts never construct an
approval from workflow inputs. Administrators must prevent unreviewed
overrides. Changing code is not release authorization.

Protected publication rejects any existing image or Release. A typed GHCR
MANIFEST_UNKNOWN is trusted only after a successful authenticated listing
of that exact package. NAME_UNKNOWN, denied access, rate limits, transport
and JSON errors block writes. This protected workflow therefore requires an
existing readable package and does not use the lightweight workflow's explicit
first-package exception. Check this constraint before the first protected
release.

## Recovering an interrupted release

Use Recover Release only when the original image has already been published.
Dispatch from an approved executor revision with tag, commit, source_run,
source_attempt, inputs_artifact, browser_artifact and delivery_artifact.
The last three are immutable artifact IDs, not names or newest artifacts.
All must remain available within retention.

Recovery and normal publication hold the identical repository-scoped lock
release-refs/tags/<tag> with cancellation disabled. Recovery verifies the
original push workflow, attempt-specific required jobs and repository/artifact
identities before downloading. The original overall run may have failed in
publish; its preceding gates must have succeeded. Artifact contents must
bind the original run/attempt. Partial job reruns do not authorize mixing
attempts. A revoked workflow or expired artifact blocks recovery.

The current executor stays at the workspace root; a separate candidate/
checkout supplies the original package/lock version, source epoch and
acceptance catalog. Recovery never changes GITHUB_RUN_ID or attempt to
impersonate the original run. It does not build, push images, move tags,
delete releases, overwrite assets or fill in an incomplete existing Release.

When the exact tag, image digest/OCI metadata and original gates match,
and the Release is explicitly absent, recovery creates it once from the
original tarball, checksum and build metadata. It then reads back the formal
Release and hashes the three asset downloads. A fully matching existing
Release returns already-complete without writes. An uncertain create response
causes read-only reconciliation, never an automatic second create. Partial,
conflicting or unknown state remains blocked/unconfirmed.

release-result.json records the mode, target, source execution/artifact IDs,
current executor and result. Failures preserve a named stage and sanitized
reason; publication-unconfirmed means a write was attempted without a verified
complete result. A failed image push runs the read-only reconcile command;
its result does not change the failed job into successful publication.
source-verified and preflight-passed describe intermediate checks only.
If checkout, cancellation or artifact download prevents later steps, these
are not publication success. Inspect the workflow conclusion as well.

Before actual publication, retain this unchecked operational checklist:

- [ ] Freeze and approve a clean candidate SHA, strict tag and image digest.
- [ ] Provision reviewed delivery adapters and isolated Mongo/SMTP/volume/PDF resources.
- [ ] Execute real Chrome/Edge/Firefox/Safari current and previous slots plus business checks.
- [ ] Verify release reviewers, branch restrictions and exact approval identity.
- [ ] Verify target-package read access and original artifact retention.
- [ ] Obtain authorization for this specific remote publish/recovery operation.
- [ ] Record actual tag, GHCR digest and Release asset read-back evidence.

## Production configuration

The production entry point must be invoked exactly as follows:

```text
/app/bin/leanote -conf /etc/leanote/app.conf -runMode prod
```

`/etc/leanote/app.conf` must be a regular read-only file with mode `0440`. Its
`[prod]` section must use the following sensitive-value interface:

```ini
[prod]
db.urlEnv=${MONGODB_URL}
db.dbname=leanote
app.secret=${LEANOTE_APP_SECRET}
content.private.data=/var/lib/leanote/private/files
content.private.quarantine=/var/lib/leanote/private/quarantine
content.public.data=/var/lib/leanote/public/upload
content.public.quarantine=/var/lib/leanote/public/quarantine
content.temporary=/var/lib/leanote/tmp
admin.backup.root=/var/lib/leanote/backup
```

`MONGODB_URL` and `LEANOTE_APP_SECRET` are the only runtime sources for the
MongoDB URL and application secret. The URL must be a valid non-localhost
`mongodb://` or `mongodb+srv://` URI whose decoded database path exactly equals
`db.dbname`, and the database must not be `leanote_test`. The secret must be at
least 32 printable ASCII bytes and must not be the repository default.

The file structure is validated before either environment value is read and
before HTTP bind or MongoDB dial. Missing or conflicting values fail closed with
a stable configuration error and process exit `78`; there is no fallback to
`conf/app.conf`, `conf/app.conf-default`, localhost, host/port settings, or
undeclared environment aliases. A valid configuration with an unavailable
MongoDB keeps the server available for `GET /healthz`, which returns `503` and
`{"status":"not_ready"}\n` until MongoDB ping succeeds. A ready service returns
`200`, `Content-Type: application/json; charset=utf-8`, and
`{"status":"ready"}\n`. The health response never contains configuration,
credentials, version, or user data.

## Container volumes and support matrix

The image runs as UID/GID `10001:10001`, targets `linux/amd64`, and requires an
external MongoDB 8.0 service. Mount these persistent volumes:

```text
/var/lib/leanote/private
/var/lib/leanote/public
/var/lib/leanote/backup
```

The image provides `/var/lib/leanote/tmp` as a non-root writable temporary
directory. Runtime startup does not migrate or fall back to the legacy
locations. `/upload/*` and `/public/upload/*` retain their URLs and read from
`content.public.data`.

For an existing Docker deployment, do not attach an old volume directly to
`/var/lib/leanote/private` or `/var/lib/leanote/public`: the old volume root
contains files, while the new contract requires `private/files` and
`public/upload`. Stop the service, create new target volumes, and copy the
contents into the required subdirectories before switching the container:

```sh
docker volume create leanote-private
docker volume create leanote-public
docker volume create leanote-backup
docker run --rm --user 0:0 \
  --mount source=leanote-files,target=/from-files,readonly \
  --mount source=leanote-upload,target=/from-upload,readonly \
  --mount source=leanote-private,target=/to-private \
  --mount source=leanote-public,target=/to-public \
  --entrypoint sh ghcr.io/yangphere/leanote:<tag> -eu -c '
    mkdir -p /to-private/files /to-private/quarantine \
      /to-public/upload /to-public/quarantine
    cp -a /from-files/. /to-private/files/
    cp -a /from-upload/. /to-public/upload/
    chown -R 10001:10001 /to-private /to-public
  '
```

Mount `leanote-private:/var/lib/leanote/private`,
`leanote-public:/var/lib/leanote/public`, and
`leanote-backup:/var/lib/leanote/backup` after the copy. The command preserves
the old data and upload volumes so they can be removed only after a successful
read-only verification.

The release tarball contains the application prefix (`bin/`, `app/`, `conf/`,
`messages/`, and `public/`) only. It intentionally does not contain
`/var/lib/leanote`, because extracting under `/app` would otherwise create
`/app/var/lib/leanote` while production configuration requires the absolute
paths above. Before starting a tarball installation, create the roots and set
ownership for the service account (replace `10001:10001` when using another
account):

```sh
sudo install -d -o 10001 -g 10001 -m 0750 \
  /var/lib/leanote/private/files \
  /var/lib/leanote/private/quarantine \
  /var/lib/leanote/public/upload \
  /var/lib/leanote/public/quarantine \
  /var/lib/leanote/backup \
  /var/lib/leanote/tmp
```

For a legacy tarball installation, stop Leanote first, create the new roots,
copy hidden files as well as ordinary files, then verify the application before
removing the old directories:

```sh
sudo cp -a /app/files/. /var/lib/leanote/private/files/
sudo cp -a /app/public/upload/. /var/lib/leanote/public/upload/
sudo chown -R 10001:10001 /var/lib/leanote/private /var/lib/leanote/public
```

The image no longer bundles a PDF binary: `docker-compose.yml` runs a pinned
Gotenberg service on an internal-only `pdf` network and `conf/app.conf-docker`
selects it (`pdf.renderer=gotenberg`, `pdf.gotenberg.url=http://gotenberg:3000`).
The tar package still uses the pinned `wkhtmltopdf` process backend. arm64
support and platform-specific PDF work remain tracked as MOD-002 in the
[modernization backlog](../modernization-backlog.md#mod-002).

Release artifacts and CI summaries are retained for at most seven days and are
allowlisted and redacted; raw browser traces, screenshots, cookies, credentials,
and service logs are never published.
