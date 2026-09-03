# Hook Guidelines

> React-style hooks do not exist in this project. This file records the equivalent reusable-logic conventions (jQuery event/lifecycle patterns) so agents don't invent hooks.

---

## Overview

Reusable stateful logic lives in jQuery event bindings, plugin lifecycle callbacks, and the editor session facade. There is no hook runtime; "sharing stateful logic" means shared window namespaces + event delegation.

## Data Fetching

- All AJAX goes through the shared wrappers in `public/js/common.js` (`ajaxPostJson`, `ajaxGetJson`): every failure surfaces to the user (alert) and invokes the failure callback; NOTLOGIN alerts and routes to login; button/link busy-state is restored after failure or thrown callbacks (asserted by `tests/e2e/business/ajax-failure.spec.mjs` and `tests/js/ajax-wrapper-contract.test.js`).
- Never call `$.ajax` directly from page modules — the wrappers are the contract that keeps failures visible.

## Event / Lifecycle Patterns

- **Delegated bindings** for dynamic lists: `$("#preview").on("click", 'li', handler)` (see `leaui_image/public/js/main.js`) — never rebind after render.
- **Plugin lifecycle**: TinyMCE `onSetup(config)` receives the control api, subscribes via `editor.on('NodeChange ModeChange', refresh)` and returns the unsubscribe function; teardown is mandatory (asserted in the leaui contract suite through the controlled editor shell).
- **Editor content pipeline**: `setEditorContent` in `public/js/common.js` runs `editor.setContent` → optional `LeanoteEditorSession.setContentProgrammatically(...)` → `callback()` → guarded `editor.undoManager.clear()` (the undoManager may not exist during the init race — the guard is load-bearing, B-E3).
- **Autosave wiring**: content mutations call `markMutation` (plugin option `leanote_markMutation` or `LeanoteEditorSession.markMutation`); the session drives the debounced save and revision counters.

## Naming Conventions

- Window namespaces are PascalCase objects (`Note`, `Notebook`, `LEA`); their methods camelCase verbs (`changeNote`, `renderNote`).
- jQuery objects locals prefixed `$` (`$li`, `$target`) in legacy files that already use it; do not mass-rename untouched files.
