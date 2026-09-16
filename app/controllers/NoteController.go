package controllers

import (
	"bytes"
	"errors"

	"github.com/revel/revel"
	//	"encoding/json"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"strings"
	"time"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"bytes"
	//	"os"
)

type Note struct {
	BaseController
}

// 笔记首页, 判断是否已登录
// 已登录, 得到用户基本信息(notebook, shareNotebook), 跳转到index.html中
// 否则, 转向登录页面
func (c Note) Index(noteId, online string) revel.Result {
	c.SetLocale()
	userInfo := c.GetUserAndBlogUrl()

	userId := userInfo.UserId.Hex()

	// 没有登录
	if userId == "" {
		return c.Redirect("/login")
	}

	c.ViewArgs["openRegister"] = configService.IsOpenRegister()

	// 已登录了, 那么得到所有信息
	notebooks := notebookService.GetNotebooks(userId)
	shareNotebooks, sharedUserInfos := shareService.GetShareNotebooks(userId)

	// 还需要按时间排序(DESC)得到notes
	notes := []info.Note{}
	noteContent := info.NoteContent{}

	if len(notebooks) > 0 {
		// noteId是否存在
		// 是否传入了正确的noteId
		hasRightNoteId := false
		if IsObjectId(noteId) {
			note := noteService.GetNoteById(noteId)

			if !note.NoteId.IsZero() {
				var noteOwner = note.UserId.Hex()
				noteContent = noteService.GetNoteContent(noteId, noteOwner)

				hasRightNoteId = true
				c.ViewArgs["curNoteId"] = noteId
				c.ViewArgs["curNotebookId"] = note.NotebookId.Hex()

				// 打开的是共享的笔记, 那么判断是否是共享给我的默认笔记
				if noteOwner != c.GetUserId() {
					if shareService.HasReadPerm(noteOwner, c.GetUserId(), noteId) {
						// 不要获取notebook下的笔记
						// 在前端下发请求
						c.ViewArgs["curSharedNoteNotebookId"] = note.NotebookId.Hex()
						c.ViewArgs["curSharedUserId"] = noteOwner
						// 没有读写权限
					} else {
						hasRightNoteId = false
					}
				} else {
					_, notes = noteService.ListNotes(c.GetUserId(), note.NotebookId.Hex(), false, c.GetPage(), 50, defaultSortField, false, false)

					// 如果指定了某笔记, 则该笔记放在首位
					lenNotes := len(notes)
					if lenNotes > 1 {
						notes2 := make([]info.Note, len(notes))
						notes2[0] = note
						i := 1
						for _, note := range notes {
							if note.NoteId.Hex() != noteId {
								if i == lenNotes { // 防止越界
									break
								}
								notes2[i] = note
								i++
							}
						}
						notes = notes2
					}
				}
			}

			// 得到最近的笔记
			_, latestNotes := noteService.ListNotes(c.GetUserId(), "", false, c.GetPage(), 50, defaultSortField, false, false)
			c.ViewArgs["latestNotes"] = latestNotes
		}

		// 没有传入笔记
		// 那么得到最新笔记
		if !hasRightNoteId {
			_, notes = noteService.ListNotes(c.GetUserId(), "", false, c.GetPage(), 50, defaultSortField, false, false)
			if len(notes) > 0 {
				noteContent = noteService.GetNoteContent(notes[0].NoteId.Hex(), userId)
				c.ViewArgs["curNoteId"] = notes[0].NoteId.Hex()
			}
		}
	}

	// 当然, 还需要得到第一个notes的content
	//...
	c.ViewArgs["isAdmin"] = configService.GetAdminUsername() == userInfo.Username

	c.ViewArgs["userInfo"] = userInfo
	c.ViewArgs["notebooks"] = notebooks
	c.ViewArgs["shareNotebooks"] = shareNotebooks // note信息在notes列表中
	c.ViewArgs["sharedUserInfos"] = sharedUserInfos

	c.ViewArgs["notes"] = notes
	c.ViewArgs["noteContentJson"] = noteContent
	c.ViewArgs["noteContent"] = noteContent.Content

	c.ViewArgs["tags"] = tagService.GetTags(c.GetUserId())

	c.ViewArgs["globalConfigs"] = configService.GetGlobalConfigForUser()

	// return c.RenderTemplate("note/note.html")

	if isDev, _ := revel.Config.Bool("mode.dev"); isDev && online == "" {
		return c.RenderTemplate("note/note-dev.html")
	} else {
		return c.RenderTemplate("note/note.html")
	}
}

// 首页, 判断是否已登录
// 已登录, 得到用户基本信息(notebook, shareNotebook), 跳转到index.html中
// 否则, 转向登录页面
func (c Note) ListNotes(notebookId string) revel.Result {
	_, notes := noteService.ListNotes(c.GetUserId(), notebookId, false, c.GetPage(), pageSize, defaultSortField, false, false)
	return c.RenderJSON(notes)
}

// 得到trash
func (c Note) ListTrashNotes() revel.Result {
	_, notes := noteService.ListNotes(c.GetUserId(), "", true, c.GetPage(), pageSize, defaultSortField, false, false)
	return c.RenderJSON(notes)
}

// 得到note和内容
func (c Note) GetNoteAndContent(noteId string) revel.Result {
	return c.RenderJSON(noteService.GetNoteAndContent(noteId, c.GetUserId()))
}

func (c Note) GetNoteAndContentBySrc(src string) revel.Result {
	noteId, noteAndContent := noteService.GetNoteAndContentBySrc(src, c.GetUserId())
	ret := info.Re{}
	if noteId != "" {
		ret.Ok = true
		ret.Item = noteAndContent
	}
	return c.RenderJSON(ret)
}

// 得到内容
func (c Note) GetNoteContent(noteId string) revel.Result {
	noteContent := noteService.GetNoteContent(noteId, c.GetUserId())
	return c.RenderJSON(noteContent)
}

// 这里不能用json, 要用post
func (c Note) UpdateNoteOrContent(noteOrContent info.NoteOrContent) revel.Result {
	re := info.NewRe()

	// 新添加note
	if noteOrContent.IsNew {
		if !db.IsValidObjectIDHex(noteOrContent.NoteId) {
			re.Msg = "noteIdNotExists"
			return c.RenderJSON(re)
		}
		if !db.IsValidObjectIDHex(noteOrContent.NotebookId) {
			re.Msg = "notebookIdNotExists"
			return c.RenderJSON(re)
		}
		if noteOrContent.FromUserId != "" && !db.IsValidObjectIDHex(noteOrContent.FromUserId) {
			re.Msg = "noAuth"
			return c.RenderJSON(re)
		}
		userId := c.GetObjectUserId()
		//		myUserId := userId
		// 为共享新建?
		if noteOrContent.FromUserId != "" {
			userId = db.MustObjectIDFromHex(noteOrContent.FromUserId)
		}

		note := info.Note{UserId: userId,
			NoteId:     db.MustObjectIDFromHex(noteOrContent.NoteId),
			NotebookId: db.MustObjectIDFromHex(noteOrContent.NotebookId),
			Title:      noteOrContent.Title,
			Src:        noteOrContent.Src, // 来源
			Tags:       strings.Split(noteOrContent.Tags, ","),
			Desc:       noteOrContent.Desc,
			ImgSrc:     noteOrContent.ImgSrc,
			IsBlog:     noteOrContent.IsBlog,
			IsMarkdown: noteOrContent.IsMarkdown,
		}
		noteContent := info.NoteContent{NoteId: note.NoteId,
			UserId:   userId,
			IsBlog:   note.IsBlog,
			Content:  noteOrContent.Content,
			Abstract: noteOrContent.Abstract}

		createdNote, ok, msg := noteService.AddNoteAndContentForControllerResult(note, noteContent, c.GetUserId())
		re.Ok = ok
		if !ok {
			re.Msg = nonEmptySaveMessage(msg)
			return c.RenderJSON(re)
		}
		re.Item = createdNote
		return c.RenderJSON(re)
	}

	noteUpdate := bson.M{}
	needUpdateNote := false

	// Desc前台传来
	if c.Has("Desc") {
		needUpdateNote = true
		noteUpdate["Desc"] = noteOrContent.Desc
	}
	if c.Has("ImgSrc") {
		needUpdateNote = true
		noteUpdate["ImgSrc"] = noteOrContent.ImgSrc
	}
	if c.Has("Title") {
		needUpdateNote = true
		noteUpdate["Title"] = noteOrContent.Title
	}

	if c.Has("Tags") {
		needUpdateNote = true
		noteUpdate["Tags"] = strings.Split(noteOrContent.Tags, ",")
	}

	var content *string
	var abstract *string
	if c.Has("Content") {
		content = &noteOrContent.Content
		abstract = &noteOrContent.Abstract
	}
	if !needUpdateNote {
		noteUpdate = nil
	}
	var expectedUSN *int
	if c.Has("ExpectedUsn") {
		expected := noteOrContent.ExpectedUsn
		expectedUSN = &expected
	}
	result := noteService.SaveNote(service.SaveNoteCommand{
		ActorUserID: c.GetUserId(), OperationID: noteOrContent.OperationId,
		NoteID: noteOrContent.NoteId, ExpectedUSN: expectedUSN, Metadata: noteUpdate,
		Content: content, Abstract: abstract, UpdatedTime: time.Now(),
	})
	re.Ok = result.OK()
	if !re.Ok {
		re.Msg = workspaceWebSaveMessage(result.Error)
	}
	return c.RenderJSON(re)
}

func workspaceWebSaveMessage(category service.WorkspaceErrorCategory) string {
	switch category {
	case service.WorkspaceNotFound:
		return "notExists"
	case service.WorkspaceUnauthorized:
		return "noAuth"
	default:
		return nonEmptySaveMessage(string(category))
	}
}

func nonEmptySaveMessage(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "saveFailed"
	}
	return msg
}

// 删除note/ 删除别人共享给我的笔记
// userId 是note.UserId
func (c Note) DeleteNote(noteIds []string, isShared bool) revel.Result {
	return c.RenderJSON(noteService.DeleteNotesWithOperation(c.GetUserId(), noteIds, isShared, c.Params.Get("OperationId")).OK())
}

// 删除trash, 已弃用, 用DeleteNote
func (c Note) DeleteTrash(noteId string) revel.Result {
	return c.RenderJSON(trashService.DeleteTrash(noteId, c.GetUserId()))
}

// 移动note
func (c Note) MoveNote(noteIds []string, notebookId string) revel.Result {
	return c.RenderJSON(noteService.MoveNotesWithOperation(c.GetUserId(), noteIds, notebookId, c.Params.Get("OperationId")).OK())
}

// 复制note
func (c Note) CopyNote(noteIds []string, notebookId string) revel.Result {
	result := noteService.CopyNotesWithOperation(c.GetUserId(), noteIds, notebookId, c.Params.Get("OperationId"))
	re := info.NewRe()
	re.Ok = result.OK()
	re.Item = result.Notes
	return c.RenderJSON(re)
}

// 复制别人共享的笔记给我
func (c Note) CopySharedNote(noteIds []string, notebookId, fromUserId string) revel.Result {
	result := noteService.CopySharedNotesWithOperation(c.GetUserId(), noteIds, notebookId, fromUserId, c.Params.Get("OperationId"))
	re := info.NewRe()
	re.Ok = result.OK()
	re.Item = result.Notes
	return c.RenderJSON(re)
}

// ------------
// search
// 通过title搜索
func (c Note) SearchNote(key string) revel.Result {
	_, blogs := noteService.SearchNote(key, c.GetUserId(), c.GetPage(), pageSize, "UpdatedTime", false, false)
	return c.RenderJSON(blogs)
}

// 通过tags搜索
func (c Note) SearchNoteByTags(tags []string) revel.Result {
	_, blogs := noteService.SearchNoteByTags(tags, c.GetUserId(), c.GetPage(), pageSize, "UpdatedTime", false)
	return c.RenderJSON(blogs)
}

// 生成PDF
func (c Note) ToPdf(noteId, appKey string) revel.Result {
	// Retained only for wire binding until interface-http freezes the legacy
	// route. appKey is never a renderer credential or callback secret.
	_, _ = noteId, appKey
	return c.RenderText("no note")
}

// 导出成PDF
func (c Note) ExportPdf(noteId string) revel.Result {
	actorID, actorErr := domain.ParseObjectID(c.GetUserId())
	noteID, noteErr := domain.ParseObjectID(noteId)
	if actorErr != nil || noteErr != nil || actorID.IsZero() || noteID.IsZero() || service.ContentPDF == nil {
		return c.RenderText("error")
	}
	artifact, err := service.ContentPDF.Export(c.RequestContext(), actorID, noteID)
	if err != nil {
		var contentErr *applicationcontent.Error
		if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorUnauthorized {
			return c.RenderText("No Perm")
		}
		return c.RenderText("error")
	}
	return c.RenderBinary(bytes.NewReader(artifact.Data), artifact.Filename, revel.Attachment, time.Now())
}

// 设置/取消Blog; 置顶
func (c Note) SetNote2Blog(noteIds []string, isBlog, isTop bool) revel.Result {
	for _, noteId := range noteIds {
		noteService.ToBlog(c.GetUserId(), noteId, isBlog, isTop)
	}
	return c.RenderJSON(true)
}
