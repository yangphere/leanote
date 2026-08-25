package harness

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestUSNMutationPairsAndConflicts(t *testing.T) {
	_, admin, repoRoot := startBaselineServer(t)
	store := goldenStoreForTest(t, repoRoot)

	noteBefore := currentUserUSN(t, admin, "admin")
	captureGolden(t, store, admin, "usn/note_add.json", RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/note/addNote",
		Form: map[string][]string{
			"NotebookId": {fixtureNotebookID},
			"Title":      {"USN Note"},
			"Content":    {"USN note content"},
		},
		Auth: "admin",
	})
	noteID, noteUsn := fixtureNoteByTitle(t, "USN Note")
	assertGreater(t, noteUsn, noteBefore, "AddNote response Usn")
	noteDelta := captureGolden(t, store, admin, "usn/note_syncAfterAdd.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getSyncNotes", Query: map[string][]string{"afterUsn": {strconv.Itoa(noteBefore)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncContainsTitle(t, noteDelta, "USN Note")

	assertConflictEnvelope(t, captureGolden(t, store, admin, "usn/note_update_conflict_assertion.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/updateNote", Form: map[string][]string{"NoteId": {noteID}, "Title": {"USN Note Conflict"}, "Usn": {strconv.Itoa(noteUsn - 1)}}, Auth: "admin"}))
	captureGolden(t, store, admin, "usn/note_update.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/updateNote", Form: map[string][]string{"NoteId": {noteID}, "Title": {"USN Note Updated"}, "Usn": {strconv.Itoa(noteUsn)}}, Auth: "admin"})
	noteID, updatedNoteUsn := fixtureNoteByTitle(t, "USN Note Updated")
	assertGreater(t, updatedNoteUsn, noteUsn, "UpdateNote response Usn")
	noteDelta = captureGolden(t, store, admin, "usn/note_syncAfterUpdate.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getSyncNotes", Query: map[string][]string{"afterUsn": {strconv.Itoa(noteUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncContainsTitle(t, noteDelta, "USN Note Updated")

	deleteConflict := captureGolden(t, store, admin, "usn/note_deleteTrash_conflict.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/deleteTrash", Form: map[string][]string{"noteId": {noteID}, "usn": {strconv.Itoa(updatedNoteUsn - 1)}}, Auth: "admin"})
	assertConflictEnvelope(t, deleteConflict)
	captureGolden(t, store, admin, "usn/note_deleteTrash.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/deleteTrash", Form: map[string][]string{"noteId": {noteID}, "usn": {strconv.Itoa(updatedNoteUsn)}}, Auth: "admin"})
	noteDelta = captureGolden(t, store, admin, "usn/note_syncAfterDelete.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getSyncNotes", Query: map[string][]string{"afterUsn": {strconv.Itoa(updatedNoteUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncDeletedTitle(t, noteDelta, "USN Note Updated")

	notebookBefore := currentUserUSN(t, admin, "admin")
	captureGolden(t, store, admin, "usn/notebook_add.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/addNotebook", Form: map[string][]string{"title": {"USN Notebook"}, "seq": {"41"}}, Auth: "admin"})
	notebookID, notebookUsn := fixtureNotebookByTitle(t, "USN Notebook")
	assertGreater(t, notebookUsn, notebookBefore, "AddNotebook response Usn")
	notebookDelta := captureGolden(t, store, admin, "usn/notebook_syncAfterAdd.json", RequestSpec{Method: http.MethodGet, Path: "/api/notebook/getSyncNotebooks", Query: map[string][]string{"afterUsn": {strconv.Itoa(notebookBefore)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncContainsTitle(t, notebookDelta, "USN Notebook")

	captureGolden(t, store, admin, "usn/notebook_update.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/updateNotebook", Form: map[string][]string{"notebookId": {notebookID}, "title": {"USN Notebook Updated"}, "seq": {"42"}, "usn": {strconv.Itoa(notebookUsn)}}, Auth: "admin"})
	notebookID, updatedNotebookUsn := fixtureNotebookByTitle(t, "USN Notebook Updated")
	assertGreater(t, updatedNotebookUsn, notebookUsn, "UpdateNotebook response Usn")
	notebookDelta = captureGolden(t, store, admin, "usn/notebook_syncAfterUpdate.json", RequestSpec{Method: http.MethodGet, Path: "/api/notebook/getSyncNotebooks", Query: map[string][]string{"afterUsn": {strconv.Itoa(notebookUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncContainsTitle(t, notebookDelta, "USN Notebook Updated")

	notebookConflict := captureGolden(t, store, admin, "usn/notebook_delete_conflict.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/deleteNotebook", Form: map[string][]string{"notebookId": {notebookID}, "usn": {strconv.Itoa(updatedNotebookUsn - 1)}}, Auth: "admin"})
	assertConflictEnvelope(t, notebookConflict)
	beforeNotebookDelete := currentUserUSN(t, admin, "admin")
	captureGolden(t, store, admin, "usn/notebook_delete.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/deleteNotebook", Form: map[string][]string{"notebookId": {notebookID}, "usn": {strconv.Itoa(updatedNotebookUsn)}}, Auth: "admin"})
	if after := currentUserUSN(t, admin, "admin"); after != beforeNotebookDelete {
		t.Fatalf("DeleteNotebook bumped user Usn from %d to %d; baseline requires the known no-bump behavior", beforeNotebookDelete, after)
	}
	notebookDelta = captureGolden(t, store, admin, "usn/notebook_syncAfterDelete.json", RequestSpec{Method: http.MethodGet, Path: "/api/notebook/getSyncNotebooks", Query: map[string][]string{"afterUsn": {strconv.Itoa(updatedNotebookUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncEmpty(t, notebookDelta)

	tagBefore := currentUserUSN(t, admin, "admin")
	captureGolden(t, store, admin, "usn/tag_add.json", RequestSpec{Method: http.MethodPost, Path: "/api/tag/addTag", Form: map[string][]string{"tag": {"usn-tag"}}, Auth: "admin"})
	tagUsn := fixtureTagByName(t, "usn-tag")
	assertGreater(t, tagUsn, tagBefore, "AddTag response Usn")
	tagDelta := captureGolden(t, store, admin, "usn/tag_syncAfterAdd.json", RequestSpec{Method: http.MethodGet, Path: "/api/tag/getSyncTags", Query: map[string][]string{"afterUsn": {strconv.Itoa(tagBefore)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncContainsTitle(t, tagDelta, "usn-tag")

	tagConflict := captureGolden(t, store, admin, "usn/tag_delete_conflict.json", RequestSpec{Method: http.MethodPost, Path: "/api/tag/deleteTag", Form: map[string][]string{"tag": {"usn-tag"}, "usn": {strconv.Itoa(tagUsn - 1)}}, Auth: "admin"})
	assertConflictEnvelope(t, tagConflict)
	captureGolden(t, store, admin, "usn/tag_delete.json", RequestSpec{Method: http.MethodPost, Path: "/api/tag/deleteTag", Form: map[string][]string{"tag": {"usn-tag"}, "usn": {strconv.Itoa(tagUsn)}}, Auth: "admin"})
	if after := currentUserUSN(t, admin, "admin"); after <= tagUsn {
		t.Fatalf("DeleteTag did not bump user Usn beyond %d: %d", tagUsn, after)
	}
	if stored := fixtureTagByName(t, "usn-tag"); stored != tagUsn {
		t.Fatalf("DeleteTag stored Usn = %d, want the known old input Usn %d", stored, tagUsn)
	}
	tagDelta = captureGolden(t, store, admin, "usn/tag_syncAfterDelete.json", RequestSpec{Method: http.MethodGet, Path: "/api/tag/getSyncTags", Query: map[string][]string{"afterUsn": {strconv.Itoa(tagUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
	assertSyncEmpty(t, tagDelta)
}

func TestUSNSyncBoundaries(t *testing.T) {
	_, admin, repoRoot := startBaselineServer(t)
	store := goldenStoreForTest(t, repoRoot)

	for index, mutation := range []RequestSpec{
		{Method: http.MethodPost, Path: "/api/note/addNote", Form: map[string][]string{"NotebookId": {fixtureNotebookID}, "Title": {"Boundary Note One"}, "Content": {"one"}}, Auth: "admin"},
		{Method: http.MethodPost, Path: "/api/note/addNote", Form: map[string][]string{"NotebookId": {fixtureNotebookID}, "Title": {"Boundary Note Two"}, "Content": {"two"}}, Auth: "admin"},
		{Method: http.MethodPost, Path: "/api/notebook/addNotebook", Form: map[string][]string{"title": {"Boundary Notebook One"}, "seq": {"51"}}, Auth: "admin"},
		{Method: http.MethodPost, Path: "/api/notebook/addNotebook", Form: map[string][]string{"title": {"Boundary Notebook Two"}, "seq": {"52"}}, Auth: "admin"},
		{Method: http.MethodPost, Path: "/api/tag/addTag", Form: map[string][]string{"tag": {"boundary-tag-one"}}, Auth: "admin"},
		{Method: http.MethodPost, Path: "/api/tag/addTag", Form: map[string][]string{"tag": {"boundary-tag-two"}}, Auth: "admin"},
	} {
		captureGolden(t, store, admin, "usn/boundary_mutation_"+strconv.Itoa(index)+"_"+requestActionName(mutation.Path)+".json", mutation)
	}

	for _, endpoint := range []string{"note/getSyncNotes", "notebook/getSyncNotebooks", "tag/getSyncTags"} {
		all := captureGolden(t, store, admin, "usn/"+requestActionName(endpoint)+"_all.json", RequestSpec{Method: http.MethodGet, Path: "/api/" + endpoint, Query: map[string][]string{"afterUsn": {"0"}, "maxEntry": {"100"}}, Auth: "admin"})
		allEntries := decodeSyncEntries(t, all)
		if len(allEntries) < 2 {
			t.Fatalf("%s afterUsn=0 returned %d entries, want at least two for pagination", endpoint, len(allEntries))
		}
		firstPage := captureGolden(t, store, admin, "usn/"+requestActionName(endpoint)+"_firstPage.json", RequestSpec{Method: http.MethodGet, Path: "/api/" + endpoint, Query: map[string][]string{"afterUsn": {"0"}, "maxEntry": {"1"}}, Auth: "admin"})
		firstEntries := decodeSyncEntries(t, firstPage)
		if len(firstEntries) != 1 {
			t.Fatalf("%s first page length = %d, want 1", endpoint, len(firstEntries))
		}
		firstUsn := entryUSN(t, firstEntries[0])
		secondPage := captureGolden(t, store, admin, "usn/"+requestActionName(endpoint)+"_secondPage.json", RequestSpec{Method: http.MethodGet, Path: "/api/" + endpoint, Query: map[string][]string{"afterUsn": {strconv.Itoa(firstUsn)}, "maxEntry": {"100"}}, Auth: "admin"})
		if len(decodeSyncEntries(t, secondPage)) == 0 {
			t.Fatalf("%s second page after Usn %d is empty", endpoint, firstUsn)
		}
		maximum := currentUserUSN(t, admin, "admin")
		emptyAtMaximum := captureGolden(t, store, admin, "usn/"+requestActionName(endpoint)+"_atMaximum.json", RequestSpec{Method: http.MethodGet, Path: "/api/" + endpoint, Query: map[string][]string{"afterUsn": {strconv.Itoa(maximum)}, "maxEntry": {"100"}}, Auth: "admin"})
		assertSyncEmpty(t, emptyAtMaximum)
		emptyAfterHuge := captureGolden(t, store, admin, "usn/"+requestActionName(endpoint)+"_afterHuge.json", RequestSpec{Method: http.MethodGet, Path: "/api/" + endpoint, Query: map[string][]string{"afterUsn": {"999999999"}, "maxEntry": {"100"}}, Auth: "admin"})
		assertSyncEmpty(t, emptyAfterHuge)
	}
}

func currentUserUSN(t testing.TB, client *Client, identity string) int {
	t.Helper()
	snapshot, err := client.Do(RequestSpec{Method: http.MethodGet, Path: "/api/user/getSyncState", Auth: identity})
	if err != nil {
		t.Fatalf("get current user Usn: %v", err)
	}
	var state map[string]interface{}
	if err := json.Unmarshal(snapshot.Body, &state); err != nil {
		t.Fatalf("decode sync state: %v", err)
	}
	value, ok := state["LastSyncUsn"].(float64)
	if !ok {
		t.Fatalf("LastSyncUsn missing from %s", describeSnapshot(snapshot))
	}
	return int(value)
}

func decodeSyncEntries(t testing.TB, snapshot Snapshot) []map[string]interface{} {
	t.Helper()
	var entries []map[string]interface{}
	if err := json.Unmarshal(snapshot.Body, &entries); err != nil {
		t.Fatalf("decode sync entries: %v; %s", err, describeSnapshot(snapshot))
	}
	return entries
}

func entryUSN(t testing.TB, entry map[string]interface{}) int {
	t.Helper()
	value, ok := entry["Usn"].(float64)
	if !ok {
		t.Fatalf("Usn missing from sync entry: %#v", entry)
	}
	return int(value)
}

func assertSyncContainsTitle(t testing.TB, snapshot Snapshot, title string) {
	t.Helper()
	for _, entry := range decodeSyncEntries(t, snapshot) {
		if entry["Title"] == title || entry["Tag"] == title {
			return
		}
	}
	t.Fatalf("sync response does not contain %q: %s", title, describeSnapshot(snapshot))
}

func assertSyncDeletedTitle(t testing.TB, snapshot Snapshot, title string) {
	t.Helper()
	for _, entry := range decodeSyncEntries(t, snapshot) {
		if entry["Title"] == title && entry["IsDeleted"] == true {
			return
		}
	}
	t.Fatalf("sync response does not contain deleted %q: %s", title, describeSnapshot(snapshot))
}

func assertSyncEmpty(t testing.TB, snapshot Snapshot) {
	t.Helper()
	if entries := decodeSyncEntries(t, snapshot); len(entries) != 0 {
		t.Fatalf("sync response has %d entries, want empty: %s", len(entries), describeSnapshot(snapshot))
	}
}

func assertConflictEnvelope(t testing.TB, snapshot Snapshot) {
	t.Helper()
	var response struct {
		OK  bool
		Msg string
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if response.OK || response.Msg != "conflict" {
		t.Fatalf("conflict response = %s", describeSnapshot(snapshot))
	}
}

func assertGreater(t testing.TB, got, lowerBound int, label string) {
	t.Helper()
	if got <= lowerBound {
		t.Fatalf("%s = %d, want greater than %d", label, got, lowerBound)
	}
}

func requestActionName(path string) string {
	return strings.ReplaceAll(strings.Trim(path, "/"), "/", "_")
}
