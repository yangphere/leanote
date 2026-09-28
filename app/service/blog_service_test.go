package service

import (
	"errors"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

var legacyBlogTestTime = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

func TestCheckedBlogReadsRequireBlogContentProjection(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	for _, collection := range []*db.Collection{db.Notes, db.NoteContents} {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range []*db.Collection{db.Notes, db.NoteContents} {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean blog read collection: %v", err)
			}
		}
	})

	ownerID, noteID := db.NewObjectID(), db.NewObjectID()
	if err := db.Notes.Insert(info.Note{
		NoteId: noteID, UserId: ownerID, IsBlog: true,
		Title: "legacy blog", PublicTime: legacyBlogTestTime,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{
		NoteId: noteID, UserId: ownerID, IsBlog: true, Abstract: "legacy abstract", Content: "legacy body",
	}); err != nil {
		t.Fatal(err)
	}

	blogService := &BlogService{}
	item, err := blogService.GetBlogChecked(noteID.Hex())
	if err != nil {
		t.Fatalf("GetBlogChecked() error = %v", err)
	}
	if item.Content != "legacy body" || item.Abstract != "legacy abstract" {
		t.Fatalf("GetBlogChecked() item = %+v", item)
	}

	_, blogs, err := blogService.ListBlogsChecked(ownerID.Hex(), "", 1, 10, "PublicTime", false)
	if err != nil {
		t.Fatalf("ListBlogsChecked() error = %v", err)
	}
	if len(blogs) != 1 || blogs[0].Content != "legacy body" {
		t.Fatalf("ListBlogsChecked() blogs = %+v", blogs)
	}

	_, blogs, err = blogService.SearchBlogChecked("legacy body", ownerID.Hex(), 1, 10, "PublicTime", false)
	if err != nil {
		t.Fatalf("SearchBlogChecked() error = %v", err)
	}
	if len(blogs) != 1 || blogs[0].NoteId != noteID {
		t.Fatalf("SearchBlogChecked() blogs = %+v", blogs)
	}

	if _, err := db.NoteContents.RemoveAll(map[string]any{"_id": noteID}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{
		NoteId: noteID, UserId: ownerID, IsBlog: false, Content: "private body",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := blogService.GetBlogChecked(noteID.Hex()); !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("GetBlogChecked() explicit private projection error = %v, want ErrPublicBlogNotFound", err)
	}
	if _, err := db.NoteContents.RemoveAll(map[string]any{"_id": noteID}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{
		NoteId: noteID, UserId: ownerID, Content: "legacy body without flag",
	}); err != nil {
		t.Fatal(err)
	}
	if item, err := blogService.GetBlogChecked(noteID.Hex()); err != nil || item.Content != "legacy body without flag" {
		t.Fatalf("GetBlogChecked() missing IsBlog compatibility = %+v, %v", item, err)
	}

	if _, err := db.NoteContents.RemoveAll(map[string]any{"_id": noteID}); err != nil {
		t.Fatal(err)
	}
	if _, err := blogService.GetBlogChecked(noteID.Hex()); !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("GetBlogChecked() missing content error = %v, want ErrPublicBlogNotFound", err)
	}
}
