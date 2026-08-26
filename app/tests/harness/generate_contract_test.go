package harness

import (
	"os"
	"testing"
)

// TestGenerateLegacyEntrypointAndBinary is the focused, Mongo-free regression
// for the x/tools-based source generation: it runs the real generator entry
// point and builds the produced binary. It fails explicitly when the generator
// toolchain is not configured (fail-closed), matching replay semantics.
func TestGenerateLegacyEntrypointAndBinary(t *testing.T) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareGeneratedPaths(repoRoot); err != nil {
		t.Fatal(err)
	}
	binary, cleanup, err := buildServerBinary(repoRoot)
	if err != nil {
		t.Fatalf("generate + build: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("stat generated binary: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated binary is empty")
	}
}
