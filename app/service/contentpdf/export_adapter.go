package contentpdf

import (
	"context"
	"errors"
	"io"
	"path"

	application "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service/contentremote"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const maxPDFImageBytes = 128 * 1024 * 1024

// PDFRepository keeps persistence and sharing rules outside the pure content
// application package while preserving lookup failures for the caller.
type PDFRepository interface {
	FindNote(context.Context, domain.ObjectID) (info.Note, error)
	FindNoteContent(context.Context, domain.ObjectID, domain.ObjectID) (info.NoteContent, error)
	CanReadNote(context.Context, info.Note, domain.ObjectID) (bool, error)
	FindFile(context.Context, domain.ObjectID, domain.ObjectID) (info.File, error)
}

// MongoPDFRepository is the production persistence adapter. Every lookup is
// owner-scoped and returns driver errors instead of collapsing them to empty
// values or denied permissions.
type MongoPDFRepository struct{}

func (MongoPDFRepository) FindNote(ctx context.Context, noteID domain.ObjectID) (info.Note, error) {
	if db.Notes == nil {
		return info.Note{}, db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "IsDeleted": false}).One(&note)
	return note, err
}

func (MongoPDFRepository) FindNoteContent(ctx context.Context, noteID, ownerID domain.ObjectID) (info.NoteContent, error) {
	if db.NoteContents == nil {
		return info.NoteContent{}, db.ErrMongoClientNotInitialized
	}
	var content info.NoteContent
	err := db.NoteContents.FindContext(ctx, bson.M{"_id": noteID, "UserId": ownerID}).One(&content)
	return content, err
}

func (MongoPDFRepository) FindFile(ctx context.Context, ownerID, fileID domain.ObjectID) (info.File, error) {
	if db.Files == nil {
		return info.File{}, db.ErrMongoClientNotInitialized
	}
	var file info.File
	err := db.Files.FindContext(ctx, bson.M{"_id": fileID, "UserId": ownerID}).One(&file)
	return file, err
}

func (MongoPDFRepository) CanReadNote(ctx context.Context, note info.Note, actorID domain.ObjectID) (bool, error) {
	if db.ShareNotes == nil || db.ShareNotebooks == nil || db.GroupUsers == nil || db.Groups == nil {
		return false, db.ErrMongoClientNotInitialized
	}
	groupIDs, err := pdfActorGroupIDs(ctx, actorID)
	if err != nil {
		return false, err
	}
	recipient := bson.M{"ToUserId": actorID}
	if len(groupIDs) != 0 {
		recipient = bson.M{"$or": []bson.M{
			bson.M{"ToUserId": actorID},
			bson.M{"ToGroupId": bson.M{"$in": groupIDs}},
		}}
	}
	recipient["UserId"] = note.UserId
	recipient["NoteId"] = note.NoteId
	count, err := db.ShareNotes.FindContext(ctx, recipient).Count()
	if err != nil || count != 0 {
		return count != 0, err
	}
	if note.NotebookId.IsZero() {
		return false, nil
	}
	delete(recipient, "NoteId")
	recipient["NotebookId"] = note.NotebookId
	count, err = db.ShareNotebooks.FindContext(ctx, recipient).Count()
	return count != 0, err
}

func pdfActorGroupIDs(ctx context.Context, actorID domain.ObjectID) ([]domain.ObjectID, error) {
	var memberships []info.GroupUser
	if err := db.GroupUsers.FindContext(ctx, bson.M{"UserId": actorID}).All(&memberships); err != nil {
		return nil, err
	}
	var owned []info.Group
	if err := db.Groups.FindContext(ctx, bson.M{"UserId": actorID}).All(&owned); err != nil {
		return nil, err
	}
	seen := make(map[domain.ObjectID]struct{}, len(memberships)+len(owned))
	groupIDs := make([]domain.ObjectID, 0, len(memberships)+len(owned))
	for _, membership := range memberships {
		if _, ok := seen[membership.GroupId]; !ok {
			seen[membership.GroupId] = struct{}{}
			groupIDs = append(groupIDs, membership.GroupId)
		}
	}
	for _, group := range owned {
		if _, ok := seen[group.GroupId]; !ok {
			seen[group.GroupId] = struct{}{}
			groupIDs = append(groupIDs, group.GroupId)
		}
	}
	return groupIDs, nil
}

type NotePort struct {
	Repository PDFRepository
}

func (port NotePort) LoadAuthorized(ctx context.Context, actorID, noteID domain.ObjectID) (application.PDFNoteSnapshot, error) {
	if err := pdfAdapterContextError(ctx, "pdf_note_canceled"); err != nil {
		return application.PDFNoteSnapshot{}, err
	}
	if port.Repository == nil || actorID.IsZero() || noteID.IsZero() {
		return application.PDFNoteSnapshot{}, application.NewError(application.ErrorValidation, "pdf_note_identity", nil)
	}
	note, err := port.Repository.FindNote(ctx, noteID)
	if err != nil {
		return application.PDFNoteSnapshot{}, repositoryError("pdf_note_lookup", err)
	}
	if err := pdfAdapterContextError(ctx, "pdf_note_canceled"); err != nil {
		return application.PDFNoteSnapshot{}, err
	}
	if note.NoteId != noteID || note.UserId.IsZero() || note.IsDeleted {
		return application.PDFNoteSnapshot{}, application.NewError(application.ErrorNotFound, "pdf_note_missing", nil)
	}
	if actorID != note.UserId && !note.IsBlog {
		allowed, err := port.Repository.CanReadNote(ctx, note, actorID)
		if err != nil {
			return application.PDFNoteSnapshot{}, repositoryError("pdf_note_permission", err)
		}
		if !allowed {
			return application.PDFNoteSnapshot{}, application.NewError(application.ErrorUnauthorized, "pdf_note_read", nil)
		}
	}
	content, err := port.Repository.FindNoteContent(ctx, noteID, note.UserId)
	if err != nil {
		return application.PDFNoteSnapshot{}, repositoryError("pdf_note_content", err)
	}
	if err := pdfAdapterContextError(ctx, "pdf_note_canceled"); err != nil {
		return application.PDFNoteSnapshot{}, err
	}
	if content.NoteId != noteID || content.UserId != note.UserId {
		return application.PDFNoteSnapshot{}, application.NewError(application.ErrorConflict, "pdf_note_content_identity", nil)
	}
	return application.PDFNoteSnapshot{
		NoteID: noteID, OwnerID: note.UserId, Title: note.Title,
		HTML: content.Content, Markdown: note.IsMarkdown,
	}, nil
}

type ResourcePort struct {
	Repository PDFRepository
	Store      application.ContentStore
}

func (port ResourcePort) LoadAuthorized(ctx context.Context, ownerID, fileID domain.ObjectID, maxBytes int64) (resource application.PDFResource, resultErr error) {
	if err := pdfAdapterContextError(ctx, "pdf_resource_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	if port.Repository == nil || port.Store == nil || ownerID.IsZero() || fileID.IsZero() || maxBytes <= 0 {
		return application.PDFResource{}, application.NewError(application.ErrorValidation, "pdf_resource_identity", nil)
	}
	file, err := port.Repository.FindFile(ctx, ownerID, fileID)
	if err != nil {
		return application.PDFResource{}, repositoryError("pdf_resource_lookup", err)
	}
	if err := pdfAdapterContextError(ctx, "pdf_resource_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	if file.FileId != fileID || file.UserId != ownerID {
		return application.PDFResource{}, application.NewError(application.ErrorNotFound, "pdf_resource_missing", nil)
	}
	logical, err := application.ParseStoredPath(file.Path)
	if err != nil {
		return application.PDFResource{}, err
	}
	opened, err := port.Store.Open(ctx, logical)
	if err != nil {
		return application.PDFResource{}, err
	}
	if opened.Reader == nil {
		return application.PDFResource{}, application.NewError(application.ErrorStorageUnavailable, "pdf_resource_reader_missing", nil)
	}
	defer func() {
		if closeErr := opened.Reader.Close(); closeErr != nil {
			resource = application.PDFResource{}
			category := application.ErrorStorageUnavailable
			if resultErr != nil {
				category = application.ErrorUnknownResult
			}
			resultErr = application.NewError(category, "pdf_resource_close", errors.Join(resultErr, closeErr))
		}
	}()
	if maxBytes > maxPDFImageBytes {
		maxBytes = maxPDFImageBytes
	}
	if opened.Size <= 0 || opened.Size > maxBytes {
		return application.PDFResource{}, application.NewError(application.ErrorTooLarge, "pdf_resource_size", nil)
	}
	data, err := application.ReadBounded(pdfContextReader{ctx: ctx, reader: opened.Reader}, maxBytes)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return application.PDFResource{}, application.NewError(application.ErrorTimeout, "pdf_resource_read_canceled", errors.Join(err, ctx.Err()))
		}
		return application.PDFResource{}, err
	}
	if err := pdfAdapterContextError(ctx, "pdf_resource_read_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	metadata, err := application.ValidateImage(data, path.Ext(logical.Value), application.HardImageBudget())
	if err != nil {
		return application.PDFResource{}, err
	}
	if err := pdfAdapterContextError(ctx, "pdf_resource_decode_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	return application.PDFResource{MIME: metadata.MIME, Data: data}, nil
}

type RemoteFetcher interface {
	Fetch(context.Context, string, int64) (contentremote.Result, error)
}

type RemoteResourcePort struct {
	Fetcher          RemoteFetcher
	UploadImageBytes func() (int64, error)
}

func (port RemoteResourcePort) LoadPublic(ctx context.Context, rawURL string, maxBytes int64) (application.PDFResource, error) {
	if err := pdfAdapterContextError(ctx, "pdf_remote_resource_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	if port.Fetcher == nil || port.UploadImageBytes == nil || maxBytes <= 0 {
		return application.PDFResource{}, application.NewError(application.ErrorDependency, "pdf_remote_resource_unavailable", nil)
	}
	uploadLimit, err := port.UploadImageBytes()
	if err != nil || uploadLimit <= 0 {
		return application.PDFResource{}, application.NewError(application.ErrorDependency, "pdf_remote_limit", err)
	}
	if maxBytes > uploadLimit {
		maxBytes = uploadLimit
	}
	result, err := port.Fetcher.Fetch(ctx, rawURL, maxBytes)
	if err != nil {
		return application.PDFResource{}, err
	}
	if err := pdfAdapterContextError(ctx, "pdf_remote_resource_canceled"); err != nil {
		return application.PDFResource{}, err
	}
	return application.PDFResource{MIME: result.Metadata.MIME, Data: append([]byte(nil), result.Data...)}, nil
}

func repositoryError(code string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return application.NewError(application.ErrorTimeout, code+"_canceled", err)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return application.NewError(application.ErrorNotFound, code, err)
	}
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return err
	}
	return application.NewError(application.ErrorDependency, code, err)
}

type pdfContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader pdfContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func pdfAdapterContextError(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return application.NewError(application.ErrorTimeout, code, err)
	}
	return nil
}

var _ application.PDFNotePort = NotePort{}
var _ application.PDFResourcePort = ResourcePort{}
var _ application.PDFRemoteResourcePort = RemoteResourcePort{}
