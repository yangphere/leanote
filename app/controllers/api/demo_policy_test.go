package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/config"
	"github.com/revel/revel"
	"github.com/revel/revel/session"
	"github.com/yangphere/leanote/app/controllers"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

func TestApiUserMutationFailsClosedWhenDemoIdentityConfigurationIsMissing(t *testing.T) {
	saved := configService
	savedConfig := revel.Config
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{
		"demoUsername": "demo@example.test",
	}}
	revel.Config = config.NewContext()
	defer func() {
		configService = saved
		revel.Config = savedConfig
	}()

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/api/user/updateUsername", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["_userId"] = "507f1f77bcf86cd799439012"
	apiUser := ApiUser{ApiBaseContrller: ApiBaseContrller{
		BaseController: controllers.BaseController{Controller: controller},
	}}

	result := apiUser.UpdateUsername("")
	result.Apply(controller.Request, controller.Response)
	var got info.ApiRe
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode API response: %v body=%q", err, recorder.Body.String())
	}
	if got.Ok || got.Msg != "configuration" {
		t.Fatalf("UpdateUsername response = %+v, want configuration failure", got)
	}
}

func TestApiUserAvatarFailsClosedBeforeUploadWhenDemoConfigurationIsMissing(t *testing.T) {
	saved := configService
	savedConfig := revel.Config
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{
		"demoUsername": "demo@example.test",
	}}
	revel.Config = config.NewContext()
	defer func() {
		configService = saved
		revel.Config = savedConfig
	}()

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/api/user/updateLogo", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["_userId"] = "507f1f77bcf86cd799439012"
	apiUser := ApiUser{ApiBaseContrller: ApiBaseContrller{
		BaseController: controllers.BaseController{Controller: controller},
	}}

	result := apiUser.UpdateLogo()
	result.Apply(controller.Request, controller.Response)
	var got info.ApiRe
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode API response: %v body=%q", err, recorder.Body.String())
	}
	if got.Ok || got.Msg != "configuration" {
		t.Fatalf("UpdateLogo response = %+v, want pre-upload configuration failure", got)
	}
}
