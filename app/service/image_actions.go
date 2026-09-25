package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type ImageUploadKind string

const (
	ImageUploadPrivate  ImageUploadKind = "private"
	ImageUploadAvatar   ImageUploadKind = "avatar"
	ImageUploadBlogLogo ImageUploadKind = "blog_logo"
)

type ImageUploadInput struct {
	ActorID      string
	AlbumID      string
	Kind         ImageUploadKind
	Reader       io.Reader
	Limit        int64
	OriginalName string
	DisplayName  string
	Budget       applicationcontent.ImageBudget
}

type ImageUploadResult struct {
	File       info.File
	ExternalID string
}

func (this *FileService) UploadImage(ctx context.Context, input ImageUploadInput) (ImageUploadResult, error) {
	actorID, err := strictImageObjectID(input.ActorID, "image_upload_actor")
	if err != nil {
		return ImageUploadResult{}, err
	}
	if contentCreateRepair == nil {
		return ImageUploadResult{}, applicationcontent.NewError(applicationcontent.ErrorDependency, "content_create_repair_unavailable", nil)
	}
	prepared, err := applicationcontent.PrepareImageUpload(applicationcontent.PrepareImageUploadRequest{
		Reader: input.Reader, Limit: input.Limit, OriginalName: input.OriginalName, DisplayName: input.DisplayName, Budget: input.Budget,
	})
	if err != nil {
		return ImageUploadResult{}, err
	}
	albumID := db.MustObjectIDFromHex(DEFAULT_ALBUM_ID)
	defaultAlbum := true
	if input.AlbumID != "" {
		albumID, err = strictImageObjectID(input.AlbumID, "image_upload_album")
		if err != nil {
			return ImageUploadResult{}, err
		}
		if db.Albums == nil {
			return ImageUploadResult{}, applicationcontent.NewError(applicationcontent.ErrorDependency, "album_store_unavailable", nil)
		}
		var album info.Album
		if err := db.Albums.FindContext(ctx, bson.M{"_id": albumID, "UserId": actorID}).One(&album); err != nil {
			return ImageUploadResult{}, imageRepositoryError("image_upload_album", err)
		}
		defaultAlbum = false
	}
	imageID := db.NewObjectID()
	filename := imageID.Hex() + prepared.Extension
	storedPath := "files/" + GetRandomFilePath(actorID.Hex(), imageID.Hex()) + "/images/" + filename
	externalID := imageID.Hex()
	if input.Kind == ImageUploadAvatar || input.Kind == ImageUploadBlogLogo {
		storedPath = "public/upload/" + Digest3(actorID.Hex()) + "/" + actorID.Hex() + "/images/logo/" + filename
		externalID = storedPath
	}
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return ImageUploadResult{}, err
	}
	digest := sha256.Sum256(prepared.Data)
	file := info.File{
		FileId: imageID, UserId: actorID, AlbumId: albumID, Name: filename, Title: prepared.DisplayName,
		Size: int64(len(prepared.Data)), Path: storedPath, IsDefaultAlbum: defaultAlbum, CreatedTime: time.Now(),
	}
	_, err = contentCreateRepair.Execute(ctx, applicationcontent.CreateRepairCommand{
		Identity: applicationcontent.CreateIdentity{
			Action: "web_image_upload", OwnerID: actorID, RecordOwnerID: actorID, Kind: applicationcontent.AssetImage,
			AssetID: imageID.Hex(), ContentDigest: digest, RecordDigest: contentCreateImageRecordDigest(file), ContentSize: int64(len(prepared.Data)),
		},
		Destination: logical,
		Source:      bytes.NewReader(prepared.Data),
		ApplyMetadata: func(ctx context.Context) error {
			if db.Files == nil {
				return db.ErrMongoClientNotInitialized
			}
			return db.Files.InsertContext(ctx, file)
		},
	})
	if err != nil {
		return ImageUploadResult{File: file, ExternalID: externalID}, err
	}
	return ImageUploadResult{File: file, ExternalID: externalID}, nil
}

type mongoImageRepository struct{}

func (mongoImageRepository) LoadImage(ctx context.Context, imageID domain.ObjectID) (applicationcontent.StoredImage, error) {
	if db.Files == nil {
		return applicationcontent.StoredImage{}, db.ErrMongoClientNotInitialized
	}
	var file info.File
	if err := db.Files.FindContext(ctx, bson.M{"_id": imageID, "Type": ""}).One(&file); err != nil {
		return applicationcontent.StoredImage{}, imageRepositoryError("image_lookup", err)
	}
	return storedImage(file), nil
}

func (mongoImageRepository) ReferencingNotes(ctx context.Context, imageID domain.ObjectID) ([]applicationcontent.ImageNote, error) {
	if db.NoteImages == nil || db.Notes == nil {
		return nil, db.ErrMongoClientNotInitialized
	}
	var projections []info.NoteImage
	if err := db.NoteImages.FindContext(ctx, bson.M{"ImageId": imageID}).All(&projections); err != nil {
		return nil, imageRepositoryError("image_projection_list", err)
	}
	noteIDs := make([]domain.ObjectID, 0, len(projections))
	for _, projection := range projections {
		noteIDs = append(noteIDs, projection.NoteId)
	}
	if len(noteIDs) == 0 {
		return nil, nil
	}
	var notes []info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": bson.M{"$in": noteIDs}, "IsTrash": false, "IsDeleted": false}).All(&notes); err != nil {
		return nil, imageRepositoryError("image_note_list", err)
	}
	result := make([]applicationcontent.ImageNote, 0, len(notes))
	for _, note := range notes {
		result = append(result, applicationcontent.ImageNote{NoteID: note.NoteId, OwnerID: note.UserId, NotebookID: note.NotebookId, Public: note.IsBlog})
	}
	return result, nil
}

func (mongoImageRepository) DeleteImageAndProjection(ctx context.Context, ownerID, imageID domain.ObjectID) (bool, error) {
	if db.Files == nil || db.NoteImages == nil {
		return false, db.ErrMongoClientNotInitialized
	}
	var file info.File
	if err := db.Files.FindContext(ctx, bson.M{"_id": imageID, "UserId": ownerID, "Type": ""}).One(&file); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			absent, verifyErr := (mongoImageRepository{}).ImageAbsent(ctx, ownerID, imageID)
			if verifyErr != nil || absent {
				return absent, verifyErr
			}
			// A prior attempt may have removed the row and then failed while
			// deleting projections, with row compensation also failing. The
			// durable manifest retains the owner-scoped identity, so a retry
			// must finish the remaining projection deletion instead of looping.
			if _, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"ImageId": imageID}); err != nil {
				return false, err
			}
			return (mongoImageRepository{}).ImageAbsent(ctx, ownerID, imageID)
		}
		return false, err
	}
	if err := db.Files.RemoveContext(ctx, bson.M{"_id": imageID, "UserId": ownerID, "Type": ""}); err != nil {
		return false, err
	}
	if _, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"ImageId": imageID}); err != nil {
		if restoreErr := db.Files.InsertContext(ctx, file); restoreErr != nil {
			return false, applicationcontent.NewError(applicationcontent.ErrorPartialWrite, "image_projection_compensation", errors.Join(err, restoreErr))
		}
		return false, err
	}
	return true, nil
}

func (mongoImageRepository) ImageAbsent(ctx context.Context, ownerID, imageID domain.ObjectID) (bool, error) {
	if db.Files == nil || db.NoteImages == nil {
		return false, db.ErrMongoClientNotInitialized
	}
	files, err := db.Files.FindContext(ctx, bson.M{"_id": imageID, "UserId": ownerID, "Type": ""}).Count()
	if err != nil {
		return false, err
	}
	projections, err := db.NoteImages.FindContext(ctx, bson.M{"ImageId": imageID}).Count()
	return files == 0 && projections == 0, err
}

type mongoImagePermission struct{}

func (mongoImagePermission) CanReadNote(ctx context.Context, note applicationcontent.ImageNote, actorID domain.ObjectID) (bool, error) {
	_, allowed, err := (sharePermissionAdapter{}).ResolveNotePermission(ctx, note.OwnerID, actorID, note.NoteID)
	return allowed, err
}

func imageAccess() applicationcontent.ImageAccessService {
	return applicationcontent.ImageAccessService{Repository: mongoImageRepository{}, Permissions: mongoImagePermission{}, Store: contentStore}
}

func (this *FileService) OpenReadableImage(ctx context.Context, actorIDText, imageIDText string) (applicationcontent.ImageDownload, error) {
	actorID, err := optionalImageActor(actorIDText)
	if err != nil {
		return applicationcontent.ImageDownload{}, err
	}
	imageID, err := strictImageObjectID(imageIDText, "image_identity")
	if err != nil {
		return applicationcontent.ImageDownload{}, err
	}
	download, err := imageAccess().Open(ctx, actorID, imageID)
	if err != nil {
		return applicationcontent.ImageDownload{}, err
	}
	return materializeImageDownload(ctx, download)
}

func materializeImageDownload(ctx context.Context, download applicationcontent.ImageDownload) (applicationcontent.ImageDownload, error) {
	if contentStore == nil || download.Reader == nil || download.Size < 0 {
		if download.Reader != nil {
			_ = download.Reader.Close()
		}
		return applicationcontent.ImageDownload{}, applicationcontent.NewError(applicationcontent.ErrorDependency, "image_materialize_input", nil)
	}
	temporary, err := contentStore.CreateTemporary(ctx, ".leanote-image-download-")
	if err != nil {
		return applicationcontent.ImageDownload{}, errors.Join(err, download.Reader.Close())
	}
	written, copyErr := io.Copy(temporary, contextContentReader{ctx: ctx, reader: download.Reader})
	closeErr := download.Reader.Close()
	if copyErr != nil || closeErr != nil || written != download.Size {
		materializeErr := applicationcontent.NewError(applicationcontent.ErrorStorageUnavailable, "image_materialize", errors.Join(copyErr, closeErr))
		if ctxErr := ctx.Err(); ctxErr != nil {
			materializeErr = applicationcontent.NewError(applicationcontent.ErrorTimeout, "image_materialize_cancelled", errors.Join(copyErr, closeErr, ctxErr))
		}
		return applicationcontent.ImageDownload{}, errors.Join(materializeErr, temporary.Abort())
	}
	reader, size, err := temporary.Seal(ctx)
	if err != nil {
		return applicationcontent.ImageDownload{}, err
	}
	if size != download.Size {
		return applicationcontent.ImageDownload{}, errors.Join(
			applicationcontent.NewError(applicationcontent.ErrorConflict, "image_temporary_size", nil),
			reader.Close(),
		)
	}
	return applicationcontent.ImageDownload{Reader: reader, Size: size, Name: download.Name}, nil
}

func (this *FileService) ListImagesReadable(ctx context.Context, actorIDText, albumIDText, key string, pageNumber, pageSize int) (info.Page, error) {
	actorID, err := strictImageObjectID(actorIDText, "image_list_actor")
	if err != nil {
		return info.Page{}, err
	}
	if db.Files == nil {
		return info.Page{}, applicationcontent.NewError(applicationcontent.ErrorDependency, "image_store_unavailable", nil)
	}
	if pageSize <= 0 || pageSize > 100 {
		return info.Page{}, applicationcontent.NewError(applicationcontent.ErrorValidation, "image_page_size", nil)
	}
	skip, sortField := parsePageAndSort(pageNumber, pageSize, "CreatedTime", false)
	query := bson.M{"UserId": actorID, "Type": ""}
	if albumIDText == "" {
		query["IsDefaultAlbum"] = true
	} else {
		albumID, err := strictImageObjectID(albumIDText, "image_list_album")
		if err != nil {
			return info.Page{}, err
		}
		query["AlbumId"] = albumID
	}
	if key != "" {
		query["Title"] = bson.M{"$regex": bson.Regex{Pattern: ".*?" + regexp.QuoteMeta(key) + ".*", Options: "i"}}
	}
	count, err := db.Files.FindContext(ctx, query).Count()
	if err != nil {
		return info.Page{}, imageRepositoryError("image_list_count", err)
	}
	files := []info.File{}
	if err := db.Files.FindContext(ctx, query).Sort(sortField).Skip(skip).Limit(pageSize).All(&files); err != nil {
		return info.Page{}, imageRepositoryError("image_list", err)
	}
	return info.Page{Count: count, List: files}, nil
}

func (this *FileService) UpdateImageTitleResult(ctx context.Context, actorIDText, imageIDText, title string) error {
	actorID, err := strictImageObjectID(actorIDText, "image_title_actor")
	if err != nil {
		return err
	}
	imageID, err := strictImageObjectID(imageIDText, "image_title_identity")
	if err != nil {
		return err
	}
	title, err = applicationcontent.CleanVisibleText(title, true)
	if err != nil {
		return err
	}
	if db.Files == nil {
		return applicationcontent.NewError(applicationcontent.ErrorDependency, "image_store_unavailable", nil)
	}
	if err := db.Files.UpdateOneMatchedContext(ctx, bson.M{"_id": imageID, "UserId": actorID, "Type": ""}, bson.M{"$set": bson.M{"Title": title}}); err != nil {
		return imageRepositoryError("image_title_update", err)
	}
	return nil
}

func (this *FileService) DeleteImageWithOperation(ctx context.Context, actorIDText, imageIDText, operationID string) (bool, string) {
	actorID, err := strictImageObjectID(actorIDText, "image_delete_actor")
	if err != nil {
		return false, "no such item"
	}
	imageID, err := strictImageObjectID(imageIDText, "image_delete_identity")
	if err != nil {
		return false, "no such item"
	}
	if contentStore == nil || contentDeleteManifests == nil || contentLifecycle == nil {
		return false, "storage unavailable"
	}
	result, err := (applicationcontent.ImageDeleteService{
		Repository: mongoImageRepository{}, Content: contentStore, Manifests: contentDeleteManifests, Storage: contentLifecycle,
	}).Delete(ctx, applicationcontent.ImageDeleteCommand{OwnerID: actorID, ImageID: imageID, OperationID: strings.TrimSpace(operationID)})
	if err != nil {
		return false, "delete file error!"
	}
	return result.Deleted, ""
}

func (this *FileService) CopyImageForNote(ctx context.Context, actorIDText, imageIDText, noteIDText string) (bool, string) {
	actorID, err := strictImageObjectID(actorIDText, "image_copy_actor")
	if err != nil {
		return false, ""
	}
	imageID, err := strictImageObjectID(imageIDText, "image_copy_source")
	if err != nil {
		return false, ""
	}
	noteID, err := strictImageObjectID(noteIDText, "image_copy_note")
	if err != nil || db.Notes == nil {
		return false, ""
	}
	var note info.Note
	if err := db.Notes.FindContext(ctx, bson.M{"_id": noteID, "IsDeleted": false}).One(&note); err != nil || note.UserId.IsZero() {
		return false, ""
	}
	if note.UserId == actorID {
		return true, imageID.Hex()
	}
	if !shareService.HasUpdateNotePerm(noteID.Hex(), actorID.Hex()) {
		return false, ""
	}
	return this.CopyImage(actorID.Hex(), imageID.Hex(), note.UserId.Hex())
}

func strictImageObjectID(value, code string) (domain.ObjectID, error) {
	id, err := domain.ParseObjectID(value)
	if err != nil || id.IsZero() {
		return domain.ObjectID{}, applicationcontent.NewError(applicationcontent.ErrorValidation, code, err)
	}
	return id, nil
}

func optionalImageActor(value string) (domain.ObjectID, error) {
	if value == "" {
		return domain.ObjectID{}, nil
	}
	return strictImageObjectID(value, "image_actor")
}

func storedImage(file info.File) applicationcontent.StoredImage {
	return applicationcontent.StoredImage{
		ImageID: file.FileId, OwnerID: file.UserId, AlbumID: file.AlbumId, StoredName: file.Name, DisplayName: file.Title,
		StoredPath: file.Path, Size: file.Size, Generation: file.CreatedTime.UnixNano(),
	}
}

func imageRepositoryError(code string, err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, db.ErrDocumentNotFound) {
		return applicationcontent.NewError(applicationcontent.ErrorNotFound, code, err)
	}
	var contentErr *applicationcontent.Error
	if errors.As(err, &contentErr) {
		return err
	}
	return applicationcontent.NewError(applicationcontent.ErrorDependency, code, err)
}

var _ applicationcontent.ImageRepository = mongoImageRepository{}
var _ applicationcontent.ImagePermissionPort = mongoImagePermission{}
