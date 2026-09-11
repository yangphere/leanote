package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/config"
	"github.com/revel/revel"
	"github.com/revel/revel/session"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

type fakePwdService struct {
	ok    bool
	msg   string
	email string
}

func (f *fakePwdService) FindPwd(email string) (bool, string) {
	f.email = email
	return f.ok, f.msg
}

func (f *fakePwdService) UpdatePwd(string, string) (bool, string) {
	return false, "unused"
}

type fakeAuthService struct {
	loginErr error
	user     info.User
}

func (f *fakeAuthService) Login(string, string) (info.User, error) {
	return f.user, f.loginErr
}

func (f *fakeAuthService) Register(string, string, string) (bool, string) {
	return false, "unused"
}

type fakeLoginSessionService struct {
	increments int
}

func (f *fakeLoginSessionService) LoginTimesIsOver(string) (bool, error) {
	return false, nil
}

func (f *fakeLoginSessionService) GetCaptcha(string) (string, error) {
	return "", nil
}

func (f *fakeLoginSessionService) IncrLoginTimes(string) error {
	f.increments++
	return nil
}

func (f *fakeLoginSessionService) ClearUserToken(string) (bool, error) {
	return true, nil
}

func (f *fakeLoginSessionService) ClearTransientSessionState(string) error {
	return nil
}

func (f *fakeLoginSessionService) SetCaptcha(string, string) error {
	return nil
}

func TestAuthDoFindPasswordUsesServiceResult(t *testing.T) {
	saved := pwdService
	savedConfig := revel.Config
	fake := &fakePwdService{ok: false, msg: "side_effect"}
	pwdService = fake
	revel.Config = config.NewContext()
	defer func() {
		pwdService = saved
		revel.Config = savedConfig
	}()

	result, response := renderFindPasswordResult(t, "User@Example.Test")
	result.Apply(response.Request, response.Response)

	var got info.Re
	if err := json.Unmarshal(response.Body.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v body=%q", err, response.Body.Body.String())
	}
	if fake.email != "User@Example.Test" {
		t.Fatalf("service email=%q, want controller-bound email", fake.email)
	}
	if got.Ok || got.Msg != "side_effect" {
		t.Fatalf("DoFindPassword response = %+v, want service failure envelope", got)
	}
}

func TestAuthDoLoginMapsStorageErrorWithoutCountingCredentialFailure(t *testing.T) {
	savedAuth := authService
	savedSession := sessionService
	savedConfig := revel.Config
	session := &fakeLoginSessionService{}
	authService = &fakeAuthService{loginErr: errors.New("database unavailable")}
	sessionService = session
	revel.Config = config.NewContext()
	defer func() {
		authService = savedAuth
		sessionService = savedSession
		revel.Config = savedConfig
	}()

	result, response := renderDoLoginResult(t, "user@example.test", "password")
	result.Apply(response.Request, response.Response)

	var got info.Re
	if err := json.Unmarshal(response.Body.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v body=%q", err, response.Body.Body.String())
	}
	if got.Ok || got.Msg != "storage" {
		t.Fatalf("DoLogin storage response = %+v, want storage envelope", got)
	}
	if session.increments != 0 {
		t.Fatalf("storage lookup incremented login failures %d times", session.increments)
	}
}

func TestAuthDemoRejectsMissingDemoIdentityConfiguration(t *testing.T) {
	savedAuth := authService
	savedConfigService := configService
	savedConfig := revel.Config
	authService = &fakeAuthService{loginErr: service.ErrInvalidCredentials}
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{
		"demoUsername": "demo@example.test",
		"demoPassword": "secret",
	}}
	revel.Config = config.NewContext()
	defer func() {
		authService = savedAuth
		configService = savedConfigService
		revel.Config = savedConfig
	}()

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/demo", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	result := (Auth{BaseController: BaseController{Controller: controller}}).Demo()
	result.Apply(controller.Request, controller.Response)

	var got info.Re
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode demo response: %v body=%q", err, recorder.Body.String())
	}
	if got.Ok || got.Msg != "configuration" {
		t.Fatalf("Demo response = %+v, want configuration failure", got)
	}
}

type renderedControllerResponse struct {
	Request  *revel.Request
	Response *revel.Response
	Body     *httptest.ResponseRecorder
}

func renderFindPasswordResult(t *testing.T, email string) (revel.Result, renderedControllerResponse) {
	t.Helper()
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/findPassword", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	auth := Auth{BaseController: BaseController{Controller: controller}}
	return auth.DoFindPassword(email), renderedControllerResponse{
		Request:  controller.Request,
		Response: controller.Response,
		Body:     recorder,
	}
}

func renderDoLoginResult(t *testing.T, email, pwd string) (revel.Result, renderedControllerResponse) {
	t.Helper()
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/doLogin", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	auth := Auth{BaseController: BaseController{Controller: controller}}
	return auth.DoLogin(email, pwd, ""), renderedControllerResponse{
		Request:  controller.Request,
		Response: controller.Response,
		Body:     recorder,
	}
}
