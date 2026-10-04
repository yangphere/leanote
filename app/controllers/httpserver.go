package controllers

import (
	"net"
	"net/http"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
)

// RegisterHTTP wires first-party controller actions into the
// httpserver registry. Production callers pass the validated config so
// sensitive actions can consume the canonical runtime values.
func RegisterHTTP(rs *httpserver.Registry, runMode string, cfg *httpserver.Config) {
	e2e := &TestE2eServer{RunMode: runMode}
	e2e.Register(rs)
	RegisterMainHTTP(rs)
	RegisterNotesHTTP(rs, runMode)
	RegisterPublishingHTTP(rs)
	if cfg != nil {
		NewNotePDFServer(cfg).Register(rs)
	}
}

// WebSessionBefore is the shared session-to-principal boundary for adapters
// hosted outside this package (member/admin). The implementation remains
// centralized so all web surfaces apply the same invalid-session behavior.
func WebSessionBefore(c *httpserver.Context) httpserver.Result {
	return webSessionBefore(c)
}

// pageParam preserves the legacy BaseController.GetPage contract: omitted,
// malformed, zero and negative values all start at page one.
func pageParam(c *httpserver.Context) int {
	page := c.Params.Int("page", 1)
	if page < 1 {
		return 1
	}
	return page
}

// TestE2eServer is the first-party host for the test-mode-only E2E identity
// endpoint (conf/routes: GET /_test/e2e/identity). The decision core is
// shared with the previous controller path; only the HTTP plumbing differs.
type TestE2eServer struct {
	RunMode string
}

// Register installs the identity action.
func (s *TestE2eServer) Register(rs *httpserver.Registry) {
	rs.Register("TestE2e", "Identity", nil, s.identity)
}

func (s *TestE2eServer) identity(c *httpserver.Context) httpserver.Result {
	if s.RunMode != "test" {
		return c.NotFound("")
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil || !isLoopbackHost(host) {
		return c.NotFound("")
	}

	databaseName := db.DatabaseName()
	token := e2eIdentityResponseToken()

	status := evaluateE2eIdentity(s.RunMode, host, databaseName, token, loadE2eRunMarkers(databaseName), time.Now())
	switch status {
	case http.StatusOK:
		return c.RenderJSON(map[string]string{
			"runToken": token,
			"database": databaseName,
		})
	case http.StatusServiceUnavailable:
		return httpserver.JSONResult(http.StatusServiceUnavailable, nil)
	default:
		return c.NotFound("")
	}
}
