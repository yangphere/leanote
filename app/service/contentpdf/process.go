// Package contentpdf owns local renderer process execution.  Controllers and
// API adapters receive only the content application service and never build a
// command line or callback URL.
package contentpdf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	application "github.com/yangphere/leanote/app/application/content"
)

const maxRendererDiagnosticBytes = 32 * 1024

type ProcessConfig struct {
	Executable    string
	TemporaryRoot string
	PolicyID      string
}

type processRunner func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

type ProcessBackend struct {
	executable    string
	temporaryRoot string
	policyID      string
	executableID  string
	executableRef fs.FileInfo
	run           processRunner
}

// ConfiguredBackend resolves the administrator-owned executable setting for
// each descriptor request. It retains only the matching validated backend so
// a config change invalidates capabilities issued for the previous binary.
type ConfiguredBackend struct {
	executable    func() string
	temporaryRoot string
	policyID      string
	mu            sync.RWMutex
	current       *ProcessBackend
}

func NewConfiguredBackend(executable func() string, temporaryRoot, policyID string) *ConfiguredBackend {
	return &ConfiguredBackend{executable: executable, temporaryRoot: temporaryRoot, policyID: policyID}
}

func (backend *ConfiguredBackend) Descriptor(ctx context.Context) (application.RendererDescriptor, error) {
	if backend == nil || backend.executable == nil {
		return application.RendererDescriptor{}, application.NewError(application.ErrorDependency, "renderer_config_unavailable", nil)
	}
	process, err := NewProcessBackend(ProcessConfig{
		Executable: backend.executable(), TemporaryRoot: backend.temporaryRoot, PolicyID: backend.policyID,
	})
	if err != nil {
		return application.RendererDescriptor{}, err
	}
	descriptor, err := process.Descriptor(ctx)
	if err != nil {
		return application.RendererDescriptor{}, err
	}
	backend.mu.Lock()
	backend.current = process
	backend.mu.Unlock()
	return descriptor, nil
}

func (backend *ConfiguredBackend) Render(ctx context.Context, descriptor application.RendererDescriptor, document []byte, maxOutputBytes int64) ([]byte, error) {
	backend.mu.RLock()
	process := backend.current
	backend.mu.RUnlock()
	if process == nil {
		return nil, application.NewError(application.ErrorConflict, "renderer_descriptor_unknown", nil)
	}
	configured, _, err := validateExecutable(backend.executable())
	if err != nil || !samePath(configured, process.executable) {
		return nil, application.NewError(application.ErrorConflict, "renderer_configuration_changed", err)
	}
	return process.Render(ctx, descriptor, document, maxOutputBytes)
}

func NewProcessBackend(config ProcessConfig) (*ProcessBackend, error) {
	if strings.TrimSpace(config.PolicyID) == "" {
		return nil, application.NewError(application.ErrorValidation, "renderer_policy_missing", nil)
	}
	executable, info, err := validateExecutable(config.Executable)
	if err != nil {
		return nil, err
	}
	temporaryRoot, err := validateTemporaryRoot(config.TemporaryRoot)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		config.PolicyID, executable, fmt.Sprintf("%d", info.Size()), fmt.Sprintf("%d", info.ModTime().UnixNano()),
	}, "\x00")))
	backend := &ProcessBackend{
		executable: executable, temporaryRoot: temporaryRoot, policyID: config.PolicyID,
		executableID: hex.EncodeToString(digest[:16]), executableRef: info,
	}
	backend.run = backend.runProcess
	return backend, nil
}

func (backend *ProcessBackend) Descriptor(context.Context) (application.RendererDescriptor, error) {
	if err := backend.revalidateExecutable(); err != nil {
		return application.RendererDescriptor{}, err
	}
	return application.RendererDescriptor{PolicyID: backend.policyID, ExecutableID: backend.executableID}, nil
}

func (backend *ProcessBackend) Render(ctx context.Context, descriptor application.RendererDescriptor, document []byte, maxOutputBytes int64) (pdf []byte, resultErr error) {
	if descriptor.PolicyID != backend.policyID || descriptor.ExecutableID != backend.executableID {
		return nil, application.NewError(application.ErrorConflict, "renderer_descriptor_changed", nil)
	}
	if maxOutputBytes <= 0 {
		return nil, application.NewError(application.ErrorValidation, "renderer_output_limit", nil)
	}
	if err := application.ValidateSelfContainedPDFDocument(document); err != nil {
		return nil, err
	}
	if err := backend.revalidateExecutable(); err != nil {
		return nil, err
	}
	requestDir, err := os.MkdirTemp(backend.temporaryRoot, "leanote-pdf-")
	if err != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "renderer_temp_create", err)
	}
	if err := os.Chmod(requestDir, 0o700); err != nil {
		_ = os.RemoveAll(requestDir)
		return nil, application.NewError(application.ErrorStorageUnavailable, "renderer_temp_mode", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(requestDir); cleanupErr != nil {
			pdf = nil
			resultErr = application.NewError(application.ErrorUnknownResult, "renderer_temp_cleanup", errors.Join(resultErr, cleanupErr))
		}
	}()

	outputPath := filepath.Join(requestDir, "artifact.pdf")
	args := []string{
		"--quiet", "--lowquality", "--disable-local-file-access",
		"--enable-javascript", "--window-status", "done", "-", outputPath,
	}
	stdout := &boundedDiagnostic{limit: maxRendererDiagnosticBytes}
	stderr := &boundedDiagnostic{limit: maxRendererDiagnosticBytes}
	if err := backend.run(ctx, backend.executable, args, strings.NewReader(string(document)), stdout, stderr); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, application.NewError(application.ErrorTimeout, "renderer_process_timeout", errors.Join(err, ctx.Err()))
		}
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_process_failed", err)
	}
	root, err := os.OpenRoot(requestDir)
	if err != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "renderer_output_root", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil && resultErr == nil {
			pdf = nil
			resultErr = application.NewError(application.ErrorStorageUnavailable, "renderer_output_root_close", closeErr)
		}
	}()
	info, err := root.Lstat("artifact.pdf")
	if err != nil {
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_output_missing", err)
	}
	if isIndirection(info) || !info.Mode().IsRegular() {
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_output_unsafe", nil)
	}
	if info.Size() <= 0 || info.Size() > maxOutputBytes {
		return nil, application.NewError(application.ErrorTooLarge, "renderer_output_limit", nil)
	}
	file, err := root.Open("artifact.pdf")
	if err != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "renderer_output_open", err)
	}
	pdf, readErr := io.ReadAll(io.LimitReader(file, maxOutputBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, application.NewError(application.ErrorStorageUnavailable, "renderer_output_read", errors.Join(readErr, closeErr))
	}
	if int64(len(pdf)) > maxOutputBytes {
		return nil, application.NewError(application.ErrorTooLarge, "renderer_output_limit", nil)
	}
	return pdf, nil
}

func (backend *ProcessBackend) revalidateExecutable() error {
	info, err := os.Lstat(backend.executable)
	if err != nil {
		return application.NewError(application.ErrorDependency, "renderer_executable_unavailable", err)
	}
	if isIndirection(info) || !isExecutable(info, backend.executable) || !os.SameFile(info, backend.executableRef) {
		return application.NewError(application.ErrorConflict, "renderer_executable_changed", nil)
	}
	canonical, err := filepath.EvalSymlinks(backend.executable)
	if err != nil || !samePath(canonical, backend.executable) {
		return application.NewError(application.ErrorConflict, "renderer_executable_changed", err)
	}
	return nil
}

func (backend *ProcessBackend) runProcess(ctx context.Context, executable string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Dir = filepath.Dir(args[len(args)-1])
	command.Env = rendererEnvironment(command.Dir)
	return command.Run()
}

func validateExecutable(value string) (string, fs.FileInfo, error) {
	if !filepath.IsAbs(value) {
		return "", nil, application.NewError(application.ErrorValidation, "renderer_executable_not_absolute", nil)
	}
	cleaned := filepath.Clean(value)
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", nil, application.NewError(application.ErrorDependency, "renderer_executable_unavailable", err)
	}
	if isIndirection(info) || !isExecutable(info, cleaned) {
		return "", nil, application.NewError(application.ErrorValidation, "renderer_executable_invalid", nil)
	}
	canonical, err := filepath.EvalSymlinks(cleaned)
	if err != nil || !samePath(canonical, cleaned) {
		return "", nil, application.NewError(application.ErrorValidation, "renderer_executable_indirect", err)
	}
	return cleaned, info, nil
}

func validateTemporaryRoot(value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", application.NewError(application.ErrorValidation, "renderer_temp_not_absolute", nil)
	}
	info, err := os.Stat(value)
	if err != nil || !info.IsDir() {
		return "", application.NewError(application.ErrorDependency, "renderer_temp_unavailable", err)
	}
	canonical, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", application.NewError(application.ErrorDependency, "renderer_temp_unavailable", err)
	}
	return filepath.Clean(canonical), nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func rendererEnvironment(temporaryRoot string) []string {
	environment := []string{"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TMPDIR=" + temporaryRoot, "TMP=" + temporaryRoot, "TEMP=" + temporaryRoot}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SYSTEMROOT", "WINDIR"} {
			if value := os.Getenv(key); value != "" {
				environment = append(environment, key+"="+value)
			}
		}
	}
	return environment
}

type boundedDiagnostic struct {
	buffer strings.Builder
	limit  int
	cut    bool
}

func (writer *boundedDiagnostic) Write(data []byte) (int, error) {
	remaining := writer.limit - writer.buffer.Len()
	if remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		_, _ = writer.buffer.Write(data[:remaining])
	}
	if remaining < len(data) {
		writer.cut = true
	}
	return len(data), nil
}

var _ application.PDFBackend = (*ProcessBackend)(nil)
var _ application.PDFBackend = (*ConfiguredBackend)(nil)
