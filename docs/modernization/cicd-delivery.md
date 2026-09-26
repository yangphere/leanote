# CI/CD delivery

Leanote publishes Linux/amd64 release tarballs and the matching GHCR image. The
release tag is strict `vX.Y.Z` and maps to
`ghcr.io/yangphere/leanote:vX.Y.Z`. There is no automatic production deployment.
The release workflow only creates the GitHub Release and pushes the immutable
image tag after the quality gate passes.

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

The image includes the pinned `wkhtmltopdf` runtime used by PDF export. arm64
support and platform-specific PDF work remain tracked as MOD-002 in the
[modernization backlog](../modernization-backlog.md#mod-002).

Release artifacts and CI summaries are retained for at most seven days and are
allowlisted and redacted; raw browser traces, screenshots, cookies, credentials,
and service logs are never published.
