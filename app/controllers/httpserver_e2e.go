package controllers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"os"
	"time"

	"github.com/yangphere/leanote/app/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	e2eRunKind         = "browser-e2e"
	e2eRunTokenEnv     = "LEANOTE_E2E_RUN_TOKEN"
	e2eRunMarkerMaxAge = 2 * time.Hour
)

type e2eRunMarker struct {
	RunId       string    `bson:"runId"`
	Kind        string    `bson:"kind"`
	TokenSha256 string    `bson:"tokenSha256"`
	CreatedAt   time.Time `bson:"createdAt"`
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
	if marker.Kind != e2eRunKind || marker.CreatedAt.IsZero() || now.Sub(marker.CreatedAt) > e2eRunMarkerMaxAge || now.Before(marker.CreatedAt.Add(-time.Minute)) {
		return 503
	}
	if subtle.ConstantTimeCompare([]byte(marker.TokenSha256), []byte(e2eTokenDigest(token))) != 1 {
		return 503
	}
	return 200
}

func e2eIdentityResponseToken() string { return os.Getenv(e2eRunTokenEnv) }
