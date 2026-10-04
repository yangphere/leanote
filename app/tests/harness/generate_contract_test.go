package harness

import (
	"os"
	"testing"
)

// TestBuildNativeEntrypoint is the focused, Mongo-free regression for the
// first-party command entrypoint.
func TestBuildNativeEntrypoint(t *testing.T) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	binary, cleanup, err := buildNativeServerBinary(repoRoot)
	if err != nil {
		t.Fatalf("build native entrypoint: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("stat native binary: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated binary is empty")
	}
}
