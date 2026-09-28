package api

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
var leanoteUserId = "admin"

type authServiceContract interface {
	Login(string, string) (info.User, error)
	Register(string, string, string) (bool, string)
}

var authService authServiceContract

// InitService binds the shared service singletons before the native registry
// is created. Authentication policy is implemented by HTTP adapter hooks,
// so this package has no runtime interceptor registration side effect.
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
