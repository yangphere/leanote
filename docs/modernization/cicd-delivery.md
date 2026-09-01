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
/app/files
/app/public/upload
```

The image includes the pinned `wkhtmltopdf` runtime used by PDF export. arm64
support and platform-specific PDF work remain tracked as MOD-002 in the
[modernization backlog](../modernization-backlog.md#mod-002).

Release artifacts and CI summaries are retained for at most seven days and are
allowlisted and redacted; raw browser traces, screenshots, cookies, credentials,
and service logs are never published.
