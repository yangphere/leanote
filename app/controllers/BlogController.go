package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/info"
	// . "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/lea/blog"
	appservice "github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"math"
	//	"os"
	//	"path"
)

type Blog struct {
	BaseController
}

func (c Blog) blogQueryInput(keywords, tag string, userBlog info.UserBlog) appservice.BlogQueryInput {
	page, pagePresent := c.Params.Get("page"), c.Has("page")
	pageSize, pageSizePresent := c.Params.Get("pageSize"), c.Has("pageSize")
	sortField, sortPresent := c.Params.Get("sort"), c.Has("sort")
	return appservice.BlogQueryInput{
		Page:               page,
		PagePresent:        pagePresent,
		PageSize:           pageSize,
		PageSizePresent:    pageSizePresent,
		Sort:               sortField,
		SortPresent:        sortPresent,
		Keywords:           keywords,
		Tag:                tag,
		ConfiguredPageSize: userBlog.PerPageSize,
		ConfiguredSort:     userBlog.SortField,
		IsAsc:              userBlog.IsAsc,
	}
}

func (c Blog) validateBlogQueryShape(keywords, tag string) error {
	_, err := appservice.ParseBlogQuery(c.blogQueryInput(keywords, tag, info.UserBlog{
		PerPageSize: appservice.DefaultBlogPageSize,
		SortField:   "PublicTime",
	}))
	return err
}

func (c Blog) resolveBlogQuery(keywords, tag string, userBlog info.UserBlog) (appservice.BlogQuery, error) {
	return appservice.ParseBlogQuery(c.blogQueryInput(keywords, tag, userBlog))
}

func (c Blog) invalidBlogQueryResult() revel.Result {
	c.Response.Status = http.StatusBadRequest
	return c.RenderText("invalid blog query")
}

func (c Blog) validateBlogJSONP(callback string, optional bool) bool {
	if optional && !c.Has("callback") {
		return true
	}
	if !c.Has("callback") || !appservice.ValidJSONPCallback(callback) {
		c.Response.Status = http.StatusBadRequest
		return false
	}
	return true
}

func (c Blog) invalidBlogJSONPResult() revel.Result {
	c.Response.Status = http.StatusBadRequest
	return c.RenderText("invalid callback")
}

func (c Blog) validateBlogPageShape() error {
	_, err := appservice.ParseBlogQuery(appservice.BlogQueryInput{
		Page:               c.Params.Get("page"),
		PagePresent:        c.Has("page"),
		PageSize:           c.Params.Get("pageSize"),
		PageSizePresent:    c.Has("pageSize"),
		ConfiguredPageSize: 15,
		ConfiguredSort:     "CreatedTime",
	})
	return err
}

func (c Blog) publicBlogReadErrorResult(err error) revel.Result {
	if errors.Is(err, appservice.ErrPublicBlogNotFound) {
		c.Response.Status = http.StatusNotFound
		return c.RenderText("not found")
	}
	return c.internalBlogErrorResult()
}

func (c Blog) domainErrorResult() revel.Result {
	if c.Response.Status == http.StatusNotFound {
		return c.E404()
	}
	if c.Response.Status == 0 {
		c.Response.Status = http.StatusInternalServerError
	}
	return c.RenderText("%s", http.StatusText(c.Response.Status))
}

func (c Blog) internalBlogErrorResult() revel.Result {
	c.Response.Status = http.StatusInternalServerError
	return c.RenderText("internal server error")
}

func (c Blog) trustedProxyAllowlist() string {
	if revel.Config != nil {
		for _, key := range []string{"blog.trustedProxyCIDRs", "trustedProxyCIDRs", "http.trustedProxyCIDRs"} {
			if value, ok := revel.Config.String(key); ok && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	if configService != nil {
		return configService.GetGlobalStringConfig("trustedProxyCIDRs")
	}
	return ""
}

//-----------------------------
// 前台
/*
公共
// 分类 [ok]
$.cates = [{title, cateId}]
// 单页 [ok]
$.singles = [{pageId, title}]
// 博客信息 [ok]
$.blog = {userId, desc, title, logo, openComment, disqusId}

// 公用url ok
$.indexUrl
$.cateUrl
$.searchUrl
$.postUrl
$.archiveUrl
$.singleUrl
$.themeBaseUrl

// 静态文件 [ok]
$.jQueryUrl
$.fontAsomeUrl
$.bootstrapCssUrl
$.bootstrapJsUrl
*/

func (c Blog) render(templateName string, themePath string) revel.Result {
	isPreview := false
	if c.ViewArgs["isPreview"] != nil {
		themePath2 := c.ViewArgs["themePath"]
		if themePath2 == nil {
			return c.E404()
		}
		isPreview = true
		themePath = themePath2.(string)
		c.setPreviewUrl()

		// 因为common的themeInfo是从UserBlog.ThemeId来取的, 所以这里要fugai下
		c.ViewArgs["themeInfo"] = c.ViewArgs["themeInfoPreview"]
	}
	return blog.RenderTemplate(templateName, c.ViewArgs, revel.BasePath+"/"+themePath, isPreview)
}

// 404
func (c Blog) e404(themePath string) revel.Result {
	// 不知道是谁的404, 则用系统的404
	if themePath == "" {
		return c.E404()
	}
	return c.render("404.html", themePath)
}

// 二级域名或自定义域名
// life.leanote.com
// lealife.com
func (c Blog) domain() (ok bool, userBlog info.UserBlog) {
	if c.Request == nil || configService == nil || blogService == nil {
		c.Response.Status = http.StatusInternalServerError
		return false, userBlog
	}
	forwarded := strings.Join(c.Request.Header.GetAll("Forwarded"), ",")
	forwardedHost := strings.Join(c.Request.Header.GetAll("X-Forwarded-Host"), ",")
	host, err := appservice.SelectBlogHost(c.Request.Host, forwarded, forwardedHost, c.Request.RemoteAddr, c.trustedProxyAllowlist())
	if err != nil {
		c.Response.Status = http.StatusBadRequest
		return false, userBlog
	}
	defaultDomain, err := appservice.CanonicalizeBlogHost(configService.GetDefaultDomain())
	if err != nil {
		c.Response.Status = http.StatusInternalServerError
		return false, userBlog
	}
	if host == defaultDomain {
		return false, userBlog
	}
	if strings.HasSuffix(host, "."+defaultDomain) {
		subDomain := strings.TrimSuffix(host, "."+defaultDomain)
		if subDomain == "" || strings.Contains(subDomain, ".") {
			c.Response.Status = http.StatusBadRequest
			return false, userBlog
		}
		userBlog, err = blogService.LookupUserBlogBySubDomain(subDomain)
		if err != nil {
			c.Response.Status = http.StatusInternalServerError
			return false, info.UserBlog{}
		}
		if userBlog.UserId.IsZero() {
			c.Response.Status = http.StatusNotFound
			return false, info.UserBlog{}
		}
		return true, userBlog
	}
	if !configService.AllowCustomDomain() {
		c.Response.Status = http.StatusNotFound
		return false, userBlog
	}
	userBlog, err = blogService.LookupUserBlogByDomain(host)
	if err != nil {
		c.Response.Status = http.StatusInternalServerError
		return false, info.UserBlog{}
	}
	if userBlog.UserId.IsZero() {
		c.Response.Status = http.StatusNotFound
		return false, info.UserBlog{}
	}
	return true, userBlog
}

// 渲染模板之
func (c Blog) setPreviewUrl() {
	var indexUrl, postUrl, searchUrl, cateUrl, singleUrl, tagsUrl, archiveUrl string

	userId := c.GetUserId()
	userIdOrEmail := userId
	username := c.GetUsername()
	if username != "" {
		userIdOrEmail = username
	}
	themeId := c.GetSession("themeId")
	theme := themeService.GetTheme(userId, themeId)

	// siteUrl := configService.GetSiteUrl()
	blogUrl := "/preview" // blog.leanote.com

	indexUrl = blogUrl + "/" + userIdOrEmail
	cateUrl = blogUrl + "/cate/" + userIdOrEmail // /notebookId

	postUrl = blogUrl + "/post/" + userIdOrEmail        // /xxxxx
	searchUrl = blogUrl + "/search/" + userIdOrEmail    // blog.leanote.com/search/userId
	singleUrl = blogUrl + "/single/" + userIdOrEmail    // blog.leanote.com/single/singleId
	archiveUrl = blogUrl + "/archives/" + userIdOrEmail // blog.leanote.com/archive/userId
	tagsUrl = blogUrl + "/tags/" + userIdOrEmail        // blog.leanote.com/archive/userId

	c.ViewArgs["indexUrl"] = indexUrl
	c.ViewArgs["cateUrl"] = cateUrl
	c.ViewArgs["postUrl"] = postUrl
	c.ViewArgs["searchUrl"] = searchUrl
	c.ViewArgs["singleUrl"] = singleUrl // 单页
	c.ViewArgs["archiveUrl"] = archiveUrl
	c.ViewArgs["archivesUrl"] = archiveUrl // 别名
	c.ViewArgs["tagsUrl"] = tagsUrl
	c.ViewArgs["tagPostsUrl"] = blogUrl + "/tag/" + userIdOrEmail
	c.ViewArgs["tagUrl"] = c.ViewArgs["tagPostsUrl"]

	// themeBaseUrl 本theme的路径url, 可以加载js, css, images之类的
	c.ViewArgs["themeBaseUrl"] = "/" + theme.Path
}

// 各种地址设置
func (c Blog) setUrl(userBlog info.UserBlog, userInfo info.User) {
	// 主页 http://leanote.com/blog/life or http://blog.leanote.com/life or http:// xxxx.leanote.com or aa.com
	// host := c.Request.Request.Host
	// var staticUrl = configService.GetUserUrl(strings.Split(host, ":")[0])
	// staticUrl == host, 为保证同源!!! 只有host, http://leanote.com, http://blog/leanote.com
	// life.leanote.com, lealife.com
	siteUrl := configService.GetSiteUrl()
	blogUrls := blogService.GetBlogUrls(&userBlog, &userInfo)
	// 分类
	// 搜索
	// 查看
	c.ViewArgs["siteUrl"] = siteUrl
	c.ViewArgs["indexUrl"] = blogUrls.IndexUrl
	c.ViewArgs["cateUrl"] = blogUrls.CateUrl
	c.ViewArgs["postUrl"] = blogUrls.PostUrl
	c.ViewArgs["searchUrl"] = blogUrls.SearchUrl
	c.ViewArgs["singleUrl"] = blogUrls.SingleUrl // 单页
	c.ViewArgs["archiveUrl"] = blogUrls.ArchiveUrl
	c.ViewArgs["archivesUrl"] = blogUrls.ArchiveUrl // 别名
	c.ViewArgs["tagsUrl"] = blogUrls.TagsUrl
	c.ViewArgs["tagPostsUrl"] = blogUrls.TagPostsUrl
	c.ViewArgs["tagUrl"] = blogUrls.TagPostsUrl // 别名

	// themeBaseUrl 本theme的路径url, 可以加载js, css, images之类的
	c.ViewArgs["themeBaseUrl"] = "/" + userBlog.ThemePath

	// 其它static js
	c.ViewArgs["jQueryUrl"] = "/js/jquery-1.9.0.min.js"

	c.ViewArgs["prettifyJsUrl"] = "/js/google-code-prettify/prettify.js"
	c.ViewArgs["prettifyCssUrl"] = "/js/google-code-prettify/prettify.css"

	c.ViewArgs["blogCommonJsUrl"] = "/public/blog/js/common.js"

	c.ViewArgs["shareCommentCssUrl"] = "/public/blog/css/share_comment.css"
	c.ViewArgs["shareCommentJsUrl"] = "/public/blog/js/share_comment.js"

	c.ViewArgs["fontAwesomeUrl"] = "/css/font-awesome-4.2.0/css/font-awesome.css"

	c.ViewArgs["bootstrapCssUrl"] = "/css/bootstrap.css"
	c.ViewArgs["bootstrapJsUrl"] = "/js/bootstrap-min.js"
}

// 笔记本分类
// cates = [{title:"xxx", cateId: "xxxx"}, {}]
func (c Blog) getCateUrlTitle(n *info.Notebook) string {
	if n.UrlTitle != "" {
		return n.UrlTitle
	}
	return n.NotebookId.Hex()
}
func (c Blog) getCates(userBlog info.UserBlog) error {
	notebooks, err := blogService.ListBlogNotebooksChecked(userBlog.UserId.Hex())
	if err != nil {
		return err
	}
	notebooksMap := map[string]info.Notebook{}
	for _, each := range notebooks {
		notebooksMap[each.NotebookId.Hex()] = each
	}

	var i = 0
	cates := make([]*info.Cate, len(notebooks))

	// 先要保证已有的是正确的排序
	cateIds := userBlog.CateIds
	has := map[string]bool{} // cateIds中有的
	cateMap := map[string]*info.Cate{}
	if cateIds != nil && len(cateIds) > 0 {
		for _, cateId := range cateIds {
			if n, ok := notebooksMap[cateId]; ok {
				parentNotebookId := ""
				if !n.ParentNotebookId.IsZero() {
					parentNotebookId = n.ParentNotebookId.Hex()
				}
				cates[i] = &info.Cate{Title: n.Title, UrlTitle: c.getCateUrlTitle(&n), CateId: n.NotebookId.Hex(), ParentCateId: parentNotebookId}
				cateMap[cates[i].CateId] = cates[i]
				i++
				has[cateId] = true
			}
		}
	}

	// 之后添加没有排序的
	for _, n := range notebooks {
		id := n.NotebookId.Hex()
		if !has[id] {
			parentNotebookId := ""
			if !n.ParentNotebookId.IsZero() {
				parentNotebookId = n.ParentNotebookId.Hex()
			}
			cates[i] = &info.Cate{Title: n.Title, UrlTitle: c.getCateUrlTitle(&n), CateId: id, ParentCateId: parentNotebookId}
			cateMap[cates[i].CateId] = cates[i]
			i++
		}
	}

	//	LogJ(">>")
	//	LogJ(cates)

	// 建立层级
	hasParent := map[string]bool{} // 有父的cate
	for _, cate := range cates {
		parentCateId := cate.ParentCateId
		if parentCateId != "" {
			if parentCate, ok := cateMap[parentCateId]; ok {
				//				Log("________")
				//				LogJ(parentCate)
				//				LogJ(cate)
				if parentCate.Children == nil {
					parentCate.Children = []*info.Cate{cate}
				} else {
					parentCate.Children = append(parentCate.Children, cate)
				}
				hasParent[cate.CateId] = true
			}
		}
	}

	// 得到没有父的cate, 作为第一级cate
	catesTree := []*info.Cate{}
	for _, cate := range cates {
		if !hasParent[cate.CateId] {
			catesTree = append(catesTree, cate)
		}
	}

	c.ViewArgs["cates"] = cates
	c.ViewArgs["catesTree"] = catesTree
	return nil
}

// 单页
func (c Blog) getSingles(userId string) {
	singles := blogService.GetSingles(userId)
	/*
		if singles == nil {
			return
		}
		singles2 := make([]map[string]string, len(singles))
		for i, page := range singles {
			singles2[i] = map[string]string{"title": page["Title"], "singleId": page["SingleId"]}
		}
	*/
	c.ViewArgs["singles"] = singles
}

// $.blog = {userId, title, subTitle, desc, openComment, }
func (c Blog) setBlog(userBlog info.UserBlog, userInfo info.User) {
	blogInfo := map[string]interface{}{
		"UserId":      userBlog.UserId.Hex(),
		"Username":    userInfo.Username,
		"UserLogo":    userInfo.Logo,
		"Title":       userBlog.Title,
		"SubTitle":    userBlog.SubTitle,
		"Logo":        userBlog.Logo,
		"OpenComment": userBlog.CanComment,
		"CommentType": userBlog.CommentType, // leanote, or disqus
		"DisqusId":    userBlog.DisqusId,
		"ThemeId":     userBlog.ThemeId,
		"SubDomain":   userBlog.SubDomain,
		"Domain":      userBlog.Domain,
	}
	c.ViewArgs["blogInfo"] = blogInfo
}

func (c Blog) setPaging(pageInfo info.Page) {
	c.ViewArgs["paging"] = pageInfo
}

// 公共
func (c Blog) blogCommon(userId string, userBlog info.UserBlog, userInfo info.User) (ok bool, ub info.UserBlog, err error) {
	if userInfo.UserId.IsZero() {
		userInfo = userService.GetUserInfoByAny(userId)
		if userInfo.UserId.IsZero() {
			return false, userBlog, nil
		}
	}
	if userBlog.UserId.IsZero() {
		userBlog, err = blogService.GetUserBlogChecked(userId)
		if err != nil {
			return false, userBlog, err
		}
	}

	// 最新笔记
	_, recentBlogs, err := blogService.ListBlogsChecked(userId, "", 1, 5, userBlog.SortField, userBlog.IsAsc)
	if err != nil {
		return false, userBlog, err
	}
	c.ViewArgs["recentPosts"] = blogService.FixBlogs(recentBlogs)
	c.ViewArgs["latestPosts"] = c.ViewArgs["recentPosts"]
	tags, err := blogService.GetBlogTagsChecked(userId)
	if err != nil {
		return false, userBlog, err
	}
	c.ViewArgs["tags"] = tags

	// 语言, url地址
	c.SetLocale()

	// 得到博客设置信息
	c.setBlog(userBlog, userInfo)
	//	c.ViewArgs["userBlog"] = userBlog

	// 分类导航
	if err := c.getCates(userBlog); err != nil {
		return false, userBlog, err
	}

	// 单页导航
	c.getSingles(userId)

	c.setUrl(userBlog, userInfo)

	// 当前分类Id, 全设为""
	c.ViewArgs["curCateId"] = ""
	c.ViewArgs["curSingleId"] = ""

	// 得到主题信息
	themeInfo := themeService.GetThemeInfo(userBlog.ThemeId.Hex(), userBlog.Style)
	c.ViewArgs["themeInfo"] = themeInfo

	//	Log(">>")
	//	Log(userBlog.Style)
	//	Log(userBlog.ThemeId.Hex())

	return true, userBlog, nil
}

// 404
func (c Blog) E(userIdOrEmail, tag string) revel.Result {
	ok, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	var userId string
	if ok {
		userId = userBlog.UserId.Hex()
	}
	var userInfo info.User
	if userId != "" {
		userInfo = userService.GetUserInfoByAny(userId)
	} else {
		// blog.leanote.com/userid/tag
		userInfo = userService.GetUserInfoByAny(userIdOrEmail)
	}
	userId = userInfo.UserId.Hex()
	var blogErr error
	if _, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	}

	return c.e404(userBlog.ThemePath)
}

func (c Blog) Tags(userIdOrEmail string) (re revel.Result) {
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			re = c.e404(userBlog.ThemePath)
		}
	}()

	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)

	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}

	c.ViewArgs["curIsTags"] = true
	return c.render("tags.html", userBlog.ThemePath)
}

// 标签的文章页
func (c Blog) Tag(userIdOrEmail, tag string) (re revel.Result) {
	queryTag := tag
	if queryTag == "" {
		queryTag = userIdOrEmail
	}
	if err := c.validateBlogQueryShape("", queryTag); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			re = c.e404(userBlog.ThemePath)
		}
	}()

	if tag == "" {
		tag = userIdOrEmail
		userIdOrEmail = ""
	}
	if !hasDomain && userIdOrEmail == "" {
		userIdOrEmail = configService.GetAdminUsername()
	}
	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)

	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}

	query, err := c.resolveBlogQuery("", tag, userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	c.ViewArgs["curIsTagPosts"] = true
	c.ViewArgs["curTag"] = tag
	pageInfo, blogs, err := blogService.SearchBlogByTagsChecked([]string{query.Tag}, userId, query.Page, query.PageSize, query.SortField, query.IsAsc)
	if err != nil {
		return c.internalBlogErrorResult()
	}
	c.setPaging(pageInfo)

	c.ViewArgs["posts"] = blogService.FixBlogs(blogs)
	tagPostsUrl := c.ViewArgs["tagPostsUrl"].(string)
	c.ViewArgs["pagingBaseUrl"] = tagPostsUrl + "/" + tag

	return c.render("tag_posts.html", userBlog.ThemePath)
}

// 归档
func (c Blog) Archives(userIdOrEmail string, cateId string, year, month int) (re revel.Result) {
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	notebookId := cateId
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			re = c.e404(userBlog.ThemePath)
		}
	}()
	// 用户id为空, 转至博客平台
	if !hasDomain && userIdOrEmail == "" {
		userIdOrEmail = configService.GetAdminUsername()
	}
	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)

	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}
	query, err := c.resolveBlogQuery("", "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	arcs, err := blogService.ListBlogsArchiveChecked(userId, notebookId, year, month, query.SortField, query.IsAsc)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidBlogQuery) {
			return c.invalidBlogQueryResult()
		}
		return c.internalBlogErrorResult()
	}
	c.ViewArgs["archives"] = arcs

	c.ViewArgs["curIsArchive"] = true
	if notebookId != "" {
		notebook := notebookService.GetNotebookById(notebookId)
		c.ViewArgs["curCateTitle"] = notebook.Title
		c.ViewArgs["curCateId"] = notebookId
	}
	c.ViewArgs["curYear"] = year
	c.ViewArgs["curMonth"] = month

	return c.render("archive.html", userBlog.ThemePath)
}

// 进入某个用户的博客
var blogPageSize = 5
var searchBlogPageSize = 30

// 分类 /cate/xxxxxxxx?notebookId=1212
func (c Blog) Cate(userIdOrEmail string, notebookId string) (re revel.Result) {
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			re = c.e404(userBlog.ThemePath)
		}
	}()

	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)
	notebookId2 := notebookId
	var notebook info.Notebook
	if userId == "" { // 证明没有userIdOrEmail, 只有singleId, 那么直接查
		notebook = notebookService.GetNotebookById(notebookId)
		userId = notebook.UserId.Hex()
	} else {
		notebook = notebookService.GetNotebookByUserIdAndUrlTitle(userId, notebookId)
		notebookId2 = notebook.NotebookId.Hex()
	}
	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}
	if !notebook.IsBlog {
		panic("")
	}
	query, err := c.resolveBlogQuery("", "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	// 分页的话, 需要分页信息, totalPage, curPage
	pageInfo, blogs, err := blogService.ListBlogsChecked(userId, notebookId2, query.Page, query.PageSize, query.SortField, query.IsAsc)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidBlogQuery) {
			return c.invalidBlogQueryResult()
		}
		return c.internalBlogErrorResult()
	}
	blogs2 := blogService.FixBlogs(blogs)
	c.ViewArgs["posts"] = blogs2

	c.setPaging(pageInfo)

	c.ViewArgs["curCateTitle"] = notebook.Title
	c.ViewArgs["curCateId"] = notebookId2
	cateUrl := c.ViewArgs["cateUrl"].(string)
	c.ViewArgs["pagingBaseUrl"] = cateUrl + "/" + notebookId
	c.ViewArgs["curIsCate"] = true

	return c.render("cate.html", userBlog.ThemePath)
}

func (c Blog) userIdOrEmail(hasDomain bool, userBlog info.UserBlog, userIdOrEmail string) (userId string, userInfo info.User) {
	userId = ""
	if hasDomain {
		userId = userBlog.UserId.Hex()
		if userIdOrEmail != "" {
			requestedUser := userService.GetUserInfoByAny(userIdOrEmail)
			if requestedUser.UserId.IsZero() || requestedUser.UserId.Hex() != userId {
				return "", info.User{}
			}
		}
	}
	if userId != "" {
		userInfo = userService.GetUserInfoByAny(userId)
	} else {
		if userIdOrEmail != "" {
			userInfo = userService.GetUserInfoByAny(userIdOrEmail)
		} else {
			return
		}
	}
	userId = userInfo.UserId.Hex()
	return
}

func (c Blog) Index(userIdOrEmail string) (re revel.Result) {
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			re = c.e404(userBlog.ThemePath)
		}
	}()
	// 用户id为空, 则是admin用户的博客
	if userIdOrEmail == "" {
		userIdOrEmail = configService.GetAdminUsername()
	}
	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)
	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}
	query, err := c.resolveBlogQuery("", "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	// 分页的话, 需要分页信息, totalPage, curPage
	pageInfo, blogs, err := blogService.ListBlogsChecked(userId, "", query.Page, query.PageSize, query.SortField, query.IsAsc)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidBlogQuery) {
			return c.invalidBlogQueryResult()
		}
		return c.internalBlogErrorResult()
	}
	blogs2 := blogService.FixBlogs(blogs)
	c.ViewArgs["posts"] = blogs2

	c.setPaging(pageInfo)
	c.ViewArgs["pagingBaseUrl"] = c.ViewArgs["indexUrl"]

	c.ViewArgs["curIsIndex"] = true

	return c.render("index.html", userBlog.ThemePath)
}

func (c Blog) Post(userIdOrEmail, noteId string) (re revel.Result) {
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			// Log(err)
			re = c.e404(userBlog.ThemePath)
		}
	}()

	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)
	var blogInfo info.BlogItem
	var blogReadErr error
	if userId == "" { // 证明没有userIdOrEmail, 只有singleId, 那么直接查
		blogInfo, blogReadErr = blogService.GetBlogChecked(noteId)
		if blogReadErr == nil {
			userId = blogInfo.UserId.Hex()
		}
	} else {
		blogInfo, blogReadErr = blogService.GetBlogByIdAndUrlTitleChecked(userId, noteId)
	}
	if blogReadErr != nil {
		if errors.Is(blogReadErr, appservice.ErrPublicBlogNotFound) {
			return c.e404(userBlog.ThemePath)
		}
		return c.internalBlogErrorResult()
	}
	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}
	query, err := c.resolveBlogQuery("", "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}
	if blogInfo.NoteId.IsZero() {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}

	post := blogService.FixBlog(blogInfo)
	c.ViewArgs["post"] = post
	// c.ViewArgs["userInfo"] = userInfo
	c.ViewArgs["curIsPost"] = true

	// 上一篇, 下一篇
	var baseTime interface{}
	if query.SortField == "PublicTime" {
		baseTime = blogInfo.PublicTime
	} else if query.SortField == "CreatedTime" {
		baseTime = blogInfo.CreatedTime
	} else if query.SortField == "UpdatedTime" {
		baseTime = blogInfo.UpdatedTime
	} else {
		baseTime = blogInfo.Title
	}

	prePost, nextPost, err := blogService.PreNextBlogChecked(userId, query.SortField, query.IsAsc, post.NoteId, baseTime)
	if err != nil {
		if errors.Is(err, appservice.ErrPublicBlogNotFound) {
			return c.e404(userBlog.ThemePath)
		}
		return c.internalBlogErrorResult()
	}
	if prePost.NoteId != "" {
		c.ViewArgs["prePost"] = prePost
	}
	if nextPost.NoteId != "" {
		c.ViewArgs["nextPost"] = nextPost
	}
	return c.render("post.html", userBlog.ThemePath)
}

func (c Blog) Single(userIdOrEmail, singleId string) (re revel.Result) {
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			re = c.e404(userBlog.ThemePath)
		}
	}()

	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)
	var single info.BlogSingle
	var singleReadErr error
	if userId == "" { // 证明没有userIdOrEmail, 只有singleId, 那么直接查
		single, singleReadErr = blogService.GetSingleChecked(singleId)
		if singleReadErr == nil {
			userId = single.UserId.Hex()
		}
	} else {
		single, singleReadErr = blogService.GetSingleByUserIdAndUrlTitleChecked(userId, singleId)
	}
	if singleReadErr != nil {
		if errors.Is(singleReadErr, appservice.ErrPublicBlogNotFound) {
			return c.e404(userBlog.ThemePath)
		}
		return c.internalBlogErrorResult()
	}
	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath) // 404 TODO 使用用户的404
	}
	if single.SingleId.IsZero() {
		panic("")
	}

	c.ViewArgs["single"] = map[string]interface{}{
		"SingleId":    single.SingleId.Hex(),
		"Title":       single.Title,
		"UrlTitle":    single.UrlTitle,
		"Content":     single.Content,
		"CreatedTime": single.CreatedTime,
		"UpdatedTime": single.UpdatedTime,
	}
	c.ViewArgs["curSingleId"] = single.SingleId.Hex()
	c.ViewArgs["curIsSingle"] = true

	return c.render("single.html", userBlog.ThemePath)
}

// 搜索
func (c Blog) Search(userIdOrEmail, keywords string) (re revel.Result) {
	if err := c.validateBlogQueryShape(keywords, ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	defer func() {
		if err := recover(); err != nil {
			re = c.e404(userBlog.ThemePath)
		}
	}()
	userId, userInfo := c.userIdOrEmail(hasDomain, userBlog, userIdOrEmail)
	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, userInfo); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath)
	}
	query, err := c.resolveBlogQuery(keywords, "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	pageInfo, blogs, err := blogService.SearchBlogChecked(query.Keywords, userId, query.Page, query.PageSize, query.SortField, query.IsAsc)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidBlogQuery) {
			return c.invalidBlogQueryResult()
		}
		return c.internalBlogErrorResult()
	}
	c.setPaging(pageInfo)

	c.ViewArgs["posts"] = blogService.FixBlogs(blogs)
	c.ViewArgs["keywords"] = query.Keywords
	searchUrl, _ := c.ViewArgs["searchUrl"].(string)
	c.ViewArgs["pagingBaseUrl"] = searchUrl + "?keywords=" + query.Keywords
	c.ViewArgs["curIsSearch"] = true

	return c.render("search.html", userBlog.ThemePath)
}

// 可以不要, 因为注册的时候已经把username设为email了
func (c Blog) setRenderUserInfo(userInfo info.User) {
	if userInfo.Username == "" {
		userInfo.Username = userInfo.Email
	}
	c.ViewArgs["userInfo"] = userInfo
}

//----------------
// 社交, 点赞, 评论

// 得到博客统计信息
func (c Blog) GetPostStat(noteId string) revel.Result {
	re := info.NewRe()
	statInfo, err := blogService.GetBlogStatChecked(noteId)
	if err != nil {
		return c.publicBlogReadErrorResult(err)
	}
	re.Ok = true
	re.Item = statInfo
	return c.RenderJSON(re)
}

// jsonP
// 我是否点过赞? 得到我的信息
// 所有点赞的用户列表
// 各个评论中是否我也点过赞?
func (c Blog) GetLikes(noteId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	userId := c.GetUserId()
	result := map[string]interface{}{}
	isILikeIt := false
	if userId != "" {
		var err error
		isILikeIt, err = blogService.IsILikeItChecked(noteId, userId)
		if err != nil {
			return c.publicBlogReadErrorResult(err)
		}
		result["visitUserInfo"] = userService.GetUserAndBlog(userId)
	}
	// 点赞用户列表
	likedUsers, hasMoreLikedUser, err := blogService.ListLikedUsersChecked(noteId, false)
	if err != nil {
		return c.publicBlogReadErrorResult(err)
	}

	re := info.NewRe()
	re.Ok = true
	result["isILikeIt"] = isILikeIt
	result["likedUsers"] = likedUsers
	result["hasMoreLikedUser"] = hasMoreLikedUser

	re.Item = result
	return c.RenderJSONP(callback, re)
}
func (c Blog) GetLikesAndComments(noteId, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	if err := c.validateBlogPageShape(); err != nil {
		return c.invalidBlogQueryResult()
	}
	userId := c.GetUserId()
	result := map[string]interface{}{}

	// 我也点过?
	isILikeIt := false
	if userId != "" {
		var err error
		isILikeIt, err = blogService.IsILikeItChecked(noteId, userId)
		if err != nil {
			return c.publicBlogReadErrorResult(err)
		}
		result["visitUserInfo"] = userService.GetUserAndBlog(userId)
	}

	// 点赞用户列表
	likedUsers, hasMoreLikedUser, err := blogService.ListLikedUsersChecked(noteId, false)
	if err != nil {
		return c.publicBlogReadErrorResult(err)
	}
	// 评论
	page := c.GetPage()
	pageInfo, comments, commentUserInfo, err := blogService.ListCommentsChecked(userId, noteId, page, 15)
	if err != nil {
		return c.publicBlogReadErrorResult(err)
	}

	re := info.NewRe()
	re.Ok = true
	result["isILikeIt"] = isILikeIt
	result["likedUsers"] = likedUsers
	result["hasMoreLikedUser"] = hasMoreLikedUser
	result["pageInfo"] = pageInfo
	result["comments"] = comments
	result["commentUserInfo"] = commentUserInfo
	re.Item = result
	return c.RenderJSONP(callback, re)
}

func (c Blog) IncReadNum(noteId string) revel.Result {
	re := info.NewRe()
	re.Ok = blogService.IncReadNum(noteId)
	return c.RenderJSON(re)
}

// 点赞, 要用jsonp
func (c Blog) LikePost(noteId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	re := info.NewRe()
	userId := c.GetUserId()
	re.Ok, re.Item = blogService.LikeBlog(noteId, userId)
	return c.RenderJSONP(callback, re)
}
func (c Blog) GetComments(noteId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, true) {
		return c.invalidBlogJSONPResult()
	}
	if err := c.validateBlogPageShape(); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 评论
	userId := c.GetUserId()
	page := c.GetPage()
	pageInfo, comments, commentUserInfo, err := blogService.ListCommentsChecked(userId, noteId, page, 15)
	if err != nil {
		return c.publicBlogReadErrorResult(err)
	}
	re := info.NewRe()
	re.Ok = true
	result := map[string]interface{}{}
	result["pageInfo"] = pageInfo
	result["comments"] = comments
	result["commentUserInfo"] = commentUserInfo
	re.Item = result

	if callback != "" {
		return c.RenderJSONP(callback, result)
	}

	return c.RenderJSON(re)
}

// jsonp
func (c Blog) DeleteComment(noteId, commentId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	re := info.NewRe()
	re.Ok = blogService.DeleteComment(noteId, commentId, c.GetUserId())
	return c.RenderJSONP(callback, re)
}

// jsonp
func (c Blog) CommentPost(noteId, content, toCommentId, submissionId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	re := info.NewRe()
	if !c.Has("submissionId") {
		return c.RenderJSONP(callback, re)
	}
	re.Ok, re.Item = blogService.Comment(noteId, toCommentId, c.GetUserId(), content, submissionId)
	return c.RenderJSONP(callback, re)
}

// jsonp
func (c Blog) LikeComment(commentId string, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	re := info.NewRe()
	ok, isILikeIt, num := blogService.LikeComment(commentId, c.GetUserId())
	re.Ok = ok
	re.Item = bson.M{"IsILikeIt": isILikeIt, "Num": num}
	return c.RenderJSONP(callback, re)
}

// 显示分类的最近博客, jsonp
func (c Blog) ListCateLatest(notebookId, callback string) revel.Result {
	if !c.validateBlogJSONP(callback, false) {
		return c.invalidBlogJSONPResult()
	}
	if notebookId == "" {
		return c.e404("")
	}
	if err := c.validateBlogQueryShape("", ""); err != nil {
		return c.invalidBlogQueryResult()
	}
	// 自定义域名
	hasDomain, userBlog := c.domain()
	if c.Response.Status >= http.StatusBadRequest {
		return c.domainErrorResult()
	}
	userId := ""
	if hasDomain {
		userId = userBlog.UserId.Hex()
	}

	var notebook info.Notebook
	notebook = notebookService.GetNotebookById(notebookId)
	if !notebook.IsBlog {
		return c.e404(userBlog.ThemePath)
	}
	if userId != "" && userId != notebook.UserId.Hex() {
		return c.e404(userBlog.ThemePath)
	}
	userId = notebook.UserId.Hex()

	var ok = false
	var blogErr error
	if ok, userBlog, blogErr = c.blogCommon(userId, userBlog, info.User{}); blogErr != nil {
		return c.internalBlogErrorResult()
	} else if !ok {
		return c.e404(userBlog.ThemePath)
	}
	query, err := c.resolveBlogQuery("", "", userBlog)
	if err != nil {
		return c.invalidBlogQueryResult()
	}

	// 分页的话, 需要分页信息, totalPage, curPage
	_, blogs, err := blogService.ListBlogsChecked(userId, notebookId, query.Page, query.PageSize, query.SortField, query.IsAsc)
	if err != nil {
		if errors.Is(err, appservice.ErrInvalidBlogQuery) {
			return c.invalidBlogQueryResult()
		}
		return c.internalBlogErrorResult()
	}
	re := info.NewRe()
	re.Ok = true
	re.List = blogs
	return c.RenderJSONP(callback, re)
}
