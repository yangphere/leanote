package harness

import (
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
	TestPort = 28017
)

// minNativeToolchainVersion is the lowest Go toolchain accepted when the
// harness resolves the native entrypoint builder from PATH. LEANOTE_TEST_GO
// bypasses this floor because it is always an explicit maintainer decision.
var minNativeToolchainVersion = goVersion{major: 1, minor: 26, patch: 7}

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
	server, err := startNativeServer(repoRoot, nil)
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

// StartHTTPServer starts the first-party cmd/leanote test entrypoint.
func StartHTTPServer(t testing.TB) *Server {
	t.Helper()
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	server, err := startNativeServer(repoRoot, nil)
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

// StartServerProcess starts the first-party cmd/leanote test entrypoint
// without a *testing.T handle, for long-lived supervisors such as the E2E
// harness command. The child process inherits the caller's environment,
// including LEANOTE_E2E_RUN_TOKEN.
func StartServerProcess() (*Server, error) {
	return StartServerProcessWithRegistration(nil)
}

// StartServerProcessWithRegistration starts cmd/leanote -runMode test and
// invokes register immediately after the process starts, before the readiness
// probe begins. Supervisors use this hook to publish the live server handle
// before any blocking startup work, ensuring an interrupt during readiness
// still tears the process down.
func StartServerProcessWithRegistration(register func(*Server)) (*Server, error) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		return nil, err
	}
	return startNativeServer(repoRoot, register)
}

// StartHTTPServerProcess starts cmd/leanote -runMode test without generating
// an entrypoint. It is retained as an explicit alias for focused callers.
func StartHTTPServerProcess() (*Server, error) {
	repoRoot, err := findRepositoryRoot()
	if err != nil {
		return nil, err
	}
	return startNativeServer(repoRoot, nil)
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

func startNativeServer(repoRoot string, register func(*Server)) (*Server, error) {
	if err := ensureTestPortAvailable(); err != nil {
		return nil, err
	}
	binary, cleanup, err := buildNativeServerBinary(repoRoot)
	if err != nil {
		return nil, err
	}
	logFile, err := ioutil.TempFile(filepath.Dir(binary), "native-server-*.log")
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create native server log: %w", err)
	}
	command := exec.Command(binary, "-runMode=test", "-conf", filepath.Join(repoRoot, "conf", "app.conf"))
	command.Dir = repoRoot
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		cleanup()
		return nil, fmt.Errorf("start native test server: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	server := &Server{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", TestPort),
		process: command,
		done:    done,
		cleanup: func() { _ = logFile.Close(); cleanup() },
	}
	if register != nil {
		register(server)
	}
	if err := server.waitForNativeReady(logFile.Name()); err != nil {
		_ = server.Close()
		return nil, err
	}
	return server, nil
}

func buildNativeServerBinary(repoRoot string) (string, func(), error) {
	goExecutable, err := goBinary()
	if err != nil {
		return "", nil, err
	}
	tempDir, err := ioutil.TempDir("", "leanote-native-test-server-")
	if err != nil {
		return "", nil, fmt.Errorf("create native test binary directory: %w", err)
	}
	binary := filepath.Join(tempDir, "leanote")
	if strings.EqualFold(filepath.Ext(goExecutable), ".exe") {
		binary += ".exe"
	}
	build := goCommand(goExecutable, "build", "-o", binary, "./cmd/leanote")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, commandError("build native test server", output, err)
	}
	return binary, func() { _ = os.RemoveAll(tempDir) }, nil
}

func (s *Server) waitForNativeReady(logPath string) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(s.BaseURL + "/healthz")
		if err == nil {
			status := response.StatusCode
			_ = response.Body.Close()
			if status == http.StatusOK {
				return nil
			}
		}
		select {
		case err := <-s.done:
			s.done <- err
			logOutput, _ := ioutil.ReadFile(logPath)
			return fmt.Errorf("native test server exited before becoming reachable: %v; log:\n%s", err, string(logOutput))
		default:
		}
		time.Sleep(250 * time.Millisecond)
	}
	logOutput, _ := ioutil.ReadFile(logPath)
	return fmt.Errorf("native test server did not become reachable at %s; log:\n%s", s.BaseURL, string(logOutput))
}

func ensureTestPortAvailable() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", TestPort))
	if err != nil {
		return fmt.Errorf("fixed regression test port %d is unavailable: %w", TestPort, err)
	}
	return listener.Close()
}

// goBinary resolves the Go executable used for native entrypoint builds.
// LEANOTE_TEST_GO is an explicit override honored verbatim;
// otherwise PATH must provide "go" with a version at or above the minimum,
// verified before anything is built (fail closed). Unreadable versions are
// rejected instead of assumed compatible.
func goBinary() (string, error) {
	if override := os.Getenv("LEANOTE_TEST_GO"); override != "" {
		return override, nil
	}
	executable, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("no default Go toolchain found on PATH for native entrypoint build; install Go %s or newer and make sure 'go' is on PATH, or set LEANOTE_TEST_GO to a Go toolchain executable: %w", minNativeToolchainVersion, err)
	}
	output, err := goCommand(executable, "env", "GOVERSION").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query version of default Go toolchain %s: %w\n%s", executable, err, strings.TrimSpace(string(output)))
	}
	version, parseErr := parseGoVersion(string(output))
	if parseErr != nil {
		return "", fmt.Errorf("default Go toolchain %s reported unreadable version %q; install Go %s or newer, or set LEANOTE_TEST_GO to a known Go toolchain executable: %v", executable, strings.TrimSpace(string(output)), minNativeToolchainVersion, parseErr)
	}
	if !version.atLeast(minNativeToolchainVersion) {
		return "", fmt.Errorf("default Go toolchain %s is go%s, but native entrypoint build requires Go %s or newer; upgrade the Go on PATH or set LEANOTE_TEST_GO to a suitable toolchain executable", executable, version, minNativeToolchainVersion)
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
// ("go1.26rc1") fail closed because their native entrypoint compatibility is
// unknown.
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
