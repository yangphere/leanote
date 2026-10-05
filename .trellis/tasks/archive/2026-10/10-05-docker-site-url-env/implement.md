# Implement: Docker site.url from .env

## Ordered checklist

1. [x] `app/httpserver/production_config_test.go` — add failing table tests for
       `validateSiteURL` and `ValidateProductionConfig` site.url cases (AC2, AC3).
       Existing fixtures that build a valid prod config must gain a `site.url`
       line so they keep passing.
2. [x] `app/httpserver/production_config.go` — implement `validateSiteURL` and
       the site.url source check per `design.md`; new code
       `CONFIG_SITE_URL_INVALID`.
3. [x] `conf/app.conf-docker` — `site.url=${LEANOTE_SITE_URL}`.
4. [x] `docker-compose.yml` — `LEANOTE_SITE_URL: ${LEANOTE_SITE_URL:?LEANOTE_SITE_URL must be set}`
       in `leanote.environment`.
5. [x] `.env.example` — new documented `LEANOTE_SITE_URL` block (R1).
6. [x] `scripts/container-smoke.sh` (`site.url=http://127.0.0.1:9000`),
       `scripts/package-smoke.sh` (`site.url=http://127.0.0.1:19090`).
7. [x] `tests/js/release-contract.test.js` — assert the three deployment
       contracts (AC4).
8. [x] `README.md` — Compose variable and reverse-proxy notes (R7).
9. [x] Check any other test/fixture that calls `ValidateProductionConfig` or
       writes a prod config (`grep -rn "\[prod\]" app cmd scripts tests`).

## Validation commands

```bash
GOTOOLCHAIN=local go test ./app/httpserver ./app/service ./cmd/leanote
gofmt -l app cmd
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go vet ./...
npm test
git diff --check
# Compose rendering (needs docker CLI):
LEANOTE_SITE_URL= docker compose --env-file .env.example config   # expect failure
docker compose --env-file .env.example config                      # expect success
```

Real container / reverse-proxy runs (AC5, AC7) stay `unrun` unless executed.

## Validation evidence

- `go test ./app/httpserver ./app/service ./cmd/leanote` passed.
- `go build ./...` and `go vet ./...` passed; changed Go files are gofmt-clean.
- `docker compose --env-file .env.example config --quiet` passed. An empty
  `LEANOTE_SITE_URL` failed with `LEANOTE_SITE_URL must be set`.
- The new release-contract assertion passed. The full Windows Node suite still
  has pre-existing `sh`/CRLF environment failures; Docker/browser and
  reverse-proxy acceptance remain `unrun`.
- `git diff --check` passed.

## Risk / rollback points

- `production_config.go` is the startup gate: a too-strict rule blocks boot.
  Keep the rule set exactly as in `design.md`; one commit, revertible.
- Smoke scripts run in CI (`container-smoke`, `package-smoke`); forgetting R6
  fails CI at startup.
