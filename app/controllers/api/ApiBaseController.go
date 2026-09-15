package api

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/revel/revel"
	//	"encoding/json"
	"github.com/yangphere/leanote/app/controllers"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"os"
	//	"fmt"
	"io/ioutil"
	//	"fmt"
	//	"math"
	//	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// 公用Controller, 其它Controller继承它
type ApiBaseContrller struct {
	controllers.BaseController // 不能用*BaseController
}

const apiUploadPartialWrite = "partial_write"

// 得到token, 这个token是在AuthInterceptor设到Session中的
func (c ApiBaseContrller) getToken() string {
	return c.GetSession("_token")
}

// userId
// _userId是在AuthInterceptor设置的
func (c ApiBaseContrller) getUserId() string {
	return c.GetSession("_userId")
}

// 得到用户信息
func (c ApiBaseContrller) getUserInfo() info.User {
	userId := c.GetSession("_userId")
	if userId == "" {
		return info.User{}
	}
	return userService.GetUserInfo(userId)
}

// 上传附件
func (c ApiBaseContrller) uploadAttach(name string, noteId string) (ok bool, msg string, id string) {
	return c.uploadAttachWithIdentity(name, noteId, "")
}

func (c ApiBaseContrller) uploadAttachWithIdentity(name string, noteId string, assetID string) (ok bool, msg string, id string) {
	userId := c.getUserId()

	// 判断是否有权限为笔记添加附件
	// 如果笔记还没有添加是不是会有问题
	/*
		if !shareService.HasUpdateNotePerm(noteId, userId) {
			return
		}
	*/

	var data []byte
	c.Params.Bind(&data, name)
	files := c.Params.Files[name]
	if len(files) == 0 {
		msg = "fileRequired"
		return
	}
	handel := files[0]
	if data == nil || len(data) == 0 {
		msg = "fileRequired"
		return
	}

	// file, handel, err := c.Request.FormFile(name)
	// if err != nil {
	// 	return
	// }
	// defer file.Close()

	// data, err := ioutil.ReadAll(file)
	// if err != nil {
	// 	return
	// }
	// > 5M?
	maxFileSize := configService.GetUploadSize("uploadAttachSize")
	if maxFileSize <= 0 {
		maxFileSize = 1000
	}
	if float64(len(data)) > maxFileSize*float64(1024*1024) {
		msg = "fileIsTooLarge"
		return
	}

	// 生成上传路径。带稳定资产身份的上传可安全重试同一个文件。
	newGuid := NewGuid()
	if assetID != "" {
		newGuid = assetID
	}
	//	filePath :=	"files/" + Digest3(userId) + "/" + userId + "/" + Digest2(newGuid) + "/attachs"
	filePath := "files/" + GetRandomFilePath(userId, newGuid) + "/attachs"
	if assetID != "" {
		filePath = stableAPIUploadPath(userId, assetID, true)
	}

	dir := revel.BasePath + "/" + filePath
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	// 生成新的文件名
	filename := handel.Filename
	_, ext := SplitFilename(filename) // .doc
	filename = newGuid + ext
	toPath := dir + "/" + filename
	attachID := db.NewObjectID()
	if assetID != "" {
		attachID = db.MustObjectIDFromHex(assetID)
		var existing info.Attach
		findErr := db.Attachs.FindContext(context.Background(), bson.M{"_id": attachID, "NoteId": db.MustObjectIDFromHex(noteId), "UploadUserId": db.MustObjectIDFromHex(userId)}).One(&existing)
		if findErr == nil {
			if existing.Path != "" {
				toPath, err = resolveAPIUploadPath(revel.BasePath, existing.Path)
				if err != nil {
					return false, "", ""
				}
				if err := os.MkdirAll(filepath.Dir(toPath), 0755); err != nil {
					return false, "", ""
				}
				if _, err := os.Stat(toPath); err == nil {
					return true, "", existing.AttachId.Hex()
				}
			}
		} else if !errors.Is(findErr, mongo.ErrNoDocuments) {
			return false, "", ""
		}
	}
	err = ioutil.WriteFile(toPath, data, 0777)
	if err != nil {
		return
	}

	// add File to db
	fileType := ""
	if ext != "" {
		fileType = strings.ToLower(ext[1:])
	}
	filesize := GetFilesize(toPath)
	fileInfo := info.Attach{AttachId: attachID,
		Name:         filename,
		Title:        handel.Filename,
		NoteId:       db.MustObjectIDFromHex(noteId),
		UploadUserId: db.MustObjectIDFromHex(userId),
		Path:         filePath + "/" + filename,
		Type:         fileType,
		Size:         filesize}

	ok, msg = attachService.AddAttach(fileInfo, true)
	if !ok {
		var existing info.Attach
		findErr := db.Attachs.FindContext(context.Background(), bson.M{"_id": attachID, "NoteId": fileInfo.NoteId, "UploadUserId": fileInfo.UploadUserId}).One(&existing)
		if findErr == nil {
			// The database write may have succeeded even though AddAttach
			// returned false. Treat it as authoritative only when its file is
			// also durable; an incomplete duplicate must fail closed.
			if durableAPIAssetFile(revel.BasePath, existing.Path) {
				return true, "", existing.AttachId.Hex()
			}
			return false, msg, ""
		}
		if !errors.Is(findErr, mongo.ErrNoDocuments) {
			// Do not remove a file while the row state is unknown; a retry can
			// reconcile it without deleting another request's asset.
			return false, apiUploadPartialWrite, ""
		}
		if cleanupErr := removeAPIUploadFiles(revel.BasePath, fileInfo.Path); cleanupErr != nil {
			return false, apiUploadPartialWrite, ""
		}
		return
	}

	id = fileInfo.AttachId.Hex()
	return
}

// 上传图片
func (c ApiBaseContrller) upload(name string, noteId string, isAttach bool) (ok bool, msg string, id string) {
	return c.uploadWithAssetIdentity(name, noteId, isAttach, "")
}

func (c ApiBaseContrller) uploadWithAssetIdentity(name string, noteId string, isAttach bool, assetID string) (ok bool, msg string, id string) {
	if isAttach {
		return c.uploadAttachWithIdentity(name, noteId, assetID)
	}
	// file, handel, err := c.Request.FormFile(name)
	// if err != nil {
	// 	return
	// }
	// defer file.Close()

	var data []byte
	c.Params.Bind(&data, name)
	files := c.Params.Files[name]
	if len(files) == 0 {
		msg = "fileRequired"
		return
	}
	handel := files[0]
	if data == nil || len(data) == 0 {
		msg = "fileRequired"
		return
	}

	newGuid := NewGuid()
	if assetID != "" {
		newGuid = assetID
	}
	// 生成上传路径
	userId := c.getUserId()
	// fileUrlPath := "files/" + Digest3(userId) + "/" + userId + "/" + Digest2(newGuid) + "/images"
	fileUrlPath := "files/" + GetRandomFilePath(userId, newGuid) + "/images"
	if assetID != "" {
		fileUrlPath = stableAPIUploadPath(userId, assetID, false)
	}

	dir := revel.BasePath + "/" + fileUrlPath
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	// 生成新的文件名
	filename := handel.Filename
	_, ext := SplitFilename(filename)
	// if ext != ".gif" && ext != ".jpg" && ext != ".png" && ext != ".bmp" && ext != ".jpeg" {
	// 	msg = "notImage"
	// 	return
	// }

	filename = newGuid + ext
	// data, err := ioutil.ReadAll(file)
	// if err != nil {
	// 	return
	// }

	maxFileSize := configService.GetUploadSize("uploadImageSize")
	if maxFileSize <= 0 {
		maxFileSize = 1000
	}

	// > 2M?
	if float64(len(data)) > maxFileSize*float64(1024*1024) {
		msg = "fileIsTooLarge"
		return
	}

	toPath := dir + "/" + filename
	fileID := db.NewObjectID()
	if assetID != "" {
		fileID = db.MustObjectIDFromHex(assetID)
		var existing info.File
		findErr := db.Files.FindContext(context.Background(), bson.M{"_id": fileID, "UserId": db.MustObjectIDFromHex(userId)}).One(&existing)
		if findErr == nil {
			if strings.TrimSpace(existing.Path) == "" {
				return false, "", ""
			}
			toPath, err = resolveAPIUploadPath(revel.BasePath, existing.Path)
			if err != nil {
				return false, "", ""
			}
			if err := os.MkdirAll(filepath.Dir(toPath), 0755); err != nil {
				return false, "", ""
			}
			if _, err := os.Stat(toPath); err == nil {
				return true, "", existing.FileId.Hex()
			}
		} else if !errors.Is(findErr, mongo.ErrNoDocuments) {
			return false, "", ""
		}
	}
	err = ioutil.WriteFile(toPath, data, 0777)
	if err != nil {
		return
	}
	// 改变成gif图片
	_, toPathGif := TransToGif(toPath, 0, true)
	filename = GetFilename(toPathGif)
	filesize := GetFilesize(toPathGif)
	fileUrlPath += "/" + filename

	// File
	fileInfo := info.File{FileId: fileID,
		Name:  filename,
		Title: handel.Filename,
		Path:  fileUrlPath,
		Size:  filesize}
	ok, msg = fileService.AddImage(fileInfo, "", c.getUserId(), true)
	if ok {
		id = fileInfo.FileId.Hex()
	} else {
		var existing info.File
		findErr := db.Files.FindContext(context.Background(), bson.M{"_id": fileID, "UserId": db.MustObjectIDFromHex(userId)}).One(&existing)
		if findErr == nil {
			if durableAPIAssetFile(revel.BasePath, existing.Path) {
				return true, "", existing.FileId.Hex()
			}
			return false, msg, ""
		}
		if !errors.Is(findErr, mongo.ErrNoDocuments) {
			// Do not remove a file while the row state is unknown; a retry can
			// reconcile it without deleting another request's asset.
			return false, apiUploadPartialWrite, ""
		}
		if cleanupErr := removeAPIUploadFiles(revel.BasePath, toPath, toPathGif); cleanupErr != nil {
			return false, apiUploadPartialWrite, ""
		}
	}
	return
}

// removeAPIUploadFiles removes only paths generated by the API upload flow.
// It is deliberately idempotent so a retry can safely finish cleanup after a
// database insert returned an error.
func removeAPIUploadFiles(basePath string, paths ...string) error {
	var firstErr error
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		fullPath, err := resolveAPIUploadPath(basePath, path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = fmt.Errorf("cleanup API upload: %w", err)
		}
	}
	return firstErr
}

func resolveAPIUploadPath(basePath, path string) (string, error) {
	base, err := filepath.Abs(filepath.Clean(basePath))
	if err != nil {
		return "", fmt.Errorf("resolve API upload base: %w", err)
	}
	fullPath := filepath.Clean(path)
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(base, strings.TrimLeft(fullPath, `/\\`))
	}
	fullPath, err = filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve API upload path: %w", err)
	}
	relative, err := filepath.Rel(base, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("cleanup API upload: path escapes upload base")
	}
	return fullPath, nil
}

func durableAPIAssetFile(basePath, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	fullPath, err := resolveAPIUploadPath(basePath, path)
	if err != nil {
		return false
	}
	stat, err := os.Stat(fullPath)
	return err == nil && !stat.IsDir()
}

// cleanupUncommittedAPINoteAssets removes only assets belonging to a note
// create whose note was not committed.  A committed note can still return a
// projection/repair error, so the note existence check must precede cleanup.
// Asset rows are checked with both owner and resource identity; image rows
// that are already referenced by another note are retained.
var errAPINoteAlreadyCommitted = errors.New("api note create already committed")

func cleanupUncommittedAPINoteAssets(noteID, ownerID string, files []info.NoteFile) error {
	if !db.IsValidObjectIDHex(noteID) || !db.IsValidObjectIDHex(ownerID) {
		return fmt.Errorf("cleanup API note assets: invalid identity")
	}
	noteObjectID := db.MustObjectIDFromHex(noteID)
	ownerObjectID := db.MustObjectIDFromHex(ownerID)
	if db.Notes == nil || db.Attachs == nil || db.Files == nil || db.NoteImages == nil {
		return db.ErrMongoClientNotInitialized
	}
	var note info.Note
	err := db.Notes.FindContext(context.Background(), bson.M{"_id": noteObjectID, "UserId": ownerObjectID}).One(&note)
	if err == nil {
		// A tombstone is still a committed note and owns its assets until the
		// permanent-delete cleanup closes.  API create failure cleanup must
		// never take ownership of assets from any existing note generation.
		return errAPINoteAlreadyCommitted
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("check API note asset cleanup boundary: %w", err)
	}
	type assetKey struct {
		id       string
		isAttach bool
	}
	assets := make(map[assetKey]struct{}, len(files))
	for _, file := range files {
		if file.HasBody && db.IsValidObjectIDHex(file.FileId) {
			assets[assetKey{id: file.FileId, isAttach: file.IsAttach}] = struct{}{}
		}
	}
	for asset := range assets {
		assetObjectID := db.MustObjectIDFromHex(asset.id)
		if asset.isAttach {
			var attach info.Attach
			err := db.Attachs.FindContext(context.Background(), bson.M{
				"_id": assetObjectID, "NoteId": noteObjectID, "UploadUserId": ownerObjectID,
			}).One(&attach)
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			if err != nil {
				return fmt.Errorf("find API attach for cleanup: %w", err)
			}
			if err := removeAPIUploadFiles(revel.BasePath, attach.Path); err != nil {
				return err
			}
			if err := db.Attachs.RemoveContext(context.Background(), bson.M{
				"_id": attach.AttachId, "NoteId": noteObjectID, "UploadUserId": ownerObjectID,
			}); err != nil {
				return fmt.Errorf("remove API attach row: %w", err)
			}
			continue
		}

		var image info.File
		err := db.Files.FindContext(context.Background(), bson.M{"_id": assetObjectID, "UserId": ownerObjectID}).One(&image)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return fmt.Errorf("find API image for cleanup: %w", err)
		}
		refs, err := db.NoteImages.FindContext(context.Background(), bson.M{"ImageId": assetObjectID}).Count()
		if err != nil {
			return fmt.Errorf("check API image references: %w", err)
		}
		if refs != 0 {
			continue
		}
		if err := removeAPIUploadFiles(revel.BasePath, image.Path); err != nil {
			return err
		}
		if err := db.Files.RemoveContext(context.Background(), bson.M{"_id": image.FileId, "UserId": ownerObjectID}); err != nil {
			return fmt.Errorf("remove API image row: %w", err)
		}
	}
	for asset := range assets {
		assetObjectID := db.MustObjectIDFromHex(asset.id)
		if asset.isAttach {
			var attach info.Attach
			err := db.Attachs.FindContext(context.Background(), bson.M{"_id": assetObjectID, "NoteId": noteObjectID, "UploadUserId": ownerObjectID}).One(&attach)
			if err == nil {
				return fmt.Errorf("verify API attach cleanup: row remains")
			}
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return fmt.Errorf("verify API attach cleanup: %w", err)
			}
			continue
		}
		var image info.File
		err := db.Files.FindContext(context.Background(), bson.M{"_id": assetObjectID, "UserId": ownerObjectID}).One(&image)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return fmt.Errorf("verify API image cleanup: %w", err)
		}
		refs, err := db.NoteImages.FindContext(context.Background(), bson.M{"ImageId": assetObjectID}).Count()
		if err != nil {
			return fmt.Errorf("verify API image cleanup references: %w", err)
		}
		if refs == 0 {
			return fmt.Errorf("verify API image cleanup: unreferenced row remains")
		}
	}
	return nil
}
