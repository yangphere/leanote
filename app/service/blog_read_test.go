package service

import (
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
)

func TestPublicBlogNoteLookupResultPreservesStorageErrors(t *testing.T) {
	storageErr := errors.New("mongo unavailable")
	_, err := publicBlogNoteLookupResult(info.Note{}, storageErr)
	if !errors.Is(err, storageErr) {
		t.Fatalf("publicBlogNoteLookupResult error = %v, want storage error", err)
	}
	if errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("storage error was converted to not found: %v", err)
	}
}

func TestGetBlogCheckedRejectsInvalidNoteIDBeforeMongo(t *testing.T) {
	_, err := (&BlogService{}).GetBlogChecked("not-an-object-id")
	if !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("GetBlogChecked error = %v, want ErrPublicBlogNotFound", err)
	}
}

func TestGetBlogByIdAndUrlTitleCheckedRejectsInvalidOwnerBeforeMongo(t *testing.T) {
	_, err := (&BlogService{}).GetBlogByIdAndUrlTitleChecked("not-an-object-id", "507f1f77bcf86cd799439011")
	if !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("GetBlogByIdAndUrlTitleChecked error = %v, want ErrPublicBlogNotFound", err)
	}
}

func TestGetSingleCheckedRejectsInvalidIDBeforeMongo(t *testing.T) {
	_, err := (&BlogService{}).GetSingleChecked("not-an-object-id")
	if !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("GetSingleChecked error = %v, want ErrPublicBlogNotFound", err)
	}
}

func TestPreNextBlogCheckedRejectsInvalidOwnerBeforeMongo(t *testing.T) {
	_, _, err := (&BlogService{}).PreNextBlogChecked("not-an-object-id", "PublicTime", false, "507f1f77bcf86cd799439011", nil)
	if !errors.Is(err, ErrPublicBlogNotFound) {
		t.Fatalf("PreNextBlogChecked error = %v, want ErrPublicBlogNotFound", err)
	}
}

func TestMapUserAndBlogByUserIdsCheckedRejectsUninitializedCollections(t *testing.T) {
	oldUsers, oldUserBlogs := db.Users, db.UserBlogs
	db.Users = nil
	db.UserBlogs = nil
	t.Cleanup(func() {
		db.Users = oldUsers
		db.UserBlogs = oldUserBlogs
	})

	_, err := (&UserService{}).MapUserAndBlogByUserIdsChecked(nil)
	if !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("MapUserAndBlogByUserIdsChecked error = %v, want mongo initialization error", err)
	}
}
