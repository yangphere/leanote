# State Management

> How state is managed in this project.

---

## Overview

<!--
Document your project's state management conventions here.

Questions to answer:
- What state management solution do you use?
- How is local vs global state decided?
- How do you handle server state?
- What are the patterns for derived state?
-->

(To be filled by the team)

---

## State Categories

<!-- Local state, global state, server state, URL state -->

(To be filled by the team)

---

## When to Use Global State

<!-- Criteria for promoting state to global -->

(To be filled by the team)

---

## Server State

<!-- How server data is cached and synchronized -->

### Editor Session Contract

The note editor owns one `window.LeanoteEditorSession` state adapter per active
note. It tracks `noteId`, `loadEpoch`, `persistedContent`, `editorBaseline`,
`currentContent`, `contentRevision`, `confirmedRevision`, and `loading`.

- `beginLoad({ noteId, persistedContent })` increments `loadEpoch` and enters
  `loading`; `completeLoad(epoch, editorContent)` accepts only the current
  epoch and establishes the editor baseline without incrementing the revision.
- User content actions call `markMutation(serializedContent[, epoch])`. Stale
  epochs, read-only mode, loading, and unchanged serialization are rejected.
  Programmatic `setContent`, Ace hydration/cleanup, and external navigation
  DOM updates must not call this mutation boundary.
- `beginSave()` captures note id, epoch, revision, and submitted content.
  `confirmSave(capture, currentSerialization)` advances both persisted and
  editor baselines only for the current epoch and a non-older revision, then
  recomputes dirty state from the current serialization. A later edit remains
  dirty until separately confirmed.

The adapter is the single owner of editor dirty state; callers must not infer
dirty state from TinyMCE's internal `isDirty()` flag or from DOM-equivalent
content when deciding whether to send stored HTML.

---

## Common Mistakes

<!-- State management mistakes your team has made -->

- Treating a programmatic note load or delayed callback as a user mutation.
- Confirming a save before the backend returns `info.Re.Ok === true`.
- Replacing the baseline with the current serialization when a save response
  is stale or when serialization failed.
