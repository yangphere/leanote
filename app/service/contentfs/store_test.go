package contentfs

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	application "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
)

func TestFileStorePublishIsNoClobberAndIdempotent(t *testing.T) {
	store, _ := newTestStore(t)
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "objects/asset.bin"}
	request := publishRequest(t, destination, "first")

	first, err := store.Publish(context.Background(), request)
	if err != nil || first.Status != application.PublishApplied {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	retry := request
	retry.Source = strings.NewReader("first")
	second, err := store.Publish(context.Background(), retry)
	if err != nil || second.Status != application.PublishAlreadyApplied || second.Digest != first.Digest {
		t.Fatalf("second=%+v err=%v", second, err)
	}

	conflict := publishRequest(t, destination, "different")
	if _, err := store.Publish(context.Background(), conflict); errorCategory(err) != application.ErrorConflict {
		t.Fatalf("conflict error=%v category=%q", err, errorCategory(err))
	}
}

func TestFileStoreConcurrentPublishHasOneCreateAndNoOverwrite(t *testing.T) {
	store, _ := newTestStore(t)
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "objects/concurrent.bin"}
	const workers = 12
	statuses := make(chan application.PublishStatus, workers)
	errorsSeen := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := store.Publish(context.Background(), publishRequest(t, destination, "same bytes"))
			if err != nil {
				errorsSeen <- err
				return
			}
			statuses <- result.Status
		}()
	}
	group.Wait()
	close(statuses)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("publish error: %v", err)
	}
	applied := 0
	already := 0
	for status := range statuses {
		switch status {
		case application.PublishApplied:
			applied++
		case application.PublishAlreadyApplied:
			already++
		default:
			t.Errorf("unexpected status %q", status)
		}
	}
	if applied != 1 || already != workers-1 {
		t.Fatalf("applied=%d already=%d", applied, already)
	}
}

func TestFileStoreVerifyChecksBytesMetadataAndProjection(t *testing.T) {
	store, _ := newTestStore(t)
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "objects/verify.bin"}
	request := publishRequest(t, destination, "verified")
	if _, err := store.Publish(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	verify := application.VerifyRequest{
		Identity: request.Identity, Destination: destination, ExpectedDigest: request.Identity.Digest,
		VerifyMetadata:   func(context.Context) (bool, error) { return true, nil },
		VerifyProjection: func(context.Context) (bool, error) { return true, nil },
	}
	result, err := store.Verify(context.Background(), verify)
	if err != nil || result.Status != application.VerificationApplied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	verify.VerifyProjection = func(context.Context) (bool, error) { return false, nil }
	result, err = store.Verify(context.Background(), verify)
	if err != nil || result.Status != application.VerificationNotApplied {
		t.Fatalf("missing projection result=%+v err=%v", result, err)
	}
	verify.VerifyProjection = func(context.Context) (bool, error) { return false, errors.New("mongo unavailable") }
	result, err = store.Verify(context.Background(), verify)
	if result.Status != application.VerificationUnknown || errorCategory(err) != application.ErrorDependency {
		t.Fatalf("dependency result=%+v err=%v", result, err)
	}
	verify.VerifyProjection = nil
	verify.ExpectedDigest = sha256.Sum256([]byte("other"))
	result, err = store.Verify(context.Background(), verify)
	if err != nil || result.Status != application.VerificationConflict {
		t.Fatalf("digest result=%+v err=%v", result, err)
	}
}

func TestFileStoreOpenReturnsOpaqueReaderAfterPathValidation(t *testing.T) {
	store, _ := newTestStore(t)
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "objects/read.bin"}
	request := publishRequest(t, destination, "read me")
	if _, err := store.Publish(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	result, err := store.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(result.Reader)
	closeErr := result.Reader.Close()
	if readErr != nil || closeErr != nil || string(data) != "read me" || result.Size != 7 {
		t.Fatalf("data=%q size=%d readErr=%v closeErr=%v", data, result.Size, readErr, closeErr)
	}
}

func TestFileStorePublicationBarrierFailureReturnsUnknownAndCanBeVerified(t *testing.T) {
	store, _ := newTestStore(t)
	if runtime.GOOS == "windows" {
		store.finalSync = func(*os.File) error { return errors.New("final sync unavailable") }
	} else {
		store.directorySync = func(*os.Root, string) error { return errors.New("directory sync unavailable") }
	}
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "asset.bin"}
	request := publishRequest(t, destination, "possibly durable")
	_, err := store.Publish(context.Background(), request)
	if errorCategory(err) != application.ErrorUnknownResult {
		t.Fatalf("publish error=%v category=%q", err, errorCategory(err))
	}
	store.finalSync = nil
	store.directorySync = func(*os.Root, string) error { return nil }
	result, verifyErr := store.Verify(context.Background(), application.VerifyRequest{
		Identity: request.Identity, Destination: destination, ExpectedDigest: request.Identity.Digest,
	})
	if verifyErr != nil || result.Status != application.VerificationApplied {
		t.Fatalf("verify=%+v err=%v", result, verifyErr)
	}
}

func TestFileStoreRejectsSymlinkInDestinationPath(t *testing.T) {
	store, config := newTestStore(t)
	outside := t.TempDir()
	link := filepath.Join(config.PrivateFiles.Data, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	destination := application.LogicalPath{Kind: application.RootPrivateFiles, Value: "linked/asset.bin"}
	_, err := store.Publish(context.Background(), publishRequest(t, destination, "blocked"))
	if errorCategory(err) != application.ErrorUnsafePath {
		t.Fatalf("publish error=%v category=%q", err, errorCategory(err))
	}
	if _, statErr := os.Stat(filepath.Join(outside, "asset.bin")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("outside destination was touched: %v", statErr)
	}
}

func TestFileStoreTemporaryArtifactSealsAndReopensBeforeResponse(t *testing.T) {
	store, config := newTestStore(t)
	artifact, err := store.CreateTemporary(context.Background(), ".archive-")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(config.Temporary)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary entries = %v, %v", entries, err)
	}
	info, err := entries[0].Info()
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("temporary mode = %v, %v", info.Mode().Perm(), err)
	}
	if _, err := artifact.Write([]byte("sealed")); err != nil {
		t.Fatal(err)
	}
	reader, size, err := artifact.Seal(context.Background())
	if err != nil || size != 6 {
		t.Fatalf("Seal() size=%d err=%v", size, err)
	}
	if _, err := artifact.Write([]byte("late")); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("write after Seal() error = %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "sealed" {
		t.Fatalf("sealed data=%q err=%v", data, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(config.Temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary artifact was not removed: %v, %v", entries, err)
	}
}

func TestFileStoreTemporaryArtifactAbortRemovesWriter(t *testing.T) {
	store, config := newTestStore(t)
	artifact, err := store.CreateTemporary(context.Background(), ".archive-")
	if err != nil {
		t.Fatal(err)
	}
	if err := artifact.Abort(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(config.Temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary artifact was not removed: %v, %v", entries, err)
	}
}

func newTestStore(t *testing.T) (*FileStore, ContentRootsConfig) {
	t.Helper()
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(roots)
	if err != nil {
		t.Fatal(err)
	}
	store.directorySync = func(*os.Root, string) error { return nil }
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store, config
}

func publishRequest(t *testing.T, destination application.LogicalPath, value string) application.PublishRequest {
	t.Helper()
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(value))
	return application.PublishRequest{
		Identity: application.AssetIdentity{
			OperationID: "upload:one", Generation: 1, OwnerID: owner,
			Kind: application.AssetImage, SourceID: "source", Digest: digest,
		},
		Destination: destination,
		Source:      strings.NewReader(value),
	}
}

func errorCategory(err error) application.ErrorCategory {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}
