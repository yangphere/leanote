(function () {
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

  function escapeAttribute(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/"/g, '&quot;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function selectedMindMap(editor) {
    var node = editor.selection && editor.selection.getNode ? editor.selection.getNode() : null;
    if (!node || node.nodeName !== 'IMG') return null;
    var json = node.getAttribute('data-mind-json');
    return json ? { node: node, json: json } : null;
  }

  function normalizeData(details) {
    var json = details && (details.json || details.mindJson);
    if (typeof json !== 'string' || !json.trim()) return null;
    var src = details.src || details.image || '';
    if (typeof src !== 'string' || !src.trim()) return null;
    src = src.trim();
    if (/^(?:javascript|vbscript|file):/i.test(src)) return null;
    if (/^data:/i.test(src) && !/^data:image\//i.test(src)) return null;
    try { JSON.parse(json); } catch (error) {
      try { JSON.parse(json.replace(/Ж/g, "'")); } catch (legacyError) { return null; }
    }
    return { src: src, json: json };
  }

  var MIND_ICON = '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">'
    + '<path d="M12 12V5M12 12v7M12 12H5M12 12h7" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>'
    + '<circle cx="12" cy="12" r="3" fill="currentColor"/>'
    + '<circle cx="5" cy="5" r="2" fill="currentColor"/><circle cx="19" cy="5" r="2" fill="currentColor"/>'
    + '<circle cx="5" cy="19" r="2" fill="currentColor"/><circle cx="19" cy="19" r="2" fill="currentColor"/>'
    + '</svg>';

  tinymce.PluginManager.add('leaui_mindmap', function (editor, url) {
    function applyMindMap(details) {
      var data = normalizeData(details);
      if (!data || isReadOnly(editor)) return false;
      var selected = selectedMindMap(editor);
      if (selected) {
        var update = function () {
          editor.dom.setAttrib(selected.node, 'src', data.src);
          editor.dom.setAttrib(selected.node, 'data-mind-json', data.json);
        };
        if (editor.undoManager && typeof editor.undoManager.transact === 'function') editor.undoManager.transact(update);
        else update();
      } else {
        editor.insertContent('<img src="' + escapeAttribute(data.src) + '" data-mind-json="' + escapeAttribute(data.json) + '" />');
      }
      markMutation(editor);
      return true;
    }

    function openMindMap() {
      if (isReadOnly(editor)) return;
      var selected = selectedMindMap(editor);
      window.LEAUI_MIND = selected ? { json: selected.json } : {};
      var language = editor.options.get('language') || 'en-US';
      editor.windowManager.openUrl({
        title: 'Mind Map',
        url: url + '/mindmap/index.html?i=1&lang=' + encodeURIComponent(language),
        width: Math.max(window.innerWidth - 10, 640),
        height: Math.max(window.innerHeight - 150, 420),
        buttons: [{ type: 'cancel', name: 'cancel', text: 'Cancel' }],
        onMessage: function (api, details) {
          if (!details || details.mceAction !== 'insertMindMap') return;
          if (applyMindMap(details)) api.close();
          else showError(editor, 'Unable to insert mind map');
        }
      });
    }

    function setupEditable(api) {
      var refresh = function () { api.setEnabled(!isReadOnly(editor)); };
      editor.on('NodeChange ModeChange', refresh);
      refresh();
      return function () { editor.off('NodeChange ModeChange', refresh); };
    }

    editor.ui.registry.addIcon('mind', MIND_ICON);
    editor.ui.registry.addButton('leaui_mindmap', {
      icon: 'mind', tooltip: 'Insert/edit mind map', onAction: openMindMap, onSetup: setupEditable
    });
    editor.ui.registry.addMenuItem('leaui_mindmap', {
      icon: 'mind', text: 'Insert mind map', onAction: openMindMap, onSetup: setupEditable
    });
    return { name: 'leaui_mindmap', mutationAware: true, urlDialog: true, dataAttribute: 'data-mind-json' };
  });
})();
