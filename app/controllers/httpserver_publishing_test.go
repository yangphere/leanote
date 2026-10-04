package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/service"
)

func TestPublishingJSONPCallbackValidation(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		ok          bool
	}{
		{"cb", "valid identifier", true},
		{"app.callbacks.done", "dotted identifier", true},
		{"", "required callback", false},
		{"x);alert(1)//", "javascript payload", false},
		{"a-b", "hyphenated identifier", false},
	} {
		if got := validJSONPCallback(tc.value, false); got != tc.ok {
			t.Errorf("callback %q (%s) = %v, want %v", tc.value, tc.want, got, tc.ok)
		}
	}
	if !validJSONPCallback("", true) {
		t.Fatal("optional callback should allow an empty value")
	}
}

func TestVersionedBlogAssetURL(t *testing.T) {
	if got := versionedBlogAssetURL("/public/blog/css/share_comment.css", "2.7.1"); got != "/public/blog/css/share_comment.css?v=2.7.1" {
		t.Fatalf("versioned asset URL = %q", got)
	}
	if got := versionedBlogAssetURL("/public/blog/css/share_comment.css", ""); got != "/public/blog/css/share_comment.css" {
		t.Fatalf("empty version should preserve asset URL, got %q", got)
	}
}

func TestPublishingCommentSubmissionIDShape(t *testing.T) {
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"0123456789abcdef0123456789abcdef", true},
		{"0123456789ABCDEF0123456789abcdef", false},
		{"0123456789abcdef", false},
		{"0123456789abcdef0123456789abcdef0", false},
	} {
		if got := commentSubmissionPattern.MatchString(tc.value); got != tc.ok {
			t.Errorf("submissionId %q = %v, want %v", tc.value, got, tc.ok)
		}
	}
}

func installShareTemplates(t *testing.T) {
	t.Helper()
	templates, err := httpserver.LoadTemplates("../views")
	if err != nil {
		t.Fatal(err)
	}
	previousRenderer := httpserver.TemplateRenderer
	t.Cleanup(func() { httpserver.TemplateRenderer = previousRenderer })
	httpserver.TemplateRenderer = httpserver.TemplateSetRenderer(templates)
}

func TestShareInfoHTTPActionsRenderRemoteModal(t *testing.T) {
	installShareTemplates(t)
	previousService := shareService
	previousNotes, previousNotebooks := db.ShareNotes, db.ShareNotebooks
	t.Cleanup(func() {
		shareService = previousService
		db.ShareNotes, db.ShareNotebooks = previousNotes, previousNotebooks
	})
	shareService = &service.ShareService{}
	db.ShareNotes, db.ShareNotebooks = nil, nil
	for _, tc := range []struct{ action, parameter string }{
		{"ListNoteShareUserInfo", "noteId"},
		{"ListNotebookShareUserInfo", "notebookId"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			app := firstPartyApp(t, "test")
			entry, ok := app.Registry.Lookup("Share", tc.action)
			if !ok || len(entry.Befores) != 2 {
				t.Fatal("share action must retain session and authentication hooks")
			}
			path := "/share/" + tc.action
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
				t.Fatalf("anonymous share response = %d", rec.Code)
			}
			// Replace only authentication dependencies; use the registered action
			// and real template renderer through the native HTTP boundary.
			entry.Befores = []httpserver.BeforeFunc{func(c *httpserver.Context) httpserver.Result {
				c.SetPrincipal(httpserver.AuthenticatedPrincipal("507f1f77bcf86cd799439011", httpserver.PrincipalSourceWebSession, httpserver.TokenStateAbsent))
				return nil
			}}
			for _, id := range []string{"507f1f77bcf86cd799439012", "invalid"} {
				rec = httptest.NewRecorder()
				app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?"+tc.parameter+"="+id, nil))
				if id == "invalid" {
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("invalid resource status = %d", rec.Code)
					}
					continue
				}
				if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
					t.Fatalf("share response = %d %s", rec.Code, rec.Header().Get("Content-Type"))
				}
				for _, fragment := range []string{`class="modal-dialog`, `class="modal-content"`, `id="shareNotebookTable"`, `id="friendsEmail"`, `addShareNoteOrNotebook(1)`} {
					if !strings.Contains(rec.Body.String(), fragment) {
						t.Fatalf("share modal missing %q", fragment)
					}
				}
			}
		})
	}
}

func TestShareInfoTemplateEscapesUserData(t *testing.T) {
	installShareTemplates(t)
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(parseRoutesForTest(t, "GET /share-fixture ShareFixture.Render")),
		Registry: httpserver.NewRegistry(),
	}
	app.Registry.Register("ShareFixture", "Render", nil, func(c *httpserver.Context) httpserver.Result {
		return renderShareUserInfo(c, "507f1f77bcf86cd799439011", true, []info.ShareUserInfo{{
			Email: `friend"><script>alert(1)</script>@example.test`,
			Perm:  1, ToUserId: db.MustObjectIDFromHex("507f1f77bcf86cd799439012"),
		}})
	})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/share-fixture", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "<script>") {
		t.Fatalf("unsafe share template response: status=%d", rec.Code)
	}
	for _, fragment := range []string{"&lt;script&gt;", `class="btn btn-secondary change-perm"`, `class="btn btn-warning delete-share"`, `noteOrNotebookId="507f1f77bcf86cd799439011"`, `toUserId="507f1f77bcf86cd799439012"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("share modal missing %q", fragment)
		}
	}
}
