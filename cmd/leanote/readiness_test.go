package main

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/service"
)

func testReadiness(databaseReady bool, connect, load func() error) (*startupReadiness, *[]time.Duration) {
	var waits []time.Duration
	r := newStartupReadiness(databaseReady, connect, load, func(string, ...interface{}) {})
	r.initialBackoff = time.Second
	r.maxBackoff = 4 * time.Second
	r.wait = func(ctx context.Context, delay time.Duration) bool {
		waits = append(waits, delay)
		return ctx.Err() == nil
	}
	return r, &waits
}

func TestStartupReadinessReconnectsThenLoadsWithBackoff(t *testing.T) {
	connects, loads := 0, 0
	r, waits := testReadiness(false,
		func() error {
			connects++
			if connects < 3 {
				return errors.New("no reachable servers")
			}
			return nil
		},
		func() error {
			loads++
			if loads < 3 {
				return errors.New("configured admin user does not exist")
			}
			return nil
		},
	)
	if r.Ready() {
		t.Fatal("ready before any attempt")
	}
	if !r.run(context.Background()) || !r.Ready() {
		t.Fatal("run did not reach readiness")
	}
	if connects != 3 {
		t.Fatalf("connects = %d, want reconnect until success and not afterwards", connects)
	}
	if loads != 3 {
		t.Fatalf("snapshot loads = %d, want retries after connect", loads)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second, 4 * time.Second}
	if len(*waits) != len(want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
	for i := range want {
		if (*waits)[i] != want[i] {
			t.Fatalf("waits = %v, want capped exponential %v", *waits, want)
		}
	}
}

func TestStartupReadinessSkipsConnectWhenDatabaseAlreadyUp(t *testing.T) {
	r, _ := testReadiness(true,
		func() error { t.Fatal("connect called although the database is initialized"); return nil },
		func() error { return nil },
	)
	if err := r.attempt(); err != nil || !r.Ready() {
		t.Fatalf("attempt = %v ready=%t", err, r.Ready())
	}
	// Already ready: run returns immediately without waiting.
	if !r.run(context.Background()) {
		t.Fatal("run on a ready instance reported not ready")
	}
}

func TestStartupReadinessStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	loads := 0
	r, _ := testReadiness(true, nil, func() error {
		loads++
		cancel()
		return errors.New("still unavailable")
	})
	if r.run(ctx) || r.Ready() {
		t.Fatal("run reported ready after shutdown")
	}
	if loads != 1 {
		t.Fatalf("loads = %d, want the loop to stop after cancellation", loads)
	}
}

func TestWaitWithContextHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitWithContext(ctx, time.Hour) {
		t.Fatal("wait returned true after cancellation")
	}
	if !waitWithContext(context.Background(), time.Millisecond) {
		t.Fatal("wait returned false without cancellation")
	}
}

// TestStartupReadinessGatesApp wires the readiness flag into httpserver.App
// the way main does: not ready → 503 not_ready for actions and healthz with
// no action executed; after the snapshot loads, actions run.
func TestStartupReadinessGatesApp(t *testing.T) {
	loaded := false
	r, _ := testReadiness(true, nil, func() error {
		if !loaded {
			return errors.New("snapshot unavailable")
		}
		return nil
	})
	if err := r.attempt(); err == nil {
		t.Fatal("first attempt unexpectedly succeeded")
	}
	registry := httpserver.NewRegistry()
	actionRuns := 0
	registry.Register("Index", "Index", nil, func(c *httpserver.Context) httpserver.Result {
		actionRuns++
		return c.RenderText("ok")
	})
	routes, err := httpserver.ParseRoutes([]byte("GET / Index.Index\n"))
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	app := &httpserver.App{
		Routes:      httpserver.CompileRoutes(routes),
		Registry:    registry,
		HealthCheck: func() error { return nil },
		Ready:       r.Ready,
	}
	for _, path := range []string{"/", "/healthz"} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "{\"status\":\"not_ready\"}\n" {
			t.Fatalf("not-ready %s = %d %q", path, rec.Code, rec.Body.String())
		}
	}
	if actionRuns != 0 {
		t.Fatalf("action ran %d times on an unloaded snapshot", actionRuns)
	}

	loaded = true
	if !r.run(context.Background()) {
		t.Fatal("run did not reach readiness")
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" || actionRuns != 1 {
		t.Fatalf("ready action = %d %q runs=%d", rec.Code, rec.Body.String(), actionRuns)
	}
}

// The validated production configuration is the application-config seam
// source: adminUsername and site.url come from the [prod] section.
func TestProductionConfigSatisfiesAppConfigSeam(t *testing.T) {
	cfg, err := httpserver.ParseConfig([]byte("adminUsername=root\nsite.url=https://notes.example.test\n[prod]\nsite.url=https://prod.example.test # comment\n"), "prod")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	var source service.AppConfigSource = cfg
	if got, ok := source.String("adminUsername"); !ok || got != "root" {
		t.Fatalf("adminUsername = %q ok=%t", got, ok)
	}
	if got, ok := source.String("site.url"); !ok || got != "https://prod.example.test" {
		t.Fatalf("site.url = %q ok=%t", got, ok)
	}
	if _, ok := source.String("missing.key"); ok {
		t.Fatal("missing key reported present")
	}
}

// The new entry must never import the removed framework configuration package.
func TestEntryNeverReadsLegacyConfig(t *testing.T) {
	for _, file := range []string{"main.go", "readiness.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if importsFrameworkConfigPackage(t, file, data) {
			t.Fatalf("%s imports the removed framework configuration package", file)
		}
	}
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(data)
	seam := strings.Index(source, "service.SetAppConfigSource(cfg)")
	registry := strings.Index(source, "httpserver.NewRegistry()")
	if seam < 0 || registry < 0 || seam > registry {
		t.Fatalf("app-config seam must be installed from the validated config before registry wiring (seam=%d registry=%d)", seam, registry)
	}
	if !regexp.MustCompile(`Ready:\s+readiness\.Ready\b`).MatchString(source) {
		t.Fatal("main does not gate the App on startup readiness")
	}
}

func importsFrameworkConfigPackage(t *testing.T, filename string, data []byte) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, data, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	for _, spec := range file.Imports {
		pathValue, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("decode import in %s: %v", filename, err)
		}
		if path.Base(pathValue) == "config" {
			return true
		}
	}
	return false
}

// TestNotReadyServesPublicDataRootStatics runs the real conf/routes table and
// the production static handler while the snapshot is unloaded: /upload/* and
// /public/upload/* keep serving from the validated public data root, other
// static prefixes from the application tree, and every non-static request
// (including HEAD /healthz) answers 503 not_ready without touching the
// OnRequest hook or HealthCheck.
func TestNotReadyServesPublicDataRootStatics(t *testing.T) {
	routesData, err := os.ReadFile(filepath.Join("..", "..", "conf", "routes"))
	if err != nil {
		t.Fatalf("read conf/routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(routesData)
	if err != nil {
		t.Fatalf("ParseRoutes: %v", err)
	}
	appRoot, uploadRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(appRoot, "public", "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "public", "js", "app.js"), []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(uploadRoot, "u1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadRoot, "u1", "logo.txt"), []byte("upload"), 0o644); err != nil {
		t.Fatal(err)
	}
	hooks, checks := 0, 0
	r, _ := testReadiness(true, nil, func() error { return errors.New("snapshot unavailable") })
	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: httpserver.NewRegistry(),
		StaticHandler: func(base string) http.Handler {
			return staticHandlerWithContent(appRoot, base, uploadRoot)
		},
		OnRequest:   func() { hooks++ },
		HealthCheck: func() error { checks++; return nil },
		Ready:       r.Ready,
	}
	for _, tc := range []struct{ path, body string }{
		{"/upload/u1/logo.txt", "upload"},
		{"/public/upload/u1/logo.txt", "upload"},
		{"/js/app.js", "asset"},
		{"/public/js/app.js", "asset"},
	} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
			t.Fatalf("not-ready static %s = %d %q, want 200 %q", tc.path, rec.Code, rec.Body.String(), tc.body)
		}
	}
	head := httptest.NewRecorder()
	app.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/upload/u1/logo.txt", nil))
	if head.Code != http.StatusOK {
		t.Fatalf("not-ready HEAD static = %d, want 200", head.Code)
	}
	for _, probe := range []struct{ method, path string }{
		{http.MethodHead, "/healthz"},
		{http.MethodGet, "/login"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/upload/u1/logo.txt"},
	} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(probe.method, probe.path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("not-ready %s %s = %d, want 503", probe.method, probe.path, rec.Code)
		}
		if probe.method != http.MethodHead && rec.Body.String() != "{\"status\":\"not_ready\"}\n" {
			t.Fatalf("not-ready %s %s body = %q", probe.method, probe.path, rec.Body.String())
		}
	}
	if hooks != 0 || checks != 0 {
		t.Fatalf("not-ready ran onRequest=%d healthCheck=%d, want none", hooks, checks)
	}
}

// Retries log one stable notice per pending stage (mongo, then snapshot)
// plus the success line, never one line per retry or the raw error text.
func TestStartupReadinessLogsOncePerPendingStage(t *testing.T) {
	connects, loads := 0, 0
	r, _ := testReadiness(false,
		func() error {
			connects++
			if connects < 3 {
				return errors.New("server selection error: 10.0.0.5:27017")
			}
			return nil
		},
		func() error {
			loads++
			if loads < 3 {
				return errors.New("configured admin user does not exist")
			}
			return nil
		},
	)
	var lines []string
	r.logf = func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	if !r.run(context.Background()) {
		t.Fatal("run did not reach readiness")
	}
	if len(lines) != 3 ||
		!strings.Contains(lines[0], "mongo unavailable") ||
		!strings.Contains(lines[1], "global configuration unavailable") ||
		!strings.Contains(lines[2], "readiness reached") {
		t.Fatalf("log lines = %q, want one per stage plus success", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "10.0.0.5") || strings.Contains(line, "admin user") {
			t.Fatalf("log line leaks error detail: %q", line)
		}
	}
}
