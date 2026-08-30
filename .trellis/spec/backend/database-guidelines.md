# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

<!--
Document your project's database conventions here.

Questions to answer:
- What ORM/query library do you use?
- How are migrations managed?
- What are the naming conventions for tables/columns?
- How do you handle transactions?
-->

(To be filled by the team)

- Shared MongoDB helpers must fail closed when the package client has not been
  initialized. Return an explicit error before dereferencing the client; do not
  silently return an empty result or create an implicit connection.
- Read-only test-support queries (for example, `e2e_runs` identity markers)
  use the current application database session and must propagate connection
  and query errors to the caller.

- Unit tests cover the uninitialized-client path and assert a non-nil error.
- Identity/database integration tests distinguish a missing or invalid marker
  from a database error; both are fail-closed and must not expose credentials.

---

## Migrations

<!-- How to create and run migrations -->

(To be filled by the team)

---

## Naming Conventions

<!-- Table names, column names, index names -->

(To be filled by the team)

---

## Common Mistakes

<!-- Database-related mistakes your team has made -->

(To be filled by the team)
