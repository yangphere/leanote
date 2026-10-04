package contentfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateContentRootsAcceptsDisjointSameVolumeRoots(t *testing.T) {
	config := newRootConfig(t)
	roots, err := ValidateContentRoots(config)
	if err != nil {
		t.Fatal(err)
	}
	if roots.privateFiles.data == "" || roots.privateFiles.quarantine == "" || roots.temporary == "" {
		t.Fatalf("roots were not canonicalized: %+v", roots)
	}
	for _, root := range []string{config.PrivateFiles.Data, config.PrivateFiles.Quarantine, config.PublicUpload.Data, config.PublicUpload.Quarantine, config.Temporary} {
		entries, readErr := os.ReadDir(root)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("root probe leaked files in %q: %v", root, entries)
		}
	}
}

func TestVerifyWritableDirectoryWritesReadsAndRemovesProbe(t *testing.T) {
	root := t.TempDir()
	if err := verifyWritableDirectory(root); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary root probe leaked files: %v", entries)
	}
}

func TestCleanupRootProbeReportsRemovalFailure(t *testing.T) {
	want := errors.New("remove denied")
	err := cleanupRootProbe("source", "destination", func(string) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("cleanupRootProbe() error = %v, want removal failure", err)
	}
}

func TestValidateContentRootsRejectsRelativeMissingAndOverlappingRoots(t *testing.T) {
	t.Run("relative", func(t *testing.T) {
		config := newRootConfig(t)
		config.Temporary = "relative"
		if _, err := ValidateContentRoots(config); err == nil {
			t.Fatal("relative root unexpectedly accepted")
		}
	})
	t.Run("missing", func(t *testing.T) {
		config := newRootConfig(t)
		config.Temporary = filepath.Join(t.TempDir(), "missing")
		if _, err := ValidateContentRoots(config); err == nil {
			t.Fatal("missing root unexpectedly accepted")
		}
	})
	t.Run("overlap", func(t *testing.T) {
		config := newRootConfig(t)
		config.PrivateFiles.Quarantine = mkdir(t, filepath.Join(config.PrivateFiles.Data, "quarantine"))
		if _, err := ValidateContentRoots(config); err == nil {
			t.Fatal("overlapping root unexpectedly accepted")
		}
	})
	t.Run("served quarantine", func(t *testing.T) {
		config := newRootConfig(t)
		config.ServedRoots = []string{config.PublicUpload.Quarantine}
		if _, err := ValidateContentRoots(config); err == nil {
			t.Fatal("served quarantine unexpectedly accepted")
		}
	})
}

func newRootConfig(t *testing.T) ContentRootsConfig {
	t.Helper()
	base := t.TempDir()
	return ContentRootsConfig{
		PrivateFiles: DurableRootConfig{
			Data:       mkdir(t, filepath.Join(base, "private")),
			Quarantine: mkdir(t, filepath.Join(base, "private-quarantine")),
		},
		PublicUpload: DurableRootConfig{
			Data:       mkdir(t, filepath.Join(base, "public-upload")),
			Quarantine: mkdir(t, filepath.Join(base, "public-quarantine")),
		},
		Temporary: mkdir(t, filepath.Join(base, "temporary")),
	}
}

func mkdir(t *testing.T, name string) string {
	t.Helper()
	if err := os.MkdirAll(name, 0o700); err != nil {
		t.Fatal(err)
	}
	return name
}
