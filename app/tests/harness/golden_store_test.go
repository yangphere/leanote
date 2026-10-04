package harness

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplayDoesNotCreateMissingGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	store := GoldenStore{Mode: Replay, Root: dir}

	err := store.Assert("missing.json", Snapshot{Status: 200, Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(`{"Ok":true}`)})
	if err == nil {
		t.Fatal("Assert() error = nil, want missing golden error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("replay created %s: stat error = %v", path, statErr)
	}
}

func TestReplayDoesNotRewriteMismatchedGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "case.json")
	original := []byte(`{"status":200,"headers":{"Content-Type":"application/json"},"body":"{\"Ok\":true}"}` + "\n")
	if err := ioutil.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := GoldenStore{Mode: Replay, Root: dir}

	err := store.Assert("case.json", Snapshot{Status: 200, Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(`{"Ok":false}`)})
	if err == nil {
		t.Fatal("Assert() error = nil, want mismatch error")
	}
	got, readErr := ioutil.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("replay rewrote golden: got %q, want %q", got, original)
	}
}

func TestRecordWritesNormalizedSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := GoldenStore{Mode: Record, Root: dir}

	err := store.Assert("nested/case.json", Snapshot{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"NoteId":"507f1f77bcf86cd799439011"}`),
	})
	if err != nil {
		t.Fatalf("Assert() error = %v", err)
	}
	got, readErr := ioutil.ReadFile(filepath.Join(dir, "nested", "case.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "{\"status\":200,\"headers\":{\"Content-Type\":\"application/json\"},\"body\":\"{\\\"NoteId\\\":\\\"OID_TOKEN\\\"}\"}\n" {
		t.Fatalf("recorded golden = %q", got)
	}
}

func TestGoldenCaseRecordThenReplayChecksRequestAndResponse(t *testing.T) {
	dir := t.TempDir()
	request := RequestSpec{
		Method: "POST",
		Path:   "/api/note/addNote",
		Form:   map[string][]string{"Title": {"Baseline note"}},
		Auth:   "admin",
	}
	actual := Snapshot{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"NoteId":"507f1f77bcf86cd799439011"}`),
	}

	if err := (GoldenStore{Mode: Record, Root: dir}).AssertCase("api/note_addNote.json", request, actual); err != nil {
		t.Fatalf("record AssertCase() error = %v", err)
	}
	if err := (GoldenStore{Mode: Replay, Root: dir}).AssertCase("api/note_addNote.json", request, actual); err != nil {
		t.Fatalf("replay AssertCase() error = %v", err)
	}

	request.Form["Title"] = []string{"Changed input"}
	err := (GoldenStore{Mode: Replay, Root: dir}).AssertCase("api/note_addNote.json", request, actual)
	if err == nil || !strings.Contains(err.Error(), "golden mismatch") {
		t.Fatalf("replay AssertCase() error = %v, want request mismatch", err)
	}
}

func TestGoldenStoreComparesBinaryPresenceInsteadOfBytes(t *testing.T) {
	dir := t.TempDir()
	request := RequestSpec{Method: http.MethodGet, Path: "/api/file/getAttach", Binary: true}
	store := GoldenStore{Mode: Record, Root: dir}
	first := Snapshot{
		Status:  http.StatusOK,
		Headers: map[string]string{"Content-Type": "application/octet-stream"},
		Body:    []byte("first binary payload"),
	}
	if err := store.AssertCase("api/file_getAttach.json", request, first); err != nil {
		t.Fatalf("record binary golden: %v", err)
	}
	second := first
	second.Body = []byte("second binary payload")
	if err := (GoldenStore{Mode: Replay, Root: dir}).AssertCase("api/file_getAttach.json", request, second); err != nil {
		t.Fatalf("replay binary golden: %v", err)
	}
	if err := store.AssertCase("api/file_getAttach-empty.json", request, Snapshot{Status: http.StatusOK, Headers: first.Headers}); err == nil {
		t.Fatal("empty binary payload was accepted")
	}
}

func TestGoldenStoreRejectsJSONBinaryResponse(t *testing.T) {
	err := (GoldenStore{Mode: Record, Root: t.TempDir()}).AssertCase("api/file_getAttach.json", RequestSpec{
		Method: http.MethodGet,
		Path:   "/api/file/getAttach",
		Binary: true,
	}, Snapshot{
		Status:  http.StatusOK,
		Headers: map[string]string{"Content-Type": "application/json; charset=utf-8"},
		Body:    []byte(`{"Ok":false,"Msg":"NOTLOGIN"}`),
	})
	if err == nil || !strings.Contains(err.Error(), "JSON response") {
		t.Fatalf("AssertCase() error = %v, want JSON binary rejection", err)
	}
}

func TestGoldenStoreAcceptsAlreadyNormalizedClientSnapshot(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Status:     http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"UserId":"OID_TOKEN"}`),
		normalized: true,
	}
	if err := (GoldenStore{Mode: Record, Root: dir}).AssertCase("api/user_info.json", RequestSpec{Method: http.MethodGet, Path: "/api/user/info"}, snapshot); err != nil {
		t.Fatalf("record normalized snapshot: %v", err)
	}
}

func TestGoldenStoreNormalizesOnlyObjectIDRequestFields(t *testing.T) {
	dir := t.TempDir()
	request := RequestSpec{
		Method: "POST",
		Path:   "/api/note/updateNote",
		Form: map[string][]string{
			"NoteId":  {"507f1f77bcf86cd799439011"},
			"Content": {"keep 507f1f77bcf86cd799439011"},
		},
	}
	actual := Snapshot{Status: http.StatusOK, Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(`{"Ok":true}`)}
	if err := (GoldenStore{Mode: Record, Root: dir}).AssertCase("api/note_updateNote.json", request, actual); err != nil {
		t.Fatalf("record request golden: %v", err)
	}
	recorded, err := ioutil.ReadFile(filepath.Join(dir, "api", "note_updateNote.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(recorded), `"NoteId":["OID_TOKEN"]`) || !strings.Contains(string(recorded), `"Content":["keep 507f1f77bcf86cd799439011"]`) {
		t.Fatalf("request normalization = %s", recorded)
	}
}
