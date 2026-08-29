package controllers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Test-mode-only E2E identity contract (jquery-upgrade PRD R-jQ3).
//
// GET /_test/e2e/identity is served only when every guard holds:
//   - the app runs in Revel "test" run mode;
//   - the request arrives from a loopback address;
//   - the harness wrote exactly one non-expired e2e_runs marker whose
//     token digest matches the run token injected into this process.
//
// Any other combination must fail closed: 404 for wrong mode/host,
// 503 for marker, database or digest failures. The handler never
// creates, refreshes or deletes markers, never logs the raw token and
// never leaks marker content or connection details.
const (
	e2eRunKind         = "browser-e2e"
	e2eRunTokenEnv     = "LEANOTE_E2E_RUN_TOKEN"
	e2eRunMarkerMaxAge = 2 * time.Hour
)

type TestE2e struct {
	*revel.Controller
}

// e2eRunMarker mirrors the document the harness writes into e2e_runs.
type e2eRunMarker struct {
	RunId       string    `bson:"runId"`
	Kind        string    `bson:"kind"`
	TokenSha256 string    `bson:"tokenSha256"`
	CreatedAt   time.Time `bson:"createdAt"`
}

func (c TestE2e) Identity() revel.Result {
	if revel.RunMode != "test" {
		return c.NotFound("")
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil || !isLoopbackHost(host) {
		return c.NotFound("")
	}

	databaseName := db.DatabaseName()

	status := evaluateE2eIdentity(revel.RunMode, host, databaseName, os.Getenv(e2eRunTokenEnv), loadE2eRunMarkers(databaseName), time.Now())
	if status == http.StatusNotFound {
		return c.NotFound("")
	}
	if status != http.StatusOK {
		c.Response.Status = http.StatusServiceUnavailable
		return c.RenderJSON(nil)
	}
	return c.RenderJSON(map[string]string{
		"runToken": os.Getenv(e2eRunTokenEnv),
		"database": databaseName,
	})
}

func loadE2eRunMarkers(databaseName string) []e2eRunMarker {
	if databaseName == "" {
		return nil
	}
	markers := []e2eRunMarker{}
	if err := db.FindInCollection(databaseName, "e2e_runs", bson.M{"kind": e2eRunKind}, &markers); err != nil {
		return nil
	}
	return markers
}

// isLoopbackHost reports whether host is an IPv4/IPv6 loopback literal.
func isLoopbackHost(host string) bool {
	if host == "127.0.0.1" || host == "::1" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func e2eTokenDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// evaluateE2eIdentity is the pure decision core of the identity endpoint.
// It returns the HTTP status the endpoint must answer with; 200 means the
// caller may reveal {runToken, databaseName}.
func evaluateE2eIdentity(runMode, host, databaseName, token string, markers []e2eRunMarker, now time.Time) int {
	if runMode != "test" || !isLoopbackHost(host) {
		return 404
	}
	if databaseName != "leanote_test" || token == "" {
		return 503
	}
	if len(markers) != 1 {
		return 503
	}
	marker := markers[0]
	if marker.Kind != e2eRunKind {
		return 503
	}
	if marker.CreatedAt.IsZero() || now.Sub(marker.CreatedAt) > e2eRunMarkerMaxAge || now.Before(marker.CreatedAt.Add(-time.Minute)) {
		return 503
	}
	if subtle.ConstantTimeCompare([]byte(marker.TokenSha256), []byte(e2eTokenDigest(token))) != 1 {
		return 503
	}
	return 200
}
