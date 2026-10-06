package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/httpserver"
)

func TestSessionCodecForRuntimeControlsCookieAndSignedExpiry(t *testing.T) {
	for _, test := range []struct {
		name   string
		secure bool
		ttl    time.Duration
	}{
		{name: "https default", secure: true, ttl: 168 * time.Hour},
		{name: "http custom", ttl: 24 * time.Hour},
		{name: "minimum", secure: true, ttl: 5 * time.Minute},
		{name: "maximum", secure: true, ttl: 8760 * time.Hour},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := httpserver.ParseConfig([]byte("app.secret=fixture-secret\ncookie.prefix=LEANOTE\ncookie.secure=false\nsession.expires=3h\n"), "")
			if err != nil {
				t.Fatal(err)
			}
			codec := sessionCodecForRuntime(cfg, &httpserver.ProductionConfig{CookieSecure: test.secure, SessionTTL: test.ttl})
			now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
			codec.NowFunc = func() time.Time { return now }
			cookie, err := codec.Encode(map[string]string{"UserId": "fixture-user"})
			if err != nil {
				t.Fatal(err)
			}
			if cookie.Secure != test.secure || cookie.MaxAge != int(test.ttl/time.Second) || !cookie.Expires.Equal(now.Add(test.ttl)) {
				t.Fatalf("cookie Secure=%t MaxAge=%d Expires=%v, want %t/%d/%v", cookie.Secure, cookie.MaxAge, cookie.Expires, test.secure, int(test.ttl/time.Second), now.Add(test.ttl))
			}
			if cookie.Name != "LEANOTE_SESSION" || cookie.Path != "/" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("existing session cookie attributes changed: %s", cookie.Name)
			}
			codec.NowFunc = func() time.Time { return now.Add(test.ttl - time.Second) }
			keys, err := codec.Decode(cookie.Value)
			if err != nil || keys["UserId"] != "fixture-user" {
				t.Fatalf("session expired before configured TTL: %v", err)
			}
			codec.NowFunc = func() time.Time { return now.Add(test.ttl) }
			if _, err := codec.Decode(cookie.Value); err == nil {
				t.Fatal("signed session remains valid at configured expiry")
			}
		})
	}
}
