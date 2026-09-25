package service

import (
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestCompleteSingleOrderRejectsUnknownDuplicateAndMissingIDs(t *testing.T) {
	singles := []map[string]string{
		{"SingleId": "one", "Title": "One"},
		{"SingleId": "two", "Title": "Two"},
	}

	tests := []struct {
		name string
		ids  []string
		want bool
	}{
		{name: "complete", ids: []string{"two", "one"}, want: true},
		{name: "unknown", ids: []string{"two", "missing"}, want: false},
		{name: "duplicate", ids: []string{"one", "one"}, want: false},
		{name: "missing", ids: []string{"one"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := completeSingleOrder(singles, test.ids); got != test.want {
				t.Fatalf("completeSingleOrder(%v) = %v, want %v", test.ids, got, test.want)
			}
		})
	}
}

func TestToBlogDoesNotLeavePublishedNoteWhenContentProjectionIsMissing(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	for _, collection := range []*db.Collection{db.Notes, db.NoteContents, db.TagCounts} {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range []*db.Collection{db.Notes, db.NoteContents, db.TagCounts} {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean blog publishing collection: %v", err)
			}
		}
	})

	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	noteID, err := domain.ParseObjectID("507f1f77bcf86cd799439012")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 0}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: owner, Usn: 0, IsTrash: false, IsDeleted: false}); err != nil {
		t.Fatal(err)
	}
	InitService()

	if (&NoteService{}).ToBlog(owner.Hex(), noteID.Hex(), true, false) {
		t.Fatal("publishing without a content projection must fail")
	}

	var stored info.Note
	if err := db.Notes.FindId(noteID).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.IsBlog {
		t.Fatalf("failed publishing left note public: %+v", stored)
	}
}

func TestNotebookBlogStateCycleCreatesNewReceiptsAndKeepsRetryStable(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	for _, collection := range []*db.Collection{db.Notes, db.NoteContents, db.TagCounts} {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range []*db.Collection{db.Notes, db.NoteContents, db.TagCounts} {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean blog publishing collection: %v", err)
			}
		}
	})

	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	notebookID, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 0}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: owner, Usn: 0}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: owner, NotebookId: notebookID, Usn: 0}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{NoteId: noteID, UserId: owner}); err != nil {
		t.Fatal(err)
	}
	service := &NotebookService{}
	for _, published := range []bool{true, false, true} {
		if !service.ToBlog(owner.Hex(), notebookID.Hex(), published) {
			t.Fatalf("notebook publish transition to %v failed", published)
		}
	}
	var notebook info.Notebook
	var note info.Note
	if err := db.Notebooks.FindId(notebookID).One(&notebook); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.FindId(noteID).One(&note); err != nil {
		t.Fatal(err)
	}
	if notebook.Usn <= 0 || note.Usn <= 0 || !notebook.IsBlog || !note.IsBlog {
		t.Fatalf("state cycle did not republish notebook and child: notebook=%+v note=%+v", notebook, note)
	}
	beforeRetry := notebook.Usn
	if !service.ToBlog(owner.Hex(), notebookID.Hex(), true) {
		t.Fatal("same-intent retry failed")
	}
	if err := db.Notebooks.FindId(notebookID).One(&notebook); err != nil || notebook.Usn != beforeRetry {
		t.Fatalf("retry repeated the notebook write: notebook=%+v err=%v", notebook, err)
	}
	for _, kind := range []string{"notebook_blog", "note_blog"} {
		count, err := db.WorkspaceOperations.Find(map[string]any{"OwnerId": owner, "Kind": kind}).Count()
		if err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("%s receipts=%d, want one per state transition", kind, count)
		}
	}
}
