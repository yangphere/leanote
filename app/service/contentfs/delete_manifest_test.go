package contentfs

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
)

func TestDeleteManifestStorePublishesNoReplaceAndCASUpdates(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)

	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Create(context.Background(), manifest); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("second Create() error = %v, want fs.ErrExist", err)
	}
	loaded, found, err := store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
	if err != nil || !found || loaded.Version != 1 || loaded.StateDigest != manifest.StateDigest {
		t.Fatalf("LoadDeleteManifest() manifest=%+v found=%v err=%v", loaded, found, err)
	}
	quarantine, err := application.ParseLogicalPath(application.RootPrivateFiles, "quarantine/asset")
	if err != nil {
		t.Fatal(err)
	}
	next, err := manifest.Advance(application.DeleteStageQuarantined, quarantine, time.Unix(1_700_000_001, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
		t.Fatalf("CompareAndSwap() error = %v", err)
	}
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, next); applicationErrorCategory(err) != application.ErrorConflict {
		t.Fatalf("stale CompareAndSwap() error = %v", err)
	}
}

func TestDeleteManifestStoreTerminalGCUsesLookupAndRetention(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	terminal, err := manifest.Terminal(time.Unix(1_700_000_100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, terminal); err != nil {
		t.Fatal(err)
	}
	removed, err := store.GCTerminals(context.Background(), terminal.TerminalAt.Add(7*24*time.Hour-time.Nanosecond))
	if err != nil || len(removed) != 0 {
		t.Fatalf("early GC removed=%v err=%v", removed, err)
	}
	removed, err = store.GCTerminals(context.Background(), terminal.TerminalAt.Add(7*24*time.Hour+time.Nanosecond))
	if err != nil || len(removed) != 1 || removed[0] != manifest.LookupKey {
		t.Fatalf("expired GC removed=%v err=%v", removed, err)
	}
	_, found, err := store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
	if err != nil || found {
		t.Fatalf("GC lookup found=%v err=%v", found, err)
	}
}

func TestDeleteManifestStoreRejectsTrailingManifestBytes(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}

	root, name, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("trailing"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
	if applicationErrorCategory(err) != application.ErrorConflict {
		t.Fatalf("LoadDeleteManifest() error = %v, want corrupt manifest conflict", err)
	}
}

func TestDeleteManifestStoreRejectsOversizedManifest(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}

	root, name, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(strings.Repeat(" ", 1<<20)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
	if applicationErrorCategory(err) != application.ErrorConflict {
		t.Fatalf("LoadDeleteManifest() error = %v, want oversized manifest conflict", err)
	}
}

func TestDeleteManifestStoreFailsClosedWhenParentSyncFails(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	called := false
	store.directorySync = func(*os.Root, string) error {
		called = true
		return errors.New("sync failed")
	}
	err = store.Create(context.Background(), newDeleteManifest(t))
	if runtime.GOOS == "windows" {
		if err != nil || called {
			t.Fatalf("Windows Create() error=%v directorySyncCalled=%v", err, called)
		}
	} else if applicationErrorCategory(err) != application.ErrorUnknownResult {
		t.Fatalf("Create() error = %v, want parent sync unknown result", err)
	}
}

func TestDeleteManifestStoreRecoversLegacyAndCrashedLease(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	root, _, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureManifestParent(root, "leases", store.directorySync); err != nil {
		t.Fatal(err)
	}
	leaseName, _ := manifestLeaseNames(manifest.LookupKey)
	legacy, err := root.OpenFile(leaseName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(root.Name(), leaseName)
	command := exec.Command(os.Args[0], "-test.run=^TestDeleteManifestLeaseHelperProcess$")
	command.Env = append(os.Environ(), "LEANOTE_TEST_LEASE_HELPER=1", "LEANOTE_TEST_LEASE_PATH="+lockPath)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("lease helper readiness line=%q err=%v", line, err)
	}
	if err := store.Create(context.Background(), manifest); applicationErrorCode(err) != "delete_manifest_lease_held" {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("Create() while helper holds lease error = %v", err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("killed lease helper exited successfully")
	}
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatalf("Create() after helper death error = %v", err)
	}
}

func TestDeleteManifestLeaseHelperProcess(t *testing.T) {
	if os.Getenv("LEANOTE_TEST_LEASE_HELPER") != "1" {
		return
	}
	file, err := os.OpenFile(os.Getenv("LEANOTE_TEST_LEASE_PATH"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		os.Exit(2)
	}
	_, held, err := tryLockFile(file)
	if err != nil || held {
		os.Exit(3)
	}
	_, _ = os.Stdout.WriteString("locked\n")
	select {}
}

func TestDeleteManifestStoreRejectsMalformedLeaseRecord(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	root, _, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureManifestParent(root, "leases", store.directorySync); err != nil {
		t.Fatal(err)
	}
	_, recordName := manifestLeaseNames(manifest.LookupKey)
	if err := root.WriteFile(recordName, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(context.Background(), manifest); applicationErrorCode(err) != "delete_manifest_lease_corrupt" {
		t.Fatalf("Create() error = %v, want corrupt lease rejection", err)
	}
}

func TestDeleteManifestStoreRecreatesMissingLeaseRecordUnderPermanentLock(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.directorySync = func(*os.Root, string) error { return nil }
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	root, _, err := store.location(manifest.Source.Kind, manifest.LookupKey)
	if err != nil {
		t.Fatal(err)
	}
	leaseName, recordName := manifestLeaseNames(manifest.LookupKey)
	if err := root.Remove(recordName); err != nil {
		t.Fatal(err)
	}
	next, err := manifest.Advance(application.DeleteStageQuarantined, manifest.Source, time.Unix(1_700_000_001, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
		t.Fatalf("CompareAndSwap() after missing record error = %v", err)
	}
	for _, name := range []string{leaseName, recordName} {
		if _, err := root.Lstat(name); err != nil {
			t.Fatalf("permanent lease artifact %q missing after recovery: %v", name, err)
		}
	}
}

func TestManifestRingRankingRotatesWithinOneShard(t *testing.T) {
	keys := []string{
		"aa10000000000000000000000000000000000000000000000000000000000000",
		"aa20000000000000000000000000000000000000000000000000000000000000",
		"aa30000000000000000000000000000000000000000000000000000000000000",
	}
	var ranked []rankedManifestKey
	cursor, err := strconv.ParseUint("aa25000000000000", 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		ranked = insertRankedManifestKey(ranked, key, cursor, 2)
	}
	if len(ranked) != 2 || ranked[0].key != keys[2] || ranked[1].key != keys[0] {
		t.Fatalf("ranked keys = %+v, want ring order after cursor", ranked)
	}
}

func newDeleteManifest(t *testing.T) application.DeleteManifest {
	t.Helper()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	source, err := application.ParseLogicalPath(application.RootPrivateFiles, "a/b/image.png")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := application.NewDeleteManifest(application.DeleteIdentity{
		Action: "delete_image", OwnerID: owner, Kind: application.AssetImage, AssetID: "507f1f77bcf86cd799439012",
		Generation: 1, ContentDigest: sha256.Sum256([]byte("image")), ContentSize: 5,
	}, source, time.Unix(1_700_000_000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func applicationErrorCategory(err error) application.ErrorCategory {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}

func applicationErrorCode(err error) string {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Code
	}
	return ""
}
