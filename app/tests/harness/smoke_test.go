package harness

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGoldenWebOwnershipControllers(t *testing.T) {
	server, _, repoRoot := startBaselineServer(t)
	store := goldenStoreForTest(t, repoRoot)
	web := NewClient(server.BaseURL)
	loginWebSession(t, web)

	captureGolden(t, store, web, "web/note_listNotes.json", RequestSpec{Method: http.MethodGet, Path: "/note/listNotes", Query: map[string][]string{"notebookId": {fixtureNotebookID}}})
	captureGolden(t, store, web, "web/notebook_getNotebooks.json", RequestSpec{Method: http.MethodGet, Path: "/notebook/getNotebooks"})
	captureGolden(t, store, web, "web/tag_updateTag.json", RequestSpec{Method: http.MethodPost, Path: "/tag/updateTag", Form: map[string][]string{"tag": {"web-golden-tag"}}})
	captureGolden(t, store, web, "web/share_listShareNotes.json", RequestSpec{Method: http.MethodGet, Path: "/share/listShareNotes", Query: map[string][]string{"notebookId": {""}, "userId": {fixtureAdminID}}})
	// The same share lookup must be exercised with the recipient identity too;
	// sharing behavior depends on both the session user and the requested owner.
	demoWeb := NewClient(server.BaseURL)
	loginWebSessionAs(t, demoWeb, "demo@leanote.com", "demo@leanote.com")
	captureGolden(t, store, demoWeb, "web/share_listShareNotes_demo.json", RequestSpec{Method: http.MethodGet, Path: "/share/listShareNotes", Query: map[string][]string{"notebookId": {fixtureNotebookID}, "userId": {fixtureAdminID}}})
	captureGolden(t, store, web, "web/attach_getAttachs.json", RequestSpec{Method: http.MethodGet, Path: "/attach/getAttachs", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}})
	captureGolden(t, store, web, "web/album_getAlbums.json", RequestSpec{Method: http.MethodGet, Path: "/album/getAlbums"})
	captureGolden(t, store, web, "web/file_getImages.json", RequestSpec{Method: http.MethodGet, Path: "/file/getImages", Query: map[string][]string{"albumId": {""}, "key": {""}, "page": {"1"}}})
}

func TestWebAdminMemberAndControllerSmoke(t *testing.T) {
	server, _, _ := startBaselineServer(t)
	web := NewClient(server.BaseURL)
	loginWebSession(t, web)

	assertJSONSmoke(t, web, RequestSpec{Method: http.MethodGet, Path: "/blog/getPostStat", Query: map[string][]string{"noteId": {fixtureActiveNoteID}}}, "Ok", "Item")
	assertJSONSmoke(t, web, RequestSpec{Method: http.MethodPost, Path: "/adminSetting/exportPdf", Form: map[string][]string{"path": {""}}}, "Ok")
	assertJSONSmoke(t, web, RequestSpec{Method: http.MethodPost, Path: "/memberGroup/addGroup", Form: map[string][]string{"title": {"Regression smoke group"}}}, "Ok")

	anonymous := NewClient(server.BaseURL)
	status, headers, _, err := anonymous.doRaw(RequestSpec{Method: http.MethodGet, Path: "/note"})
	if err != nil {
		t.Fatalf("request anonymous /note: %v", err)
	}
	if status != http.StatusFound || headers.Get("Location") != "/login" {
		t.Fatalf("anonymous /note = status %d location %q, want 302 /login", status, headers.Get("Location"))
	}

	assertHTMLPage(t, web, "/", http.StatusOK)
	assertHTMLPage(t, web, "/login", http.StatusOK)
	assertHTMLPage(t, web, "/note", http.StatusOK)
	assertHTMLPage(t, web, "/blog", http.StatusOK)
	assertHTMLPage(t, web, "/index", http.StatusOK)
	assertHTMLPage(t, web, "/preview", http.StatusNotFound)

	demo := NewClient(server.BaseURL)
	status, headers, _, err = demo.doRaw(RequestSpec{Method: http.MethodGet, Path: "/demo"})
	if err != nil {
		t.Fatalf("request /demo: %v", err)
	}
	if status != http.StatusFound || headers.Get("Location") != "/note" {
		t.Fatalf("/demo = status %d location %q, want 302 /note", status, headers.Get("Location"))
	}
}

func loginWebSession(t testing.TB, client *Client) {
	t.Helper()
	loginWebSessionAs(t, client, "admin@leanote.com", "abc123")
}

func loginWebSessionAs(t testing.TB, client *Client, email, password string) {
	t.Helper()
	status, _, body, err := client.doRaw(RequestSpec{
		Method: http.MethodPost,
		Path:   "/doLogin",
		Form:   map[string][]string{"email": {email}, "pwd": {password}, "captcha": {""}},
	})
	if err != nil {
		t.Fatalf("web login request: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("web login status = %d", status)
	}
	var response struct{ OK bool }
	if err := json.Unmarshal(body, &response); err != nil || !response.OK {
		t.Fatalf("web login response = %q, decode error = %v", body, err)
	}
}

func assertJSONSmoke(t testing.TB, client *Client, request RequestSpec, requiredKeys ...string) {
	t.Helper()
	snapshot, err := client.Do(request)
	if err != nil {
		t.Fatalf("JSON smoke %s: %v", request.Path, err)
	}
	if snapshot.Status != http.StatusOK {
		t.Fatalf("JSON smoke %s status = %d", request.Path, snapshot.Status)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("JSON smoke %s decode: %v", request.Path, err)
	}
	for _, key := range requiredKeys {
		if _, ok := response[key]; !ok {
			t.Fatalf("JSON smoke %s missing %q: %s", request.Path, key, describeSnapshot(snapshot))
		}
	}
}

func assertHTMLPage(t testing.TB, client *Client, path string, expectedStatus int) {
	t.Helper()
	status, _, body, err := client.doRaw(RequestSpec{Method: http.MethodGet, Path: path})
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	if status != expectedStatus {
		t.Fatalf("%s status = %d, want %d", path, status, expectedStatus)
	}
	if !strings.Contains(strings.ToLower(string(body)), "<html") {
		t.Fatalf("%s did not return an HTML document", path)
	}
}
