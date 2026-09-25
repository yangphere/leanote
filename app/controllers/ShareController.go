package controllers

import (
	"errors"
	"fmt"
	"time"

	"github.com/revel/revel"
	//	"encoding/json"
	//	"go.mongodb.org/mongo-driver/v2/bson"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"fmt"
)

type Share struct {
	BaseController
}

func (c Share) shareReadErrorResult(err error) revel.Result {
	if errors.Is(err, service.ErrShareResource) {
		c.Response.Status = 404
		return c.RenderText("not found")
	}
	c.Response.Status = 500
	return c.RenderText("internal server error")
}

func (c Share) shareGrantOptions() (service.ShareGrantOptions, error) {
	options := service.ShareGrantOptions{Now: time.Now().UTC()}
	if c.Params == nil {
		return options, nil
	}
	expiresValues, hasExpires := c.Params.Values["expiresAt"]
	clearValues, hasClear := c.Params.Values["clearExpiresAt"]
	if (hasExpires && len(expiresValues) != 1) || (hasClear && len(clearValues) != 1) || len(expiresValues) > 1 || len(clearValues) > 1 {
		return options, fmt.Errorf("%w: repeated expiry parameter", service.ErrInvalidShareGrant)
	}
	if hasClear && (len(clearValues) != 1 || clearValues[0] != "true") {
		return options, fmt.Errorf("%w: clearExpiresAt must be true", service.ErrInvalidShareGrant)
	}
	if hasExpires {
		expiresAt, err := service.ParseShareExpiry(expiresValues[0], true, options.Now)
		if err != nil {
			return options, err
		}
		options.ExpiresAt = expiresAt
	}
	options.ClearExpiresAt = hasClear
	if options.ExpiresAt != nil && options.ClearExpiresAt {
		return options, fmt.Errorf("%w: expiresAt and clearExpiresAt are mutually exclusive", service.ErrInvalidShareGrant)
	}
	return options, nil
}

// 添加共享note
func (c Share) AddShareNote(noteId string, emails []string, perm int) revel.Result {
	status := make(map[string]info.Re, len(emails))
	options, optionsErr := c.shareGrantOptions()
	if optionsErr != nil {
		for _, email := range emails {
			status[email] = info.Re{Ok: false, Msg: optionsErr.Error()}
		}
		return c.RenderJSON(status)
	}
	// 自己不能给自己添加共享
	myEmail := c.GetEmail()
	validRecipientIDs := make([]string, 0, len(emails))
	for _, email := range emails {
		if email != "" && email != myEmail {
			if userId := userService.GetUserId(email); userId != "" {
				validRecipientIDs = append(validRecipientIDs, userId)
			}
		}
	}
	if options.ClearExpiresAt && len(validRecipientIDs) > 0 {
		if err := shareService.ValidateShareNoteBatch(noteId, c.GetUserId(), validRecipientIDs, options); err != nil {
			for _, email := range emails {
				status[email] = info.Re{Ok: false, Msg: err.Error()}
			}
			return c.RenderJSON(status)
		}
	}
	for _, email := range emails {
		if email == "" {
			continue
		}
		if myEmail != email {
			userId := userService.GetUserId(email)
			if userId == "" {
				status[email] = info.Re{Ok: false, Msg: "无该用户"}
				continue
			}
			err := shareService.AddShareNoteToUserIdWithOptions(noteId, perm, c.GetUserId(), userId, options)
			ok, msg := err == nil, ""
			if err != nil {
				msg = err.Error()
			}
			status[email] = info.Re{Ok: ok, Msg: msg, Id: userId}

		} else {
			status[email] = info.Re{Ok: false, Msg: "不能分享给自己"}
		}
	}

	return c.RenderJSON(status)
}

// 添加共享notebook
func (c Share) AddShareNotebook(notebookId string, emails []string, perm int) revel.Result {
	status := make(map[string]info.Re, len(emails))
	options, optionsErr := c.shareGrantOptions()
	if optionsErr != nil {
		for _, email := range emails {
			status[email] = info.Re{Ok: false, Msg: optionsErr.Error()}
		}
		return c.RenderJSON(status)
	}
	// 自己不能给自己添加共享
	myEmail := c.GetEmail()
	validRecipientIDs := make([]string, 0, len(emails))
	for _, email := range emails {
		if email != "" && email != myEmail {
			if userId := userService.GetUserId(email); userId != "" {
				validRecipientIDs = append(validRecipientIDs, userId)
			}
		}
	}
	if options.ClearExpiresAt && len(validRecipientIDs) > 0 {
		if err := shareService.ValidateShareNotebookBatch(notebookId, c.GetUserId(), validRecipientIDs, options); err != nil {
			for _, email := range emails {
				status[email] = info.Re{Ok: false, Msg: err.Error()}
			}
			return c.RenderJSON(status)
		}
	}
	for _, email := range emails {
		if email == "" {
			continue
		}
		if myEmail != email {
			userId := userService.GetUserId(email)
			if userId == "" {
				status[email] = info.Re{Ok: false, Msg: "无该用户"}
				continue
			}
			err := shareService.AddShareNotebookToUserIdWithOptions(notebookId, perm, c.GetUserId(), userId, options)
			ok, msg := err == nil, ""
			if err != nil {
				msg = err.Error()
			}
			status[email] = info.Re{Ok: ok, Msg: msg, Id: userId}
		} else {
			status[email] = info.Re{Ok: false, Msg: "不能分享给自己"}
		}
	}

	return c.RenderJSON(status)
}

// 得到notes
// userId 该userId分享给我的
func (c Share) ListShareNotes(notebookId, userId string) revel.Result {
	// 表示是默认笔记本, 不是某个特定notebook的共享
	if notebookId == "" {
		notes, err := shareService.ListShareNotesChecked(c.GetUserId(), userId, c.GetPage(), pageSize, defaultSortField, false)
		if err != nil {
			return c.shareReadErrorResult(err)
		}
		return c.RenderJSON(notes)
	} else {
		// 有notebookId的
		notes, err := shareService.ListShareNotesByNotebookIdChecked(notebookId, c.GetUserId(), userId, c.GetPage(), pageSize, defaultSortField, false)
		if err != nil {
			return c.shareReadErrorResult(err)
		}
		return c.RenderJSON(notes)
	}
}

// 得到内容
// sharedUserId 是谁的笔记
func (c Share) GetShareNoteContent(noteId, sharedUserId string) revel.Result {
	noteContent, err := shareService.GetShareNoteContentChecked(noteId, c.GetUserId(), sharedUserId)
	if err != nil {
		return c.shareReadErrorResult(err)
	}
	return c.RenderJSON(noteContent)
}

// 查看note的分享信息
// 分享给了哪些用户和权限
// ShareNotes表 userId = me, noteId = ...
// 还要查看该note的notebookId分享的信息
func (c Share) ListNoteShareUserInfo(noteId string) revel.Result {
	note := noteService.GetNote(noteId, c.GetUserId())

	noteShareUserInfos := shareService.ListNoteShareUserInfo(noteId, c.GetUserId())
	c.ViewArgs["noteOrNotebookShareUserInfos"] = noteShareUserInfos

	c.ViewArgs["noteOrNotebookShareGroupInfos"] = shareService.GetNoteShareGroups(noteId, c.GetUserId())

	c.ViewArgs["isNote"] = true
	c.ViewArgs["noteOrNotebookId"] = note.NoteId.Hex()
	c.ViewArgs["title"] = note.Title

	return c.RenderTemplate("share/note_notebook_share_user_infos.html")
}
func (c Share) ListNotebookShareUserInfo(notebookId string) revel.Result {
	notebook := notebookService.GetNotebook(notebookId, c.GetUserId())

	notebookShareUserInfos := shareService.ListNotebookShareUserInfo(notebookId, c.GetUserId())
	c.ViewArgs["noteOrNotebookShareUserInfos"] = notebookShareUserInfos

	c.ViewArgs["noteOrNotebookShareGroupInfos"] = shareService.GetNotebookShareGroups(notebookId, c.GetUserId())
	LogJ(c.ViewArgs["noteOrNotebookShareGroupInfos"])

	c.ViewArgs["isNote"] = false
	c.ViewArgs["noteOrNotebookId"] = notebook.NotebookId.Hex()
	c.ViewArgs["title"] = notebook.Title

	return c.RenderTemplate("share/note_notebook_share_user_infos.html")
}

// ------------
// 改变share note 权限
func (c Share) UpdateShareNotePerm(noteId string, perm int, toUserId string) revel.Result {
	options, err := c.shareGrantOptions()
	if err != nil {
		return c.RenderJSON(false)
	}
	return c.RenderJSON(shareService.UpdateShareNotePermWithOptions(noteId, perm, c.GetUserId(), toUserId, options) == nil)
}

// 改变share notebook 权限
func (c Share) UpdateShareNotebookPerm(notebookId string, perm int, toUserId string) revel.Result {
	options, err := c.shareGrantOptions()
	if err != nil {
		return c.RenderJSON(false)
	}
	return c.RenderJSON(shareService.UpdateShareNotebookPermWithOptions(notebookId, perm, c.GetUserId(), toUserId, options) == nil)
}

// ---------------
// 删除share note
func (c Share) DeleteShareNote(noteId string, toUserId string) revel.Result {
	return c.RenderJSON(shareService.DeleteShareNote(noteId, c.GetUserId(), toUserId))
}

// 删除share notebook
func (c Share) DeleteShareNotebook(notebookId string, toUserId string) revel.Result {
	return c.RenderJSON(shareService.DeleteShareNotebook(notebookId, c.GetUserId(), toUserId))
}

// 删除share note, 被共享方删除
func (c Share) DeleteShareNoteBySharedUser(noteId string, fromUserId string) revel.Result {
	return c.RenderJSON(shareService.DeleteShareNote(noteId, fromUserId, c.GetUserId()))
}

// 删除share notebook, 被共享方删除
func (c Share) DeleteShareNotebookBySharedUser(notebookId string, fromUserId string) revel.Result {
	return c.RenderJSON(shareService.DeleteShareNotebook(notebookId, fromUserId, c.GetUserId()))
}

// 删除fromUserId分享给我的所有note, notebook
func (c Share) DeleteUserShareNoteAndNotebook(fromUserId string) revel.Result {
	return c.RenderJSON(shareService.DeleteUserShareNoteAndNotebook(fromUserId, c.GetUserId()))
}

//-------------
// 用户组

// 将笔记分享给分组
func (c Share) AddShareNoteGroup(noteId, groupId string, perm int) revel.Result {
	re := info.NewRe()
	options, err := c.shareGrantOptions()
	if err != nil {
		re.Msg = err.Error()
		return c.RenderJSON(re)
	}
	re.Ok = shareService.AddShareNoteGroupWithOptions(c.GetUserId(), noteId, groupId, perm, options) == nil
	return c.RenderJSON(re)
}

// 删除
func (c Share) DeleteShareNoteGroup(noteId, groupId string) revel.Result {
	re := info.NewRe()
	re.Ok = shareService.DeleteShareNoteGroup(c.GetUserId(), noteId, groupId)
	return c.RenderJSON(re)
}

// 更新, 也是一样, 先删后加
func (c Share) UpdateShareNoteGroupPerm(noteId, groupId string, perm int) revel.Result {
	return c.AddShareNoteGroup(noteId, groupId, perm)
}

//------

// 将笔记分享给分组
func (c Share) AddShareNotebookGroup(notebookId, groupId string, perm int) revel.Result {
	re := info.NewRe()
	options, err := c.shareGrantOptions()
	if err != nil {
		re.Msg = err.Error()
		return c.RenderJSON(re)
	}
	re.Ok = shareService.AddShareNotebookGroupWithOptions(c.GetUserId(), notebookId, groupId, perm, options) == nil
	return c.RenderJSON(re)
}

// 删除
func (c Share) DeleteShareNotebookGroup(notebookId, groupId string) revel.Result {
	re := info.NewRe()
	re.Ok = shareService.DeleteShareNotebookGroup(c.GetUserId(), notebookId, groupId)
	return c.RenderJSON(re)
}

// 更新, 也是一样, 先删后加
func (c Share) UpdateShareNotebookGroupPerm(notebookId, groupId string, perm int) revel.Result {
	return c.AddShareNotebookGroup(notebookId, groupId, perm)
}
