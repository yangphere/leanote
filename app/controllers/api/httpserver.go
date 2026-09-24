package api

import (
	"errors"
	"strings"

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
	rs.Register("ApiAuth", "Login", []httpserver.BeforeFunc{before}, auth.Login)
	rs.Register("ApiAuth", "Logout", []httpserver.BeforeFunc{before}, auth.Logout)
	rs.Register("ApiAuth", "Register", []httpserver.BeforeFunc{before}, auth.Register)

	tag := &ApiTagServer{}
	rs.Register("ApiTag", "GetSyncTags", []httpserver.BeforeFunc{before}, tag.GetSyncTags)
	rs.Register("ApiTag", "AddTag", []httpserver.BeforeFunc{before}, tag.AddTag)
	rs.Register("ApiTag", "DeleteTag", []httpserver.BeforeFunc{before}, tag.DeleteTag)
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

		if !needValidateAPI(whitelist, c.Controller, c.Action) {
			return nil
		}
		if userId != "" {
			return nil
		}
		re := info.NewApiRe()
		re.Msg = "NOTLOGIN"
		return c.RenderJSON(re)
	}
}

func PrincipalPolicyFromConfig() httpserver.PrincipalPolicy {
	return func(userID string) (httpserver.PrincipalRole, bool, error) {
		if configService == nil {
			return "", false, errors.New("identity configuration is not initialized")
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

func needValidateAPI(whitelist map[string]map[string]bool, controller, method string) bool {
	if actions, ok := whitelist[controller]; ok {
		return !actions[method]
	}
	return true
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
