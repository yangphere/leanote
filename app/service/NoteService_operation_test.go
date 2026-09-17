package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/revel/revel"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

type recordingContentAssetPort struct {
	copyCommand applicationnotes.CopyNoteAssetsCommand
	copyAssets  []applicationnotes.OperationAsset
}

func (*recordingContentAssetPort) ReconcileNote(context.Context, applicationnotes.ReconcileNoteAssetsCommand) error {
	return nil
}

func (*recordingContentAssetPort) VerifyReconcileNote(context.Context, applicationnotes.ReconcileNoteAssetsCommand) (bool, error) {
	return true, nil
}

func (port *recordingContentAssetPort) CopyNote(ctx context.Context, command applicationnotes.CopyNoteAssetsCommand) (applicationnotes.CopyNoteAssetsResult, error) {
	port.copyCommand = command
	receipt, err := db.GetWorkspaceOperation(ctx, command.DestinationOwnerID, command.OperationID)
	if err != nil {
		return applicationnotes.CopyNoteAssetsResult{}, err
	}
	port.copyAssets = append([]applicationnotes.OperationAsset(nil), receipt.Assets...)
	return applicationnotes.CopyNoteAssetsResult{Content: command.Content}, nil
}

func (*recordingContentAssetPort) VerifyCopyNote(context.Context, applicationnotes.CopyNoteAssetsCommand) (bool, error) {
	return true, nil
}

func (*recordingContentAssetPort) DeleteNote(context.Context, applicationnotes.DeleteNoteAssetsCommand) error {
	return nil
}

func (*recordingContentAssetPort) VerifyDeleteNote(context.Context, applicationnotes.DeleteNoteAssetsCommand) (bool, error) {
	return true, nil
}

func useNoteOperationTestDatabase(t *testing.T) {
	t.Helper()
	useNotebookReceiptTestDatabase(t)
	InitService()
	collections := []*db.Collection{
		db.Notes, db.NoteContents, db.NoteContentHistories, db.Tags, db.NoteTags,
		db.NoteImages, db.Files, db.Attachs, db.ShareNotes, db.ShareNotebooks,
	}
	for _, collection := range collections {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range collections {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean note operation collection: %v", err)
			}
		}
	})
}

func mustOperationTestID(t *testing.T, value string) domain.ObjectID {
	t.Helper()
	id, err := domain.ParseObjectID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func commitOperationReceipt(t *testing.T, receipt applicationnotes.OperationReceipt) {
	t.Helper()
	now := time.Now()
	receipt.Status = applicationnotes.OperationPending
	receipt.CreatedAt = now
	receipt.UpdatedAt = now
	store := db.MongoWorkspaceOperationStore{}
	created, err := store.Begin(context.Background(), receipt)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(context.Background(), receipt.OwnerID, created.OperationID, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claimed.Status = applicationnotes.OperationCommitted
	if _, err := store.Save(context.Background(), claimed); err != nil {
		t.Fatal(err)
	}
}

func beginPendingProjectionReceipt(t *testing.T, ownerID, resourceID domain.ObjectID, operationID, inputDigest string) {
	t.Helper()
	_, err := (db.MongoWorkspaceOperationStore{}).Begin(context.Background(), applicationnotes.OperationReceipt{
		OperationID: operationID + ":projections",
		OwnerID:     ownerID,
		ResourceID:  resourceID,
		Kind:        "note_create_projections",
		InputDigest: inputDigest,
		StepNames:   []string{"tags", "notebook_count", "image_index"},
		Status:      applicationnotes.OperationPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCopyReceiptResumeFailsClosedOnDigestMismatchAndTerminalFailure(t *testing.T) {
	digest := "expected"
	for _, test := range []struct {
		name    string
		receipt applicationnotes.OperationReceipt
		err     error
		want    bool
	}{
		{name: "committed", receipt: applicationnotes.OperationReceipt{InputDigest: digest, Status: applicationnotes.OperationCommitted}, want: true},
		{name: "repair pending", receipt: applicationnotes.OperationReceipt{InputDigest: digest, Status: applicationnotes.OperationRepairPending}, want: true},
		{name: "digest mismatch", receipt: applicationnotes.OperationReceipt{InputDigest: "different", Status: applicationnotes.OperationCommitted}},
		{name: "failed", receipt: applicationnotes.OperationReceipt{InputDigest: digest, Status: applicationnotes.OperationFailed}},
		{name: "compensated", receipt: applicationnotes.OperationReceipt{InputDigest: digest, Status: applicationnotes.OperationCompensated}},
		{name: "lookup error", receipt: applicationnotes.OperationReceipt{InputDigest: digest, Status: applicationnotes.OperationCommitted}, err: errors.New("storage")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := copyReceiptCanResume(test.receipt, test.err, digest); got != test.want {
				t.Fatalf("copyReceiptCanResume()=%v, want %v", got, test.want)
			}
		})
	}
}

func TestCopyNoteImagesWithManifestIgnoresImageOutsideFrozenSet(t *testing.T) {
	const laterImageID = "507f1f77bcf86cd799439009"
	content := `<img src="/file/outputImage?fileId=` + laterImageID + `">`
	got, err := (&NoteImageService{}).CopyNoteImagesWithManifest(
		"507f1f77bcf86cd799439001",
		"507f1f77bcf86cd799439002",
		"507f1f77bcf86cd799439003",
		content,
		"507f1f77bcf86cd799439004",
		"shared-copy-frozen-images",
		nil,
	)
	if err != nil || got != content {
		t.Fatalf("copy outside frozen image set=(%q, %v), want unchanged", got, err)
	}
}

func TestCopyNoteRetryResumesPendingCreationProjections(t *testing.T) {
	useNoteOperationTestDatabase(t)
	owner := mustOperationTestID(t, "507f1f77bcf86cd799439011")
	sourceID := mustOperationTestID(t, "507f1f77bcf86cd799439012")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439013")
	const clientOperationID = "copy-projection-retry"
	destinationID := stableCopyNoteIDForOwner(clientOperationID, sourceID.Hex(), owner.Hex())
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 9}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: owner}); err != nil {
		t.Fatal(err)
	}
	source := info.Note{NoteId: sourceID, UserId: owner, NotebookId: notebookID, Title: "source"}
	destination := info.Note{NoteId: destinationID, UserId: owner, NotebookId: notebookID, Title: "source", Tags: []string{"repair"}, Usn: 10}
	if err := db.Notes.Insert(source, destination); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(
		info.NoteContent{NoteId: sourceID, UserId: owner, Content: "body"},
		info.NoteContent{NoteId: destinationID, UserId: owner, Content: "body"},
	); err != nil {
		t.Fatal(err)
	}
	operationID, digest, err := copyNoteOperationIdentity("note_copy", owner, destinationID, sourceID.Hex(), notebookID.Hex(), "", clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	commitOperationReceipt(t, applicationnotes.OperationReceipt{OperationID: operationID, OwnerID: owner, ResourceID: destinationID, Kind: "note_create", InputDigest: digest, AssignedUSN: 10})
	beginPendingProjectionReceipt(t, owner, destinationID, operationID, digest)

	got := (&NoteService{}).CopyNoteWithOperation(sourceID.Hex(), notebookID.Hex(), owner.Hex(), clientOperationID)
	if got.NoteId != destinationID {
		t.Fatalf("retry result=%+v, want destination %s", got, destinationID.Hex())
	}
	projection, err := db.GetWorkspaceOperation(context.Background(), owner, operationID+":projections")
	if err != nil || projection.Status != applicationnotes.OperationCommitted {
		t.Fatalf("projection receipt=%+v err=%v, want committed repair", projection, err)
	}
}

func TestCopySharedNoteRetryResumesPendingCreationProjections(t *testing.T) {
	useNoteOperationTestDatabase(t)
	sourceOwner := mustOperationTestID(t, "507f1f77bcf86cd799439021")
	destinationOwner := mustOperationTestID(t, "507f1f77bcf86cd799439022")
	sourceID := mustOperationTestID(t, "507f1f77bcf86cd799439023")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439024")
	const clientOperationID = "shared-copy-projection-retry"
	destinationID := stableCopyNoteIDForOwner(clientOperationID, sourceID.Hex(), destinationOwner.Hex())
	if err := db.Users.Insert(info.User{UserId: sourceOwner}, info.User{UserId: destinationOwner, Usn: 14}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	source := info.Note{NoteId: sourceID, UserId: sourceOwner, NotebookId: notebookID, Title: "shared"}
	destination := info.Note{NoteId: destinationID, UserId: destinationOwner, NotebookId: notebookID, Title: "shared", Tags: []string{"repair"}, Usn: 15}
	if err := db.Notes.Insert(source, destination); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(
		info.NoteContent{NoteId: sourceID, UserId: sourceOwner, Content: "body"},
		info.NoteContent{NoteId: destinationID, UserId: destinationOwner, Content: "body"},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.ShareNotes.Insert(info.ShareNote{NoteId: sourceID, UserId: sourceOwner, ToUserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	operationID, digest, err := copyNoteOperationIdentity("note_shared_copy", destinationOwner, destinationID, sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	commitOperationReceipt(t, applicationnotes.OperationReceipt{OperationID: operationID, OwnerID: destinationOwner, ResourceID: destinationID, Kind: "note_create", InputDigest: digest, AssignedUSN: 15})
	beginPendingProjectionReceipt(t, destinationOwner, destinationID, operationID, digest)

	got := (&NoteService{}).CopySharedNoteWithOperation(sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), destinationOwner.Hex(), clientOperationID)
	if got.NoteId != destinationID {
		t.Fatalf("retry result=%+v, want destination %s", got, destinationID.Hex())
	}
	projection, err := db.GetWorkspaceOperation(context.Background(), destinationOwner, operationID+":projections")
	if err != nil || projection.Status != applicationnotes.OperationCommitted {
		t.Fatalf("projection receipt=%+v err=%v, want committed repair", projection, err)
	}
}

func TestCopySharedNoteWithoutOperationKeepsLegacySingleUSNCreate(t *testing.T) {
	useNoteOperationTestDatabase(t)
	sourceOwner := mustOperationTestID(t, "507f1f77bcf86cd799439041")
	destinationOwner := mustOperationTestID(t, "507f1f77bcf86cd799439042")
	sourceID := mustOperationTestID(t, "507f1f77bcf86cd799439043")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439044")
	missingImageID := "507f1f77bcf86cd799439045"
	sourceAttachID := mustOperationTestID(t, "507f1f77bcf86cd799439046")
	oldBasePath := revel.BasePath
	revel.BasePath = t.TempDir()
	t.Cleanup(func() { revel.BasePath = oldBasePath })
	if err := db.Users.Insert(info.User{UserId: sourceOwner}, info.User{UserId: destinationOwner, Usn: 20}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(info.Note{NoteId: sourceID, UserId: sourceOwner, NotebookId: notebookID, Title: "shared"}); err != nil {
		t.Fatal(err)
	}
	sourceContent := "<img src=\"/file/outputImage?fileId=" + missingImageID + "\">"
	if err := db.NoteContents.Insert(info.NoteContent{NoteId: sourceID, UserId: sourceOwner, Content: sourceContent}); err != nil {
		t.Fatal(err)
	}
	if err := db.ShareNotes.Insert(info.ShareNote{NoteId: sourceID, UserId: sourceOwner, ToUserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	sourceAttachPath := filepath.ToSlash(filepath.Join("files", sourceOwner.Hex(), "attachs", "legacy.txt"))
	absoluteAttachPath := filepath.Join(revel.BasePath, filepath.FromSlash(sourceAttachPath))
	if err := os.MkdirAll(filepath.Dir(absoluteAttachPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absoluteAttachPath, []byte("legacy attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Attachs.Insert(info.Attach{
		AttachId: sourceAttachID, NoteId: sourceID, UploadUserId: sourceOwner,
		Name: "legacy.txt", Title: "legacy.txt", Path: sourceAttachPath, Size: int64(len("legacy attachment")),
	}); err != nil {
		t.Fatal(err)
	}

	got := (&NoteService{}).CopySharedNoteWithOperation(sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), destinationOwner.Hex(), "")
	if got.NoteId.IsZero() || got.Usn != 21 {
		t.Fatalf("legacy shared copy=%+v, want one-USN committed destination", got)
	}
	var user info.User
	if err := db.Users.FindId(destinationOwner).One(&user); err != nil || user.Usn != 21 {
		t.Fatalf("destination user=%+v err=%v, want exactly one allocated USN", user, err)
	}
	content := (&NoteService{}).GetNoteContent(got.NoteId.Hex(), destinationOwner.Hex())
	if content.Content != sourceContent {
		t.Fatalf("legacy content=%q, want original %q when image copy fails", content.Content, sourceContent)
	}
	var copied []info.Attach
	if err := db.Attachs.Find(map[string]any{"NoteId": got.NoteId}).All(&copied); err != nil {
		t.Fatal(err)
	}
	if len(copied) != 1 || copied[0].Title != "legacy.txt" {
		t.Fatalf("legacy copied attachments=%+v, want source attachment", copied)
	}
}

func TestCopySharedNoteRetryUsesFrozenAttachmentManifest(t *testing.T) {
	useNoteOperationTestDatabase(t)
	sourceOwner := mustOperationTestID(t, "507f1f77bcf86cd799439061")
	destinationOwner := mustOperationTestID(t, "507f1f77bcf86cd799439062")
	sourceID := mustOperationTestID(t, "507f1f77bcf86cd799439063")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439065")
	frozenAttachID := mustOperationTestID(t, "507f1f77bcf86cd799439066")
	laterAttachID := mustOperationTestID(t, "507f1f77bcf86cd799439067")
	const clientOperationID = "shared-copy-frozen-assets"
	destinationID := stableCopyNoteIDForOwner(clientOperationID, sourceID.Hex(), destinationOwner.Hex())

	oldBasePath := revel.BasePath
	revel.BasePath = t.TempDir()
	t.Cleanup(func() { revel.BasePath = oldBasePath })
	previousStore := contentStore
	testStore := archiveStore{}
	contentStore = testStore
	t.Cleanup(func() {
		contentStore = previousStore
	})
	writeSourceAttach := func(id domain.ObjectID, name, body string) info.Attach {
		relativePath := filepath.ToSlash(filepath.Join("files", sourceOwner.Hex(), "attachs", name))
		path := filepath.Join(revel.BasePath, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return info.Attach{AttachId: id, NoteId: sourceID, UploadUserId: sourceOwner, Name: name, Title: name, Path: relativePath, Size: int64(len(body))}
	}

	if err := db.Users.Insert(info.User{UserId: sourceOwner}, info.User{UserId: destinationOwner, Usn: 15}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(
		info.Note{NoteId: sourceID, UserId: sourceOwner, NotebookId: notebookID, Title: "shared"},
		info.Note{NoteId: destinationID, UserId: destinationOwner, NotebookId: notebookID, Title: "shared", Usn: 15},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(
		info.NoteContent{NoteId: sourceID, UserId: sourceOwner, Content: "body"},
		info.NoteContent{NoteId: destinationID, UserId: destinationOwner, Content: "body"},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.ShareNotes.Insert(info.ShareNote{NoteId: sourceID, UserId: sourceOwner, ToUserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	frozenAttach := writeSourceAttach(frozenAttachID, "frozen.txt", "frozen")
	laterAttach := writeSourceAttach(laterAttachID, "later.txt", "later")
	if err := db.Attachs.Insert(frozenAttach, laterAttach); err != nil {
		t.Fatal(err)
	}

	operationID, digest, err := copyNoteOperationIdentity("note_shared_copy", destinationOwner, destinationID, sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	frozenDestination, err := copiedAttachmentDestination(frozenAttach, destinationOwner, destinationID, clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	frozenContentDigest := sha256.Sum256([]byte("frozen"))
	frozenRecordDigest := contentCreateAttachmentRecordDigest(frozenDestination)
	commitOperationReceipt(t, applicationnotes.OperationReceipt{
		OperationID: operationID,
		OwnerID:     destinationOwner,
		ResourceID:  destinationID,
		Kind:        "note_create",
		InputDigest: digest,
		AssignedUSN: 15,
		Assets: []applicationnotes.OperationAsset{{
			AssetID: frozenDestination.AttachId.Hex(), LocalFileID: frozenAttachID.Hex(),
			ContentSHA256: hex.EncodeToString(frozenContentDigest[:]), RecordSHA256: hex.EncodeToString(frozenRecordDigest[:]),
			Index: 0, IsAttach: true,
		}},
	})
	recorder := &recordingContentAssetPort{}
	previousAssets := ContentAssets
	ContentAssets = recorder
	t.Cleanup(func() { ContentAssets = previousAssets })

	got := (&NoteService{}).CopySharedNoteWithOperation(sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), destinationOwner.Hex(), clientOperationID)
	if got.NoteId != destinationID {
		t.Fatalf("shared-copy retry=%+v, want destination %s", got, destinationID.Hex())
	}
	if len(recorder.copyAssets) != 1 || recorder.copyAssets[0].AssetID != frozenDestination.AttachId.Hex() || recorder.copyAssets[0].LocalFileID != frozenAttachID.Hex() {
		t.Fatalf("copied assets=%+v, want only the frozen source attachment", recorder.copyAssets)
	}
}

func TestCopySharedNoteRetryUsesRootReceiptGeneration(t *testing.T) {
	useNoteOperationTestDatabase(t)
	sourceOwner := mustOperationTestID(t, "507f1f77bcf86cd799439071")
	destinationOwner := mustOperationTestID(t, "507f1f77bcf86cd799439072")
	sourceID := mustOperationTestID(t, "507f1f77bcf86cd799439073")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439074")
	const clientOperationID = "shared-copy-frozen-generation"
	destinationID := stableCopyNoteIDForOwner(clientOperationID, sourceID.Hex(), destinationOwner.Hex())

	if err := db.Users.Insert(info.User{UserId: sourceOwner}, info.User{UserId: destinationOwner, Usn: 16}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(
		info.Note{NoteId: sourceID, UserId: sourceOwner, NotebookId: notebookID, Title: "source"},
		info.Note{NoteId: destinationID, UserId: destinationOwner, NotebookId: notebookID, Title: "source", Usn: 16},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.NoteContents.Insert(
		info.NoteContent{NoteId: sourceID, UserId: sourceOwner, Content: "body"},
		info.NoteContent{NoteId: destinationID, UserId: destinationOwner, Content: "body"},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.ShareNotes.Insert(info.ShareNote{NoteId: sourceID, UserId: sourceOwner, ToUserId: destinationOwner}); err != nil {
		t.Fatal(err)
	}
	operationID, digest, err := copyNoteOperationIdentity("note_shared_copy", destinationOwner, destinationID, sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	commitOperationReceipt(t, applicationnotes.OperationReceipt{
		OperationID: operationID, OwnerID: destinationOwner, ResourceID: destinationID,
		Kind: "note_create", InputDigest: digest, AssignedUSN: 15,
	})

	recorder := &recordingContentAssetPort{}
	previous := ContentAssets
	ContentAssets = recorder
	t.Cleanup(func() { ContentAssets = previous })

	got := (&NoteService{}).CopySharedNoteWithOperation(sourceID.Hex(), notebookID.Hex(), sourceOwner.Hex(), destinationOwner.Hex(), clientOperationID)
	if got.NoteId != destinationID {
		t.Fatalf("shared-copy retry=%+v, want destination %s", got, destinationID.Hex())
	}
	if recorder.copyCommand.Generation != 15 {
		t.Fatalf("copy generation=%d, want root receipt generation 15 instead of current note generation 16", recorder.copyCommand.Generation)
	}
}

func TestSaveNoteClientNoOpPersistsReceiptAndConflictsOnChangedInput(t *testing.T) {
	useNoteOperationTestDatabase(t)
	owner := mustOperationTestID(t, "507f1f77bcf86cd799439031")
	noteID := mustOperationTestID(t, "507f1f77bcf86cd799439032")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439033")
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 7}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: owner}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: owner, NotebookId: notebookID, Title: "before", Usn: 7}); err != nil {
		t.Fatal(err)
	}
	const clientOperationID = "save-noop-retry"
	service := &NoteService{}
	first := service.SaveNote(SaveNoteCommand{ActorUserID: owner.Hex(), NoteID: noteID.Hex(), OperationID: clientOperationID})
	if !first.OK() || first.USN != 7 {
		t.Fatalf("first no-op=%+v", first)
	}
	operationID, err := applicationnotes.NewClientOperationIdentity("note_save", owner, clientOperationID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := db.GetWorkspaceOperation(context.Background(), owner, operationID)
	if err != nil || receipt.Status != applicationnotes.OperationCommitted || receipt.AssignedUSN != 7 {
		t.Fatalf("no-op receipt=%+v err=%v, want committed USN 7", receipt, err)
	}
	if err := db.Notes.UpdateOneMatchedContext(context.Background(), map[string]any{"_id": noteID, "UserId": owner}, map[string]any{"$set": map[string]any{"Usn": 8}}); err != nil {
		t.Fatal(err)
	}
	replay := service.SaveNote(SaveNoteCommand{ActorUserID: owner.Hex(), NoteID: noteID.Hex(), OperationID: clientOperationID})
	if !replay.OK() || replay.USN != 7 {
		t.Fatalf("no-op replay=%+v, want original committed USN 7", replay)
	}
	changed := service.SaveNote(SaveNoteCommand{
		ActorUserID: owner.Hex(), NoteID: noteID.Hex(), OperationID: clientOperationID,
		Metadata: map[string]any{"Title": "after"},
	})
	if changed.Error != WorkspaceConflict || changed.Committed {
		t.Fatalf("changed input replay=%+v, want conflict", changed)
	}
	var user info.User
	if err := db.Users.FindId(owner).One(&user); err != nil || user.Usn != 7 {
		t.Fatalf("no-op/conflict allocated a USN: user=%+v err=%v", user, err)
	}
}

func TestSaveNoteLegacyNoOpDoesNotCreateReceiptOrAllocateUSN(t *testing.T) {
	useNoteOperationTestDatabase(t)
	owner := mustOperationTestID(t, "507f1f77bcf86cd799439051")
	noteID := mustOperationTestID(t, "507f1f77bcf86cd799439052")
	notebookID := mustOperationTestID(t, "507f1f77bcf86cd799439053")
	if err := db.Users.Insert(info.User{UserId: owner, Usn: 11}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notebooks.Insert(info.Notebook{NotebookId: notebookID, UserId: owner}); err != nil {
		t.Fatal(err)
	}
	if err := db.Notes.Insert(info.Note{NoteId: noteID, UserId: owner, NotebookId: notebookID, Usn: 11}); err != nil {
		t.Fatal(err)
	}

	result := (&NoteService{}).SaveNote(SaveNoteCommand{ActorUserID: owner.Hex(), NoteID: noteID.Hex()})
	if !result.OK() || result.RetrySafe || result.USN != 11 {
		t.Fatalf("legacy no-op=%+v", result)
	}
	count, err := db.WorkspaceOperations.Find(map[string]any{}).Count()
	if err != nil || count != 0 {
		t.Fatalf("legacy no-op receipts=%d err=%v, want none", count, err)
	}
	var user info.User
	if err := db.Users.FindId(owner).One(&user); err != nil || user.Usn != 11 {
		t.Fatalf("legacy no-op allocated a USN: user=%+v err=%v", user, err)
	}
}
