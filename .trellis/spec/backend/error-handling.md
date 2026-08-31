# Error Handling

> How errors are handled in this project.

---

## Overview

<!--
Document your project's error handling conventions here.

Questions to answer:
- What error types do you define?
- How are errors propagated?
- How are errors logged?
- How are errors returned to clients?
-->

(To be filled by the team)

- HTTP test-only identity endpoints use an explicit status matrix: requests
  outside test mode or loopback return `404`; marker, database, token digest,
  or time-boundary validation failures return `503` without sensitive details.
- Boundary checks are inclusive at the documented limit. The e2e marker future
  skew accepts exactly `validationNow + 60s` and rejects any later timestamp.
- Lower-layer errors are returned to the caller with context; handlers must not
  convert database initialization or query failures into successful empty data.

- Table-driven tests cover the 404/503 matrix, including the exact future-skew
  boundary and the smallest representable value beyond it.

---

## Error Types

<!-- Custom error classes/types -->

(To be filled by the team)

---

<!-- Try-catch patterns, error propagation -->

---

## API Error Responses

<!-- Standard error response format -->

### Note Save Envelope

`POST /note/updateNoteOrContent` always returns the existing `info.Re` JSON
shape. Success is `{ "Ok": true }`; a successful new-note request also puts
the created note in `Item`. Failure is `{ "Ok": false, "Msg": "..." }` with a
non-empty, user-visible message. HTTP 200 does not imply business success.

The controller must inspect every `UpdateNote` and `UpdateNoteContent` result
before setting `Ok`. A missing note/content record, permission failure,
database insert/update failure, conflict, or a metadata-success/content-failure
partial write must return `Ok:false`; the frontend may confirm its save
revision and show success only after `Ok:true`.

---

## Common Mistakes

<!-- Error handling mistakes your team has made -->

- Returning a zero-value note or a bare boolean as a successful save response.
- Ignoring one service result when the request updates both metadata and
  content, which masks a partial write.
- Treating transport status 200 as the business success signal.
