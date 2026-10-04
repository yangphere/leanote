package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

var errHealthTest = errors.New("not ready")

func TestHealthCheckUsesFixedJSONLineContract(t *testing.T) {
	app := &App{HealthCheck: func() error { return nil }}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if got, want := res.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Fatalf("content type = %q, want %q", got, want)
	}
	if got, want := res.Body.String(), "{\"status\":\"ready\"}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHealthCheckReturnsNotReady(t *testing.T) {
	app := &App{HealthCheck: func() error { return errHealthTest }}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || res.Body.String() != "{\"status\":\"not_ready\"}\n" {
		t.Fatalf("response = %d %q, want 503 not_ready", res.Code, res.Body.String())
	}
}

func TestHealthCheckKeepsFixedPayloadWhenGzipIsAccepted(t *testing.T) {
	app := &App{HealthCheck: func() error { return nil }}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
	if got := res.Body.String(); got != "{\"status\":\"ready\"}\n" {
		t.Fatalf("body = %q, want fixed JSON line", got)
	}
	if got := res.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("content encoding = %q, want empty", got)
	}
}
