package controllers

import (
	"testing"
	"time"
)

var e2eIdentityNow = time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)

func e2eIdentityMarker(createdAt time.Time, tokenHash, kind string) e2eRunMarker {
	return e2eRunMarker{RunId: "run-1", Kind: kind, TokenSha256: tokenHash, CreatedAt: createdAt}
}

func TestEvaluateE2eIdentityFailClosed(t *testing.T) {
	token := "run-token"
	valid := e2eTokenDigest(token)
	fresh := e2eIdentityNow.Add(-time.Minute)

	tests := []struct {
		name    string
		status  int
		runMode string
		host    string
		dbName  string
		token   string
		markers []e2eRunMarker
	}{
		{name: "happy path", status: 200, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "ipv6 loopback", status: 200, runMode: "test", host: "::1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "non-test mode is 404", status: 404, runMode: "prod", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "non-loopback is 404", status: 404, runMode: "test", host: "10.0.0.8", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "wrong database is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "missing process token is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: "", markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "missing marker is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: nil},
		{name: "duplicate markers are 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "browser-e2e"), e2eIdentityMarker(fresh, valid, "browser-e2e")}},
		{name: "expired marker is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(e2eIdentityNow.Add(-e2eRunMarkerMaxAge-time.Second), valid, "browser-e2e")}},
		{name: "future marker is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(e2eIdentityNow.Add(10*time.Minute), valid, "browser-e2e")}},
		{name: "zero created time is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(time.Time{}, valid, "browser-e2e")}},
		{name: "digest mismatch is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, e2eTokenDigest("other-token"), "browser-e2e")}},
		{name: "wrong run kind is 503", status: 503, runMode: "test", host: "127.0.0.1", dbName: "leanote_test", token: token, markers: []e2eRunMarker{e2eIdentityMarker(fresh, valid, "other-kind")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := evaluateE2eIdentity(test.runMode, test.host, test.dbName, test.token, test.markers, e2eIdentityNow)
			if status != test.status {
				t.Fatalf("evaluateE2eIdentity() = %d, want %d", status, test.status)
			}
		})
	}
}

func TestE2eTokenDigestIsDeterministicSha256Hex(t *testing.T) {
	first := e2eTokenDigest("run-token")
	second := e2eTokenDigest("run-token")
	if first != second {
		t.Fatalf("digest is not deterministic")
	}
	if len(first) != 64 {
		t.Fatalf("digest length = %d, want 64", len(first))
	}
	if first == e2eTokenDigest("other-token") {
		t.Fatalf("digest does not depend on the token")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1":        true,
		"::1":              true,
		"127.5.4.3":        true,
		"::ffff:127.0.0.1": true,
		"10.1.2.3":         false,
		"localhost":        false,
		"":                 false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Fatalf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
