package service

import (
	"fmt"
	"testing"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestAppendHistoryStoresBeforeImageNewestFirstAndCapsAtTen(t *testing.T) {
	current := info.NoteContentHistory{}
	for i := 0; i < 12; i++ {
		current.Histories = append(current.Histories, info.EachHistory{Content: fmt.Sprintf("old-%d", i)})
	}
	actor, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	entry := info.EachHistory{UpdatedUserId: actor, UpdatedTime: time.Unix(1, 0), Content: "before"}
	got := appendHistory(current, entry)
	if len(got.Histories) != 10 {
		t.Fatalf("len=%d want 10", len(got.Histories))
	}
	if got.Histories[0].Content != "before" || got.Histories[9].Content != "old-8" {
		t.Fatalf("histories=%+v", got.Histories)
	}
}

func TestWorkspaceCommandResultRejectsPartialBatch(t *testing.T) {
	result := WorkspaceCommandResult{Committed: true, Items: []WorkspaceItemResult{
		{ResourceID: "one", Committed: true},
		{ResourceID: "two", Error: WorkspaceStorage},
	}}
	if result.OK() {
		t.Fatal("partial batch reported success")
	}
}

func TestSaveNoteExposesUnavailableStorage(t *testing.T) {
	savedNotes := db.Notes
	db.Notes = nil
	t.Cleanup(func() { db.Notes = savedNotes })

	result := (&NoteService{}).SaveNote(SaveNoteCommand{
		ActorUserID: "507f1f77bcf86cd799439011",
		NoteID:      "507f1f77bcf86cd799439012",
		Metadata:    map[string]any{"Title": "new"},
	})
	if result.Error != WorkspaceStorage || result.Committed {
		t.Fatalf("result=%+v, want explicit storage failure", result)
	}
}

func TestMutationLookupsExposeUnavailableStorage(t *testing.T) {
	savedNotebooks := db.Notebooks
	savedNotes := db.Notes
	savedTags := db.NoteTags
	db.Notebooks = nil
	db.Notes = nil
	db.NoteTags = nil
	t.Cleanup(func() {
		db.Notebooks = savedNotebooks
		db.Notes = savedNotes
		db.NoteTags = savedTags
	})

	const owner = "507f1f77bcf86cd799439011"
	const resource = "507f1f77bcf86cd799439012"
	if ok, msg := (&NotebookService{}).DeleteNotebook(owner, resource); ok || msg != "storage" {
		t.Fatalf("notebook delete ok=%v msg=%q, want storage failure", ok, msg)
	}
	if ok, msg := (&NotebookService{}).DeleteNotebookForce(owner, resource, 1); ok || msg != "storage" {
		t.Fatalf("forced notebook delete ok=%v msg=%q, want storage failure", ok, msg)
	}
	if ok, msg, _ := (&TagService{}).DeleteTagApi(owner, "tag", 1); ok || msg != "storage" {
		t.Fatalf("tag delete ok=%v msg=%q, want storage failure", ok, msg)
	}
	if ok, msg, _ := (&TrashService{}).DeleteTrashApi(resource, owner, 1); ok || msg != "storage" {
		t.Fatalf("trash delete ok=%v msg=%q, want storage failure", ok, msg)
	}
}

func TestNoteCreateReceiptTransitionsStayInServiceAndExposeStorageFailure(t *testing.T) {
	savedOperations := db.WorkspaceOperations
	db.WorkspaceOperations = nil
	t.Cleanup(func() { db.WorkspaceOperations = savedOperations })

	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	service := &NoteService{}
	if got := service.BeginNoteCreateAssetReceipt(owner, noteID, "op", "digest", nil); got != WorkspaceStorage {
		t.Fatalf("begin category=%q, want storage", got)
	}
	if got := service.FailNoteCreateAssetReceipt(owner, "op"); got != WorkspaceStorage {
		t.Fatalf("fail category=%q, want storage", got)
	}
}

func TestFinalizeWorkspaceBatchDistinguishesTotalAndPartialFailure(t *testing.T) {
	total := WorkspaceCommandResult{Error: WorkspaceStorage, Items: []WorkspaceItemResult{{Error: WorkspaceStorage}}}
	finalizeWorkspaceBatch(&total)
	if total.PartialWrite || total.Error != WorkspaceStorage {
		t.Fatalf("total=%+v", total)
	}

	partial := WorkspaceCommandResult{Items: []WorkspaceItemResult{{Committed: true}, {Error: WorkspaceStorage}}}
	finalizeWorkspaceBatch(&partial)
	if !partial.PartialWrite || partial.Error != WorkspacePartialWrite {
		t.Fatalf("partial=%+v", partial)
	}
}

func TestSameNoteCreationRejectsReusedIdentityWithDifferentPayload(t *testing.T) {
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	noteID, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	notebookID, err := domain.ParseObjectID("507f1f77bcf86cd799439013")
	if err != nil {
		t.Fatal(err)
	}
	desired := info.Note{NoteId: noteID, UserId: owner, NotebookId: notebookID, Title: "title", Tags: []string{"one"}}
	desiredContent := info.NoteContent{NoteId: noteID, UserId: owner, Content: "body"}
	if !sameNoteCreation(desired, desiredContent, desired, desiredContent) {
		t.Fatal("identical creation payload was not recognized")
	}
	reused := desired
	reused.Tags = []string{"two"}
	if sameNoteCreation(desired, desiredContent, reused, desiredContent) {
		t.Fatal("reused note identity with different tags was accepted")
	}
}

func TestCreatedNoteRetryUsesPersistedUSN(t *testing.T) {
	request := info.Note{NoteId: domain.ObjectID{1}, UserId: domain.ObjectID{2}}
	persisted := request
	persisted.Usn = 42
	got := preferPersistedCreatedNote(request, persisted)
	if got.Usn != 42 {
		t.Fatalf("retry note usn=%d, want persisted 42", got.Usn)
	}
}

func TestWorkspaceBatchRejectsMalformedObjectIDs(t *testing.T) {
	service := NoteService{}
	owner := "507f1f77bcf86cd799439011"
	target := "507f1f77bcf86cd799439013"

	deleted := service.DeleteNotes(owner, []string{"invalid"}, false)
	if deleted.OK() || deleted.RetrySafe || deleted.Error != WorkspaceValidation || len(deleted.Items) != 1 || deleted.Items[0].Error != WorkspaceValidation {
		t.Fatalf("delete result=%+v", deleted)
	}
	moved := service.MoveNotes(owner, []string{"invalid"}, target)
	if moved.OK() || moved.RetrySafe || moved.Error != WorkspaceValidation || len(moved.Items) != 1 || moved.Items[0].Error != WorkspaceValidation {
		t.Fatalf("move result=%+v", moved)
	}
}

func TestCommittedNoteRetryRequiresProjectionReceiptWhenGenerationAdvanced(t *testing.T) {
	noteID, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	otherNotebook, err := domain.ParseObjectID("507f1f77bcf86cd799439013")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		command SaveNoteCommand
		changed bool
		want    bool
	}{
		{name: "metadata-only", command: SaveNoteCommand{Metadata: map[string]any{"Title": "new"}}},
		{name: "content", command: SaveNoteCommand{Content: stringPointer("new body")}, changed: true, want: true},
		{name: "asset", command: SaveNoteCommand{AssetWork: &applicationnotes.AssetMutation{}}, want: true},
		{name: "move", command: SaveNoteCommand{Metadata: map[string]any{"NotebookId": otherNotebook}}, want: true},
		{name: "tags", command: SaveNoteCommand{Metadata: map[string]any{"Tags": []string{"tag"}}}, want: true},
		{name: "trash-flag", command: SaveNoteCommand{Metadata: map[string]any{"IsTrash": false}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := noteSaveNeedsProjection(tc.command, info.Note{NoteId: noteID}, tc.changed); got != tc.want {
				t.Fatalf("needs projection=%v want %v", got, tc.want)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
