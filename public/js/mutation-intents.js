(function (window) {
    function validRevision(value) {
        return Number.isSafeInteger(value) && value >= 0;
    }

    function create(options) {
        var active = Object.create(null);
        var locks = Object.create(null);

        function notify() { options.changed(summary()); }
        function summary() {
            var counts = { pending: 0, unknown: 0, rejected: 0 };
            Object.keys(active).forEach(function (id) { counts[active[id].state]++; });
            return counts;
        }
        function verdict(intent, result) {
            if (result === false || (result && result.Ok === false)) return 'rejected';
            if (intent.kind === 'boolean') return result === true ? 'confirmed' : 'unknown';
            if (!result || result.Ok !== true) return 'unknown';
            if (intent.kind === 'save') {
                return validRevision(result.Usn) && result.Usn >= intent.payload.ExpectedUsn ? 'confirmed' : 'unknown';
            }
            return Array.isArray(result.Item) && result.Item.length === intent.noteIds.length &&
                result.Item.every(function (note) { return note && typeof note.NoteId === 'string' && note.NoteId.length > 0; })
                ? 'confirmed' : 'unknown';
        }
        function send(intent) {
            intent.state = 'pending';
            var attempt = ++intent.attempt;
            notify();
            function settle(result, transportFailure) {
                if (intent.state !== 'pending' || attempt !== intent.attempt) return;
                intent.state = transportFailure && result !== false && !(result && result.Ok === false)
                    ? 'unknown' : verdict(intent, result);
                if (intent.state === 'confirmed') {
                    delete active[intent.payload.OperationId];
                    intent.noteIds.forEach(function (id) { delete locks[id]; });
                    notify();
                    if (intent.success) intent.success(result);
                } else {
                    notify();
                    if (intent.failure) intent.failure(result, intent.state);
                }
            }
            options.send(intent.path, intent.body, function (result) { settle(result, false); },
                function (error) { settle(error, true); });
        }
        function start(command) {
            if (command.noteIds.some(function (id) { return locks[id]; })) return null;
            if (command.kind === 'save' && !validRevision(command.payload.ExpectedUsn)) {
                var revisionError = new Error('Authoritative note revision is required');
                revisionError.code = 'REVISION_REQUIRED';
                throw revisionError;
            }
            if (!options.crypto || typeof options.crypto.getRandomValues !== 'function') {
                var randomnessError = new Error('Cryptographic randomness is unavailable');
                randomnessError.code = 'RANDOMNESS_UNAVAILABLE';
                throw randomnessError;
            }
            var bytes = new Uint8Array(16);
            options.crypto.getRandomValues(bytes);
            var id = Array.from(bytes, function (value) { return value.toString(16).padStart(2, '0'); }).join('');
            var payload = JSON.parse(JSON.stringify(command.payload));
            payload.OperationId = id;
            Object.keys(payload).forEach(function (key) {
                if (Array.isArray(payload[key])) Object.freeze(payload[key]);
            });
            Object.freeze(payload);
            var intent = { path: command.path, kind: command.kind, payload: payload,
                body: options.serialize(payload), noteIds: command.noteIds.slice(),
                success: command.success, failure: command.failure, state: 'pending', attempt: 0 };
            active[id] = intent;
            intent.noteIds.forEach(function (noteId) { locks[noteId] = intent; });
            send(intent);
            return intent;
        }
        return {
            start: start,
            get: function (noteId) { return locks[noteId]; },
            summary: summary,
            hasUnresolved: function () { return Object.keys(active).length > 0; },
            retryUnknown: function () {
                Object.keys(active).forEach(function (id) {
                    var intent = active[id];
                    if (intent && intent.state === 'unknown') send(intent);
                });
            }
        };
    }
    window.LeanoteMutationIntents = { create: create, validRevision: validRevision };
})(window);
