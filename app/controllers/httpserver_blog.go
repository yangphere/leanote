package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

// renderBlogPage is the page side of the publishing adapter. JSON/JSONP
// actions stay in httpserver_publishing.go; this path owns the legacy page
// projection and delegates theme file reads to ThemeService.
func (s *BlogHTTPServer) renderBlogPage(c *httpserver.Context) httpserver.Result {
	if userService == nil || blogService == nil || configService == nil || themeService == nil {
		return c.RenderText("storage")
	}
	hasDomain, domainBlog, domainResult := resolveBlogDomain(c)
	if domainResult != nil {
		return domainResult
	}

	userRef := strings.TrimSpace(c.Params.String("userIdOrEmail"))
	if hasDomain {
		userRef = domainBlog.UserId.Hex()
	}
	// Post and Single have routes that carry only the resource id. Resolve the
	// owner from that resource before loading the owner-scoped blog settings.
	if userRef == "" {
		switch c.Action {
		case "Post":
			blog, err := blogService.GetBlogChecked(c.Params.String("noteId"))
			if err != nil || blog.UserId.IsZero() {
				return c.NotFound("")
			}
			userRef = blog.UserId.Hex()
		case "Single":
			single, err := blogService.GetSingleChecked(c.Params.String("singleId"))
			if err != nil || single.UserId.IsZero() {
				return c.NotFound("")
			}
			userRef = single.UserId.Hex()
		case "Default", "Index":
			userRef = configService.GetAdminUsername()
		}
	}
	userInfo := userService.GetUserInfoByAny(userRef)
	if userInfo.UserId.IsZero() {
		return c.NotFound("")
	}
	userID := userInfo.UserId.Hex()
	userBlog, err := blogService.GetUserBlogChecked(userID)
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
	}
	if userBlog.UserId.IsZero() {
		return c.NotFound("")
	}

	args, err := buildBlogView(c, userBlog, userInfo)
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
	}
	if c.Action == "E" {
		return renderBlogNotFound(c, userBlog, args)
	}
	if err := populateBlogPage(c, args, userBlog, userID, userRef); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBlogQuery):
			return httpserver.TextResult(http.StatusBadRequest, "invalid blog query")
		case errors.Is(err, service.ErrPublicBlogNotFound):
			return c.NotFound("")
		default:
			return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
		}
	}

	templateAction := c.Action
	if templateAction == "Default" {
		templateAction = "Index"
	}
	templateName := strings.ToLower(templateAction)
	if templateName == "index" {
		templateName = "index"
	}
	body, err := themeService.RenderBlogTemplate(userBlog, templateName+".html", args)
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "internal server error")
	}
	return httpserver.HTMLResult(http.StatusOK, string(body))
}

func renderBlogNotFound(c *httpserver.Context, userBlog info.UserBlog, args map[string]interface{}) httpserver.Result {
	body, err := themeService.RenderBlogTemplate(userBlog, "404.html", args)
	if err != nil {
		return c.NotFound("")
	}
	return httpserver.HTMLResult(http.StatusNotFound, string(body))
}

func resolveBlogDomain(c *httpserver.Context) (bool, info.UserBlog, httpserver.Result) {
	forwarded := strings.Join(c.Request.Header.Values("Forwarded"), ",")
	forwardedHost := strings.Join(c.Request.Header.Values("X-Forwarded-Host"), ",")
	allowlist := configService.GetGlobalStringConfig("trustedProxyCIDRs")
	host, err := service.SelectBlogHost(c.Request.Host, forwarded, forwardedHost, c.Request.RemoteAddr, allowlist)
	if err != nil {
		return false, info.UserBlog{}, httpserver.TextResult(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}
	defaultDomain, err := service.CanonicalizeBlogHost(configService.GetDefaultDomain())
	if err != nil {
		return false, info.UserBlog{}, httpserver.TextResult(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
	if host == defaultDomain {
		return false, info.UserBlog{}, nil
	}
	if strings.HasSuffix(host, "."+defaultDomain) {
		subDomain := strings.TrimSuffix(host, "."+defaultDomain)
		if subDomain == "" || strings.Contains(subDomain, ".") {
			return false, info.UserBlog{}, httpserver.TextResult(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		}
		userBlog, lookupErr := blogService.LookupUserBlogBySubDomain(subDomain)
		if lookupErr != nil {
			return false, info.UserBlog{}, httpserver.TextResult(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
		if userBlog.UserId.IsZero() {
			return false, info.UserBlog{}, httpserver.TextResult(http.StatusNotFound, http.StatusText(http.StatusNotFound))
		}
		return true, userBlog, nil
	}
	if !configService.AllowCustomDomain() {
		return false, info.UserBlog{}, httpserver.TextResult(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}
	userBlog, lookupErr := blogService.LookupUserBlogByDomain(host)
	if lookupErr != nil {
		return false, info.UserBlog{}, httpserver.TextResult(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
	if userBlog.UserId.IsZero() {
		return false, info.UserBlog{}, httpserver.TextResult(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}
	return true, userBlog, nil
}

func buildBlogView(c *httpserver.Context, userBlog info.UserBlog, userInfo info.User) (map[string]interface{}, error) {
	args := prepareView(c)
	args["userInfo"] = userInfo
	args["userBlog"] = userBlog
	args["blogInfo"] = map[string]interface{}{
		"UserId":      userBlog.UserId.Hex(),
		"Username":    userInfo.Username,
		"UserLogo":    userInfo.Logo,
		"Title":       userBlog.Title,
		"SubTitle":    userBlog.SubTitle,
		"Logo":        userBlog.Logo,
		"OpenComment": userBlog.CanComment,
		"CommentType": userBlog.CommentType,
		"DisqusId":    userBlog.DisqusId,
		"ThemeId":     userBlog.ThemeId.Hex(),
		"SubDomain":   userBlog.SubDomain,
		"Domain":      userBlog.Domain,
	}
	blogURLs := blogService.GetBlogUrls(&userBlog, &userInfo)
	args["indexUrl"] = blogURLs.IndexUrl
	args["cateUrl"] = blogURLs.CateUrl
	args["postUrl"] = blogURLs.PostUrl
	args["searchUrl"] = blogURLs.SearchUrl
	args["singleUrl"] = blogURLs.SingleUrl
	args["archiveUrl"] = blogURLs.ArchiveUrl
	args["archivesUrl"] = blogURLs.ArchiveUrl
	args["tagsUrl"] = blogURLs.TagsUrl
	args["tagPostsUrl"] = blogURLs.TagPostsUrl
	args["tagUrl"] = blogURLs.TagPostsUrl
	args["themeBaseUrl"] = "/" + strings.TrimSuffix(strings.TrimPrefix(userBlog.ThemePath, "/"), "/")
	args["jQueryUrl"] = "/js/jquery-1.9.0.min.js"
	args["prettifyJsUrl"] = "/js/google-code-prettify/prettify.js"
	args["prettifyCssUrl"] = "/js/google-code-prettify/prettify.css"
	args["blogCommonJsUrl"] = "/public/blog/js/common.js"
	args["shareCommentCssUrl"] = "/public/blog/css/share_comment.css"
	args["shareCommentJsUrl"] = "/public/blog/js/share_comment.js"
	args["fontAwesomeUrl"] = "/css/font-awesome-4.2.0/css/font-awesome.css"
	args["bootstrapCssUrl"] = "/css/bootstrap.css"
	args["bootstrapJsUrl"] = "/js/bootstrap-min.js"
	args["leanoteUrl"] = args["siteUrl"]
	args["curCateId"] = ""
	args["curSingleId"] = ""
	args["keywords"] = c.Params.String("keywords")

	notebooks, err := blogService.ListBlogNotebooksChecked(userBlog.UserId.Hex())
	if err != nil {
		return nil, err
	}
	cates, catesTree := buildBlogCates(notebooks, userBlog.CateIds)
	args["cates"] = cates
	args["catesTree"] = catesTree
	args["singles"] = blogService.GetSingles(userBlog.UserId.Hex())
	_, recentBlogs, err := blogService.ListBlogsChecked(userBlog.UserId.Hex(), "", 1, 5, userBlog.SortField, userBlog.IsAsc)
	if err != nil {
		return nil, err
	}
	args["recentPosts"] = blogService.FixBlogs(recentBlogs)
	args["latestPosts"] = args["recentPosts"]
	tags, err := blogService.GetBlogTagsChecked(userBlog.UserId.Hex())
	if err != nil {
		return nil, err
	}
	args["tags"] = tags
	if themeInfo := themeService.GetThemeInfo(userBlog.ThemeId.Hex(), userBlog.Style); themeInfo != nil {
		args["themeInfo"] = themeInfo
	} else {
		args["themeInfo"] = map[string]interface{}{}
	}
	return args, nil
}

func populateBlogPage(c *httpserver.Context, args map[string]interface{}, userBlog info.UserBlog, userID, userRef string) error {
	query, err := parseBlogPageQuery(c, userBlog, "", "")
	if err != nil {
		return service.ErrInvalidBlogQuery
	}
	switch c.Action {
	case "Index":
		page, blogs, err := blogService.ListBlogsChecked(userID, "", query.Page, query.PageSize, query.SortField, query.IsAsc)
		if err != nil {
			return err
		}
		args["posts"] = blogService.FixBlogs(blogs)
		args["paging"] = page
		args["pagingBaseUrl"] = args["indexUrl"]
		args["curIsIndex"] = true
	case "Tags":
		args["curIsTags"] = true
	case "Tag":
		tag := c.Params.String("tag")
		if tag == "" {
			tag = userRef
		}
		query, err = parseBlogPageQuery(c, userBlog, "", tag)
		if err != nil {
			return service.ErrInvalidBlogQuery
		}
		page, blogs, err := blogService.SearchBlogByTagsChecked([]string{query.Tag}, userID, query.Page, query.PageSize, query.SortField, query.IsAsc)
		if err != nil {
			return err
		}
		args["curIsTagPosts"] = true
		args["curTag"] = query.Tag
		args["posts"] = blogService.FixBlogs(blogs)
		args["paging"] = page
		args["pagingBaseUrl"] = args["tagPostsUrl"].(string) + "/" + query.Tag
	case "Archives":
		notebookID := c.Params.String("cateId")
		if notebookID == "" {
			notebookID = c.Params.String("notebookId")
		}
		archives, err := blogService.ListBlogsArchiveChecked(userID, notebookID, c.Params.Int("year", 0), c.Params.Int("month", 0), query.SortField, query.IsAsc)
		if err != nil {
			return err
		}
		args["archives"] = archives
		args["curIsArchive"] = true
		args["curYear"] = c.Params.Int("year", 0)
		args["curMonth"] = c.Params.Int("month", 0)
		if notebookID != "" {
			notebook := notebookService.GetNotebookById(notebookID)
			args["curCateTitle"] = notebook.Title
			args["curCateId"] = notebook.NotebookId.Hex()
		}
	case "Cate":
		if notebookService == nil {
			return fmt.Errorf("notebook service unavailable")
		}
		notebookID := c.Params.String("notebookId")
		notebook := notebookService.GetNotebookByUserIdAndUrlTitle(userID, notebookID)
		if notebook.UserId.IsZero() || !notebook.IsBlog {
			return service.ErrPublicBlogNotFound
		}
		page, blogs, err := blogService.ListBlogsChecked(userID, notebook.NotebookId.Hex(), query.Page, query.PageSize, query.SortField, query.IsAsc)
		if err != nil {
			return err
		}
		args["posts"] = blogService.FixBlogs(blogs)
		args["paging"] = page
		args["curCateTitle"] = notebook.Title
		args["curCateId"] = notebook.NotebookId.Hex()
		args["pagingBaseUrl"] = args["cateUrl"].(string) + "/" + notebookID
		args["curIsCate"] = true
	case "Post":
		noteID := c.Params.String("noteId")
		var blog info.BlogItem
		if c.Params.String("userIdOrEmail") == "" {
			blog, err = blogService.GetBlogChecked(noteID)
		} else {
			blog, err = blogService.GetBlogByIdAndUrlTitleChecked(userID, noteID)
		}
		if err != nil {
			return err
		}
		post := blogService.FixBlog(blog)
		args["post"] = post
		args["curIsPost"] = true
		var baseTime interface{}
		switch query.SortField {
		case "PublicTime":
			baseTime = blog.PublicTime
		case "CreatedTime":
			baseTime = blog.CreatedTime
		case "UpdatedTime":
			baseTime = blog.UpdatedTime
		default:
			baseTime = blog.Title
		}
		prePost, nextPost, err := blogService.PreNextBlogChecked(userID, query.SortField, query.IsAsc, post.NoteId, baseTime)
		if err != nil {
			return err
		}
		if prePost.NoteId != "" {
			args["prePost"] = prePost
		}
		if nextPost.NoteId != "" {
			args["nextPost"] = nextPost
		}
	case "Single":
		singleID := c.Params.String("singleId")
		var single info.BlogSingle
		if c.Params.String("userIdOrEmail") == "" {
			single, err = blogService.GetSingleChecked(singleID)
		} else {
			single, err = blogService.GetSingleByUserIdAndUrlTitleChecked(userID, singleID)
		}
		if err != nil {
			return err
		}
		args["single"] = map[string]interface{}{
			"SingleId":    single.SingleId.Hex(),
			"Title":       single.Title,
			"UrlTitle":    single.UrlTitle,
			"Content":     single.Content,
			"CreatedTime": single.CreatedTime,
			"UpdatedTime": single.UpdatedTime,
		}
		args["curSingleId"] = single.SingleId.Hex()
		args["curIsSingle"] = true
	case "Search":
		keywords := c.Params.String("keywords")
		if keywords == "" {
			keywords = c.Params.String("key")
		}
		query, err = parseBlogPageQuery(c, userBlog, keywords, "")
		if err != nil {
			return service.ErrInvalidBlogQuery
		}
		page, blogs, err := blogService.SearchBlogChecked(query.Keywords, userID, query.Page, query.PageSize, query.SortField, query.IsAsc)
		if err != nil {
			return err
		}
		args["keywords"] = query.Keywords
		args["posts"] = blogService.FixBlogs(blogs)
		args["paging"] = page
		args["pagingBaseUrl"] = args["searchUrl"].(string) + "?keywords=" + query.Keywords
		args["curIsSearch"] = true
	default:
		return service.ErrPublicBlogNotFound
	}
	return nil
}

func parseBlogPageQuery(c *httpserver.Context, userBlog info.UserBlog, keywords, tag string) (service.BlogQuery, error) {
	return service.ParseBlogQuery(service.BlogQueryInput{
		Page: c.Params.String("page"), PagePresent: c.Params.Has("page"),
		PageSize: c.Params.String("pageSize"), PageSizePresent: c.Params.Has("pageSize"),
		Sort: c.Params.String("sort"), SortPresent: c.Params.Has("sort"),
		Keywords: keywords, Tag: tag,
		ConfiguredPageSize: userBlog.PerPageSize, ConfiguredSort: userBlog.SortField, IsAsc: userBlog.IsAsc,
	})
}

func buildBlogCates(notebooks []info.Notebook, orderedIDs []string) ([]*info.Cate, []*info.Cate) {
	byID := make(map[string]info.Notebook, len(notebooks))
	for _, notebook := range notebooks {
		byID[notebook.NotebookId.Hex()] = notebook
	}
	ordered := make([]info.Notebook, 0, len(notebooks))
	seen := map[string]bool{}
	for _, id := range orderedIDs {
		if notebook, ok := byID[id]; ok {
			ordered = append(ordered, notebook)
			seen[id] = true
		}
	}
	for _, notebook := range notebooks {
		if !seen[notebook.NotebookId.Hex()] {
			ordered = append(ordered, notebook)
		}
	}
	cates := make([]*info.Cate, 0, len(ordered))
	byCateID := make(map[string]*info.Cate, len(ordered))
	for _, notebook := range ordered {
		id := notebook.NotebookId.Hex()
		parent := ""
		if !notebook.ParentNotebookId.IsZero() {
			parent = notebook.ParentNotebookId.Hex()
		}
		urlTitle := notebook.UrlTitle
		if urlTitle == "" {
			urlTitle = id
		}
		cate := &info.Cate{CateId: id, ParentCateId: parent, Title: notebook.Title, UrlTitle: urlTitle}
		cates = append(cates, cate)
		byCateID[id] = cate
	}
	children := map[string]bool{}
	for _, cate := range cates {
		if parent, ok := byCateID[cate.ParentCateId]; ok {
			parent.Children = append(parent.Children, cate)
			children[cate.CateId] = true
		}
	}
	roots := make([]*info.Cate, 0, len(cates))
	for _, cate := range cates {
		if !children[cate.CateId] {
			roots = append(roots, cate)
		}
	}
	return cates, roots
}
