package controllers

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
)

// notePDFDependencies isolates the database-backed lookups from the HTTP
// adapter. Production uses the service singletons; tests can exercise the
// real route and template without requiring MongoDB.
type notePDFDependencies struct {
	GetNoteByID    func(string) info.Note
	GetNoteContent func(string, string) info.NoteContent
	GetUserInfo    func(string) info.User
	GetUserBlog    func(string) info.UserBlog
	GetImageBase64 func(string, string) string
}

// NotePDFServer exposes the legacy /note/toPdf action through the first-party
// HTTP registry. AppSecret is read from the validated production config.
type NotePDFServer struct {
	AppSecret    string
	SiteURL      string
	Dependencies notePDFDependencies
}

// NewNotePDFServer builds the production adapter from the validated config.
func NewNotePDFServer(cfg *httpserver.Config) *NotePDFServer {
	secret, _ := cfg.String("app.secret")
	return &NotePDFServer{
		AppSecret: secret,
		SiteURL:   cfg.StringDefault("site.url", ""),
	}
}

// Register installs Note.ToPdf in the first-party action registry.
func (s *NotePDFServer) Register(rs *httpserver.Registry) {
	s.ensureDependencies()
	rs.Register("Note", "ToPdf", nil, s.toPDF)
}

func (s *NotePDFServer) ensureDependencies() {
	if s.Dependencies.GetNoteByID == nil {
		s.Dependencies.GetNoteByID = service.NoteS.GetNoteById
	}
	if s.Dependencies.GetNoteContent == nil {
		s.Dependencies.GetNoteContent = service.NoteS.GetNoteContent
	}
	if s.Dependencies.GetUserInfo == nil {
		s.Dependencies.GetUserInfo = service.UserS.GetUserInfo
	}
	if s.Dependencies.GetUserBlog == nil {
		s.Dependencies.GetUserBlog = service.BlogS.GetUserBlog
	}
	if s.Dependencies.GetImageBase64 == nil {
		s.Dependencies.GetImageBase64 = service.FileS.GetImageBase64
	}
}

func (s *NotePDFServer) toPDF(c *httpserver.Context) httpserver.Result {
	appKey := c.Params.String("appKey")
	if s.AppSecret == "" || subtle.ConstantTimeCompare([]byte(s.AppSecret), []byte(appKey)) != 1 {
		return c.RenderText("auth error")
	}

	noteID := c.Params.String("noteId")
	if !lea.IsObjectId(noteID) {
		return c.RenderText("no note")
	}
	args, ok := buildNotePDFView(noteID, s.SiteURL, requestOrigin(c.Request), s.Dependencies)
	if !ok {
		return c.RenderText("no note")
	}
	return c.RenderTemplate("file/pdf.html", args)
}

func buildNotePDFView(noteID, configuredSiteURL, requestSiteURL string, deps notePDFDependencies) (map[string]interface{}, bool) {
	note := deps.GetNoteByID(noteID)
	if note.NoteId.IsZero() {
		return nil, false
	}
	noteUserID := note.UserId.Hex()
	content := deps.GetNoteContent(noteID, noteUserID)
	contentStr := replaceNotePDFImages(content.Content, note.IsMarkdown, noteUserID, configuredSiteURL, requestSiteURL, deps.GetImageBase64)
	if len(note.Tags) == 0 || note.Tags[0] == "" {
		note.Tags = nil
	}
	return map[string]interface{}{
		"blog":     note,
		"content":  contentStr,
		"userInfo": deps.GetUserInfo(noteUserID),
		"userBlog": deps.GetUserBlog(noteUserID),
	}, true
}

func requestOrigin(r *http.Request) string {
	if r == nil || r.Host == "" {
		return ""
	}
	scheme := "http"
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func replaceNotePDFImages(content string, markdown bool, userID, configuredSiteURL, requestSiteURL string, getImageBase64 func(string, string) string) string {
	if getImageBase64 == nil || content == "" {
		return content
	}
	origins := pdfImageOrigins(configuredSiteURL, requestSiteURL)
	originPattern := ""
	if len(origins) > 0 {
		originPattern = "(?:" + strings.Join(origins, "|") + ")"
	}
	prefix := "(?:" + originPattern + ")?"
	htmlPattern := regexp.MustCompile(`(?is)(<img\b[^>]*?\ssrc=)([\"'])` + prefix + `/((?:file/outputImage|api/file/getImage))\?fileId=([a-zA-Z0-9]{24})([\"'])`)
	content = htmlPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := htmlPattern.FindStringSubmatch(match)
		if len(parts) != 6 {
			return match
		}
		fileBase64 := getImageBase64(userID, parts[4])
		if fileBase64 == "" {
			return match
		}
		return parts[1] + `"` + fileBase64 + `"`
	})

	if !markdown {
		return content
	}
	markdownPattern := regexp.MustCompile(`(?is)!\[.*?\]\(` + prefix + `/(?:file/outputImage|api/file/getImage)\?fileId=([a-zA-Z0-9]{24})\)`)
	return markdownPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := markdownPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		fileBase64 := getImageBase64(userID, parts[1])
		if fileBase64 == "" {
			return match
		}
		return "![](" + fileBase64 + ")"
	})
}

func pdfImageOrigins(siteURLs ...string) []string {
	seen := map[string]bool{}
	origins := make([]string, 0, len(siteURLs))
	for _, siteURL := range siteURLs {
		u, err := url.Parse(strings.TrimSpace(siteURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		path := strings.TrimRight(u.EscapedPath(), "/")
		origin := `https?://` + regexp.QuoteMeta(u.Host) + regexp.QuoteMeta(path)
		if !seen[origin] {
			seen[origin] = true
			origins = append(origins, origin)
		}
	}
	return origins
}
