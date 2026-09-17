package controllers

import (
	"github.com/revel/revel"
	//	"encoding/json"
	"fmt"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
	"strings"
	"time"
)

// 附件
type Attach struct {
	BaseController
}

// 上传附件
func (c Attach) UploadAttach(noteId string) revel.Result {
	re := c.uploadAttach(noteId)
	return c.RenderJSON(re)
}
func (c Attach) uploadAttach(noteId string) (re info.Re) {
	var fileId = ""
	var resultMsg = "error" // 错误信息
	var Ok = false
	var fileInfo info.Attach

	re = info.NewRe()

	defer func() {
		re.Id = fileId // 只是id, 没有其它信息
		re.Msg = resultMsg
		re.Ok = Ok
		re.Item = fileInfo
	}()

	files := c.Params.Files["file"]
	if len(files) != 1 {
		return re
	}
	// > 5M?
	maxFileSize, err := configService.GetUploadLimitBytes("uploadAttachSize")
	if err != nil {
		resultMsg = "upload config error"
		return re
	}
	reader, err := files[0].Open()
	if err != nil {
		return re
	}

	handel := files[0]
	clientOperationID := strings.TrimSpace(c.Params.Get("OperationId"))
	fileInfo, Ok, resultMsg = attachService.UploadWebAttachment(service.WebAttachmentUploadInput{
		ActorID: c.GetUserId(), NoteID: noteId, OriginalFilename: handel.Filename,
		Reader: reader, Limit: maxFileSize, OperationID: clientOperationID,
	})
	if resultMsg == "too large" {
		resultMsg = fmt.Sprintf("The file's size is bigger than %vM", float64(maxFileSize)/(1024*1024))
	}
	fileId = fileInfo.AttachId.Hex()
	if resultMsg != "" {
		resultMsg = c.Message(resultMsg)
	}

	if Ok {
		resultMsg = "success"
	}
	return re
}

// 删除附件
func (c Attach) DeleteAttach(attachId string) revel.Result {
	re := info.NewRe()
	re.Ok, re.Msg = attachService.DeleteAttachWithOperation(attachId, c.GetUserId(), strings.TrimSpace(c.Params.Get("OperationId")))
	return c.RenderJSON(re)
}

// get all attachs by noteId
func (c Attach) GetAttachs(noteId string) revel.Result {
	re := info.NewRe()
	attachments, _, err := attachService.ListReadable(c.RequestContext(), c.GetUserId(), noteId)
	if err != nil {
		re.Msg = "error"
		return c.RenderJSON(re)
	}
	re.Ok = true
	re.List = attachments
	return c.RenderJSON(re)
}

// 下载附件
// 权限判断
func (c Attach) Download(attachId string) revel.Result {
	download, err := attachService.OpenReadable(c.RequestContext(), c.GetUserId(), attachId)
	if err != nil {
		return c.RenderText("")
	}
	filename := applicationcontent.AttachmentDownloadFilename(download.DisplayName)
	return c.RenderBinary(download.Reader, filename, revel.Attachment, time.Now())
}

func (c Attach) DownloadAll(noteId string) revel.Result {
	archive, title, err := attachService.OpenReadableArchive(c.RequestContext(), c.GetUserId(), noteId)
	if err != nil {
		return c.RenderText("")
	}
	filename := applicationcontent.ArchiveDownloadFilename(title)
	return c.RenderBinary(archive, filename, revel.Attachment, time.Now())
}
