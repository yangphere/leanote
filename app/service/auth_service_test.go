package service

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestAuthServiceRegisterBuildsTransactionalInitializationAndOutbox(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var insertedUser info.User
	var notebooks []info.Notebook
	var userBlog info.UserBlog
	var single info.BlogSingle
	var issuedToken string
	service := AuthService{
		now:            func() time.Time { return now },
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		insertUser: func(_ context.Context, user info.User) error {
			insertedUser = user
			return nil
		},
		insertNotebook: func(_ context.Context, notebook info.Notebook) error {
			notebooks = append(notebooks, notebook)
			return nil
		},
		upsertUserBlog: func(_ context.Context, blog info.UserBlog) error {
			userBlog = blog
			return nil
		},
		insertBlogSingle: func(_ context.Context, page info.BlogSingle) error {
			single = page
			return nil
		},
		issueActionToken: func(_ context.Context, userID domain.ObjectID, email, value string, tokenType int, createdAt time.Time) (db.ActionToken, error) {
			if tokenType != info.TokenActiveEmail || email != "user.name-tag@example.com" || value == "" || !createdAt.Equal(now) {
				t.Fatalf("issue activation token args user=%s email=%q value=%q type=%d at=%s", userID.Hex(), email, value, tokenType, createdAt)
			}
			issuedToken = value
			return db.ActionToken{UserID: userID, Email: email, Token: value, Type: tokenType, CreatedTime: createdAt}, nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			if plan.UserID.IsZero() || plan.Outbox == nil {
				t.Fatalf("plan missing user id or outbox: %+v", plan)
			}
			if plan.Outbox.Kind != "activate-email" || plan.Outbox.IdempotencyKey != "register:"+plan.UserID.Hex() {
				t.Fatalf("outbox = %+v, want stable activate-email event", plan.Outbox)
			}
			plan.EnqueueOutbox = func(_ context.Context, event db.OutboxEvent) error {
				if event.Payload["token"] == "" || event.Payload["email"] != "user.name-tag@example.com" {
					t.Fatalf("outbox payload=%#v", event.Payload)
				}
				return nil
			}
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("User.Name-tag@example.com", "new-password", "")
	if !ok || msg != "" {
		t.Fatalf("Register = ok:%v msg:%q", ok, msg)
	}
	if insertedUser.Email != "user.name-tag@example.com" || insertedUser.Username != "user-name-tag" || insertedUser.UsernameRaw != insertedUser.Username {
		t.Fatalf("inserted user = %+v", insertedUser)
	}
	if !insertedUser.CreatedTime.Equal(now) {
		t.Fatalf("CreatedTime=%s, want %s", insertedUser.CreatedTime, now)
	}
	var titles []string
	for _, notebook := range notebooks {
		titles = append(titles, notebook.Title)
		if notebook.UserId != insertedUser.UserId || !notebook.CreatedTime.Equal(now) || !notebook.UpdatedTime.Equal(now) {
			t.Fatalf("notebook = %+v, want registered user and injected clock", notebook)
		}
	}
	if !reflect.DeepEqual(titles, []string{"life", "study", "work"}) {
		t.Fatalf("default notebook titles=%v", titles)
	}
	if userBlog.UserId != insertedUser.UserId || single.UserId != insertedUser.UserId || single.Title != "About Me" {
		t.Fatalf("blog=%+v single=%+v", userBlog, single)
	}
	if issuedToken == "" {
		t.Fatal("activation token was not issued")
	}
}

func TestAuthServiceRegisterDoesNotReportSuccessWhenInitializationFails(t *testing.T) {
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.UserInitializationResult{PartialWrite: true}, db.ErrPartialWrite
		},
	}

	ok, msg := service.Register("user@example.test", "new-password", "")
	if ok || msg != "partial_write" {
		t.Fatalf("Register = ok:%v msg:%q, want partial_write failure", ok, msg)
	}
}

func TestAuthServiceRegisterIncludesConfiguredSharesAndCopiedNotes(t *testing.T) {
	now := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439011")
	sharedNotebookID := db.MustObjectIDFromHex("507f1f77bcf86cd799439012")
	sharedNoteID := db.MustObjectIDFromHex("507f1f77bcf86cd799439013")
	copySourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439014")
	var lifeNotebookID domain.ObjectID
	var hasShareFrom, hasShareTo domain.ObjectID
	var sharedNotebooks []registrationShareEntry
	var sharedNotes []registrationShareEntry
	var copiedNotes []registrationNoteCopy
	service := AuthService{
		now:            func() time.Time { return now },
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		sharedNotebooks: func() []map[string]string {
			return []map[string]string{{"notebookId": sharedNotebookID.Hex(), "perm": "1"}}
		},
		sharedNotes: func() []map[string]string {
			return []map[string]string{{"noteId": sharedNoteID.Hex(), "perm": "0"}}
		},
		copyNoteIDs: func() []string {
			return []string{copySourceID.Hex()}
		},
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		insertNotebook: func(_ context.Context, notebook info.Notebook) error {
			if notebook.Title == "life" {
				lifeNotebookID = notebook.NotebookId
			}
			return nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		insertHasShareNote: func(_ context.Context, from, to domain.ObjectID) error {
			hasShareFrom = from
			hasShareTo = to
			return nil
		},
		insertShareNotebook: func(_ context.Context, share registrationShareEntry, from, to domain.ObjectID, at time.Time) error {
			if from != sharedUserID || to.IsZero() || !at.Equal(now) {
				t.Fatalf("share notebook owner/target/time = %s/%s/%s", from.Hex(), to.Hex(), at)
			}
			sharedNotebooks = append(sharedNotebooks, share)
			return nil
		},
		insertShareNote: func(_ context.Context, share registrationShareEntry, from, to domain.ObjectID, at time.Time) error {
			if from != sharedUserID || to.IsZero() || !at.Equal(now) {
				t.Fatalf("share note owner/target/time = %s/%s/%s", from.Hex(), to.Hex(), at)
			}
			sharedNotes = append(sharedNotes, share)
			return nil
		},
		copyRegistrationNote: func(_ context.Context, copy registrationNoteCopy, at time.Time) error {
			if !at.Equal(now) {
				t.Fatalf("copy time = %s, want %s", at, now)
			}
			copiedNotes = append(copiedNotes, copy)
			return nil
		},
		issueActionToken: func(_ context.Context, userID domain.ObjectID, email, value string, tokenType int, createdAt time.Time) (db.ActionToken, error) {
			return db.ActionToken{UserID: userID, Email: email, Token: value, Type: tokenType, CreatedTime: createdAt}, nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			plan.EnqueueOutbox = func(context.Context, db.OutboxEvent) error { return nil }
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if !ok || msg != "" {
		t.Fatalf("Register = ok:%v msg:%q", ok, msg)
	}
	if hasShareFrom != sharedUserID || hasShareTo.IsZero() {
		t.Fatalf("has share owner/target = %s/%s", hasShareFrom.Hex(), hasShareTo.Hex())
	}
	if len(sharedNotebooks) != 1 || sharedNotebooks[0].ID != sharedNotebookID || sharedNotebooks[0].Perm != 1 {
		t.Fatalf("shared notebooks = %+v", sharedNotebooks)
	}
	if len(sharedNotes) != 1 || sharedNotes[0].ID != sharedNoteID || sharedNotes[0].Perm != 0 {
		t.Fatalf("shared notes = %+v", sharedNotes)
	}
	if len(copiedNotes) != 1 {
		t.Fatalf("copied notes = %+v", copiedNotes)
	}
	copy := copiedNotes[0]
	if copy.SourceNoteID != copySourceID || copy.FromUserID != sharedUserID || copy.ToUserID.IsZero() || copy.TargetNoteID.IsZero() || copy.TargetNotebookID != lifeNotebookID {
		t.Fatalf("copy = %+v, life notebook=%s", copy, lifeNotebookID.Hex())
	}
}

func TestAuthServiceDefaultRegistrationCopyUsesMongoBoundaryInsteadOfBusinessCopy(t *testing.T) {
	savedNoteService := noteService
	savedNotes := db.Notes
	savedContents := db.NoteContents
	noteService = &NoteService{}
	db.Notes = nil
	db.NoteContents = nil
	defer func() {
		noteService = savedNoteService
		db.Notes = savedNotes
		db.NoteContents = savedContents
	}()

	copy := registrationNoteCopy{
		SourceNoteID:     db.MustObjectIDFromHex("507f1f77bcf86cd799439071"),
		TargetNoteID:     db.MustObjectIDFromHex("507f1f77bcf86cd799439072"),
		TargetNotebookID: db.MustObjectIDFromHex("507f1f77bcf86cd799439073"),
		FromUserID:       db.MustObjectIDFromHex("507f1f77bcf86cd799439074"),
		ToUserID:         db.MustObjectIDFromHex("507f1f77bcf86cd799439075"),
	}
	_, err := (&AuthService{}).saveRegistrationNoteCopy(context.Background(), copy, time.Now())
	if !errors.Is(err, db.ErrMongoClientNotInitialized) {
		t.Fatalf("saveRegistrationNoteCopy error = %v, want Mongo boundary initialization error", err)
	}
}

func TestRegistrationNoteDocumentsUsePreallocatedIdentityAndExcludeFileSideEffects(t *testing.T) {
	now := time.Date(2026, 9, 11, 14, 0, 0, 0, time.UTC)
	copy := registrationNoteCopy{
		TargetNoteID:     db.MustObjectIDFromHex("507f1f77bcf86cd799439072"),
		TargetNotebookID: db.MustObjectIDFromHex("507f1f77bcf86cd799439073"),
		ToUserID:         db.MustObjectIDFromHex("507f1f77bcf86cd799439075"),
	}
	source := info.Note{
		NoteId:      db.MustObjectIDFromHex("507f1f77bcf86cd799439071"),
		UserId:      db.MustObjectIDFromHex("507f1f77bcf86cd799439074"),
		ImgSrc:      "/files/source/image.png",
		AttachNum:   3,
		IsBlog:      true,
		IsTop:       true,
		IsRecommend: true,
		IsTrash:     true,
		IsDeleted:   true,
	}
	sourceContent := info.NoteContent{NoteId: source.NoteId, UserId: source.UserId, IsBlog: true, Content: "body"}

	note, content := registrationNoteDocuments(source, sourceContent, copy, now, 9)
	if note.NoteId != copy.TargetNoteID || content.NoteId != copy.TargetNoteID || note.UserId != copy.ToUserID || content.UserId != copy.ToUserID || note.NotebookId != copy.TargetNotebookID {
		t.Fatalf("copied identities note=%+v content=%+v", note, content)
	}
	if note.ImgSrc != "" || note.AttachNum != 0 || note.IsBlog || content.IsBlog || note.IsTop || note.IsRecommend || note.IsTrash || note.IsDeleted {
		t.Fatalf("registration copy retained external/blog state: note=%+v content=%+v", note, content)
	}
	if note.Usn != 9 || !note.CreatedTime.Equal(now) || !content.CreatedTime.Equal(now) || content.Content != "body" {
		t.Fatalf("registration copy metadata note=%+v content=%+v", note, content)
	}
}

func TestAuthServiceRegisterRejectsInvalidSharedConfigBeforeWriting(t *testing.T) {
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return "not-an-object-id" },
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			t.Fatal("initialization must not run when shared config is invalid")
			return db.UserInitializationResult{}, nil
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "configuration" {
		t.Fatalf("Register invalid shared config = ok:%v msg:%q, want configuration", ok, msg)
	}
}

func TestAuthServiceRegisterRejectsSharedConfigEntryMissingID(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439041")
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		sharedNotebooks: func() []map[string]string {
			return []map[string]string{{"perm": "1"}}
		},
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			t.Fatal("initialization must not run when a shared config entry omits its ID")
			return db.UserInitializationResult{}, nil
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "configuration" {
		t.Fatalf("Register shared entry without id = ok:%v msg:%q, want configuration", ok, msg)
	}
}

func TestAuthServiceRegisterAddsDefaultNotebooksThroughUSNSemantics(t *testing.T) {
	var usns []int
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		addRegistrationNotebook: func(_ context.Context, notebook info.Notebook) (info.Notebook, error) {
			if notebook.Usn != 0 {
				t.Fatalf("registration plan preassigned notebook USN=%d; AddNotebook semantics must own USN", notebook.Usn)
			}
			notebook.Usn = len(usns) + 1
			notebook.UrlTitle = notebook.Title
			usns = append(usns, notebook.Usn)
			return notebook, nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		issueActionToken: func(_ context.Context, userID domain.ObjectID, email, value string, tokenType int, createdAt time.Time) (db.ActionToken, error) {
			return db.ActionToken{UserID: userID, Email: email, Token: value, Type: tokenType, CreatedTime: createdAt}, nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			plan.EnqueueOutbox = func(context.Context, db.OutboxEvent) error { return nil }
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if !ok || msg != "" {
		t.Fatalf("Register = ok:%v msg:%q", ok, msg)
	}
	if !reflect.DeepEqual(usns, []int{1, 2, 3}) {
		t.Fatalf("default notebook USNs=%v, want legacy AddNotebook sequence", usns)
	}
}

func TestAuthServiceRegisterCleansDefaultNotebooksWhenNotebookStepFails(t *testing.T) {
	added := 0
	cleaned := false
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		removeUser: func(context.Context, domain.ObjectID) error {
			return nil
		},
		addRegistrationNotebook: func(_ context.Context, notebook info.Notebook) (info.Notebook, error) {
			added++
			if added == 1 {
				return notebook, nil
			}
			return info.Notebook{}, errors.New("notebook insert failed")
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			cleaned = true
			return nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "storage" {
		t.Fatalf("Register notebook step failure = ok:%v msg:%q, want storage", ok, msg)
	}
	if added != 2 || !cleaned {
		t.Fatalf("default notebook failure added=%d cleaned=%v, want cleanup after partial step write", added, cleaned)
	}
}

func TestAuthServiceRegisterCleansSharedResourcesWhenShareStepFails(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439021")
	sharedNotebookID := db.MustObjectIDFromHex("507f1f77bcf86cd799439022")
	cleaned := false
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		sharedNotebooks: func() []map[string]string {
			return []map[string]string{{"notebookId": sharedNotebookID.Hex(), "perm": "1"}}
		},
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		removeUser: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertNotebook: func(context.Context, info.Notebook) error {
			return nil
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			return nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		removeUserBlog: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		removeBlogSingles: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertHasShareNote: func(context.Context, domain.ObjectID, domain.ObjectID) error {
			return nil
		},
		insertShareNotebook: func(context.Context, registrationShareEntry, domain.ObjectID, domain.ObjectID, time.Time) error {
			return errors.New("share notebook insert failed")
		},
		removeShares: func(_ context.Context, from, to domain.ObjectID) error {
			if from != sharedUserID || to.IsZero() {
				t.Fatalf("removeShares owner/target = %s/%s", from.Hex(), to.Hex())
			}
			cleaned = true
			return nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "storage" {
		t.Fatalf("Register share step failure = ok:%v msg:%q, want storage", ok, msg)
	}
	if !cleaned {
		t.Fatal("shared_resources step did not clean writes from the failed step")
	}
}

func TestAuthServiceRegisterCleansCopiedNotesWhenCopyStepFails(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439031")
	firstCopyID := db.MustObjectIDFromHex("507f1f77bcf86cd799439032")
	secondCopyID := db.MustObjectIDFromHex("507f1f77bcf86cd799439033")
	copyAttempts := 0
	var cleaned []registrationNoteCopy
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		copyNoteIDs:    func() []string { return []string{firstCopyID.Hex(), secondCopyID.Hex()} },
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		removeUser: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertNotebook: func(context.Context, info.Notebook) error {
			return nil
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			return nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		removeUserBlog: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		removeBlogSingles: func(context.Context, domain.ObjectID) error {
			return nil
		},
		copyRegistrationNote: func(_ context.Context, copy registrationNoteCopy, _ time.Time) error {
			copyAttempts++
			if copyAttempts == 1 {
				if copy.SourceNoteID != firstCopyID || copy.TargetNoteID.IsZero() {
					t.Fatalf("first copy = %+v", copy)
				}
				return nil
			}
			if copy.SourceNoteID != secondCopyID {
				t.Fatalf("second copy = %+v", copy)
			}
			return errors.New("copy note failed")
		},
		removeCopiedNotes: func(_ context.Context, copies []registrationNoteCopy) error {
			cleaned = append([]registrationNoteCopy(nil), copies...)
			return nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "storage" {
		t.Fatalf("Register copy step failure = ok:%v msg:%q, want storage", ok, msg)
	}
	if copyAttempts != 2 {
		t.Fatalf("copy attempts=%d, want 2", copyAttempts)
	}
	if len(cleaned) != 1 || cleaned[0].SourceNoteID != firstCopyID {
		t.Fatalf("cleaned copies = %+v, want first successful copy only", cleaned)
	}
}

func TestAuthServiceCopyNotesStepPreservesCleanupFailure(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439071")
	firstSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439072")
	secondSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439073")
	copyErr := errors.New("copy note failed")
	cleanupErr := errors.New("remove copied note failed")
	copyAttempts := 0
	service := AuthService{
		sharedUserID: func() string { return sharedUserID.Hex() },
		copyNoteIDs:  func() []string { return []string{firstSourceID.Hex(), secondSourceID.Hex()} },
		copyRegistrationNote: func(_ context.Context, _ registrationNoteCopy, _ time.Time) error {
			copyAttempts++
			if copyAttempts == 1 {
				return nil
			}
			return copyErr
		},
		removeCopiedNotes: func(_ context.Context, copies []registrationNoteCopy) error {
			if len(copies) != 1 || copies[0].SourceNoteID != firstSourceID {
				t.Fatalf("cleanup copies = %+v, want first successful copy", copies)
			}
			return cleanupErr
		},
	}
	plan, err := service.registrationPlan(info.User{UserId: db.NewObjectID()}, "", time.Now())
	if err != nil {
		t.Fatalf("registrationPlan: %v", err)
	}
	var copyStep db.InitializationStep
	for _, step := range plan.Steps {
		if step.Name == "copy_notes" {
			copyStep = step
			break
		}
	}
	if copyStep.Apply == nil {
		t.Fatal("registration plan did not include copy_notes step")
	}

	err = copyStep.Apply(context.Background())
	if !errors.Is(err, copyErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("copy_notes error = %v, want copy and cleanup causes", err)
	}
}

func TestAuthServiceDefaultNotebooksStepPreservesCleanupFailure(t *testing.T) {
	writeErr := errors.New("insert notebook failed")
	cleanupErr := errors.New("remove notebooks failed")
	insertAttempts := 0
	service := AuthService{
		insertNotebook: func(context.Context, info.Notebook) error {
			insertAttempts++
			if insertAttempts == 1 {
				return nil
			}
			return writeErr
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			return cleanupErr
		},
	}
	plan, err := service.registrationPlan(info.User{UserId: db.NewObjectID()}, "", time.Now())
	if err != nil {
		t.Fatalf("registrationPlan: %v", err)
	}

	err = plan.Steps[1].Apply(context.Background())
	if !errors.Is(err, writeErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("default_notebooks error = %v, want write and cleanup causes", err)
	}
}

func TestAuthServiceSharedResourcesStepPreservesCleanupFailure(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439081")
	sharedNotebookID := db.MustObjectIDFromHex("507f1f77bcf86cd799439082")
	writeErr := errors.New("insert share notebook failed")
	cleanupErr := errors.New("remove shares failed")
	service := AuthService{
		sharedUserID:    func() string { return sharedUserID.Hex() },
		sharedNotebooks: func() []map[string]string { return []map[string]string{{"notebookId": sharedNotebookID.Hex()}} },
		insertHasShareNote: func(context.Context, domain.ObjectID, domain.ObjectID) error {
			return nil
		},
		insertShareNotebook: func(context.Context, registrationShareEntry, domain.ObjectID, domain.ObjectID, time.Time) error {
			return writeErr
		},
		removeShares: func(context.Context, domain.ObjectID, domain.ObjectID) error {
			return cleanupErr
		},
	}
	plan, err := service.registrationPlan(info.User{UserId: db.NewObjectID()}, "", time.Now())
	if err != nil {
		t.Fatalf("registrationPlan: %v", err)
	}
	var sharedStep db.InitializationStep
	for _, step := range plan.Steps {
		if step.Name == "shared_resources" {
			sharedStep = step
			break
		}
	}
	if sharedStep.Apply == nil {
		t.Fatal("registration plan did not include shared_resources step")
	}

	err = sharedStep.Apply(context.Background())
	if !errors.Is(err, writeErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("shared_resources error = %v, want write and cleanup causes", err)
	}
}

func TestAuthServiceRegisterCleansPreallocatedNoteIDWhenCopyFails(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439051")
	firstSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439052")
	secondSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439053")
	copyAttempts := 0
	var firstTargetID domain.ObjectID
	var cleaned []registrationNoteCopy
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		copyNoteIDs:    func() []string { return []string{firstSourceID.Hex(), secondSourceID.Hex()} },
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		removeUser: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertNotebook: func(context.Context, info.Notebook) error {
			return nil
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			return nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		removeUserBlog: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		removeBlogSingles: func(context.Context, domain.ObjectID) error {
			return nil
		},
		copyRegistrationNote: func(_ context.Context, copy registrationNoteCopy, _ time.Time) error {
			copyAttempts++
			if copyAttempts == 1 {
				firstTargetID = copy.TargetNoteID
				return nil
			}
			return errors.New("copy failed")
		},
		removeCopiedNotes: func(_ context.Context, copies []registrationNoteCopy) error {
			cleaned = append([]registrationNoteCopy(nil), copies...)
			return nil
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			runner := func(parent context.Context, apply func(context.Context) error) error {
				return apply(parent)
			}
			return db.ExecuteUserInitialization(ctx, plan, runner)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "storage" {
		t.Fatalf("Register copy step failure = ok:%v msg:%q, want storage", ok, msg)
	}
	if copyAttempts != 2 {
		t.Fatalf("copy attempts=%d, want 2", copyAttempts)
	}
	if firstTargetID.IsZero() || len(cleaned) != 1 || cleaned[0].SourceNoteID != firstSourceID || cleaned[0].TargetNoteID != firstTargetID {
		t.Fatalf("cleaned copies = %+v, want preallocated first copied note id %s", cleaned, firstTargetID.Hex())
	}
}

func TestAuthServiceRegisterCompensatesPreallocatedCopiedNoteIDsWhenLaterStepFails(t *testing.T) {
	sharedUserID := db.MustObjectIDFromHex("507f1f77bcf86cd799439061")
	firstSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439062")
	secondSourceID := db.MustObjectIDFromHex("507f1f77bcf86cd799439063")
	var copiedIDs []domain.ObjectID
	var cleaned []registrationNoteCopy
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		sharedUserID:   func() string { return sharedUserID.Hex() },
		copyNoteIDs:    func() []string { return []string{firstSourceID.Hex(), secondSourceID.Hex()} },
		insertUser: func(context.Context, info.User) error {
			return nil
		},
		removeUser: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertNotebook: func(context.Context, info.Notebook) error {
			return nil
		},
		removeNotebooks: func(context.Context, domain.ObjectID) error {
			return nil
		},
		upsertUserBlog: func(context.Context, info.UserBlog) error {
			return nil
		},
		removeUserBlog: func(context.Context, domain.ObjectID) error {
			return nil
		},
		insertBlogSingle: func(context.Context, info.BlogSingle) error {
			return nil
		},
		removeBlogSingles: func(context.Context, domain.ObjectID) error {
			return nil
		},
		copyRegistrationNote: func(_ context.Context, copy registrationNoteCopy, _ time.Time) error {
			copiedIDs = append(copiedIDs, copy.TargetNoteID)
			switch copy.SourceNoteID {
			case firstSourceID:
				return nil
			case secondSourceID:
				return nil
			default:
				t.Fatalf("unexpected copy source: %+v", copy)
				return nil
			}
		},
		removeCopiedNotes: func(_ context.Context, copies []registrationNoteCopy) error {
			cleaned = append([]registrationNoteCopy(nil), copies...)
			return nil
		},
		issueActionToken: func(context.Context, domain.ObjectID, string, string, int, time.Time) (db.ActionToken, error) {
			return db.ActionToken{}, errors.New("activation token write failed")
		},
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.ExecuteUserInitialization(ctx, plan, nil)
		},
	}

	ok, msg := service.Register("user@example.com", "new-password", "")
	if ok || msg != "partial_write" {
		t.Fatalf("Register later failure = ok:%v msg:%q, want partial_write", ok, msg)
	}
	if len(copiedIDs) != 2 || copiedIDs[0].IsZero() || copiedIDs[1].IsZero() || len(cleaned) != 2 || cleaned[0].TargetNoteID != copiedIDs[0] || cleaned[1].TargetNoteID != copiedIDs[1] {
		t.Fatalf("compensated copies = %+v, want preallocated copied note IDs %+v", cleaned, copiedIDs)
	}
}

func TestAuthServiceRegisterFailsOnOutboxSideEffect(t *testing.T) {
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.UserInitializationResult{PartialWrite: true, FailedStep: "outbox"}, db.ErrSideEffect
		},
	}

	ok, msg := service.Register("user@example.test", "new-password", "")
	if ok || msg != "side_effect" {
		t.Fatalf("Register outbox failure = ok:%v msg:%q, want side_effect", ok, msg)
	}
}

func TestAuthServiceRegisterDoesNotUseInvalidGeneratedUsernames(t *testing.T) {
	taken := map[string]bool{"a-b-c": true}
	service := AuthService{
		userExists: func(string) bool { return false },
		usernameExists: func(username string) bool {
			return taken[username]
		},
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.UserInitializationResult{Committed: true}, nil
		},
	}

	username := service.registrationUsername("A+B.C@example.test", domain.ObjectID{1})
	if username != "a-b-c-1" {
		t.Fatalf("username=%q, want sanitized collision suffix", username)
	}
	short := service.registrationUsername("a@example.test", domain.ObjectID{2})
	if len(short) < 4 || short == "a" {
		t.Fatalf("short username=%q, want deterministic valid fallback", short)
	}
}

func TestAuthServiceRegisterValidatesEmailAndPasswordInsideService(t *testing.T) {
	service := AuthService{
		userExists: func(string) bool {
			t.Fatal("duplicate lookup must not run after invalid input")
			return false
		},
	}
	if ok, msg := service.Register("not-email", "new-password", ""); ok || msg != "errorEmail" {
		t.Fatalf("invalid email Register = ok:%v msg:%q", ok, msg)
	}
	if ok, msg := service.Register("user@example.test", "short", ""); ok || msg != "errorPassword" {
		t.Fatalf("invalid password Register = ok:%v msg:%q", ok, msg)
	}
}

func TestAuthServiceRegisterMapsStorageFailure(t *testing.T) {
	service := AuthService{
		userExists:     func(string) bool { return false },
		usernameExists: func(string) bool { return false },
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.UserInitializationResult{}, errors.New("database unavailable")
		},
	}

	ok, msg := service.Register("user@example.test", "new-password", "")
	if ok || msg != "storage" {
		t.Fatalf("Register storage failure = ok:%v msg:%q", ok, msg)
	}
}

func TestAuthServiceLoginSurfacesStorageLookupError(t *testing.T) {
	storageErr := errors.New("database unavailable")
	service := AuthService{
		findUserByName: func(name string) (info.User, error) {
			if name != "user@example.test" {
				t.Fatalf("lookup name=%q, want normalized login", name)
			}
			return info.User{}, storageErr
		},
	}

	_, err := service.Login(" User@Example.Test ", "password")
	if !errors.Is(err, storageErr) {
		t.Fatalf("Login error=%v, want wrapped storage error", err)
	}
	if err != nil && err.Error() == "wrong username or password" {
		t.Fatalf("Login mapped storage error to invalid credentials: %v", err)
	}
}

func TestAuthServiceLoginMapsMissingUserToInvalidCredentials(t *testing.T) {
	service := AuthService{
		findUserByName: func(string) (info.User, error) {
			return info.User{}, nil
		},
	}

	_, err := service.Login("missing@example.test", "password")
	if err == nil || err.Error() != "invalid_credentials" {
		t.Fatalf("Login missing user error=%v, want invalid_credentials", err)
	}
}

func TestAuthServiceThirdRegisterUsesCompositeThirdIdentity(t *testing.T) {
	existing := info.User{UserId: domain.ObjectID{1}, ThirdType: info.ThirdQQ, ThirdUserId: "same-third-id"}
	service := AuthService{
		findUserByThirdIdentity: func(thirdType int, thirdUserID string) (info.User, error) {
			if thirdType != info.ThirdQQ || thirdUserID != "same-third-id" {
				t.Fatalf("third identity lookup type=%d id=%q, want QQ/same-third-id", thirdType, thirdUserID)
			}
			return existing, nil
		},
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			t.Fatal("existing third identity must not register a new user")
			return db.UserInitializationResult{}, nil
		},
	}

	exists, got := service.ThirdRegister(strconv.Itoa(info.ThirdQQ), "same-third-id", "remote-name")
	if !exists || got.UserId != existing.UserId {
		t.Fatalf("ThirdRegister existing = exists:%v user:%+v, want existing composite identity", exists, got)
	}
}

func TestAuthServiceThirdRegisterStoresThirdType(t *testing.T) {
	var inserted info.User
	service := AuthService{
		findUserByThirdIdentity: func(thirdType int, thirdUserID string) (info.User, error) {
			if thirdType != info.ThirdQQ || thirdUserID != "same-third-id" {
				t.Fatalf("third identity lookup type=%d id=%q, want QQ/same-third-id", thirdType, thirdUserID)
			}
			return info.User{}, nil
		},
		usernameExists: func(string) bool { return false },
		insertUser: func(_ context.Context, user info.User) error {
			inserted = user
			return nil
		},
		insertNotebook:   func(context.Context, info.Notebook) error { return nil },
		upsertUserBlog:   func(context.Context, info.UserBlog) error { return nil },
		insertBlogSingle: func(context.Context, info.BlogSingle) error { return nil },
		runUserInitialization: func(ctx context.Context, plan db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.ExecuteUserInitialization(ctx, plan, nil)
		},
	}

	exists, got := service.ThirdRegister(strconv.Itoa(info.ThirdQQ), "same-third-id", "remote-name")
	if exists || got.UserId.IsZero() {
		t.Fatalf("ThirdRegister new user = exists:%v user:%+v, want persisted new user", exists, got)
	}
	if inserted.ThirdType != info.ThirdQQ || inserted.ThirdUserId != "same-third-id" || inserted.ThirdUsername != "remote-name" {
		t.Fatalf("inserted third user = %+v, want composite identity fields", inserted)
	}
}

func TestAuthServiceThirdRegisterReturnsZeroUserWhenRegisterFails(t *testing.T) {
	service := AuthService{
		findUserByThirdIdentity: func(int, string) (info.User, error) {
			return info.User{}, nil
		},
		usernameExists: func(string) bool { return false },
		runUserInitialization: func(context.Context, db.UserInitializationPlan) (db.UserInitializationResult, error) {
			return db.UserInitializationResult{PartialWrite: true}, db.ErrPartialWrite
		},
	}

	exists, got := service.ThirdRegister(strconv.Itoa(info.ThirdGithub), "new-third-id", "remote-name")
	if exists || !got.UserId.IsZero() {
		t.Fatalf("ThirdRegister failed create = exists:%v user:%+v, want zero user and no success", exists, got)
	}
}
