package controllers

import (
	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/db"
	//	"strings"
	//	"time"
	//	"encoding/json"
	//	"github.com/yangphere/leanote/app/info"
	//	. "github.com/yangphere/leanote/app/lea"
	//	"github.com/yangphere/leanote/app/lea/blog"
	//	"go.mongodb.org/mongo-driver/v2/bson"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"math"
	//	"os"
	//	"path"
)

type Preview struct {
	Blog
}

// 得到要预览的主题绝对路径
func (c Preview) getPreviewThemeAbsolutePath(themeId string) bool {
	selectedThemeID := themeId
	if selectedThemeID == "" {
		selectedThemeID = c.GetSession("themeId") // 直接从session中获取
	}
	if selectedThemeID == "" {
		return false
	}
	theme, err := themeService.ResolvePreviewTheme(c.GetUserId(), selectedThemeID)
	if err != nil {
		return false
	}
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= 400 || (hasDomain && userBlog.UserId.Hex() != c.GetUserId()) {
		return false
	}
	c.ViewArgs["isPreview"] = true
	c.ViewArgs["themeId"] = selectedThemeID
	c.ViewArgs["themeInfoPreview"] = theme.Info
	c.ViewArgs["themePath"] = theme.Path
	if theme.Path == "" {
		return false
	}
	c.Session["themeId"] = selectedThemeID
	return true
}

func (c Preview) matchesPreviewOwner(requested string) bool {
	current := c.GetUserId()
	if current == "" {
		return false
	}
	if requested == "" || requested == current {
		return true
	}
	if db.IsValidObjectIDHex(requested) {
		return false
	}
	if userService == nil {
		return false
	}
	return userService.GetUserInfoByAny(requested).UserId.Hex() == current
}

func (c Preview) Index(userIdOrEmail string, themeId string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath(themeId) {
		return c.E404()
	}
	return c.Blog.Index(c.GetUserId())
	//	return blog.RenderTemplate("index.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(themeId))
}

func (c Preview) Tag(userIdOrEmail, tag string) revel.Result {
	if tag != "" && !c.matchesPreviewOwner(userIdOrEmail) {
		return c.E404()
	}
	if !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	if tag == "" {
		tag = userIdOrEmail
	}
	return c.Blog.Tag(c.GetUserId(), tag)
}
func (c Preview) Tags(userIdOrEmail string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Tags(c.GetUserId())
	//	if tag == "" {
	//		return blog.RenderTemplate("tags.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
	//	}
	//	return blog.RenderTemplate("tag_posts.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
func (c Preview) Archives(userIdOrEmail string, notebookId string, year, month int) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Archives(c.GetUserId(), notebookId, year, month)
	//	return blog.RenderTemplate("archive.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
func (c Preview) Cate(userIdOrEmail, notebookId string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Cate(c.GetUserId(), notebookId)
	//	return blog.RenderTemplate("cate.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
func (c Preview) Post(userIdOrEmail, noteId string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Post(c.GetUserId(), noteId)
	//	return blog.RenderTemplate("view.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
func (c Preview) Single(userIdOrEmail, singleId string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Single(c.GetUserId(), singleId)
	//	return blog.RenderTemplate("single.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
func (c Preview) Search(userIdOrEmail, keywords string) revel.Result {
	if !c.matchesPreviewOwner(userIdOrEmail) || !c.getPreviewThemeAbsolutePath("") {
		return c.E404()
	}
	return c.Blog.Search(c.GetUserId(), keywords)
	//	return blog.RenderTemplate("search.html", c.ViewArgs, c.getPreviewThemeAbsolutePath(""))
}
