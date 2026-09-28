package admin

// Standard library adapters for the administrator surface.  The adapter is
// deliberately explicit: every action is registered by name and all writes
// pass through the existing application services and admin principal policy.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
)

// HTTPDeps is the read-only startup handoff consumed by admin actions.
// Keeping this value typed prevents admin code from reparsing credentials or
// filesystem configuration.
type HTTPDeps struct {
	Production    *httpserver.ProductionConfig
	SessionBefore httpserver.BeforeFunc
}

type server struct{ deps HTTPDeps }

var userPageSize = 10

func allowedAdminTemplate(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\\\x00\r\n") || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return false
	}
	_, ok := map[string]struct{}{
		"email/set": {}, "email/template": {}, "email/sendToUsers": {}, "email/send": {},
		"setting/site_url": {}, "setting/home_page": {}, "setting/demo": {}, "setting/open_register": {},
		"setting/share_note": {}, "setting/upload": {}, "setting/export_pdf": {}, "data/configuration": {},
		"upgrade/beta2": {}, "upgrade/beta3": {},
	}[name]
	return ok
}

// RegisterHTTP exposes the complete admin action inventory.  The variadic
// dependency form keeps tests small while allowing the production entrypoint
// to pass the validated ProductionConfig.
func RegisterHTTP(rs *httpserver.Registry, deps ...HTTPDeps) {
	s := &server{}
	var sessionBefore httpserver.BeforeFunc
	if len(deps) != 0 {
		s.deps = deps[0]
		sessionBefore = deps[0].SessionBefore
	}
	if len(deps) > 1 {
		sessionBefore = deps[1].SessionBefore
	}
	before := []httpserver.BeforeFunc{requireAdminHTTP}
	if sessionBefore != nil {
		before = []httpserver.BeforeFunc{sessionBefore, requireAdminHTTP}
	}
	reg := func(controller, action string, h httpserver.ActionFunc) { rs.Register(controller, action, before, h) }
	reg("Admin", "Index", s.adminIndex)
	reg("Admin", "T", s.adminTemplate)
	reg("Admin", "GetView", s.adminView)
	reg("AdminBlog", "Index", s.blogIndex)
	reg("AdminBlog", "SetRecommend", s.setRecommend)
	reg("AdminData", "Index", s.dataIndex)
	reg("AdminData", "Backup", s.backup)
	reg("AdminData", "Restore", s.restore)
	reg("AdminData", "Delete", s.deleteBackup)
	reg("AdminData", "UpdateRemark", s.updateRemark)
	reg("AdminData", "Download", s.download)
	reg("AdminUser", "Index", s.userIndex)
	reg("AdminUser", "Add", s.userAdd)
	reg("AdminUser", "Register", s.userRegister)
	reg("AdminUser", "ResetPwd", s.resetPwd)
	reg("AdminUser", "DoResetPwd", s.doResetPwd)
	reg("AdminUpgrade", "UpgradeBlog", s.upgradeBlog)
	reg("AdminUpgrade", "UpgradeBetaToBeta2", s.upgradeBeta2)
	reg("AdminUpgrade", "UpgradeBeta3ToBeta4", s.upgradeBeta4)
	for _, a := range []string{"Email", "Blog", "Demo", "ToImage", "SendEmailDialog", "List"} {
		reg("AdminEmail", a, s.emailView)
	}
	for _, a := range []string{"DoBlogTag", "DoDemo", "DoToImage", "Set", "Template", "SendEmailToEmails", "SendToUsers", "SendToUsers2", "DeleteEmails"} {
		reg("AdminEmail", a, s.emailWrite)
	}
	for _, a := range []string{"Email", "Blog", "Demo", "SubDomain"} {
		reg("AdminSetting", a, s.settingView)
	}
	for _, a := range []string{"DoBlogTag", "DoDemo", "DoSiteUrl", "DoSubDomain", "ExportPdf", "OpenRegister", "HomePage", "Mongodb", "FeedbackRecipients", "MongoExecutableAllowlist", "PdfExecutableAllowlist", "UploadSize", "ShareNote"} {
		reg("AdminSetting", a, s.settingWrite)
	}
}

func requireAdminHTTP(c *httpserver.Context) httpserver.Result {
	if c.GetPrincipal().Role == httpserver.PrincipalRoleAdmin {
		return nil
	}
	if c.Request != nil && strings.EqualFold(c.Request.Header.Get("X-Requested-With"), "XMLHttpRequest") {
		return c.RenderJSON(info.Re{Ok: false, Msg: "NOTLOGIN"})
	}
	return c.Redirect("/login")
}

func (s *server) viewArgs(c *httpserver.Context) map[string]interface{} {
	args := make(map[string]interface{}, len(c.ViewArgs)+8)
	for key, value := range c.ViewArgs {
		args[key] = value
	}
	if userService != nil && c.GetPrincipal().UserID != "" {
		args["userInfo"] = userService.GetUserInfo(c.GetPrincipal().UserID)
	}
	if configService != nil {
		args["str"] = configService.RedactedStringConfigs()
		args["arr"] = configService.GlobalArrayConfigs
		args["map"] = configService.GlobalMapConfigs
		args["arrMap"] = configService.GlobalArrMapConfigs
		args["version"] = configService.GetVersion()
	}
	args["currentLocale"] = c.Locale
	args["locale"] = c.Locale
	return args
}

func pageParam(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

func adminSorter(value, defaultField string, allowed []string) (string, bool) {
	if value == "" {
		return defaultField, false
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return defaultField, false
	}
	field := strings.ToLower(parts[0])
	for _, candidate := range allowed {
		if field == strings.ToLower(candidate) {
			return strings.ToUpper(field[:1]) + field[1:], parts[1] == "up"
		}
	}
	return defaultField, false
}

func (s *server) adminIndex(c *httpserver.Context) httpserver.Result {
	args := s.viewArgs(c)
	args["title"] = "leanote"
	if userService != nil {
		args["countUser"] = userService.CountUser()
	}
	if noteService != nil {
		args["countNote"] = noteService.CountNote("")
		args["countBlog"] = noteService.CountBlog("")
	}
	return c.RenderTemplate("admin/index.html", args)
}
func (s *server) adminTemplate(c *httpserver.Context) httpserver.Result {
	t := c.Params.String("t")
	if !allowedAdminTemplate(t) {
		return c.NotFound("admin template not found")
	}
	args := s.viewArgs(c)
	if configService != nil {
		args["version"] = configService.GetVersion()
	}
	return c.RenderTemplate("admin/"+t+".html", args)
}
func (s *server) adminView(c *httpserver.Context) httpserver.Result {
	v := c.Params.String("view")
	if !allowedAdminTemplate(v) {
		return c.NotFound("admin view not found")
	}
	return c.RenderTemplate("admin/"+v+".html", s.viewArgs(c))
}
func (s *server) blogIndex(c *httpserver.Context) httpserver.Result {
	if blogService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	sorterField, isAsc := adminSorter(c.Params.String("sorter"), "CreatedTime", []string{"title", "userId", "isRecommed", "createdTime"})
	p, b := blogService.ListAllBlogs("", "", c.Params.String("keywords"), false, pageParam(c.Params.Int("page", 1)), userPageSize, sorterField, isAsc)
	args := s.viewArgs(c)
	args["pageInfo"], args["blogs"] = p, b
	args["keywords"], args["sorter"] = c.Params.String("keywords"), c.Params.String("sorter")
	return c.RenderTemplate("admin/blog/list.html", args)
}
func (s *server) setRecommend(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if blogService != nil {
		r.Ok = blogService.SetRecommend(c.Params.String("noteId"), c.Params.Bool("recommend", false))
	}
	return c.RenderJSON(r)
}
func (s *server) dataIndex(c *httpserver.Context) httpserver.Result {
	if configService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	backups := configService.GetGlobalArrMapConfig("backups")
	ordered := make([]map[string]string, len(backups))
	for i := range backups {
		ordered[len(backups)-1-i] = backups[i]
	}
	args := s.viewArgs(c)
	args["backups"] = ordered
	return c.RenderTemplate("admin/data/index.html", args)
}
func (s *server) backup(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if configService != nil {
		r.Ok, r.Msg = configService.Backup(c.Params.String("remark"))
	}
	return c.RenderJSON(r)
}
func (s *server) restore(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if configService != nil {
		r.Ok, r.Msg = configService.Restore(c.Params.String("createdTime"))
	}
	return c.RenderJSON(r)
}
func (s *server) deleteBackup(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if configService != nil {
		r.Ok, r.Msg = configService.DeleteBackup(c.Params.String("createdTime"))
	}
	return c.RenderJSON(r)
}
func (s *server) updateRemark(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if configService != nil {
		r.Ok, r.Msg = configService.UpdateBackupRemark(c.Params.String("createdTime"), c.Params.String("remark"))
	}
	return c.RenderJSON(r)
}
func (s *server) download(c *httpserver.Context) httpserver.Result {
	if s.deps.Production == nil || configService == nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	backupID := c.Params.String("createdTime")
	backup, ok := configService.GetBackup(backupID)
	if !ok {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	root := s.deps.Production.BackupRoot
	registered, err := service.ValidateContainedPath(root, backup["path"])
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	path, err := service.ValidateContainedPath(root, filepath.Join(registered, s.deps.Production.DatabaseName))
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	files, _, err := service.StableBackupPaths(path, service.DefaultBackupLimits)
	if err != nil || len(files) == 0 {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	if service.ValidateOpaqueBackupID(backupID) != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	databaseName := s.deps.Production.DatabaseName
	if databaseName == "" || filepath.Base(databaseName) != databaseName || strings.ContainsAny(databaseName, "\\/\x00\r\n") {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	filename := "backup_" + databaseName + "_" + backupID + ".tar.gz"
	tempDir := filepath.Join(root, ".downloads")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	archiveFile, err := os.CreateTemp(tempDir, ".backup-*.tar.gz")
	if err != nil {
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	tempName := archiveFile.Name()
	cleanup := func() {
		_ = archiveFile.Close()
		_ = os.Remove(tempName)
	}
	gz := gzip.NewWriter(archiveFile)
	tw := tar.NewWriter(gz)
	for _, rel := range files {
		full, e := service.ValidateContainedPath(path, filepath.Join(path, filepath.FromSlash(rel)))
		if e != nil {
			cleanup()
			return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
		}
		f, e := os.Open(full)
		if e != nil {
			cleanup()
			return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
		}
		st, e := f.Stat()
		if e != nil || !st.Mode().IsRegular() {
			_ = f.Close()
			cleanup()
			return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
		}
		if e = tw.WriteHeader(&tar.Header{Name: rel, Size: st.Size(), Mode: int64(st.Mode()), ModTime: st.ModTime()}); e == nil {
			_, e = io.Copy(tw, f)
		}
		_ = f.Close()
		if e != nil {
			cleanup()
			return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
		}
		if stat, statErr := archiveFile.Stat(); statErr != nil || stat.Size() > service.DefaultBackupLimits.MaxArchiveBytes {
			cleanup()
			return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
		}
	}
	if err := tw.Close(); err != nil {
		cleanup()
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	if err := archiveFile.Close(); err != nil {
		_ = os.Remove(tempName)
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	stat, err := os.Stat(tempName)
	if err != nil || stat.Size() > service.DefaultBackupLimits.MaxArchiveBytes {
		_ = os.Remove(tempName)
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	reader, err := os.Open(tempName)
	if err != nil {
		_ = os.Remove(tempName)
		return httpserver.TextResult(http.StatusInternalServerError, "backup download unavailable")
	}
	return &backupDownloadResult{reader: reader, path: tempName, filename: filename}
}

type backupDownloadResult struct {
	reader   *os.File
	path     string
	filename string
}

func (r *backupDownloadResult) Apply(w http.ResponseWriter, _ *http.Request) {
	defer func() {
		_ = r.reader.Close()
		_ = os.Remove(r.path)
	}()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", httpserver.DownloadDisposition("attachment", r.filename))
	if info, err := r.reader.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, r.reader)
}
func (s *server) userIndex(c *httpserver.Context) httpserver.Result {
	if userService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	pageSize := c.Params.Int("pageSize", userPageSize)
	if pageSize == 0 {
		pageSize = userPageSize
	}
	sorterField, isAsc := adminSorter(c.Params.String("sorter"), "CreatedTime", []string{"email", "username", "verified", "createdTime", "accountType"})
	p, u := userService.ListUsers(pageParam(c.Params.Int("page", 1)), pageSize, sorterField, isAsc, c.Params.String("keywords"))
	args := s.viewArgs(c)
	args["pageInfo"], args["users"] = p, u
	args["keywords"], args["sorter"] = c.Params.String("keywords"), c.Params.String("sorter")
	return c.RenderTemplate("admin/user/list.html", args)
}
func (s *server) userAdd(c *httpserver.Context) httpserver.Result {
	return c.RenderTemplate("admin/user/add.html", s.viewArgs(c))
}
func (s *server) userRegister(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if authService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	if r.Ok, r.Msg = Vd("email", c.Params.String("email")); !r.Ok {
		return c.RenderJSON(r)
	}
	if r.Ok, r.Msg = Vd("password", c.Params.String("pwd")); !r.Ok {
		return c.RenderJSON(r)
	}
	r.Ok, r.Msg = authService.Register(c.Params.String("email"), c.Params.String("pwd"), "")
	return c.RenderJSON(r)
}
func (s *server) resetPwd(c *httpserver.Context) httpserver.Result {
	args := s.viewArgs(c)
	if userService != nil {
		args["userInfo"] = userService.GetUserInfo(c.Params.String("userId"))
	}
	return c.RenderTemplate("admin/user/reset_pwd.html", args)
}
func (s *server) doResetPwd(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if userService != nil {
		r.Ok, r.Msg = userService.ResetPwd(c.GetPrincipal().UserID, c.Params.String("userId"), c.Params.String("pwd"))
	}
	return c.RenderJSON(r)
}
func (s *server) upgradeBlog(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if upgradeService != nil {
		r.Ok, r.Msg = upgradeService.UpgradeBlog()
	}
	return c.RenderJSON(r)
}
func (s *server) upgradeBeta2(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if upgradeService != nil {
		r.Ok, r.Msg = upgradeService.UpgradeBetaToBeta2(c.GetPrincipal().UserID)
	}
	return c.RenderJSON(r)
}
func (s *server) upgradeBeta4(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if upgradeService != nil {
		r.Ok, r.Msg = upgradeService.Api(c.GetPrincipal().UserID)
	}
	return c.RenderJSON(r)
}
func (s *server) emailView(c *httpserver.Context) httpserver.Result {
	args := s.viewArgs(c)
	var templateName string
	switch c.Action {
	case "Email":
		templateName = "admin/email/set.html"
	case "Blog":
		templateName = "admin/setting/blog.html"
		if configService != nil {
			args["recommendTags"] = strings.Join(configService.GetGlobalArrayConfig("recommendTags"), ",")
			args["newTags"] = strings.Join(configService.GetGlobalArrayConfig("newTags"), ",")
		}
	case "Demo":
		templateName = "admin/setting/demo.html"
		if configService != nil {
			args["demoUsername"] = configService.GetGlobalStringConfig("demoUsername")
			args["demoPassword"] = service.RedactedSecretValue
		}
	case "ToImage":
		templateName = "admin/setting/toImage.html"
		if configService != nil {
			args["toImageBinPath"] = configService.GetGlobalStringConfig("toImageBinPath")
		}
	case "SendEmailDialog":
		templateName = "admin/email/emailDialog.html"
		emails := strings.Split(c.Params.String("emails"), ",")
		args["emailsNl"] = strings.Join(emails, "\n")
	case "List":
		templateName = "admin/email/list.html"
		if emailService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		sorterField, isAsc := adminSorter(c.Params.String("sorter"), "CreatedTime", []string{"email", "ok", "subject", "createdTime"})
		pageInfo, emails := emailService.ListEmailLogs(pageParam(c.Params.Int("page", 1)), userPageSize, sorterField, isAsc, c.Params.String("keywords"))
		args["pageInfo"], args["emails"] = pageInfo, emails
		args["keywords"], args["sorter"] = c.Params.String("keywords"), c.Params.String("sorter")
	default:
		return c.RenderJSON(info.Re{Ok: false, Msg: "admin.action_not_implemented"})
	}
	return c.RenderTemplate(templateName, args)
}
func (s *server) emailWrite(c *httpserver.Context) httpserver.Result {
	if configService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	switch c.Action {
	case "DoBlogTag":
		return c.RenderJSON(s.updateStringLists(c, map[string][]string{
			"recommendTags": splitCSVParams(c.Params.Strings("recommendTags")),
			"newTags":       splitCSVParams(c.Params.Strings("newTags")),
		}))
	case "DoDemo":
		return c.RenderJSON(s.updateDemo(c))
	case "DoToImage":
		return c.RenderJSON(s.updateStrings(c, map[string]string{"toImageBinPath": c.Params.String("toImageBinPath")}))
	case "Set":
		return c.RenderJSON(s.updateStrings(c, map[string]string{
			"emailHost": c.Params.String("emailHost"), "emailPort": c.Params.String("emailPort"),
			"emailUsername": c.Params.String("emailUsername"), "emailPassword": c.Params.String("emailPassword"),
			"emailSSL": c.Params.String("emailSSL"),
		}))
	case "Template":
		return c.RenderJSON(s.updateEmailTemplates(c))
	case "SendEmailToEmails":
		if result := s.persistEmailSendState(c, []string{"sendEmails", "latestEmailSubject", "latestEmailBody"}); !result.Ok {
			return c.RenderJSON(result)
		}
		if result := s.saveOldEmail(c); !result.Ok {
			return c.RenderJSON(result)
		}
		return c.RenderJSON(s.sendToRecipients(c, splitLines(c.Params.String("sendEmails"))))
	case "SendToUsers2":
		emails := c.Params.String("emails")
		if userService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		if result := s.persistEmailSendState(c, []string{"sendEmails", "latestEmailSubject", "latestEmailBody"}); !result.Ok {
			return c.RenderJSON(result)
		}
		if result := s.saveOldEmail(c); !result.Ok {
			return c.RenderJSON(result)
		}
		users := userService.ListUserInfosByEmails(splitLines(emails))
		recipients := make([]string, 0, len(users))
		for _, user := range users {
			recipients = append(recipients, user.Email)
		}
		return c.RenderJSON(s.sendToRecipients(c, recipients))
	case "SendToUsers":
		if userService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		if result := s.persistEmailSendState(c, []string{"userFilterEmail", "userFilterWhiteList", "userFilterBlackList", "latestEmailSubject", "latestEmailBody"}); !result.Ok {
			return c.RenderJSON(result)
		}
		if result := s.saveOldEmail(c); !result.Ok {
			return c.RenderJSON(result)
		}
		users := userService.GetAllUserByFilter(c.Params.String("userFilterEmail"), c.Params.String("userFilterWhiteList"), c.Params.String("userFilterBlackList"), c.Params.Bool("verified", false))
		if len(users) == 0 {
			return c.RenderJSON(info.Re{Msg: "no users"})
		}
		recipients := make([]string, 0, len(users))
		for _, user := range users {
			recipients = append(recipients, user.Email)
		}
		result := s.sendToRecipients(c, recipients)
		if result.Ok {
			result.Msg = "users:" + strconv.Itoa(len(users))
		}
		return c.RenderJSON(result)
	case "DeleteEmails":
		if emailService == nil {
			return c.RenderJSON(info.Re{Msg: "storage"})
		}
		return c.RenderJSON(info.Re{Ok: emailService.DeleteEmails(splitLines(c.Params.String("ids")))})
	default:
		return c.RenderJSON(info.Re{Ok: false, Msg: "admin.action_not_implemented"})
	}
}

func (s *server) persistEmailSendState(c *httpserver.Context, keys []string) info.Re {
	result := info.NewRe()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = c.Params.String(key)
	}
	if err := configService.UpdateGlobalStringConfigs(c.GetPrincipal().UserID, values).Err(); err != nil {
		result.Msg = "admin.config_update_failed"
	}
	result.Ok = result.Msg == ""
	return result
}

func (s *server) saveOldEmail(c *httpserver.Context) info.Re {
	result := info.NewRe()
	if !c.Params.Bool("saveAsOldEmail", false) {
		result.Ok = true
		return result
	}
	subject := c.Params.String("latestEmailSubject")
	body := c.Params.String("latestEmailBody")
	// The legacy actions validate these fields before persisting an old
	// template. Keep blank submissions from creating an empty history entry;
	// sendToRecipients will return the published validation message afterward.
	if subject == "" || body == "" {
		result.Ok = true
		return result
	}
	oldEmails := configService.GetGlobalMapConfig("oldEmails")
	oldEmails[subject] = body
	if !configService.UpdateGlobalMapConfig(c.GetPrincipal().UserID, "oldEmails", oldEmails) {
		result.Msg = "old email template persistence failed"
		return result
	}
	result.Ok = true
	return result
}

func (s *server) updateStringLists(c *httpserver.Context, values map[string][]string) info.Re {
	result := info.NewRe()
	mutation := configService.UpdateGlobalConfigs(c.GetPrincipal().UserID, nil, values)
	result.Ok = mutation.Err() == nil
	if !result.Ok {
		result.Msg = "admin.config_update_failed"
	}
	return result
}

func (s *server) updateStrings(c *httpserver.Context, values map[string]string) info.Re {
	result := info.NewRe()
	mutation := configService.UpdateGlobalStringConfigs(c.GetPrincipal().UserID, values)
	result.Ok = mutation.Err() == nil
	if !result.Ok {
		result.Msg = "admin.config_update_failed"
	}
	return result
}

func (s *server) updateDemo(c *httpserver.Context) info.Re {
	result := info.NewRe()
	if authService == nil {
		result.Msg = "storage"
		return result
	}
	username, password := c.Params.String("demoUsername"), c.Params.String("demoPassword")
	user, err := authService.Login(username, password)
	if err != nil || user.UserId.IsZero() {
		result.Msg = "The User is Not Exists"
		return result
	}
	return s.updateStrings(c, map[string]string{"demoUserId": user.UserId.Hex(), "demoUsername": username, "demoPassword": password})
}

func (s *server) updateEmailTemplates(c *httpserver.Context) info.Re {
	result := info.NewRe()
	keys := []string{"emailTemplateHeader", "emailTemplateFooter", "emailTemplateRegisterSubject", "emailTemplateRegister", "emailTemplateFindPasswordSubject", "emailTemplateFindPassword", "emailTemplateUpdateEmailSubject", "emailTemplateUpdateEmail", "emailTemplateInviteSubject", "emailTemplateInvite", "emailTemplateCommentSubject", "emailTemplateComment"}
	values := make(map[string]string)
	for _, key := range keys {
		value := c.Params.String(key)
		if value == "" {
			continue
		}
		if emailService == nil {
			result.Msg = "storage"
			return result
		}
		if ok, msg := emailService.ValidTpl(value); !ok {
			result.Msg = "Error key: " + key + "<br />" + msg
			return result
		}
		values[key] = value
	}
	if len(values) == 0 {
		result.Ok = true
		return result
	}
	return s.updateStrings(c, values)
}

func (s *server) sendToRecipients(c *httpserver.Context, recipients []string) info.Re {
	result := info.NewRe()
	subject, body := c.Params.String("latestEmailSubject"), c.Params.String("latestEmailBody")
	if subject == "" || body == "" {
		result.Msg = "subject or body is blank"
		return result
	}
	if emailService == nil {
		result.Msg = "storage"
		return result
	}
	actor, err := domain.ParseObjectID(c.GetPrincipal().UserID)
	if err != nil || actor.IsZero() {
		result.Msg = "storage"
		return result
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	submission, err := emailService.EnqueueBroadcast(ctx, actor, c.Params.String("batchId"), recipients, subject, body)
	if err != nil {
		result.Msg = "admin.mail_enqueue_failed"
		return result
	}
	result.Ok = true
	result.Id = submission.ReconciliationID
	return result
}

func splitCSVParams(values []string) []string {
	var result []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
	}
	return result
}

func splitLines(value string) []string {
	value = strings.ReplaceAll(value, "\r", "")
	return splitCSVParams(strings.Split(value, "\n"))
}
func (s *server) settingView(c *httpserver.Context) httpserver.Result {
	args := s.viewArgs(c)
	if configService != nil {
		switch c.Action {
		case "Blog":
			args["recommendTags"] = strings.Join(configService.GetGlobalArrayConfig("recommendTags"), ",")
			args["newTags"] = strings.Join(configService.GetGlobalArrayConfig("newTags"), ",")
		case "Demo":
			args["demoUsername"] = configService.GetGlobalStringConfig("demoUsername")
			args["demoPassword"] = service.RedactedSecretValue
		case "SubDomain":
			args["noteSubDomain"] = configService.GetGlobalStringConfig("noteSubDomain")
			args["blogSubDomain"] = configService.GetGlobalStringConfig("blogSubDomain")
			args["leaSubDomain"] = configService.GetGlobalStringConfig("leaSubDomain")
		}
	}
	return c.RenderTemplate("admin/setting/"+strings.ToLower(c.Action)+".html", args)
}
func (s *server) settingWrite(c *httpserver.Context) httpserver.Result {
	r := info.NewRe()
	if configService == nil {
		return c.RenderJSON(info.Re{Msg: "storage"})
	}
	actor := c.GetPrincipal().UserID
	var result service.ConfigMutationResult
	switch c.Action {
	case "DoBlogTag":
		result = configService.UpdateGlobalConfigs(actor, nil, map[string][]string{
			"recommendTags": splitCSVParams(c.Params.Strings("recommendTags")),
			"newTags":       splitCSVParams(c.Params.Strings("newTags")),
		})
	case "DoDemo":
		re := s.updateDemo(c)
		return c.RenderJSON(re)
	case "DoSiteUrl":
		result = configService.UpdateGlobalStringConfigs(actor, map[string]string{"siteUrl": c.Params.String("siteUrl")})
	case "DoSubDomain":
		result = configService.UpdateGlobalConfigs(actor,
			map[string]string{
				"noteSubDomain": c.Params.String("noteSubDomain"), "blogSubDomain": c.Params.String("blogSubDomain"),
				"leaSubDomain": c.Params.String("leaSubDomain"), "allowCustomDomain": c.Params.String("allowCustomDomain"),
			},
			map[string][]string{
				"blackSubDomains":    splitCSVParams(c.Params.Strings("blackSubDomains")),
				"blackCustomDomains": splitCSVParams(c.Params.Strings("blackCustomDomains")),
			})
	case "ExportPdf":
		path := strings.TrimSpace(c.Params.String("path"))
		descriptor, err := service.BuildExecutableDescriptor(path, []string{path}, "admin-pdf")
		if err != nil {
			return c.RenderJSON(info.Re{Ok: false, Msg: "admin.validation"})
		}
		result = configService.UpdateGlobalConfigs(actor, map[string]string{"exportPdfBinPath": descriptor.Path}, map[string][]string{"pdfExecutableAllowlist": {path}})
	case "OpenRegister":
		result = configService.UpdateGlobalStringConfigs(actor, map[string]string{"openRegister": c.Params.String("openRegister")})
	case "HomePage":
		value := c.Params.String("homePage")
		if value == "0" {
			value = ""
		}
		result = configService.UpdateGlobalStringConfigs(actor, map[string]string{"homePage": value})
	case "Mongodb":
		result = configService.UpdateGlobalConfigs(actor,
			map[string]string{"mongodumpPath": c.Params.String("mongodumpPath"), "mongorestorePath": c.Params.String("mongorestorePath")},
			map[string][]string{"mongoExecutableAllowlist": {c.Params.String("mongodumpPath"), c.Params.String("mongorestorePath")}})
	case "FeedbackRecipients":
		result = configService.UpdateGlobalConfigs(actor, nil, map[string][]string{"feedbackRecipients": splitCSVParams(c.Params.Strings("recipients"))})
	case "MongoExecutableAllowlist", "PdfExecutableAllowlist":
		key := strings.ToLower(c.Action[:1]) + c.Action[1:]
		if c.Action == "MongoExecutableAllowlist" {
			key = "mongoExecutableAllowlist"
		} else {
			key = "pdfExecutableAllowlist"
		}
		result = configService.UpdateGlobalConfigs(actor, nil, map[string][]string{key: splitCSVParams(c.Params.Strings("paths"))})
	case "UploadSize":
		values := map[string]string{}
		for _, key := range []string{"uploadImageSize", "uploadAvatarSize", "uploadBlogLogoSize", "uploadAttachSize"} {
			value := strings.TrimSpace(c.Params.String(key))
			if value != "" {
				if _, err := strconv.ParseFloat(value, 64); err != nil {
					return c.RenderJSON(info.Re{Ok: false, Msg: "admin.validation"})
				}
			}
			values[key] = value
		}
		result = configService.UpdateGlobalStringConfigs(actor, values)
	case "ShareNote":
		ok, msg := s.updateShareNote(c)
		r.Ok, r.Msg = ok, msg
		return c.RenderJSON(r)
	default:
		return c.RenderJSON(info.Re{Ok: false, Msg: "admin.action_not_implemented"})
	}
	r.Ok = result.Err() == nil
	if !r.Ok {
		r.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(r)
}

func (s *server) updateShareNote(c *httpserver.Context) (bool, string) {
	parseInts := func(name string) []int {
		values := c.Params.Strings(name)
		result := make([]int, 0, len(values))
		for _, value := range values {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				continue
			}
			result = append(result, parsed)
		}
		return result
	}
	return configService.UpdateShareNoteConfig(
		c.Params.String("registerSharedUserId"), parseInts("registerSharedNotebookPerms"), parseInts("registerSharedNotePerms"),
		c.Params.Strings("registerSharedNotebookIds"), c.Params.Strings("registerSharedNoteIds"), c.Params.Strings("registerCopyNoteIds"),
	)
}
