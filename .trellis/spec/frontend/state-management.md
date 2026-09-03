# State Management

> How state is managed in this project.

---

## Overview

No state library. This is a legacy jQuery application: state lives in window-level namespace objects plus a first-party editor session facade. Server state is mirrored into those objects by AJAX calls that follow the shared wrapper contract.

## State Categories

- **Global app state** — window namespaces: `Note` (current note + list cache `Note.cache`), `Notebook`, `Tag`, `LEA` (runtime flags incl. `readOnly`). See `public/js/app/note.js`.
- **Editor session state** — `window.LeanoteEditorSession` (source `public/js/editor-state-source.js`): dirty flag, content revision counter, persisted content snapshot; the note page and e2e suites poll it (`isDirty()`, `snapshot()`). First-party plugins mark mutations through `LeanoteEditorSession.markMutation` or the plugin's `leanoteMarkMutation` option (see `leaui_image/plugin.js`).
- **Server state** — fetched via the shared AJAX wrappers (`ajaxPostJson`/`ajaxGetJson` in `public/js/common.js`): failures always surface (alert + failure callback), never silent; NOTLOGIN responses alert and route to login.
- **Dialog/iframe state** — TinyMCE dialogs exchange data with `window.parent.postMessage({mceAction, images|data})`; the opener reseeds `window.LEAUI_DATAS` on each open (opener reset is part of the contract — tests must pass expected data through their own scope, not reread the window variable).

## Patterns

- Mutation → `markMutation` → debounced autosave → `/note/updateNoteOrContent` envelope (`Ok:true` confirms the save revision; title-only saves omit `Content` and keep content clean).
- Read-only gate: `leaui_image` checks `LEA.readOnly`/`Note.readOnly`/`editor.mode.isReadOnly()` and blocks mutations incl. `dragstart`.
- Undo/redo: TinyMCE `undoManager` transitions must flow through `LeanoteEditorSession` dirtiness — `editor.undoManager` may be absent during init races; guard before use (`public/js/common.js` `setEditorContent`).

## What NOT to do

- Do not introduce a second state container or a parallel editor runtime — the session facade and namespace objects are the contract the e2e suites assert against.
- Do not read `window.LEAUI_DATAS` after triggering `openAlbum`-style dialogs expecting the seed to persist (the opener legitimately resets it).
