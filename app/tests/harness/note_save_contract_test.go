package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestUpdateNoteOrContentPropagatesUpdateFailureEnvelope(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	loginWebSession(t, client)

	snapshot, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"NoteId": {"000000000000000000000001"},
			"Title":  {"missing note update"},
		},
	})
	if err != nil {
		t.Fatalf("update missing note: %v", err)
	}
	if snapshot.Status != http.StatusOK {
		t.Fatalf("update missing note status = %d, want %d", snapshot.Status, http.StatusOK)
	}

	var response struct {
		Ok  bool   `json:"Ok"`
		Msg string `json:"Msg"`
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode update failure envelope: %v; body=%s", err, snapshot.Body)
	}
	if response.Ok || response.Msg != "notExists" {
		t.Fatalf("update failure envelope = %s, want Ok=false and Msg=notExists", snapshot.Body)
	}
}

func TestUpdateNoteOrContentNewSuccessUsesItemEnvelope(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	loginWebSession(t, client)
	noteID := db.NewObjectID().Hex()
	t.Cleanup(func() {
		database := fixtureDatabase(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = database.Collection("notes").DeleteOne(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteID)})
		_, _ = database.Collection("note_contents").DeleteOne(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteID)})
	})

	status, _, body, err := client.doRaw(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"IsNew":      {"true"},
			"NoteId":     {noteID},
			"NotebookId": {fixtureNotebookID},
			"Title":      {"structured new note"},
			"Content":    {"<p>structured content</p>"},
		},
	})
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("create note status = %d, want %d", status, http.StatusOK)
	}

	var response struct {
		Ok   bool   `json:"Ok"`
		Msg  string `json:"Msg"`
		Item *struct {
			NoteId string `json:"NoteId"`
		} `json:"Item"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode create envelope: %v; body=%s", err, body)
	}
	if !response.Ok || response.Msg != "" {
		t.Fatalf("create envelope = %s, want Ok=true and empty Msg", body)
	}
	if response.Item == nil || response.Item.NoteId != noteID {
		t.Fatalf("create envelope Item = %s, want note %s", body, noteID)
	}
}

func TestUpdateNoteOrContentReportsPartialWriteAsFailure(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	loginWebSession(t, client)
	noteID := db.NewObjectID().Hex()
	t.Cleanup(func() {
		database := fixtureDatabase(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = database.Collection("notes").DeleteOne(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteID)})
		_, _ = database.Collection("note_contents").DeleteOne(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteID)})
	})

	created, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"IsNew":      {"true"},
			"NoteId":     {noteID},
			"NotebookId": {fixtureNotebookID},
			"Title":      {"partial write note"},
			"Content":    {"<p>initial</p>"},
		},
	})
	if err != nil {
		t.Fatalf("create note for partial-write test: %v", err)
	}
	if created.Status != http.StatusOK {
		t.Fatalf("create note status = %d, want %d", created.Status, http.StatusOK)
	}

	database := fixtureDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = database.Collection("note_contents").DeleteOne(ctx, bson.M{"_id": db.MustObjectIDFromHex(noteID)})
	cancel()
	if err != nil {
		t.Fatalf("delete note content to force partial write: %v", err)
	}

	snapshot, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"NoteId":  {noteID},
			"Title":   {"metadata was written"},
			"Content": {"<p>content fails</p>"},
		},
	})
	if err != nil {
		t.Fatalf("update note with missing content record: %v", err)
	}
	var response struct {
		Ok  bool   `json:"Ok"`
		Msg string `json:"Msg"`
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode partial-write envelope: %v; body=%s", err, snapshot.Body)
	}
	if response.Ok || response.Msg == "" {
		t.Fatalf("partial-write envelope = %s, want Ok=false and non-empty Msg", snapshot.Body)
	}
}

func TestUpdateNoteOrContentNewPermissionFailureUsesEnvelope(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	loginWebSession(t, client)

	snapshot, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"IsNew":      {"true"},
			"NoteId":     {db.NewObjectID().Hex()},
			"NotebookId": {fixtureNotebookID},
			"FromUserId": {"000000000000000000000001"},
			"Title":      {"unauthorized shared note"},
			"Content":    {"<p>must not be inserted</p>"},
		},
	})
	if err != nil {
		t.Fatalf("create unauthorized shared note: %v", err)
	}
	var response struct {
		Ok  bool   `json:"Ok"`
		Msg string `json:"Msg"`
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode permission failure envelope: %v; body=%s", err, snapshot.Body)
	}
	if response.Ok || response.Msg != "noAuth" {
		t.Fatalf("permission failure envelope = %s, want Ok=false and Msg=noAuth", snapshot.Body)
	}
}

func TestUpdateNoteOrContentNewInsertFailureUsesEnvelope(t *testing.T) {
	_, client, _ := startBaselineServer(t)
	loginWebSession(t, client)

	snapshot, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/note/updateNoteOrContent",
		Form: map[string][]string{
			"IsNew":      {"true"},
			"NoteId":     {fixtureActiveNoteID},
			"NotebookId": {fixtureNotebookID},
			"Title":      {"duplicate note id"},
			"Content":    {"<p>must not replace existing note</p>"},
		},
	})
	if err != nil {
		t.Fatalf("create duplicate note: %v", err)
	}
	var response struct {
		Ok  bool   `json:"Ok"`
		Msg string `json:"Msg"`
	}
	if err := json.Unmarshal(snapshot.Body, &response); err != nil {
		t.Fatalf("decode insert failure envelope: %v; body=%s", err, snapshot.Body)
	}
	if response.Ok || response.Msg != "noteInsertFailed" {
		t.Fatalf("insert failure envelope = %s, want Ok=false and Msg=noteInsertFailed", snapshot.Body)
	}
}
