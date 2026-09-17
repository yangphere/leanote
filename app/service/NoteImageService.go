package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	//	"time"
)

type NoteImageService struct {
}

var noteImagePattern = regexp.MustCompile("(outputImage|getImage)\\?fileId=([a-z0-9A-Z]{24})")

func noteImageSourceFileIDs(content string) []string {
	matches := noteImagePattern.FindAllStringSubmatch(content, -1)
	ids := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		if len(match) != 3 || !db.IsValidObjectIDHex(match[2]) || seen[match[2]] {
			continue
		}
		seen[match[2]] = true
		ids = append(ids, match[2])
	}
	return ids
}

// 通过id, userId得到noteIds
func (this *NoteImageService) GetNoteIds(imageId string) []ObjectID {
	noteImages := []info.NoteImage{}
	db.ListByQWithFields(db.NoteImages, bson.M{"ImageId": db.MustObjectIDFromHex(imageId)}, []string{"NoteId"}, &noteImages)

	if noteImages != nil && len(noteImages) > 0 {
		noteIds := make([]ObjectID, len(noteImages))
		cnt := len(noteImages)
		for i := 0; i < cnt; i++ {
			noteIds[i] = noteImages[i].NoteId
		}
		return noteIds
	}

	return nil
}

// TODO 这个web可以用, 但api会传来, 不用用了
// 解析内容中的图片, 建立图片与note的关系
// <img src="/file/outputImage?fileId=12323232" />
// 图片必须是我的, 不然不添加
// imgSrc 防止博客修改了, 但内容删除了
func (this *NoteImageService) UpdateNoteImages(userId, noteId, imgSrc, content string) bool {
	if !db.IsValidObjectIDHex(userId) || !db.IsValidObjectIDHex(noteId) {
		return false
	}
	return this.updateNoteImages(context.Background(), db.MustObjectIDFromHex(userId), db.MustObjectIDFromHex(noteId), imgSrc, content) == nil
}

func (this *NoteImageService) desiredNoteImageIDs(ctx context.Context, ownerID domain.ObjectID, imgSrc, content string) ([]domain.ObjectID, error) {
	// 让主图成为内容的一员
	if imgSrc != "" {
		content = "<img src=\"" + imgSrc + "\" >" + content
	}
	matches := noteImagePattern.FindAllStringSubmatch(content, -1)
	candidates := make([]domain.ObjectID, 0, len(matches))
	seen := make(map[domain.ObjectID]bool, len(matches))
	for _, match := range matches {
		if len(match) != 3 || !db.IsValidObjectIDHex(match[2]) {
			continue
		}
		id := db.MustObjectIDFromHex(match[2])
		if !seen[id] {
			seen[id] = true
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	files := []info.File{}
	if err := db.Files.FindContext(ctx, bson.M{"_id": bson.M{"$in": candidates}, "UserId": ownerID}).All(&files); err != nil {
		return nil, err
	}
	ids := make([]domain.ObjectID, 0, len(files))
	for _, file := range files {
		ids = append(ids, file.FileId)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Hex() < ids[j].Hex() })
	return ids, nil
}

func (this *NoteImageService) updateNoteImages(ctx context.Context, ownerID, noteID domain.ObjectID, imgSrc, content string) error {
	ids, err := this.desiredNoteImageIDs(ctx, ownerID, imgSrc, content)
	if err != nil {
		return err
	}
	if _, err := db.NoteImages.RemoveAllContext(ctx, bson.M{"NoteId": noteID}); err != nil {
		return err
	}
	for _, imageID := range ids {
		if err := db.NoteImages.InsertContext(ctx, info.NoteImage{NoteId: noteID, ImageId: imageID}); err != nil {
			return err
		}
	}
	return nil
}

func (this *NoteImageService) verifyNoteImages(ctx context.Context, ownerID, noteID domain.ObjectID, imgSrc, content string) (bool, error) {
	want, err := this.desiredNoteImageIDs(ctx, ownerID, imgSrc, content)
	if err != nil {
		return false, err
	}
	rows := []info.NoteImage{}
	if err := db.NoteImages.FindContext(ctx, bson.M{"NoteId": noteID}).All(&rows); err != nil {
		return false, err
	}
	got := make([]domain.ObjectID, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.ImageId)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Hex() < got[j].Hex() })
	if len(got) != len(want) {
		return false, nil
	}
	for i := range want {
		if got[i] != want[i] {
			return false, nil
		}
	}
	return true, nil
}

// 复制图片, 把note的图片都copy给我, 且修改noteContent图片路径
func (this *NoteImageService) CopyNoteImages(fromNoteId, fromUserId, newNoteId, content, toUserId string) string {
	result, _ := this.CopyNoteImagesWithError(fromNoteId, fromUserId, newNoteId, content, toUserId)
	return result
}

// CopyNoteImagesWithError keeps the copy failure visible to the durable copy
// caller.  The legacy method above preserves its historical string-only API.
func (this *NoteImageService) CopyNoteImagesWithError(fromNoteId, fromUserId, newNoteId, content, toUserId string) (string, error) {
	return this.CopyNoteImagesWithOperation(fromNoteId, fromUserId, newNoteId, content, toUserId, "")
}

func (this *NoteImageService) CopyNoteImagesWithOperation(fromNoteId, fromUserId, newNoteId, content, toUserId, operationID string) (string, error) {
	return this.copyNoteImages(fromNoteId, fromUserId, newNoteId, content, toUserId, operationID, nil, false)
}

// CopyNoteImagesWithManifest limits a stable shared-copy retry to the source
// image identities frozen in the root create receipt. Images added to the
// source after the first attempt are not silently adopted by that operation.
func (this *NoteImageService) CopyNoteImagesWithManifest(fromNoteId, fromUserId, newNoteId, content, toUserId, operationID string, assets []applicationnotes.OperationAsset) (string, error) {
	allowed := make(map[string]applicationnotes.OperationAsset)
	for _, asset := range assets {
		if asset.IsAttach {
			continue
		}
		imageOperationID := operationID + ":image:" + asset.LocalFileID
		if !db.IsValidObjectIDHex(asset.LocalFileID) || asset.AssetID != stableCopiedImageID(imageOperationID, asset.LocalFileID, toUserId).Hex() {
			return content, fmt.Errorf("copy note image: invalid frozen asset manifest")
		}
		if _, duplicate := allowed[asset.LocalFileID]; duplicate {
			return content, fmt.Errorf("copy note image: duplicate frozen asset identity")
		}
		allowed[asset.LocalFileID] = asset
	}
	return this.copyNoteImages(fromNoteId, fromUserId, newNoteId, content, toUserId, operationID, allowed, true)
}

func (this *NoteImageService) copyNoteImages(fromNoteId, fromUserId, newNoteId, content, toUserId, operationID string, allowed map[string]applicationnotes.OperationAsset, enforceManifest bool) (string, error) {
	/* 弃用之
	// 得到fromNoteId的noteImages, 如果为空, 则直接返回content
	noteImages := []info.NoteImage{}
	db.ListByQWithFields(db.NoteImages, bson.M{"NoteId": db.MustObjectIDFromHex(fromNoteId)}, []string{"ImageId"}, &noteImages)
	if len(noteImages) == 0 {
		return content;
	}
	for _, noteImage := range noteImages {
		imageId := noteImage.ImageId.Hex()
		ok, newImageId := fileService.CopyImage(fromUserId, imageId, toUserId)
		if ok {
			replaceMap[imageId] = newImageId
		}
	}
	*/

	// 因为很多图片上传就会删除, 所以直接从内容中查看图片id进行复制

	// <img src="/file/outputImage?fileId=12323232" />
	// 把fileId=1232替换成新的
	replaceMap := map[string]string{}
	copyFailed := false

	content = noteImagePattern.ReplaceAllStringFunc(content, func(each string) string {
		// each = outputImage?fileId=541bd2f599c37b4f3r000003
		// each = getImage?fileId=541bd2f599c37b4f3r000003

		fileId := each[len(each)-24:] // 得到后24位, 也即id

		if _, ok := replaceMap[fileId]; !ok {
			if enforceManifest {
				if _, frozen := allowed[fileId]; !frozen {
					return each
				}
			}
			if db.IsValidObjectIDHex(fileId) {
				imageOperationID := ""
				if operationID != "" {
					imageOperationID = operationID + ":image:" + fileId
				}
				var ok2 bool
				var newImageId string
				if enforceManifest {
					ok2, newImageId = fileService.CopyImageWithFrozenAsset(fromUserId, toUserId, imageOperationID, allowed[fileId])
				} else {
					ok2, newImageId = fileService.CopyImageWithOperation(fromUserId, fileId, toUserId, imageOperationID)
				}
				if ok2 {
					if enforceManifest && allowed[fileId].AssetID != newImageId {
						copyFailed = true
						return each
					}
					replaceMap[fileId] = newImageId
				} else {
					copyFailed = true
					return each
				}
			} else {
				copyFailed = true
				replaceMap[fileId] = ""
			}
		}

		replaceFileId := replaceMap[fileId]
		if replaceFileId != "" {
			if each[0] == 'o' {
				return "outputImage?fileId=" + replaceFileId
			}
			return "getImage?fileId=" + replaceFileId
		}
		return each
	})

	if copyFailed {
		return content, fmt.Errorf("copy note image: source image copy failed")
	}
	return content, nil
}

func (this *NoteImageService) getImagesByNoteIds(noteIds []ObjectID) map[string][]info.File {
	noteNoteImages := []info.NoteImage{}
	db.ListByQ(db.NoteImages, bson.M{"NoteId": bson.M{"$in": noteIds}}, &noteNoteImages)

	// 得到imageId, 再去files表查所有的Files
	imageIds := []ObjectID{}

	// 图片1 => N notes
	imageIdNotes := map[string][]string{} // imageId => [noteId1, noteId2, ...]
	for _, noteImage := range noteNoteImages {
		imageId := noteImage.ImageId
		imageIds = append(imageIds, imageId)

		imageIdHex := imageId.Hex()
		noteId := noteImage.NoteId.Hex()
		if notes, ok := imageIdNotes[imageIdHex]; ok {
			imageIdNotes[imageIdHex] = append(notes, noteId)
		} else {
			imageIdNotes[imageIdHex] = []string{noteId}
		}
	}

	// 得到所有files
	files := []info.File{}
	db.ListByQ(db.Files, bson.M{"_id": bson.M{"$in": imageIds}}, &files)

	// 建立note->file关联
	noteImages := make(map[string][]info.File)
	for _, file := range files {
		fileIdHex := file.FileId.Hex() // == imageId
		// 这个fileIdHex有哪些notes呢?
		if notes, ok := imageIdNotes[fileIdHex]; ok {
			for _, noteId := range notes {
				if files, ok2 := noteImages[noteId]; ok2 {
					noteImages[noteId] = append(files, file)
				} else {
					noteImages[noteId] = []info.File{file}
				}
			}
		}
	}
	return noteImages
}
