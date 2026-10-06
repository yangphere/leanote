# Design: 生产会话 cookie 的 Secure 推导与有效期配置

## Boundaries

| Layer | Change |
|---|---|
| Deployment inputs | `conf/app.conf-docker` (`session.expires=${LEANOTE_SESSION_EXPIRES}`), `docker-compose.yml` (`LEANOTE_SESSION_EXPIRES: ${LEANOTE_SESSION_EXPIRES:-168h}`), `.env.example` |
| Startup validation | `app/httpserver/production_config.go`: `forbiddenProductionKey` += `cookie.secure`; new `validateProductionSessionExpires(sections)` called from `ValidateProductionConfig` next to `validateProductionSiteURL` |
| Typed runtime handoff | `app/httpserver/production_runtime.go`: `ProductionConfig` += `CookieSecure bool`, `SessionTTL time.Duration`, derived in `ValidateProductionRuntimeConfig` |
| Local runtime | `app/httpserver/local_runtime.go`: `CookieSecure = cfg.BoolDefault("cookie.secure", false)`, `SessionTTL` = existing codec parsing (3h fallback) |
| Composition root | `cmd/leanote/main.go`: `sessionCodecForRuntime(cfg, runtimeCfg)` creates the codec and assigns its `Secure` / `TTL` from the typed runtime handoff; the helper is exercised by cookie/expiry regression tests |
| Session codec | `session.go` behaviour unchanged; extract the TTL parse into a small helper (`sessionTTLFromConfig`) shared with local runtime so dev keeps identical semantics |
| Docs | `.env.example`, `README.md` |
| Contract tests | `production_config_test.go`, runtime tests, `tests/js/release-contract.test.js` |

## Data flow

```
.env LEANOTE_SITE_URL ─────┐                 .env LEANOTE_SESSION_EXPIRES (optional)
                           │                   └─ compose ${..:-168h} ─▶ container env
/etc/leanote/app.conf  site.url=${LEANOTE_SITE_URL}   session.expires=${LEANOTE_SESSION_EXPIRES}
  ─▶ ValidateProductionConfig
       reject cookie.secure (prod/DEFAULT)                     (R2)
       validateProductionSiteURL                               (existing)
       validateProductionSessionExpires                        (R3)
  ─▶ ParseConfig(prod)
  ─▶ ValidateProductionRuntimeConfig
       CookieSecure = HasPrefix(site.url, "https://")
       SessionTTL   = productionSessionTTL(cfg)  // default 168h
  ─▶ main.go: SessionCodec{Secure, TTL} ─▶ Set-Cookie (Secure?, Max-Age=TTL), payload exp
```

## session.expires validation contract (prod)

Source = `[prod]` value if present, else DEFAULT value, else absent.
After `stripQuotes`:

| Raw | Env | Result |
|---|---|---|
| absent | – | 168h |
| `${LEANOTE_SESSION_EXPIRES}` | unset / blank | 168h |
| `${LEANOTE_SESSION_EXPIRES}` | value v | check(v), key `LEANOTE_SESSION_EXPIRES` |
| contains other `${` | – | `CONFIG_SOURCE_CONFLICT` / `session.expires` |
| literal v | – | check(v), key `session.expires` |

`check(v)`: `strings.TrimSpace(v)` then `time.ParseDuration`; error or outside
`[5*time.Minute, 8760*time.Hour]` → `CONFIG_SESSION_EXPIRES_INVALID`.
Errors never include the value.

`ValidateProductionConfig` and `ValidateProductionRuntimeConfig` both use
`parseProductionSessionExpires` through their small validation/runtime wrappers.
The runtime wrapper reads `cfg.data` with the same prod-over-DEFAULT precedence,
so source validation, optional-env defaults, duration bounds, and error keys
cannot drift. This also preserves source-aware errors when tests call the
runtime validator directly. No raw configuration escapes the typed handoff.

## Why the typed handoff

`ProductionConfig` is the documented sole startup handoff. Deriving both
values there keeps the rules testable without HTTP and avoids synthetic keys
in `*Config`. `NewSessionCodec(cfg)` remains for dev/test and tests.

## Compatibility

- dev/test: unchanged (`cookie.secure`, `session.expires=3h` from `conf/app.conf`).
- Production `http://` site.url: Secure unchanged (false); TTL 3h → 168h by default.
- Production `https://` site.url: cookie gains `Secure`; HTTP direct access cannot keep a session (documented).
- Existing signed cookies remain valid (format unchanged); their embedded `exp` is honored until re-issued.
- A production conf declaring `cookie.secure` or an invalid `session.expires` now fails startup (exit 78). No shipped conf does; project not yet deployed.

## Rollback

Revert the commit. No data migration; sessions issued with 168h `exp` stay
valid until that `exp` even after rollback (acceptable; rotate `LEANOTE_APP_SECRET`
to force logout if required).
