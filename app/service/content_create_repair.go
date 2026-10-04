package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type mongoContentCreateRows struct{}

func (mongoContentCreateRows) VerifyCreateRow(ctx context.Context, manifest applicationcontent.CreateManifest) (applicationcontent.CreateRowStatus, error) {
	assetID, err := strictImageObjectID(manifest.AssetID, "create_repair_asset")
	if err != nil {
		return applicationcontent.CreateRowConflict, err
	}
	switch manifest.Kind {
	case applicationcontent.AssetImage:
		if db.Files == nil {
			return applicationcontent.CreateRowAbsent, db.ErrMongoClientNotInitialized
		}
		var row info.File
		err := db.Files.FindContext(ctx, bson.M{"_id": assetID, "Type": ""}).One(&row)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return applicationcontent.CreateRowAbsent, nil
		}
		if err != nil {
			return applicationcontent.CreateRowAbsent, err
		}
		logical, pathErr := applicationcontent.ParseStoredPath(row.Path)
		if pathErr != nil || logical != manifest.Destination || contentCreateImageRecordDigest(row) != manifest.RecordDigest {
			return applicationcontent.CreateRowConflict, nil
		}
		return applicationcontent.CreateRowExact, nil
	case applicationcontent.AssetAttachment:
		if db.Attachs == nil {
			return applicationcontent.CreateRowAbsent, db.ErrMongoClientNotInitialized
		}
		var row info.Attach
		err := db.Attachs.FindContext(ctx, bson.M{"_id": assetID}).One(&row)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return applicationcontent.CreateRowAbsent, nil
		}
		if err != nil {
			return applicationcontent.CreateRowAbsent, err
		}
		logical, pathErr := applicationcontent.ParseStoredPath(row.Path)
		if pathErr != nil || logical != manifest.Destination || contentCreateAttachmentRecordDigest(row) != manifest.RecordDigest {
			return applicationcontent.CreateRowConflict, nil
		}
		if manifest.Action == "web_attachment_upload" {
			if attachService.verifyAttachNum(ctx, row.NoteId, manifest.OwnerID, row.AttachId) {
				return applicationcontent.CreateRowExact, nil
			}
			ok, message := attachService.addAttachToNote(row, "")
			if !ok {
				return applicationcontent.CreateRowExact, errors.New("reconcile web attachment: " + message)
			}
		}
		return applicationcontent.CreateRowExact, nil
	default:
		return applicationcontent.CreateRowConflict, applicationcontent.NewError(applicationcontent.ErrorValidation, "create_repair_kind", nil)
	}
}

func contentCreateImageRecordDigest(row info.File) [sha256.Size]byte {
	record := struct {
		FileID         string
		UserID         string
		AlbumID        string
		Name           string
		Title          string
		Size           int64
		Type           string
		Path           string
		IsDefaultAlbum bool
		CreatedTime    string
		FromFileID     string
	}{
		FileID: row.FileId.Hex(), UserID: row.UserId.Hex(), AlbumID: row.AlbumId.Hex(), Name: row.Name, Title: row.Title,
		Size: row.Size, Type: row.Type, Path: row.Path, IsDefaultAlbum: row.IsDefaultAlbum,
		CreatedTime: row.CreatedTime.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), FromFileID: row.FromFileId.Hex(),
	}
	data, _ := json.Marshal(record)
	return sha256.Sum256(data)
}

func contentCreateAttachmentRecordDigest(row info.Attach) [sha256.Size]byte {
	record := struct {
		AttachID     string
		NoteID       string
		UploadUserID string
		Name         string
		Title        string
		Size         int64
		Type         string
		Path         string
		CreatedTime  string
	}{
		AttachID: row.AttachId.Hex(), NoteID: row.NoteId.Hex(), UploadUserID: row.UploadUserId.Hex(), Name: row.Name,
		Title: row.Title, Size: row.Size, Type: row.Type, Path: row.Path, CreatedTime: row.CreatedTime.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano),
	}
	data, _ := json.Marshal(record)
	return sha256.Sum256(data)
}
