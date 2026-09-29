//go:build !windows

package contentfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveOpenedLifecycleFileRemovesVerifiedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := root.Open("a.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := removeOpenedLifecycleFile(root, "a.bin", file); err != nil {
		t.Fatalf("removeOpenedLifecycleFile() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("directory after removal: entries=%v err=%v", entries, err)
	}
}

func TestRemoveOpenedLifecycleFileKeepsReplacedName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(path, []byte("verified"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := root.Open("a.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// Replace the name after the handle was verified.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeOpenedLifecycleFile(root, "a.bin", file); err == nil {
		t.Fatal("removeOpenedLifecycleFile() removed a replaced name")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement after refused removal: data=%q err=%v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory after refused removal: entries=%v err=%v", entries, err)
	}
}
