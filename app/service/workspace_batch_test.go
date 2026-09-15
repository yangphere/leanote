package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestWorkspaceBatchIdentityBindsOwnerAndOrderedInput(t *testing.T) {
	owner := "507f1f77bcf86cd799439011"
	one, oneDigest, _, err := workspaceBatchOperationIdentity(owner, "copy", "batch-1", []string{"note-1", "note-2"}, "book-1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	two, twoDigest, _, err := workspaceBatchOperationIdentity(owner, "copy", "batch-1", []string{"note-2", "note-1"}, "book-1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if one != two || oneDigest == twoDigest {
		t.Fatal("batch identity must keep client scope while binding ordered input")
	}
	otherOwner, _, _, err := workspaceBatchOperationIdentity("507f1f77bcf86cd799439012", "copy", "batch-1", []string{"note-1", "note-2"}, "book-1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if one == otherOwner {
		t.Fatal("batch identity must bind owner")
	}
}

func TestRestoreMoveRetryStateFreezesOriginalGenerationAndPublicTime(t *testing.T) {
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	when := time.Date(2026, 9, 12, 3, 4, 5, 0, time.UTC)
	before, _ := json.Marshal(struct {
		Note struct {
			NoteID domain.ObjectID `json:"NoteId"`
			USN    int             `json:"Usn"`
		} `json:"Note"`
	}{Note: struct {
		NoteID domain.ObjectID `json:"NoteId"`
		USN    int             `json:"Usn"`
	}{noteID, 17}})
	desired, _ := json.Marshal(map[string]any{"Metadata": map[string]any{"PublicTime": when}})
	generation, publicTime, hasPublicTime, err := restoreMoveRetryState(applicationnotes.OperationReceipt{BeforeState: before, DesiredState: desired})
	if err != nil || generation != 17 || !hasPublicTime || !publicTime.Equal(when) {
		t.Fatalf("generation=%d public=%v present=%v err=%v", generation, publicTime, hasPublicTime, err)
	}
}

func TestReplayCommittedMoveUsesFrozenCommittedUSN(t *testing.T) {
	note := info.Note{Usn: 44}
	got := replayCommittedMoveNote(note, applicationnotes.OperationReceipt{AssignedUSN: 18})
	if got.Usn != 18 {
		t.Fatalf("replayed USN=%d", got.Usn)
	}
}

func TestTrashedNoteDeleteDecisionContinuesNewPermanentDelete(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	resource, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	replay, proceed := trashedNoteDeleteDecision(applicationnotes.OperationReceipt{}, mongo.ErrNoDocuments, resource)
	if replay || !proceed {
		t.Fatalf("replay=%v proceed=%v", replay, proceed)
	}
	committed := applicationnotes.OperationReceipt{OwnerID: owner, ResourceID: resource, Status: applicationnotes.OperationCommitted}
	replay, proceed = trashedNoteDeleteDecision(committed, nil, resource)
	if !replay || proceed {
		t.Fatalf("committed replay=%v proceed=%v", replay, proceed)
	}
	replay, proceed = trashedNoteDeleteDecision(applicationnotes.OperationReceipt{}, errors.New("storage unavailable"), resource)
	if replay || proceed {
		t.Fatalf("storage failure replay=%v proceed=%v", replay, proceed)
	}
}

func TestStableCopyNoteIDBindsDestinationOwner(t *testing.T) {
	one := stableCopyNoteIDForOwner("copy-operation", "source-note", "507f1f77bcf86cd799439011")
	two := stableCopyNoteIDForOwner("copy-operation", "source-note", "507f1f77bcf86cd799439012")
	if one.IsZero() || two.IsZero() || one == two {
		t.Fatal("copy destination identity must bind destination owner")
	}
}

func TestCopyOperationDigestBindsDestinationNotebook(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	destination := stableCopyNoteIDForOwner("copy-operation", "source-note", owner.Hex())
	_, first, err := copyNoteOperationIdentity("note_copy", owner, destination, "source-note", "book-1", "", "copy-operation")
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := copyNoteOperationIdentity("note_copy", owner, destination, "source-note", "book-2", "", "copy-operation")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("copy receipt digest must bind destination notebook")
	}
}

func TestTrashRetryUsesNoteSaveActionScope(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	first, err := applicationnotes.NewClientOperationIdentity("note_save", owner, "delete-1")
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := applicationnotes.NewClientOperationIdentity("note_delete_cleanup", owner, "delete-1")
	if err != nil {
		t.Fatal(err)
	}
	if first == cleanup {
		t.Fatal("trash retry and permanent cleanup must not share receipt scope")
	}
}

func TestWorkspaceBatchItemOperationIdentityIsStableAndDistinct(t *testing.T) {
	one := workspaceBatchItemOperationID("batch-1", "copy", "note-1", 0)
	retry := workspaceBatchItemOperationID("batch-1", "copy", "note-1", 0)
	two := workspaceBatchItemOperationID("batch-1", "copy", "note-2", 1)
	if one == "" || one != retry || one == two {
		t.Fatalf("item identities are not stable/distinct: one=%q retry=%q two=%q", one, retry, two)
	}
}

func TestFrozenWorkspaceBatchResultPreservesOriginalCopyResponseWithoutNoteContent(t *testing.T) {
	destination, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	frozen := frozenWorkspaceBatchResult{
		Command: WorkspaceCommandResult{Committed: true, RetrySafe: true, Items: []WorkspaceItemResult{{ResourceID: "source-1", USN: 12, Committed: true}}},
		Notes:   []info.Note{{NoteId: destination, Title: "original response", Usn: 12}},
	}
	payload, err := freezeWorkspaceBatchResult(frozen)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restoreWorkspaceBatchResult(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Command.OK() || got.Command.Items[0].USN != 12 || len(got.Notes) != 1 || got.Notes[0].NoteId != destination || got.Notes[0].Title != "original response" || got.Notes[0].Usn != 12 {
		t.Fatalf("restored=%+v", got)
	}
}
