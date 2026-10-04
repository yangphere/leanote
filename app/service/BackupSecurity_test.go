package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguredDatabaseIdentityDigestIsCanonicalAndCredentialFree(t *testing.T) {
	identity := ConfiguredDatabaseIdentity{Scheme: "MONGODB", ClusterHost: "Mongo.Example", ClusterPort: "27017", DatabaseName: "leanote", TLSMode: "DISABLED"}
	one, err := identity.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	two, err := (ConfiguredDatabaseIdentity{Scheme: "mongodb", ClusterHost: "mongo.example", ClusterPort: "27017", DatabaseName: "leanote", TLSMode: "disabled"}).Digest()
	if err != nil || one != two {
		t.Fatalf("canonical digest mismatch: %q/%q err=%v", one, two, err)
	}
	changed, err := (ConfiguredDatabaseIdentity{Scheme: "mongodb", ClusterHost: "mongo.example", ClusterPort: "27017", DatabaseName: "other", TLSMode: "disabled"}).Digest()
	if err != nil || changed == one {
		t.Fatalf("database identity change did not change digest: %q/%q", one, changed)
	}
}

func TestBackupPathContainmentRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "one")
	if err := os.Mkdir(inside, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateContainedPath(root, inside); err != nil {
		t.Fatalf("inside path rejected: %v", err)
	}
	if _, err := ValidateContainedPath(root, filepath.Join(root, "..", "outside")); err == nil {
		t.Fatal("traversal path accepted")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(inside, link); err == nil {
		if _, err := ValidateContainedPath(root, link); err == nil {
			t.Fatal("symlink path accepted")
		}
	}
}

func TestValidateOpaqueBackupIDRejectsPathSyntax(t *testing.T) {
	if err := ValidateOpaqueBackupID("../outside"); err == nil {
		t.Fatal("path-like backup identity accepted")
	}
	if err := ValidateOpaqueBackupID("2026_September_25_123"); err != nil {
		t.Fatalf("valid backup identity rejected: %v", err)
	}
}

func TestMongoCommandArgsNeverContainPassword(t *testing.T) {
	args := BuildMongoDumpArgs("/usr/bin/mongodump", "mongo.internal", "27017", "leanote", "/safe/root", "leanote")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "password") || strings.Contains(joined, "secret") {
		t.Fatalf("dump args contain credential material: %q", joined)
	}
	restore := BuildMongoRestoreArgs("/usr/bin/mongorestore", "mongo.internal", "27017", "leanote", "/safe/root", "leanote")
	if strings.Contains(strings.Join(restore, " "), "password") {
		t.Fatalf("restore args contain password flag/value: %q", restore)
	}
}

func TestBackupDirectorySizeRejectsSymlinksAndCountsRetainedBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.bson"), []byte("1234"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "two.bson"), []byte("567"), 0600); err != nil {
		t.Fatal(err)
	}
	size, err := backupDirectorySize(root)
	if err != nil || size != 7 {
		t.Fatalf("backupDirectorySize=%d err=%v, want 7", size, err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "one.bson"), link); err == nil {
		if _, err := backupDirectorySize(root); err == nil {
			t.Fatal("backupDirectorySize accepted symlink")
		}
	}
}
