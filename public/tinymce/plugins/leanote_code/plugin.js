(function () {
  var LANGUAGES = [
    { text: 'JavaScript', value: 'javascript' },
    { text: 'TypeScript', value: 'typescript' },
    { text: 'HTML', value: 'html' },
    { text: 'CSS', value: 'css' },
    { text: 'Go', value: 'go' },
    { text: 'Python', value: 'python' },
    { text: 'Plain text', value: 'text' }
  ];

  function isReadOnly(editor) {
    if (window.LEA && window.LEA.readOnly) return true;
    if (window.Note && (window.Note.readOnly || window.Note.isReadOnly)) return true;
    return Boolean(editor.mode && typeof editor.mode.isReadOnly === 'function' && editor.mode.isReadOnly());
  }

  function markMutation(editor) {
    var callback = editor.leanoteMarkMutation;
    if (!callback && editor.options && typeof editor.options.get === 'function') {
      try { callback = editor.options.get('leanote_markMutation'); } catch (error) { callback = null; }
    }
    if (typeof callback === 'function') {
      callback(editor.getContent());
      return;
    }
    if (window.LeanoteEditorSession && typeof window.LeanoteEditorSession.markMutation === 'function') {
      window.LeanoteEditorSession.markMutation(editor.getContent());
    }
  }

  function showError(editor, text) {
    if (editor.notificationManager && typeof editor.notificationManager.open === 'function') {
      editor.notificationManager.open({ text: text, type: 'error', timeout: 4000 });
    }
  }

  function selectedPre(editor) {
    var node = editor.selection && editor.selection.getNode ? editor.selection.getNode() : null;
    while (node && node !== editor.getBody()) {
      if (node.nodeName === 'PRE') return node;
      node = node.parentNode;
    }
    return null;
  }

  function normalizeLanguage(value) {
    value = String(value || 'javascript');
    return LANGUAGES.some(function (item) { return item.value === value; }) ? value : 'javascript';
  }

  tinymce.PluginManager.add('leanote_code', function (editor) {
    function toggleCode() {
      if (isReadOnly(editor)) return false;
      var pre = selectedPre(editor);
      if (pre) {
        editor.selection.select(pre, true);
        editor.execCommand('mceToggleFormat', false, 'p');
      } else {
        editor.insertContent('<pre class="brush:javascript" data-language="javascript"><br></pre>');
      }
      markMutation(editor);
      return true;
    }

    function toggleInlineCode() {
      if (isReadOnly(editor)) return false;
      editor.execCommand('mceToggleFormat', false, 'code');
      markMutation(editor);
      return true;
    }

    function openLanguageDialog() {
      if (isReadOnly(editor)) return;
      var pre = selectedPre(editor);
      var current = normalizeLanguage(pre ? pre.getAttribute('data-language') : 'javascript');
      editor.windowManager.open({
        title: 'Code block',
        size: 'normal',
        body: { type: 'panel', items: [{ type: 'selectbox', name: 'language', label: 'Language', items: LANGUAGES }] },
        initialData: { language: current },
        buttons: [{ type: 'cancel', name: 'cancel', text: 'Cancel' }, { type: 'submit', name: 'save', text: 'Apply', primary: true }],
        onSubmit: function (api) {
          var value = normalizeLanguage(api.getData().language);
          if (isReadOnly(editor)) { api.close(); return; }
          if (pre) {
            var update = function () {
              editor.dom.setAttrib(pre, 'data-language', value);
              editor.dom.setAttrib(pre, 'class', 'brush:' + value);
            };
            if (editor.undoManager && typeof editor.undoManager.transact === 'function') editor.undoManager.transact(update);
            else update();
          } else {
            editor.insertContent('<pre class="brush:' + value + '" data-language="' + value + '"><br></pre>');
          }
          markMutation(editor);
          api.close();
        }
      });
    }

    function toggleAce() {
      if (isReadOnly(editor)) return false;
      if (!window.LeaAce || typeof window.LeaAce.canAce !== 'function' || !window.LeaAce.canAce()) {
        showError(editor, 'Code editor is unavailable');
        return false;
      }
      if (window.LeaAce.isAce && typeof window.LeaAce.allToPre === 'function') window.LeaAce.allToPre(editor);
      else if (typeof window.LeaAce.initAceFromContent === 'function') window.LeaAce.initAceFromContent(editor);
      return true;
    }

    function setupEditable(api) {
      var refresh = function () { api.setEnabled(!isReadOnly(editor)); };
      editor.on('NodeChange ModeChange', refresh);
      refresh();
      return function () { editor.off('NodeChange ModeChange', refresh); };
    }

    editor.addCommand('toggleCode', toggleCode);
    editor.addShortcut('ctrl+shift+c', '', 'toggleCode');
    editor.addShortcut('meta+shift+c', '', 'toggleCode');
    editor.ui.registry.addButton('leanote_code', { text: 'Code', tooltip: 'Toggle code block', onAction: toggleCode, onSetup: setupEditable });
    editor.ui.registry.addButton('leanote_inline_code', { icon: 'code', tooltip: 'Inline code', onAction: toggleInlineCode, onSetup: setupEditable });
    editor.ui.registry.addButton('leanote_ace_pre', { icon: 'code', tooltip: 'Toggle raw code', onAction: toggleAce, onSetup: setupEditable });
    editor.ui.registry.addMenuItem('leanote_code', { text: 'Code block', onAction: openLanguageDialog, onSetup: setupEditable });
    return { name: 'leanote_code', mutationAware: true, aceAware: true };
  });
})();
