package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func useWorkspaceNoteLeaseTestCollection(t *testing.T) {
	t.Helper()
	_, raw := testCollection(t)
	collection := raw.Database().Collection("workspace_note_lease_test")
	if err := collection.Drop(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved := Notes
	Notes = wrapCollection(collection)
	t.Cleanup(func() { Notes = saved })
}

func TestWorkspaceNoteMutationLeaseFencesUnrelatedNoteWrites(t *testing.T) {
	useWorkspaceNoteLeaseTestCollection(t)
	owner := workspaceTestOwner(t)
	noteID := mustWorkspaceTestObjectID(t, "507f1f77bcf86cd799439012")
	if err := Notes.InsertContext(context.Background(), info.Note{
		NoteId: noteID, UserId: owner, Usn: 7, IsDeleted: false,
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Unix(100, 0)
	if err := AcquireWorkspaceNoteMutationLease(context.Background(), noteID, owner, 7, "operation-one", now); err != nil {
		t.Fatal(err)
	}

	blocked := bson.M{"_id": noteID, "UserId": owner, "Usn": 7, "IsDeleted": false}
	AddWorkspaceNoteMutationLeaseFilter(blocked, "", now)
	if err := Notes.UpdateOneMatchedContext(context.Background(), blocked, bson.M{"$set": bson.M{"Title": "blocked"}}); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("unrelated mutation err=%v, want lease fence", err)
	}

	owned := bson.M{"_id": noteID, "UserId": owner, "Usn": 7, "IsDeleted": false}
	AddWorkspaceNoteMutationLeaseFilter(owned, "operation-one", now)
	if err := Notes.UpdateOneMatchedContext(context.Background(), owned, bson.M{"$set": bson.M{"Title": "owned"}}); err != nil {
		t.Fatalf("owner mutation err=%v", err)
	}
	if err := ReleaseWorkspaceNoteMutationLease(context.Background(), noteID, owner, "operation-two"); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("foreign release err=%v, want lease ownership failure", err)
	}
	if err := ReleaseWorkspaceNoteMutationLease(context.Background(), noteID, owner, "operation-one"); err != nil {
		t.Fatalf("owner release err=%v", err)
	}
}

func TestWorkspaceNoteMutationLeaseAllowsWriteAfterExpiry(t *testing.T) {
	useWorkspaceNoteLeaseTestCollection(t)
	owner := workspaceTestOwner(t)
	noteID := mustWorkspaceTestObjectID(t, "507f1f77bcf86cd799439012")
	if err := Notes.InsertContext(context.Background(), info.Note{
		NoteId: noteID, UserId: owner, Usn: 7, IsDeleted: false,
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Unix(100, 0)
	if err := AcquireWorkspaceNoteMutationLease(context.Background(), noteID, owner, 7, "operation-one", now); err != nil {
		t.Fatal(err)
	}
	filter := bson.M{"_id": noteID, "UserId": owner, "Usn": 7, "IsDeleted": false}
	AddWorkspaceNoteMutationLeaseFilter(filter, "", now.Add(WorkspaceNoteMutationLeaseTTL+time.Nanosecond))
	if err := Notes.UpdateOneMatchedContext(context.Background(), filter, bson.M{"$set": bson.M{"Title": "after-expiry"}}); err != nil {
		t.Fatalf("expired lease did not become writable: %v", err)
	}
}

func mustWorkspaceTestObjectID(t *testing.T, value string) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
