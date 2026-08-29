package httpserver

import (
	"strings"
	"testing"
	"time"
)

func TestNewSessionCodecWiresConfigValues(t *testing.T) {
	cfg, err := ParseConfig([]byte(strings.Join([]string{
		"app.secret=codec-secret",
		"cookie.prefix=LEANOTE",
		"cookie.domain=example.com",
		"cookie.secure=true",
		"session.expires=3h",
	}, "\n")), "")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	codec := NewSessionCodec(cfg)
	if string(codec.Secret) != "codec-secret" {
		t.Fatalf("Secret = %q", codec.Secret)
	}
	if codec.Prefix != "LEANOTE" || codec.Domain != "example.com" || !codec.Secure {
		t.Fatalf("prefix/domain/secure not wired: %+v", codec)
	}
	if codec.TTL != 3*time.Hour {
		t.Fatalf("TTL = %v, want 3h", codec.TTL)
	}
	cookie, err := codec.Encode(map[string]string{"UserId": "x"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !cookie.Secure || cookie.Domain != "example.com" || cookie.Name != "LEANOTE_SESSION" {
		t.Fatalf("cookie attributes not wired: %+v", cookie)
	}
}

func TestNewSessionCodecFallsBackOnInvalidTTL(t *testing.T) {
	for _, bad := range []string{"abc", "-5m", "0s"} {
		cfg, err := ParseConfig([]byte("app.secret=s\ncookie.prefix=P\nsession.expires="+bad+"\n"), "")
		if err != nil {
			t.Fatalf("ParseConfig(%q): %v", bad, err)
		}
		codec := NewSessionCodec(cfg)
		if codec.TTL != 3*time.Hour {
			t.Fatalf("session.expires=%q TTL = %v, want fallback 3h", bad, codec.TTL)
		}
	}
}
