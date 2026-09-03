# Error Handling

> How errors are handled in this project.

---

## Overview

- Go error convention throughout: return errors upward with `fmt.Errorf("...: %w", err)` wrapping; log once at the boundary that terminates handling (`cmd/leanote/main.go`); never log-and-return at every layer.
- Lower-layer errors are returned to the caller with context; handlers must not convert database initialization or query failures into successful empty data.
- HTTP test-only identity endpoints use an explicit status matrix: requests outside test mode or loopback return `404`; marker, database, token digest, or time-boundary validation failures return `503` without sensitive details.
- Boundary checks are inclusive at the documented limit. The e2e marker future skew accepts exactly `validationNow + 60s` and rejects any later timestamp.

## Error Types

- `httpserver.ConfigError{Code, Key}` (`app/httpserver/production_config.go`) — stable, redacted production config failures (`CONFIG_PATH_INVALID`, `CONFIG_FILE_MISSING`, `CONFIG_MONGO_INVALID`, ...). Create with `configError(code, key)`; render via `logConfigError`. Never include file contents or credential fragments in these.
- Plain `error` for service/db layers; sentinel messages like `"mongo client is not initialized"` are asserted by tests — keep the wording stable.
- Shell scripts use exit codes: config misuse exits `78` (packaged binary), smoke scripts exit non-zero with a one-line reason on stderr plus failure dumps (app log tail / pdf headers / docker logs — see `logging-guidelines.md`).

## Propagation

- Controller → service → db: each layer wraps with context; only the outermost handler decides the HTTP response.
- Startup degradation: `initDatabase` failure in `cmd/leanote/main.go` logs one notice and keeps serving so `/healthz` can answer 503 — the process stays up for diagnostics (this is the documented pattern, do not turn it into a hard exit).
- JSON controllers: transport 200 does not imply business success; the envelope carries the verdict (below).

## API Error Responses

### Note Save Envelope

`POST /note/updateNoteOrContent` always returns the existing `info.Re` JSON shape. Success is `{ "Ok": true }`; a successful new-note request also puts the created note in `Item`. Failure is `{ "Ok": false, "Msg": "..." }` with a non-empty, user-visible message. HTTP 200 does not imply business success.

The controller must inspect every `UpdateNote` and `UpdateNoteContent` result before setting `Ok`. A missing note/content record, permission failure, database insert/update failure, conflict, or a metadata-success/content-failure partial write must return `Ok:false`; the frontend may confirm its save revision and show success only after `Ok:true`.

## Common Mistakes

- Returning a zero-value note or a bare boolean as a successful save response.
- Ignoring one service result when the request updates both metadata and content, which masks a partial write.
- Treating transport status 200 as the business success signal.
- Table-driven tests must keep covering the 404/503 matrix, including the exact future-skew boundary and the smallest representable value beyond it.
