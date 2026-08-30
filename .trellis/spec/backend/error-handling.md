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

(To be filled by the team)

---

## Common Mistakes

<!-- Error handling mistakes your team has made -->

(To be filled by the team)
