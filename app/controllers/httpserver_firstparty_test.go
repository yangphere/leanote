package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
)

// firstPartyApp builds the post-Revel server over the REAL conf/routes with
// only the first-party actions registered — everything still on Revel 404s
// until its migration lands.
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
	RegisterHTTP(registry, runMode)
	return &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: registry,
	}
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
