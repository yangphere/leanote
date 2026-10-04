package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUnzipRejectsUnsafeAndDuplicateEntries(t *testing.T) {
	for name, entries := range map[string]map[string]string{
		"traversal": {"theme/../outside.txt": "x"},
		"absolute":  {"/outside.txt": "x"},
		"duplicate": {"theme/a.txt": "x", "theme//a.txt": "y"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archivePath := filepath.Join(root, "theme.zip")
			destination := filepath.Join(root, "out")
			writeTestZip(t, archivePath, entries)
			if ok, _ := Unzip(archivePath, destination); ok {
				t.Fatal("Unzip accepted unsafe or duplicate entry")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("destination still exists after rejection: %v", err)
			}
		})
	}
}

func TestUnzipStripsSingleTopLevelFolderAndKeepsRootSafe(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "theme.zip")
	destination := filepath.Join(root, "out")
	writeTestZip(t, archivePath, map[string]string{
		"theme/theme.json":   "{}",
		"theme/images/a.txt": "image",
	})
	if ok, msg := Unzip(archivePath, destination); !ok {
		t.Fatalf("Unzip() = false, %s", msg)
	}
	if _, err := os.Stat(filepath.Join(destination, "theme.json")); err != nil {
		t.Fatal("top-level folder was not removed")
	}
	if _, err := os.Stat(filepath.Join(destination, "images", "a.txt")); err != nil {
		t.Fatal("nested file was not extracted")
	}
}

func TestUnzipStripsSingleTopLevelFolderWithExplicitDirectoryEntry(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "theme.zip")
	destination := filepath.Join(root, "out")
	writeTestZip(t, archivePath, map[string]string{
		"theme/":             "",
		"theme/theme.json":   "{}",
		"theme/images/a.txt": "image",
	})
	if ok, msg := Unzip(archivePath, destination); !ok {
		t.Fatalf("Unzip() = false, %s", msg)
	}
	if _, err := os.Stat(filepath.Join(destination, "theme.json")); err != nil {
		t.Fatal("top-level folder was not removed")
	}
}

func TestUnzipCountsActualExpandedBytes(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "theme.zip")
	destination := filepath.Join(root, "out")
	writeTestZip(t, archivePath, map[string]string{
		"theme/large.txt": strings.Repeat("x", int(maxArchiveFileBytes)+1),
	})
	if ok, _ := Unzip(archivePath, destination); ok {
		t.Fatal("Unzip accepted a file over the actual expanded-byte budget")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination still exists after budget rejection: %v", err)
	}
}
