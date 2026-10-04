package contentfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	application "github.com/yangphere/leanote/app/application/content"
)

type cancelingLifecycleReader struct {
	cancel context.CancelFunc
	read   bool
}

func (reader *cancelingLifecycleReader) Read(buffer []byte) (int, error) {
	if reader.read {
		return 0, errors.New("source read after cancellation")
	}
	reader.read = true
	copy(buffer, "first")
	reader.cancel()
	return len("first"), nil
}

func TestCopyLifecycleBytesStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var destination bytes.Buffer
	_, err := copyLifecycleBytes(ctx, &destination, &cancelingLifecycleReader{cancel: cancel})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copyLifecycleBytes() error = %v, want context canceled", err)
	}
	if destination.String() != "first" {
		t.Fatalf("destination = %q, want first chunk only", destination.String())
	}
}

func TestCopyLifecycleFileReportsCleanupSyncFailureAsUnknown(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()

	sourceName := filepath.Join("owner", "image.png")
	destinationName := filepath.Join("objects", "aa", "copy")
	if err := os.MkdirAll(filepath.Join(config.PrivateFiles.Data, "owner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(config.PrivateFiles.Quarantine, "objects", "aa"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.PrivateFiles.Data, sourceName), []byte("actual"), 0o600); err != nil {
		t.Fatal(err)
	}
	lifecycle.directorySync = func(*os.Root, string) error { return errors.New("cleanup sync failed") }

	err = lifecycle.copyLifecycleFile(
		context.Background(),
		lifecycle.dataRoots[application.RootPrivateFiles], sourceName,
		lifecycle.quarantineRoots[application.RootPrivateFiles], destinationName,
		sha256.Sum256([]byte("expected")), 6,
	)
	var contentErr *application.Error
	if runtime.GOOS == "windows" {
		if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorConflict || contentErr.Code != "lifecycle_copy_digest" {
			t.Fatalf("copyLifecycleFile() error = %v, want digest conflict with retry-safe Windows cleanup", err)
		}
	} else if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorUnknownResult || contentErr.Code != "lifecycle_copy_cleanup" {
		t.Fatalf("copyLifecycleFile() error = %v, want cleanup unknown result", err)
	}
	if _, statErr := os.Stat(filepath.Join(config.PrivateFiles.Quarantine, destinationName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("partial destination remains after cleanup: %v", statErr)
	}
}

func TestLifecycleStoreQuarantineRestoreAndPurgeAreIdempotent(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	lifecycle.directorySync = func(*os.Root, string) error { return nil }
	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	request := publishRequest(t, source, "image")
	if _, err := files.Publish(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("image"))
	quarantine, err := lifecycle.Quarantine(context.Background(), source, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", digest, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.Quarantine(context.Background(), source, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", digest, 5); err != nil {
		t.Fatalf("idempotent Quarantine() error = %v", err)
	}
	if err := lifecycle.Restore(context.Background(), source, quarantine, digest, 5); err != nil {
		t.Fatal(err)
	}
	quarantine, err = lifecycle.Quarantine(context.Background(), source, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", digest, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Purge(context.Background(), quarantine, digest, 5); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Purge(context.Background(), quarantine, digest, 5); err != nil {
		t.Fatalf("idempotent Purge() error = %v", err)
	}
}

func TestLifecycleStoreRejectsNonHexLookupKey(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()

	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	lookupKey := strings.Repeat("a", 31) + "/../" + strings.Repeat("b", 29)
	_, err = lifecycle.Quarantine(context.Background(), source, lookupKey, sha256.Sum256(nil), 5)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorValidation || contentErr.Code != "lifecycle_input" {
		t.Fatalf("Quarantine() error = %v, want lifecycle_input validation error", err)
	}
}

func TestLifecycleStoreRejectsExpectedSizeMismatchBeforeRelocation(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	lifecycle.directorySync = func(*os.Root, string) error { return nil }
	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	if _, err := files.Publish(context.Background(), publishRequest(t, source, "image")); err != nil {
		t.Fatal(err)
	}
	_, err = lifecycle.Quarantine(context.Background(), source, strings.Repeat("a", 64), sha256.Sum256([]byte("image")), 4)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorConflict || contentErr.Code != "lifecycle_size_mismatch" {
		t.Fatalf("Quarantine() error = %v, want size conflict", err)
	}
	if body, readErr := os.ReadFile(filepath.Join(config.PrivateFiles.Data, filepath.FromSlash(source.Value))); readErr != nil || string(body) != "image" {
		t.Fatalf("source body=%q err=%v", body, readErr)
	}
}

func TestLifecycleStoreUsesOpenedRootsAfterConfiguredPathSwap(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	lifecycle.directorySync = func(*os.Root, string) error { return nil }

	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	if _, err := files.Publish(context.Background(), publishRequest(t, source, "image")); err != nil {
		t.Fatal(err)
	}
	movedRoot := config.PrivateFiles.Data + "-moved"
	if err := os.Rename(config.PrivateFiles.Data, movedRoot); err != nil {
		t.Skipf("platform cannot rename opened content root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(config.PrivateFiles.Data, "owner"), 0o700); err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(config.PrivateFiles.Data, filepath.FromSlash(source.Value))
	if err := os.WriteFile(decoy, []byte("decoy"), 0o600); err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256([]byte("image"))
	quarantine, err := lifecycle.Quarantine(context.Background(), source, strings.Repeat("a", 64), digest, 5)
	if err != nil {
		t.Fatalf("Quarantine() error = %v", err)
	}
	if body, err := os.ReadFile(decoy); err != nil || string(body) != "decoy" {
		t.Fatalf("replacement-root decoy body=%q err=%v", body, err)
	}
	quarantined := filepath.Join(config.PrivateFiles.Quarantine, filepath.FromSlash(quarantine.Value))
	if body, err := os.ReadFile(quarantined); err != nil || string(body) != "image" {
		t.Fatalf("quarantined body=%q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(movedRoot, filepath.FromSlash(source.Value))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("opened-root source remains after quarantine: %v", err)
	}
}

func TestLifecycleStoreFailsClosedWhenParentSyncFails(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	if _, err := files.Publish(context.Background(), publishRequest(t, source, "image")); err != nil {
		t.Fatal(err)
	}
	called := false
	lifecycle.directorySync = func(*os.Root, string) error {
		called = true
		return errors.New("sync failed")
	}
	_, err = lifecycle.Quarantine(context.Background(), source, strings.Repeat("a", 64), sha256.Sum256([]byte("image")), 5)
	if runtime.GOOS == "windows" {
		if err != nil || called {
			t.Fatalf("Windows Quarantine() error=%v directorySyncCalled=%v", err, called)
		}
		return
	}
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorUnknownResult || contentErr.Code != "quarantine_parent_sync" {
		t.Fatalf("Quarantine() error = %v, want quarantine_parent_sync unknown result", err)
	}
}

func TestLifecycleStoreRetryCompletesAfterDestinationSyncUnknown(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/image.png"}
	if _, err := files.Publish(context.Background(), publishRequest(t, source, "image")); err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	if err := os.MkdirAll(filepath.Join(config.PrivateFiles.Quarantine, "objects", key[:2]), 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		lifecycle.finalSync = func(*os.File) error { return errors.New("sync result unknown") }
	} else {
		lifecycle.directorySync = func(*os.Root, string) error { return errors.New("sync result unknown") }
	}
	_, err = lifecycle.Quarantine(context.Background(), source, key, sha256.Sum256([]byte("image")), 5)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorUnknownResult || contentErr.Code != "lifecycle_destination_sync" {
		t.Fatalf("Quarantine() error = %v", err)
	}
	lifecycle.directorySync = func(*os.Root, string) error { return nil }
	lifecycle.finalSync = nil
	quarantine, err := lifecycle.Quarantine(context.Background(), source, key, sha256.Sum256([]byte("image")), 5)
	if err != nil {
		t.Fatalf("retry Quarantine() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(config.PrivateFiles.Data, filepath.FromSlash(source.Value))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source remains after retry: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(config.PrivateFiles.Quarantine, filepath.FromSlash(quarantine.Value))); err != nil || string(body) != "image" {
		t.Fatalf("quarantine body=%q err=%v", body, err)
	}
}

func TestLifecycleStoreRetryCompletesAfterBusinessRemovalBarrierUnknown(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	files, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	files.directorySync = func(*os.Root, string) error { return nil }
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	lifecycle.directorySync = func(*os.Root, string) error { return nil }
	source := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "owner/removal-barrier.png"}
	if _, err := files.Publish(context.Background(), publishRequest(t, source, "image")); err != nil {
		t.Fatal(err)
	}
	lifecycle.removalSync = func(*os.Root, string) error { return errors.New("removal durability unknown") }
	key := strings.Repeat("b", 64)
	_, err = lifecycle.Quarantine(context.Background(), source, key, sha256.Sum256([]byte("image")), 5)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorUnknownResult || contentErr.Code != "lifecycle_source_sync" {
		t.Fatalf("Quarantine() error=%v, want lifecycle_source_sync unknown_result", err)
	}
	lifecycle.removalSync = nil
	quarantine, err := lifecycle.Quarantine(context.Background(), source, key, sha256.Sum256([]byte("image")), 5)
	if err != nil {
		t.Fatalf("retry Quarantine() error=%v", err)
	}
	if body, err := os.ReadFile(filepath.Join(config.PrivateFiles.Quarantine, filepath.FromSlash(quarantine.Value))); err != nil || string(body) != "image" {
		t.Fatalf("quarantine body=%q err=%v", body, err)
	}
}

func TestLifecycleStoreRejectsParentIndirectionOutsideOpenedRoot(t *testing.T) {
	config := newRootConfig(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "image.png"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(config.PrivateFiles.Data, "owner")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := NewLifecycleStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	defer lifecycle.Close()
	lifecycle.directorySync = func(*os.Root, string) error { return nil }
	_, err = lifecycle.Quarantine(context.Background(), application.LogicalPath{
		Kind: application.RootPrivateFiles, Value: "owner/image.png",
	}, strings.Repeat("a", 64), sha256.Sum256([]byte("image")), 5)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorUnsafePath {
		t.Fatalf("Quarantine() error=%v, want unsafe path", err)
	}
	if body, err := os.ReadFile(filepath.Join(outside, "image.png")); err != nil || string(body) != "image" {
		t.Fatalf("outside body=%q err=%v", body, err)
	}
}
