//go:build windows

package contentfs

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

func TestWindowsFinalFileBarrierFailpointsRemainUnknownAndRetryable(t *testing.T) {
	for _, test := range []struct {
		name   string
		inject func(*FileStore)
	}{
		{name: "open", inject: func(store *FileStore) {
			store.finalOpen = func(*os.Root, string) (*os.File, error) { return nil, errors.New("open failed") }
		}},
		{name: "sync", inject: func(store *FileStore) {
			store.finalSync = func(*os.File) error { return errors.New("sync failed") }
		}},
		{name: "close", inject: func(store *FileStore) {
			store.finalClose = func(file *os.File) error { return errors.Join(file.Close(), errors.New("close failed")) }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "barriers/asset.bin"}
			request := publishRequest(t, destination, "durable")
			test.inject(store)
			if _, err := store.Publish(context.Background(), request); applicationErrorCategory(err) != application.ErrorUnknownResult || applicationErrorCode(err) != "publish_final_sync" {
				t.Fatalf("Publish() error=%v, want publish_final_sync unknown_result", err)
			}
			store.finalOpen, store.finalSync, store.finalClose = nil, nil, nil
			result, err := store.Publish(context.Background(), request)
			if err != nil || result.Status != application.PublishAlreadyApplied {
				t.Fatalf("retry result=%+v err=%v", result, err)
			}
		})
	}
}

func TestWindowsManifestNoReplaceAndReplaceUseFinalFileBarrier(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	count := 0
	store.finalSync = func(file *os.File) error {
		count++
		return file.Sync()
	}
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	next, err := manifest.Advance(application.DeleteStageQuarantined, manifest.Source, time.Unix(1_700_000_001, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, next); err != nil {
		t.Fatal(err)
	}
	create := newCreateManifest(t, time.Unix(1_800_000_000, 0).UTC())
	if err := store.CreateCreateManifest(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	published, err := create.Published(time.Unix(1_800_000_001, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompareAndSwapCreateManifest(context.Background(), create.LookupKey, create.Version, create.StateDigest, published); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("final sync count=%d, want one for each no-replace/replace", count)
	}
}

func TestWindowsManifestReplaceBarrierFailureConvergesThroughLoad(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manifest := newDeleteManifest(t)
	if err := store.Create(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	next, err := manifest.Advance(application.DeleteStageQuarantined, manifest.Source, time.Unix(1_700_000_001, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	store.finalSync = func(*os.File) error { return errors.New("sync failed after replace") }
	if err := store.CompareAndSwap(context.Background(), manifest.LookupKey, manifest.Version, manifest.StateDigest, next); applicationErrorCategory(err) != application.ErrorUnknownResult || applicationErrorCode(err) != "delete_manifest_replace_sync" {
		t.Fatalf("CompareAndSwap() error=%v, want delete_manifest_replace_sync unknown_result", err)
	}
	store.finalSync = nil
	loaded, found, err := store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
	if err != nil || !found || loaded.Version != next.Version || loaded.StateDigest != next.StateDigest {
		t.Fatalf("LoadDeleteManifest() manifest=%+v found=%v err=%v", loaded, found, err)
	}
}

func TestWindowsManifestNoReplaceBarrierFailpointsConvergeThroughLoad(t *testing.T) {
	for _, test := range []struct {
		name   string
		inject func(*DeleteManifestStore)
	}{
		{name: "open", inject: func(store *DeleteManifestStore) {
			store.finalOpen = func(*os.Root, string) (*os.File, error) { return nil, errors.New("open failed") }
		}},
		{name: "sync", inject: func(store *DeleteManifestStore) {
			store.finalSync = func(*os.File) error { return errors.New("sync failed") }
		}},
		{name: "close", inject: func(store *DeleteManifestStore) {
			store.finalClose = func(file *os.File) error { return errors.Join(file.Close(), errors.New("close failed")) }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			roots, err := ValidateContentRoots(newRootConfig(t))
			if err != nil {
				t.Fatal(err)
			}
			store, err := NewDeleteManifestStore(roots)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			manifest := newDeleteManifest(t)
			test.inject(store)
			if err := store.Create(context.Background(), manifest); applicationErrorCategory(err) != application.ErrorUnknownResult || applicationErrorCode(err) != "delete_manifest_publish_sync" {
				t.Fatalf("Create() error=%v, want delete_manifest_publish_sync unknown_result", err)
			}
			store.finalOpen, store.finalSync, store.finalClose = nil, nil, nil
			loaded, found, err := store.LoadDeleteManifest(context.Background(), manifest.LookupKey)
			if err != nil || !found || loaded.StateDigest != manifest.StateDigest {
				t.Fatalf("LoadDeleteManifest() manifest=%+v found=%v err=%v", loaded, found, err)
			}
		})
	}
}

func TestWindowsManifestParentCreationIsProvisionalAndRetryable(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := store.handles[application.RootPrivateFiles]
	called := false
	directorySync := func(*os.Root, string) error {
		called = true
		return errors.New("Windows directory sync must not be used")
	}
	if err := ensureManifestParent(root, "provisional/child", directorySync); err != nil || called {
		t.Fatalf("first ensure error=%v directorySyncCalled=%v", err, called)
	}
	if err := root.Remove("provisional/child"); err != nil {
		t.Fatal(err)
	}
	if err := root.Remove("provisional"); err != nil {
		t.Fatal(err)
	}
	if err := ensureManifestParent(root, "provisional/child", directorySync); err != nil || called {
		t.Fatalf("retry ensure error=%v directorySyncCalled=%v", err, called)
	}
}

func TestWindowsTerminalGCUsesSafeReappearanceRetryContract(t *testing.T) {
	roots, err := ValidateContentRoots(newRootConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewDeleteManifestStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
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
	store.directorySync = func(*os.Root, string) error { return errors.New("directory sync unavailable") }
	result, err := store.GCTerminalsBounded(context.Background(), terminal.TerminalAt.Add(8*24*time.Hour), 1)
	if err != nil || len(result.Removed) != 1 {
		t.Fatalf("GC result=%+v err=%v", result, err)
	}
	root := store.handles[manifest.Source.Kind]
	name := manifestName(manifest.LookupKey)
	if err := store.writeManifestNoReplace(root, name, terminal); err != nil {
		t.Fatalf("recreate terminal marker: %v", err)
	}
	result, err = store.GCTerminalsBounded(context.Background(), terminal.TerminalAt.Add(8*24*time.Hour), 1)
	if err != nil || len(result.Removed) != 1 || result.Removed[0] != manifest.LookupKey {
		t.Fatalf("GC after marker reappearance result=%+v err=%v", result, err)
	}
}
