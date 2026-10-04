# Real GHCR publication

## Authorization and target

- On 2026-10-04 the user requested: “帮我真实在GHCR发布”. This authorizes
  the planned real image publication, including its scoped commit, remote
  candidate integration and `v1.0.0` tag push. No production deployment is
  included.
- Target: `ghcr.io/yangphere/leanote:v1.0.0`, `linux/amd64`.
- `gh` is authenticated as repository administrator `yangphere`.
- Package/lock version is `1.0.0`; remote `v1.0.0` is absent.
- Remote master starts at `5bc6bd439b55c88f2aeb42477cc99dd38fc860be`.
  It has one unique merge commit; the current dev candidate has 227 unique
  commits. Integrate the candidate while retaining master history.
- The user's existing `CONTEXT.md` edit and Compose task are excluded from the
  publication commit.

## Execution state

Preparing a reviewed candidate. Real Actions execution, GHCR push/digest
read-back and anonymous pull are not yet verified. Any public-visibility change
must be recorded separately; initial GHCR package visibility defaults to private.

Recent repository CI runs failed. Read their real job diagnostics before the
publication tag; do not bypass the full reusable quality gate.
