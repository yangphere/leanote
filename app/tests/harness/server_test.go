package harness

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerRunModeMapsApplicationAndRevelPaths(t *testing.T) {
	encoded, err := serverRunMode(`D:\work\leanote`, `C:\mod\github.com\revel\revel@v1.0.0`)
	if err != nil {
		t.Fatalf("serverRunMode() error = %v", err)
	}

	var got struct {
		Mode           string            `json:"mode"`
		PackagePathMap map[string]string `json:"packagePathMap"`
	}
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		t.Fatalf("serverRunMode() output is not JSON: %v", err)
	}
	if got.Mode != "test" {
		t.Fatalf("serverRunMode() mode = %q, want test", got.Mode)
	}
	if got.PackagePathMap[appImportPath] != `D:\work\leanote` || got.PackagePathMap[revelImportPath] != `C:\mod\github.com\revel\revel@v1.0.0` {
		t.Fatalf("serverRunMode() map = %#v", got.PackagePathMap)
	}
}

func TestEnsureTestPortAvailableRejectsOccupiedFixedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:28017")
	if err != nil {
		t.Skipf("fixed test port is already occupied: %v", err)
	}
	defer listener.Close()

	err = ensureTestPortAvailable()
	if err == nil || !strings.Contains(err.Error(), "28017") {
		t.Fatalf("ensureTestPortAvailable() error = %v, want fixed-port error", err)
	}
}

func TestGoBinaryRequiresExplicitLegacyGeneratorToolchain(t *testing.T) {
	t.Setenv("LEANOTE_TEST_GO", "C:/tools/go1.20.14/bin/go.exe")
	if got := goBinary(); got != "C:/tools/go1.20.14/bin/go.exe" {
		t.Fatalf("goBinary() = %q", got)
	}
}

func TestPrepareGeneratedPathsRemovesOnlyEmptyDirectories(t *testing.T) {
	repoRoot := t.TempDir()
	for _, path := range []string{
		filepath.Join(repoRoot, "app", "tmp"),
		filepath.Join(repoRoot, "app", "routes"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "app", "tmp", "run"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := prepareGeneratedPaths(repoRoot); err != nil {
		t.Fatalf("prepareGeneratedPaths() error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(repoRoot, "app", "tmp"),
		filepath.Join(repoRoot, "app", "routes"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("prepareGeneratedPaths() left %s: %v", path, err)
		}
	}
}

func TestPrepareGeneratedPathsRejectsExistingFiles(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "app", "tmp")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "main.go"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := prepareGeneratedPaths(repoRoot)
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("prepareGeneratedPaths() error = %v", err)
	}
}

func TestServerServesLoginOverRealHTTP(t *testing.T) {
	if os.Getenv("LEANOTE_HTTP_INTEGRATION") != "1" {
		t.Skip("set LEANOTE_HTTP_INTEGRATION=1 to run the real server smoke test")
	}
	server := StartServer(t)
	response, err := http.Get(server.BaseURL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /login status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}
