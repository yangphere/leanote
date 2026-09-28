package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/lea/captcha"
	"github.com/yangphere/leanote/app/lea/i18n"
	"github.com/yangphere/leanote/app/service"
)

// RegisterMainHTTP wires the public web actions that do not belong to a
// migrated feature batch yet. The route table remains the source of method
// semantics: explicit routes get their observed GET/POST method, while
// catch-all User/Captcha/Index actions intentionally accept any method.
func RegisterMainHTTP(rs *httpserver.Registry) {
	public := []httpserver.BeforeFunc{webSessionBefore}
	authenticated := []httpserver.BeforeFunc{webSessionBefore, requireWebAuthentication}

	index := &IndexHTTPServer{}
	rs.RegisterMethods("Index", "Default", []string{http.MethodGet}, public, index.Default)
	rs.RegisterMethods("Index", "Index", []string{http.MethodGet}, public, index.Index)
	rs.Register("Index", "Suggestion", public, index.Suggestion)

	auth := &AuthHTTPServer{}
	rs.RegisterMethods("Auth", "Login", []string{http.MethodGet}, public, auth.Login)
	rs.RegisterMethods("Auth", "DoLogin", []string{http.MethodPost}, public, auth.DoLogin)
	rs.RegisterMethods("Auth", "Logout", []string{http.MethodGet}, public, auth.Logout)
	rs.RegisterMethods("Auth", "Demo", []string{http.MethodGet}, public, auth.Demo)
	rs.RegisterMethods("Auth", "Register", []string{http.MethodGet}, public, auth.Register)
	rs.RegisterMethods("Auth", "DoRegister", []string{http.MethodPost}, public, auth.DoRegister)
	rs.RegisterMethods("Auth", "FindPassword2", []string{http.MethodGet}, public, auth.FindPassword2)
	rs.RegisterMethods("Auth", "FindPassword", []string{http.MethodGet}, public, auth.FindPassword)
	rs.RegisterMethods("Auth", "DoFindPassword", []string{http.MethodPost}, public, auth.DoFindPassword)
	rs.RegisterMethods("Auth", "FindPasswordUpdate", []string{http.MethodPost}, public, auth.FindPasswordUpdate)

	rs.Register("Captcha", "Get", public, (&CaptchaHTTPServer{}).Get)

	user := &UserHTTPServer{}
	rs.Register("User", "Account", authenticated, user.Account)
	rs.Register("User", "UpdateUsername", authenticated, user.UpdateUsername)
	rs.Register("User", "UpdatePwd", authenticated, user.UpdatePwd)
	rs.Register("User", "UpdateTheme", authenticated, user.UpdateTheme)
	rs.Register("User", "SendRegisterEmail", authenticated, user.SendRegisterEmail)
	rs.Register("User", "ReSendActiveEmail", authenticated, user.ReSendActiveEmail)
	rs.Register("User", "UpdateEmail", public, user.UpdateEmail)
	rs.Register("User", "ActiveEmail", public, user.ActiveEmail)
	rs.Register("User", "UpdateColumnWidth", authenticated, user.UpdateColumnWidth)
	rs.Register("User", "UpdateLeftIsMin", authenticated, user.UpdateLeftIsMin)
}

// webSessionBefore promotes the authenticated user stored in the web
// session into the request principal. Invalid or missing UserId values are
// anonymous; storage and policy errors fail closed instead of guessing a
// user from request parameters.
func webSessionBefore(c *httpserver.Context) httpserver.Result {
	userID, ok, err := c.Get("UserId")
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	if !ok || strings.TrimSpace(userID) == "" || !db.IsValidObjectIDHex(strings.TrimSpace(userID)) {
		c.SetPrincipal(httpserver.AnonymousPrincipal())
		return nil
	}
	if err := c.SetAuthenticatedPrincipal(strings.TrimSpace(userID), httpserver.PrincipalSourceWebSession, httpserver.TokenStateAbsent); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: principalErrorMessage(err)})
	}
	return nil
}

func requireWebAuthentication(c *httpserver.Context) httpserver.Result {
	if c.GetPrincipal().UserID != "" {
		return nil
	}
	if c.Request != nil && strings.EqualFold(c.Request.Header.Get("X-Requested-With"), "XMLHttpRequest") {
		return c.RenderJSON(info.Re{Ok: false, Msg: "NOTLOGIN"})
	}
	return c.Redirect("/login")
}

func principalErrorMessage(err error) string {
	if errors.Is(err, service.ErrGlobalConfigNotLoaded) || errors.Is(err, service.ErrDemoConfiguration) {
		return "configuration"
	}
	return "storage"
}

func anonymousSessionID(c *httpserver.Context) (string, error) {
	if value, ok, err := c.Get("_ID"); err != nil {
		return "", err
	} else if ok && strings.TrimSpace(value) != "" {
		return value, nil
	}
	id, err := db.NewAnonymousSessionID()
	if err != nil {
		return "", err
	}
	if err := c.Set("_ID", id); err != nil {
		return "", err
	}
	return id, nil
}

func setWebUserSession(c *httpserver.Context, user info.User) error {
	if user.UserId.IsZero() {
		return errors.New("authenticated user has no id")
	}
	values := map[string]string{
		"UserId":        user.UserId.Hex(),
		"Email":         user.Email,
		"Username":      user.Username,
		"UsernameRaw":   user.UsernameRaw,
		"Theme":         user.Theme,
		"Logo":          user.Logo,
		"NotebookWidth": strconvItoa(user.NotebookWidth),
		"NoteListWidth": strconvItoa(user.NoteListWidth),
		"Verified":      boolSessionValue(user.Verified),
		"LeftIsMin":     boolSessionValue(user.LeftIsMin),
	}
	for key, value := range values {
		if err := c.SetSession(key, value); err != nil {
			return err
		}
	}
	return nil
}

func clearWebUserSession(c *httpserver.Context) error {
	for _, key := range []string{"UserId", "Email", "Username", "UsernameRaw", "Logo", "Verified", "Theme", "theme", "NotebookWidth", "NoteListWidth", "MdEditorWidth", "LeftIsMin", "_token", "_userId"} {
		if err := c.DeleteSession(key); err != nil {
			return err
		}
	}
	return nil
}

func updateWebSession(c *httpserver.Context, key, value string) error {
	return c.SetSession(key, value)
}

func boolSessionValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}

// rotateWebSession is a seam for contract tests. The production path keeps a
// stable anonymous _ID for captcha/API fallback, rotates it on login, and
// retires the old database mapping before publishing the new cookie.
var rotateWebSession = rotateWebSessionDefault

func rotateWebSessionDefault(c *httpserver.Context, userID string) error {
	if sessionService == nil {
		return errors.New("session service is not initialized")
	}
	oldID, err := anonymousSessionID(c)
	if err != nil {
		return fmt.Errorf("read anonymous session: %w", err)
	}
	newID, err := db.NewAnonymousSessionID()
	if err != nil {
		return fmt.Errorf("rotate anonymous session: %w", err)
	}
	if err := db.CreateSessionForToken(requestContext(c), newID, userID, time.Now()); err != nil {
		return fmt.Errorf("create authenticated session mapping: %w", err)
	}
	rollback := func() {
		_, _ = sessionService.ClearUserToken(newID)
	}
	if err := sessionService.ClearTransientSessionState(oldID); err != nil {
		rollback()
		return fmt.Errorf("clear anonymous session state: %w", err)
	}
	if _, err := sessionService.ClearUserToken(oldID); err != nil {
		rollback()
		return fmt.Errorf("retire anonymous session mapping: %w", err)
	}
	if err := c.Set("_ID", newID); err != nil {
		rollback()
		return fmt.Errorf("publish authenticated session id: %w", err)
	}
	if err := c.Delete("_token"); err != nil {
		rollback()
		return err
	}
	if err := c.Delete("_userId"); err != nil {
		rollback()
		return err
	}
	return nil
}

func requestContext(c *httpserver.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}

func prepareView(c *httpserver.Context) map[string]interface{} {
	args := c.ViewArgs
	if args == nil {
		args = map[string]interface{}{}
	}
	locale := strings.ToLower(strings.TrimSpace(c.Locale))
	if !i18n.HasLang(locale) {
		locale = i18n.GetDefaultLang()
	}
	args["currentLocale"] = locale
	args["locale"] = locale
	if configService != nil {
		args["siteUrl"] = configService.GetSiteUrl()
		args["blogUrl"] = configService.GetBlogUrl()
		args["leaUrl"] = configService.GetLeaUrl()
		args["noteUrl"] = configService.GetNoteUrl()
	}
	c.ViewArgs = args
	return args
}

func renderLocalizedRe(c *httpserver.Context, re info.Re) httpserver.Result {
	oldMsg := re.Msg
	if re.Msg != "" {
		parts := strings.Split(re.Msg, "-")
		args := make([]interface{}, 0, len(parts)-1)
		for _, value := range parts[1:] {
			args = append(args, value)
		}
		if len(args) > 0 {
			re.Msg = c.Message(parts[0], args...)
		} else {
			re.Msg = c.Message(parts[0])
		}
	}
	if strings.HasPrefix(re.Msg, "???") {
		re.Msg = oldMsg
	}
	return c.RenderJSON(re)
}

type AuthHTTPServer struct{}

func authLoginStorageError(err error) bool {
	return err != nil && !errors.Is(err, service.ErrInvalidCredentials)
}

func demoPolicyErrorMessage(err error) string {
	if errors.Is(err, service.ErrDemoConfiguration) {
		return "configuration"
	}
	return "storage"
}

func (s *AuthHTTPServer) Login(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	args["title"], args["subTitle"] = c.Message("login"), c.Message("login")
	args["email"] = c.Params.String("email")
	args["from"] = c.Params.String("from")
	if configService != nil {
		args["openRegister"] = configService.IsOpenRegister()
	}
	if sessionID, err := anonymousSessionID(c); err == nil && sessionService != nil {
		if over, readErr := sessionService.LoginTimesIsOver(sessionID); readErr == nil && over {
			args["needCaptcha"] = true
		}
	}
	if c.Params.Has("demo") {
		args["demo"] = true
		args["email"] = "demo@leanote.com"
	}
	return c.RenderTemplate("home/login.html", args)
}

func (s *AuthHTTPServer) DoLogin(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if authService == nil || sessionService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	sessionID, err := anonymousSessionID(c)
	if err != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	over, err := sessionService.LoginTimesIsOver(sessionID)
	if err != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	msg := ""
	if over {
		stored, readErr := sessionService.GetCaptcha(sessionID)
		if readErr != nil {
			re.Msg = "storage"
			return c.RenderJSON(re)
		}
		if stored != c.Params.String("captcha") {
			msg = "captchaError"
		}
	}
	if msg == "" {
		user, loginErr := authService.Login(c.Params.String("email"), c.Params.String("pwd"))
		if loginErr != nil {
			if authLoginStorageError(loginErr) {
				re.Msg = "storage"
				return c.RenderJSON(re)
			}
			msg = "wrongUsernameOrPassword"
			if err := sessionService.IncrLoginTimes(sessionID); err != nil {
				re.Msg = "storage"
				return c.RenderJSON(re)
			}
		} else {
			if err := rotateWebSession(c, user.UserId.Hex()); err != nil || setWebUserSession(c, user) != nil {
				re.Msg = "storage"
				return c.RenderJSON(re)
			}
			re.Ok = true
			return c.RenderJSON(re)
		}
	}
	over, err = sessionService.LoginTimesIsOver(sessionID)
	if err != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Item = over
	re.Msg = c.Message(msg)
	return c.RenderJSON(re)
}

func (s *AuthHTTPServer) Logout(c *httpserver.Context) httpserver.Result {
	if sessionService == nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "logout_cleanup_failed"})
	}
	sessionID, err := anonymousSessionID(c)
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "logout_cleanup_failed"})
	}
	if _, err := sessionService.ClearUserToken(sessionID); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "logout_cleanup_failed"})
	}
	if err := clearWebUserSession(c); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "logout_cleanup_failed"})
	}
	return c.Redirect("/login")
}

func (s *AuthHTTPServer) Demo(c *httpserver.Context) httpserver.Result {
	if configService == nil || !configService.GlobalSnapshotLoaded() {
		return c.RenderJSON(info.Re{Ok: false, Msg: "configuration"})
	}
	if authService == nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	account, err := configService.DemoAccount()
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: demoPolicyErrorMessage(err)})
	}
	user, err := authService.Login(account.Login, configService.GetGlobalStringConfig("demoPassword"))
	if err != nil {
		if authLoginStorageError(err) {
			return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
		}
		return c.RenderJSON(info.Re{Ok: false})
	}
	if user.UserId != account.UserID {
		return c.RenderJSON(info.Re{Ok: false, Msg: "configuration"})
	}
	if err := rotateWebSession(c, user.UserId.Hex()); err != nil || setWebUserSession(c, user) != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	return c.Redirect("/note")
}

func (s *AuthHTTPServer) Register(c *httpserver.Context) httpserver.Result {
	if configService == nil || !configService.IsOpenRegister() {
		return c.Redirect("/index")
	}
	args := prepareView(c)
	args["from"], args["iu"] = c.Params.String("from"), c.Params.String("iu")
	args["title"], args["subTitle"] = c.Message("register"), c.Message("register")
	return c.RenderTemplate("home/register.html", args)
}

func (s *AuthHTTPServer) DoRegister(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if configService == nil || !configService.IsOpenRegister() {
		return c.Redirect("/index")
	}
	email, pwd := c.Params.String("email"), c.Params.String("pwd")
	if re.Ok, re.Msg = lea.Vd("email", email); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	if re.Ok, re.Msg = lea.Vd("password", pwd); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	email = strings.ToLower(email)
	if authService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = authService.Register(email, pwd, c.Params.String("iu"))
	if re.Ok {
		user, loginErr := authService.Login(email, pwd)
		if loginErr != nil || rotateWebSession(c, user.UserId.Hex()) != nil || setWebUserSession(c, user) != nil {
			re.Ok = false
			re.Msg = "registered_relogin_required"
		}
	}
	return renderLocalizedRe(c, re)
}

func (s *AuthHTTPServer) FindPassword(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	args["title"], args["subTitle"] = c.Message("findPassword"), c.Message("findPassword")
	return c.RenderTemplate("home/find_password.html", args)
}

func (s *AuthHTTPServer) DoFindPassword(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if pwdService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = pwdService.FindPwd(c.Params.String("email"))
	return c.RenderJSON(re)
}

func (s *AuthHTTPServer) FindPassword2(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	args["title"], args["subTitle"] = c.Message("findPassword"), c.Message("findPassword")
	token := c.Params.String("token")
	if token == "" || tokenService == nil {
		return c.RenderTemplate("home/find_password2_timeout.html", args)
	}
	ok, _, findPwd := tokenService.VerifyToken(token, info.TokenPwd)
	if !ok {
		return c.RenderTemplate("home/find_password2_timeout.html", args)
	}
	args["findPwd"] = findPwd
	args["title"], args["subTitle"] = c.Message("updatePassword"), c.Message("updatePassword")
	return c.RenderTemplate("home/find_password2.html", args)
}

func (s *AuthHTTPServer) FindPasswordUpdate(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if re.Ok, re.Msg = lea.Vd("password", c.Params.String("pwd")); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	if pwdService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = pwdService.UpdatePwd(c.Params.String("token"), c.Params.String("pwd"))
	return renderLocalizedRe(c, re)
}

type CaptchaHTTPServer struct{}

func (s *CaptchaHTTPServer) Get(c *httpserver.Context) httpserver.Result {
	if sessionService == nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	sessionID, err := anonymousSessionID(c)
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	image, value := captcha.Fetch()
	if err := sessionService.SetCaptcha(sessionID, value); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	var body bytes.Buffer
	if _, err := image.WriteTo(&body); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	return httpserver.BinaryResult(http.StatusOK, "image/png", body.Bytes())
}

type IndexHTTPServer struct{}

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

func (s *IndexHTTPServer) Default(c *httpserver.Context) httpserver.Result {
	if configService != nil && configService.HomePageIsAdminsBlog() {
		return (&BlogHTTPServer{}).renderBlogPage(c)
	}
	return s.Index(c)
}

func (s *IndexHTTPServer) Index(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	if userID := c.GetPrincipal().UserID; userID != "" && userService != nil {
		args["userInfo"] = userService.GetUserInfo(userID)
	}
	args["title"] = "leanote"
	if configService != nil {
		args["openRegister"] = configService.GlobalStringConfigs["openRegister"]
	}
	return c.RenderTemplate("home/index.html", args)
}

func (s *IndexHTTPServer) Suggestion(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if suggestionService == nil {
		re.Msg = "suggestion.side_effect"
		return c.RenderJSON(re)
	}
	userID := domain.ObjectID{}
	if id := c.GetPrincipal().UserID; db.IsValidObjectIDHex(id) {
		userID = db.MustObjectIDFromHex(id)
	}
	result, err := suggestionService.AddSuggestionDurable(requestContext(c), info.Suggestion{
		Addr: c.Params.String("addr"), UserId: userID, Suggestion: c.Params.String("suggestion"),
	}, c.Params.String("submissionId"))
	if err != nil {
		applySuggestionError(&re, result, err)
		return c.RenderJSON(re)
	}
	re.Ok = true
	re.Id = result.ReconciliationID
	return c.RenderJSON(re)
}

type UserHTTPServer struct{}

func currentUser(c *httpserver.Context) info.User {
	if userService == nil || c.GetPrincipal().UserID == "" {
		return info.User{}
	}
	return userService.GetUserInfo(c.GetPrincipal().UserID)
}

func (s *UserHTTPServer) Account(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	args["userInfo"] = currentUser(c)
	args["tab"] = c.Params.Int("tab", 0)
	return c.RenderTemplate("user/account.html", args)
}

func (s *UserHTTPServer) UpdateUsername(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if c.GetPrincipal().IsDemo {
		re.Msg = "cannotUpdateDemo"
		return renderLocalizedRe(c, re)
	}
	if re.Ok, re.Msg = lea.Vd("username", c.Params.String("username")); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	if userService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = userService.UpdateUsername(c.GetPrincipal().UserID, c.Params.String("username"))
	if re.Ok {
		if err := updateWebSession(c, "Username", c.Params.String("username")); err != nil {
			re.Ok, re.Msg = false, "session_commit"
		}
	}
	return renderLocalizedRe(c, re)
}

func (s *UserHTTPServer) UpdatePwd(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if c.GetPrincipal().IsDemo {
		re.Msg = "cannotUpdateDemo"
		return renderLocalizedRe(c, re)
	}
	if re.Ok, re.Msg = lea.Vd("password", c.Params.String("oldPwd")); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	if re.Ok, re.Msg = lea.Vd("password", c.Params.String("pwd")); !re.Ok {
		return renderLocalizedRe(c, re)
	}
	if userService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = userService.UpdatePwd(c.GetPrincipal().UserID, c.Params.String("oldPwd"), c.Params.String("pwd"))
	return renderLocalizedRe(c, re)
}

func (s *UserHTTPServer) UpdateTheme(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if userService == nil {
		return c.RenderJSON(re)
	}
	theme := c.Params.String("theme")
	re.Ok = userService.UpdateTheme(c.GetPrincipal().UserID, theme)
	if re.Ok {
		if err := updateWebSession(c, "Theme", theme); err != nil {
			re.Ok, re.Msg = false, "session_commit"
		}
	}
	return c.RenderJSON(re)
}

func (s *UserHTTPServer) SendRegisterEmail(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if emailService == nil || c.Params.String("content") == "" || !lea.IsEmail(c.Params.String("toEmail")) {
		return c.RenderJSON(re)
	}
	re.Ok = emailService.SendInviteEmail(currentUser(c), c.Params.String("toEmail"), c.Params.String("content"))
	return c.RenderJSON(re)
}

func (s *UserHTTPServer) ReSendActiveEmail(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if emailService == nil {
		return c.RenderJSON(re)
	}
	email, _, err := c.Get("Email")
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	re.Ok = emailService.RegisterSendActiveEmail(currentUser(c), email)
	return c.RenderJSON(re)
}

func (s *UserHTTPServer) UpdateEmail(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	user := currentUser(c)
	args["userInfo"] = user
	ok, msg, email := false, "storage", ""
	if userService != nil {
		ok, msg, email = userService.UpdateEmail(c.Params.String("token"))
	}
	args["title"], args["ok"], args["msg"], args["email"] = "验证邮箱", ok, msg, email
	if ok {
		if err := updateWebSession(c, "Email", email); err != nil {
			args["ok"], args["msg"] = false, "session_commit"
		}
	}
	return c.RenderTemplate("user/update_email.html", args)
}

func (s *UserHTTPServer) ActiveEmail(c *httpserver.Context) httpserver.Result {
	args := prepareView(c)
	user := currentUser(c)
	args["userInfo"] = user
	ok, msg, email := false, "storage", ""
	if userService != nil {
		ok, msg, email = userService.ActiveEmail(c.Params.String("token"))
	}
	args["title"], args["ok"], args["msg"], args["email"] = "验证邮箱", ok, msg, email
	if ok {
		if err := updateWebSession(c, "Verified", "1"); err != nil {
			args["ok"], args["msg"] = false, "session_commit"
		}
	}
	return c.RenderTemplate("user/active_email.html", args)
}

func (s *UserHTTPServer) UpdateColumnWidth(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if userService == nil {
		return c.RenderJSON(re)
	}
	notebookWidth := c.Params.Int("notebookWidth", 0)
	noteListWidth := c.Params.Int("noteListWidth", 0)
	mdEditorWidth := c.Params.Int("mdEditorWidth", 0)
	re.Ok = userService.UpdateColumnWidth(c.GetPrincipal().UserID, notebookWidth, noteListWidth, mdEditorWidth)
	if re.Ok {
		for key, value := range map[string]string{
			"NotebookWidth": strconvItoa(notebookWidth),
			"NoteListWidth": strconvItoa(noteListWidth),
			"MdEditorWidth": strconvItoa(mdEditorWidth),
		} {
			if err := updateWebSession(c, key, value); err != nil {
				re.Ok, re.Msg = false, "session_commit"
				break
			}
		}
	}
	return c.RenderJSON(re)
}

func (s *UserHTTPServer) UpdateLeftIsMin(c *httpserver.Context) httpserver.Result {
	re := info.NewRe()
	if userService == nil {
		return c.RenderJSON(re)
	}
	leftIsMin := c.Params.Bool("leftIsMin", false)
	re.Ok = userService.UpdateLeftIsMin(c.GetPrincipal().UserID, leftIsMin)
	if re.Ok {
		if err := updateWebSession(c, "LeftIsMin", boolSessionValue(leftIsMin)); err != nil {
			re.Ok, re.Msg = false, "session_commit"
		}
	}
	return c.RenderJSON(re)
}
