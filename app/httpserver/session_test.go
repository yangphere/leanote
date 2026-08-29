package httpserver

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func newTestCodec(t *testing.T) *SessionCodec {
	t.Helper()
	return &SessionCodec{
		Secret: []byte("test-secret"),
		Prefix: "LEANOTE",
		TTL:    3 * time.Hour,
		NowFunc: func() time.Time {
			return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
		},
	}
}

func TestSessionEncodeDecodeRoundTrip(t *testing.T) {
	codec := newTestCodec(t)
	keys := map[string]string{"UserId": "abc123", "Email": "a@b.c", "Username": "admin"}
	cookie, err := codec.Encode(keys)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := codec.Decode(cookie.Value)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for k, v := range keys {
		if got[k] != v {
			t.Fatalf("key %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	codec := newTestCodec(t)
	cookie, err := codec.Encode(map[string]string{"UserId": "abc"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if cookie.Name != "LEANOTE_SESSION" {
		t.Fatalf("cookie name = %q, want prefix+_SESSION", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly must default to true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Secure {
		t.Fatal("Secure must follow config (absent here = false)")
	}
	if cookie.Path != "/" {
		t.Fatalf("Path = %q, want /", cookie.Path)
	}
	wantExpiry := codec.now().Add(3 * time.Hour)
	if !cookie.Expires.Equal(wantExpiry) {
		t.Fatalf("Expires = %v, want %v (session.expires)", cookie.Expires, wantExpiry)
	}
	if cookie.MaxAge != int(3*time.Hour/time.Second) {
		t.Fatalf("MaxAge = %d", cookie.MaxAge)
	}
}

func TestSessionRejectsTamperedPayload(t *testing.T) {
	codec := newTestCodec(t)
	cookie, _ := codec.Encode(map[string]string{"UserId": "abc"})
	body, sig, _ := strings.Cut(cookie.Value, ".")
	tampered := flipLastChar(body) + "." + sig
	if _, err := codec.Decode(tampered); err == nil {
		t.Fatal("tampered payload must be rejected")
	}
}

func TestSessionRejectsForgedSignature(t *testing.T) {
	codec := newTestCodec(t)
	cookie, _ := codec.Encode(map[string]string{"UserId": "abc"})
	body, _, _ := strings.Cut(cookie.Value, ".")
	forged := body + "." + flipLastChar(codec.sign(body))
	if _, err := codec.Decode(forged); err == nil {
		t.Fatal("forged signature must be rejected")
	}
}

func TestSessionRejectsExpired(t *testing.T) {
	codec := newTestCodec(t)
	cookie, _ := codec.Encode(map[string]string{"UserId": "abc"})
	later := newTestCodec(t)
	later.NowFunc = func() time.Time {
		return codec.now().Add(3*time.Hour + time.Minute)
	}
	if _, err := later.Decode(cookie.Value); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired cookie err = %v, want expired", err)
	}
}

func TestSessionRejectsLegacyRevelCookie(t *testing.T) {
	codec := newTestCodec(t)
	// A legacy Revel session cookie (base64 gob blob) must not decode.
	legacy := "DACTYlTDjJiX5xI9AhUJFAZ1c2VySWQYAiQBSGVtYWlsGgIkB0FkbWlu"
	if _, err := codec.Decode(legacy); err == nil {
		t.Fatal("legacy Revel cookie must be rejected (anonymous), not decoded")
	}
}

func TestSessionDifferentSecretCannotDecode(t *testing.T) {
	codec := newTestCodec(t)
	cookie, _ := codec.Encode(map[string]string{"UserId": "abc"})
	other := newTestCodec(t)
	other.Secret = []byte("another-secret")
	if _, err := other.Decode(cookie.Value); err == nil {
		t.Fatal("cookie from another secret must be rejected")
	}
}

func flipLastChar(s string) string {
	if s == "" {
		return "x"
	}
	last := s[len(s)-1]
	replacement := byte(last + 1)
	if replacement == '%' || replacement == '.' {
		replacement = 'A'
	}
	return s[:len(s)-1] + string(replacement)
}
