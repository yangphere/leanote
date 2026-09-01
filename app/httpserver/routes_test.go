package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const realRoutesPath = "../../conf/routes"

func TestParseRealRoutesFile(t *testing.T) {
	data, err := os.ReadFile(realRoutesPath)
	if err != nil {
		t.Fatalf("read conf/routes: %v", err)
	}
	routes, err := ParseRoutes(data)
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	var explicit, catchAll, static int
	for _, r := range routes {
		switch {
		case r.IsStatic:
			static++
		case r.IsCatchAll:
			catchAll++
			if r.Method != "*" {
				t.Fatalf("catch-all %q must be a * route", r.Path)
			}
		default:
			explicit++
		}
	}
	if explicit != 83 {
		t.Fatalf("explicit routes = %d, want 83", explicit)
	}
	if catchAll != 3 {
		t.Fatalf("catch-alls = %d, want 3", catchAll)
	}
	if static != 9 {
		t.Fatalf("static routes = %d, want 9", static)
	}
	// Method census: GET 60 / POST 4 / * 31 across all 95.
	census := map[string]int{}
	for _, r := range routes {
		census[r.Method]++
	}
	if census["GET"] != 60 || census["POST"] != 4 || census["*"] != 31 {
		t.Fatalf("method census = %v, want GET 60 POST 4 * 31", census)
	}
	// Spot-check parsing details.
	first := routes[0]
	if first.Method != "GET" || first.Path != "/_test/e2e/identity" || first.Action != "TestE2e.Identity" {
		t.Fatalf("first route = %+v", first)
	}
	for _, r := range routes {
		if r.Path == "/findPassword/:token" && (len(r.segments) != 2 || r.segments[1].kind != segParam || r.segments[1].text != "token") {
			t.Fatalf(":param segment not compiled: %+v", r)
		}
		if r.Path == "/public/*filepath" && (len(r.segments) == 0 || r.segments[0].kind != segLiteral || r.segments[1].kind != segRest) {
			t.Fatalf("*filepath segment not compiled: %+v", r)
		}
		if r.Action == "Blog.E()" {
			t.Fatalf("Blog.E() must normalise to Blog.E, got %q", r.Action)
		}
	}
}

func TestRouteTableExplicitFirstAndCatchAlls(t *testing.T) {
	routes, err := ParseRoutes([]byte(strings.Join([]string{
		"* /note/listNotes Note.ListNotes", // registered first
		"GET /note/:noteId Note.Index",     // would also match listNotes
		"* /api/:controller/:action :controller.:action",
		"* /member/:controller/:action :controller.:action",
		"* /:controller/:action :controller.:action",
	}, "\n")))
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	table := CompileRoutes(routes)

	// Explicit first-match wins over the later :param route.
	route, params, ok := table.Match("GET", "/note/listNotes")
	if !ok || route.Action != "Note.ListNotes" || params["noteId"] != "" {
		t.Fatalf("explicit priority broken: %v %v", route, params)
	}
	// The :param route still matches other ids.
	route, params, ok = table.Match("GET", "/note/abc123")
	if !ok || route.Action != "Note.Index" || params["noteId"] != "abc123" {
		t.Fatalf("param route: %v %v", route, params)
	}
	// POST to a GET-only explicit route falls through to the catch-all.
	route, _, ok = table.Match("POST", "/note/abc123")
	if !ok || route.Action != ":controller.:action" {
		t.Fatalf("method fallthrough: %v", route)
	}
}

func TestRouteTableCatchAllDispatchNames(t *testing.T) {
	app := &App{
		Routes: CompileRoutes(mustParse(t, strings.Join([]string{
			"* /api/:controller/:action :controller.:action",
			"* /member/:controller/:action :controller.:action",
			"* /:controller/:action :controller.:action",
		}, "\n"))),
		Registry: NewRegistry(),
	}
	captured := ""
	app.Registry.Register("ApiNote", "Get", nil, func(c *Context) Result {
		captured = c.Controller + "." + c.Action
		return c.RenderText("api")
	})
	app.Registry.Register("MemberGroup", "AddGroup", nil, func(c *Context) Result {
		captured = c.Controller + "." + c.Action
		return c.RenderText("member")
	})
	app.Registry.Register("Note", "Index", nil, func(c *Context) Result {
		captured = c.Controller + "." + c.Action
		return c.RenderText("main")
	})

	for _, tc := range []struct{ path, want string }{
		{"/api/note/get", "ApiNote.Get"},
		{"/member/group/addGroup", "MemberGroup.AddGroup"},
		{"/note/index", "Note.Index"},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || captured != tc.want {
			t.Errorf("%s: status=%d captured=%q, want 200 %s", tc.path, rec.Code, captured, tc.want)
		}
	}
}

func TestRouteTableNegatives(t *testing.T) {
	app := &App{
		Routes: CompileRoutes(mustParse(t, strings.Join([]string{
			"GET /note/:noteId Note.Index",
			"* /:controller/:action :controller.:action",
		}, "\n"))),
		Registry: NewRegistry(),
	}
	app.Registry.Register("Note", "Index", nil, func(c *Context) Result {
		return c.RenderText("ok")
	})
	// /note/:noteId legitimately routes to Note.Index for any id.
	// The negatives are the catch-all names with nothing registered.
	for _, path := range []string{
		"/notebook/index", // Notebook controller not registered
		"/user/other",     // User controller not registered
		"/noSuch/method",  // no such controller at all
	} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
	}
	// A path with no route at all.
	req := httptest.NewRequest("GET", "/deeply/nested/path/here", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unroutable path status = %d", rec.Code)
	}
}

func TestStaticRouteServesFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/app.js", []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app := &App{
		Routes:   CompileRoutes(mustParse(t, `GET /js/*filepath Static.Serve("public/js")`)),
		Registry: NewRegistry(),
		StaticHandler: func(base string) http.Handler {
			return http.FileServer(http.Dir(root))
		},
	}
	req := httptest.NewRequest("GET", "/js/app.js", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" {
		t.Fatalf("static serve: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestLoginRequiredHook(t *testing.T) {
	whitelist := map[string]map[string]bool{
		"Index": {"Index": true, "Login": true},
		"Note":  {"ToPdf": true},
	}
	hook := LoginRequired(whitelist, func(c *Context) Result {
		if c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			return c.RenderJSON(struct {
				Ok  bool
				Msg string
			}{false, "NOTLOGIN"})
		}
		return c.Redirect("/login")
	})

	runHook := func(controller, action string, xhr bool, session map[string]string) (*httptest.ResponseRecorder, *Context) {
		req := httptest.NewRequest("GET", "/probe", nil)
		if xhr {
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
		}
		rec := httptest.NewRecorder()
		ctx := &Context{Request: req, Writer: newStatusWriter(rec), Session: session, Controller: controller, Action: action}
		result := hook(ctx)
		if result != nil {
			result.Apply(ctx.Writer, ctx.Request)
		}
		return rec, ctx
	}

	// Whitelisted actions pass without a session.
	rec, _ := runHook("Index", "Index", false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("whitelisted Index.Index status = %d", rec.Code)
	}
	rec, _ = runHook("Note", "ToPdf", false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("whitelisted Note.ToPdf status = %d", rec.Code)
	}
	// Non-whitelisted, anonymous XHR: NOTLOGIN envelope.
	rec, _ = runHook("Note", "Index", true, nil)
	if !strings.Contains(rec.Body.String(), "NOTLOGIN") {
		t.Fatalf("anonymous xhr body = %q", rec.Body.String())
	}
	// Non-whitelisted, anonymous plain: redirect to /login.
	rec, _ = runHook("Note", "Index", false, nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("anonymous redirect status = %d", rec.Code)
	}
	// Logged in: pass.
	rec, _ = runHook("Note", "Index", false, map[string]string{"UserId": "u1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("logged-in status = %d", rec.Code)
	}
}

func TestAppWritesSessionCookieBeforeActionResponse(t *testing.T) {
	cfg, err := ParseConfig([]byte("app.secret=session-test-secret\n"), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	app := &App{
		Routes:   CompileRoutes(mustParse(t, "GET /login Auth.Login")),
		Registry: NewRegistry(),
		Sessions: NewSessionCodec(cfg),
	}
	app.Registry.Register("Auth", "Login", nil, func(c *Context) Result {
		c.SetSession("UserId", "u1")
		return c.RenderText("logged in")
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "logged in" {
		t.Fatalf("login response = status %d body %q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("session cookie count = %d, want 1", len(cookies))
	}
	decoded, err := app.Sessions.Decode(cookies[0].Value)
	if err != nil || decoded["UserId"] != "u1" {
		t.Fatalf("session cookie = %#v, decode error = %v", decoded, err)
	}
}

func mustParse(t *testing.T, conf string) []Route {
	t.Helper()
	routes, err := ParseRoutes([]byte(conf))
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	return routes
}
