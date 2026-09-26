package httpserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/yangphere/leanote/app/service"
)

func TestMapContentRootErrorUsesStableCodeAndConfiguredKey(t *testing.T) {
	tests := []struct {
		name string
		err  string
		code string
		key  string
	}{
		{name: "missing", err: "content roots: public_upload.data is inaccessible", code: "CONFIG_CONTENT_ROOT_MISSING", key: "content.public.data"},
		{name: "relative", err: "content roots: temporary must be an absolute path", code: "CONFIG_CONTENT_ROOT_RELATIVE", key: "content.temporary"},
		{name: "unwritable", err: "content roots: temporary root is not writable", code: "CONFIG_CONTENT_ROOT_UNWRITABLE", key: "content.temporary"},
		{name: "atomic rename failure", err: "content roots: private files pair is not on one filesystem", code: "CONFIG_CONTENT_ROOT_UNWRITABLE", key: "content.private.data"},
		{name: "public", err: "content roots: public_upload.quarantine overlaps a served root", code: "CONFIG_CONTENT_ROOT_PUBLIC", key: "content.public.quarantine"},
		{name: "overlap", err: "content roots: public_upload.data overlaps temporary", code: "CONFIG_CONTENT_ROOT_OVERLAP", key: "content.public.data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mapContentRootError(errors.New(test.err))
			var configErr *ConfigError
			if !errors.As(got, &configErr) {
				t.Fatalf("error = %v, want ConfigError", got)
			}
			if configErr.Code != test.code || configErr.Key != test.key {
				t.Fatalf("ConfigError = %+v, want code=%s key=%s", configErr, test.code, test.key)
			}
		})
	}
}

func TestMapContentRootErrorDistinguishesCrossDeviceFromProbeFailure(t *testing.T) {
	crossDevice := fmt.Errorf("content roots: private files pair is not on one filesystem: %w", &os.LinkError{Op: "rename", Old: "private", New: "quarantine", Err: syscall.EXDEV})
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{name: "cross device", err: crossDevice, code: "CONFIG_CONTENT_ROOT_CROSS_DEVICE"},
		{name: "permission", err: fmt.Errorf("content roots: private files pair is not on one filesystem: %w", os.ErrPermission), code: "CONFIG_CONTENT_ROOT_UNWRITABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var configErr *ConfigError
			if err := mapContentRootError(test.err); !errors.As(err, &configErr) || configErr.Code != test.code {
				t.Fatalf("mapContentRootError(%v) = %v, want %s", test.err, err, test.code)
			}
		})
	}
}

func TestDatabaseIdentityAndCredentialReferenceAreCanonicalAndCredentialFree(t *testing.T) {
	identity, err := databaseIdentity("mongodb://db.Example:27018/leanote?authSource=admin&tls=true", "leanote")
	if err != nil {
		t.Fatalf("databaseIdentity: %v", err)
	}
	if identity.Scheme != "mongodb" || identity.ClusterHost != "db.example" || identity.ClusterPort != "27018" || identity.DatabaseName != "leanote" || identity.AuthSource != "admin" || identity.TLSMode != "enabled" {
		t.Fatalf("identity = %+v", identity)
	}
	digest, err := identity.Digest()
	if err != nil || len(digest) != 64 || strings.Contains(digest, "password") {
		t.Fatalf("identity digest = %q, err=%v", digest, err)
	}
	ref := service.CredentialProviderRef{ProviderKind: "env", OpaqueHandle: "MONGODB_URL", Owner: "interface-http"}
	if err := ref.Validate(); err != nil {
		t.Fatalf("credential provider reference: %v", err)
	}
}

func TestCanonicalBackupRootRejectsMissingDirectory(t *testing.T) {
	_, err := canonicalBackupRoot(t.TempDir() + "/missing")
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("canonicalBackupRoot error = %v, want missing error", err)
	}
}

func TestValidateProductionRuntimeConfigRejectsNilConfig(t *testing.T) {
	if _, err := ValidateProductionRuntimeConfig(nil); err == nil {
		t.Fatal("nil config unexpectedly accepted")
	}
}

func TestValidateProductionRuntimeConfigBuildsCanonicalTypedRoots(t *testing.T) {
	base := t.TempDir()
	mkdir := func(name string) string {
		path := filepath.Join(base, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		return path
	}
	privateData := mkdir("private-files")
	privateQuarantine := mkdir("private-quarantine")
	publicData := mkdir("public-upload")
	publicQuarantine := mkdir("public-quarantine")
	temporary := mkdir("temporary")
	backup := mkdir("backup")
	publicStatic := mkdir("public-static")
	configText := fmt.Sprintf("[prod]\nhttp.addr=127.0.0.1\nhttp.port=9010\nhttp.shutdownTimeoutMs=7000\ndb.dbname=leanote\ncontent.private.data=%s\ncontent.private.quarantine=%s\ncontent.public.data=%s\ncontent.public.quarantine=%s\ncontent.temporary=%s\nadmin.backup.root=%s\n", privateData, privateQuarantine, publicData, publicQuarantine, temporary, backup)
	cfg, err := ParseConfig([]byte(configText), "prod")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	t.Setenv("MONGODB_URL", "mongodb://db.example/leanote")
	runtimeConfig, err := ValidateProductionRuntimeConfig(cfg, publicStatic)
	if err != nil {
		t.Fatalf("ValidateProductionRuntimeConfig: %v", err)
	}
	if runtimeConfig.ContentRoots.PrivateFiles.Data != privateData || runtimeConfig.ContentRoots.PublicUpload.Data != publicData || runtimeConfig.BackupRoot != backup {
		t.Fatalf("typed roots = %+v backup=%q", runtimeConfig.ContentRoots, runtimeConfig.BackupRoot)
	}
	if len(runtimeConfig.ContentRoots.ServedRoots) != 2 || runtimeConfig.ContentRoots.ServedRoots[0] != publicData || runtimeConfig.ContentRoots.ServedRoots[1] != publicStatic {
		t.Fatalf("served roots = %v, want canonical public data and static roots", runtimeConfig.ContentRoots.ServedRoots)
	}
	if runtimeConfig.ConfiguredDatabaseIdentity.DatabaseName != "leanote" || runtimeConfig.DatabaseIdentityDigest == "" {
		t.Fatalf("database handoff = %+v digest=%q", runtimeConfig.ConfiguredDatabaseIdentity, runtimeConfig.DatabaseIdentityDigest)
	}
	if runtimeConfig.Addr != "127.0.0.1:9010" || runtimeConfig.ShutdownTimeout.String() != "7s" {
		t.Fatalf("server handoff = addr %q timeout %s", runtimeConfig.Addr, runtimeConfig.ShutdownTimeout)
	}
}

func TestValidateProductionRuntimeConfigClassifiesBackupPlacement(t *testing.T) {
	for _, test := range []struct {
		name       string
		backupBase string
		code       string
	}{
		{name: "public", backupBase: "public-upload", code: "CONFIG_CONTENT_ROOT_PUBLIC"},
		{name: "private overlap", backupBase: "private-files", code: "CONFIG_CONTENT_ROOT_OVERLAP"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			mkdir := func(name string) string {
				path := filepath.Join(base, name)
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", name, err)
				}
				return path
			}
			privateData := mkdir("private-files")
			privateQuarantine := mkdir("private-quarantine")
			publicData := mkdir("public-upload")
			publicQuarantine := mkdir("public-quarantine")
			temporary := mkdir("temporary")
			publicStatic := mkdir("public-static")
			backup := filepath.Join(base, test.backupBase, "backup")
			if err := os.MkdirAll(backup, 0o755); err != nil {
				t.Fatalf("mkdir backup: %v", err)
			}
			configText := fmt.Sprintf("[prod]\ndb.dbname=leanote\ncontent.private.data=%s\ncontent.private.quarantine=%s\ncontent.public.data=%s\ncontent.public.quarantine=%s\ncontent.temporary=%s\nadmin.backup.root=%s\n", privateData, privateQuarantine, publicData, publicQuarantine, temporary, backup)
			cfg, err := ParseConfig([]byte(configText), "prod")
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			t.Setenv("MONGODB_URL", "mongodb://db.example/leanote")
			_, err = ValidateProductionRuntimeConfig(cfg, publicStatic)
			var configErr *ConfigError
			if !errors.As(err, &configErr) || configErr.Code != test.code || configErr.Key != "admin.backup.root" {
				t.Fatalf("backup placement error = %v, want %s/admin.backup.root", err, test.code)
			}
		})
	}
}

func TestValidateProductionRuntimeConfigRejectsMissingPublicStaticRoot(t *testing.T) {
	base := t.TempDir()
	mkdir := func(name string) string {
		path := filepath.Join(base, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		return path
	}
	configText := fmt.Sprintf("[prod]\ndb.dbname=leanote\ncontent.private.data=%s\ncontent.private.quarantine=%s\ncontent.public.data=%s\ncontent.public.quarantine=%s\ncontent.temporary=%s\nadmin.backup.root=%s\n", mkdir("private-files"), mkdir("private-quarantine"), mkdir("public-upload"), mkdir("public-quarantine"), mkdir("temporary"), mkdir("backup"))
	cfg, err := ParseConfig([]byte(configText), "prod")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	t.Setenv("MONGODB_URL", "mongodb://db.example/leanote")
	_, err = ValidateProductionRuntimeConfig(cfg, filepath.Join(base, "missing-public"))
	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Code != "CONFIG_CONTENT_ROOT_MISSING" || configErr.Key != "content.public.data" {
		t.Fatalf("missing public static root error = %v", err)
	}
}
