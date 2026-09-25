package admin

import (
	"strings"

	"github.com/revel/revel"
)

// admin 首页

type Admin struct {
	AdminBaseController
}

// admin 主页
func (c Admin) Index() revel.Result {
	c.SetUserInfo()

	c.ViewArgs["title"] = "leanote"
	c.SetLocale()

	c.ViewArgs["countUser"] = userService.CountUser()
	c.ViewArgs["countNote"] = noteService.CountNote("")
	c.ViewArgs["countBlog"] = noteService.CountBlog("")

	return c.RenderTemplate("admin/index.html")
}

// 模板
func (c Admin) T(t string) revel.Result {
	if !allowedAdminTemplate(t) {
		return c.NotFound("admin template not found")
	}
	c.ViewArgs["str"] = configService.RedactedStringConfigs()
	c.ViewArgs["arr"] = configService.GlobalArrayConfigs
	c.ViewArgs["map"] = configService.GlobalMapConfigs
	c.ViewArgs["arrMap"] = configService.GlobalArrMapConfigs
	c.ViewArgs["version"] = configService.GetVersion()
	return c.RenderTemplate("admin/" + t + ".html")
}

func (c Admin) GetView(view string) revel.Result {
	if !allowedAdminTemplate(view) {
		return c.NotFound("admin view not found")
	}
	return c.RenderTemplate("admin/" + view + ".html")
}

func allowedAdminTemplate(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "\\\x00\r\n") || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return false
	}
	allowed := map[string]struct{}{
		"email/set": {}, "email/template": {}, "email/sendToUsers": {}, "email/send": {},
		"setting/site_url": {}, "setting/home_page": {}, "setting/demo": {}, "setting/open_register": {},
		"setting/share_note": {}, "setting/upload": {}, "setting/export_pdf": {}, "data/configuration": {},
		"upgrade/beta2": {}, "upgrade/beta3": {},
	}
	if _, ok := allowed[name]; ok {
		return true
	}
	return false
}
