const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const ROOT = path.resolve(__dirname, "../..");
const PRODUCTION_ASSETS = [
  "public/tinymce/plugins/paste/classes/Clipboard.js",
  "public/tinymce/plugins/paste/plugin.js",
  "public/tinymce/plugins/paste/plugin.min.js",
  "public/tinymce/tinymce.full.js",
  "public/tinymce/tinymce.full.min.js",
];

function extractPastePlugin(source) {
  const moduleMarker = '"tinymce/pasteplugin/Utils"';
  const moduleIndex = source.indexOf(moduleMarker);
  assert.notEqual(moduleIndex, -1, "paste plugin module is present");

  const minifiedStart = source.lastIndexOf("!function(", moduleIndex);
  const start = minifiedStart !== -1
    ? minifiedStart
    : source.lastIndexOf("(function(", moduleIndex);
  const endMarker = minifiedStart !== -1 ? "}(this);" : "})(this);";
  const end = source.indexOf(endMarker, moduleIndex);
  assert.notEqual(start, -1, "paste plugin bundle start is present");
  assert.notEqual(end, -1, "paste plugin bundle end is present");

  return source.slice(start, end + endMarker.length);
}

function exposeClipboardModule(source) {
  const exposePattern = /(\w+)\(\["tinymce\/pasteplugin\/Utils","tinymce\/pasteplugin\/WordFilter"\]\)/;
  const match = source.match(exposePattern);
  assert.ok(match, "paste plugin exposure call is present");

  return source.replace(
    exposePattern,
    `${match[1]}(["tinymce/pasteplugin/Utils","tinymce/pasteplugin/WordFilter","tinymce/pasteplugin/Clipboard"])`,
  );
}

function loadClipboard(assetPath) {
  const source = fs.readFileSync(path.join(ROOT, assetPath), "utf8");
  const context = {
    LeaAce: { nowIsInAce: () => null },
    setTimeout: () => 0,
    tinymce: {
      Env: { gecko: false, ie: false, mac: false, webkit: false },
      PluginManager: { add: () => {} },
      dom: { DOMUtils: {} },
      html: {
        DomParser: function DomParser() {},
        Node: function Node() {},
        Schema: function Schema() {},
        Serializer: function Serializer() {},
      },
      util: {
        Delay: {},
        LocalStorage: {},
        Tools: {},
        VK: {
          metaKeyPressed: (event) => event.ctrlKey || event.metaKey,
        },
      },
    },
  };

  if (assetPath.endsWith("classes/Clipboard.js")) {
    context.define = (_id, _dependencies, definition) => {
      context.tinymce.pasteplugin = {
        Clipboard: definition(
          context.tinymce.Env,
          context.tinymce.util.VK,
          { innerText: () => "" },
        ),
      };
    };
    vm.runInNewContext(source, context, { filename: assetPath });
    return context.tinymce.pasteplugin.Clipboard;
  }

  const pluginSource = exposeClipboardModule(extractPastePlugin(source));
  vm.runInNewContext(pluginSource, context, { filename: assetPath });
  return context.tinymce.pasteplugin.Clipboard;
}

function createEditor() {
  const handlers = {};
  const pasteBin = {
    firstChild: null,
    focus: () => {},
    innerHTML: "%MCEPASTEBIN%",
  };
  const range = {};

  return {
    editor: {
      dom: {
        add: () => pasteBin,
        bind: () => {},
        getStyle: () => "ltr",
        getViewPort: () => ({ h: 100, y: 0 }),
        remove: () => {},
        setStyle: () => {},
        unbind: () => {},
      },
      getBody: () => ({ clientHeight: 100, scrollTop: 0 }),
      getDoc: () => ({}),
      getWin: () => ({}),
      on: (events, handler) => {
        for (const event of events.split(" ")) {
          handlers[event] = handler;
        }
      },
      selection: {
        getRng: () => range,
        select: () => {},
        setRng: () => {},
      },
      settings: { paste_data_images: true },
    },
    handlers,
  };
}

function pasteImage(Clipboard, clipboardEvent) {
  const { editor, handlers } = createEditor();
  new Clipboard(editor);

  handlers.keydown({
    ctrlKey: true,
    isDefaultPrevented: () => false,
    keyCode: 86,
    metaKey: false,
    shiftKey: false,
    stopImmediatePropagation: () => {},
  });

  let preventDefaultCalls = 0;
  handlers.paste({
    ...clipboardEvent,
    preventDefault: () => {
      preventDefaultCalls += 1;
    },
  });

  return preventDefaultCalls;
}

for (const assetPath of PRODUCTION_ASSETS) {
  test(`${assetPath} prevents TinyMCE from inserting a directly pasted image`, () => {
    const Clipboard = loadClipboard(assetPath);
    const calls = pasteImage(Clipboard, {
      clipboardData: { items: [{ type: "image/png" }] },
    });

    assert.equal(calls, 1);
  });

  test(`${assetPath} detects image items on a jQuery-wrapped paste event`, () => {
    const Clipboard = loadClipboard(assetPath);
    const calls = pasteImage(Clipboard, {
      originalEvent: {
        clipboardData: { items: [{ type: "image/png" }] },
      },
    });

    assert.equal(calls, 1);
  });
}
