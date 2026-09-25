package lea

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirRejectsNonDirectorySource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := CopyDir(source, destination); err == nil {
		t.Fatal("CopyDir accepted a regular file as a directory")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination stat error = %v, want destination to remain absent", err)
	}
}

func TestCopyDirTruncatesExistingDestinationFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "theme.json"), []byte("new"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destination, "theme.json"), []byte("old-content"), 0644); err != nil {
		t.Fatalf("write destination file: %v", err)
	}

	if err := CopyDir(source, destination); err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "theme.json"))
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(content) != "new" {
		t.Fatalf("destination content = %q, want new", content)
	}
}
