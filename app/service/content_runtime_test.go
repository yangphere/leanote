package service

import (
	"os"
	"path/filepath"
	"testing"
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
	store, err := initializeContentStore(base)
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
	store, err := initializeContentStore(t.TempDir())
	if store != nil {
		t.Cleanup(func() { _ = store.Close() })
	}
	if err == nil {
		t.Fatal("initializeContentStore() succeeded without pre-created content roots")
	}
}
