# Implement: 生产会话 cookie Secure 推导与有效期配置

## Checklist

1. [x] Tests first (`app/httpserver/production_session_test.go`, local runtime and entrypoint tests):
   - `validateProductionKeys`: `cookie.secure` in `[prod]` / DEFAULT → `CONFIG_KEY_INVALID` / `cookie.secure`.
   - `validateProductionSessionExpires` table per design (absent, env unset/blank, boundaries 5m / 8760h, rejects 4m59s / 8761h / 0s / -1h / 7d / abc, other `${X}`), key by source, error text excludes value.
   - Runtime test: `ValidateProductionRuntimeConfig` → `CookieSecure` and `SessionTTL`; actual entrypoint codec helper → cookie attributes and signed expiry.
   - Local runtime test: `CookieSecure` / `SessionTTL` follow `conf` values with existing 3h fallback.
2. [x] `production_config.go`: add `cookie.secure` to `forbiddenProductionKey`; share `parseProductionSessionExpires` between validation and typed runtime; call validation from `ValidateProductionConfig`.
3. [x] `session.go`: extract TTL parsing into a helper reused by `NewSessionCodec` and local runtime (no behaviour change).
4. [x] `production_runtime.go` / `local_runtime.go`: add and populate `CookieSecure`, `SessionTTL`.
5. [x] `cmd/leanote/main.go`: assign `Secure` / `TTL` from `runtimeCfg` through `sessionCodecForRuntime`.
6. [x] Deployment: `conf/app.conf-docker`, `docker-compose.yml` (`:-168h`), `.env.example`; actual Compose rendering verifies dev inherits the base env.
7. [x] Docs: README Compose / 反向代理 paragraph.
8. [x] `tests/js/release-contract.test.js`: conf/compose/.env.example assertions per AC4; CRLF-compatible related deployment checks.
9. [x] Spec update in Phase 3: `.trellis/spec/backend/quality-guidelines.md` startup matrix, `image-publishing.md` Compose variable list.

## Validation

```bash
go test ./app/httpserver ./cmd/leanote
gofmt -l app cmd
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go vet ./...
npm test
git diff --check
```

Container HTTPS / browser evidence: record as `unrun` unless executed.

## Risky files / rollback points

- `production_config.go` fail-closed startup path.
- `cmd/leanote/main.go` session wiring.
- `docker-compose.yml` / `conf/app.conf-docker` (release contract tests guard them).

## Evidence

See `validation.md` for executed checks and explicit external `unrun` gates.
Full Node suite: 245 passed, 0 failed, 1 skipped out of 246. The full-repository
format scan records 5 unchanged HEAD files; all changed Go files are formatted.
