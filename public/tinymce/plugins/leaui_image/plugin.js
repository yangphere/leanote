(function () {
  function isReadOnly(editor) {
    if (window.LEA && window.LEA.readOnly) return true;
    if (window.Note && (window.Note.readOnly || window.Note.isReadOnly)) return true;
    return Boolean(editor.mode && typeof editor.mode.isReadOnly === 'function' && editor.mode.isReadOnly());
  }

  function markMutation(editor) {
    var content = editor.getContent();
    var callback = editor.leanoteMarkMutation;
    if (!callback && editor.options && typeof editor.options.get === 'function') {
      try { callback = editor.options.get('leanote_markMutation'); } catch (error) { callback = null; }
    }
    if (typeof callback === 'function') {
      callback(content);
      return;
    }
    if (window.LeanoteEditorSession && typeof window.LeanoteEditorSession.markMutation === 'function') {
      window.LeanoteEditorSession.markMutation(content);
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

  function getSelectedImage(editor) {
    var node = editor.selection && editor.selection.getNode ? editor.selection.getNode() : null;
    return node && node.nodeName === 'IMG' && !node.getAttribute('data-mind-json') ? node : null;
  }

  function imageDataFromNode(node) {
    if (!node) return null;
    return {
      src: node.getAttribute('src') || node.getAttribute('data-src') || '',
      alt: node.getAttribute('alt') || '',
      title: node.getAttribute('title') || '',
      width: node.getAttribute('width') || '',
      height: node.getAttribute('height') || ''
    };
  }

  function normalizeImageData(value) {
    if (!value || typeof value !== 'object') return null;
    var src = typeof value.src === 'string' ? value.src.trim() : '';
    if (!src) return null;
    if (/^(?:javascript|vbscript|file):/i.test(src)) return null;
    if (/^data:/i.test(src) && !/^data:image\//i.test(src)) return null;
    return {
      src: src,
      alt: typeof value.alt === 'string' ? value.alt : '',
      title: typeof value.title === 'string' ? value.title : '',
      width: value.width == null ? '' : String(value.width),
      height: value.height == null ? '' : String(value.height)
    };
  }

  function imageDataFromHtml(html) {
    if (typeof html !== 'string' || !html) return null;
    var holder = document.createElement('div');
    holder.innerHTML = html;
    return normalizeImageData(imageDataFromNode(holder.querySelector('img')));
  }

  function imageHtml(data) {
    var html = '<img src="' + escapeAttribute(data.src) + '"';
    if (data.alt) html += ' alt="' + escapeAttribute(data.alt) + '"';
    if (data.title) html += ' title="' + escapeAttribute(data.title) + '"';
    if (/^\d+(?:\.\d+)?$/.test(data.width)) html += ' width="' + escapeAttribute(data.width) + '"';
    if (/^\d+(?:\.\d+)?$/.test(data.height)) html += ' height="' + escapeAttribute(data.height) + '"';
    return html + ' />';
  }

  function updateImage(editor, node, data) {
    if (!node || !data || isReadOnly(editor)) return false;
    var dom = editor.dom;
    var update = function () {
      dom.setAttrib(node, 'src', data.src);
      dom.setAttrib(node, 'alt', data.alt || null);
      dom.setAttrib(node, 'title', data.title || null);
      dom.setAttrib(node, 'width', /^\d+(?:\.\d+)?$/.test(data.width) ? data.width : null);
      dom.setAttrib(node, 'height', /^\d+(?:\.\d+)?$/.test(data.height) ? data.height : null);
    };
    if (editor.undoManager && typeof editor.undoManager.transact === 'function') editor.undoManager.transact(update);
    else update();
    markMutation(editor);
    return true;
  }

  tinymce.PluginManager.add('leaui_image', function (editor) {
    function applyImage(data) {
      data = normalizeImageData(data);
      if (!data || isReadOnly(editor)) return false;
      var selected = getSelectedImage(editor);
      if (selected) return updateImage(editor, selected, data);
      editor.insertContent(imageHtml(data));
      markMutation(editor);
      return true;
    }

    function openAlbum() {
      if (isReadOnly(editor)) return;
      var selected = imageDataFromNode(getSelectedImage(editor));
      window.LEAUI_DATAS = selected ? [selected] : [];
      var baseUrl = '/tinymce/plugins/leaui_image/index.html';
      editor.windowManager.openUrl({
        title: 'Image',
        url: baseUrl + '?' + Date.now(),
        width: Math.min(window.innerWidth - 10, 805),
        height: Math.min(window.innerHeight - 100, 365),
        buttons: [{ type: 'cancel', name: 'cancel', text: 'Cancel' }],
        onMessage: function (api, details) {
          if (!details || (details.mceAction !== 'insertImage' && details.mceAction !== 'insertContent')) return;
          var data = details.data || details.image || null;
          if (!data && details.mceAction === 'insertContent') data = imageDataFromHtml(details.content);
          if (Array.isArray(details.images)) data = details.images[0];
          if (applyImage(data)) api.close();
          else showError(editor, 'Unable to insert image');
        }
      });
    }

    function setupEditable(api) {
      var refresh = function () { api.setEnabled(!isReadOnly(editor)); };
      editor.on('NodeChange ModeChange', refresh);
      refresh();
      return function () { editor.off('NodeChange ModeChange', refresh); };
    }

    editor.ui.registry.addButton('leaui_image', {
      icon: 'image',
      tooltip: 'Insert/edit image',
      onAction: openAlbum,
      onSetup: setupEditable
    });
    editor.ui.registry.addMenuItem('leaui_image', {
      icon: 'image',
      text: 'Insert image',
      onAction: openAlbum,
      onSetup: setupEditable
    });
    editor.on('dragstart', function (event) {
      if (isReadOnly(editor)) event.preventDefault();
    });
    return { name: 'leaui_image', mutationAware: true, urlDialog: true };
  });
})();
