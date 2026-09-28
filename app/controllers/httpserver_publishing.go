package controllers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

var jsonpCallbackPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]*$`)
var commentSubmissionPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// RegisterPublishingHTTP wires the public publishing surface. The handlers
// deliberately use service seams directly; no reflection or legacy controller
// instance is involved in the new runtime.
func RegisterPublishingHTTP(rs *httpserver.Registry) {
	public := []httpserver.BeforeFunc{webSessionBefore}
	auth := []httpserver.BeforeFunc{webSessionBefore, requireWebAuthentication}
	b := &BlogHTTPServer{}
	for _, action := range []string{"Index", "E", "Tags", "Tag", "Archives", "Cate", "Post", "Single", "Search", "ListCateLatest", "GetPostStat", "GetLikes", "GetLikesAndComments", "IncReadNum", "LikePost", "LikeComment", "DeleteComment", "GetComments", "CommentPost"} {
		before := public
		if action == "CommentPost" || action == "LikePost" || action == "LikeComment" || action == "DeleteComment" {
			before = auth
		}
		rs.Register("Blog", action, before, b.dispatch)
	}
	p := &PreviewHTTPServer{}
	for _, action := range []string{"Index", "Tag", "Tags", "Archives", "Cate", "Post", "Single", "Search"} {
		rs.RegisterMethods("Preview", action, []string{http.MethodGet}, auth, p.dispatch)
	}
	s := &ShareHTTPServer{}
	for _, action := range []string{"AddShareNote", "AddShareNotebook", "AddShareNotebookGroup", "AddShareNoteGroup", "DeleteShareNote", "DeleteShareNotebook", "DeleteShareNotebookBySharedUser", "DeleteShareNotebookGroup", "DeleteShareNoteBySharedUser", "DeleteShareNoteGroup", "DeleteUserShareNoteAndNotebook", "GetShareNoteContent", "ListNotebookShareUserInfo", "ListNoteShareUserInfo", "ListShareNotes", "UpdateShareNotebookGroupPerm", "UpdateShareNotebookPerm", "UpdateShareNoteGroupPerm", "UpdateShareNotePerm"} {
		rs.Register("Share", action, auth, s.dispatch)
	}
}

func validJSONPCallback(callback string, optional bool) bool {
	if callback == "" {
		return optional
	}
	return jsonpCallbackPattern.MatchString(callback)
}

type BlogHTTPServer struct{}

func (s *BlogHTTPServer) dispatch(c *httpserver.Context) httpserver.Result {
	callback := c.Params.String("callback")
	jsonp := func(v interface{}) httpserver.Result {
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		return c.RenderJSONP(callback, v)
	}
	if blogService == nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	userID := c.GetPrincipal().UserID
	switch c.Action {
	case "GetPostStat":
		stat, err := blogService.GetBlogStatChecked(c.Params.String("noteId"))
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "not_found"})
		}
		return c.RenderJSON(info.Re{Ok: true, Item: stat})
	case "GetLikes":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		liked, more, err := blogService.ListLikedUsersChecked(c.Params.String("noteId"), false)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "not_found"})
		}
		like := false
		if userID != "" {
			like, _ = blogService.IsILikeItChecked(c.Params.String("noteId"), userID)
		}
		return c.RenderJSONP(callback, info.Re{Ok: true, Item: map[string]interface{}{"isILikeIt": like, "likedUsers": liked, "hasMoreLikedUser": more}})
	case "GetComments":
		if !validJSONPCallback(callback, true) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		page, comments, users, err := blogService.ListCommentsChecked(userID, c.Params.String("noteId"), pageParam(c), 15)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "not_found"})
		}
		value := info.Re{Ok: true, Item: map[string]interface{}{"pageInfo": page, "comments": comments, "commentUserInfo": users}}
		if callback != "" {
			return c.RenderJSONP(callback, value)
		}
		return c.RenderJSON(value)
	case "GetLikesAndComments":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		liked, more, err := blogService.ListLikedUsersChecked(c.Params.String("noteId"), false)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "not_found"})
		}
		page, comments, users, err := blogService.ListCommentsChecked(userID, c.Params.String("noteId"), pageParam(c), 15)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "not_found"})
		}
		return c.RenderJSONP(callback, info.Re{Ok: true, Item: map[string]interface{}{"likedUsers": liked, "hasMoreLikedUser": more, "pageInfo": page, "comments": comments, "commentUserInfo": users}})
	case "IncReadNum":
		return c.RenderJSON(info.Re{Ok: blogService.IncReadNum(c.Params.String("noteId"))})
	case "LikePost":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		ok, liked := blogService.LikeBlog(c.Params.String("noteId"), userID)
		return c.RenderJSONP(callback, info.Re{Ok: ok, Item: liked})
	case "LikeComment":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		v := c.Params.String("commentId")
		ok, liked, n := blogService.LikeComment(v, userID)
		return jsonp(info.Re{Ok: ok, Item: map[string]interface{}{"IsILikeIt": liked, "Num": n}})
	case "DeleteComment":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		return jsonp(info.Re{Ok: blogService.DeleteComment(c.Params.String("noteId"), c.Params.String("commentId"), userID)})
	case "CommentPost":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		submission := c.Params.String("submissionId")
		if !c.Params.Has("submissionId") || !commentSubmissionPattern.MatchString(submission) {
			return jsonp(info.Re{Ok: false, Msg: "validation"})
		}
		ok, comment := blogService.Comment(c.Params.String("noteId"), c.Params.String("toCommentId"), userID, c.Params.String("content"), submission)
		return jsonp(info.Re{Ok: ok, Item: comment})
	case "ListCateLatest":
		if !validJSONPCallback(callback, false) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		if notebookService == nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		notebook := notebookService.GetNotebookById(c.Params.String("notebookId"))
		if notebook.UserId.IsZero() || !notebook.IsBlog {
			return c.NotFound("")
		}
		userBlog, err := blogService.GetUserBlogChecked(notebook.UserId.Hex())
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		query, err := service.ParseBlogQuery(service.BlogQueryInput{
			Page: c.Params.String("page"), PagePresent: c.Params.Has("page"),
			PageSize: c.Params.String("pageSize"), PageSizePresent: c.Params.Has("pageSize"),
			Sort: c.Params.String("sort"), SortPresent: c.Params.Has("sort"),
			ConfiguredPageSize: userBlog.PerPageSize, ConfiguredSort: userBlog.SortField, IsAsc: userBlog.IsAsc,
		})
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
		}
		_, blogs, err := blogService.ListBlogsChecked(notebook.UserId.Hex(), notebook.NotebookId.Hex(), query.Page, query.PageSize, query.SortField, query.IsAsc)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		return c.RenderJSONP(callback, info.Re{Ok: true, List: blogs})
	default:
		return s.renderBlogPage(c)
	}
}

type PreviewHTTPServer struct{}

func (s *PreviewHTTPServer) dispatch(c *httpserver.Context) httpserver.Result {
	principal := c.GetPrincipal()
	if themeService == nil || blogService == nil || userService == nil || principal.UserID == "" {
		return c.NotFound("")
	}
	userRef := strings.TrimSpace(c.Params.String("userIdOrEmail"))
	if !previewOwnerMatches(principal.UserID, userRef) {
		return c.NotFound("")
	}
	themeID := strings.TrimSpace(c.Params.String("themeId"))
	if themeID == "" {
		themeID, _, _ = c.Get("themeId")
		themeID = strings.TrimSpace(themeID)
	}
	if themeID == "" {
		return c.NotFound("")
	}
	theme, err := themeService.ResolvePreviewTheme(principal.UserID, themeID)
	if err != nil {
		return httpserver.TextResult(http.StatusNotFound, "preview theme: "+err.Error())
	}
	if err := c.SetSession("themeId", themeID); err != nil {
		return c.RenderText("storage")
	}
	userInfo := userService.GetUserInfo(principal.UserID)
	if userInfo.UserId.IsZero() {
		return c.NotFound("")
	}
	userBlog, err := blogService.GetUserBlogChecked(principal.UserID)
	if err != nil || userBlog.UserId.IsZero() {
		return c.NotFound("")
	}
	userBlog.ThemeId = theme.ThemeId
	userBlog.ThemePath = theme.Path
	userBlog.Style = theme.Style
	args, err := buildBlogView(c, userBlog, userInfo)
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "preview view: "+err.Error())
	}
	args["isPreview"] = true
	args["themeId"] = themeID
	args["themeInfoPreview"] = theme.Info
	args["themePath"] = theme.Path
	args["userIdOrEmail"] = userRef
	if userRef == "" {
		userRef = principal.UserID
	}
	if err := populateBlogPage(c, args, userBlog, principal.UserID, userRef); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidBlogQuery):
			return httpserver.TextResult(http.StatusBadRequest, "invalid preview query")
		case errors.Is(err, service.ErrPublicBlogNotFound):
			return c.NotFound("")
		default:
			return httpserver.TextResult(http.StatusInternalServerError, "preview page: "+err.Error())
		}
	}
	name := strings.ToLower(c.Action) + ".html"
	body, err := themeService.RenderBlogTemplate(userBlog, name, args)
	if err != nil {
		if errors.Is(err, service.ErrThemePath) || errors.Is(err, service.ErrThemeTemplate) {
			return httpserver.TextResult(http.StatusInternalServerError, "preview template: "+err.Error())
		}
		return httpserver.TextResult(http.StatusInternalServerError, "preview template: "+err.Error())
	}
	return httpserver.HTMLResult(http.StatusOK, string(body))
}

func previewOwnerMatches(currentID, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" || requested == currentID {
		return true
	}
	if db.IsValidObjectIDHex(requested) || userService == nil {
		return false
	}
	return userService.GetUserInfoByAny(requested).UserId.Hex() == currentID
}

type ShareHTTPServer struct{}

func (s *ShareHTTPServer) dispatch(c *httpserver.Context) httpserver.Result {
	if shareService == nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	uid := c.GetPrincipal().UserID
	switch c.Action {
	case "AddShareNote", "AddShareNotebook":
		if userService == nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		result := map[string]info.Re{}
		emails := c.Params.Strings("emails")
		if len(emails) == 0 {
			emails = c.Params.Strings("emails[]")
		}
		for _, email := range emails {
			if email == "" {
				continue
			}
			toUserID := userService.GetUserId(email)
			if toUserID == "" {
				result[email] = info.Re{Ok: false, Msg: "无该用户"}
				continue
			}
			var err error
			if c.Action == "AddShareNote" {
				err = shareService.AddShareNoteToUserIdWithOptions(c.Params.String("noteId"), c.Params.Int("perm", 0), uid, toUserID, serviceShareOptions(c))
			} else {
				err = shareService.AddShareNotebookToUserIdWithOptions(c.Params.String("notebookId"), c.Params.Int("perm", 0), uid, toUserID, serviceShareOptions(c))
			}
			result[email] = info.Re{Ok: err == nil, Id: toUserID}
			if err != nil {
				result[email] = info.Re{Ok: false, Id: toUserID, Msg: err.Error()}
			}
		}
		return c.RenderJSON(result)
	case "GetShareNoteContent":
		v, err := shareService.GetShareNoteContentChecked(c.Params.String("noteId"), uid, c.Params.String("sharedUserId"))
		if err != nil {
			return c.NotFound("")
		}
		return c.RenderJSON(v)
	case "ListShareNotes":
		var v interface{}
		var err error
		if c.Params.String("notebookId") == "" {
			v, err = shareService.ListShareNotesChecked(uid, c.Params.String("userId"), pageParam(c), pageSize, defaultSortField, false)
		} else {
			v, err = shareService.ListShareNotesByNotebookIdChecked(c.Params.String("notebookId"), uid, c.Params.String("userId"), pageParam(c), pageSize, defaultSortField, false)
		}
		if err != nil {
			if errors.Is(err, service.ErrShareResource) {
				// The legacy action represented an absent or unauthorized
				// shared notebook as a null JSON list. The checked service
				// remains fail-closed and returns no notes.
				return c.RenderJSON(nil)
			}
			return c.NotFound("")
		}
		if notes, ok := v.([]info.ShareNoteWithPerm); ok && len(notes) == 0 {
			// The legacy controller encoded an empty shared-note result as null.
			// Keep that wire shape while the checked service may use an empty
			// slice internally for callers that prefer a non-nil collection.
			return c.RenderJSON(nil)
		}
		return c.RenderJSON(v)
	case "DeleteShareNote":
		return c.RenderJSON(shareService.DeleteShareNote(c.Params.String("noteId"), uid, c.Params.String("toUserId")))
	case "DeleteShareNotebook":
		return c.RenderJSON(shareService.DeleteShareNotebook(c.Params.String("notebookId"), uid, c.Params.String("toUserId")))
	case "DeleteShareNoteBySharedUser":
		return c.RenderJSON(shareService.DeleteShareNote(c.Params.String("noteId"), c.Params.String("fromUserId"), uid))
	case "DeleteShareNotebookBySharedUser":
		return c.RenderJSON(shareService.DeleteShareNotebook(c.Params.String("notebookId"), c.Params.String("fromUserId"), uid))
	case "DeleteUserShareNoteAndNotebook":
		return c.RenderJSON(shareService.DeleteUserShareNoteAndNotebook(c.Params.String("fromUserId"), uid))
	case "UpdateShareNotePerm":
		return c.RenderJSON(shareService.UpdateShareNotePermWithOptions(c.Params.String("noteId"), c.Params.Int("perm", 0), uid, c.Params.String("toUserId"), serviceShareOptions(c)))
	case "UpdateShareNotebookPerm":
		return c.RenderJSON(shareService.UpdateShareNotebookPermWithOptions(c.Params.String("notebookId"), c.Params.Int("perm", 0), uid, c.Params.String("toUserId"), serviceShareOptions(c)))
	case "AddShareNoteGroup", "UpdateShareNoteGroupPerm":
		return c.RenderJSON(info.Re{Ok: shareService.AddShareNoteGroupWithOptions(uid, c.Params.String("noteId"), c.Params.String("groupId"), c.Params.Int("perm", 0), serviceShareOptions(c)) == nil})
	case "DeleteShareNoteGroup":
		return c.RenderJSON(info.Re{Ok: shareService.DeleteShareNoteGroup(uid, c.Params.String("noteId"), c.Params.String("groupId"))})
	case "AddShareNotebookGroup", "UpdateShareNotebookGroupPerm":
		return c.RenderJSON(info.Re{Ok: shareService.AddShareNotebookGroupWithOptions(uid, c.Params.String("notebookId"), c.Params.String("groupId"), c.Params.Int("perm", 0), serviceShareOptions(c)) == nil})
	case "DeleteShareNotebookGroup":
		return c.RenderJSON(info.Re{Ok: shareService.DeleteShareNotebookGroup(uid, c.Params.String("notebookId"), c.Params.String("groupId"))})
	default:
		return c.RenderJSON(info.Re{Ok: false, Msg: "validation"})
	}
}

func serviceShareOptions(c *httpserver.Context) service.ShareGrantOptions {
	return service.ShareGrantOptions{Now: time.Now().UTC(), ClearExpiresAt: c.Params.Bool("clearExpiresAt", false)}
}
