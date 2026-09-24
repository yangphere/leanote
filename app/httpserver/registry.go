package httpserver

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	i18n "github.com/yangphere/leanote/app/lea/i18n"
)

// RouteTable is the priority-ordered route list compiled from conf/routes.
// Matching is first-match-wins over the file order — not most-specific —
// because conf/routes documents that ordering as its contract.
type RouteTable struct {
	routes []Route
}

func CompileRoutes(routes []Route) *RouteTable {
	compiled := make([]Route, len(routes))
	copy(compiled, routes)
	for i := range compiled {
		if compiled[i].segments == nil {
			compiled[i].segments = compileSegments(compiled[i].Path)
		}
	}
	return &RouteTable{routes: compiled}
}

// Match finds the first route that accepts method+path. HEAD is served by
// GET routes. Returns the extracted path params on success.
func (t *RouteTable) Match(method, path string) (*Route, map[string]string, bool) {
	lookupMethod := strings.ToUpper(method)
	if lookupMethod == http.MethodHead {
		lookupMethod = http.MethodGet
	}
	for i := range t.routes {
		route := &t.routes[i]
		if !routeMethodMatches(route.Method, lookupMethod) {
			continue
		}
		if params, ok := matchSegments(route.segments, path); ok {
			return route, params, true
		}
	}
	return nil, nil, false
}

func routeMethodMatches(routeMethod, lookupMethod string) bool {
	if routeMethod == "*" {
		return true
	}
	return routeMethod == lookupMethod
}

func matchSegments(segs []routeSegment, path string) (map[string]string, bool) {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 && parts[0] == "" {
		parts = nil
	}
	params := map[string]string{}
	for i, seg := range segs {
		switch seg.kind {
		case segLiteral:
			if i >= len(parts) || parts[i] != seg.text {
				return nil, false
			}
		case segParam:
			if i >= len(parts) || parts[i] == "" {
				return nil, false
			}
			params[seg.text] = parts[i]
		case segRest:
			if i >= len(parts) {
				return nil, false
			}
			params[seg.text] = strings.Join(parts[i:], "/")
			return params, true
		}
	}
	if len(parts) != len(segs) {
		return nil, false
	}
	return params, true
}

// BeforeFunc runs before the action; a non-nil return short-circuits the
// action (auth redirects, NOTLOGIN envelopes).
type BeforeFunc func(c *Context) Result

// ActionFunc is a registered controller action.
type ActionFunc func(c *Context) Result

// ActionEntry is one registered controller action with its before hooks.
// Registration is explicit source code — an action that is not registered
// cannot be reached by any URL (no reflection over exported methods).
type ActionEntry struct {
	Controller string
	Name       string
	Befores    []BeforeFunc
	Handler    ActionFunc
}

// Registry holds every reachable action.
type Registry struct {
	actions map[string]*ActionEntry
}

func NewRegistry() *Registry {
	return &Registry{actions: map[string]*ActionEntry{}}
}

func titleFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// Register makes an action reachable under "Controller.Method" (the exact
// name conf/routes dispatches to) and records its controller-level before
// hooks.
func (r *Registry) Register(controller, method string, befores []BeforeFunc, handler ActionFunc) {
	name := titleFirst(controller) + "." + titleFirst(method)
	r.actions[name] = &ActionEntry{Controller: titleFirst(controller), Name: method, Befores: befores, Handler: handler}
}

func (r *Registry) Lookup(controller, method string) (*ActionEntry, bool) {
	entry, ok := r.actions[titleFirst(controller)+"."+titleFirst(method)]
	return entry, ok
}

// Registered returns all action names, sorted (for tests and startup logs).
func (r *Registry) Registered() []string {
	names := make([]string, 0, len(r.actions))
	for name := range r.actions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Context is the first-party controller context replacing
// *revel.Controller: request/params/session/locale plus the render helpers
// controllers use. It is handed to actions and before hooks.
type Context struct {
	Request   *http.Request
	Writer    *statusWriter
	Params    *Params
	Session   map[string]string
	SessionID string
	Locale    string
	// Controller/Action are the canonical dispatched names.
	Controller      string
	Action          string
	Principal       Principal
	SessionReader   SessionReader
	SessionWriter   SessionWriter
	PrincipalPolicy PrincipalPolicy

	// sessionDirty collects SetSession/DeleteSession calls; the dispatcher
	// persists them into the session cookie after the action.
	sessionDirty map[string]*string

	result        Result
	sessionCommit func() error
}

type sessionCommitFailure struct {
	OK  bool   `json:"Ok"`
	Msg string `json:"Msg"`
}

func (c *Context) Get(key string) (string, bool, error) {
	if c.SessionReader != nil && c.SessionReader != c {
		return c.SessionReader.Get(key)
	}
	value, ok := c.Session[key]
	return value, ok, nil
}

func (c *Context) Set(key, value string) error {
	if c.SessionWriter != nil && c.SessionWriter != c {
		return c.SessionWriter.Set(key, value)
	}
	if c.Session == nil {
		c.Session = map[string]string{}
	}
	if c.sessionDirty == nil {
		c.sessionDirty = map[string]*string{}
	}
	c.Session[key] = value
	valueCopy := value
	c.sessionDirty[key] = &valueCopy
	return nil
}

func (c *Context) Delete(key string) error {
	if c.SessionWriter != nil && c.SessionWriter != c {
		return c.SessionWriter.Delete(key)
	}
	if c.Session == nil {
		c.Session = map[string]string{}
	}
	if c.sessionDirty == nil {
		c.sessionDirty = map[string]*string{}
	}
	delete(c.Session, key)
	c.sessionDirty[key] = nil
	return nil
}

func (c *Context) Commit() error {
	if c.SessionWriter != nil && c.SessionWriter != c {
		return c.SessionWriter.Commit()
	}
	if c.sessionCommit == nil {
		return nil
	}
	return c.sessionCommit()
}

func (c *Context) SetPrincipal(principal Principal) { c.Principal = principal }

func (c *Context) SetAuthenticatedPrincipal(userID string, source PrincipalSource, tokenState TokenState) error {
	principal, err := AuthenticatedPrincipalWithPolicy(userID, source, tokenState, c.PrincipalPolicy)
	if err != nil {
		c.SetPrincipal(AnonymousPrincipal())
		return err
	}
	c.SetPrincipal(principal)
	return nil
}

func (c *Context) GetPrincipal() Principal {
	if c.Principal.Role == "" {
		return AnonymousPrincipal()
	}
	return c.Principal
}

// SetSession writes a session key; the cookie is refreshed after the action.
func (c *Context) SetSession(key, value string) error {
	return c.Set(key, value)
}

// DeleteSession removes a session key; the cookie is refreshed after the
// action.
func (c *Context) DeleteSession(key string) error {
	return c.Delete(key)
}

// Render helpers return Results that the dispatcher applies.
func (c *Context) RenderJSON(v interface{}) Result             { return JSONResult(http.StatusOK, v) }
func (c *Context) RenderJSONP(cb string, v interface{}) Result { return JSONPResult(cb, v) }
func (c *Context) RenderText(s string) Result                  { return TextResult(http.StatusOK, s) }
func (c *Context) RenderHTML(s string) Result                  { return HTMLResult(http.StatusOK, s) }
func (c *Context) Redirect(url string) Result                  { return RedirectResult(url) }
func (c *Context) RenderTemplate(name string, args map[string]interface{}) Result {
	return TemplateResult(http.StatusOK, name, args)
}

// Message resolves an i18n message for the request locale.
func (c *Context) Message(message string, args ...interface{}) string {
	return i18n.Message(c.Locale, message, args...)
}

// NotFound renders a 404, preferring the errors/404 page when a template
// renderer is installed.
func (c *Context) NotFound(msg string) Result {
	if body, err := TemplateRenderer("errors/404.html", nil); err == nil {
		return HTMLResult(http.StatusNotFound, string(body))
	}
	return TextResult(http.StatusNotFound, msg)
}

// App assembles the route table, action registry and session codec into the
// root http.Handler, replicating the revel filter chain order:
// recover → route/rewrite → session → i18n/locale → interceptors → action,
// with gzip around the response (CompressFilter).
type App struct {
	Routes               *RouteTable
	Registry             *Registry
	Sessions             *SessionCodec // optional; nil = every request anonymous
	SessionReaderFactory func(map[string]string) SessionReader
	SessionWriterFactory func(*Context) SessionWriter
	PrincipalPolicy      PrincipalPolicy
	LocaleResolver       func(r *http.Request) string
	OnRequest            func() // pre-dispatch hook, e.g. db.CheckMongoSessionLost
	// HealthCheck is an optional unauthenticated readiness probe. When set,
	// GET /healthz is handled before route matching and returns the fixed JSON
	// contract without exposing application state.
	HealthCheck   func() error
	StaticHandler func(base string) http.Handler // serves a Static.Serve base dir
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	next := http.HandlerFunc(a.dispatch)
	Gzip(Recover(nil)(next)).ServeHTTP(w, r)
}

func (a *App) dispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" && a.HealthCheck != nil {
		status := http.StatusOK
		body := map[string]string{"status": "ready"}
		if err := a.HealthCheck(); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "not_ready"
		}
		ctx := &Context{Request: r, Writer: newStatusWriter(w)}
		ApplyResult(ctx, JSONLineResult(status, body))
		return
	}
	if a.OnRequest != nil {
		a.OnRequest()
	}

	sw := newStatusWriter(w)
	route, pathParams, matched := a.Routes.Match(r.Method, r.URL.Path)
	if !matched {
		NotFoundText(sw, "No matching route found: "+r.URL.Path)
		return
	}

	if route.IsStatic {
		if a.StaticHandler == nil {
			NotFoundText(sw, "(intentionally)")
			return
		}
		prefix := "/public"
		if i := strings.Index(route.Path, "/*"); i > 0 {
			prefix = route.Path[:i]
		}
		http.StripPrefix(prefix, a.StaticHandler(route.StaticBase)).ServeHTTP(sw, r)
		return
	}

	controller, action := route.Controller, route.MethodName
	if route.IsCatchAll {
		// Port of RouterFilter's prefix rewrite: /api/note/get dispatches to
		// ApiNote.Get, /member/group/addGroup to MemberGroup.AddGroup.
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) < 2 {
			NotFoundText(sw, "No matching route found: "+r.URL.Path)
			return
		}
		switch {
		case strings.HasPrefix(route.Path, "/api/"):
			if len(parts) < 3 {
				NotFoundText(sw, "No matching route found: "+r.URL.Path)
				return
			}
			controller, action = "Api"+titleFirst(parts[1]), parts[2]
		case strings.HasPrefix(route.Path, "/member/"):
			if len(parts) < 3 {
				NotFoundText(sw, "No matching route found: "+r.URL.Path)
				return
			}
			controller, action = "Member"+titleFirst(parts[1]), parts[2]
		default:
			controller, action = titleFirst(parts[0]), parts[1]
		}
	}

	entry, ok := a.Registry.Lookup(controller, action)
	if !ok {
		ctx := &Context{Request: r, Writer: sw, Controller: titleFirst(controller), Action: action}
		ApplyResult(ctx, ctx.NotFound("No matching action: "+controller+"."+action))
		return
	}

	session, sessionID := map[string]string{}, ""
	if a.Sessions != nil {
		if cookie, err := r.Cookie(a.Sessions.cookieName()); err == nil {
			if keys, derr := a.Sessions.Decode(cookie.Value); derr == nil {
				session = keys
				sessionID = cookie.Value
			} else {
				// 旧/损坏 Cookie 一律匿名（安全级日志由调用方注入）。
				_ = derr
			}
		}
	}
	var locale string
	if a.LocaleResolver != nil {
		locale = a.LocaleResolver(r)
	}

	ctx := &Context{
		Request: r, Writer: sw,
		Params:  newParams(r, pathParams),
		Session: session, SessionID: sessionID,
		Locale:     locale,
		Controller: titleFirst(controller), Action: titleFirst(action),
		PrincipalPolicy: a.PrincipalPolicy,
	}
	if a.SessionReaderFactory != nil {
		ctx.SessionReader = a.SessionReaderFactory(session)
	}
	if a.SessionWriterFactory != nil {
		ctx.SessionWriter = a.SessionWriterFactory(ctx)
	}
	ctx.sessionCommit = func() error { return a.applySessionCookie(ctx) }
	for _, before := range entry.Befores {
		if result := before(ctx); result != nil {
			if err := ctx.Commit(); err != nil {
				applySessionCommitFailure(ctx, result)
				return
			}
			ApplyResult(ctx, result)
			return
		}
	}
	// SessionFilter must emit the refreshed cookie before the action result
	// writes response headers. Applying it afterward loses Set-Cookie because
	// net/http has already committed the response.
	result := entry.Handler(ctx)
	if err := ctx.Commit(); err != nil {
		applySessionCommitFailure(ctx, result)
		return
	}
	ApplyResult(ctx, result)
}

func applySessionCommitFailure(ctx *Context, result Result) {
	if result != nil && resultFailed(result) {
		ApplyResult(ctx, result)
		return
	}
	ApplyResult(ctx, ctx.RenderJSON(sessionCommitFailure{Msg: "session_commit"}))
}

// applySessionCookie refreshes the session cookie when the action wrote
// session keys (revel SessionFilter behaviour).
func (a *App) applySessionCookie(ctx *Context) error {
	if a.Sessions == nil || len(ctx.sessionDirty) == 0 {
		return nil
	}
	for key, value := range ctx.sessionDirty {
		if value == nil {
			delete(ctx.Session, key)
		} else {
			ctx.Session[key] = *value
		}
	}
	cookie, err := a.Sessions.Encode(ctx.Session)
	if err != nil {
		return fmt.Errorf("commit session cookie: %w", err)
	}
	ctx.Writer.Header().Add("Set-Cookie", cookie.String())
	return nil
}

// ApplyResult funnels every response through the status writer so the
// "status written once" invariant holds for all result types.
func ApplyResult(ctx *Context, result Result) {
	result.Apply(ctx.Writer, ctx.Request)
}

// NotFoundText writes a plain 404 (used before a Context exists).
func NotFoundText(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(msg))
}
