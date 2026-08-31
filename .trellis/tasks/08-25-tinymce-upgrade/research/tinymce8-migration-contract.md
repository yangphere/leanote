# TinyMCE 8.8.2 Migration Contract

Research date: 2026-08-30

## Official Sources

- TinyMCE 4 → 8 migration: <https://www.tiny.cloud/docs/tinymce/latest/migration-from-4x/>
- TinyMCE 7 → 8 migration/security changes: <https://www.tiny.cloud/docs/tinymce/latest/migration-from-7x/>
- Self-hosted license configuration: <https://www.tiny.cloud/docs/tinymce/latest/license-key/>
- UI Registry toolbar buttons: <https://www.tiny.cloud/docs/tinymce/latest/custom-toolbarbuttons/>
- Dialog components and URL-dialog guidance: <https://www.tiny.cloud/docs/tinymce/latest/dialog-components/>
- UI localization: <https://www.tiny.cloud/docs/tinymce/latest/ui-localization/>
- npm registry metadata for `tinymce@8.8.2`: <https://registry.npmjs.org/tinymce/8.8.2>

## Confirmed Upstream Constraints

1. TinyMCE 4 custom themes/skins are incompatible with 8. Silver/Oxide replaces the old UI architecture, and custom UI CSS must be rebuilt against the v8 structure.
2. `paste`, `hr`, and `contextmenu` are core behavior; `textcolor` is no longer a plugin; `tabfocus` was removed. `fullpage` is no longer a free core plugin. The actual `tinymce@8.8.2` npm artifact still ships `plugins/table`, so Leanote must keep `table` as a declared plugin.
3. Toolbar identifiers changed: `formatselect` to `blocks`, `fontselect` to `fontfamily`, and `fontsizeselect` to `fontsize`.
4. Custom UI must register through `editor.ui.registry.*`. Actions use `onAction`/lifecycle callbacks, and old plugin UI calls cannot be preserved by copying core internals.
5. TinyMCE 8 dialogs require the v8 schema. Embedded HTML uses an iframe component; a dialog that loads a local page by URL uses the URL-dialog API. The iframe is sandboxed by default.
6. Self-hosted open-source deployments explicitly set `license_key: 'gpl'`. A missing key produces a license error; premium plugins are not available under GPL.
7. TinyMCE 8 language codes use RFC5646 hyphen form. When `language_url` is used, `language` is also required, the pack must be reachable, and the loaded pack code must match the configured language.
8. TinyMCE 7 enabled `sandbox_iframes` and `convert_unsafe_embeds` by default. TinyMCE 8 upgraded DOMPurify and enables stricter `SAFE_FOR_XML` handling; previously accepted comments/attribute values may be removed or altered.
9. `editor.selection.setContent` is deprecated in favor of `editor.insertContent`. Content and dialog migration must use supported public APIs.

## Project Decisions Derived From Existing Contracts

| Upstream change | Leanote contract |
| --- | --- |
| v4 theme/skin incompatible | Use Silver/Oxide plus minimal application layout CSS; remove `leanote/custom`, no v4 DOM facade |
| Plugins integrated/removed | Use the explicit disposition in PRD R-TM2; do not vendor obsolete plugin copies |
| `fullpage` premium | Remove it from member fragment editors; no commercial dependency; prove saved fragment behavior |
| UI Registry/dialog changes | Migrate each first-party plugin to public v8 API; no compatibility shim |
| Stricter content security | Keep v8 defaults; protect untouched DB bytes via no-save state, and fixture supported vs executable content separately |
| RFC5646 languages | Central app-locale mapping with explicit `language` + stable root-relative `language_url` |
| GPL key required | Set `license_key: 'gpl'` in the shared base profile and fail on warnings |
| Core serialization changes | Track persisted bytes separately from post-load editor baseline and real mutation revisions |

## Project Save-response Precondition

This boundary is derived from current Leanote code, not from a TinyMCE upstream change. `NoteController.UpdateNoteOrContent` currently discards the success/message values returned by `UpdateNote` and `UpdateNoteContent` and always reports JSON `true`; the browser therefore cannot implement a success-only revision state machine. The migration must reuse the existing `info.Re`/`reIsOk` convention, report any requested update failure as `Ok:false`, and confirm a captured content revision only after `Ok:true`. A metadata write followed by a content failure remains non-transactional, but it is reported as failure and kept retryable rather than mislabeled as overall success.

This is a narrow prerequisite of the editor save contract. It does not authorize a new schema, transaction layer, USN algorithm, authentication change or broad API redesign.

## Compatibility Boundaries

- ADR-0003 explicitly protects DOM semantics for text, links, images, code blocks and first-party plugin markers; the task adds tables and heading/navigation semantics because they are enabled current behavior.
- Script/object/embed/unsafe iframe behavior is not a supported editor feature in the current plugin/toolbar profiles. TinyMCE 8 security defaults remain authoritative after a real edit, while untouched records remain byte-identical because no content save occurs.
- The parent task fixes the browser matrix and test infrastructure. This task does not redefine browser support or create another Playwright configuration.
- The archived build-chain task intentionally deferred TinyMCE core/plugin/language repackaging to E-TM. Updating manifest ownership here is required and does not reopen the build-chain architecture.

## Decision Status

None. Product scope, license mode, locale set, supported content semantics, browser matrix, runtime order and no-bulk-migration boundary are all fixed by the parent PRD and ADR. Implementation-time discoveries that contradict one of these contracts must return to planning instead of selecting a silent fallback.
