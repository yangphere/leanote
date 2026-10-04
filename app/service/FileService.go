package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service/contentremote"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const DEFAULT_ALBUM_ID = "52d3e8ac99c37b7f0d000001"

type FileService struct {
}

var remoteImageFetcher = contentremote.New(nil, nil)

// add Image
func (this *FileService) AddImage(image info.File, albumId, userId string, needCheckSize bool) (ok bool, msg string) {
	cleanTitle, err := applicationcontent.CleanVisibleText(image.Title, true)
	if err != nil {
		return false, "invalid title"
	}
	image.Title = cleanTitle
	image.CreatedTime = time.Now()
	if albumId != "" {
		image.AlbumId = db.MustObjectIDFromHex(albumId)
	} else {
		image.AlbumId = db.MustObjectIDFromHex(DEFAULT_ALBUM_ID)
		image.IsDefaultAlbum = true
	}
	image.UserId = db.MustObjectIDFromHex(userId)

	ok = db.Insert(db.Files, image)
	return
}

// list images
// if albumId == "" get default album images
func (this *FileService) ListImagesWithPage(userId, albumId, key string, pageNumber, pageSize int) info.Page {
	page, err := this.ListImagesReadable(context.Background(), userId, albumId, key, pageNumber, pageSize)
	if err != nil {
		return info.Page{List: []info.File{}}
	}
	return page
}

func (this *FileService) UpdateImageTitle(userId, fileId, title string) bool {
	return this.UpdateImageTitleResult(context.Background(), userId, fileId, title) == nil
}

// get all images names
// for upgrade
func (this *FileService) GetAllImageNamesMap(userId string) (m map[string]bool) {
	q := bson.M{"UserId": db.MustObjectIDFromHex(userId)}
	files := []info.File{}
	db.ListByQWithFields(db.Files, q, []string{"Name"}, &files)

	m = make(map[string]bool)
	if len(files) == 0 {
		return
	}

	for _, file := range files {
		m[file.Name] = true
	}
	return
}

// delete image
func (this *FileService) DeleteImage(userId, fileId string) (bool, string) {
	return this.DeleteImageWithOperation(context.Background(), userId, fileId, "")
}

// update image title
func (this *FileService) UpdateImage(userId, fileId, title string) bool {
	return this.UpdateImageTitle(userId, fileId, title)
}

// 复制共享的笔记时, 复制其中的图片到我本地
// 复制图片
func (this *FileService) CopyImage(userId, fileId, toUserId string) (bool, string) {
	return this.CopyImageWithOperation(userId, fileId, toUserId, "")
}

// CopyHTTPImage applies the shared public-only remote policy and publishes the
// validated bytes through the same no-clobber content store used by other
// content adapters. The legacy action has no client operation identity, so a
// request retry intentionally remains a new import rather than claiming a
// durable replay guarantee that the caller cannot address.
func (this *FileService) CopyHTTPImage(ctx context.Context, userID, sourceURL string, uploadLimit int64) (bool, string, string) {
	if !db.IsValidObjectIDHex(userID) || contentCreateRepair == nil {
		return false, "", "copy error"
	}
	remote, err := remoteImageFetcher.Fetch(ctx, sourceURL, uploadLimit)
	if err != nil {
		return false, "", "copy error"
	}
	title, err := applicationcontent.CleanVisibleText(remote.DisplayName, false)
	if err != nil {
		return false, "", "copy error"
	}
	ownerID := db.MustObjectIDFromHex(userID)
	fileID := db.NewObjectID()
	filename := fileID.Hex() + remote.Metadata.Extension
	storedPath := "files/" + GetRandomFilePath(userID, fileID.Hex()) + "/images/" + filename
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return false, "", "copy error"
	}
	digest := sha256.Sum256(remote.Data)
	image := info.File{
		FileId: fileID, UserId: ownerID, AlbumId: db.MustObjectIDFromHex(DEFAULT_ALBUM_ID), IsDefaultAlbum: true,
		Name: filename, Title: title, Path: storedPath, Size: int64(len(remote.Data)), CreatedTime: time.Now(),
	}
	_, err = contentCreateRepair.Execute(ctx, applicationcontent.CreateRepairCommand{
		Identity: applicationcontent.CreateIdentity{
			Action: "copy_http_image", OwnerID: ownerID, RecordOwnerID: ownerID, Kind: applicationcontent.AssetImage,
			AssetID: fileID.Hex(), ContentDigest: digest, RecordDigest: contentCreateImageRecordDigest(image), ContentSize: int64(len(remote.Data)),
		},
		Destination: logical,
		Source:      bytes.NewReader(remote.Data),
		ApplyMetadata: func(ctx context.Context) error {
			if db.Files == nil {
				return db.ErrMongoClientNotInitialized
			}
			return db.Files.InsertContext(ctx, image)
		},
	})
	if err != nil {
		return false, fileID.Hex(), "partial_write"
	}
	return true, fileID.Hex(), ""
}

func stableCopiedImageID(operationID, sourceFileID, destinationOwnerID string) domain.ObjectID {
	sum := sha256.Sum256([]byte("note-copy-image\x00" + operationID + "\x00" + sourceFileID + "\x00" + destinationOwnerID))
	var raw [12]byte
	copy(raw[:], sum[:12])
	if raw == ([12]byte{}) {
		raw[11] = 1
	}
	return domain.ObjectID(raw)
}

func (this *FileService) CopyImageWithOperation(userId, fileId, toUserId, operationID string) (bool, string) {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(fileId) || !db.IsValidObjectIDHex(toUserId) || contentStore == nil || db.Files == nil {
		return false, ""
	}
	if operationID != "" {
		return this.copyImageDurable(userId, fileId, toUserId, operationID, nil)
	}
	sourceID := db.MustObjectIDFromHex(fileId)
	destinationOwner := db.MustObjectIDFromHex(toUserId)
	var existing info.File
	if err := db.Files.FindContext(context.Background(), bson.M{"UserId": destinationOwner, "FromFileId": sourceID, "Type": ""}).One(&existing); err == nil {
		return true, existing.FileId.Hex()
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return false, ""
	}
	var source info.File
	if err := db.Files.FindContext(context.Background(), bson.M{"_id": sourceID, "UserId": db.MustObjectIDFromHex(userId), "Type": ""}).One(&source); err != nil {
		return false, ""
	}
	cleanTitle, err := applicationcontent.CleanVisibleText(source.Title, true)
	if err != nil {
		return false, ""
	}
	data, err := readImageSource(context.Background(), source)
	if err != nil {
		return false, ""
	}
	destinationID := db.NewObjectID()
	extension := strings.ToLower(filepath.Ext(source.Name))
	filename := destinationID.Hex() + extension
	storedPath := "files/" + GetRandomFilePath(toUserId, destinationID.Hex()) + "/images/" + filename
	err = publishCopiedImage(context.Background(), destinationOwner, destinationID, source.FileId, storedPath, data)
	if err != nil {
		return false, ""
	}
	destination := info.File{FileId: destinationID, UserId: destinationOwner, AlbumId: db.MustObjectIDFromHex(DEFAULT_ALBUM_ID), Name: filename, Title: cleanTitle, Path: storedPath, Size: int64(len(data)), FromFileId: source.FileId, IsDefaultAlbum: true, CreatedTime: time.Now()}
	if err := db.Files.InsertContext(context.Background(), destination); err != nil {
		return false, ""
	}
	return true, destinationID.Hex()
}

func (this *FileService) CopyImageWithFrozenAsset(userID, destinationUserID, operationID string, asset applicationnotes.OperationAsset) (bool, string) {
	return this.copyImageDurable(userID, asset.LocalFileID, destinationUserID, operationID, &asset)
}

func (this *FileService) copyImageDurable(userID, sourceFileID, destinationUserID, operationID string, frozen *applicationnotes.OperationAsset) (bool, string) {
	if !db.IsValidObjectIDHex(userID) || !db.IsValidObjectIDHex(sourceFileID) || !db.IsValidObjectIDHex(destinationUserID) || contentCreateRepair == nil {
		return false, ""
	}
	var source info.File
	if err := db.Files.FindContext(context.Background(), bson.M{"_id": db.MustObjectIDFromHex(sourceFileID), "UserId": db.MustObjectIDFromHex(userID)}).One(&source); err != nil {
		return false, ""
	}
	if _, err := applicationcontent.CleanVisibleText(source.Title, true); err != nil {
		return false, ""
	}
	data, err := readImageSource(context.Background(), source)
	if err != nil {
		return false, ""
	}
	destinationOwner := db.MustObjectIDFromHex(destinationUserID)
	destination := copiedImageDestination(source, destinationOwner, operationID)
	contentDigest := sha256.Sum256(data)
	recordDigest := contentCreateImageRecordDigest(destination)
	if frozen != nil && (frozen.AssetID != destination.FileId.Hex() || frozen.LocalFileID != sourceFileID || frozen.ContentSHA256 != hex.EncodeToString(contentDigest[:]) || frozen.RecordSHA256 != hex.EncodeToString(recordDigest[:])) {
		return false, ""
	}
	logical, err := applicationcontent.ParseStoredPath(destination.Path)
	if err != nil {
		return false, ""
	}
	_, err = contentCreateRepair.Execute(context.Background(), applicationcontent.CreateRepairCommand{
		Identity: applicationcontent.CreateIdentity{
			Action: "note_image_copy", OwnerID: destinationOwner, RecordOwnerID: destinationOwner,
			Kind: applicationcontent.AssetImage, AssetID: destination.FileId.Hex(), ContentDigest: contentDigest,
			RecordDigest: recordDigest, ContentSize: int64(len(data)),
		},
		Destination: logical, Source: bytes.NewReader(data),
		ApplyMetadata: func(ctx context.Context) error {
			var existing info.File
			err := db.Files.FindContext(ctx, bson.M{"_id": destination.FileId}).One(&existing)
			if err == nil {
				if contentCreateImageRecordDigest(existing) != recordDigest {
					return fmt.Errorf("image identity conflict")
				}
				return nil
			}
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return err
			}
			return db.Files.InsertContext(ctx, destination)
		},
	})
	if err != nil {
		return false, ""
	}
	return true, destination.FileId.Hex()
}

func copiedImageDestination(source info.File, destinationOwner domain.ObjectID, operationID string) info.File {
	destinationID := stableCopiedImageID(operationID, source.FileId.Hex(), destinationOwner.Hex())
	_, ext := SplitFilename(source.Name)
	filename := destinationID.Hex() + ext
	cleanTitle, _ := applicationcontent.CleanVisibleText(source.Title, true)
	return info.File{
		FileId: destinationID, UserId: destinationOwner, AlbumId: db.MustObjectIDFromHex(DEFAULT_ALBUM_ID),
		Name: filename, Title: cleanTitle, Path: "files/" + destinationOwner.Hex() + "/" + destinationID.Hex() + "/images/" + filename,
		Size: source.Size, FromFileId: source.FileId, IsDefaultAlbum: true, CreatedTime: source.CreatedTime,
	}
}

func readImageSource(ctx context.Context, source info.File) ([]byte, error) {
	logical, err := applicationcontent.ParseStoredPath(source.Path)
	if err != nil {
		return nil, err
	}
	opened, err := contentStore.Open(ctx, logical)
	if err != nil {
		return nil, err
	}
	if opened.Reader == nil || opened.Size != source.Size {
		if opened.Reader != nil {
			_ = opened.Reader.Close()
		}
		return nil, applicationcontent.NewError(applicationcontent.ErrorConflict, "image_copy_size", nil)
	}
	data, readErr := io.ReadAll(opened.Reader)
	closeErr := opened.Reader.Close()
	return data, errors.Join(readErr, closeErr)
}

func publishCopiedImage(ctx context.Context, ownerID, destinationID, sourceID domain.ObjectID, storedPath string, data []byte) error {
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	_, err = contentStore.Publish(ctx, applicationcontent.PublishRequest{Identity: applicationcontent.AssetIdentity{
		OperationID: destinationID.Hex(), OwnerID: ownerID, Kind: applicationcontent.AssetImage,
		SourceID: sourceID.Hex(), DestinationID: destinationID.Hex(), Digest: digest,
	}, Destination: logical, Source: bytes.NewReader(data)})
	return err
}

func verifyCopiedImage(ctx context.Context, ownerID, destinationID, sourceID domain.ObjectID, storedPath string, digest [sha256.Size]byte) (bool, error) {
	logical, err := applicationcontent.ParseStoredPath(storedPath)
	if err != nil {
		return false, err
	}
	result, err := contentStore.Verify(ctx, applicationcontent.VerifyRequest{Identity: applicationcontent.AssetIdentity{
		OperationID: destinationID.Hex(), OwnerID: ownerID, Kind: applicationcontent.AssetImage,
		SourceID: sourceID.Hex(), DestinationID: destinationID.Hex(), Digest: digest,
	}, Destination: logical, ExpectedDigest: digest})
	return result.Status == applicationcontent.VerificationApplied, err
}

// 是否是我的文件
func (this *FileService) IsMyFile(userId, fileId string) bool {
	// 如果有问题会panic
	if !db.IsValidObjectIDHex(fileId) || !db.IsValidObjectIDHex(userId) {
		return false
	}
	return db.Has(db.Files, bson.M{"UserId": db.MustObjectIDFromHex(userId), "_id": db.MustObjectIDFromHex(fileId)})
}
