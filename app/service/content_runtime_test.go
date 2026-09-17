package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/service/contentfs"
)

func TestInitializeContentStoreUsesExplicitDisjointRoots(t *testing.T) {
	base := t.TempDir()
	for _, relative := range []string{
		"files",
		filepath.Join("public", "upload"),
		".content-private-quarantine",
		".content-public-quarantine",
		".content-temporary",
	} {
		if err := os.MkdirAll(filepath.Join(base, relative), 0o700); err != nil {
			t.Fatalf("create root %q: %v", relative, err)
		}
	}
	store, err := initializeContentStore(testContentRoots(base))
	if err != nil {
		t.Fatalf("initializeContentStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, relative := range []string{
		"files",
		filepath.Join("public", "upload"),
		".content-private-quarantine",
		".content-public-quarantine",
		".content-temporary",
	} {
		info, err := os.Stat(filepath.Join(base, relative))
		if err != nil || !info.IsDir() {
			t.Fatalf("root %q info=%v err=%v", relative, info, err)
		}
	}
}

func TestInitializeContentStoreRejectsMissingConfiguredRoot(t *testing.T) {
	store, err := initializeContentStore(testContentRoots(t.TempDir()))
	if store != nil {
		t.Cleanup(func() { _ = store.Close() })
	}
	if err == nil {
		t.Fatal("initializeContentStore() succeeded without pre-created content roots")
	}
}

func TestInitializeContentStorageBuildsDeleteLifecycleFromSameRoots(t *testing.T) {
	base := t.TempDir()
	for _, relative := range []string{
		"files", filepath.Join("public", "upload"), ".content-private-quarantine", ".content-public-quarantine", ".content-temporary",
	} {
		if err := os.MkdirAll(filepath.Join(base, relative), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	store, manifests, lifecycle, temporaryRoot, err := initializeContentStorage(testContentRoots(base))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer manifests.Close()
	defer lifecycle.Close()
	if store == nil || manifests == nil || lifecycle == nil {
		t.Fatalf("initializeContentStorage() = %v, %v, %v", store, manifests, lifecycle)
	}
	if temporaryRoot != filepath.Join(base, ".content-temporary") {
		t.Fatalf("canonical temporary root = %q", temporaryRoot)
	}
}

func TestInitializeContentStorageCanonicalizesTemporaryRootOnce(t *testing.T) {
	base := t.TempDir()
	for _, relative := range []string{
		"files", filepath.Join("public", "upload"), ".content-private-quarantine", ".content-public-quarantine", ".content-temporary",
	} {
		if err := os.MkdirAll(filepath.Join(base, relative), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	config := testContentRoots(base)
	config.Temporary = filepath.Join(base, "public", "..", ".content-temporary")
	store, manifests, lifecycle, temporaryRoot, err := initializeContentStorage(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer manifests.Close()
	defer lifecycle.Close()
	want, err := filepath.EvalSymlinks(filepath.Join(base, ".content-temporary"))
	if err != nil {
		t.Fatal(err)
	}
	if temporaryRoot != filepath.Clean(want) {
		t.Fatalf("canonical temporary root = %q, want %q", temporaryRoot, want)
	}
}

func testContentRoots(base string) ContentRoots {
	return ContentRoots{
		PrivateFiles: ContentRootPair{Data: filepath.Join(base, "files"), Quarantine: filepath.Join(base, ".content-private-quarantine")},
		PublicUpload: ContentRootPair{Data: filepath.Join(base, "public", "upload"), Quarantine: filepath.Join(base, ".content-public-quarantine")},
		Temporary:    filepath.Join(base, ".content-temporary"),
		ServedRoots:  []string{filepath.Join(base, "public")},
	}
}

func TestContentStartupMaintenanceUsesFixedRetentionAndBound(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	gc := &recordingTerminalGC{result: contentfs.TerminalGCResult{Scanned: 3, Removed: []string{"first"}, Truncated: true}}
	creates := &recordingCreateRecovery{result: applicationcontent.CreateRecoveryResult{Scanned: 2, Pending: 2}}
	preNotes := &recordingAPIPreNoteRecovery{result: applicationcontent.CreateRecoveryResult{Scanned: 4, Committed: 3, Pending: 1}}
	result, err := runContentStartupMaintenance(context.Background(), gc, creates, preNotes, now)
	if err != nil {
		t.Fatal(err)
	}
	if gc.before != now || gc.limit != contentTerminalGCLimit {
		t.Fatalf("GC before=%v limit=%d", gc.before, gc.limit)
	}
	if result.DeleteTerminalGC.Scanned != 3 || len(result.DeleteTerminalGC.Removed) != 1 || !result.DeleteTerminalGC.Truncated {
		t.Fatalf("maintenance result = %+v", result)
	}
	if creates.limit != contentCreateRecoveryLimit || result.CreateRecovery.Scanned != 2 || result.CreateRecovery.Pending != 2 {
		t.Fatalf("create recovery limit=%d result=%+v", creates.limit, result.CreateRecovery)
	}
	if preNotes.limit != contentAPIPreNoteRecoveryLimit || result.APIPreNoteRecovery.Scanned != 4 || result.APIPreNoteRecovery.Committed != 3 || result.APIPreNoteRecovery.Pending != 1 {
		t.Fatalf("API pre-note recovery limit=%d result=%+v", preNotes.limit, result.APIPreNoteRecovery)
	}
}

type recordingCreateRecovery struct {
	limit  int
	result applicationcontent.CreateRecoveryResult
	err    error
}

func (recovery *recordingCreateRecovery) RecoverAbandoned(_ context.Context, limit int) (applicationcontent.CreateRecoveryResult, error) {
	recovery.limit = limit
	return recovery.result, recovery.err
}

type recordingAPIPreNoteRecovery struct {
	limit  int
	result applicationcontent.CreateRecoveryResult
	err    error
}

func (recovery *recordingAPIPreNoteRecovery) RecoverAPIPreNotes(_ context.Context, limit int) (applicationcontent.CreateRecoveryResult, error) {
	recovery.limit = limit
	return recovery.result, recovery.err
}

type recordingTerminalGC struct {
	before time.Time
	limit  int
	result contentfs.TerminalGCResult
	err    error
}

func (gc *recordingTerminalGC) GCTerminalsBounded(_ context.Context, before time.Time, limit int) (contentfs.TerminalGCResult, error) {
	gc.before, gc.limit = before, limit
	return gc.result, gc.err
}

func (gc *recordingTerminalGC) GCCreateTerminalsBounded(_ context.Context, before time.Time, limit int) (contentfs.TerminalGCResult, error) {
	if gc.before != before || gc.limit != limit {
		return contentfs.TerminalGCResult{}, errors.New("create GC input differs from delete GC")
	}
	return contentfs.TerminalGCResult{}, nil
}
