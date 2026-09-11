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
)

type fakeCaptchaSessionService struct {
	sessionID     string
	captcha       string
	setCaptchaErr error
}

func (f *fakeCaptchaSessionService) LoginTimesIsOver(string) (bool, error) {
	return false, nil
}

func (f *fakeCaptchaSessionService) GetCaptcha(string) (string, error) {
	return "", nil
}

func (f *fakeCaptchaSessionService) IncrLoginTimes(string) error {
	return nil
}

func (f *fakeCaptchaSessionService) ClearUserToken(string) (bool, error) {
	return true, nil
}

func (f *fakeCaptchaSessionService) ClearTransientSessionState(string) error {
	return nil
}

func (f *fakeCaptchaSessionService) SetCaptcha(sessionID, captcha string) error {
	f.sessionID = sessionID
	f.captcha = captcha
	return f.setCaptchaErr
}

func TestCaptchaGetCreatesStableAnonymousSessionIDBeforePersistingCaptcha(t *testing.T) {
	saved := sessionService
	fake := &fakeCaptchaSessionService{}
	sessionService = fake
	defer func() { sessionService = saved }()

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/captcha", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	captchaController := Captcha{BaseController: BaseController{Controller: controller}}

	if result := captchaController.Get(); result == nil {
		t.Fatal("Captcha.Get returned nil result")
	}

	id, _ := controller.Session["_ID"].(string)
	if id == "" {
		t.Fatal("Captcha.Get did not persist a stable anonymous _ID")
	}
	if fake.sessionID != id || fake.captcha == "" {
		t.Fatalf("SetCaptcha session=%q captcha=%q, want generated _ID %q and non-empty captcha", fake.sessionID, fake.captcha, id)
	}
}

func TestCaptchaGetDoesNotWriteImageWhenCaptchaPersistenceFails(t *testing.T) {
	saved := sessionService
	savedConfig := revel.Config
	fake := &fakeCaptchaSessionService{setCaptchaErr: errors.New("database unavailable")}
	sessionService = fake
	revel.Config = config.NewContext()
	defer func() {
		sessionService = saved
		revel.Config = savedConfig
	}()

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/captcha", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	captchaController := Captcha{BaseController: BaseController{Controller: controller}}

	result := captchaController.Get()
	if recorder.Body.Len() != 0 {
		t.Fatalf("Captcha.Get wrote %d bytes before persistence failure result was applied", recorder.Body.Len())
	}
	result.Apply(controller.Request, controller.Response)

	var got info.Re
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode failure response: %v body=%q", err, recorder.Body.String())
	}
	if got.Ok || got.Msg != "storage" {
		t.Fatalf("Captcha.Get failure response = %+v, want storage envelope", got)
	}
}
