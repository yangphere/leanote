# Design: Docker site.url from .env

## Boundaries

| Layer | Change |
|---|---|
| Deployment inputs | `.env.example`, `docker-compose.yml` (required interpolation), `conf/app.conf-docker` (`site.url=${LEANOTE_SITE_URL}`) |
| Interface (startup validation) | `app/httpserver/production_config.go` `ValidateProductionConfig` gains a site.url check |
| Service | unchanged — `service.SetAppConfigSource(cfg)` (`cmd/leanote/main.go:66`) keeps reading `site.url` from the validated `*Config` |
| Tooling | `scripts/container-smoke.sh`, `scripts/package-smoke.sh` add a literal `site.url` |
| Contract tests | `tests/js/release-contract.test.js`, `app/httpserver/production_config_test.go` |
| Docs | `README.md` Compose section; spec update in Phase 3 |

## Data flow

```
.env LEANOTE_SITE_URL
  → docker-compose.yml environment (Compose rejects missing/empty via :?)
  → container env
  → /etc/leanote/app.conf  site.url=${LEANOTE_SITE_URL}
  → ValidateProductionConfig: raw-section check + env lookup + validateSiteURL
  → ParseConfig(data,"prod") expands ${LEANOTE_SITE_URL}
  → service.SetAppConfigSource(cfg) → applySiteURLDomain / GetSiteUrl
```

## Validation contract

Inside `ValidateProductionConfig`, after the existing Mongo/secret checks and
before `ParseConfig`:

1. `raw, ok := prod["site.url"]`; absent → `CONFIG_KEY_INVALID` key `site.url`.
   DEFAULT-section `site.url` is not accepted as a substitute (same rule as
   secret/db: the value must be declared in `[prod]`). If DEFAULT also has it
   while prod overrides → `CONFIG_SOURCE_CONFLICT` key `site.url`.
2. Strip inline comment/quotes as the parser does (`stripQuotes`).
   - value == `${LEANOTE_SITE_URL}` → `os.LookupEnv("LEANOTE_SITE_URL")`;
     not present → `CONFIG_VALUE_MISSING` key `LEANOTE_SITE_URL`;
     empty after TrimSpace → `CONFIG_VALUE_EMPTY` key `LEANOTE_SITE_URL`;
     candidate = raw env value (not trimmed — whitespace is rejected below).
   - value contains `${` otherwise → `CONFIG_SOURCE_CONFLICT` key `site.url`.
   - else candidate = literal value.
3. `validateSiteURL(candidate)` → on failure `CONFIG_SITE_URL_INVALID` with key
   `LEANOTE_SITE_URL` when sourced from env, `site.url` when literal.

`validateSiteURL` (pure function, unit-tested table):

- `candidate != strings.TrimSpace(candidate)` → invalid.
- `url.Parse` error, `Opaque != ""` → invalid.
- `Scheme` must be exactly `http` or `https` (case-sensitive, because
  `applySiteURLDomain` matches lowercase prefixes; `url.Parse` lowercases the
  scheme, so compare against the original prefix).
- `User != nil`, `Path != ""`, `RawPath != ""`, `RawQuery != ""`,
  `ForceQuery`, `Fragment != ""`, or a trailing `?`/`#` in the raw string → invalid.
- `Host` non-empty; `Port()` if present must parse 1–65535;
  `domain.CanonicalizeBlogHost(u.Host)` must succeed.

Error strings stay `ConfigError{Code, Key}` only — never the URL value
(`.trellis/spec/backend/error-handling.md`, `logging-guidelines.md`).

## Compatibility

The project is still in development and has never been deployed (confirmed by
the user), so there are no existing Compose or package deployments and no
stored note content to migrate. Requiring `LEANOTE_SITE_URL` and a valid
`[prod]` `site.url` is therefore a clean contract, not a breaking change.

- The pinned published image (`LEANOTE_IMAGE_TAG=2.0.1`) still ships the old
  hard-coded conf; the variable takes effect with an image built from this
  change (dev override `leanote:local` or the next release).
- dev/test modes are untouched (`ValidateProductionConfig` runs only for prod).

## Rollback

Revert the commit: conf returns to the literal `http://127.0.0.1:9000`, the
Compose variable disappears, and validation is removed. No data migration is
involved.
