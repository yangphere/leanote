package controllers

import (
	"context"
	"errors"
	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
	//	. "github.com/yangphere/leanote/app/lea"
)

// 首页

type Index struct {
	BaseController
}

func (c Index) Default() revel.Result {
	if configService.HomePageIsAdminsBlog() {
		blog := Blog{c.BaseController}
		return blog.Index(configService.GetAdminUsername())
	}
	return c.Index()
}

// leanote展示页, 没有登录的, 或已登录明确要进该页的
func (c Index) Index() revel.Result {
	c.SetUserInfo()
	c.ViewArgs["title"] = "leanote"
	c.ViewArgs["openRegister"] = configService.GlobalStringConfigs["openRegister"]
	c.SetLocale()

	return c.RenderTemplate("home/index.html")
}

// 建议
func (c Index) Suggestion(addr, suggestion, submissionId string) revel.Result {
	re := info.NewRe()
	result, err := suggestionService.AddSuggestionDurable(context.Background(), info.Suggestion{Addr: addr, UserId: c.GetObjectUserId(), Suggestion: suggestion}, submissionId)
	if err != nil {
		applySuggestionError(&re, result, err)
		return c.RenderJSON(re)
	}
	re.Ok = true
	re.Id = result.ReconciliationID

	return c.RenderJSON(re)
}

func applySuggestionError(re *info.Re, result service.SuggestionSubmission, err error) {
	re.Msg = suggestionErrorMessage(err)
	re.Id = result.ReconciliationID
}

func suggestionErrorMessage(err error) string {
	switch {
	case errors.Is(err, service.ErrSuggestionValidation):
		return "suggestion.validation"
	case errors.Is(err, service.ErrSuggestionConflict):
		return "suggestion.conflict"
	case errors.Is(err, service.ErrSuggestionSideEffect):
		return "suggestion.side_effect"
	default:
		return "suggestion.unknown"
	}
}
