(function () {
  tinymce.PluginManager.add('leanote_nav', function (editor) {
    var previous = '';

    function refresh() {
      var body = editor.getBody();
      var target = document.getElementById('leanoteNavContent');
      if (!body || !target) return;
      var html = body.innerHTML;
      if (html === previous) return;
      previous = html;
      var list = document.createElement('ul');
      Array.prototype.forEach.call(body.querySelectorAll('h1,h2,h3,h4,h5,h6'), function (heading) {
        var tag = heading.tagName.toLowerCase();
        var text = heading.textContent || '';
        var item = document.createElement('li');
        item.className = 'nav-' + tag;
        var link = document.createElement('a');
        link.setAttribute('data-a', tag + '-' + encodeURI(text));
        link.textContent = text;
        item.appendChild(link);
        list.appendChild(item);
      });
      target.replaceChildren(list);
      target.style.height = 'auto';
      if (!list.children.length) target.textContent = '\u00a0 Nothing...';
    }

    editor.on('init SetContent Undo Redo Paste ExecCommand Change NodeChange', refresh);
    editor.on('click', refresh);
    return { name: 'leanote_nav', externalOnly: true, mutationAware: false };
  });
})();
