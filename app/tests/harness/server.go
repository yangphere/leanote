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
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	TestPort        = 28017
	appImportPath   = "github.com/leanote/leanote"
	revelImportPath = "github.com/revel/revel"
)

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
	server, err := startServer(repoRoot)
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

func startServer(repoRoot string) (*Server, error) {
	if err := ensureTestPortAvailable(); err != nil {
		return nil, err
	}
	binary, cleanup, err := buildServerBinary(repoRoot)
	if err != nil {
		return nil, err
	}

	goExecutable := goBinary()
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
	goExecutable := goBinary()
	if goExecutable == "" {
		return "", nil, fmt.Errorf("LEANOTE_TEST_GO is required for legacy Revel source generation; set it to a Go 1.20.x executable")
	}
	if err := prepareGeneratedPaths(repoRoot); err != nil {
		return "", nil, err
	}

	cleanupGenerated := func() {
		_ = os.RemoveAll(filepath.Join(repoRoot, "app", "tmp"))
		_ = os.RemoveAll(filepath.Join(repoRoot, "app", "routes"))
	}
	generator := exec.Command(goExecutable, "run", ".", "build", "-v", "../../", "./tmptmp")
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
	build := exec.Command(goExecutable, "build", "-o", binary, appImportPath+"/app/tmp")
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

func goBinary() string {
	return os.Getenv("LEANOTE_TEST_GO")
}

func moduleDirectory(goExecutable, modulePath, repoRoot string) (string, error) {
	command := exec.Command(goExecutable, "list", "-m", "-f={{.Dir}}", modulePath)
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
