package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
)

type fakeLoginSessionService struct{ increments int }

func (f *fakeLoginSessionService) LoginTimesIsOver(string) (bool, error)   { return false, nil }
func (f *fakeLoginSessionService) GetCaptcha(string) (string, error)       { return "", nil }
func (f *fakeLoginSessionService) IncrLoginTimes(string) error             { f.increments++; return nil }
func (f *fakeLoginSessionService) ClearUserToken(string) (bool, error)     { return true, nil }
func (f *fakeLoginSessionService) ClearTransientSessionState(string) error { return nil }
func (f *fakeLoginSessionService) SetCaptcha(string, string) error         { return nil }

type fakeAuthService struct{ loginErr error }

func (f *fakeAuthService) Login(string, string) (info.User, error)        { return info.User{}, f.loginErr }
func (f *fakeAuthService) Register(string, string, string) (bool, string) { return false, "" }

type fakeCaptchaSessionService struct{ sessionID, captcha string }

func (f *fakeCaptchaSessionService) LoginTimesIsOver(string) (bool, error)   { return false, nil }
func (f *fakeCaptchaSessionService) GetCaptcha(string) (string, error)       { return f.captcha, nil }
func (f *fakeCaptchaSessionService) IncrLoginTimes(string) error             { return nil }
func (f *fakeCaptchaSessionService) ClearUserToken(string) (bool, error)     { return true, nil }
func (f *fakeCaptchaSessionService) ClearTransientSessionState(string) error { return nil }
func (f *fakeCaptchaSessionService) SetCaptcha(id, value string) error {
	f.sessionID, f.captcha = id, value
	return nil
}

func TestRegisterMainHTTPExposesB2WebActions(t *testing.T) {
	registry := httpserver.NewRegistry()
	RegisterMainHTTP(registry)

	for _, name := range []string{
		"Auth.Login", "Auth.DoLogin", "Auth.Logout", "Auth.Demo", "Auth.Register",
		"Auth.DoRegister", "Auth.FindPassword", "Auth.DoFindPassword", "Auth.FindPassword2",
		"Auth.FindPasswordUpdate", "Captcha.Get", "Index.Default", "Index.Index",
		"Index.Suggestion", "User.Account", "User.UpdateUsername", "User.UpdatePwd",
		"User.UpdateTheme", "User.SendRegisterEmail", "User.ReSendActiveEmail",
		"User.UpdateEmail", "User.ActiveEmail", "User.UpdateColumnWidth", "User.UpdateLeftIsMin",
	} {
		if _, ok := registry.Lookup(strings.SplitN(name, ".", 2)[0], strings.SplitN(name, ".", 2)[1]); !ok {
			t.Errorf("registry missing %s", name)
		}
	}
	login, ok := registry.Lookup("Auth", "Login")
	if !ok || len(login.AllowedMethods) != 1 || login.AllowedMethods[0] != http.MethodGet {
		t.Fatalf("Auth.Login methods = %#v, want GET", login.AllowedMethods)
	}
	account, ok := registry.Lookup("User", "Account")
	if !ok || len(account.AllowedMethods) != 0 {
		t.Fatalf("User.Account methods = %#v, want catch-all semantics", account.AllowedMethods)
	}
}

func TestNativeAuthDoLoginMapsStorageErrorBeforeCounting(t *testing.T) {
	savedAuth, savedSession, savedRotate := authService, sessionService, rotateWebSession
	loginSession := &fakeLoginSessionService{}
	authService = &fakeAuthService{loginErr: errors.New("database unavailable")}
	sessionService = loginSession
	rotateWebSession = func(*httpserver.Context, string) error { return nil }
	defer func() {
		authService, sessionService, rotateWebSession = savedAuth, savedSession, savedRotate
	}()

	registry := httpserver.NewRegistry()
	registry.RegisterMethods("Auth", "DoLogin", []string{http.MethodPost}, []httpserver.BeforeFunc{webSessionBefore}, (&AuthHTTPServer{}).DoLogin)
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "POST /doLogin Auth.DoLogin")),
		Registry: registry,
	}
	req := httptest.NewRequest(http.MethodPost, "/doLogin", strings.NewReader(url.Values{
		"email": {"user@example.test"}, "pwd": {"password"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Msg":"storage"`) {
		t.Fatalf("native DoLogin response = %d %q, want storage envelope", rec.Code, rec.Body.String())
	}
	if loginSession.increments != 0 {
		t.Fatalf("storage lookup incremented login failures %d times", loginSession.increments)
	}
}

func TestNativeCaptchaPersistsBeforeWritingImage(t *testing.T) {
	saved := sessionService
	fake := &fakeCaptchaSessionService{}
	sessionService = fake
	defer func() { sessionService = saved }()

	registry := httpserver.NewRegistry()
	registry.Register("Captcha", "Get", []httpserver.BeforeFunc{webSessionBefore}, (&CaptchaHTTPServer{}).Get)
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "* /captcha/get Captcha.Get")),
		Registry: registry,
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/captcha/get", nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("native captcha response = %d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if len(rec.Body.Bytes()) == 0 || fake.sessionID == "" || fake.captcha == "" {
		t.Fatalf("captcha response/session = body=%d session=%q captcha=%q", rec.Body.Len(), fake.sessionID, fake.captcha)
	}
}

func TestNativeUserActionUsesWebAuthBoundary(t *testing.T) {
	registry := httpserver.NewRegistry()
	registry.Register("User", "Account", []httpserver.BeforeFunc{webSessionBefore, requireWebAuthentication}, (&UserHTTPServer{}).Account)
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "* /user/account User.Account")),
		Registry: registry,
	}

	req := httptest.NewRequest(http.MethodGet, "/user/account", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated user page = %d location=%q", rec.Code, rec.Header().Get("Location"))
	}

	ajax := httptest.NewRequest(http.MethodGet, "/user/account", nil)
	ajax.Header.Set("X-Requested-With", "XMLHttpRequest")
	ajaxRec := httptest.NewRecorder()
	app.ServeHTTP(ajaxRec, ajax)
	if ajaxRec.Code != http.StatusOK || !strings.Contains(ajaxRec.Body.String(), `"Msg":"NOTLOGIN"`) {
		t.Fatalf("unauthenticated ajax user page = %d %q", ajaxRec.Code, ajaxRec.Body.String())
	}
}

// Keep info referenced in this file when the compiler evaluates the package
// with build tags that omit the legacy controller tests containing the fake.
var _ = info.Re{}
