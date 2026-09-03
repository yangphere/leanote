# Logging Guidelines

> How logging is done in the Go backend.

---

## Overview

The backend uses the standard library `log` package (and the Revel-compatible logger shim `app/lea/revel_logger.go` for migrated presentation code). There is no structured-logging dependency; lines are plain text on stdout/stderr and are captured by CI job logs.

## Log Levels

Only the levels the shims expose:

- `INFO` — lifecycle milestones: startup (`leanote starting: addr=... runMode=... shutdownTimeout=...`), clean shutdown (`leanote stopped cleanly`), presentation asset loading (`Successfully loaded messages ...` in `revel_logger.go`).
- Default `log.Printf` — operational notes that are neither milestones nor failures (e.g. degraded-mode notices like `mongo readiness unavailable; healthz will return not_ready` in `cmd/leanote/main.go`).
- Errors returned to callers — Go convention: return the error; log once at the boundary that terminates handling. Do not log-and-return the same error at every layer.

## Structured Logging

There is no JSON logger in the app. Machine-readable summaries exist only in CI (`scripts/ci/write-summary.mjs`, schema `leanote.ci.failure-summary.v1`); application code does not emit them.

## What to Log

- Startup/shutdown lifecycle and the active address/run mode (see `cmd/leanote/main.go`).
- Degraded-but-serving conditions (db unavailable → healthz stays 503, one notice line).
- Packaging/e2e failure diagnostics are the scripts' job: `scripts/package-smoke.sh` dumps the app log tail + pdf headers on failure; `scripts/container-smoke.sh` dumps `docker logs` tail. Keep those dumps — they are the "original failure cause" record the delivery gates require.

## What NOT to Log

- Credentials, tokens, cookies, session material, `app.secret` values.
- Raw config values or Mongo URIs with credentials — production config failures are reported as redacted `ConfigError{Code, Key}` strings (`app/httpserver/production_config.go` `logConfigError`), never with file contents.
- Page bodies, user data, or full request dumps. The CI summary allowlist (`sanitizeSummary` in `tests/e2e/build/sanitized-summary-reporter.mjs`) is the reference for what is safe to record.
