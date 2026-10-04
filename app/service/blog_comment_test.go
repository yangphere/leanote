package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestValidCommentSubmissionIdRequiresLowercase128BitHex(t *testing.T) {
	tests := map[string]bool{
		"0123456789abcdef0123456789abcdef":   true,
		"0123456789ABCDEF0123456789abcdef":   false,
		"0123456789abcdef0123456789abcde":    false,
		"0123456789abcdef0123456789abcdef ":  false,
		"0123456789abcdef0123456789abcdef\n": false,
	}
	for submissionId, want := range tests {
		if got := validCommentSubmissionId(submissionId); got != want {
			t.Fatalf("validCommentSubmissionId(%q) = %t, want %t", submissionId, got, want)
		}
	}
}

func TestCommentContentDigestIsStableSHA256Hex(t *testing.T) {
	if got, want := commentContentDigest("hello"), "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"; got != want {
		t.Fatalf("commentContentDigest(hello) = %q, want %q", got, want)
	}
}

func TestValidCommentContentEnforcesUnicodeAndByteLimits(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "empty", text: "", want: false},
		{name: "unicode whitespace", text: "\u3000\t\n", want: false},
		{name: "valid", text: "你好", want: true},
		{name: "rune limit", text: strings.Repeat("界", 2001), want: false},
		{name: "byte limit", text: strings.Repeat("a", 8*1024+1), want: false},
		{name: "invalid utf8", text: string([]byte{0xff}), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validCommentContent(test.text); got != test.want {
				t.Fatalf("validCommentContent(%q) = %t, want %t", test.text, got, test.want)
			}
		})
	}
}

func TestValidateBlogCommentPaginationRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		wantErr  bool
	}{
		{name: "valid lower bound", page: 1, pageSize: 1, wantErr: false},
		{name: "valid upper bound", page: MaxBlogPage, pageSize: MaxBlogPageSize, wantErr: false},
		{name: "zero page", page: 0, pageSize: 15, wantErr: true},
		{name: "page overflow", page: MaxBlogPage + 1, pageSize: 15, wantErr: true},
		{name: "zero page size", page: 1, pageSize: 0, wantErr: true},
		{name: "page size overflow", page: 1, pageSize: MaxBlogPageSize + 1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateBlogCommentPagination(test.page, test.pageSize)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateBlogCommentPagination(%d, %d) error=%v, wantErr=%t", test.page, test.pageSize, err, test.wantErr)
			}
		})
	}
}

func TestBlogCommentBeforeStateUsesNoteOwnerInsteadOfActor(t *testing.T) {
	noteID := domain.ObjectID{1}
	ownerID := domain.ObjectID{2}
	actorID := domain.ObjectID{3}
	note := info.Note{NoteId: noteID, UserId: ownerID}

	if !blogCommentBeforeStateMatches(note, noteID, ownerID) {
		t.Fatal("comment before state rejected the note owner")
	}
	if blogCommentBeforeStateMatches(note, noteID, actorID) {
		t.Fatal("comment before state accepted the actor as the note owner")
	}
}

func TestBlogCommentReceiptReplayIgnoresAssignedCommentID(t *testing.T) {
	want := info.BlogCommentSubmissionReceipt{
		ActorId:       domain.ObjectID{1},
		SubmissionId:  "0123456789abcdef0123456789abcdef",
		NoteId:        domain.ObjectID{2},
		ContentSHA256: "digest",
	}
	got := want
	got.CommentId = domain.ObjectID{3}
	got.ToUserId = domain.ObjectID{4}
	if !blogCommentReceiptRequestMatches(got, want) {
		t.Fatal("replay intent should match without server-assigned comment and recipient IDs")
	}
	if blogCommentReceiptMatches(got, want) {
		t.Fatal("strict receipt matching must still require the assigned comment ID")
	}
}

func TestBlogCommentPendingReceiptRebuildsFullComment(t *testing.T) {
	receipt := info.BlogCommentSubmissionReceipt{
		ReceiptId:    domain.ObjectID{4},
		ActorId:      domain.ObjectID{1},
		SubmissionId: "0123456789abcdef0123456789abcdef",
		NoteId:       domain.ObjectID{2},
		CommentId:    domain.ObjectID{3},
		ToCommentId:  domain.ObjectID{5},
		ToUserId:     domain.ObjectID{6},
	}
	comment := blogCommentFromPendingReceipt(receipt, "hello")
	want := info.BlogComment{
		CommentId: comment.CommentId, NoteId: receipt.NoteId, UserId: receipt.ActorId,
		Content: "hello", ToCommentId: receipt.ToCommentId, ToUserId: receipt.ToUserId,
	}
	if !blogCommentsMatch(comment, want) {
		t.Fatalf("pending receipt reconstruction = %+v, want %+v", comment, want)
	}
}

func TestCommentConfirmedReplayAfterUnpublishReturnsOriginalResult(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	collections := []*db.Collection{db.Notes, db.NoteContents, db.UserBlogs, db.BlogComments, db.BlogCommentReceipts}
	for _, collection := range collections {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range collections {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean comment replay collection: %v", err)
			}
		}
	})

	ownerID, noteID := db.NewObjectID(), db.NewObjectID()
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: ownerID, IsBlog: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{NoteId: noteID, UserId: ownerID, IsBlog: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.UserBlogs.Insert(info.UserBlog{UserId: ownerID, CanComment: true}); err != nil {
		t.Fatal(err)
	}

	service := &BlogService{}
	submissionID := "0123456789abcdef0123456789abcdef"
	ok, original := service.Comment(noteID.Hex(), "", ownerID.Hex(), "first comment", submissionID)
	if !ok || original.CommentId.IsZero() {
		t.Fatalf("initial comment failed: ok=%t comment=%+v", ok, original)
	}
	if err := db.Notes.UpdateOneMatchedContext(context.Background(), map[string]any{"_id": noteID}, map[string]any{"$set": map[string]any{"IsBlog": false}}); err != nil {
		t.Fatal(err)
	}
	ok, replay := service.Comment(noteID.Hex(), "", ownerID.Hex(), "first comment", submissionID)
	if !ok || replay.CommentId != original.CommentId {
		t.Fatalf("confirmed replay after unpublish = ok=%t comment=%+v, want %+v", ok, replay, original)
	}
	if ok, _ := service.Comment(noteID.Hex(), "", ownerID.Hex(), "first comment", "fedcba9876543210fedcba9876543210"); ok {
		t.Fatal("new comment intent succeeded after unpublish")
	}
	var stored info.Note
	if err := db.Notes.FindId(noteID).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.CommentNum != 1 {
		t.Fatalf("replay changed comment count to %d", stored.CommentNum)
	}
}

func TestCommentNotificationTargetsReplyAuthorOnly(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	ownerID, replyID, actorID := db.NewObjectID(), db.NewObjectID(), db.NewObjectID()
	if err := db.Users.Insert(
		info.User{UserId: ownerID, Email: "owner@example.test"},
		info.User{UserId: replyID, Email: "reply@example.test"},
	); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		replyID     domain.ObjectID
		actorID     domain.ObjectID
		recipientID domain.ObjectID
	}{
		{name: "top-level comment", actorID: actorID, recipientID: ownerID},
		{name: "reply", replyID: replyID, actorID: actorID, recipientID: replyID},
		{name: "self-reply", replyID: replyID, actorID: replyID},
	} {
		t.Run(test.name, func(t *testing.T) {
			recipients, err := commentNotificationRecipientIDs(context.Background(), ownerID, test.replyID, test.actorID)
			if err != nil {
				t.Fatal(err)
			}
			if test.recipientID.IsZero() {
				if len(recipients) != 0 {
					t.Fatalf("unexpected recipients: %v", recipients)
				}
				return
			}
			if len(recipients) != 1 || recipients[0] != test.recipientID {
				t.Fatalf("recipients = %v, want %s", recipients, test.recipientID.Hex())
			}
		})
	}
}

func TestLikeCommentConcurrentActorsKeepEveryLike(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	collections := []*db.Collection{db.Notes, db.NoteContents, db.BlogComments}
	for _, collection := range collections {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range collections {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean comment like collection: %v", err)
			}
		}
	})

	ownerID, noteID, commentID := db.NewObjectID(), db.NewObjectID(), db.NewObjectID()
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: ownerID, IsBlog: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(info.NoteContent{NoteId: noteID, UserId: ownerID, IsBlog: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.BlogComments.Insert(info.BlogComment{CommentId: commentID, NoteId: noteID}); err != nil {
		t.Fatal(err)
	}

	const actors = 8
	start := make(chan struct{})
	results := make(chan bool, actors)
	var workers sync.WaitGroup
	for i := 0; i < actors; i++ {
		actorID := db.NewObjectID().Hex()
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			ok, liked, _ := (&BlogService{}).LikeComment(commentID.Hex(), actorID)
			results <- ok && liked
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for result := range results {
		if !result {
			t.Fatal("concurrent like failed")
		}
	}
	var stored info.BlogComment
	if err := db.BlogComments.FindId(commentID).One(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.LikeNum != actors || len(stored.LikeUserIds) != actors {
		t.Fatalf("concurrent likes left count=%d users=%d, want %d", stored.LikeNum, len(stored.LikeUserIds), actors)
	}
}
