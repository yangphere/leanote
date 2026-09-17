package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/revel/revel"
	//	"encoding/json"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	applicationnotes "github.com/yangphere/leanote/app/application/notes"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"regexp"
	"strings"
	"time"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"fmt"
	//	"bytes"
	//	"os"
)

// 笔记API

type ApiNote struct {
	ApiBaseContrller
}

// 获取同步的笔记
// > afterUsn的笔记
// 无Desc, Abstract, 有Files
/*
  {
    "NoteId": "55195fa199c37b79be000005",
    "NotebookId": "55195fa199c37b79be000002",
    "UserId": "55195fa199c37b79be000001",
    "Title": "Leanote语法Leanote语法Leanote语法Leanote语法Leanote语法",
    "Desc": "",
    "Tags": null,
    "Abstract": "",
    "Content": "",
    "IsMarkdown": true,
    "IsBlog": false,
    "IsTrash": false,
    "Usn": 5,
    "Files": [],
    "CreatedTime": "2015-03-30T22:37:21.695+08:00",
    "UpdatedTime": "2015-03-30T22:37:21.724+08:00",
    "PublicTime": "2015-03-30T22:37:21.695+08:00"
  }
*/
func (c ApiNote) GetSyncNotes(afterUsn, maxEntry int) revel.Result {
	if maxEntry == 0 {
		maxEntry = 100
	}
	notes := noteService.GetSyncNotes(c.getUserId(), afterUsn, maxEntry)
	return c.RenderJSON(notes)
}

// 得到笔记本下的笔记
// [OK]
func (c ApiNote) GetNotes(notebookId string) revel.Result {
	if notebookId != "" && !db.IsValidObjectIDHex(notebookId) {
		re := info.NewApiRe()
		re.Msg = "notebookIdInvalid"
		return c.RenderJSON(re)
	}
	_, notes := noteService.ListNotes(c.getUserId(), notebookId, false, c.GetPage(), pageSize, defaultSortField, false, false)
	return c.RenderJSON(noteService.ToApiNotes(notes))
}

// 得到trash
// [OK]
func (c ApiNote) GetTrashNotes() revel.Result {
	_, notes := noteService.ListNotes(c.getUserId(), "", true, c.GetPage(), pageSize, defaultSortField, false, false)
	return c.RenderJSON(noteService.ToApiNotes(notes))

}

// get Note
// [OK]
/*
{
  "NoteId": "550c0bee2ec82a2eb5000000",
  "NotebookId": "54a1676399c37b1c77000004",
  "UserId": "54a1676399c37b1c77000002",
  "Title": "asdfadsf--=",
  "Desc": "",
  "Tags": [
  ],
  "Abstract": "",
  "Content": "",
  "IsMarkdown": false,
  "IsBlog": false,
  "IsTrash": false,
  "Usn": 8,
  "Files": [
    {
      "FileId": "551975d599c37b970f000000",
      "LocalFileId": "",
      "Type": "",
      "Title": "",
      "HasBody": false,
      "IsAttach": false
    },
    {
      "FileId": "551975de99c37b970f000001",
      "LocalFileId": "",
      "Type": "doc",
      "Title": "李铁-print-en.doc",
      "HasBody": false,
      "IsAttach": true
    },
    {
      "FileId": "551975de99c37b970f000002",
      "LocalFileId": "",
      "Type": "doc",
      "Title": "李铁-print.doc",
      "HasBody": false,
      "IsAttach": true
    }
  ],
  "CreatedTime": "2015-03-20T20:00:52.463+08:00",
  "UpdatedTime": "2015-03-31T00:12:44.967+08:00",
  "PublicTime": "2015-03-20T20:00:52.463+08:00"
}
*/
func (c ApiNote) GetNote(noteId string) revel.Result {
	if !db.IsValidObjectIDHex(noteId) {
		re := info.NewApiRe()
		re.Msg = "noteIdInvalid"
		return c.RenderJSON(re)
	}

	note := noteService.GetNote(noteId, c.getUserId())
	if note.NoteId.IsZero() {
		re := info.NewApiRe()
		re.Msg = "notExists"
		return c.RenderJSON(re)
	}
	apiNotes := noteService.ToApiNotes([]info.Note{note})
	return c.RenderJSON(apiNotes[0])
}

// 得到note和内容
// [OK]
func (c ApiNote) GetNoteAndContent(noteId string) revel.Result {
	noteAndContent := noteService.GetNoteAndContent(noteId, c.getUserId())

	apiNotes := noteService.ToApiNotes([]info.Note{noteAndContent.Note})
	apiNote := apiNotes[0]
	apiNote.Content = noteService.FixContent(noteAndContent.Content, noteAndContent.IsMarkdown)
	return c.RenderJSON(apiNote)
}

// content里的image, attach链接是
// https://leanote.com/api/file/getImage?fileId=xx
// https://leanote.com/api/file/getAttach?fileId=xx
// 将fileId=映射成ServerFileId, 这里的fileId可能是本地的FileId
func (c ApiNote) fixPostNotecontent(noteOrContent *info.ApiNote) {
	if noteOrContent.Content == "" {
		return
	}

	files := noteOrContent.Files
	if files != nil && len(files) > 0 {
		for _, file := range files {
			if file.LocalFileId != "" {
				LogJ(file)
				if !file.IsAttach {
					// <img src="https://"
					// ![](http://demo.leanote.top/api/file/getImage?fileId=5863219465b68e4fd5000001)
					reg, _ := regexp.Compile(`https*://[^/]*?/api/file/getImage\?fileId=` + file.LocalFileId)
					// Log(reg)
					noteOrContent.Content = reg.ReplaceAllString(noteOrContent.Content, `/api/file/getImage?fileId=`+file.FileId)

					// // "http://a.com/api/file/getImage?fileId=localId" => /api/file/getImage?fileId=serverId
					// noteOrContent.Content = strings.Replace(noteOrContent.Content,
					// 	baseUrl + "/api/file/getImage?fileId="+file.LocalFileId,
					// 	"/api/file/getImage?fileId="+file.FileId, -1)
				} else {
					reg, _ := regexp.Compile(`https*://[^/]*?/api/file/getAttach\?fileId=` + file.LocalFileId)
					// Log(reg)
					noteOrContent.Content = reg.ReplaceAllString(noteOrContent.Content, `/api/file/getAttach?fileId=`+file.FileId)
					/*
						noteOrContent.Content = strings.Replace(noteOrContent.Content,
							baseUrl + "/api/file/getAttach?fileId="+file.LocalFileId,
							"/api/file/getAttach?fileId="+file.FileId, -1)
					*/
				}
			}
		}
	}
}

// 得到内容
func (c ApiNote) GetNoteContent(noteId string) revel.Result {
	userId := c.getUserId()
	note := noteService.GetNote(noteId, userId)
	//	re := info.NewRe()
	noteContent := noteService.GetNoteContent(noteId, userId)
	if noteContent.Content != "" {
		noteContent.Content = noteService.FixContent(noteContent.Content, note.IsMarkdown)
	}

	apiNoteContent := info.ApiNoteContent{
		NoteId:  noteContent.NoteId,
		UserId:  noteContent.UserId,
		Content: noteContent.Content,
	}

	return c.RenderJSON(apiNoteContent)
}

// 添加笔记
// [OK]
func (c ApiNote) AddNote(noteOrContent info.ApiNote) revel.Result {
	userId := db.MustObjectIDFromHex(c.getUserId())
	re := info.NewRe()
	myUserId := userId
	// 为共享新建?
	/*
		if noteOrContent.FromUserId != "" {
			userId = db.MustObjectIDFromHex(noteOrContent.FromUserId)
		}
	*/
	//	Log(noteOrContent.Title)
	//		LogJ(noteOrContent)

	/*
		LogJ(c.Params)
		for name, _ := range c.Params.Files {
			Log(name)
			file, _, _ := c.Request.FormFile(name)
			LogJ(file)
		}
	*/
	//	return c.RenderJSON(re)
	if noteOrContent.NotebookId == "" || !db.IsValidObjectIDHex(noteOrContent.NotebookId) {
		re.Msg = "notebookIdNotExists"
		return c.RenderJSON(re)
	}
	if !noteService.CanCreateNote(c.getUserId(), c.getUserId(), noteOrContent.NotebookId) {
		re.Msg = "notebookIdNotExists"
		return c.RenderJSON(re)
	}

	noteId := db.NewObjectID()
	clientNoteID := noteOrContent.NoteId != ""
	if clientNoteID {
		if !db.IsValidObjectIDHex(noteOrContent.NoteId) {
			re.Msg = "noteIdNotExists"
			return c.RenderJSON(re)
		}
		noteId = db.MustObjectIDFromHex(noteOrContent.NoteId)
	}
	var createOperationID string
	var createInputDigest string
	var createAssets []applicationnotes.OperationAsset
	if clientNoteID {
		contentDigests := make(map[int]string, len(noteOrContent.Files))
		for index, file := range noteOrContent.Files {
			if !file.HasBody || file.LocalFileId == "" {
				continue
			}
			candidate, message := c.prepareAPINoteAsset("FileDatas["+file.LocalFileId+"]", noteId.Hex(), file.IsAttach, "")
			if message != "" {
				re.Msg = message
				return c.RenderJSON(re)
			}
			contentDigests[index] = candidate.Digest
		}
		var identityErr error
		createOperationID, createInputDigest, createAssets, identityErr = newAPINoteCreateOperationWithDigests(userId, noteId, noteOrContent, contentDigests)
		if identityErr != nil {
			re.Msg = "saveFailed"
			return c.RenderJSON(re)
		}
		if category := noteService.BeginNoteCreateAssetReceipt(userId, noteId, createOperationID, createInputDigest, createAssets); category != "" {
			if category == service.WorkspaceConflict {
				re.Msg = "conflict"
			} else {
				re.Msg = "saveFailed"
			}
			return c.RenderJSON(re)
		}
	}
	assetByIndex := make(map[int]string, len(createAssets))
	for _, asset := range createAssets {
		assetByIndex[asset.Index] = asset.AssetID
	}
	// Keep only request-owned asset identities.  For a stable create retry this
	// includes assets written by an earlier attempt; cleanup is safe because it
	// runs only when the note is proven not to exist.
	cleanupFiles := make([]info.NoteFile, len(noteOrContent.Files))
	copy(cleanupFiles, noteOrContent.Files)
	for i := range cleanupFiles {
		cleanupFiles[i].FileId = ""
		if assetID := assetByIndex[i]; assetID != "" && cleanupFiles[i].HasBody {
			cleanupFiles[i].FileId = assetID
		}
	}
	cleanupAfterCreateFailure := func() error {
		cleanupErr := attachService.CleanupAPINoteAssets(noteId.Hex(), userId.Hex(), cleanupFiles)
		if cleanupErr != nil {
			Log(cleanupErr.Error())
			// The pending receipt remains the recovery identity while either the
			// row or the file is uncertain.  Terminal failure is only safe after
			// cleanup has been verified.
			return cleanupErr
		}
		if createOperationID == "" {
			return nil
		}
		if category := noteService.FailNoteCreateAssetReceipt(userId, createOperationID); category != "" {
			return fmt.Errorf("close note create asset receipt: %s", category)
		}
		return nil
	}
	attachNum := 0
	if noteOrContent.Files != nil && len(noteOrContent.Files) > 0 {
		for i, file := range noteOrContent.Files {
			if file.HasBody {
				if file.LocalFileId != "" {
					// FileDatas[54c7ae27d98d0329dd000000]
					ok, msg, fileId := c.uploadWithAssetIdentity("FileDatas["+file.LocalFileId+"]", noteId.Hex(), file.IsAttach, assetByIndex[i])

					if !ok {
						re.Ok = false
						if msg != "" {
							Log(msg)
							Log(file.LocalFileId)
							if msg == apiUploadPartialWrite {
								re.Msg = apiUploadPartialWrite
							} else {
								re.Msg = "fileUploadError"
							}
						}
						// 报不是图片的错误没关系, 证明客户端传来非图片的数据
						if msg != "notImage" {
							if cleanupAfterCreateFailure() != nil {
								re.Msg = "partial_write"
							}
							return c.RenderJSON(re)
						}
					} else {
						// 建立映射
						file.FileId = fileId
						noteOrContent.Files[i] = file
						cleanupFiles[i] = file

						if file.IsAttach {
							attachNum++
						}
					}
				} else {
					if cleanupAfterCreateFailure() != nil {
						re.Msg = "partial_write"
					}
					return c.RenderJSON(re)
				}
			}
		}
	}

	c.fixPostNotecontent(&noteOrContent)

	//	Log("Add")
	//	LogJ(noteOrContent)

	//	return c.RenderJSON(re)

	note := info.Note{UserId: userId,
		NoteId:     noteId,
		NotebookId: db.MustObjectIDFromHex(noteOrContent.NotebookId),
		Title:      noteOrContent.Title,
		Tags:       noteOrContent.Tags,
		Desc:       noteOrContent.Desc,
		//		ImgSrc:     noteOrContent.ImgSrc,
		IsBlog:      noteOrContent.IsBlog,
		IsMarkdown:  noteOrContent.IsMarkdown,
		AttachNum:   attachNum,
		CreatedTime: noteOrContent.CreatedTime,
		UpdatedTime: noteOrContent.UpdatedTime,
	}
	noteContent := info.NoteContent{NoteId: note.NoteId,
		UserId:      userId,
		IsBlog:      note.IsBlog,
		Content:     noteOrContent.Content,
		Abstract:    noteOrContent.Abstract,
		CreatedTime: noteOrContent.CreatedTime,
		UpdatedTime: noteOrContent.UpdatedTime,
	}

	// 通过内容得到Desc, abstract
	if noteOrContent.Abstract == "" {
		note.Desc = SubStringHTMLToRaw(noteContent.Content, 200)
		noteContent.Abstract = SubStringHTML(noteContent.Content, 200, "")
	} else {
		note.Desc = SubStringHTMLToRaw(noteContent.Abstract, 200)
	}

	var ok bool
	var msg string
	committedAssets := make([]applicationnotes.OperationAsset, 0, len(noteOrContent.Files))
	for index, file := range noteOrContent.Files {
		if file.FileId == "" {
			continue
		}
		asset := applicationnotes.OperationAsset{AssetID: file.FileId, LocalFileID: file.LocalFileId, Index: index, IsAttach: file.IsAttach}
		if frozenID := assetByIndex[index]; frozenID != "" {
			for _, frozen := range createAssets {
				if frozen.AssetID == frozenID {
					asset.ContentSHA256 = frozen.ContentSHA256
					break
				}
			}
		}
		committedAssets = append(committedAssets, asset)
	}
	if createOperationID != "" {
		note, ok, msg = noteService.AddNoteAndContentApiResultWithIdentity(note, noteContent, myUserId, createOperationID, createInputDigest, committedAssets)
	} else {
		note, ok, msg = noteService.AddNoteAndContentApiResultWithAssets(note, noteContent, myUserId, committedAssets)
	}
	if !ok || note.NoteId.IsZero() {
		re.Ok = false
		re.Msg = nonEmptyAPIMessage(msg)
		// A non-zero note means the required create committed and a later
		// projection/repair failed; its assets are still live.  A zero result
		// is the only safe cleanup trigger, and the helper rechecks the store
		// for a possible partial note before deleting anything.
		if note.NoteId.IsZero() && cleanupAfterCreateFailure() != nil {
			re.Msg = "partial_write"
		}
		return c.RenderJSON(re)
	}
	if err := attachService.FinalizeAPINoteAssets(note.NoteId.Hex(), userId.Hex(), noteOrContent.Files); err != nil {
		Log(err.Error())
		re.Ok = false
		re.Msg = apiUploadPartialWrite
		return c.RenderJSON(re)
	}

	// 添加需要返回的
	noteOrContent.NoteId = note.NoteId.Hex()
	noteOrContent.Usn = note.Usn
	noteOrContent.CreatedTime = note.CreatedTime
	noteOrContent.UpdatedTime = note.UpdatedTime
	noteOrContent.UserId = c.getUserId()
	noteOrContent.IsMarkdown = note.IsMarkdown
	// 删除一些不要返回的, 删除Desc?
	noteOrContent.Content = ""
	noteOrContent.Abstract = ""
	//	apiNote := info.NoteToApiNote(note, noteOrContent.Files)
	return c.RenderJSON(noteOrContent)
}

func nonEmptyAPIMessage(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "saveFailed"
	}
	return msg
}

// apiNoteFilesPresent decodes the API update's three-state files contract at
// the request boundary. An absent collection leaves assets unchanged; either
// marker represents an explicit empty collection; indexed fields or a
// non-empty decoded collection represent complete replacement values.
func apiNoteFilesPresent(values url.Values, files []info.NoteFile) bool {
	if len(files) > 0 {
		return true
	}
	for key, fieldValues := range values {
		if key == "Files" || strings.HasPrefix(key, "Files[") {
			return true
		}
		if key != "FilesPresent" && key != "HasFiles" {
			continue
		}
		for _, value := range fieldValues {
			if value == "1" {
				return true
			}
		}
	}
	return false
}

// 更新笔记
// [OK]
func (c ApiNote) UpdateNote(noteOrContent info.ApiNote) revel.Result {
	re := info.NewReUpdate()

	noteUpdate := bson.M{}
	needUpdateNote := false

	noteId := noteOrContent.NoteId

	if noteOrContent.NoteId == "" {
		re.Msg = "noteIdNotExists"
		return c.RenderJSON(re)
	}

	if noteOrContent.Usn <= 0 {
		re.Msg = "usnNotExists"
		return c.RenderJSON(re)
	}

	//	Log("_____________")
	//	LogJ(noteOrContent)
	/*
		LogJ(c.Params.Files)
		LogJ(c.Request.Header)
		LogJ(c.Params.Values)
	*/

	// 先判断USN的问题, 因为很可能添加完附件后, 会有USN冲突, 这时附件就添错了
	userId := c.getUserId()
	note := noteService.GetNote(noteId, userId)
	if note.NoteId.IsZero() {
		re.Msg = "notExists"
		return c.RenderJSON(re)
	}
	if c.Has("NotebookId") && (!db.IsValidObjectIDHex(noteOrContent.NotebookId) || !noteService.CanCreateNote(userId, userId, noteOrContent.NotebookId)) {
		re.Msg = "notebookIdNotExists"
		return c.RenderJSON(re)
	}

	var assetWork *applicationnotes.AssetMutation
	filesPresent := apiNoteFilesPresent(c.Params.Values, noteOrContent.Files)
	if filesPresent {
		contentDigests := make(map[int]string, len(noteOrContent.Files))
		for i, file := range noteOrContent.Files {
			if !file.HasBody || file.LocalFileId == "" {
				continue
			}
			if _, present := c.Params.Files["FileDatas["+file.LocalFileId+"]"]; present {
				candidate, message := c.prepareAPINoteAsset("FileDatas["+file.LocalFileId+"]", noteId, file.IsAttach, "")
				if message != "" {
					re.Msg = message
					return c.RenderJSON(re)
				}
				contentDigests[i] = candidate.Digest
			}
		}
		assetSeed := stableAPIUpdateAssetSeedWithDigests(db.MustObjectIDFromHex(userId), note.NoteId, noteOrContent.Usn, noteOrContent, contentDigests)
		if assetSeed == "" {
			re.Msg = "saveFailed"
			return c.RenderJSON(re)
		}
		files := append([]info.NoteFile(nil), noteOrContent.Files...)
		assetByIndex := make(map[int]string, len(files))
		for i, file := range files {
			if !file.HasBody {
				continue
			}
			if file.LocalFileId == "" {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			assetID := stableAPIAssetID(assetSeed, file.LocalFileId, i, file.IsAttach)
			file.FileId = assetID
			files[i] = file
			assetByIndex[i] = assetID
		}
		assets := make([]applicationnotes.OperationAsset, 0, len(files))
		for i, file := range files {
			if file.FileId == "" || !db.IsValidObjectIDHex(file.FileId) {
				re.Msg = "fileRequired"
				return c.RenderJSON(re)
			}
			asset := applicationnotes.OperationAsset{AssetID: file.FileId, Index: i, IsAttach: file.IsAttach}
			if file.HasBody {
				asset.LocalFileID = file.LocalFileId
				asset.ContentSHA256 = contentDigests[i]
			}
			assets = append(assets, asset)
		}
		noteOrContent.Files = files
		assetWork = &applicationnotes.AssetMutation{
			Assets: assets,
			Apply: func(ctx context.Context, expectedUSN int) error {
				for i, file := range files {
					if !file.HasBody {
						continue
					}
					ok, msg, _ := c.uploadWithAssetIdentity("FileDatas["+file.LocalFileId+"]", noteId, file.IsAttach, assetByIndex[i])
					if !ok {
						if msg == "" {
							msg = "fileUploadError"
						}
						return fmt.Errorf("upload asset: %s", msg)
					}
				}
				if err := service.ContentAssets.ReconcileNote(ctx, applicationnotes.ReconcileNoteAssetsCommand{
					OperationID: assetWork.OperationID, ActorID: db.MustObjectIDFromHex(userId), OwnerID: note.UserId,
					NoteID: note.NoteId, Generation: expectedUSN,
				}); err != nil {
					return err
				}
				return attachService.FinalizeAPINoteAssets(note.NoteId.Hex(), note.UserId.Hex(), files)
			},
			Verify: func(ctx context.Context) (bool, error) {
				return service.ContentAssets.VerifyReconcileNote(ctx, applicationnotes.ReconcileNoteAssetsCommand{
					OperationID: assetWork.OperationID, ActorID: db.MustObjectIDFromHex(userId), OwnerID: note.UserId,
					NoteID: note.NoteId,
				})
			},
		}
	}

	// Desc前台传来
	if c.Has("Desc") {
		needUpdateNote = true
		noteUpdate["Desc"] = noteOrContent.Desc
	}
	/*
		if c.Has("ImgSrc") {
			needUpdateNote = true
			noteUpdate["ImgSrc"] = noteOrContent.ImgSrc
		}
	*/
	if c.Has("Title") {
		needUpdateNote = true
		noteUpdate["Title"] = noteOrContent.Title
	}
	if c.Has("IsTrash") {
		needUpdateNote = true
		noteUpdate["IsTrash"] = noteOrContent.IsTrash
	}

	// 是否是博客
	if c.Has("IsBlog") {
		needUpdateNote = true
		noteUpdate["IsBlog"] = noteOrContent.IsBlog
	}

	/*
		Log(c.Has("tags[0]"))
		Log(c.Has("Tags[]"))
		for key, v := range c.Params.Values {
			Log(key)
			Log(v)
		}
	*/

	if c.Has("Tags[0]") {
		needUpdateNote = true
		noteUpdate["Tags"] = noteOrContent.Tags
	}

	if c.Has("NotebookId") {
		if db.IsValidObjectIDHex(noteOrContent.NotebookId) {
			needUpdateNote = true
			noteUpdate["NotebookId"] = db.MustObjectIDFromHex(noteOrContent.NotebookId)
		}
	}

	if c.Has("Content") {
		// 通过内容得到Desc, 如果有Abstract, 则用Abstract生成Desc
		if noteOrContent.Abstract == "" {
			noteUpdate["Desc"] = SubStringHTMLToRaw(noteOrContent.Content, 200)
		} else {
			noteUpdate["Desc"] = SubStringHTMLToRaw(noteOrContent.Abstract, 200)
		}
	}

	noteUpdate["UpdatedTime"] = noteOrContent.UpdatedTime

	var content *string
	var abstract *string
	if c.Has("Content") {
		// 把fileId替换下
		c.fixPostNotecontent(&noteOrContent)
		// 如果传了Abstract就用之
		if noteOrContent.Abstract == "" {
			noteOrContent.Abstract = SubStringHTML(noteOrContent.Content, 200, "")
		}

		//		Log("--------> afte fixed")
		//		Log(noteOrContent.Content)
		content = &noteOrContent.Content
		abstract = &noteOrContent.Abstract
	}
	if !needUpdateNote {
		noteUpdate = nil
	}
	expectedUSN := noteOrContent.Usn
	result := noteService.SaveNote(service.SaveNoteCommand{
		ActorUserID: c.getUserId(),
		NoteID:      noteOrContent.NoteId,
		ExpectedUSN: &expectedUSN,
		Metadata:    noteUpdate,
		Content:     content,
		Abstract:    abstract,
		UpdatedTime: noteOrContent.UpdatedTime,
		AssetWork:   assetWork,
	})
	re.Ok = result.OK()
	re.Msg = workspaceAPIMessage(result.Error)
	re.Usn = result.USN

	if !re.Ok {
		return c.RenderJSON(re)
	}
	noteOrContent.Content = ""
	noteOrContent.Usn = re.Usn
	noteOrContent.UpdatedTime = time.Now()

	//	Log("after upload")
	//	LogJ(noteOrContent.Files)
	noteOrContent.UserId = c.getUserId()

	return c.RenderJSON(noteOrContent)
}

func workspaceAPIMessage(category service.WorkspaceErrorCategory) string {
	switch category {
	case service.WorkspaceNotFound:
		return "notExists"
	case service.WorkspaceUnauthorized:
		return "noAuth"
	default:
		return string(category)
	}
}

// 删除trash
func (c ApiNote) DeleteTrash(noteId string, usn int) revel.Result {
	re := info.NewReUpdate()
	re.Ok, re.Msg, re.Usn = trashService.DeleteTrashApi(noteId, c.getUserId(), usn)
	return c.RenderJSON(re)
}

// 得到历史列表
/*
func (c ApiNote) GetHistories(noteId string) revel.Result {
	re := info.NewRe()
	histories := noteContentHistoryService.ListHistories(noteId, c.getUserId())
	if len(histories) > 0 {
		re.Ok = true
		re.Item = histories
	}
	return c.RenderJSON(re)
}
*/

// 0.2 新增
// 导出成PDF
func (c ApiNote) ExportPdf(noteId string) revel.Result {
	re := info.NewApiRe()
	actorID, actorErr := domain.ParseObjectID(c.getUserId())
	noteID, noteErr := domain.ParseObjectID(noteId)
	if actorErr != nil || noteErr != nil || actorID.IsZero() || noteID.IsZero() {
		re.Msg = "noteNotExists"
		return c.RenderJSON(re)
	}
	if service.ContentPDF == nil {
		re.Msg = "sysError"
		return c.RenderJSON(re)
	}
	artifact, err := service.ContentPDF.Export(c.RequestContext(), actorID, noteID)
	if err != nil {
		var contentErr *applicationcontent.Error
		if errors.As(err, &contentErr) && (contentErr.Category == applicationcontent.ErrorUnauthorized || contentErr.Category == applicationcontent.ErrorNotFound) {
			re.Msg = "noteNotExists"
			return c.RenderJSON(re)
		}
		re.Msg = "sysError"
		return c.RenderJSON(re)
	}
	return c.RenderBinary(bytes.NewReader(artifact.Data), artifact.Filename, revel.Attachment, time.Now())
}
