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

## Workspace mutation result boundary

### 1. Scope / Trigger

This contract applies to the existing `/note/deleteNote` and `/note/moveNote`
AJAX actions while their response envelope remains unchanged. It prevents a
truthy `info.Re` failure object from being treated as a committed mutation.

### 2. Signatures

```js
Note.submitMutation({path, kind: 'boolean', noteIds, payload, success, failure})
// delete: { noteIds: string[], isShared: boolean }
// move:   { noteIds: string[], notebookId: string }
```

Request identity, locks and explicit retry follow **Protected Web mutation
intents** below. Do not call `ajaxPost` directly for these protected actions.

### 3. Contracts

- The manager invokes `success(ret)` for delete/move only when `ret === true`.
- `false` and objects such as `{Ok: false, Msg: "storage"}` are explicit
  failures; transport/parse failure is an unknown result, not a rejection.
- DOM removal, cache/count changes, current-note clearing, and batch reset occur
  only after strict success. A hidden delete selection is shown again on either
  failure path. Error display uses the server `Msg` or `getMsg("Error")`.

### 4. Validation & Error Matrix

| Result | Mutation state | UI/cache action |
| --- | --- | --- |
| `true` | confirmed | apply list, selection, cache, count and batch changes |
| `false` | rejected | retain state and note lock; show error, require reconciliation |
| `{Ok:false, Msg}` | rejected | retain state and note lock; show `Msg`, require reconciliation |
| transport/parse failure | unknown | retain state and lock; user explicitly retries the frozen body/ID |

### 5. Good/Base/Bad Cases

- Good: hide a delete selection, restore it when the server returns `false`,
  and leave `Note.cache` and `Note.batch` unchanged.
- Base: apply a move only after the raw boolean `true` response.
- Bad: use `if (ret)` and clear the current note before the request completes;
  a failure object then permanently removes visible state.

### 6. Tests Required

`tests/js/note-mutation-failure-contract.test.js` must exercise strict success,
`false`, `Ok:false`, transport failure, localized fallback text, and the
generated `public/js/app.min.js` handler for both actions.

### 7. Wrong vs Correct

```js
// Wrong: any object is truthy and commits a rejected mutation.
if (ret) removeSelectedNotes();

// Correct: preserve state until the existing response contract confirms it.
if (ret === true) removeSelectedNotes();
else showMsg(ret && ret.Msg ? ret.Msg : getMsg("Error"), 3000);
```

## Protected Web mutation intents

### 1. Scope / Trigger

Existing-note saves and private/shared copy, delete and move use the single
page-memory request manager in `public/js/mutation-intents.js`. It owns request
identity and outcome certainty, not editor dirty/revision state.

### 2. Signatures

- `LeanoteMutationIntents.create({crypto, serialize, send, changed})`.
- `start({path, kind, noteIds, payload, success, failure})`, `get(noteId)`,
  `retryUnknown()`, `summary()`, `hasUnresolved()`.
- `Note.submitSave(submitted, context, capture, callback)` serves immediate and
  queued saves. A save payload adds `OperationId` and `ExpectedUsn`.
- `Note.reloadMovedNote(noteId)` reads `/note/getNoteAndContent` with `{noteId}`
  after a confirmed move. The legacy response is flat (`NotebookId`, `Usn`,
  `Title`, `Tags`, `Content`, etc.); duplicate embedded fields such as `NoteId`
  are absent. Bind the response to the request, not an invented response ID.
- Protected existing-save success is the legacy envelope plus integer `Usn`.
- `Note.pendingNoteView(note)` projects confirmed cache, frozen save and latest
  queued edits for display only. `LeanoteEditorSession.restoreDraft(content,
  loadEpoch)` restores captured text without confirming or changing the baseline.

### 3. Contracts

- Generate a fresh 128-bit cryptographic random ID for each new intent. Freeze
  serialized body, action, ordered note IDs, destination and owner before
  sending. Explicit unknown-result retry uses the identical body and ID.
- Lock affected notes while a request is pending, unknown or rejected. Older
  attempts and repeated callbacks cannot apply a result twice. A definite
  rejection requires preserving edits and reloading/reconciling before a new
  attempt; it is not automatically retried under a new ID.
- `ExpectedUsn` comes only from confirmed note cache. The next save consumes
  the exact committed `Usn` returned by the preceding request. A boolean move
  response invalidates the cached revision; another protected save requires a
  fresh authoritative note load, never a locally incremented revision. Start
  that load on move success, keeping the current editor and uncaptured edits
  intact. `revisionReloads` tracks this read only; it is not an editor baseline.
  Block subsequent mutations until the read verifies the destination, valid
  non-regressing USN, non-deleted/non-trash state and unchanged cached editable
  fields. Never adopt a newer USN to silently overwrite concurrent server edits.
- Queue later edits against the frozen preceding request, including a user
  reverting to the pre-request title/content. The existing editor session
  alone owns dirty, load epoch and revision confirmation. Cache mirrors only
  confirmed state and never stores `OperationId` or `ExpectedUsn`.
- Reopening a pending note must restore its captured draft after loading the
  confirmed baseline. Snapshot the load input before asynchronous editor setup;
  a save callback can mutate cache during that setup. Restore from the newest
  cache/intent/queue projection, never write stale loaded text back.
- An unsent queued save remains in `savePool`. Confirmed copy/shared-copy clears
  the source queue's `reconcile` flag and drains it against the unchanged source
  USN. Confirmed delete removes queues for every deleted ID, including batches.
  Confirmed move keeps queued edits blocked until the authoritative read passes,
  then rebinds their `NotebookId`, clears `reconcile` and drains the queue. Read
  failure retains edits and invalid revision; a later save/pool attempt retries
  the read, never the already confirmed move. Reverting captured edits to the
  confirmed baseline removes the obsolete queue. Unavailable serialization cannot confirm dirty
  state or display save success, even if the server commit is confirmed.
- New-note create keeps its existing `NoteId`/`Item` contract and is outside
  this generation/retry guarantee. No request body or authentication material
  is persisted. The page warns on leave; unload never initiates a save. After
  reload the user checks server state and no old request is automatically sent.

### 4. Validation & Error Matrix

| Condition | Outcome |
| --- | --- |
| Missing secure randomness | unsent `RANDOMNESS_UNAVAILABLE`; secure-randomness message |
| Invalid cached USN or failed/mismatched move reload | preserve edits; revision-specific message, no write |
| Confirmed copy/shared-copy | release source queue; retain source notebook and USN |
| Confirmed delete | discard deleted IDs' queues; leave unrelated edits intact |
| Confirmed move + verified authoritative read | rebind queue destination and resume with returned USN |
| Pending/unknown/rejected operation on a note | block a new intent |
| Valid save `Ok:true` and safe integer committed USN | confirm exact revision |
| Boolean action returns raw `true` | apply success once |
| Copy `Ok:true` with complete returned note list | apply returned notes once |
| `false` or `Ok:false` | rejected; retain visible state and require reconciliation |
| Network/parse failure or malformed success | unknown; retain frozen retry |
| Different active note/load epoch | update confirmed cache only |

### 5. Good/Base/Bad Cases

- Good: retry a lost save response with its original body, then send queued
  edits using the returned committed USN and a new ID.
- Base: metadata-only edit sends no `Content`; force without edits sends nothing.
- Bad: reserialize current text for an old ID, take the current selection in a
  late batch callback, or clear another note after delayed delete.

### 6. Tests Required

`mutation-intents-contract.test.js`, `note-save-contract.test.js` and
`note-mutation-failure-contract.test.js` execute actual handlers/state owners,
including generated-bundle delete/move behavior. Cover immutable retry,
ordering, generation, conflict, queued reverts, old epochs, changed selection,
unload, reopened drafts and commits arriving during asynchronous loads. Exercise
move in the all-notes view through the real handler and read callback, without
manually repairing cache USN. Cover copy/shared-copy and batch-delete queue exits,
failed/malformed/stale move reads, concurrent server edits, and editor switching. Node
tests and mapper serialization tests do not replace real
HTTP/Mongo receipt and browser evidence.

### 7. Wrong vs Correct

Wrong: create a new ID after timeout and submit the editor's current text.
Correct: retry the frozen request, confirm its committed USN, then submit the
later edit as a separate intent.

## What NOT to do

- Do not introduce a second state container or a parallel editor runtime — the session facade and namespace objects are the contract the e2e suites assert against.
- Do not read `window.LEAUI_DATAS` after triggering `openAlbum`-style dialogs expecting the seed to persist (the opener legitimately resets it).
