package contentfs

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
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
		Generation: 1, ContentDigest: sha256.Sum256([]byte("image")),
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
