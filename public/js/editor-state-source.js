(function (window) {
	function create(options) {
		var readOnly = Boolean(options && options.readOnly);
		var state = { noteId: null, loadEpoch: 0, persistedContent: '', editorBaseline: '', currentContent: '', contentRevision: 0, confirmedRevision: 0, loading: false };
		function snapshot() { return { noteId: state.noteId, loadEpoch: state.loadEpoch, persistedContent: state.persistedContent, editorBaseline: state.editorBaseline, currentContent: state.currentContent, contentRevision: state.contentRevision, confirmedRevision: state.confirmedRevision, loading: state.loading }; }
		function beginLoad(input) {
			input = input || {};
			state.loadEpoch += 1;
			state.loading = true;
			state.noteId = input.noteId || null;
			state.persistedContent = input.persistedContent || '';
			state.currentContent = '';
			state.editorBaseline = '';
			state.contentRevision = 0;
			state.confirmedRevision = 0;
			return state.loadEpoch;
		}
		function isCurrentLoad(epoch) { return epoch === state.loadEpoch; }
		function completeLoad(epoch, editorContent) {
			if (!isCurrentLoad(epoch)) return false;
			state.currentContent = editorContent || '';
			state.editorBaseline = state.currentContent;
			state.loading = false;
			return true;
		}
		function load(input) {
			var epoch = beginLoad(input);
			completeLoad(epoch, input && input.editorContent);
			return snapshot();
		}
		function setContentProgrammatically(content, epoch) {
			// Programmatic content is only part of a load transaction. Calls made
			// after loading (for example a user action) must use markMutation so
			// they receive a content revision and an explicit dirty transition.
			if (epoch === undefined || !isCurrentLoad(epoch) || !state.loading) return false;
			state.currentContent = content || '';
			return true;
		}
		function noteContentChanged(content, epoch) {
			content = content || '';
			if ((epoch !== undefined && !isCurrentLoad(epoch)) || readOnly || state.loading || content === state.currentContent) return false;
			state.currentContent = content;
			state.contentRevision += 1;
			return true;
		}
		function beginSave(serializedContent) {
			if (!state.noteId) return null;
			if (serializedContent === undefined) {
				if (state.currentContent === state.editorBaseline) return null;
				serializedContent = state.currentContent;
			} else if (typeof serializedContent !== 'string') {
				return null;
			}
			return { noteId: state.noteId, loadEpoch: state.loadEpoch, revision: state.contentRevision, content: serializedContent };
		}
		function confirmSave(capture, currentContent) {
			if (!capture || capture.noteId !== state.noteId || capture.loadEpoch !== state.loadEpoch || capture.revision < state.confirmedRevision || typeof currentContent !== 'string') return false;
			state.currentContent = currentContent;
			state.persistedContent = capture.content;
			state.editorBaseline = capture.content;
			state.confirmedRevision = capture.revision;
			return true;
		}
		return { load: load, beginLoad: beginLoad, completeLoad: completeLoad, isCurrentLoad: isCurrentLoad, setContentProgrammatically: setContentProgrammatically, noteContentChanged: noteContentChanged, markMutation: noteContentChanged, beginSave: beginSave, confirmSave: confirmSave, failSave: function () {}, setReadOnly: function (value) { readOnly = Boolean(value); }, isDirty: function () { return state.currentContent !== state.editorBaseline; }, snapshot: snapshot };
	}
	window.LeanoteEditorState = { create: create };
	window.LeanoteEditorSession = create({ readOnly: false });
})(window);
