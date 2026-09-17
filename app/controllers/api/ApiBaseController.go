package api

import (
	"github.com/yangphere/leanote/app/controllers"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

type ApiBaseContrller struct {
	controllers.BaseController
}

const apiUploadPartialWrite = "partial_write"

func (c ApiBaseContrller) getToken() string  { return c.GetSession("_token") }
func (c ApiBaseContrller) getUserId() string { return c.GetSession("_userId") }

func (c ApiBaseContrller) getUserInfo() info.User {
	userID := c.getUserId()
	if userID == "" {
		return info.User{}
	}
	return userService.GetUserInfo(userID)
}

// prepareAPINoteAsset owns only multipart presence and opening. Content
// validation, paths, persistence, and cleanup are service responsibilities.
func (c ApiBaseContrller) prepareAPINoteAsset(name, noteID string, isAttach bool, assetID string) (service.APINoteAssetCandidate, string) {
	files := c.Params.Files[name]
	if len(files) != 1 {
		return service.APINoteAssetCandidate{}, "fileRequired"
	}
	reader, err := files[0].Open()
	if err != nil {
		return service.APINoteAssetCandidate{}, "fileRequired"
	}
	return attachService.PrepareAPINoteAsset(service.APINoteAssetPrepareInput{
		ActorID: c.getUserId(), NoteID: noteID, AssetID: assetID, IsAttach: isAttach,
		OriginalFilename: files[0].Filename, Reader: reader,
	})
}

func (c ApiBaseContrller) upload(name, noteID string, isAttach bool) (bool, string, string) {
	return c.uploadWithAssetIdentity(name, noteID, isAttach, "")
}

func (c ApiBaseContrller) uploadWithAssetIdentity(name, noteID string, isAttach bool, assetID string) (bool, string, string) {
	candidate, message := c.prepareAPINoteAsset(name, noteID, isAttach, assetID)
	if message != "" {
		return false, message, ""
	}
	return attachService.PublishAPINoteAsset(candidate)
}
