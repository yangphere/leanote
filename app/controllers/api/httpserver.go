package api

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
)

// apiCommonUrl is the ported api whitelist (api/init.go commonUrl):
// actions reachable without a valid token.
var apiCommonUrl = map[string]map[string]bool{
	"ApiAuth": {"Login": true, "Logout": true, "Register": true},
	"ApiFile": {"GetImage": true, "GetAttach": true, "GetAllAttachs": true},
}

// RegisterHTTP wires the first-party api actions. Callers must have run the
// InitService chain (service → controllers → api) first: the actions use
// the package service singletons.
func RegisterHTTP(rs *httpserver.Registry, runMode string, policies ...httpserver.PrincipalPolicy) {
	var policy httpserver.PrincipalPolicy
	if len(policies) > 0 {
		policy = policies[0]
	} else if configService != nil {
		policy = PrincipalPolicyFromConfig()
	}
	before := apiAuthBefore(apiCommonUrl, policy)

	auth := &ApiAuthServer{}
	rs.RegisterMethods("ApiAuth", "Login", []string{"GET", "POST"}, []httpserver.BeforeFunc{before}, auth.Login)
	rs.RegisterMethods("ApiAuth", "Logout", []string{"GET", "POST"}, []httpserver.BeforeFunc{before}, auth.Logout)
	rs.RegisterMethods("ApiAuth", "Register", []string{"POST"}, []httpserver.BeforeFunc{before}, auth.Register)

	user := &ApiUserServer{}
	rs.RegisterMethods("ApiUser", "Info", []string{"GET"}, []httpserver.BeforeFunc{before}, user.Info)
	rs.RegisterMethods("ApiUser", "UpdateUsername", []string{"POST"}, []httpserver.BeforeFunc{before}, user.UpdateUsername)
	rs.RegisterMethods("ApiUser", "UpdatePwd", []string{"POST"}, []httpserver.BeforeFunc{before}, user.UpdatePwd)
	rs.RegisterMethods("ApiUser", "UpdateLogo", []string{"POST"}, []httpserver.BeforeFunc{before}, user.UpdateLogo)
	rs.RegisterMethods("ApiUser", "GetSyncState", []string{"POST"}, []httpserver.BeforeFunc{before}, user.GetSyncState)

	tag := &ApiTagServer{}
	rs.Register("ApiTag", "GetSyncTags", []httpserver.BeforeFunc{before}, tag.GetSyncTags)
	rs.Register("ApiTag", "AddTag", []httpserver.BeforeFunc{before}, tag.AddTag)
	rs.Register("ApiTag", "DeleteTag", []httpserver.BeforeFunc{before}, tag.DeleteTag)
	RegisterNotesContentHTTP(rs, before)
}

// apiUserId returns the bound userId the api interceptor stored in the
// session (_userId).
func apiUserId(c *httpserver.Context) string {
	return c.GetPrincipal().UserID
}

// apiAuthBefore ports the api AuthInterceptor: token param (or web-session
// fallback) resolves the userId via sessionService; _token/_userId are
// written back into the session cookie; unmet auth renders the NOTLOGIN
// envelope. Whitelisted actions pass through.
func apiAuthBefore(whitelist map[string]map[string]bool, policy httpserver.PrincipalPolicy) httpserver.BeforeFunc {
	return func(c *httpserver.Context) httpserver.Result {
		if policy != nil {
			c.PrincipalPolicy = policy
		}
		if sessionService == nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		token, supplied := c.Params.Get("token")
		if !supplied {
			var ok bool
			var err error
			token, ok, err = c.Get("_ID")
			if err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			if !ok || token == "" {
				var err error
				token, err = db.NewAnonymousSessionID()
				if err != nil {
					return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
				}
				if err := c.Set("_ID", token); err != nil {
					return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
				}
			}
		}

		// Explicit invalid tokens never fall back to a cookie identity. The
		// same rule applies to an unmapped anonymous _ID: lookup is read-only.
		userId, err := sessionService.ResolveUserID(token)
		if err == nil && userId != "" {
			if requestedID, present := c.Params.Get("userId"); present && strings.TrimSpace(requestedID) != "" && strings.TrimSpace(requestedID) != userId {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "forbidden"})
			}
			source := httpserver.PrincipalSourceAPIToken
			if !supplied {
				source = httpserver.PrincipalSourceWebSession
			}
			if err := c.SetAuthenticatedPrincipal(userId, source, tokenState(source)); err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: apiDemoPolicyErrorMessage(err)})
			}
			if err := c.Set("_token", token); err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			if err := c.Set("_userId", userId); err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
		} else {
			if requestedID, present := c.Params.Get("userId"); present && strings.TrimSpace(requestedID) != "" {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "not_authenticated"})
			}
			if err != nil && !errors.Is(err, db.ErrSessionNotFound) {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			if err := c.Delete("_token"); err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			if err := c.Delete("_userId"); err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			c.SetPrincipal(httpserver.AnonymousPrincipal())
		}

		if !httpserver.NeedValidateWhitelist(whitelist, c.Controller, c.Action) {
			return nil
		}
		if userId != "" {
			required, gateErr := service.AdminPasswordChangeRequired(userId)
			if gateErr != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			if required && !(c.Controller == "ApiUser" && c.Action == "UpdatePwd") {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "admin_password_change_required"})
			}
			return nil
		}
		re := info.NewApiRe()
		re.Msg = "NOTLOGIN"
		return c.RenderJSON(re)
	}
}

// ApiUserServer is the standard-library adapter for the identity actions that
// were previously only reachable through the legacy controller.
type ApiUserServer struct{}

func apiDemoPolicyErrorMessage(err error) string {
	if errors.Is(err, service.ErrDemoConfiguration) {
		return "configuration"
	}
	return "storage"
}

func (s *ApiUserServer) Info(c *httpserver.Context) httpserver.Result {
	if userService == nil {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
	}
	userInfo := userService.GetUserInfo(apiUserId(c))
	if userInfo.UserId.IsZero() {
		return c.RenderJSON(info.NewApiRe())
	}
	return c.RenderJSON(info.ApiUser{
		UserId: userInfo.UserId.Hex(), Username: userInfo.Username, Email: userInfo.Email,
		Logo: userInfo.Logo, Verified: userInfo.Verified,
	})
}

func (s *ApiUserServer) UpdateUsername(c *httpserver.Context) httpserver.Result {
	re := info.NewApiRe()
	if err := ensureAPIDemoPolicy(c); err != nil {
		re.Msg = apiDemoPolicyErrorMessage(err)
		return c.RenderJSON(re)
	}
	if userService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if c.GetPrincipal().IsDemo {
		re.Msg = "cannotUpdateDemo"
		return c.RenderJSON(re)
	}
	if re.Ok, re.Msg = Vd("username", c.Params.String("username")); !re.Ok {
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = userService.UpdateUsername(apiUserId(c), c.Params.String("username"))
	return c.RenderJSON(re)
}

func (s *ApiUserServer) UpdatePwd(c *httpserver.Context) httpserver.Result {
	re := info.NewApiRe()
	if err := ensureAPIDemoPolicy(c); err != nil {
		re.Msg = apiDemoPolicyErrorMessage(err)
		return c.RenderJSON(re)
	}
	if userService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if c.GetPrincipal().IsDemo {
		re.Msg = "cannotUpdateDemo"
		return c.RenderJSON(re)
	}
	oldPwd, pwd := c.Params.String("oldPwd"), c.Params.String("pwd")
	required, err := service.AdminPasswordChangeRequired(apiUserId(c))
	if err != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if !required {
		if re.Ok, re.Msg = Vd("password", oldPwd); !re.Ok {
			return c.RenderJSON(re)
		}
	}
	if re.Ok, re.Msg = Vd("password", pwd); !re.Ok {
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = userService.UpdatePwd(apiUserId(c), oldPwd, pwd)
	return c.RenderJSON(re)
}

func (s *ApiUserServer) GetSyncState(c *httpserver.Context) httpserver.Result {
	if userService == nil {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
	}
	return c.RenderJSON(map[string]interface{}{"LastSyncUsn": userService.GetUsn(apiUserId(c)), "LastSyncTime": time.Now().Unix()})
}

func (s *ApiUserServer) UpdateLogo(c *httpserver.Context) httpserver.Result {
	re := info.NewApiRe()
	if err := ensureAPIDemoPolicy(c); err != nil {
		re.Msg = apiDemoPolicyErrorMessage(err)
		return c.RenderJSON(re)
	}
	if userService == nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if c.GetPrincipal().IsDemo {
		re.Msg = "cannotUpdateDemo"
		return c.RenderJSON(re)
	}
	file, header, err := c.Params.FormFile("file")
	if err != nil || file == nil || header == nil {
		re.Msg = "fileRequired"
		return c.RenderJSON(re)
	}
	defer file.Close()
	data, readErr := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if readErr != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if len(data) == 0 {
		re.Msg = "fileRequired"
		return c.RenderJSON(re)
	}
	if len(data) > 5*1024*1024 {
		re.Msg = "fileIsTooLarge"
		return c.RenderJSON(re)
	}
	_, ext := SplitFilename(header.Filename)
	if ext != ".gif" && ext != ".jpg" && ext != ".png" && ext != ".bmp" && ext != ".jpeg" {
		re.Msg = "notImage"
		return c.RenderJSON(re)
	}
	publication, err := service.PublishAPIAvatar(c.Request.Context(), apiUserId(c), header.Filename, data)
	if err != nil {
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	if !userService.UpdateAvatar(apiUserId(c), publication.Path) {
		_ = service.CleanupAPIAvatar(c.Request.Context(), publication)
		re.Msg = "storage"
		return c.RenderJSON(re)
	}
	return c.RenderJSON(map[string]string{"Logo": configService.GetSiteUrl() + "/" + publication.Path})
}

func ensureAPIDemoPolicy(c *httpserver.Context) error {
	if configService == nil || !configService.GlobalSnapshotLoaded() {
		return service.ErrGlobalConfigNotLoaded
	}
	return nil
}

// globalConfigLoaded reports whether the global configuration snapshot the
// principal policy reads has been published. Package-level so tests can
// exercise a loaded policy without MongoDB.
var globalConfigLoaded = func() bool {
	return configService.GlobalSnapshotLoaded()
}

// PrincipalPolicyFromConfig derives admin/demo facts from the loaded global
// configuration snapshot. An unloaded snapshot fails closed with
// service.ErrGlobalConfigNotLoaded instead of reading as "demo/admin not
// configured"; the D-H9 readiness gate keeps requests away until it loads.
func PrincipalPolicyFromConfig() httpserver.PrincipalPolicy {
	return func(userID string) (httpserver.PrincipalRole, bool, error) {
		if configService == nil {
			return "", false, errors.New("identity configuration is not initialized")
		}
		if !globalConfigLoaded() {
			return "", false, service.ErrGlobalConfigNotLoaded
		}
		role := httpserver.PrincipalRoleMember
		if adminID := strings.TrimSpace(configService.GetAdminUserId()); adminID != "" && userID == adminID {
			role = httpserver.PrincipalRoleAdmin
		}
		demoID := strings.TrimSpace(configService.GetGlobalStringConfig("demoUserId"))
		demoLogin := strings.TrimSpace(configService.GetGlobalStringConfig("demoUsername"))
		if demoID == "" && demoLogin == "" {
			return role, false, nil
		}
		isDemo, err := configService.IsDemoUser(userID)
		if err != nil {
			return "", false, err
		}
		return role, isDemo, nil
	}
}

func tokenState(source httpserver.PrincipalSource) httpserver.TokenState {
	if source == httpserver.PrincipalSourceAPIToken {
		return httpserver.TokenStateValid
	}
	return httpserver.TokenStateAbsent
}

// ApiAuthServer is the first-party host of the api auth actions.
type ApiAuthServer struct{}

// Login issues a token for the account and stores token→userId.
func (s *ApiAuthServer) Login(c *httpserver.Context) httpserver.Result {
	userInfo, err := authService.Login(c.Params.String("email"), c.Params.String("pwd"))
	if err == nil {
		token, err := sessionService.IssueUserToken(userInfo.UserId.Hex())
		if err != nil {
			return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
		}
		return c.RenderJSON(info.AuthOk{Ok: true, Token: token, UserId: userInfo.UserId, Email: userInfo.Email, Username: userInfo.Username})
	}
	if !errors.Is(err, service.ErrInvalidCredentials) {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
	}
	re := info.ApiRe{Ok: false, Msg: c.Message("wrongUsernameOrPassword")}
	return c.RenderJSON(re)
}

// Logout clears the token's stored userId.
func (s *ApiAuthServer) Logout(c *httpserver.Context) httpserver.Result {
	token := c.Params.String("token")
	if _, err := sessionService.ClearUserToken(token); err != nil {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "logout_cleanup_failed"})
	}
	re := info.ApiRe{Ok: true}
	return c.RenderJSON(re)
}

// Register creates an account when registration is open.
func (s *ApiAuthServer) Register(c *httpserver.Context) httpserver.Result {
	re := info.NewApiRe()
	if !configService.IsOpenRegister() {
		re.Msg = "notOpenRegister"
		return c.RenderJSON(re)
	}
	email := c.Params.String("email")
	pwd := c.Params.String("pwd")
	if re.Ok, re.Msg = Vd("email", email); !re.Ok {
		return c.RenderJSON(re)
	}
	if re.Ok, re.Msg = Vd("password", pwd); !re.Ok {
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = authService.Register(email, pwd, "")
	return c.RenderJSON(re)
}

// ApiTagServer is the first-party host of the api tag actions.
type ApiTagServer struct{}

// GetSyncTags returns tags with Usn after afterUsn, capped by maxEntry.
func (s *ApiTagServer) GetSyncTags(c *httpserver.Context) httpserver.Result {
	maxEntry := c.Params.Int("maxEntry", 0)
	if maxEntry == 0 {
		maxEntry = 100
	}
	tags := tagService.GeSyncTags(apiUserId(c), c.Params.Int("afterUsn", 0), maxEntry)
	return c.RenderJSON(tags)
}

// AddTag adds or updates a tag for the bound user.
func (s *ApiTagServer) AddTag(c *httpserver.Context) httpserver.Result {
	ret := tagService.AddOrUpdateTag(apiUserId(c), c.Params.String("tag"))
	return c.RenderJSON(ret)
}

// DeleteTag removes a tag at the given Usn.
func (s *ApiTagServer) DeleteTag(c *httpserver.Context) httpserver.Result {
	re := info.NewReUpdate()
	re.Ok, re.Msg, re.Usn = tagService.DeleteTagApi(apiUserId(c), c.Params.String("tag"), c.Params.Int("usn", 0))
	return c.RenderJSON(re)
}
