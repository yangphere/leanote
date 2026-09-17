package controllers

import (
	"errors"

	"github.com/revel/revel"
	//	"encoding/json"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	//	. "github.com/yangphere/leanote/app/lea"
	//	"io/ioutil"
)

// Album controller
type Album struct {
	BaseController
}

// 图片管理, iframe
func (c Album) Index() revel.Result {
	c.SetLocale()
	return c.RenderTemplate("album/index.html")
}

// all albums by userId
func (c Album) GetAlbums() revel.Result {
	re, err := albumService.ListAlbums(c.RequestContext(), c.GetUserId())
	if err != nil {
		re = []info.Album{}
	}
	return c.RenderJSON(re)
}
func (c Album) DeleteAlbum(albumId string) revel.Result {
	err := albumService.DeleteAlbumResult(c.RequestContext(), c.GetUserId(), albumId)
	msg := ""
	var contentErr *applicationcontent.Error
	if errors.As(err, &contentErr) && contentErr.Code == "album_has_images" {
		msg = "has images"
	}
	return c.RenderJSON(info.Re{Ok: err == nil, Msg: msg})
}

// add album
func (c Album) AddAlbum(name string) revel.Result {
	album, err := albumService.CreateAlbum(c.RequestContext(), c.GetUserId(), name, domain.ObjectID{}, -1)
	if err == nil {
		return c.RenderJSON(album)
	} else {
		return c.RenderJSON(false)
	}
}

// update alnum name
func (c Album) UpdateAlbum(albumId, name string) revel.Result {
	return c.RenderJSON(albumService.UpdateAlbumResult(c.RequestContext(), c.GetUserId(), albumId, name) == nil)
}
