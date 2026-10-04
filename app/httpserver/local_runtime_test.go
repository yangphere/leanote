package httpserver

import (
	"os"
	"path/filepath"
	"testing"
)

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
