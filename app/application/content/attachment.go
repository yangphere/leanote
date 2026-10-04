package content

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type AttachmentNote struct {
	NoteID  domain.ObjectID
	OwnerID domain.ObjectID
	Title   string
	Public  bool
}

type AttachmentMetadata struct {
	AttachmentID domain.ObjectID
	NoteID       domain.ObjectID
	UploaderID   domain.ObjectID
	StoredName   string
	DisplayName  string
	StoredPath   string
	MediaType    string
	Size         int64
	CreatedAt    time.Time
}

type AttachmentRepository interface {
	LoadNote(context.Context, domain.ObjectID) (AttachmentNote, error)
	LoadAttachment(context.Context, domain.ObjectID) (AttachmentMetadata, error)
	ListAttachments(context.Context, domain.ObjectID) ([]AttachmentMetadata, error)
}

type AttachmentPermissionPort interface {
	CanReadNote(context.Context, domain.ObjectID, domain.ObjectID, domain.ObjectID) (bool, error)
}

type AttachmentAccessService struct {
	Repository  AttachmentRepository
	Permissions AttachmentPermissionPort
	Store       ContentStore
}

type AttachmentList struct {
	Note        AttachmentNote
	Attachments []AttachmentMetadata
}

type AttachmentDownload struct {
	Reader      io.ReadCloser
	Size        int64
	DisplayName string
}

func (service AttachmentAccessService) List(ctx context.Context, actorID, noteID domain.ObjectID) (AttachmentList, error) {
	if service.Repository == nil {
		return AttachmentList{}, contentError(ErrorDependency, "attachment_repository_missing", nil)
	}
	if noteID.IsZero() {
		return AttachmentList{}, validationError("attachment_note_identity", nil)
	}
	note, err := service.Repository.LoadNote(ctx, noteID)
	if err != nil {
		return AttachmentList{}, attachmentDependencyError("attachment_note_lookup", err)
	}
	if note.NoteID != noteID || note.OwnerID.IsZero() {
		return AttachmentList{}, conflictError("attachment_note_identity", nil)
	}
	if err := service.authorizeRead(ctx, actorID, note); err != nil {
		return AttachmentList{}, err
	}
	attachments, err := service.Repository.ListAttachments(ctx, noteID)
	if err != nil {
		return AttachmentList{}, attachmentDependencyError("attachment_list", err)
	}
	for _, attachment := range attachments {
		if attachment.AttachmentID.IsZero() || attachment.NoteID != noteID || attachment.StoredPath == "" || attachment.Size < 0 {
			return AttachmentList{}, conflictError("attachment_list_identity", nil)
		}
	}
	return AttachmentList{Note: note, Attachments: attachments}, nil
}

func (service AttachmentAccessService) Open(ctx context.Context, actorID, attachmentID domain.ObjectID) (AttachmentDownload, error) {
	if service.Repository == nil {
		return AttachmentDownload{}, contentError(ErrorDependency, "attachment_repository_missing", nil)
	}
	if service.Store == nil {
		return AttachmentDownload{}, contentError(ErrorDependency, "attachment_store_missing", nil)
	}
	if attachmentID.IsZero() {
		return AttachmentDownload{}, validationError("attachment_identity", nil)
	}
	attachment, err := service.Repository.LoadAttachment(ctx, attachmentID)
	if err != nil {
		return AttachmentDownload{}, attachmentDependencyError("attachment_lookup", err)
	}
	if attachment.AttachmentID != attachmentID || attachment.NoteID.IsZero() || attachment.StoredPath == "" || attachment.Size < 0 {
		return AttachmentDownload{}, conflictError("attachment_identity", nil)
	}
	note, err := service.Repository.LoadNote(ctx, attachment.NoteID)
	if err != nil {
		return AttachmentDownload{}, attachmentDependencyError("attachment_note_lookup", err)
	}
	if note.NoteID != attachment.NoteID || note.OwnerID.IsZero() {
		return AttachmentDownload{}, conflictError("attachment_note_identity", nil)
	}
	if err := service.authorizeRead(ctx, actorID, note); err != nil {
		return AttachmentDownload{}, err
	}
	logical, err := ParseStoredPath(attachment.StoredPath)
	if err != nil {
		return AttachmentDownload{}, err
	}
	opened, err := service.Store.Open(ctx, logical)
	if err != nil {
		return AttachmentDownload{}, err
	}
	if opened.Reader == nil {
		return AttachmentDownload{}, storageError("attachment_reader_missing", nil)
	}
	if opened.Size != attachment.Size {
		closeErr := opened.Reader.Close()
		return AttachmentDownload{}, conflictError("attachment_size_mismatch", closeErr)
	}
	return AttachmentDownload{Reader: opened.Reader, Size: opened.Size, DisplayName: attachment.DisplayName}, nil
}

func (service AttachmentAccessService) authorizeRead(ctx context.Context, actorID domain.ObjectID, note AttachmentNote) error {
	if note.Public || (!actorID.IsZero() && actorID == note.OwnerID) {
		return nil
	}
	if actorID.IsZero() {
		return contentError(ErrorUnauthorized, "attachment_note_read", nil)
	}
	if service.Permissions == nil {
		return contentError(ErrorDependency, "attachment_permission_port_missing", nil)
	}
	allowed, err := service.Permissions.CanReadNote(ctx, note.NoteID, note.OwnerID, actorID)
	if err != nil {
		return attachmentDependencyError("attachment_permission", err)
	}
	if !allowed {
		return contentError(ErrorUnauthorized, "attachment_note_read", nil)
	}
	return nil
}

func attachmentDependencyError(code string, err error) error {
	var contentErr *Error
	if errors.As(err, &contentErr) {
		return err
	}
	return contentError(ErrorDependency, code, err)
}
