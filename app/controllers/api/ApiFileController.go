package api

import (
	"strings"
	"time"

	"github.com/revel/revel"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
)

// 文件操作, 图片, 头像上传, 输出

type ApiFile struct {
	ApiBaseContrller
}

// 输出image
// [OK]
func (c ApiFile) GetImage(fileId string) revel.Result {
	download, err := fileService.OpenReadableImage(c.RequestContext(), c.getUserId(), fileId)
	if err != nil {
		return c.RenderText("")
	}
	filename := applicationcontent.AttachmentDownloadFilename(download.Name)
	return c.RenderBinary(download.Reader, filename, revel.Inline, time.Now())
}

// 下载附件
// [OK]
func (c ApiFile) GetAttach(fileId string) revel.Result {
	download, err := attachService.OpenReadable(c.RequestContext(), c.getUserId(), fileId)
	if err != nil {
		return c.RenderText("No Such File")
	}
	filename := applicationcontent.AttachmentDownloadFilename(download.DisplayName)
	return c.RenderBinary(download.Reader, filename, revel.Attachment, time.Now())
}

// 下载所有附件
// [OK]
func (c ApiFile) GetAllAttachs(noteId string) revel.Result {
	// Preserve the published API's no-token empty response. Anonymous public
	// note reads are supported by the content service for Web publishing, but
	// the token API has never exposed that capability.
	if strings.TrimSpace(c.getUserId()) == "" {
		return c.RenderText("")
	}
	archive, title, err := attachService.OpenReadableArchive(c.RequestContext(), c.getUserId(), noteId)
	if err != nil {
		return c.RenderText("")
	}
	filename := applicationcontent.ArchiveDownloadFilename(title)
	return c.RenderBinary(archive, filename, revel.Attachment, time.Now())
}
