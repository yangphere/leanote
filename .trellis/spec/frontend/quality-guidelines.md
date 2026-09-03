# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

Frontend build and regression code must fail closed: source discovery, manifest
validation, generated output publication, and browser smoke reporting must not
silently accept malformed or ambiguous input.

No linter is configured — the gates are `npm ci && npm run build && npm test` on Node 24 (CI `node-build` job, zero-drift `git diff --exit-code`) plus the Chromium e2e suite. Legacy page code uses four-space indent; Node tests two-space; do not reformat vendored/minified files (AGENTS.md).

---

## Forbidden Patterns

- Do not treat text inside JavaScript/HTML strings, comments, or regular
  expression literals as executable i18n calls.
- Do not parse a `getMsg` call by searching for a nearby closing parenthesis;
  scan the complete call while respecting strings, comments, regex literals,
  and nested delimiters.
- Do not add fallback output, global dependency lookup, or implicit browser or
  service startup to hide build or smoke-test failures.

---

## Required Patterns

- I18n diagnostics must include the source path, one-based line, and one-based
  column (`path:line:column`) for dynamic or missing keys.
- Build tests must use disposable roots for publication and rollback tests;
  tests must never rename tracked production files in the checkout.
- CI browser artifacts may contain only allowlisted sanitized summary fields;
  headers, cookies, tokens, page content, traces, screenshots, videos, and raw
  logs are prohibited.

---

## Testing Requirements

- Every parser or publication bug requires a regression test for the malformed
  input before and after the fix.
- Run `npm test`, `npm run build`, `npm run test:e2e:build -- --list`, and
  `git diff --check` before committing build-chain changes. Real service E2E
  requires the explicit Mongo/Revel harness and credential environment.

---

## Code Review Checklist

- Confirm manifest paths are the single source of truth and every declared
  output is tracked.
- Confirm staging, backup, message inputs, and published outputs reject
  symlink or junction escapes.
- Confirm rollback preserves recovery material when restoration or cleanup
  fails, and sanitized summaries contain no sensitive payloads.
