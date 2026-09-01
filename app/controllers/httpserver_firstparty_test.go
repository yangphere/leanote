package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
)

// firstPartyApp builds the post-Revel server over the REAL conf/routes with
// the actions currently migrated into the first-party registry.
func firstPartyApp(t *testing.T, runMode string) *httpserver.App {
	t.Helper()
	data, err := os.ReadFile("../../conf/routes")
	if err != nil {
		t.Fatalf("read conf/routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(data)
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	registry := httpserver.NewRegistry()
	RegisterHTTP(registry, runMode, nil)
	return &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: registry,
	}
}

func parseRoutesForTest(t *testing.T, source string) []httpserver.Route {
	t.Helper()
	routes, err := httpserver.ParseRoutes([]byte(source))
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	return routes
}

func TestFirstPartyE2EIdentityFailsClosedWithoutDB(t *testing.T) {
	t.Setenv(e2eRunTokenEnv, "")

	// test run-mode + loopback: fail-closed 503 (no token, no database).
	app := firstPartyApp(t, "test")
	req := httptest.NewRequest("GET", "/_test/e2e/identity", nil)
	req.RemoteAddr = "127.0.0.1:51000"
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("test-mode identity status = %d, want 503 fail-closed", rec.Code)
	}
}

func TestFirstPartyE2EIdentity404OutsideTestMode(t *testing.T) {
	app := firstPartyApp(t, "dev")
	req := httptest.NewRequest("GET", "/_test/e2e/identity", nil)
	req.RemoteAddr = "127.0.0.1:51000"
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("dev-mode identity status = %d, want 404", rec.Code)
	}
}

func TestFirstPartyUnregisteredRoutes404(t *testing.T) {
	app := firstPartyApp(t, "test")
	for _, path := range []string{
		"/",      // Index.Default not migrated yet
		"/login", // Auth.Login not migrated yet
		"/note/abc123",
	} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404 until migrated", path, rec.Code)
		}
	}
	// The identity route IS reachable and its name dispatches correctly.
	req := httptest.NewRequest("GET", "/_test/e2e/identity", nil)
	req.RemoteAddr = "127.0.0.1:51000"
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "") || rec.Code == http.StatusNotFound {
		t.Fatalf("identity route should reach the registered action, got %d", rec.Code)
	}
}

func TestFirstPartyNoteToPDFRendersRealTemplate(t *testing.T) {
	views, err := httpserver.LoadTemplates("../../app/views")
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	savedRenderer := httpserver.TemplateRenderer
	httpserver.TemplateRenderer = httpserver.TemplateSetRenderer(views)
	defer func() { httpserver.TemplateRenderer = savedRenderer }()

	noteID := db.NewObjectID()
	server := &NotePDFServer{
		AppSecret: "pdf-secret",
		Dependencies: notePDFDependencies{
			GetNoteByID: func(string) info.Note {
				return info.Note{NoteId: noteID, UserId: db.NewObjectID(), Title: "Release note", Tags: []string{"ci"}}
			},
			GetNoteContent: func(string, string) info.NoteContent {
				return info.NoteContent{Content: "<p>PDF body</p>"}
			},
			GetUserInfo:    func(string) info.User { return info.User{} },
			GetUserBlog:    func(string) info.UserBlog { return info.UserBlog{} },
			GetImageBase64: func(string, string) string { return "" },
		},
	}

	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "* /note/toPdf Note.ToPdf")),
		Registry: httpserver.NewRegistry(),
	}
	server.Register(app.Registry)
	req := httptest.NewRequest(http.MethodGet, "/note/toPdf?noteId="+noteID.Hex()+"&appKey=pdf-secret", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ToPdf status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("ToPdf content type = %q, want text/html", got)
	}
	for _, want := range []string{"Release note", "PDF body"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("ToPdf body missing %q: %s", want, rec.Body.String())
		}
	}
}

func TestRegisterHTTPExposesNoteToPDFWhenProductionConfigIsPresent(t *testing.T) {
	cfg, err := httpserver.ParseConfig([]byte("[prod]\napp.secret=pdf-secret\n"), "prod")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	registry := httpserver.NewRegistry()
	RegisterHTTP(registry, "prod", cfg)
	if _, ok := registry.Lookup("Note", "ToPdf"); !ok {
		t.Fatal("production registry does not expose Note.ToPdf")
	}
}

func TestFirstPartyNoteToPDFRejectsInvalidNoteIDBeforeLookup(t *testing.T) {
	lookedUp := false
	server := &NotePDFServer{
		AppSecret: "pdf-secret",
		Dependencies: notePDFDependencies{
			GetNoteByID: func(string) info.Note {
				lookedUp = true
				return info.Note{}
			},
		},
	}
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "* /note/toPdf Note.ToPdf")),
		Registry: httpserver.NewRegistry(),
	}
	server.Register(app.Registry)
	req := httptest.NewRequest(http.MethodGet, "/note/toPdf?noteId=not-an-object-id&appKey=pdf-secret", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "no note" {
		t.Fatalf("invalid note id response = %d %q, want 200 no note", rec.Code, rec.Body.String())
	}
	if lookedUp {
		t.Fatal("invalid note id reached the database lookup")
	}
}

func TestNotePDFViewInlinesRelativeImagesAndMarkdownImages(t *testing.T) {
	noteID := db.NewObjectID()
	ownerID := db.NewObjectID()
	imageID := "5483207cf4e87203a4000001"
	args, ok := buildNotePDFView(noteID.Hex(), "", "http://127.0.0.1:9000", notePDFDependencies{
		GetNoteByID: func(string) info.Note {
			return info.Note{NoteId: noteID, UserId: ownerID, IsMarkdown: true}
		},
		GetNoteContent: func(string, string) info.NoteContent {
			return info.NoteContent{Content: "![image](/file/outputImage?fileId=" + imageID + ")"}
		},
		GetUserInfo: func(string) info.User { return info.User{} },
		GetUserBlog: func(string) info.UserBlog { return info.UserBlog{} },
		GetImageBase64: func(userID, fileID string) string {
			if userID == ownerID.Hex() && fileID == imageID {
				return "data:image/png;base64,AAAA"
			}
			return ""
		},
	})
	if !ok {
		t.Fatal("buildNotePDFView reported a missing note")
	}
	if got := args["content"]; got != "![](data:image/png;base64,AAAA)" {
		t.Fatalf("inlined markdown = %q", got)
	}
}
