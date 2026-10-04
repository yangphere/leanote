package content

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

type fakeAttachmentRepository struct {
	note       AttachmentNote
	attachment AttachmentMetadata
	list       []AttachmentMetadata
	err        error
}

func (repository fakeAttachmentRepository) LoadNote(context.Context, domain.ObjectID) (AttachmentNote, error) {
	return repository.note, repository.err
}

func (repository fakeAttachmentRepository) LoadAttachment(context.Context, domain.ObjectID) (AttachmentMetadata, error) {
	return repository.attachment, repository.err
}

func (repository fakeAttachmentRepository) ListAttachments(context.Context, domain.ObjectID) ([]AttachmentMetadata, error) {
	return repository.list, repository.err
}

type fakeAttachmentPermission struct {
	allowed bool
	err     error
}

func (permission fakeAttachmentPermission) CanReadNote(context.Context, domain.ObjectID, domain.ObjectID, domain.ObjectID) (bool, error) {
	return permission.allowed, permission.err
}

type fakeAttachmentStore struct {
	data      []byte
	err       error
	nilReader bool
}

func (store fakeAttachmentStore) Open(context.Context, LogicalPath) (OpenResult, error) {
	if store.err != nil {
		return OpenResult{}, store.err
	}
	if store.nilReader {
		return OpenResult{Size: int64(len(store.data))}, nil
	}
	return OpenResult{Reader: io.NopCloser(bytes.NewReader(store.data)), Size: int64(len(store.data))}, nil
}

func (fakeAttachmentStore) Publish(context.Context, PublishRequest) (PublishResult, error) {
	panic("unexpected Publish")
}

func (fakeAttachmentStore) Verify(context.Context, VerifyRequest) (VerifyResult, error) {
	panic("unexpected Verify")
}

func attachmentIDs(t *testing.T) (domain.ObjectID, domain.ObjectID, domain.ObjectID) {
	t.Helper()
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	note, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	return actor, owner, note
}

func TestAttachmentAccessListAllowsOwnerPublicAndSharedReaders(t *testing.T) {
	actor, owner, noteID := attachmentIDs(t)
	attachmentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	metadata := AttachmentMetadata{AttachmentID: attachmentID, NoteID: noteID, DisplayName: "report.txt", StoredPath: "files/owner/report.txt", Size: 3}
	for _, test := range []struct {
		name       string
		actor      domain.ObjectID
		public     bool
		permission bool
	}{
		{name: "owner", actor: owner},
		{name: "public", public: true},
		{name: "shared", actor: actor, permission: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := AttachmentAccessService{
				Repository:  fakeAttachmentRepository{note: AttachmentNote{NoteID: noteID, OwnerID: owner, Public: test.public}, list: []AttachmentMetadata{metadata}},
				Permissions: fakeAttachmentPermission{allowed: test.permission},
			}
			result, err := service.List(context.Background(), test.actor, noteID)
			if err != nil || len(result.Attachments) != 1 || result.Note.OwnerID != owner {
				t.Fatalf("List() = %#v, %v", result, err)
			}
		})
	}
}

func TestAttachmentAccessListDenialAndDependencyErrorsStayDistinct(t *testing.T) {
	actor, owner, noteID := attachmentIDs(t)
	service := AttachmentAccessService{
		Repository:  fakeAttachmentRepository{note: AttachmentNote{NoteID: noteID, OwnerID: owner}},
		Permissions: fakeAttachmentPermission{},
	}
	if _, err := service.List(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorUnauthorized {
		t.Fatalf("denied List() error = %v", err)
	}
	service.Repository = fakeAttachmentRepository{err: errors.New("database unavailable")}
	if _, err := service.List(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("dependency List() error = %v", err)
	}
	if _, err := service.List(context.Background(), actor, domain.ObjectID{}); errorCategoryOf(err) != ErrorValidation {
		t.Fatalf("invalid List() error = %v", err)
	}
	service.Repository = nil
	if _, err := service.List(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("missing repository List() error = %v", err)
	}
}

func TestAttachmentAccessOpenReturnsOpaqueVerifiedStream(t *testing.T) {
	actor, owner, noteID := attachmentIDs(t)
	attachmentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	service := AttachmentAccessService{
		Repository: fakeAttachmentRepository{
			note:       AttachmentNote{NoteID: noteID, OwnerID: owner},
			attachment: AttachmentMetadata{AttachmentID: attachmentID, NoteID: noteID, DisplayName: "report.txt", StoredPath: "files/owner/report.txt", Size: 3},
		},
		Permissions: fakeAttachmentPermission{allowed: true},
		Store:       fakeAttachmentStore{data: []byte("one")},
	}
	download, err := service.Open(context.Background(), actor, attachmentID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer download.Reader.Close()
	data, err := io.ReadAll(download.Reader)
	if err != nil || string(data) != "one" || download.DisplayName != "report.txt" || download.Size != 3 {
		t.Fatalf("Open() = %#v, data=%q, err=%v", download, data, err)
	}
}

func TestAttachmentAccessOpenRejectsMetadataAndFileSizeMismatch(t *testing.T) {
	actor, owner, noteID := attachmentIDs(t)
	attachmentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	service := AttachmentAccessService{
		Repository: fakeAttachmentRepository{
			note:       AttachmentNote{NoteID: noteID, OwnerID: owner},
			attachment: AttachmentMetadata{AttachmentID: attachmentID, NoteID: noteID, DisplayName: "report.txt", StoredPath: "files/owner/report.txt", Size: 99},
		},
		Permissions: fakeAttachmentPermission{allowed: true},
		Store:       fakeAttachmentStore{data: []byte("one")},
	}
	if download, err := service.Open(context.Background(), actor, attachmentID); errorCategoryOf(err) != ErrorConflict || download.Reader != nil {
		t.Fatalf("Open() = %#v, %v", download, err)
	}
}

func TestAttachmentAccessOpenRejectsMissingStoreReader(t *testing.T) {
	actor, owner, noteID := attachmentIDs(t)
	attachmentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	service := AttachmentAccessService{
		Repository: fakeAttachmentRepository{
			note:       AttachmentNote{NoteID: noteID, OwnerID: owner},
			attachment: AttachmentMetadata{AttachmentID: attachmentID, NoteID: noteID, StoredPath: "files/owner/report.txt", Size: 3},
		},
		Permissions: fakeAttachmentPermission{allowed: true},
		Store:       fakeAttachmentStore{data: []byte("one"), nilReader: true},
	}
	if download, err := service.Open(context.Background(), actor, attachmentID); errorCategoryOf(err) != ErrorStorageUnavailable || download.Reader != nil {
		t.Fatalf("Open() = %#v, %v", download, err)
	}
}

func TestAttachmentAccessOpenRejectsZeroIdentity(t *testing.T) {
	service := AttachmentAccessService{Repository: fakeAttachmentRepository{}, Store: fakeAttachmentStore{}}
	if download, err := service.Open(context.Background(), domain.ObjectID{}, domain.ObjectID{}); errorCategoryOf(err) != ErrorValidation || download.Reader != nil {
		t.Fatalf("Open() = %#v, %v", download, err)
	}
}

func TestAttachmentAccessOpenReportsMissingDependencies(t *testing.T) {
	_, _, noteID := attachmentIDs(t)
	attachmentID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	for _, service := range []AttachmentAccessService{
		{Store: fakeAttachmentStore{}},
		{Repository: fakeAttachmentRepository{note: AttachmentNote{NoteID: noteID}}},
	} {
		if download, err := service.Open(context.Background(), domain.ObjectID{}, attachmentID); errorCategoryOf(err) != ErrorDependency || download.Reader != nil {
			t.Fatalf("Open() = %#v, %v", download, err)
		}
	}
}
