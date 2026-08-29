package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
)

// firstPartyAPIApp boots the real conf/routes with the api batch registered.
// Requires a reachable MongoDB fixture at 127.0.0.1:27017 (skipped
// otherwise, mirroring the CI split between DB-independent and DB tests).
func firstPartyAPIApp(t *testing.T) *httpserver.App {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "127.0.0.1:27017", 300*time.Millisecond)
	if err != nil {
		t.Skip("MongoDB fixture not reachable")
	}
	conn.Close()

	initDatabaseStub()
	data, err := os.ReadFile("../../../conf/routes")
	if err != nil {
		t.Fatalf("read conf/routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(data)
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	registry := httpserver.NewRegistry()
	RegisterHTTP(registry, "test")
	return &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: registry,
	}
}

// initDatabaseStub mirrors cmd/leanote's initDatabase for tests.
func initDatabaseStub() {
	db.Init("mongodb://127.0.0.1:27017/leanote_test", "leanote_test")
}

func apiPost(t *testing.T, app *httpserver.App, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func TestApiAuthLoginRoundTrip(t *testing.T) {
	app := firstPartyAPIApp(t)

	// Wrong password: wrongUsernameOrPassword envelope.
	rec := apiPost(t, app, "/api/auth/login", "email=admin%40leanote.com&pwd=wrong")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	var bad struct {
		Ok  bool
		Msg string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bad); err != nil || bad.Ok {
		t.Fatalf("bad login body = %q err=%v", rec.Body.String(), err)
	}

	// Good credentials: token issued.
	rec = apiPost(t, app, "/api/auth/login", "email=admin%40leanote.com&pwd=abc123")
	var ok struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil || !ok.Ok || ok.Token == "" {
		t.Fatalf("login body = %q err=%v", rec.Body.String(), err)
	}
}

func TestApiAuthLogoutClearsToken(t *testing.T) {
	app := firstPartyAPIApp(t)
	login := apiPost(t, app, "/api/auth/login", "email=admin%40leanote.com&pwd=abc123")
	var issued struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(login.Body.Bytes(), &issued); err != nil || !issued.Ok {
		t.Fatalf("login = %q", login.Body.String())
	}

	rec := apiPost(t, app, "/api/auth/logout", "token="+issued.Token)
	var out struct {
		Ok bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.Ok {
		t.Fatalf("logout body = %q", rec.Body.String())
	}
	// The stored userId for the token must be gone.
	if uid := sessionService.GetUserId(issued.Token); uid != "" {
		t.Fatalf("token still resolves userId %q after logout", uid)
	}
}

func TestApiTagLifecycle(t *testing.T) {
	app := firstPartyAPIApp(t)
	login := apiPost(t, app, "/api/auth/login", "email=admin%40leanote.com&pwd=abc123")
	var issued struct {
		Ok    bool
		Token string
	}
	if err := json.Unmarshal(login.Body.Bytes(), &issued); err != nil || !issued.Ok {
		t.Fatalf("login = %q", login.Body.String())
	}

	// Add a tag: returns the tag document with Usn.
	rec := apiPost(t, app, "/api/tag/addTag", "tag=probe-tag&token="+issued.Token)
	var added struct {
		TagId string
		Usn   int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.TagId == "" {
		t.Fatalf("addTag body = %q err=%v", rec.Body.String(), err)
	}

	// GetSyncTags lists it.
	rec = apiPost(t, app, "/api/tag/getSyncTags", "afterUsn=0&maxEntry=100&token="+issued.Token)
	var tags []struct {
		Tag string
		Usn int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tags); err != nil {
		t.Fatalf("getSyncTags body = %q", rec.Body.String())
	}
	found := false
	for _, tg := range tags {
		if tg.Tag == "probe-tag" {
			found = true
		}
	}
	if !found {
		t.Fatalf("probe-tag missing from sync tags: %q", rec.Body.String())
	}

	// Delete it at the Usn the addTag response reported (tags[0] may be a
	// different tag on a warm database).
	rec = apiPost(t, app, "/api/tag/deleteTag", "tag=probe-tag&usn="+strconv.Itoa(added.Usn)+"&token="+issued.Token)
	var deleted struct {
		Ok bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil || !deleted.Ok {
		t.Fatalf("deleteTag body = %q", rec.Body.String())
	}
}
