package contentpdf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	application "github.com/yangphere/leanote/app/application/content"
)

func TestProcessBackendUsesDirectArgvAndRequestScopedOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewProcessBackend(ProcessConfig{
		Executable: executable, TemporaryRoot: t.TempDir(), PolicyID: "admin-policy-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	var gotExecutable string
	var gotArgs []string
	var gotInput string
	backend.run = func(_ context.Context, executable string, args []string, stdin io.Reader, _, _ io.Writer) error {
		gotExecutable = executable
		gotArgs = append([]string(nil), args...)
		input, readErr := io.ReadAll(stdin)
		if readErr != nil {
			return readErr
		}
		gotInput = string(input)
		return os.WriteFile(args[len(args)-1], []byte("%PDF-1.4\n%%EOF\n"), 0o600)
	}
	descriptor, err := backend.Descriptor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	document := []byte(`<!doctype html><html><body>safe</body></html>`)
	pdf, err := backend.Render(context.Background(), descriptor, document, 1024)
	if err != nil || string(pdf) != "%PDF-1.4\n%%EOF\n" {
		t.Fatalf("pdf=%q err=%v", pdf, err)
	}
	wantPrefix := []string{"--quiet", "--lowquality", "--disable-local-file-access", "--enable-javascript", "--window-status", "done", "-"}
	if gotExecutable != executable || len(gotArgs) != len(wantPrefix)+1 || !reflect.DeepEqual(gotArgs[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("executable=%q args=%q", gotExecutable, gotArgs)
	}
	if !filepath.IsAbs(gotArgs[len(gotArgs)-1]) || gotInput != string(document) {
		t.Fatalf("output=%q input=%q", gotArgs[len(gotArgs)-1], gotInput)
	}
}

func TestProcessBackendRejectsDescriptorChangeAndCleansFailureArtifacts(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	temporaryRoot := t.TempDir()
	backend, err := NewProcessBackend(ProcessConfig{Executable: executable, TemporaryRoot: temporaryRoot, PolicyID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	backend.run = func(_ context.Context, _ string, args []string, _ io.Reader, _, stderr io.Writer) error {
		_, _ = io.Copy(stderr, bytes.NewReader(bytes.Repeat([]byte("secret stderr "), 100_000)))
		_ = os.WriteFile(args[len(args)-1], []byte("partial"), 0o600)
		return errors.New("process failed")
	}
	descriptor, err := backend.Descriptor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	changed := descriptor
	changed.PolicyID = "other"
	if _, err := backend.Render(context.Background(), changed, []byte("safe"), 1024); errorCategory(err) != application.ErrorConflict {
		t.Fatalf("changed descriptor error=%v category=%q", err, errorCategory(err))
	}
	if _, err := backend.Render(context.Background(), descriptor, []byte("safe"), 1024); errorCategory(err) != application.ErrorRendererFailed {
		t.Fatalf("process error=%v category=%q", err, errorCategory(err))
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary entries=%v err=%v", entries, err)
	}
}

func TestConfiguredBackendRejectsDescriptorAfterAdminPathChanges(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	configured := executable
	backend := NewConfiguredBackend(func() string { return configured }, t.TempDir(), "admin-policy-v1")
	descriptor, err := backend.Descriptor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	configured = filepath.Join(t.TempDir(), "missing-renderer")
	document := []byte(`<!doctype html><html><body>safe</body></html>`)
	if _, err := backend.Render(context.Background(), descriptor, document, 1024); errorCategory(err) != application.ErrorConflict {
		t.Fatalf("changed admin path error=%v category=%q", err, errorCategory(err))
	}
}

func TestProcessBackendClassifiesCallerCancellationAndCleansArtifacts(t *testing.T) {
	backend, descriptor, temporaryRoot := newProcessTestBackend(t)
	backend.run = func(ctx context.Context, _ string, _ []string, _ io.Reader, _, _ io.Writer) error {
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := backend.Render(ctx, descriptor, []byte("safe"), 1024); errorCategory(err) != application.ErrorTimeout || !errors.Is(err, context.Canceled) {
		t.Fatalf("Render() cancellation error=%v category=%q", err, errorCategory(err))
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary entries=%v err=%v", entries, err)
	}
}

func TestProcessBackendReportsOutputLifecycleAndCleanupFailures(t *testing.T) {
	tests := []struct {
		name     string
		inject   func(*ProcessBackend)
		category application.ErrorCategory
		code     string
	}{
		{name: "root open", category: application.ErrorStorageUnavailable, code: "renderer_output_root", inject: func(backend *ProcessBackend) {
			backend.openRoot = func(string) (*os.Root, error) { return nil, errors.New("open root") }
		}},
		{name: "output open", category: application.ErrorStorageUnavailable, code: "renderer_output_open", inject: func(backend *ProcessBackend) {
			backend.openOutput = func(*os.Root, string) (*os.File, error) { return nil, errors.New("open output") }
		}},
		{name: "output sync", category: application.ErrorStorageUnavailable, code: "renderer_output_sync", inject: func(backend *ProcessBackend) {
			backend.syncOutput = func(*os.File) error { return errors.New("sync output") }
		}},
		{name: "output seek", category: application.ErrorStorageUnavailable, code: "renderer_output_seek", inject: func(backend *ProcessBackend) {
			backend.seekOutput = func(*os.File, int64, int) (int64, error) { return 0, errors.New("seek output") }
		}},
		{name: "output identity", category: application.ErrorRendererFailed, code: "renderer_output_identity", inject: func(backend *ProcessBackend) {
			backend.sameFile = func(os.FileInfo, os.FileInfo) bool { return false }
		}},
		{name: "output read", category: application.ErrorStorageUnavailable, code: "renderer_output_read", inject: func(backend *ProcessBackend) {
			backend.readOutput = func(io.Reader, int64) ([]byte, error) { return nil, errors.New("read output") }
		}},
		{name: "output close", category: application.ErrorStorageUnavailable, code: "renderer_output_read", inject: func(backend *ProcessBackend) {
			backend.closeOutput = func(file *os.File) error { return errors.Join(file.Close(), errors.New("close output")) }
		}},
		{name: "root close", category: application.ErrorStorageUnavailable, code: "renderer_output_root_close", inject: func(backend *ProcessBackend) {
			backend.closeRoot = func(root *os.Root) error { return errors.Join(root.Close(), errors.New("close root")) }
		}},
		{name: "cleanup", category: application.ErrorUnknownResult, code: "renderer_temp_cleanup", inject: func(backend *ProcessBackend) {
			backend.removeAll = func(path string) error { return errors.Join(os.RemoveAll(path), errors.New("remove temp")) }
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend, descriptor, _ := newProcessTestBackend(t)
			backend.run = func(_ context.Context, _ string, args []string, _ io.Reader, _, _ io.Writer) error {
				return os.WriteFile(args[len(args)-1], []byte("pdf output"), 0o600)
			}
			test.inject(backend)
			_, err := backend.Render(context.Background(), descriptor, []byte("safe"), 1024)
			if errorCategory(err) != test.category || applicationErrorCode(err) != test.code {
				t.Fatalf("Render() error=%v category=%q code=%q", err, errorCategory(err), applicationErrorCode(err))
			}
		})
	}
}

func TestProcessBackendObservesCancellationAfterSuccessfulProcessAndCleansArtifacts(t *testing.T) {
	backend, descriptor, temporaryRoot := newProcessTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	backend.run = func(_ context.Context, _ string, args []string, _ io.Reader, _, _ io.Writer) error {
		if err := os.WriteFile(args[len(args)-1], []byte("partial"), 0o600); err != nil {
			return err
		}
		cancel()
		return nil
	}
	if _, err := backend.Render(ctx, descriptor, []byte("safe"), 1024); errorCategory(err) != application.ErrorTimeout || !errors.Is(err, context.Canceled) {
		t.Fatalf("Render() cancellation error=%v category=%q", err, errorCategory(err))
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary entries=%v err=%v", entries, err)
	}
}

func TestBoundedDiagnosticConsumesInputAndRetainsOnlyLimit(t *testing.T) {
	writer := &boundedDiagnostic{limit: 4}
	written, err := writer.Write([]byte("secret diagnostic"))
	if err != nil || written != len("secret diagnostic") || writer.buffer.String() != "secr" || !writer.cut {
		t.Fatalf("Write() written=%d err=%v buffer=%q cut=%v", written, err, writer.buffer.String(), writer.cut)
	}
}

func newProcessTestBackend(t *testing.T) (*ProcessBackend, application.RendererDescriptor, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	temporaryRoot := t.TempDir()
	backend, err := NewProcessBackend(ProcessConfig{Executable: executable, TemporaryRoot: temporaryRoot, PolicyID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := backend.Descriptor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return backend, descriptor, temporaryRoot
}

func errorCategory(err error) application.ErrorCategory {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}

func applicationErrorCode(err error) string {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Code
	}
	return ""
}
