package service

import (
	"context"
	"errors"
	"io"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type mongoAttachmentRepository struct{}

func (mongoAttachmentRepository) LoadNote(ctx context.Context, noteID domain.ObjectID) (applicationcontent.AttachmentNote, error) {
	if db.Notes == nil {
		return applicationcontent.AttachmentNote{}, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "IsTrash": false, "IsDeleted": false}).One(&note)
	if err != nil {
		return applicationcontent.AttachmentNote{}, attachmentRepositoryError("attachment_note_lookup", err)
	}
	return applicationcontent.AttachmentNote{NoteID: note.NoteId, OwnerID: note.UserId, Title: note.Title, Public: note.IsBlog}, nil
}

func (mongoAttachmentRepository) LoadAttachment(ctx context.Context, attachmentID domain.ObjectID) (applicationcontent.AttachmentMetadata, error) {
	if db.Attachs == nil {
		return applicationcontent.AttachmentMetadata{}, db.ErrMongoClientNotInitialized
	}
	var attachment info.Attach
	err := db.Attachs.FindContext(ctx, bson.M{"_id": attachmentID}).One(&attachment)
	if err != nil {
		return applicationcontent.AttachmentMetadata{}, attachmentRepositoryError("attachment_lookup", err)
	}
	return attachmentMetadata(attachment), nil
}

func (mongoAttachmentRepository) ListAttachments(ctx context.Context, noteID domain.ObjectID) ([]applicationcontent.AttachmentMetadata, error) {
	if db.Attachs == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	var attachments []info.Attach
	if err := db.Attachs.FindContext(ctx, bson.M{"NoteId": noteID}).Sort("CreatedTime", "_id").All(&attachments); err != nil {
		return nil, attachmentRepositoryError("attachment_list", err)
	}
	result := make([]applicationcontent.AttachmentMetadata, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, attachmentMetadata(attachment))
	}
	return result, nil
}

type mongoAttachmentPermission struct{}

func (mongoAttachmentPermission) CanReadNote(ctx context.Context, noteID, ownerID, actorID domain.ObjectID) (bool, error) {
	_, allowed, err := (sharePermissionAdapter{}).ResolveNotePermission(ctx, ownerID, actorID, noteID)
	return allowed, err
}

type sharePermissionAdapter struct{}

func (sharePermissionAdapter) ResolveNotePermission(ctx context.Context, ownerID, actorID, noteID domain.ObjectID) (int, bool, error) {
	return (&ShareService{}).ResolveNotePermission(ctx, ownerID, actorID, noteID, time.Now().UTC())
}

func attachmentAccess() applicationcontent.AttachmentAccessService {
	var store applicationcontent.ContentStore
	if contentStore != nil {
		store = contentStore
	}
	return applicationcontent.AttachmentAccessService{
		Repository: mongoAttachmentRepository{}, Permissions: mongoAttachmentPermission{}, Store: store,
	}
}

func (this *AttachService) ListReadable(ctx context.Context, actorIDText, noteIDText string) ([]info.Attach, string, error) {
	actorID, err := optionalAttachmentActor(actorIDText)
	if err != nil {
		return nil, "", err
	}
	noteID, err := domain.ParseObjectID(noteIDText)
	if err != nil || noteID.IsZero() {
		return nil, "", applicationcontent.NewError(applicationcontent.ErrorValidation, "attachment_note_identity", nil)
	}
	list, err := attachmentAccess().List(ctx, actorID, noteID)
	if err != nil {
		return nil, "", err
	}
	attachments := make([]info.Attach, 0, len(list.Attachments))
	for _, metadata := range list.Attachments {
		attachments = append(attachments, attachmentInfo(metadata))
	}
	return attachments, list.Note.Title, nil
}

func (this *AttachService) OpenReadable(ctx context.Context, actorIDText, attachmentIDText string) (applicationcontent.AttachmentDownload, error) {
	actorID, err := optionalAttachmentActor(actorIDText)
	if err != nil {
		return applicationcontent.AttachmentDownload{}, err
	}
	attachmentID, err := domain.ParseObjectID(attachmentIDText)
	if err != nil || attachmentID.IsZero() {
		return applicationcontent.AttachmentDownload{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "attachment_identity", nil)
	}
	download, err := attachmentAccess().Open(ctx, actorID, attachmentID)
	if err != nil {
		return applicationcontent.AttachmentDownload{}, err
	}
	return materializeAttachmentDownload(ctx, contentStore, download)
}

func (this *AttachService) OpenReadableArchive(ctx context.Context, actorIDText, noteIDText string) (io.ReadCloser, string, error) {
	attachments, title, err := this.ListReadable(ctx, actorIDText, noteIDText)
	if err != nil {
		return nil, "", err
	}
	if len(attachments) == 0 {
		return nil, title, applicationcontent.NewError(applicationcontent.ErrorNotFound, "attachment_list_empty", nil)
	}
	archive, err := OpenAttachmentArchive(ctx, attachments)
	return archive, title, err
}

func materializeAttachmentDownload(ctx context.Context, store attachmentArchiveStore, download applicationcontent.AttachmentDownload) (applicationcontent.AttachmentDownload, error) {
	if store == nil || download.Reader == nil || download.Size < 0 {
		var closeErr error
		if download.Reader != nil {
			closeErr = download.Reader.Close()
		}
		return applicationcontent.AttachmentDownload{}, errors.Join(
			applicationcontent.NewError(applicationcontent.ErrorDependency, "attachment_materialize_input", nil),
			closeErr,
		)
	}
	temporary, err := store.CreateTemporary(ctx, ".leanote-attachment-download-")
	if err != nil {
		return applicationcontent.AttachmentDownload{}, errors.Join(err, download.Reader.Close())
	}
	written, copyErr := io.Copy(temporary, contextContentReader{ctx: ctx, reader: download.Reader})
	closeErr := download.Reader.Close()
	if copyErr != nil || closeErr != nil || written != download.Size {
		materializeErr := applicationcontent.NewError(applicationcontent.ErrorStorageUnavailable, "attachment_materialize", errors.Join(copyErr, closeErr))
		if ctxErr := ctx.Err(); ctxErr != nil {
			materializeErr = applicationcontent.NewError(applicationcontent.ErrorTimeout, "attachment_materialize_cancelled", errors.Join(copyErr, closeErr, ctxErr))
		}
		return applicationcontent.AttachmentDownload{}, errors.Join(
			materializeErr,
			temporary.Abort(),
		)
	}
	reader, size, err := temporary.Seal(ctx)
	if err != nil {
		return applicationcontent.AttachmentDownload{}, errors.Join(
			err,
			temporary.Abort(),
		)
	}
	if size != written {
		return applicationcontent.AttachmentDownload{}, errors.Join(
			applicationcontent.NewError(applicationcontent.ErrorConflict, "attachment_temporary_size", nil),
			reader.Close(),
		)
	}
	return applicationcontent.AttachmentDownload{Reader: reader, Size: size, DisplayName: download.DisplayName}, nil
}

type contextContentReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextContentReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func attachmentMetadata(attachment info.Attach) applicationcontent.AttachmentMetadata {
	return applicationcontent.AttachmentMetadata{
		AttachmentID: attachment.AttachId, NoteID: attachment.NoteId, UploaderID: attachment.UploadUserId,
		StoredName: attachment.Name, DisplayName: attachment.Title, StoredPath: attachment.Path,
		MediaType: attachment.Type, Size: attachment.Size, CreatedAt: attachment.CreatedTime,
	}
}

func attachmentInfo(metadata applicationcontent.AttachmentMetadata) info.Attach {
	return info.Attach{
		AttachId: metadata.AttachmentID, NoteId: metadata.NoteID, UploadUserId: metadata.UploaderID,
		Name: metadata.StoredName, Title: metadata.DisplayName, Path: metadata.StoredPath,
		Type: metadata.MediaType, Size: metadata.Size, CreatedTime: metadata.CreatedAt,
	}
}

func optionalAttachmentActor(value string) (domain.ObjectID, error) {
	if value == "" {
		return domain.ObjectID{}, nil
	}
	actorID, err := domain.ParseObjectID(value)
	if err != nil || actorID.IsZero() {
		return domain.ObjectID{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "attachment_actor_identity", nil)
	}
	return actorID, nil
}

func attachmentRepositoryError(code string, err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return applicationcontent.NewError(applicationcontent.ErrorNotFound, code, err)
	}
	var contentErr *applicationcontent.Error
	if errors.As(err, &contentErr) {
		return err
	}
	return applicationcontent.NewError(applicationcontent.ErrorDependency, code, err)
}

var _ applicationcontent.AttachmentRepository = mongoAttachmentRepository{}
var _ applicationcontent.AttachmentPermissionPort = mongoAttachmentPermission{}
