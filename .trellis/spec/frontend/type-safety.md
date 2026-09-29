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

## Delivery provenance and recovery

### 1. Scope / Trigger

Apply these rules when changing quality summaries, protected delivery evidence,
release artifact validators, publishing workflows or recovery commands.

### 2. Signatures

- scripts/release-runner.mjs accepts source-check, preflight, create, recover
  and read-only reconcile. executeReleaseCommand(mode, env) returns a
  leanote.release-result.v1 record; the CLI fails on a failure record.
- validateReleaseArtifact({directory, candidateRoot, expected}) and
  validateBrowserArtifact(options, expected) are the shared validator seams.
- scripts/ci/delivery-evidence.mjs produces a separate two-file catalog/evidence
  artifact; it calls a protected executable, not a synthetic result importer.

### 3. Contracts

Quality job IDs come from scripts/ci/quality-contract.mjs. Each summary must
match the trusted current SHA/ref/workflow/run/attempt, and success requires
an explicitly observed CI_EXIT_CODE=0. Internally consistent stale summaries
and absent exit codes are failures. The workflow writes zero only after its
command completes successfully.

Recovery requires RELEASE_TAG, RECOVERY_COMMIT, SOURCE_RUN_ID, SOURCE_ATTEMPT,
SOURCE_INPUTS_ARTIFACT, SOURCE_BROWSER_ARTIFACT, SOURCE_DELIVERY_ARTIFACT and
RELEASE_APPROVED_IDENTITY=repository@tag:commit. The last value is operator
authorization from a protected environment, never generated from inputs.
Current execution and original source identities remain separate. Candidate
version/lock, epoch and source catalog come from the original clean checkout.

Normal and recovery workflows share release-refs/tags/<tag> with cancellation
disabled. Source verification uses the attempt-specific jobs endpoint plus
workflow ID/path/state and immutable artifact IDs. Optional job.run_attempt
must match when present; artifact contents always bind the original attempt.
Checksums hash the actual file bytes, even when syntax accepts CRLF text.

### 4. Validation & Error Matrix

| Input/state | Required result |
| --- | --- |
| Wrong run/attempt, revoked workflow, expired/missing artifact | Block before writes |
| Wrong original version/epoch, modified tracked candidate, hash mismatch | Block before writes |
| GHCR denied/rate-limited/malformed response or NAME_UNKNOWN | Unknown/blocked, never infer absence |
| MANIFEST_UNKNOWN with exact-package authenticated listing | Tag absence may be accepted |
| Recovery image absent or tag/digest conflicting | Block; no rebuild or push |
| Matching image and explicitly absent Release | Single create, then actual asset-byte read-back |
| Existing complete Release | Recovery read-only no-op; ordinary publish rejects |
| Existing partial Release or unknown create/read-back | No overwrite/upload repair or blind retry |
| Missing real scenario runner, missing cases, nonzero exit or unknown cleanup | Delivery gate blocks |

### 5. Good / Base / Bad Cases

Good: original v2.3.4 candidate is verified from candidate/ even when the
recovery executor has another version. Base: a second approved recovery reads
the matching complete Release without writes. Bad: changing environment
run/attempt to make old artifacts appear newly produced, or treating a
schema-valid protected result as proof that its runner has been reviewed.

### 6. Tests Required

Keep release-runner.test.js for cross-version/attempt composition, actual
asset hash read-back and zero-write failures; release-source.test.js for
structured remote absence and source API identity; release-recovery.test.js
for uncertain writes, no-op and duplicate guards; delivery-workflow.test.js
for the dependency graph and shared lock; ci-summary-integrity.test.js for
stale identity and unknown/nonzero exit. Contract fixtures are not real
Mongo/HTTP/SMTP/Linux/PDF/browser/GitHub/GHCR acceptance evidence.

### 7. Wrong vs Correct

Wrong: normalize checksum bytes before hashing, then discover tampering only
after uploading. Correct: validate the original bytes against the original
manifest before any write, then independently hash the remote downloads.

Wrong: add another claimed-passed receipt to compensate for a missing runner.
Correct: keep the release gate blocked until reviewed scenario commands,
fixtures, assertions and resource cleanup are implemented and executed.

## What NOT to do

- Do not introduce TypeScript or a schema dependency for page code; the manifest-driven build and the contract suites are the established mechanism.
- Do not loosen an `assertKeys`/enum/pattern in a validator to make a test pass — the F contract documents them as non-negotiable.
