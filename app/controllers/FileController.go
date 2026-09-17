package controllers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/revel/revel"
	//	"encoding/json"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
	//	"strconv"
)

// 首页
type File struct {
	BaseController
}

// 上传的是博客logo
// TODO logo不要设置权限, 另外的目录
func (c File) UploadBlogLogo() revel.Result {
	re := c.uploadImage("blogLogo", "")

	c.ViewArgs["fileUrlPath"] = re.Id
	c.ViewArgs["resultCode"] = re.Code
	c.ViewArgs["resultMsg"] = re.Msg

	return c.RenderTemplate("file/blog_logo.html")
}

// 拖拉上传, pasteImage
// noteId 是为了判断是否是协作的note, 如果是则需要复制一份到note owner中
func (c File) PasteImage(noteId string) revel.Result {
	re := c.uploadImage("pasteImage", "")

	if noteId != "" {
		userId := c.GetUserId()
		if copied, copiedID := fileService.CopyImageForNote(c.RequestContext(), userId, re.Id, noteId); copied {
			re.Id = copiedID
		}
	}

	return c.RenderJSON(re)
}

// 头像设置
func (c File) UploadAvatar() revel.Result {
	isDemo, err := configService.IsDemoUser(c.GetUserId())
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: demoPolicyErrorMessage(err)})
	}
	if isDemo {
		return c.RenderJSON(info.Re{Ok: false, Msg: "cannotUpdateDemo"})
	}
	re := c.uploadImage("logo", "")

	c.ViewArgs["fileUrlPath"] = re.Id
	c.ViewArgs["resultCode"] = re.Code
	c.ViewArgs["resultMsg"] = re.Msg

	if re.Ok {
		re.Ok = userService.UpdateAvatar(c.GetUserId(), re.Id)
		if re.Ok {
			c.UpdateSession("Logo", re.Id)
		}
	}

	return c.RenderJSON(re)
}

// leaui image plugin upload image
func (c File) UploadImageLeaui(albumId string) revel.Result {
	re := c.uploadImage("", albumId)
	return c.RenderJSON(re)
}

// 上传图片, 公用方法
// upload image common func
func (c File) uploadImage(from, albumId string) (re info.Re) {
	files := c.Params.Files["file"]
	if len(files) != 1 {
		re.Msg = "error"
		return re
	}
	handle := files[0]
	reader, err := handle.Open()
	if err != nil {
		re.Msg = "error"
		return re
	}
	defer reader.Close()

	configKey := "uploadImageSize"
	kind := service.ImageUploadPrivate
	if from == "logo" {
		configKey = "uploadAvatarSize"
		kind = service.ImageUploadAvatar
	} else if from == "blogLogo" {
		configKey = "uploadBlogLogoSize"
		kind = service.ImageUploadBlogLogo
	}
	maxFileSize, err := configService.GetUploadLimitBytes(configKey)
	if err != nil {
		re.Msg = "upload config error"
		return re
	}
	displayName := ""
	if from == "pasteImage" {
		displayName = c.Message("unTitled")
	}
	result, err := fileService.UploadImage(c.RequestContext(), service.ImageUploadInput{
		ActorID: c.GetUserId(), AlbumID: albumId, Kind: kind, Reader: reader, Limit: maxFileSize,
		OriginalName: handle.Filename, DisplayName: displayName, Budget: applicationcontent.HardImageBudget(),
	})
	if err != nil {
		var contentErr *applicationcontent.Error
		if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorTooLarge {
			re.Msg = fmt.Sprintf("The file Size is bigger than %vM", float64(maxFileSize)/(1024*1024))
		} else if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorUnsupportedMedia {
			re.Msg = "Please upload image"
		} else if errors.As(err, &contentErr) && contentErr.Category == applicationcontent.ErrorPartialWrite {
			re.Id = result.ExternalID
			re.Msg = "partial_write"
		} else {
			re.Msg = "error"
		}
		return re
	}
	result.File.Path = ""
	re.Ok, re.Code, re.Msg, re.Id, re.Item = true, 1, c.Message("Upload Success!"), result.ExternalID, result.File
	return re
}

// get all images by userId with page
func (c File) GetImages(albumId, key string, page int) revel.Result {
	re, err := fileService.ListImagesReadable(c.RequestContext(), c.GetUserId(), albumId, key, page, 12)
	if err != nil {
		re = info.Page{List: []info.File{}}
	}
	return c.RenderJSON(re)
}

func (c File) UpdateImageTitle(fileId, title string) revel.Result {
	re := info.NewRe()
	re.Ok = fileService.UpdateImageTitleResult(c.RequestContext(), c.GetUserId(), fileId, title) == nil
	return c.RenderJSON(re)
}

func (c File) DeleteImage(fileId string) revel.Result {
	re := info.NewRe()
	re.Ok, re.Msg = fileService.DeleteImageWithOperation(c.RequestContext(), c.GetUserId(), fileId, strings.TrimSpace(c.Params.Get("OperationId")))
	return c.RenderJSON(re)
}

//-----------

// 输出image
// 权限判断
func (c File) OutputImage(noteId, fileId string) revel.Result {
	download, err := fileService.OpenReadableImage(c.RequestContext(), c.GetUserId(), fileId)
	if err != nil {
		return c.RenderText("")
	}
	return c.RenderBinary(download.Reader, applicationcontent.AttachmentDownloadFilename(download.Name), revel.Inline, time.Now())
}

// 协作时复制图片到owner
// 需要计算对方大小
func (c File) CopyImage(userId, fileId, toUserId string) revel.Result {
	re := info.NewRe()
	actorID := c.GetUserId()
	if userId == actorID && toUserId == actorID {
		re.Ok, re.Id = fileService.CopyImage(actorID, fileId, actorID)
	}
	return c.RenderJSON(re)
}

// 复制外网的图片
// 都要好好的计算大小
func (c File) CopyHttpImage(src string) revel.Result {
	re := info.NewRe()
	maxFileSize, err := configService.GetUploadLimitBytes("uploadImageSize")
	if err != nil {
		re.Msg = "upload config error"
		return c.RenderJSON(re)
	}
	re.Ok, re.Id, re.Msg = fileService.CopyHTTPImage(c.RequestContext(), c.GetUserId(), src, maxFileSize)

	return c.RenderJSON(re)
}
