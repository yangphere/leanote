package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const wkhtmltopdfPath = "/usr/local/bin/wkhtmltopdf"

func TestGoldenAPIActions(t *testing.T) {
	server, admin, repoRoot := startBaselineServer(t)
	store := goldenStoreForTest(t, repoRoot)
	demo := NewClient(server.BaseURL)
	if err := demo.Login("demo", "demo@leanote.com", "demo@leanote.com"); err != nil {
		t.Fatalf("login fixture demo: %v", err)
	}
	t.Cleanup(func() { removeGeneratedAdminLogo(t, repoRoot) })

	protectedActions := []string{
		"auth/logout",
		"user/info", "user/updateUsername", "user/updatePwd", "user/getSyncState", "user/updateLogo",
		"notebook/getSyncNotebooks", "notebook/getNotebooks", "notebook/addNotebook", "notebook/updateNotebook", "notebook/deleteNotebook",
		"note/getSyncNotes", "note/getNotes", "note/getTrashNotes", "note/getNote", "note/getNoteAndContent", "note/getNoteContent", "note/addNote", "note/updateNote", "note/deleteTrash", "note/exportPdf",
		"tag/getSyncTags", "tag/addTag", "tag/deleteTag",
	}
	for _, action := range protectedActions {
		path := "/api/" + action
		for _, auth := range []string{"none", "invalid"} {
			snapshot := captureGolden(t, store, NewClient(server.BaseURL), "api/"+action+"_"+auth+".json", RequestSpec{
				Method: http.MethodGet,
				Path:   path,
				Auth:   auth,
			})
			assertNotLoggedInEnvelope(t, snapshot)
		}
	}

	captureGolden(t, store, admin, "api/auth_login.json", RequestSpec{
		Method: http.MethodGet,
		Path:   "/api/auth/login",
		Query:  map[string][]string{"email": {"admin@leanote.com"}, "pwd": {"abc123"}},
	})
	captureGolden(t, store, NewClient(server.BaseURL), "api/auth_login_invalidPassword.json", RequestSpec{
		Method: http.MethodGet,
		Path:   "/api/auth/login",
		Query:  map[string][]string{"email": {"admin@leanote.com"}, "pwd": {"wrong-password"}},
	})
	captureGolden(t, store, NewClient(server.BaseURL), "api/auth_register.json", RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/auth/register",
		Form:   map[string][]string{"email": {"golden-regression@example.test"}, "pwd": {"golden-pass"}},
	})

	captureGolden(t, store, admin, "api/user_info.json", RequestSpec{Method: http.MethodGet, Path: "/api/user/info", Auth: "admin"})
	captureGolden(t, store, admin, "api/user_getSyncState.json", RequestSpec{Method: http.MethodGet, Path: "/api/user/getSyncState", Auth: "admin"})
	captureGolden(t, store, admin, "api/user_updateUsername.json", RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/user/updateUsername",
		Form:   map[string][]string{"username": {"goldenadmin"}},
		Auth:   "admin",
	})
	captureGolden(t, store, admin, "api/user_updateLogo.json", RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/user/updateLogo",
		Files: map[string]FilePart{
			"file": {Filename: "golden-logo.png", ContentType: "image/png", Body: []byte("golden image payload")},
		},
		Auth: "admin",
	})

	captureGolden(t, store, admin, "api/notebook_getSyncNotebooks.json", RequestSpec{Method: http.MethodGet, Path: "/api/notebook/getSyncNotebooks", Query: map[string][]string{"afterUsn": {"0"}, "maxEntry": {"100"}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/notebook_getNotebooks.json", RequestSpec{Method: http.MethodGet, Path: "/api/notebook/getNotebooks", Auth: "admin"})
	captureGolden(t, store, admin, "api/notebook_addNotebook.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/addNotebook", Form: map[string][]string{"title": {"Golden Notebook"}, "parentNotebookId": {""}, "seq": {"11"}}, Auth: "admin"})
	notebookID, notebookUsn := fixtureNotebookByTitle(t, "Golden Notebook")
	captureGolden(t, store, admin, "api/notebook_updateNotebook.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/updateNotebook", Form: map[string][]string{"notebookId": {notebookID}, "title": {"Golden Notebook Updated"}, "parentNotebookId": {""}, "seq": {"12"}, "usn": {itoa(notebookUsn)}}, Auth: "admin"})
	notebookID, notebookUsn = fixtureNotebookByTitle(t, "Golden Notebook Updated")
	captureGolden(t, store, admin, "api/notebook_deleteNotebook.json", RequestSpec{Method: http.MethodPost, Path: "/api/notebook/deleteNotebook", Form: map[string][]string{"notebookId": {notebookID}, "usn": {itoa(notebookUsn)}}, Auth: "admin"})

	captureGolden(t, store, admin, "api/note_getSyncNotes.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getSyncNotes", Query: map[string][]string{"afterUsn": {"0"}, "maxEntry": {"100"}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNotes.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNotes", Query: map[string][]string{"notebookId": {fixtureNotebookID}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getTrashNotes.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getTrashNotes", Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNote.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNote", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNoteAndContent.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNoteAndContent", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNoteContent.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNoteContent", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNotes_invalidNotebookId.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNotes", Query: map[string][]string{"notebookId": {"invalid"}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/note_getNote_notFound.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNote", Query: map[string][]string{"noteId": {"000000000000000000000000"}}, Auth: "admin"})
	captureGolden(t, store, demo, "api/note_getNote_demoForbidden.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/getNote", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "demo"})
	captureGolden(t, store, admin, "api/note_addNote.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/addNote", Form: map[string][]string{"NotebookId": {fixtureNotebookID}, "Title": {"Golden Note"}, "Content": {"Golden note content"}, "IsMarkdown": {"false"}}, Auth: "admin"})
	noteID, noteUsn := fixtureNoteByTitle(t, "Golden Note")
	captureGolden(t, store, admin, "api/note_updateNote.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/updateNote", Form: map[string][]string{"NoteId": {noteID}, "Title": {"Golden Note Updated"}, "Usn": {itoa(noteUsn)}}, Auth: "admin"})
	noteID, noteUsn = fixtureNoteByTitle(t, "Golden Note Updated")
	captureGolden(t, store, admin, "api/note_deleteTrash.json", RequestSpec{Method: http.MethodPost, Path: "/api/note/deleteTrash", Form: map[string][]string{"noteId": {noteID}, "usn": {itoa(noteUsn)}}, Auth: "admin"})

	captureGolden(t, store, admin, "api/tag_getSyncTags.json", RequestSpec{Method: http.MethodGet, Path: "/api/tag/getSyncTags", Query: map[string][]string{"afterUsn": {"0"}, "maxEntry": {"100"}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/tag_addTag.json", RequestSpec{Method: http.MethodPost, Path: "/api/tag/addTag", Form: map[string][]string{"tag": {"golden-tag"}}, Auth: "admin"})
	tagUsn := fixtureTagByName(t, "golden-tag")
	captureGolden(t, store, admin, "api/tag_deleteTag.json", RequestSpec{Method: http.MethodPost, Path: "/api/tag/deleteTag", Form: map[string][]string{"tag": {"golden-tag"}, "usn": {itoa(tagUsn)}}, Auth: "admin"})

	seedBinaryFiles(t, repoRoot)
	captureGolden(t, store, admin, "api/file_getImage.json", RequestSpec{Method: http.MethodGet, Path: "/api/file/getImage", Query: map[string][]string{"fileId": {seedImageID}}, Auth: "admin", Binary: true})
	captureGolden(t, store, admin, "api/file_getAttach.json", RequestSpec{Method: http.MethodGet, Path: "/api/file/getAttach", Query: map[string][]string{"fileId": {seedAttachID}}, Auth: "admin", Binary: true})
	captureGolden(t, store, admin, "api/file_getAllAttachs.json", RequestSpec{Method: http.MethodGet, Path: "/api/file/getAllAttachs", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "admin", Binary: true})
	for _, action := range []string{"getImage", "getAttach", "getAllAttachs"} {
		captureGolden(t, store, NewClient(server.BaseURL), "api/file_"+action+"_noToken.json", RequestSpec{Method: http.MethodGet, Path: "/api/file/" + action, Query: map[string][]string{"fileId": {seedImageID}, "noteId": {fixtureActiveNoteID}}})
	}

	captureGolden(t, store, admin, "api/user_updatePwd.json", RequestSpec{Method: http.MethodPost, Path: "/api/user/updatePwd", Form: map[string][]string{"oldPwd": {"abc123"}, "pwd": {"golden-pass"}}, Auth: "admin"})
	captureGolden(t, store, admin, "api/auth_logout.json", RequestSpec{Method: http.MethodPost, Path: "/api/auth/logout", Auth: "admin"})
}

func TestGoldenExportPdf(t *testing.T) {
	mode, err := GoldenModeFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join(repoRoot, "app", "tests", "golden", "api", "note_exportPdf.json")
	if mode == Replay {
		if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
			t.Skipf("ExportPdf replay skipped until the reviewed Linux golden exists at %s; run the manual record-export-pdf job", goldenPath)
		} else if err != nil {
			t.Fatal(err)
		}
		if err := requireExecutable(wkhtmltopdfPath); err != nil {
			t.Skipf("ExportPdf replay skipped: %v", err)
		}
	} else if err := requireExecutable(wkhtmltopdfPath); err != nil {
		t.Fatalf("ExportPdf record requires %s: %v", wkhtmltopdfPath, err)
	}
	_, admin, repoRoot := startBaselineServer(t)
	t.Cleanup(func() {
		// ExportPdf writes a random-named file before returning its response.
		// Keep that production side effect out of the worktree after recording.
		_ = os.RemoveAll(filepath.Join(repoRoot, "files", "export_pdf"))
	})
	store := goldenStoreForTest(t, repoRoot)
	captureGolden(t, store, admin, "api/note_exportPdf.json", RequestSpec{Method: http.MethodGet, Path: "/api/note/exportPdf", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}, Auth: "admin", Binary: true})
}

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("not an executable file")
	}
	return nil
}

func assertNotLoggedInEnvelope(t testing.TB, snapshot Snapshot) {
	t.Helper()
	var response struct {
		OK  *bool
		Msg string
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode unauthenticated response %s: %v", describeSnapshot(snapshot), err)
	}
	if response.OK == nil || *response.OK || response.Msg != "NOTLOGIN" {
		t.Fatalf("unexpected unauthenticated envelope: %s", describeSnapshot(snapshot))
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
