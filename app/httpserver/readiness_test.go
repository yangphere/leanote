package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

const notReadyLine = "{\"status\":\"not_ready\"}\n"

// readinessGateApp wires one static route, one explicit action route and the
// catch-all so the D-H9 gate is exercised across every dispatch branch.
func readinessGateApp(t *testing.T, ready *atomic.Bool, actionRuns, hookRuns, healthRuns *int32) *App {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("seed static asset: %v", err)
	}
	registry := NewRegistry()
	registry.Register("Index", "Index", nil, func(c *Context) Result {
		atomic.AddInt32(actionRuns, 1)
		return c.RenderText("action")
	})
	registry.Register("Probe", "Run", nil, func(c *Context) Result {
		atomic.AddInt32(actionRuns, 1)
		return c.RenderText("probe")
	})
	return &App{
		Routes: CompileRoutes(mustParse(t, "GET /js/*filepath Static.Serve(\"public/js\")\n"+
			"GET / Index.Index\n"+
			"* /:controller/:action :controller.:action\n")),
		Registry: registry,
		StaticHandler: func(string) http.Handler {
			return http.FileServer(http.Dir(root))
		},
		OnRequest: func() { atomic.AddInt32(hookRuns, 1) },
		HealthCheck: func() error {
			atomic.AddInt32(healthRuns, 1)
			return nil
		},
		Ready: ready.Load,
	}
}

func serve(app *App, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func TestReadinessGateBlocksActionsUntilReady(t *testing.T) {
	var ready atomic.Bool
	var actionRuns, hookRuns, healthRuns int32
	app := readinessGateApp(t, &ready, &actionRuns, &hookRuns, &healthRuns)

	health := serve(app, http.MethodGet, "/healthz", nil)
	if health.Code != http.StatusServiceUnavailable || health.Body.String() != notReadyLine {
		t.Fatalf("not-ready healthz = %d %q, want 503 %q", health.Code, health.Body.String(), notReadyLine)
	}
	if got := health.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("not-ready healthz content type = %q", got)
	}

	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, "/"},
		{http.MethodPost, "/probe/run"},
		{http.MethodGet, "/no/such/route/here"},
		{http.MethodPost, "/healthz"},
	} {
		rec := serve(app, probe.method, probe.path, nil)
		if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != notReadyLine {
			t.Fatalf("not-ready %s %s = %d %q, want 503 %q", probe.method, probe.path, rec.Code, rec.Body.String(), notReadyLine)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("not-ready %s %s content type = %q", probe.method, probe.path, got)
		}
		if cookies := rec.Header().Values("Set-Cookie"); len(cookies) != 0 {
			t.Fatalf("not-ready %s %s wrote cookies %v", probe.method, probe.path, cookies)
		}
	}

	static := serve(app, http.MethodGet, "/js/app.js", nil)
	if static.Code != http.StatusOK || static.Body.String() != "console.log(1)" {
		t.Fatalf("not-ready static = %d %q, want asset", static.Code, static.Body.String())
	}
	if actionRuns != 0 || hookRuns != 0 || healthRuns != 0 {
		t.Fatalf("not-ready ran action=%d onRequest=%d healthCheck=%d, want none", actionRuns, hookRuns, healthRuns)
	}

	ready.Store(true)
	health = serve(app, http.MethodGet, "/healthz", nil)
	if health.Code != http.StatusOK || health.Body.String() != "{\"status\":\"ready\"}\n" || healthRuns != 1 {
		t.Fatalf("ready healthz = %d %q (checks=%d), want HealthCheck-backed 200", health.Code, health.Body.String(), healthRuns)
	}
	for _, probe := range []struct{ method, path, body string }{
		{http.MethodGet, "/", "action"},
		{http.MethodPost, "/probe/run", "probe"},
	} {
		rec := serve(app, probe.method, probe.path, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != probe.body {
			t.Fatalf("ready %s %s = %d %q, want 200 %q", probe.method, probe.path, rec.Code, rec.Body.String(), probe.body)
		}
	}
	if unrouted := serve(app, http.MethodGet, "/no/such/route/here", nil); unrouted.Code != http.StatusNotFound {
		t.Fatalf("ready unrouted status = %d, want 404", unrouted.Code)
	}
	if actionRuns != 2 || hookRuns != 3 {
		t.Fatalf("ready ran action=%d onRequest=%d, want 2 and 3", actionRuns, hookRuns)
	}
}

func TestReadinessGateKeepsHealthCheckFailureAfterReady(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	app := &App{Ready: ready.Load, HealthCheck: func() error { return errHealthTest }}
	rec := serve(app, http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != notReadyLine {
		t.Fatalf("ready but unhealthy = %d %q, want 503 not_ready", rec.Code, rec.Body.String())
	}
}

func TestReadinessGateNotReadyBodyIsFixedUnderGzipNegotiation(t *testing.T) {
	var ready atomic.Bool
	var actionRuns, hookRuns, healthRuns int32
	app := readinessGateApp(t, &ready, &actionRuns, &hookRuns, &healthRuns)
	rec := serve(app, http.MethodGet, "/healthz", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != notReadyLine || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("gzip healthz = %d %q encoding=%q, want plain not_ready line", rec.Code, rec.Body.String(), rec.Header().Get("Content-Encoding"))
	}
}

func TestReadinessGateAbsentKeepsExistingDispatch(t *testing.T) {
	app := &App{
		Routes:   CompileRoutes(mustParse(t, "GET / Index.Index\n")),
		Registry: NewRegistry(),
	}
	app.Registry.Register("Index", "Index", nil, func(c *Context) Result { return c.RenderText("ok") })
	if rec := serve(app, http.MethodGet, "/", nil); rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("ungated action = %d %q", rec.Code, rec.Body.String())
	}
	// Without HealthCheck or Ready, /healthz is an ordinary (unrouted) path.
	if rec := serve(app, http.MethodGet, "/healthz", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("ungated healthz status = %d, want 404", rec.Code)
	}
}
