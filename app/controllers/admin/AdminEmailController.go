package admin

import (
	"context"
	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"strconv"
	"strings"
)

// admin 首页

type AdminEmail struct {
	AdminBaseController
}

func (c AdminEmail) enqueueBroadcast(recipients []string, subject, body string) info.Re {
	re := info.NewRe()
	batchID := ""
	if c.Params != nil {
		batchID = c.Params.Values.Get("batchId")
	}
	result, err := emailService.EnqueueBroadcast(context.Background(), c.GetObjectUserId(), batchID, recipients, subject, body)
	if err != nil {
		re.Msg = "admin.mail_enqueue_failed"
		return re
	}
	re.Ok = true
	re.Id = result.ReconciliationID
	return re
}

// email配置
func (c AdminEmail) Email() revel.Result {
	return nil
}

// blog标签设置
func (c AdminEmail) Blog() revel.Result {
	recommendTags := configService.GetGlobalArrayConfig("recommendTags")
	newTags := configService.GetGlobalArrayConfig("newTags")
	c.ViewArgs["recommendTags"] = strings.Join(recommendTags, ",")
	c.ViewArgs["newTags"] = strings.Join(newTags, ",")
	return c.RenderTemplate("admin/setting/blog.html")
}
func (c AdminEmail) DoBlogTag(recommendTags, newTags string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalConfigs(c.GetUserId(), nil, map[string][]string{
		"recommendTags": strings.Split(recommendTags, ","),
		"newTags":       strings.Split(newTags, ","),
	})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

// demo
// blog标签设置
func (c AdminEmail) Demo() revel.Result {
	c.ViewArgs["demoUsername"] = configService.GetGlobalStringConfig("demoUsername")
	c.ViewArgs["demoPassword"] = service.RedactedSecretValue
	return c.RenderTemplate("admin/setting/demo.html")
}
func (c AdminEmail) DoDemo(demoUsername, demoPassword string) revel.Result {
	re := info.NewRe()

	userInfo, err := authService.Login(demoUsername, demoPassword)
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false})
	}
	if userInfo.UserId.IsZero() {
		re.Msg = "The User is Not Exists"
		return c.RenderJSON(re)
	}

	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{
		"demoUserId":   userInfo.UserId.Hex(),
		"demoUsername": demoUsername,
		"demoPassword": demoPassword,
	})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}

// ToImage
// 长微博的bin路径phantomJs
func (c AdminEmail) ToImage() revel.Result {
	c.ViewArgs["toImageBinPath"] = configService.GetGlobalStringConfig("toImageBinPath")
	return c.RenderTemplate("admin/setting/toImage.html")
}
func (c AdminEmail) DoToImage(toImageBinPath string) revel.Result {
	re := info.NewRe()
	re.Ok = configService.UpdateGlobalStringConfig(c.GetUserId(), "toImageBinPath", toImageBinPath)
	return c.RenderJSON(re)
}

func (c AdminEmail) Set(emailHost, emailPort, emailUsername, emailPassword, emailSSL string) revel.Result {
	re := info.NewRe()
	result := configService.UpdateGlobalStringConfigs(c.GetUserId(), map[string]string{"emailHost": emailHost, "emailPort": emailPort, "emailUsername": emailUsername, "emailPassword": emailPassword, "emailSSL": emailSSL})
	re.Ok = result.Err() == nil
	if !re.Ok {
		re.Msg = "admin.config_update_failed"
	}

	return c.RenderJSON(re)
}
func (c AdminEmail) Template() revel.Result {
	re := info.NewRe()

	keys := []string{"emailTemplateHeader", "emailTemplateFooter",
		"emailTemplateRegisterSubject",
		"emailTemplateRegister",
		"emailTemplateFindPasswordSubject",
		"emailTemplateFindPassword",
		"emailTemplateUpdateEmailSubject",
		"emailTemplateUpdateEmail",
		"emailTemplateInviteSubject",
		"emailTemplateInvite",
		"emailTemplateCommentSubject",
		"emailTemplateComment",
	}

	userId := c.GetUserId()
	for _, key := range keys {
		v := c.Params.Values.Get(key)
		if v != "" {
			ok, msg := emailService.ValidTpl(v)
			if !ok {
				re.Ok = false
				re.Msg = "Error key: " + key + "<br />" + msg
				return c.RenderJSON(re)
			} else {
				result := configService.UpdateGlobalStringConfigs(userId, map[string]string{key: v})
				if err := result.Err(); err != nil {
					re.Ok = false
					re.Msg = "admin.template_update_failed"
					return c.RenderJSON(re)
				}
			}
		}
	}

	re.Ok = true
	return c.RenderJSON(re)
}

// 发送Email
func (c AdminEmail) SendEmailToEmails(sendEmails, latestEmailSubject, latestEmailBody string, verified, saveAsOldEmail bool) revel.Result {
	re := info.NewRe()

	if err := c.updateConfig([]string{"sendEmails", "latestEmailSubject", "latestEmailBody"}).Err(); err != nil {
		re.Msg = "admin.config_update_failed"
		return c.RenderJSON(re)
	}

	if latestEmailSubject == "" || latestEmailBody == "" {
		re.Msg = "subject or body is blank"
		return c.RenderJSON(re)
	}

	if saveAsOldEmail {
		oldEmails := configService.GetGlobalMapConfig("oldEmails")
		oldEmails[latestEmailSubject] = latestEmailBody
		if !configService.UpdateGlobalMapConfig(c.GetUserId(), "oldEmails", oldEmails) {
			re.Msg = "old email template persistence failed"
			return c.RenderJSON(re)
		}
	}

	sendEmails = strings.Replace(sendEmails, "\r", "", -1)
	emails := strings.Split(sendEmails, "\n")

	re = c.enqueueBroadcast(emails, latestEmailSubject, latestEmailBody)
	return c.RenderJSON(re)
}

// 发送Email
func (c AdminEmail) SendToUsers2(emails, latestEmailSubject, latestEmailBody string, verified, saveAsOldEmail bool) revel.Result {
	re := info.NewRe()

	if err := c.updateConfig([]string{"sendEmails", "latestEmailSubject", "latestEmailBody"}).Err(); err != nil {
		re.Msg = "admin.config_update_failed"
		return c.RenderJSON(re)
	}

	if latestEmailSubject == "" || latestEmailBody == "" {
		re.Msg = "subject or body is blank"
		return c.RenderJSON(re)
	}

	if saveAsOldEmail {
		oldEmails := configService.GetGlobalMapConfig("oldEmails")
		oldEmails[latestEmailSubject] = latestEmailBody
		if !configService.UpdateGlobalMapConfig(c.GetUserId(), "oldEmails", oldEmails) {
			re.Msg = "old email template persistence failed"
			return c.RenderJSON(re)
		}
	}

	emails = strings.Replace(emails, "\r", "", -1)
	emailsArr := strings.Split(emails, "\n")

	users := userService.ListUserInfosByEmails(emailsArr)
	LogJ(emailsArr)

	emailsArr = emailsArr[:0]
	for _, user := range users {
		emailsArr = append(emailsArr, user.Email)
	}
	re = c.enqueueBroadcast(emailsArr, latestEmailSubject, latestEmailBody)

	return c.RenderJSON(re)
}

// send Email dialog
func (c AdminEmail) SendEmailDialog(emails string) revel.Result {
	emailsArr := strings.Split(emails, ",")
	emailsNl := strings.Join(emailsArr, "\n")

	c.ViewArgs["emailsNl"] = emailsNl
	c.ViewArgs["str"] = configService.RedactedStringConfigs()
	c.ViewArgs["map"] = configService.GlobalMapConfigs

	return c.RenderTemplate("admin/email/emailDialog.html")
}

func (c AdminEmail) SendToUsers(userFilterEmail, userFilterWhiteList, userFilterBlackList, latestEmailSubject, latestEmailBody string, verified, saveAsOldEmail bool) revel.Result {
	re := info.NewRe()

	if err := c.updateConfig([]string{"userFilterEmail", "userFilterWhiteList", "userFilterBlackList", "latestEmailSubject", "latestEmailBody"}).Err(); err != nil {
		re.Msg = "admin.config_update_failed"
		return c.RenderJSON(re)
	}

	if latestEmailSubject == "" || latestEmailBody == "" {
		re.Msg = "subject or body is blank"
		return c.RenderJSON(re)
	}

	if saveAsOldEmail {
		oldEmails := configService.GetGlobalMapConfig("oldEmails")
		oldEmails[latestEmailSubject] = latestEmailBody
		if !configService.UpdateGlobalMapConfig(c.GetUserId(), "oldEmails", oldEmails) {
			re.Msg = "old email template persistence failed"
			return c.RenderJSON(re)
		}
	}

	users := userService.GetAllUserByFilter(userFilterEmail, userFilterWhiteList, userFilterBlackList, verified)

	if users == nil || len(users) == 0 {
		re.Ok = false
		re.Msg = "no users"
		return c.RenderJSON(re)
	}

	re = c.enqueueBroadcast(func() []string {
		emails := make([]string, 0, len(users))
		for _, user := range users {
			emails = append(emails, user.Email)
		}
		return emails
	}(), latestEmailSubject, latestEmailBody)
	if !re.Ok {
		return c.RenderJSON(re)
	}

	re.Msg = "users:" + strconv.Itoa(len(users))

	return c.RenderJSON(re)
}

// 删除emails
func (c AdminEmail) DeleteEmails(ids string) revel.Result {
	re := info.NewRe()
	re.Ok = emailService.DeleteEmails(strings.Split(ids, ","))
	return c.RenderJSON(re)
}

func (c AdminEmail) List(sorter, keywords string) revel.Result {
	pageNumber := c.GetPage()
	sorterField, isAsc := c.getSorter("CreatedTime", false, []string{"email", "ok", "subject", "createdTime"})
	pageInfo, emails := emailService.ListEmailLogs(pageNumber, userPageSize, sorterField, isAsc, keywords)
	c.ViewArgs["pageInfo"] = pageInfo
	c.ViewArgs["emails"] = emails
	c.ViewArgs["keywords"] = keywords
	return c.RenderTemplate("admin/email/list.html")
}
