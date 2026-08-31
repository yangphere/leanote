# TinyMCE 4.1.9 Runtime Inventory

Audit date: 2026-08-30

## 1. Core And Entry Points

The tracked runtime is TinyMCE 4.1.9, not 4.1.4:

- `public/tinymce/tinymce.js:31631-31639` and `tinymce.full.js:31631-31639` declare major `4`, minor `1.9`.
- Production note: `app/views/note/note.html:895` loads `/tinymce/tinymce.full.min.js`.
- Development note source: `app/views/note/note-dev.html:899` loads `/tinymce/tinymce.js`.
- Member single-page editor: `app/views/member/blog/add_single.html:47-73` loads readable core and owns an inline init.
- Member abstract editor: `app/views/member/blog/update_abstract.html:75-101` does the same.

`note-dev.html` is the source from which the build creates `note.html`; both paths must be changed through the existing template-generation contract.

## 2. Init And Official Plugin Disposition

Main note configuration is at `public/js/app/page.js:490-571`. It uses inline mode, custom `leanote` theme, `custom` skin, two toolbars and the following plugin names:

| Current name | Current entry | TinyMCE 8 disposition |
| --- | --- | --- |
| `autolink`, `link`, `lists`, `searchreplace`, `table` | note | Keep as supported plugins; `table` is present in the 8.8.2 npm artifact |
| `paste`, `hr` | note | Integrated into core; remove from plugin list |
| `textcolor` | note | Color buttons are core; remove plugin |
| `tabfocus` | note | Removed; browser/core owns Tab behavior |
| `advlist`, `charmap`, `visualblocks`, `visualchars`, `directionality` | member | Keep as supported plugins |
| `contextmenu` | member | Integrated into core |
| `fullpage` | member | Removed/replaced by premium capability; no menu/toolbar action exposes it, so remove and prove fragment round-trip |

Toolbar renames required in note and member profiles are `formatselect → blocks`, `fontselect → fontfamily`, and `fontsizeselect → fontsize`.

## 3. First-party Plugins

| Plugin | Evidence | Required boundary |
| --- | --- | --- |
| `leaui_image` | `plugin.js:7,72,182-213` uses PluginManager, v4 URL dialog, addButton/addMenuItem and drag events | Preserve Bootstrap 5 iframe, upload/select/update/drag; migrate UI Registry and URL dialog |
| `leaui_mindmap` | `plugin.js:7,22,37,62-78` uses local mindmap dialog and `data-mind-json` | Preserve local KityMinder and marker round-trip |
| `leanote_nav` | `plugin.js:5-57` scans headings and writes `#leanoteNavContent` | External navigation refresh must not dirty or alter stored HTML |
| `leanote_code` | `plugin.js:7,257-367` uses v4 buttons/listbox and key handlers | Preserve block/inline code, language/Ace and serialized attributes |
| `leaui_mind` | `plugin.js:29` loads `//leanote.com/public/libs/mind/edit.html`; not present in active plugin list | Delete external fallback; do not merge it into the local path |

All four active plugins currently call removed v4 UI APIs such as `editor.addButton`, `addMenuItem`, or old dialog config. A core-level compatibility shim would create a second API truth and is prohibited.

## 4. Save And Content State

- `public/js/app/note.js:192-194` explicitly rejects TinyMCE `isDirty()`.
- `note.js:234-305` serializes editor content whenever the note is writable/forced and compares bytes against cached `cacheNote.Content`.
- `public/js/common.js:382-407` programmatically calls `editor.setContent(content)` and clears undo on every note load.
- `note.js:438-458` sends the update and currently updates the cache before server success is known.
- `note.js:470-483` contains a second `updatePoolNote` ajax save path with no response/error handling; both direct and pooled saves must share the same `reIsOk` gate and success-only cache/revision confirmation.
- `app/controllers/NoteController.go:178-254` returns a raw note for new saves, then calls `NoteService.UpdateNote` and `UpdateNoteContent` for existing notes, discards both `(ok, msg, usn)` results, and always returns JSON `true`. A zero-value new note or existing-note business failure is therefore indistinguishable from success to the browser.
- `app/service/NoteService.go:382-407` exposes only `info.Note` from the controller-specific new-note helper; `AddNote` ignores the boolean result of `db.Insert`, and the content insert helper also returns a value without surfacing its insert status. A structured HTTP response therefore needs a small controller-facing result boundary if insert/content failures are to be reported instead of inferred from a note id.
- `app/info/Re.go` already defines the project response envelope (`Ok`, `Msg`, `Item`), and `public/js/common.js:1017-1019` already exposes `reIsOk`; the save endpoint can reuse both instead of inventing a second response convention.
- `leanote_nav/plugin.js` refreshes an editor-external DOM tree in response to setcontent/undo/paste/commands/click.

TinyMCE 8 can normalize HTML during `setContent`, so byte comparison alone would turn an untouched load into a content update. The migration needs two baselines (persisted bytes and post-load editor serialization), a programmatic-load boundary, per-note epoch/revision, and success-only confirmation. Every successful save must advance both baselines to the submitted bytes even when a later edit already exists; otherwise undo can compare against a stale pre-save baseline. Title/tag-only saves must omit `Content`; business, partial and HTTP failures must leave the submitted content retryable.

## 5. Asset, Style And Locale Surfaces

- `scripts/build/manifest.mjs:67-71` generates only seven TinyMCE locale files: `de-de`, `en-us`, `es-co`, `fr-fr`, `pt-pt`, `zh-cn`, `zh-hk`.
- `public/tinymce/langs/en.js`, `zh.js`, and `readme.md` are outside that manifest. They require reference/request proof before deletion or ownership.
- Each generated locale is sourced from `messages/<locale>/tinymce_editor.conf`; this remains the translation fact source.
- The note init passes the lower-case application locale directly. TinyMCE 8 requires an explicit RFC5646-compatible mapping and a matching `language_url`.
- `.mce-*` selectors occur in `public/css/editor/editor.less`, `public/css/private-share-note.less`, `public/css/blog/blog_left_fixed.less`, and the theme files `basic.less`, `mobile.less`, `writting.less`, `writting-overwrite.less`, `includes/editor.less`, `includes/icon.less`, and `includes/tinymce.less`. Some are editor-content classes, while others bind to v4 toolbar/dialog chrome. Every matched file needs classification; a global prefix replacement is invalid.
- The current manifest does not own TinyMCE core, themes, skins, official plugins, first-party plugin minified assets, or their static dialog applications.

## 6. Existing Test And Delivery Infrastructure

- `tests/js/paste-plugin.test.js` loads v4 `paste/classes/Clipboard.js`, plugin bundles and `tinymce.full.*` internals. It cannot be carried forward as a TinyMCE 8 core test.
- E2E business tests use `.spec.mjs`; the task's former `.spec.js` filename was wrong.
- `playwright.config.mjs` has one shared `business` Chromium project and a release `browser-smoke` project. The latter currently discovers only `business-flows.spec.mjs`, so editor smoke discovery must be made explicit.
- `tests/e2e/e2e-environment.mjs` requires base URL, account, run token, identity endpoint confirmation and a fresh confirmation before writes.
- `tests/e2e/business/business-flows.spec.mjs:266-286` already posts form data through the real `/note/updateNoteOrContent` route and is the success-contract anchor. `tests/e2e/business/ajax-failure.spec.mjs` owns browser-visible HTTP failure regressions; new-note zero-value, business and partial-failure response coverage belongs in the real Revel/MongoDB harness plus the editor state tests.
- Parent task and ADR own the browser matrix: Chromium blocks PR/push; real Chrome, Edge, Firefox and Safari current/previous major are release evidence, with real Safari required.

## 7. Planning Consequences

This is a cross-cutting runtime migration, not a core-file replacement. Implementation must atomically cover asset ownership, three entry points, official plugin disposition, four first-party plugins, save-state semantics, the endpoint's minimal structured response boundary, paste/upload behavior, seven locales, application UI CSS, generated outputs and real-service verification.
