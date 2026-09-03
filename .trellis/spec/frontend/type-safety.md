# Type Safety

> There is no TypeScript in this project. This file records the runtime-validation and contract-test patterns that serve the same purpose.

---

## Overview

Plain JavaScript (ES5-flavored legacy plus Node 24 ESM tooling). Safety comes from three layers: runtime validation in shared helpers, JSON-schema-style contract validators in `scripts/`, and the Node contract suite (`tests/js/*.test.js`).

## Runtime Validation Patterns

- Image data normalization (`leaui_image/plugin.js` `normalizeImageData`): reject `javascript:`/`vbscript:`/`file:` schemes, non-image `data:` URIs, empty src — return null and surface an error toast instead of inserting.
- Attribute values escaped before HTML assembly (`escapeAttribute`); numeric width/height gated by `/^\d+(?:\.\d+)?$/`.
- The shared AJAX wrappers normalize transport/business failure so callers cannot mistake HTTP 200 for success (see `hook-guidelines.md`).

## Contract Validators (Node side)

- `scripts/browser-release-evidence.mjs` + `scripts/validate-browser-artifact.mjs` enforce exact key sets (`assertKeys` ≙ `additionalProperties:false`), enums, patterns (`^[a-z0-9][a-z0-9._/-]{0,79}$` identifiers), counts, and RFC 8785 JCS digests for release artifacts. Follow this validator style for any new machine-readable artifact.
- `scripts/ci/write-summary.mjs` validates env-provided provenance (fail closed on missing/malformed run identity).

## Contract Tests as the Type Check

- `tests/js/build-pipeline.test.js` (build closure, staging/rollback, mode contract), `release-contract.test.js` (artifact/summary/browser evidence schemas), `jcs-contract.test.js` (canonicalization domain), `ajax-wrapper-contract.test.js`, `note-save-contract.test.js`, `editor-state-contract.test.js`, `tinymce-*-contract.test.js`.
- When you add or change a data shape that crosses a boundary (page ↔ server, plugin ↔ dialog iframe, script ↔ artifact), add or extend the matching contract test — that is this repo's type safety.

## What NOT to do

- Do not introduce TypeScript or a schema dependency for page code; the manifest-driven build and the contract suites are the established mechanism.
- Do not loosen an `assertKeys`/enum/pattern in a validator to make a test pass — the F contract documents them as non-negotiable.
