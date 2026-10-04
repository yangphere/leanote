package member

// Native HTTP adapters for the member centre.  These handlers deliberately
// keep the old service calls as the source of business behaviour; this file
// only binds request values and converts them to first-party results.

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

// RegisterHTTP exposes the member actions without reflection.  It is kept in
// this package so the main registry can wire it once the member batch is ready.
func RegisterHTTP(rs *httpserver.Registry, session ...httpserver.BeforeFunc) {
	before := make([]httpserver.BeforeFunc, 0, len(session)+1)
	before = append(before, session...)
	before = append(before, memberAuthBefore)
	i := &memberIndexServer{}
	rs.Register("MemberIndex", "Index", before, i.Index)
	rs.Register("MemberIndex", "T", before, i.T)
	rs.Register("MemberIndex", "GetView", before, i.GetView)

	g := &memberGroupServer{}
	for name, fn := range map[string]httpserver.ActionFunc{
		"Index": g.Index, "AddGroup": g.AddGroup, "UpdateGroupTitle": g.UpdateGroupTitle,
		"DeleteGroup": g.DeleteGroup, "AddUser": g.AddUser, "DeleteUser": g.DeleteUser,
	} {
		rs.Register("MemberGroup", name, before, fn)
	}

	u := &memberUserServer{}
	for name, fn := range map[string]httpserver.ActionFunc{
		"Username": u.Username, "Email": u.Email, "Password": u.Password, "Avatar": u.Avatar,
	} {
		rs.Register("MemberUser", name, before, fn)
	}

	b := &memberBlogServer{}
	for name, fn := range map[string]httpserver.ActionFunc{
		"Index": b.Index, "UpdateBlogUrlTitle": b.UpdateBlogUrlTitle, "UpdateBlogAbstract": b.UpdateBlogAbstract,
		"DoUpdateBlogAbstract": b.DoUpdateBlogAbstract, "Base": b.Base, "Comment": b.Comment, "Paging": b.Paging,
		"Cate": b.Cate, "UpateCateIds": b.UpateCateIds, "UpdateCateUrlTitle": b.UpdateCateUrlTitle,
		"DoAddOrUpdateSingle": b.DoAddOrUpdateSingle, "AddOrUpdateSingle": b.AddOrUpdateSingle,
		"SortSingles": b.SortSingles, "DeleteSingle": b.DeleteSingle, "UpdateSingleUrlTitle": b.UpdateSingleUrlTitle,
		"Single": b.Single, "Theme": b.Theme, "UpdateTheme": b.UpdateTheme, "GetTplContent": b.GetTplContent,
		"UpdateTplContent": b.UpdateTplContent, "DeleteTpl": b.DeleteTpl, "ListThemeImages": b.ListThemeImages,
		"DeleteThemeImage": b.DeleteThemeImage, "UploadThemeImage": b.UploadThemeImage, "ActiveTheme": b.ActiveTheme,
		"DeleteTheme": b.DeleteTheme, "PublicTheme": b.PublicTheme, "ExportTheme": b.ExportTheme,
		"ImportTheme": b.ImportTheme, "InstallTheme": b.InstallTheme, "NewTheme": b.NewTheme,
		"SetUserBlogBase": b.SetUserBlogBase, "SetUserBlogComment": b.SetUserBlogComment,
		"SetUserBlogStyle": b.SetUserBlogStyle, "SetUserBlogPaging": b.SetUserBlogPaging,
	} {
		rs.Register("MemberBlog", name, before, fn)
	}
}

func memberAuthBefore(c *httpserver.Context) httpserver.Result {
	if c.GetPrincipal().UserID != "" {
		required, err := service.AdminPasswordChangeRequired(c.GetPrincipal().UserID)
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		if required {
			if c.Request != nil && strings.EqualFold(c.Request.Header.Get("X-Requested-With"), "XMLHttpRequest") {
				return c.RenderJSON(info.Re{Ok: false, Msg: "admin_password_change_required"})
			}
			return c.Redirect("/user/account?tab=2")
		}
		return nil
	}
	if c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		r := info.NewRe()
		r.Msg = "NOTLOGIN"
		return c.RenderJSON(r)
	}
	return c.Redirect("/login")
}

func uid(c *httpserver.Context) string {
	if principal := c.GetPrincipal(); principal.UserID != "" {
		return principal.UserID
	}
	v, _, _ := c.Get("_userId")
	if v == "" {
		v, _, _ = c.Get("UserId")
	}
	return v
}
func re(c *httpserver.Context, r info.Re) httpserver.Result { return c.RenderJSON(r) }
func template(c *httpserver.Context, name string) httpserver.Result {
	return c.RenderTemplate(name, memberViewArgs(c))
}

func memberViewArgs(c *httpserver.Context) map[string]interface{} {
	args := make(map[string]interface{}, len(c.ViewArgs)+7)
	for key, value := range c.ViewArgs {
		args[key] = value
	}
	args["currentLocale"] = c.Locale
	args["locale"] = c.Locale
	args["isAdmin"] = c.GetPrincipal().Role == httpserver.PrincipalRoleAdmin
	if userService != nil {
		args["userInfo"] = userService.GetUserInfo(uid(c))
	}
	return args
}

func memberBlogCommon(c *httpserver.Context, args map[string]interface{}) info.UserBlog {
	if userService != nil {
		args["userInfo"] = userService.GetUserInfo(uid(c))
	}
	if configService != nil {
		args["allowCustomDomain"] = configService.GetGlobalStringConfig("allowCustomDomain")
	}
	if blogService == nil {
		return info.UserBlog{}
	}
	blog := blogService.GetUserBlog(uid(c))
	args["userBlog"] = blog
	return blog
}

type memberIndexServer struct{}

func (s *memberIndexServer) Index(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	args["title"] = c.Message("Leanote Member Center")
	userId := uid(c)
	if userId != "" && db.IsValidObjectIDHex(userId) {
		if noteService != nil {
			args["countNote"] = noteService.CountNote(userId)
			args["countBlog"] = noteService.CountBlog(userId)
			_, recentNotes := noteService.ListNotes(userId, "", false, 1, 6, "UpdatedTime", false, false)
			args["recentNotes"] = recentNotes
		}
		if notebookService != nil {
			args["countNotebook"] = len(notebookService.GetActiveNotebooks(userId))
		}
		if groupService != nil {
			args["countGroup"] = len(groupService.GetGroups(userId))
		}
	}
	return c.RenderTemplate("member/index.html", args)
}
func (s *memberIndexServer) T(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	if configService != nil {
		args["str"] = configService.RedactedStringConfigs()
		args["arr"] = configService.GlobalArrayConfigs
		args["map"] = configService.GlobalMapConfigs
		args["arrMap"] = configService.GlobalArrMapConfigs
	}
	return c.RenderTemplate("admin/"+c.Params.String("t")+".html", args)
}
func (s *memberIndexServer) GetView(c *httpserver.Context) httpserver.Result {
	return c.RenderTemplate("admin/"+c.Params.String("view"), memberViewArgs(c))
}

type memberGroupServer struct{}

func (s *memberGroupServer) Index(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	args["title"] = c.Message("My Group")
	if groupService != nil {
		args["groups"] = groupService.GetGroupsAndUsers(uid(c))
	}
	return c.RenderTemplate("member/group/index.html", args)
}
func (s *memberGroupServer) AddGroup(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if groupService != nil {
		r.Ok, r.Item = groupService.AddGroup(uid(c), c.Params.String("title"))
	}
	return re(c, r)
}
func (s *memberGroupServer) UpdateGroupTitle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if groupService != nil {
		r.Ok = groupService.UpdateGroupTitle(uid(c), c.Params.String("groupId"), c.Params.String("title"))
	}
	return re(c, r)
}
func (s *memberGroupServer) DeleteGroup(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if groupService != nil {
		r.Ok, r.Msg = groupService.DeleteGroup(uid(c), c.Params.String("groupId"))
	}
	return re(c, r)
}
func (s *memberGroupServer) AddUser(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if userService == nil || groupService == nil {
		r.Msg = "storage"
		return re(c, r)
	}
	u := userService.GetUserInfoByAny(c.Params.String("email"))
	if u.UserId.IsZero() {
		r.Msg = "userNotExists"
	} else {
		r.Ok, r.Msg = groupService.AddUser(uid(c), c.Params.String("groupId"), u.UserId.Hex())
		r.Item = u
	}
	return re(c, r)
}
func (s *memberGroupServer) DeleteUser(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if groupService != nil {
		r.Ok, r.Msg = groupService.DeleteUser(uid(c), c.Params.String("groupId"), c.Params.String("userId"))
	}
	return re(c, r)
}

type memberUserServer struct{}

func (s *memberUserServer) Username(c *httpserver.Context) httpserver.Result {
	return template(c, "member/user/username.html")
}
func (s *memberUserServer) Email(c *httpserver.Context) httpserver.Result {
	return template(c, "member/user/email.html")
}
func (s *memberUserServer) Password(c *httpserver.Context) httpserver.Result {
	return template(c, "member/user/password.html")
}
func (s *memberUserServer) Avatar(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	if configService != nil {
		args["globalConfigs"] = configService.GetGlobalConfigForUser()
	}
	return c.RenderTemplate("member/user/avatar.html", args)
}

type memberBlogServer struct{}

func (s *memberBlogServer) Index(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	blog := memberBlogCommon(c, args)
	args["title"] = c.Message("Posts")
	args["keywords"] = c.Params.String("keywords")
	args["sorter"] = c.Params.String("sorter")
	if blogService != nil {
		field, asc := memberSorter(c.Params.String("sorter"))
		page, blogs := blogService.ListAllBlogs(uid(c), "", c.Params.String("keywords"), false, memberPage(c), 15, field, asc)
		args["pageInfo"], args["blogs"] = page, blogs
		if userService != nil {
			args["userAndBlog"] = userService.GetUserAndBlog(uid(c))
		}
	}
	_ = blog
	return c.RenderTemplate("member/blog/list.html", args)
}
func (s *memberBlogServer) UpdateBlogUrlTitle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok, r.Item = blogService.UpateBlogUrlTitle(uid(c), c.Params.String("noteId"), c.Params.String("urlTitle"))
	}
	return re(c, r)
}
func (s *memberBlogServer) UpdateBlogAbstract(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Update Post Abstract")
	if noteService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	note := noteService.GetNoteAndContent(c.Params.String("noteId"), uid(c))
	if !note.Note.IsBlog {
		return c.NotFound("")
	}
	args["note"], args["noteId"] = note, c.Params.String("noteId")
	return c.RenderTemplate("member/blog/update_abstract.html", args)
}
func (s *memberBlogServer) DoUpdateBlogAbstract(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.UpateBlogAbstract(uid(c), c.Params.String("noteId"), c.Params.String("imgSrc"), c.Params.String("desc"), c.Params.String("abstract"))
	}
	return re(c, r)
}
func (s *memberBlogServer) Base(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Blog Base Info")
	return c.RenderTemplate("member/blog/base.html", args)
}
func (s *memberBlogServer) Comment(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Comment")
	return c.RenderTemplate("member/blog/comment.html", args)
}
func (s *memberBlogServer) Paging(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Paging")
	return c.RenderTemplate("member/blog/paging.html", args)
}
func (s *memberBlogServer) Cate(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	blog := memberBlogCommon(c, args)
	args["title"] = c.Message("Category")
	if blogService != nil {
		notebooks := blogService.ListBlogNotebooks(uid(c))
		byID := make(map[string]info.Notebook, len(notebooks))
		for _, notebook := range notebooks {
			byID[notebook.NotebookId.Hex()] = notebook
		}
		ordered := make([]info.Notebook, 0, len(notebooks))
		seen := map[string]bool{}
		for _, id := range blog.CateIds {
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
		args["notebooks"] = ordered
	}
	return c.RenderTemplate("member/blog/cate.html", args)
}
func (s *memberBlogServer) UpateCateIds(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.UpateCateIds(uid(c), c.Params.Strings("cateIds"))
	}
	return re(c, r)
}
func (s *memberBlogServer) UpdateCateUrlTitle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok, r.Item = blogService.UpateCateUrlTitle(uid(c), c.Params.String("cateId"), c.Params.String("urlTitle"))
	}
	return re(c, r)
}
func (s *memberBlogServer) DoAddOrUpdateSingle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.AddOrUpdateSingle(uid(c), c.Params.String("singleId"), c.Params.String("title"), c.Params.String("content"))
	}
	return re(c, r)
}
func (s *memberBlogServer) AddOrUpdateSingle(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	singleID := c.Params.String("singleId")
	args["title"], args["singleId"] = c.Message("Add Single"), singleID
	if singleID != "" && blogService != nil {
		args["title"], args["single"] = c.Message("Update Single"), blogService.GetSingle(singleID)
	}
	return c.RenderTemplate("member/blog/add_single.html", args)
}
func (s *memberBlogServer) SortSingles(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.SortSingles(uid(c), c.Params.Strings("singleIds"))
	}
	return re(c, r)
}
func (s *memberBlogServer) DeleteSingle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.DeleteSingle(uid(c), c.Params.String("singleId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) UpdateSingleUrlTitle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok, r.Item = blogService.UpdateSingleUrlTitle(uid(c), c.Params.String("singleId"), c.Params.String("urlTitle"))
	}
	return re(c, r)
}
func (s *memberBlogServer) Single(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Single")
	if blogService != nil {
		args["singles"] = blogService.GetSingles(uid(c))
	}
	return c.RenderTemplate("member/blog/single.html", args)
}
func (s *memberBlogServer) Theme(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Theme")
	if themeService != nil {
		args["activeTheme"], args["otherThemes"] = themeService.GetUserThemes(uid(c))
		optionThemes, err := themeService.GetDefaultThemesChecked()
		if err != nil {
			return c.RenderText("theme error")
		}
		args["optionThemes"] = optionThemes
	}
	return c.RenderTemplate("member/blog/theme.html", args)
}
func (s *memberBlogServer) UpdateTheme(c *httpserver.Context) httpserver.Result {
	if themeService == nil || blogService == nil {
		return c.RenderText("storage")
	}
	userBlog := blogService.GetUserBlog(uid(c))
	themeID := c.Params.String("themeId")
	if themeID == "" {
		_, themeID = themeService.NewThemeForFirst(userBlog)
		return c.Redirect("/member/blog/updateTheme?themeId=" + themeID)
	}
	args := memberViewArgs(c)
	memberBlogCommon(c, args)
	args["title"] = c.Message("Update Theme")
	args["isNew"] = c.Params.Int("isNew", 0)
	args["themeId"] = themeID
	theme := themeService.GetTheme(uid(c), themeID)
	if theme.ThemeId.IsZero() {
		return c.NotFound("")
	}
	args["theme"] = theme
	path := themeService.GetThemeAbsolutePath(uid(c), themeID)
	if path == "" {
		return c.NotFound("")
	}
	base := []string{"header.html", "footer.html", "index.html", "cate.html", "search.html", "post.html", "single.html", "tags.html", "tag_posts.html", "archive.html", "share_comment.html", "404.html", "theme.json", "style.css", "blog.js"}
	seen := map[string]bool{}
	for _, name := range base {
		seen[name] = true
	}
	for _, entry := range listThemeDirectory(path) {
		if entry != "images" && !seen[entry] {
			base = append(base, entry)
		}
	}
	args["myTpls"] = base
	return c.RenderTemplate("member/blog/update_theme.html", args)
}
func (s *memberBlogServer) GetTplContent(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Item, r.Ok = themeService.ReadTplContent(uid(c), c.Params.String("themeId"), c.Params.String("filename"))
	}
	return re(c, r)
}
func (s *memberBlogServer) UpdateTplContent(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok, r.Msg = themeService.UpdateTplContent(uid(c), c.Params.String("themeId"), c.Params.String("filename"), c.Params.String("content"))
	}
	return re(c, r)
}
func (s *memberBlogServer) DeleteTpl(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.DeleteTpl(uid(c), c.Params.String("themeId"), c.Params.String("filename"))
	}
	return re(c, r)
}
func (s *memberBlogServer) ListThemeImages(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.List, r.Ok = themeService.ListThemeImages(uid(c), c.Params.String("themeId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) DeleteThemeImage(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.DeleteThemeImage(uid(c), c.Params.String("themeId"), c.Params.String("filename"))
	}
	return re(c, r)
}
func (s *memberBlogServer) UploadThemeImage(c *httpserver.Context) httpserver.Result {
	args := memberViewArgs(c)
	result := info.NewRe()
	if themeService == nil {
		result.Msg = "storage"
	} else {
		file, header, err := c.Params.FormFile("file")
		if err != nil || file == nil || header == nil {
			result.Msg = "fileRequired"
		} else {
			data, readErr := io.ReadAll(io.LimitReader(file, 5<<20+1))
			_ = file.Close()
			if readErr != nil || len(data) > 5<<20 {
				result.Msg = "图片大于5M"
			} else {
				ok, message := themeService.SaveThemeImage(uid(c), c.Params.String("themeId"), header.Filename, data)
				result.Ok, result.Msg, result.Id = ok, message, header.Filename
				if ok {
					result.Code = 1
				}
			}
		}
	}
	args["fileUrlPath"], args["resultCode"], args["resultMsg"] = result.Id, result.Code, result.Msg
	return c.RenderTemplate("file/blog_logo.html", args)
}
func (s *memberBlogServer) ActiveTheme(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.ActiveTheme(uid(c), c.Params.String("themeId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) DeleteTheme(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.DeleteTheme(uid(c), c.Params.String("themeId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) PublicTheme(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.PublicTheme(uid(c), c.Params.String("themeId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) ExportTheme(c *httpserver.Context) httpserver.Result {
	if themeService == nil {
		return c.RenderText("error")
	}
	ok, path := themeService.ExportTheme(uid(c), c.Params.String("themeId"))
	if !ok {
		return c.RenderText("error...")
	}
	return httpserver.FileResult(path, filepath.Base(path))
}
func (s *memberBlogServer) ImportTheme(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if themeService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	file, header, err := c.Params.FormFile("file")
	if err != nil || file == nil || header == nil {
		re.Msg = "Please upload zip file"
		return c.RenderJSON(re)
	}
	if !strings.EqualFold(filepath.Ext(filepath.Base(filepath.FromSlash(strings.ReplaceAll(header.Filename, "\\", "/")))), ".zip") {
		re.Msg = "Please upload zip file"
		_ = file.Close()
		return c.RenderJSON(re)
	}
	defer file.Close()
	root := themeService.GetThemeUploadTempPath(uid(c))
	if root == "" || os.MkdirAll(root, 0755) != nil {
		re.Msg = "error"
		return c.RenderJSON(re)
	}
	temp, err := os.CreateTemp(root, ".theme-*.zip")
	if err != nil {
		re.Msg = "error"
		return c.RenderJSON(re)
	}
	tempPath := temp.Name()
	const maxThemeArchiveBytes int64 = 10 << 20
	if _, err = io.Copy(temp, io.LimitReader(file, maxThemeArchiveBytes+1)); err != nil || func() bool { info, statErr := temp.Stat(); return statErr != nil || info.Size() > maxThemeArchiveBytes }() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		re.Msg = "fileUploadError"
		return c.RenderJSON(re)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		re.Msg = "fileUploadError"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = themeService.ImportTheme(uid(c), tempPath)
	return c.RenderJSON(re)
}
func (s *memberBlogServer) InstallTheme(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if themeService != nil {
		r.Ok = themeService.InstallTheme(uid(c), c.Params.String("themeId"))
	}
	return re(c, r)
}
func (s *memberBlogServer) NewTheme(c *httpserver.Context) httpserver.Result {
	if themeService == nil {
		return c.RenderText("error")
	}
	_, id := themeService.NewTheme(uid(c))
	return c.Redirect("/member/blog/updateTheme?isNew=1&themeId=" + id)
}
func (s *memberBlogServer) SetUserBlogBase(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.UpdateUserBlogBase(uid(c), info.UserBlogBase{Logo: c.Params.String("Logo"), Title: c.Params.String("Title"), SubTitle: c.Params.String("SubTitle")})
	}
	return re(c, r)
}
func (s *memberBlogServer) SetUserBlogComment(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.UpdateUserBlogComment(uid(c), info.UserBlogComment{CanComment: c.Params.Bool("CanComment", false), CommentType: c.Params.String("CommentType"), DisqusId: c.Params.String("DisqusId")})
	}
	return re(c, r)
}
func (s *memberBlogServer) SetUserBlogStyle(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.UpdateUserBlogStyle(uid(c), info.UserBlogStyle{Style: c.Params.String("Style"), Css: c.Params.String("Css")})
	}
	return re(c, r)
}
func (s *memberBlogServer) SetUserBlogPaging(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		n, _ := strconv.Atoi(c.Params.String("perPageSize"))
		r.Ok, r.Msg = blogService.UpdateUserBlogPaging(uid(c), n, c.Params.String("sortField"), c.Params.Bool("isAsc", false))
	}
	return re(c, r)
}

func memberPage(c *httpserver.Context) int {
	page, err := strconv.Atoi(c.Params.String("page"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func memberSorter(value string) (string, bool) {
	if value == "" {
		return "CreatedTime", false
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return "CreatedTime", false
	}
	allowed := map[string]string{"title": "Title", "urltitle": "UrlTitle", "updatedtime": "UpdatedTime", "publictime": "PublicTime", "createdtime": "CreatedTime"}
	field, ok := allowed[strings.ToLower(parts[0])]
	if !ok {
		return "CreatedTime", false
	}
	return field, parts[1] == "up"
}

func listThemeDirectory(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Name())
	}
	return result
}
