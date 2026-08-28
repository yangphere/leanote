package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	TestPort        = 28017
	appImportPath   = "github.com/yangphere/leanote"
	revelImportPath = "github.com/revel/revel"
)

// minGeneratorVersion is the lowest Go toolchain accepted when the harness
// resolves the generator from PATH. LEANOTE_TEST_GO bypasses this floor
// because it is always an explicit maintainer decision.
var minGeneratorVersion = goVersion{major: 1, minor: 26, patch: 7}

type Server struct {
	BaseURL   string
	process   *exec.Cmd
	done      chan error
	cleanup   func()
	closeOnce sync.Once
	closeErr  error
}

func StartServer(t testing.TB) *Server {
	t.Helper()
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	server, err := startServer(repoRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return server
}

// StartServerProcess starts the application under test in Revel "test" run
// mode without a *testing.T handle, for long-lived supervisors such as the
// E2E harness command (app/tests/harness/cmd/e2e). The child process
// inherits the caller's environment, including LEANOTE_E2E_RUN_TOKEN.
func StartServerProcess() (*Server, error) {
	return StartServerProcessWithRegistration(nil)
}

// StartServerProcessWithRegistration starts the application under test in
// Revel "test" run mode and invokes register immediately after the process
// starts, before the readiness probe begins. Supervisors use this hook to
// publish the live server handle before any blocking startup work, ensuring
// an interrupt during readiness still tears the process down.
func StartServerProcessWithRegistration(register func(*Server)) (*Server, error) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		return nil, err
	}
	return startServer(repoRoot, register)
}

// RepositoryRoot resolves the repository root from the working directory.
func RepositoryRoot() (string, error) {
	return findRepositoryRoot()
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		if s.process != nil && s.process.Process != nil {
			if err := s.process.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) && !strings.Contains(err.Error(), "process already finished") {
				s.closeErr = err
			}
			if s.done != nil {
				<-s.done
			}
		}
		if s.cleanup != nil {
			s.cleanup()
		}
	})
	return s.closeErr
}

func startServer(repoRoot string, register func(*Server)) (*Server, error) {
	if err := ensureTestPortAvailable(); err != nil {
		return nil, err
	}
	binary, cleanup, err := buildServerBinary(repoRoot)
	if err != nil {
		return nil, err
	}

	goExecutable, err := goBinary()
	if err != nil {
		cleanup()
		return nil, err
	}
	revelPath, err := moduleDirectory(goExecutable, revelImportPath, repoRoot)
	if err != nil {
		cleanup()
		return nil, err
	}
	runMode, err := serverRunMode(repoRoot, revelPath)
	if err != nil {
		cleanup()
		return nil, err
	}

	logFile, err := ioutil.TempFile(filepath.Dir(binary), "server-*.log")
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create server log: %w", err)
	}
	command := exec.Command(binary,
		"-importPath="+appImportPath,
		"-runMode="+runMode,
		fmt.Sprintf("-port=%d", TestPort),
	)
	command.Dir = repoRoot
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		cleanup()
		return nil, fmt.Errorf("start test server: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()

	server := &Server{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", TestPort),
		process: command,
		done:    done,
		cleanup: func() {
			_ = logFile.Close()
			cleanup()
		},
	}
	if register != nil {
		register(server)
	}
	if err := server.waitForReady(logFile.Name()); err != nil {
		_ = server.Close()
		return nil, err
	}
	return server, nil
}

func (s *Server) waitForReady(logPath string) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(s.BaseURL + "/login")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case err := <-s.done:
			s.done <- err
			logOutput, _ := ioutil.ReadFile(logPath)
			return fmt.Errorf("test server exited before becoming ready: %v; log:\n%s", err, string(logOutput))
		default:
		}
		time.Sleep(250 * time.Millisecond)
	}
	logOutput, _ := ioutil.ReadFile(logPath)
	return fmt.Errorf("test server did not become ready at %s; log:\n%s", s.BaseURL, string(logOutput))
}

func buildServerBinary(repoRoot string) (string, func(), error) {
	goExecutable, err := goBinary()
	if err != nil {
		return "", nil, err
	}
	if err := prepareGeneratedPaths(repoRoot); err != nil {
		return "", nil, err
	}

	cleanupGenerated := func() {
		_ = os.RemoveAll(filepath.Join(repoRoot, "app", "tmp"))
		_ = os.RemoveAll(filepath.Join(repoRoot, "app", "routes"))
	}
	generator := goCommand(goExecutable, "run", ".", "build", "-v", "../../", "./tmptmp")
	generator.Dir = filepath.Join(repoRoot, "app", "cmd")
	if output, err := generator.CombinedOutput(); err != nil {
		cleanupGenerated()
		return "", nil, commandError("generate Revel test entrypoint", output, err)
	}
	defer func() {
		_ = os.RemoveAll(filepath.Join(repoRoot, "app", "cmd", "tmptmp"))
	}()

	tempDir, err := ioutil.TempDir("", "leanote-regression-server-")
	if err != nil {
		cleanupGenerated()
		return "", nil, fmt.Errorf("create test binary directory: %w", err)
	}
	binary := filepath.Join(tempDir, "leanote")
	if strings.EqualFold(filepath.Ext(goExecutable), ".exe") {
		binary += ".exe"
	}
	build := goCommand(goExecutable, "build", "-o", binary, appImportPath+"/app/tmp")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		cleanupGenerated()
		_ = os.RemoveAll(tempDir)
		return "", nil, commandError("build Revel test server", output, err)
	}
	return binary, func() {
		cleanupGenerated()
		_ = os.RemoveAll(tempDir)
	}, nil
}

func prepareGeneratedPaths(repoRoot string) error {
	for _, generated := range []string{
		filepath.Join(repoRoot, "app", "tmp"),
		filepath.Join(repoRoot, "app", "routes"),
	} {
		empty, err := emptyDirectoryTree(generated)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect generated path %s: %w", generated, err)
		}
		if !empty {
			return fmt.Errorf("refusing to overwrite existing generated path %s", generated)
		}
		if err := os.RemoveAll(generated); err != nil {
			return fmt.Errorf("remove empty generated path %s: %w", generated, err)
		}
	}
	return nil
}

func emptyDirectoryTree(path string) (bool, error) {
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return false, nil
		}
		empty, err := emptyDirectoryTree(filepath.Join(path, entry.Name()))
		if err != nil || !empty {
			return empty, err
		}
	}
	return true, nil
}

func serverRunMode(appPath, revelPath string) (string, error) {
	payload := struct {
		Mode           string            `json:"mode"`
		PackagePathMap map[string]string `json:"packagePathMap"`
	}{
		Mode: "test",
		PackagePathMap: map[string]string{
			appImportPath:   appPath,
			revelImportPath: revelPath,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode Revel module path map: %w", err)
	}
	return string(encoded), nil
}

func ensureTestPortAvailable() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", TestPort))
	if err != nil {
		return fmt.Errorf("fixed regression test port %d is unavailable: %w", TestPort, err)
	}
	return listener.Close()
}

// goBinary resolves the Go executable used for legacy Revel source generation
// and server builds. LEANOTE_TEST_GO is an explicit override honored verbatim;
// otherwise PATH must provide "go" with a version at or above the minimum,
// verified before anything is generated (fail closed). Unreadable versions are
// rejected instead of assumed compatible.
func goBinary() (string, error) {
	if override := os.Getenv("LEANOTE_TEST_GO"); override != "" {
		return override, nil
	}
	executable, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("no default Go toolchain found on PATH for legacy Revel source generation; install Go %s or newer and make sure 'go' is on PATH, or set LEANOTE_TEST_GO to a Go toolchain executable: %w", minGeneratorVersion, err)
	}
	output, err := goCommand(executable, "env", "GOVERSION").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query version of default Go toolchain %s: %w\n%s", executable, err, strings.TrimSpace(string(output)))
	}
	version, parseErr := parseGoVersion(string(output))
	if parseErr != nil {
		return "", fmt.Errorf("default Go toolchain %s reported unreadable version %q; install Go %s or newer, or set LEANOTE_TEST_GO to a known Go toolchain executable: %v", executable, strings.TrimSpace(string(output)), minGeneratorVersion, parseErr)
	}
	if !version.atLeast(minGeneratorVersion) {
		return "", fmt.Errorf("default Go toolchain %s is go%s, but legacy Revel source generation requires Go %s or newer; upgrade the Go on PATH or set LEANOTE_TEST_GO to a suitable toolchain executable", executable, version, minGeneratorVersion)
	}
	return executable, nil
}

// goCommand wraps exec.Command for every harness-spawned Go subprocess and pins
// GOTOOLCHAIN=local so an old or mismatched toolchain can never satisfy itself
// by downloading another one.
func goCommand(goExecutable string, args ...string) *exec.Cmd {
	command := exec.Command(goExecutable, args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOTOOLCHAIN=") {
			continue
		}
		env = append(env, entry)
	}
	command.Env = append(env, "GOTOOLCHAIN=local")
	return command
}

type goVersion struct {
	major int
	minor int
	patch int
}

func (v goVersion) String() string {
	return strconv.Itoa(v.major) + "." + strconv.Itoa(v.minor) + "." + strconv.Itoa(v.patch)
}

func (v goVersion) atLeast(floor goVersion) bool {
	if v.major != floor.major {
		return v.major > floor.major
	}
	if v.minor != floor.minor {
		return v.minor > floor.minor
	}
	return v.patch >= floor.patch
}

// parseGoVersion extracts the toolchain version from `go env GOVERSION` output
// such as "go1.27.0". Development builds ("devel ...") and pre-releases
// ("go1.26rc1") fail closed because their generator compatibility is unknown.
func parseGoVersion(text string) (goVersion, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return goVersion{}, fmt.Errorf("empty version output")
	}
	original := fields[0]
	version := strings.TrimPrefix(original, "go")
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return goVersion{}, fmt.Errorf("unsupported version format %q", original)
	}
	numbers := [3]int{}
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return goVersion{}, fmt.Errorf("unsupported version format %q", original)
		}
		numbers[index] = value
	}
	return goVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}, nil
}

func moduleDirectory(goExecutable, modulePath, repoRoot string) (string, error) {
	command := goCommand(goExecutable, "list", "-m", "-f={{.Dir}}", modulePath)
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return "", commandError("locate "+modulePath, output, err)
	}
	directory := strings.TrimSpace(string(output))
	if directory == "" {
		return "", fmt.Errorf("locate %s: empty module directory", modulePath)
	}
	return directory, nil
}

func commandError(action string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	if len(message) > 4096 {
		message = message[len(message)-4096:]
	}
	return fmt.Errorf("%s: %w\n%s", action, err, message)
}

func findRepositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("could not find repository root from %s", directory)
		}
		directory = parent
	}
}
