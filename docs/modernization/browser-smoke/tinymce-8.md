# TinyMCE 8 Browser Smoke Evidence

| Field | Value |
| --- | --- |
| Date | 2026-08-31 |
| Commit under review | `8c8c832532af63f16d177013515e915c4dea00c7` |
| Product/runtime | Leanote self-hosted TinyMCE `8.8.2` (GPL) |
| OS | Windows 11 Professional Workstation |
| Node | `v24.20.0` |
| Playwright | `1.62.1` |
| Identity/error gate | Defined by `tests/e2e/e2e-environment.mjs` and the editor/business suites; not executed in this environment |
| Overall result | **BLOCKED: release evidence incomplete** |

## Browser Matrix

| Browser | Current major | Previous major | Covered entry points | Result |
| --- | --- | --- | --- | --- |
| Chrome (real binary) | Not run | Not run | `/note`, `/member/blog/addOrUpdateSingle`, `/member/blog/updateBlogAbstract`, `leaui_image` iframe | Blocked |
| Edge (real binary) | Not run | Not run | Same editor and iframe flows | Blocked |
| Firefox (real binary) | Not run | Not run | Same editor and iframe flows | Blocked |
| Safari (real Safari) | Not run | Not run | Same editor and iframe flows | Blocked |

## Blocking Evidence

- The required test-mode service identity variables were not present: `LEANOTE_BASE_URL`, `LEANOTE_E2E_EMAIL`, `LEANOTE_E2E_PASSWORD`, and `LEANOTE_E2E_RUN_TOKEN`.
- No authenticated Revel/MongoDB run was available, so `npm run test:e2e` and `npm run test:e2e:smoke` could not provide execution evidence.
- Real Chrome, Edge, Firefox, and Safari current/previous-major product versions were not recorded. Chromium or WebKit is not treated as a substitute for those release rows.

The missing environment and browser evidence are release blockers under R-TM8/AC-TM8; this document records the gap and does not claim a passing smoke run.
