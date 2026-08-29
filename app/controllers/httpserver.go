package controllers

import (
	"net"
	"net/http"
	"os"
	"time"

	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
)

// RegisterHTTP wires every first-party (post-Revel) controller action into
// the httpserver registry. cmd/leanote calls this at startup; Task 4 adds
// the remaining controllers here as they migrate.
func RegisterHTTP(rs *httpserver.Registry, runMode string) {
	e2e := &TestE2eServer{RunMode: runMode}
	e2e.Register(rs)
}

// TestE2eServer is the first-party host for the test-mode-only E2E identity
// endpoint (conf/routes: GET /_test/e2e/identity). The decision core is
// shared with the legacy Revel path; only the HTTP plumbing differs.
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
	token := os.Getenv(e2eRunTokenEnv)

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
