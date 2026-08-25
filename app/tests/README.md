# Regression test environment

The legacy Revel route generator must run with **Go 1.20.14**. Go 1.26 and
1.27 currently panic in the pinned `golang.org/x/tools` type checker while
generating the legacy entrypoint. Install Go 1.20.14, then point the harness
at that executable before running HTTP integration tests:

```powershell
$env:LEANOTE_TEST_GO = 'C:\path\to\go1.20.14\bin\go.exe'
```

Run the following command from the repository root to start MongoDB 5.0 and
restore `mongodb_backup/leanote_install_data` into the isolated
`leanote_test` database:

```powershell
go run ./app/tests/harness/cmd/env up
```

The command waits for MongoDB, verifies that the restored fixture contains the
two expected users, and fails rather than using a different database. Stop and
remove the named test container when finished:

```powershell
go run ./app/tests/harness/cmd/env down
```

Replay is the default golden mode and never writes files:

```powershell
$env:LEANOTE_GOLDEN = 'replay'
go test -p 1 ./app/tests/... -count=1 -timeout 30m
Remove-Item Env:LEANOTE_GOLDEN
```

CI verifies `TestAuth` immediately after fixture restore with
`LEANOTE_REQUIRE_MONGO=1`; a missing MongoDB is therefore a failure in CI,
while local runs retain the useful skip behavior.

Only an explicit `LEANOTE_GOLDEN=record` invocation may update golden files.
`record` is for reviewed baseline updates only; CI runs `replay`. `ExportPdf`
is recorded on Linux through the manual GitHub Actions job because the legacy
controller shells out through `/bin/sh` and requires wkhtmltopdf at
`/usr/local/bin/wkhtmltopdf`.

Until the reviewed `note_exportPdf.json` artifact is committed, replay prints an
explicit skip directing maintainers to that manual record job; it never creates
the missing golden implicitly.

The integration tests restore the fixture before starting their server, so a
second replay starts from the same database state. Stop any standalone test
container after manual work with `go run ./app/tests/harness/cmd/env down`.
The `[test]` configuration binds the generated server to `127.0.0.1`; this
keeps Windows firewall prompts scoped out of the regression run without
changing the production `0.0.0.0` listener.
