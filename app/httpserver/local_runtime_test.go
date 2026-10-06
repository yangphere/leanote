package httpserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateLocalRuntimeConfigPreservesSessionSettings(t *testing.T) {
	for _, mode := range []string{"dev", "test"} {
		for _, test := range []struct {
			name     string
			settings string
			secure   bool
			ttl      time.Duration
		}{
			{name: "default", ttl: 3 * time.Hour},
			{name: "custom", settings: "cookie.secure=true\nsession.expires=24h\n", secure: true, ttl: 24 * time.Hour},
			{name: "invalid", settings: "cookie.secure=invalid\nsession.expires=7d\n", ttl: 3 * time.Hour},
			{name: "zero", settings: "session.expires=0s\n", ttl: 3 * time.Hour},
			{name: "negative", settings: "session.expires=-1h\n", ttl: 3 * time.Hour},
			{name: "short local TTL", settings: "session.expires=1s\n", ttl: time.Second},
			{name: "long local TTL", settings: "session.expires=9000h\n", ttl: 9000 * time.Hour},
		} {
			t.Run(mode+"/"+test.name, func(t *testing.T) {
				base := t.TempDir()
				if err := os.Mkdir(filepath.Join(base, "public"), 0o755); err != nil {
					t.Fatal(err)
				}
				cfg, err := ParseConfig([]byte("["+mode+"]\ndb.dbname=leanote_test\n"+test.settings), mode)
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := ValidateLocalRuntimeConfig(cfg, mode, base)
				if err != nil {
					t.Fatal(err)
				}
				codec := NewSessionCodec(cfg)
				if runtime.CookieSecure != test.secure || runtime.SessionTTL != test.ttl || codec.Secure != test.secure || codec.TTL != test.ttl {
					t.Fatalf("local runtime/codec settings differ from expected Secure %t TTL %v", test.secure, test.ttl)
				}
			})
		}
	}
}

func TestValidateLocalRuntimeConfigResolvesRepositoryDefaults(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "public"), 0o755); err != nil {
		t.Fatalf("create public root: %v", err)
	}
	cfg, err := ParseConfig([]byte("db.dbname=leanote_test\ndb.host=127.0.0.1\ndb.port=27017\n[test]\nhttp.addr=127.0.0.1\nhttp.port=28017\n"), "test")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	runtime, err := ValidateLocalRuntimeConfig(cfg, "test", base)
	if err != nil {
		t.Fatalf("ValidateLocalRuntimeConfig: %v", err)
	}
	if runtime.Addr != "127.0.0.1:28017" || runtime.DatabaseName != "leanote_test" {
		t.Fatalf("runtime = %#v, want loopback test configuration", runtime)
	}
	if !filepath.IsAbs(runtime.ContentRoots.PublicUpload.Data) || runtime.ContentRoots.PublicUpload.Data == "" {
		t.Fatalf("public upload root = %q, want absolute default", runtime.ContentRoots.PublicUpload.Data)
	}
}

func TestValidateLocalRuntimeConfigRejectsNonIsolatedTestDatabase(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "public"), 0o755); err != nil {
		t.Fatalf("create public root: %v", err)
	}
	cfg, err := ParseConfig([]byte("db.dbname=leanote\n[test]\n"), "test")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if _, err := ValidateLocalRuntimeConfig(cfg, "test", base); err == nil {
		t.Fatal("ValidateLocalRuntimeConfig accepted non-isolated test database")
	}
}
