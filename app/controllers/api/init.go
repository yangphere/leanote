package api

import (
	"errors"
	"strings"

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
	//		. "github.com/yangphere/leanote/app/lea"
)

var userService *service.UserService
var noteService *service.NoteService
var trashService *service.TrashService
var notebookService *service.NotebookService
var noteContentHistoryService *service.NoteContentHistoryService
var shareService *service.ShareService
var blogService *service.BlogService
var tagService *service.TagService
var pwdService *service.PwdService
var tokenService *service.TokenService
var suggestionService *service.SuggestionService
var albumService *service.AlbumService
var noteImageService *service.NoteImageService
var fileService *service.FileService
var attachService *service.AttachService
var configService *service.ConfigService
var emailService *service.EmailService
var sessionService *service.SessionService

var pageSize = 1000
var defaultSortField = "UpdatedTime"
var leanoteUserId = "admin" // 不能更改

type authServiceContract interface {
	Login(string, string) (info.User, error)
	Register(string, string, string) (bool, string)
}

var authService authServiceContract

// 状态
const (
	S_DEFAULT                 = iota // 0
	S_NOT_LOGIN                      // 1
	S_WRONG_USERNAME_PASSWORD        // 2
	S_WRONG_CAPTCHA                  // 3
	S_NEED_CAPTCHA                   // 4
	S_NOT_OPEN_REGISTER              // 4
)

// 拦截器
// 不需要拦截的url
var commonUrl = map[string]map[string]bool{"ApiAuth": map[string]bool{"Login": true,
	"Logout":   true,
	"Register": true,
},
	// 文件的操作也不用登录, userId会从session中获取
	"ApiFile": map[string]bool{"GetImage": true,
		"GetAttach":     true,
		"GetAllAttachs": true,
	},
}

func needValidate(controller, method string) bool {
	// 在里面
	if v, ok := commonUrl[controller]; ok {
		// 在commonUrl里
		if _, ok2 := v[method]; ok2 {
			return false
		}
		return true
	} else {
		// controller不在这里的, 肯定要验证
		return true
	}
}

// 这里得到token, 若不是login, logout等公用操作, 必须验证是否已登录
func AuthInterceptor(c *revel.Controller) revel.Result {
	// 得到token /api/user/info?userId=xxx&token=xxxxx
	provided := false
	token := ""
	if values, ok := c.Params.Values["token"]; ok {
		provided = true
		if len(values) > 0 {
			token = values[0]
		}
	}
	if !provided {
		if value, ok := c.Session["_ID"]; ok {
			token, _ = value.(string)
		}
		if token == "" {
			var err error
			token, err = db.NewAnonymousSessionID()
			if err != nil {
				return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
			}
			c.Session["_ID"] = token
		}
	}

	// 全部变成首字大写
	var controller = strings.Title(c.Name)
	var method = strings.Title(c.MethodName)

	// 验证是否已登录
	// 通过sessionService判断该token下是否有userId, 并返回userId
	userId, err := sessionService.ResolveUserID(token)
	if err != nil && !errors.Is(err, db.ErrSessionNotFound) {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "storage"})
	}
	if err == nil && userId != "" {
		c.Session["_token"] = token
		c.Session["_userId"] = userId
	} else {
		delete(c.Session, "_token")
		delete(c.Session, "_userId")
	}

	// 是否需要验证?
	if !needValidate(controller, method) {
		return nil
	}

	if userId != "" {
		return nil // 已登录
	}

	// 没有登录, 返回错误的信息, 需要登录
	re := info.NewApiRe()
	re.Msg = "NOTLOGIN"
	return c.RenderJSON(re)
}

func init() {
	// interceptors
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiAuth{})
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiUser{})
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiFile{})
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiNote{})
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiTag{})
	revel.InterceptFunc(AuthInterceptor, revel.BEFORE, &ApiNotebook{})
}

// 最外层init.go调用
// 获取service, 单例
func InitService() {
	notebookService = service.NotebookS
	noteService = service.NoteS
	noteContentHistoryService = service.NoteContentHistoryS
	trashService = service.TrashS
	shareService = service.ShareS
	userService = service.UserS
	tagService = service.TagS
	blogService = service.BlogS
	tokenService = service.TokenS
	noteImageService = service.NoteImageS
	fileService = service.FileS
	albumService = service.AlbumS
	attachService = service.AttachS
	pwdService = service.PwdS
	suggestionService = service.SuggestionS
	authService = service.AuthS
	configService = service.ConfigS
	emailService = service.EmailS
	sessionService = service.SessionS
}
