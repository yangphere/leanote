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

func errorCategory(err error) application.ErrorCategory {
	var contentErr *application.Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}
