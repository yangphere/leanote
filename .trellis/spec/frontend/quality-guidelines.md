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

### Standalone Bootstrap-derived views

Views that reuse a modal fragment in a full-page response must keep page-only
close behavior at the standalone wrapper boundary. A `data-bs-dismiss="modal"`
attribute only closes an element when Bootstrap can find a `.modal` instance;
it does not navigate away from a page containing only `.modal-dialog` and
`.modal-content`.

Keep the shared fragment's `data-bs-dismiss` contract unchanged so AJAX callers
continue to close the real Bootstrap modal. Do not add standalone navigation to
the fragment itself.

### HTTP route path literals

Frontend request paths must match the corresponding `conf/routes` path exactly.
The first-party HTTP matcher does not normalize a trailing slash, so a request
such as `/note/listNotes/` does not match a route registered as
`/note/listNotes`. When a route literal changes, add a contract test that reads
the registered path and checks every relevant caller.

### Remote Bootstrap modal fragments and generated assets

Remote share actions are HTML-fragment requests, not JSON API calls. The caller
must set the active resource kind and ID before `showDialogRemote()` so the
fragment's permission and delete handlers address the same note or notebook.
The response must contain a complete `.modal-dialog`/`.modal-content` subtree;
Bootstrap owns opening and closing the real `#leanoteDialogRemote` container.

### 1. Scope / Trigger

Use this contract when changing note/notebook share menus, remote modal markup,
or any source consumed by the manifest-driven frontend build.

### 2. Signatures

```js
Share.dialogIsNote = true|false;
Share.dialogNoteOrNotebookId = resourceId;
showDialogRemote('/share/listNoteShareUserInfo', {noteId: resourceId});
```

### 3. Contracts

- Note and notebook callers set `Share.dialogIsNote` and
  `Share.dialogNoteOrNotebookId` immediately before the remote request.
- The remote response contains `.modal-dialog`, `.modal-content`,
  `#shareNotebookTable`, and `data-bs-dismiss="modal"` on the close button.
- Source files are authoritative; generated `public/js/app.min.js` is updated
  only by `npm run build`.

### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Valid note/notebook response | modal opens and close button hides `#leanoteDialogRemote` |
| Empty or non-HTML response | visible error modal; no uncaught Bootstrap exception |
| Resource kind changed | permission/delete requests use the matching note/notebook endpoint |
| Source changed | build refreshes generated output and i18n line fixture without changing message keys |

### 5. Good / Base / Bad Cases

- Good: set the shared state, fetch HTML, inject it into the remote container,
  and call the existing Bootstrap wrapper.
- Base: open and close both note and notebook share dialogs in a logged-in
  browser and inspect console errors after a fresh page load.
- Bad: hand-edit `public/js/app.min.js`, return JSON to an HTML modal caller, or
  leave `dialogIsNote` pointing at the previous resource.

### 6. Tests Required

- Add source-level tests for both state assignments and run `npm run build` plus
  `npm test` and `npm run test:e2e:build -- --list`.
- Keep the real browser modal/open-close and console evidence separate from
  Node contract tests; do not claim browser acceptance from build output.

### 7. Wrong vs Correct

```js
// Wrong: the modal has no resource context and receives an API-shaped body.
showDialogRemote('/share/listNoteShareUserInfo', {noteId: id});

// Correct: state and complete HTML fragment agree before Bootstrap opens it.
Share.dialogIsNote = true;
Share.dialogNoteOrNotebookId = id;
showDialogRemote('/share/listNoteShareUserInfo', {noteId: id});
```

### Clipboard image paste ownership

Ordinary-note clipboard images have one upload owner: `editor_drop_paste`.
TinyMCE must not create a second `blob:` image for the same paste event.

#### 1. Scope / Trigger

Use this contract when changing TinyMCE paste configuration, the editor image
uploader, or the ordinary-note editor boundary.

#### 2. Signatures

```js
config.paste_data_images = false;
clipboardImageFiles(event);
$('#editorContent').fileupload('add', {files: files});
```

#### 3. Contracts

- The ordinary-note TinyMCE profile keeps `paste_data_images` disabled.
- The editor boundary captures image clipboard items, prevents the browser and
  TinyMCE image handlers from continuing, and passes the files to the existing
  `/file/pasteImage` uploader.
- One clipboard image produces one upload request and one durable image node;
  no `blob:` image is persisted in note content.

#### 4. Validation & Error Matrix

| Condition | Required result |
|---|---|
| Image clipboard paste in an editable ordinary note | one server image and one editor node |
| Same paste event reaches TinyMCE | capture handler prevents default and stops propagation |
| Non-image clipboard content | existing text paste behavior remains unchanged |
| Upload failure or note switch during upload | placeholder is removed; stale upload cannot mutate the new note |

#### 5. Good / Base / Bad Cases

- Good: disable TinyMCE data-image insertion and call the existing uploader
  from one capture handler.
- Base: paste once, save, refresh, and confirm the note and public blog each
  contain one image URL.
- Bad: leave `paste_data_images` enabled while also submitting the same file to
  `/file/pasteImage`, or add a second independent paste uploader.

#### 6. Tests Required

- Assert the ordinary TinyMCE profile disables `paste_data_images`.
- Assert the paste boundary captures image clipboard items, calls
  `preventDefault()` and `stopImmediatePropagation()`, and invokes the single
  file uploader.
- Keep browser/Mongo evidence separate from Node contract tests; browser
  evidence must count image nodes and newly created server files.

#### 7. Wrong vs Correct

```js
// Wrong: TinyMCE inserts a blob while Leanote also uploads the file.
config.paste_data_images = true;

// Correct: Leanote owns the event and inserts the durable server image once.
config.paste_data_images = false;
event.preventDefault();
event.stopImmediatePropagation();
$('#editorContent').fileupload('add', {files: files});
```

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
