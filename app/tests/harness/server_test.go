package harness

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestGoBinaryHonorsExplicitOverride locks the pass-through contract: an
// explicitly provided LEANOTE_TEST_GO is used verbatim and never version-checked.
func TestGoBinaryHonorsExplicitOverride(t *testing.T) {
	override := filepath.Join("C:", "tools", "custom-go", "bin", "go.exe")
	t.Setenv("LEANOTE_TEST_GO", override)
	got, err := goBinary()
	if err != nil {
		t.Fatalf("goBinary() error = %v", err)
	}
	if got != override {
		t.Fatalf("goBinary() = %q, want the explicit override %q", got, override)
	}
}

// TestGoBinaryRejectsDefaultToolchainBelowFloor locks the fail-closed floor:
// a default PATH go older than 1.26.7 fails before any generation with an
// error naming the required minimum.
func TestGoBinaryRejectsDefaultToolchainBelowFloor(t *testing.T) {
	stubDir := buildGoVersionStub(t, "go1.25.9")
	t.Setenv("LEANOTE_TEST_GO", "")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := goBinary()
	if err == nil {
		t.Fatalf("goBinary() = %q, want an explicit below-floor failure", got)
	}
	if !strings.Contains(err.Error(), minGeneratorVersion.String()) {
		t.Fatalf("goBinary() error = %v, want it to name the %s floor", err, minGeneratorVersion)
	}
	if !strings.Contains(err.Error(), "LEANOTE_TEST_GO") {
		t.Fatalf("goBinary() error = %v, want install/override guidance", err)
	}
}

// TestGoBinaryAcceptsDefaultToolchainAtFloor locks the boundary-inclusive
// behavior: exactly go1.26.7 on PATH passes without any override.
func TestGoBinaryAcceptsDefaultToolchainAtFloor(t *testing.T) {
	stubDir := buildGoVersionStub(t, "go1.26.7")
	t.Setenv("LEANOTE_TEST_GO", "")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := goBinary()
	if err != nil {
		t.Fatalf("goBinary() error = %v", err)
	}
	want := filepath.Join(stubDir, stubExecutableName())
	if got != want {
		t.Fatalf("goBinary() = %q, want stub %q", got, want)
	}
}

// TestGoBinaryRejectsUnreadableDefaultVersionOutput keeps unknown formats
// fail-closed instead of assuming compatibility.
func TestGoBinaryRejectsUnreadableDefaultVersionOutput(t *testing.T) {
	stubDir := buildGoVersionStub(t, "devel +f00ba12")
	t.Setenv("LEANOTE_TEST_GO", "")
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := goBinary(); err == nil || !strings.Contains(err.Error(), "unreadable version") {
		t.Fatalf("goBinary() error = %v, want unreadable-version rejection", err)
	}
}

// TestGoBinaryResolvesRealSystemToolchain exercises the default resolution
// against the actual host toolchain; the blocking CI matrix guarantees that
// toolchain satisfies the floor.
func TestGoBinaryResolvesRealSystemToolchain(t *testing.T) {
	t.Setenv("LEANOTE_TEST_GO", "")
	got, err := goBinary()
	if err != nil {
		t.Fatalf("goBinary() error = %v", err)
	}
	if got == "" {
		t.Fatal("goBinary() returned an empty executable")
	}
	output, err := goCommand(got, "env", "GOVERSION").CombinedOutput()
	if err != nil {
		t.Fatalf("%s env GOVERSION: %v: %s", got, err, strings.TrimSpace(string(output)))
	}
	version, err := parseGoVersion(string(output))
	if err != nil {
		t.Fatalf("parse system toolchain version: %v", err)
	}
	if !version.atLeast(minGeneratorVersion) {
		t.Fatalf("system toolchain go%s is below the %s floor", version, minGeneratorVersion)
	}
}

// TestGoCommandPinsLocalToolchain proves every harness-spawned Go subprocess
// runs with GOTOOLCHAIN=local and cannot be overridden by inherited values.
func TestGoCommandPinsLocalToolchain(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto")
	command := goCommand("go", "version")
	pinned := false
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "GOTOOLCHAIN=") && entry != "GOTOOLCHAIN=local" {
			t.Fatalf("goCommand() environment contains conflicting entry %q", entry)
		}
		if entry == "GOTOOLCHAIN=local" {
			pinned = true
		}
	}
	if !pinned {
		t.Fatal("goCommand() environment does not pin GOTOOLCHAIN=local")
	}
}

func TestParseGoVersionFormats(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    goVersion
		wantErr bool
	}{
		{name: "patch release", text: "go1.26.7", want: goVersion{major: 1, minor: 26, patch: 7}},
		{name: "minor only", text: "go1.27", want: goVersion{major: 1, minor: 27}},
		{name: "surrounding whitespace", text: "  go1.27.0\n", want: goVersion{major: 1, minor: 27}},
		{name: "release candidate rejected", text: "go1.26rc1", wantErr: true},
		{name: "development build rejected", text: "devel +8a3fc2b", wantErr: true},
		{name: "empty output rejected", text: "", wantErr: true},
		{name: "garbage rejected", text: "not-a-version", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseGoVersion(test.text)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseGoVersion(%q) = %v, want error", test.text, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoVersion(%q) error = %v", test.text, err)
			}
			if got != test.want {
				t.Fatalf("parseGoVersion(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestGoVersionFloorComparison(t *testing.T) {
	tests := []struct {
		name  string
		have  goVersion
		floor goVersion
		pass  bool
	}{
		{name: "equal to floor", have: goVersion{major: 1, minor: 26, patch: 7}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: true},
		{name: "patch newer", have: goVersion{major: 1, minor: 26, patch: 9}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: true},
		{name: "patch older", have: goVersion{major: 1, minor: 26, patch: 6}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: false},
		{name: "minor newer", have: goVersion{major: 1, minor: 27, patch: 0}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: true},
		{name: "minor older", have: goVersion{major: 1, minor: 25, patch: 9}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: false},
		{name: "major newer", have: goVersion{major: 2, minor: 0, patch: 0}, floor: goVersion{major: 1, minor: 26, patch: 7}, pass: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.have.atLeast(test.floor); got != test.pass {
				t.Fatalf("atLeast(%v) with floor %v = %v, want %v", test.have, test.floor, got, test.pass)
			}
		})
	}
}

// buildGoVersionStub compiles a minimal stand-in for the `go` executable whose
// `env GOVERSION` output is fixed. A compiled binary (instead of a shell
// script) keeps the stub working identically on Windows and Unix, and the
// directory containing it can be prepended to PATH for default-resolution tests.
func buildGoVersionStub(t *testing.T, versionOutput string) string {
	t.Helper()
	stubDir := t.TempDir()
	sourcePath := filepath.Join(stubDir, "stub_main.go")
	source := `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "env" && os.Args[2] == "GOVERSION" {
		fmt.Println(` + "`" + versionOutput + "`" + `)
		return
	}
	fmt.Fprintf(os.Stderr, "unexpected stub invocation: %v\n", os.Args)
	os.Exit(1)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command(hostGoExecutable(t), "build", "-o", filepath.Join(stubDir, stubExecutableName()), sourcePath)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build version stub: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return stubDir
}

func stubExecutableName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

// hostGoExecutable returns the Go toolchain that compiled this test binary so
// stub compilation never depends on the (possibly mutated) PATH.
func hostGoExecutable(t testing.TB) string {
	t.Helper()
	executable := filepath.Join(runtime.GOROOT(), "bin", stubExecutableName())
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("locate host Go toolchain: %v", err)
	}
	return executable
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
