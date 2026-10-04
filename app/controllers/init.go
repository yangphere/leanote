package controllers

import (
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

var userService *service.UserService
var noteService *service.NoteService
var trashService *service.TrashService
var notebookService *service.NotebookService
var noteContentHistoryService *service.NoteContentHistoryService
var shareService *service.ShareService
var blogService *service.BlogService
var tagService *service.TagService

type pwdServiceContract interface {
	FindPwd(string) (bool, string)
	UpdatePwd(string, string) (bool, string)
}

type authServiceContract interface {
	Login(string, string) (info.User, error)
	Register(string, string, string) (bool, string)
}

type sessionServiceContract interface {
	LoginTimesIsOver(string) (bool, error)
	GetCaptcha(string) (string, error)
	IncrLoginTimes(string) error
	ClearUserToken(string) (bool, error)
	ClearTransientSessionState(string) error
	SetCaptcha(string, string) error
}

var pwdService pwdServiceContract
var authService authServiceContract
var tokenService *service.TokenService
var suggestionService *service.SuggestionService
var albumService *service.AlbumService
var noteImageService *service.NoteImageService
var fileService *service.FileService
var attachService *service.AttachService
var configService *service.ConfigService
var emailService *service.EmailService
var sessionService sessionServiceContract
var themeService *service.ThemeService

var pageSize = 1000
var defaultSortField = "UpdatedTime"

// InitService binds the application service singletons used by the native
// HTTP adapters. Registration and request handling are deliberately separate
// from this wiring step so tests can replace the services before dispatch.
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
	themeService = service.ThemeS
}
