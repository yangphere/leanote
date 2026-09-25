package admin

import (
	"github.com/revel/revel"
	//	. "github.com/yangphere/leanote/app/lea"
	"fmt"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
	"strings"
)

// admin 首页

type AdminSetting struct {
	AdminBaseController
}

// email配置
func (c AdminSetting) Email() revel.Result {
	return nil
}

// blog标签设置
func (c AdminSetting) Blog() revel.Result {
	recommendTags := configService.GetGlobalArrayConfig("recommendTags")
	newTags := configService.GetGlobalArrayConfig("newTags")
	c.ViewArgs["recommendTags"] = strings.Join(recommendTags, ",")
	c.ViewArgs["newTags"] = strings.Join(newTags, ",")
	return c.RenderTemplate("admin/setting/blog.html")
}
func (c AdminSetting) DoBlogTag(recommendTags, newTags string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), nil, map[string][]string{"recommendTags": strings.Split(recommendTags, ","), "newTags": strings.Split(newTags, ",")})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

// 共享设置
func (c AdminSetting) ShareNote(registerSharedUserId string,
	registerSharedNotebookPerms, registerSharedNotePerms []int,
	registerSharedNotebookIds, registerSharedNoteIds, registerCopyNoteIds []string) revel.Result {

	re := info.NewRe()
	re.Ok, re.Msg = configService.UpdateShareNoteConfig(registerSharedUserId, registerSharedNotebookPerms, registerSharedNotePerms, registerSharedNotebookIds, registerSharedNoteIds, registerCopyNoteIds)
	return c.RenderJSON(re)
}

// demo
// blog标签设置
func (c AdminSetting) Demo() revel.Result {
	c.ViewArgs["demoUsername"] = configService.GetGlobalStringConfig("demoUsername")
	c.ViewArgs["demoPassword"] = service.RedactedSecretValue
	return c.RenderTemplate("admin/setting/demo.html")
}
func (c AdminSetting) DoDemo(demoUsername, demoPassword string) revel.Result {
	re := info.NewRe()

	userInfo, err := authService.Login(demoUsername, demoPassword)
	if err != nil {
		fmt.Println(err)
		return c.RenderJSON(info.Re{Ok: false})
	}
	if userInfo.UserId.IsZero() {
		re.Msg = "The User is Not Exists"
		return c.RenderJSON(re)
	}

	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"demoUserId": userInfo.UserId.Hex(), "demoUsername": demoUsername, "demoPassword": demoPassword})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

func (c AdminSetting) ExportPdf(path string) revel.Result {
	re := info.NewRe()
	allowlist := []string{path}
	descriptor, err := service.BuildExecutableDescriptor(path, allowlist, "admin-pdf")
	if err != nil {
		re.Msg = "admin.validation"
		return c.RenderJSON(re)
	}
	result := configService.UpdateGlobalConfigs(c.GetUserId(), map[string]string{"exportPdfBinPath": descriptor.Path}, map[string][]string{"pdfExecutableAllowlist": allowlist})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) DoSiteUrl(siteUrl string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"siteUrl": siteUrl})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

// SubDomain
func (c AdminSetting) SubDomain() revel.Result {
	c.ViewArgs["str"] = configService.RedactedStringConfigs()
	c.ViewArgs["arr"] = configService.GlobalArrayConfigs

	c.ViewArgs["noteSubDomain"] = configService.GetGlobalStringConfig("noteSubDomain")
	c.ViewArgs["blogSubDomain"] = configService.GetGlobalStringConfig("blogSubDomain")
	c.ViewArgs["leaSubDomain"] = configService.GetGlobalStringConfig("leaSubDomain")

	return c.RenderTemplate("admin/setting/subDomain.html")
}
func (c AdminSetting) DoSubDomain(noteSubDomain, blogSubDomain, leaSubDomain, blackSubDomains, allowCustomDomain, blackCustomDomains string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), map[string]string{"noteSubDomain": noteSubDomain, "blogSubDomain": blogSubDomain, "leaSubDomain": leaSubDomain, "allowCustomDomain": allowCustomDomain}, map[string][]string{"blackSubDomains": strings.Split(blackSubDomains, ","), "blackCustomDomains": strings.Split(blackCustomDomains, ",")})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

func (c AdminSetting) OpenRegister(openRegister string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"openRegister": openRegister})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) HomePage(homePage string) revel.Result {
	re := info.NewRe()
	if homePage == "0" {
		homePage = ""
	}
	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"homePage": homePage})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) Mongodb(mongodumpPath, mongorestorePath string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), map[string]string{"mongodumpPath": mongodumpPath, "mongorestorePath": mongorestorePath}, map[string][]string{"mongoExecutableAllowlist": []string{mongodumpPath, mongorestorePath}})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

// FeedbackRecipients is the explicit admin write path for the internal
// feedback audience. Validation and normalization live in ConfigService so
// startup, API and this adapter share one policy.
func (c AdminSetting) FeedbackRecipients(recipients []string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), nil, map[string][]string{"feedbackRecipients": recipients})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) MongoExecutableAllowlist(paths []string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), nil, map[string][]string{"mongoExecutableAllowlist": paths})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) PdfExecutableAllowlist(paths []string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), nil, map[string][]string{"pdfExecutableAllowlist": paths})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}

func (c AdminSetting) UploadSize(uploadImageSize, uploadAvatarSize, uploadBlogLogoSize, uploadAttachSize float64) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"uploadImageSize": fmt.Sprintf("%v", uploadImageSize), "uploadAvatarSize": fmt.Sprintf("%v", uploadAvatarSize), "uploadBlogLogoSize": fmt.Sprintf("%v", uploadBlogLogoSize), "uploadAttachSize": fmt.Sprintf("%v", uploadAttachSize)})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}
	return c.RenderJSON(re)
}
