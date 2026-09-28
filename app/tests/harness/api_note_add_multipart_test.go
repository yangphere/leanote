package harness

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNativeAPINoteAddMultipartReplay(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	noteID := db.NewObjectID().Hex()
	const (
		localID    = "local-attachment"
		title      = "Native multipart note"
		attachment = "multipart attachment payload\n"
	)
	content := `本文 <a href="https://client.example/api/file/getAttach?fileId=local-attachment">附件</a>`
	request := RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/note/addNote",
		Form: map[string][]string{
			"NoteId":                {noteID},
			"NotebookId":            {fixtureNotebookID},
			"Title":                 {title},
			"Content":               {content},
			"Files[0][LocalFileId]": {localID},
			"Files[0][Title]":       {"multipart.txt"},
			"Files[0][HasBody]":     {"true"},
			"Files[0][IsAttach]":    {"true"},
			"Files[0][Type]":        {"txt"},
		},
		Files: map[string]FilePart{
			"FileDatas[" + localID + "]": {Filename: "multipart.txt", ContentType: "text/plain", Body: []byte(attachment)},
		},
		Auth: "admin",
	}
	snapshot, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != http.StatusOK {
		t.Fatalf("addNote status=%d body=%s", snapshot.Status, snapshot.Body)
	}
	var created struct {
		NoteID     string          `json:"NoteId"`
		NotebookID string          `json:"NotebookId"`
		Title      string          `json:"Title"`
		Content    string          `json:"Content"`
		Files      []info.NoteFile `json:"Files"`
	}
	if err := json.Unmarshal(snapshot.Body, &created); err != nil {
		t.Fatalf("decode addNote response: %v; body=%s", err, snapshot.Body)
	}
	// Client.Do normalizes documented ObjectID response fields to keep replay
	// assertions stable across fixture-generated identifiers.
	if created.NoteID != "OID_TOKEN" || created.NotebookID != "OID_TOKEN" || created.Title != title {
		t.Fatalf("addNote response identity = %+v; body=%s", created, snapshot.Body)
	}
	if len(created.Files) != 1 || created.Files[0].FileId != "OID_TOKEN" {
		t.Fatalf("addNote response files = %+v", created.Files)
	}
	if created.Content != "" {
		t.Fatalf("addNote response content = %q, want response contract to omit content", created.Content)
	}

	database := fixtureDatabase(t)
	ctx, cancel := harnessContext()
	defer cancel()
	var stored info.NoteContent
	if err := database.Collection("note_contents").FindOne(ctx, bson.M{"_id": dbObjectID(t, noteID)}).Decode(&stored); err != nil {
		t.Fatalf("load stored note content: %v", err)
	}
	var storedNote info.Note
	if err := database.Collection("notes").FindOne(ctx, bson.M{"_id": dbObjectID(t, noteID)}).Decode(&storedNote); err != nil {
		t.Fatalf("load stored note: %v", err)
	}
	if storedNote.AttachNum != 1 {
		t.Fatalf("stored note AttachNum=%d, want 1", storedNote.AttachNum)
	}
	var storedAttachment info.Attach
	if err := database.Collection("attachs").FindOne(ctx, bson.M{
		"NoteId":       dbObjectID(t, noteID),
		"UploadUserId": dbObjectID(t, fixtureAdminID),
	}).Decode(&storedAttachment); err != nil {
		t.Fatalf("load stored attachment: %v", err)
	}
	serverLink := "/api/file/getAttach?fileId=" + storedAttachment.AttachId.Hex()
	if !strings.Contains(stored.Content, serverLink) || strings.Contains(stored.Content, localID) {
		t.Fatalf("stored content = %q, want server asset link %q", stored.Content, serverLink)
	}

	asset, err := client.Do(RequestSpec{
		Method: http.MethodGet,
		Path:   "/api/file/getAttach",
		Query:  map[string][]string{"fileId": {storedAttachment.AttachId.Hex()}},
		Auth:   "admin",
		Binary: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.Status != http.StatusOK || string(asset.Body) != attachment {
		t.Fatalf("downloaded attachment status=%d body=%q", asset.Status, asset.Body)
	}
}

func dbObjectID(t testing.TB, value string) interface{} {
	t.Helper()
	return db.MustObjectIDFromHex(value)
}
