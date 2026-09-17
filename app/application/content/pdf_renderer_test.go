package content

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakePDFBackend struct {
	descriptor RendererDescriptor
	result     []byte
	err        error
	document   []byte
	maxOutput  int64
	wait       bool
}

func (backend *fakePDFBackend) Descriptor(context.Context) (RendererDescriptor, error) {
	return backend.descriptor, nil
}

func (backend *fakePDFBackend) Render(ctx context.Context, descriptor RendererDescriptor, document []byte, maxOutput int64) ([]byte, error) {
	backend.document = append([]byte(nil), document...)
	backend.maxOutput = maxOutput
	if backend.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return append([]byte(nil), backend.result...), backend.err
}

func TestPDFRendererDoesNotAllowConfiguredArtifactLimitAboveHardCap(t *testing.T) {
	backend := &fakePDFBackend{descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"}, result: validTestPDF()}
	renderer := PDFRenderer{Backend: backend, Timeout: time.Second, MaxOutputBytes: 128 * 1024 * 1024}
	if _, err := renderer.Render(context.Background(), PDFRenderRequest{Title: "x", HTML: "safe"}); err != nil {
		t.Fatal(err)
	}
	if backend.maxOutput != 64*1024*1024 {
		t.Fatalf("backend max output=%d, want 64 MiB hard cap", backend.maxOutput)
	}
}

func TestPDFRendererSerializesAndValidatesArtifact(t *testing.T) {
	backend := &fakePDFBackend{
		descriptor: RendererDescriptor{PolicyID: "approved-v1", ExecutableID: "renderer-one"},
		result:     validTestPDF(),
	}
	renderer := PDFRenderer{Backend: backend, Timeout: time.Second, MaxOutputBytes: 1024 * 1024}
	artifact, err := renderer.Render(context.Background(), PDFRenderRequest{
		Title: "../unsafe, title\x00", HTML: `<script>fetch('https://evil.test')</script><p>safe</p>`, Markdown: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Filename != "unsafe- title.pdf" || artifact.ContentType != "application/pdf" || string(artifact.Data) != string(backend.result) {
		t.Fatalf("artifact=%+v", artifact)
	}
	if strings.Contains(string(backend.document), "evil.test") || !strings.Contains(string(backend.document), "<p>safe</p>") {
		t.Fatalf("backend document was not sanitized: %s", backend.document)
	}
}

func TestSafePDFNameRejectsDispositionAndPathDelimiters(t *testing.T) {
	tests := map[string]string{
		"/":                "Untitled.pdf",
		"\\":               "Untitled.pdf",
		`folder/unsafe";x`: "unsafe--x.pdf",
	}
	for title, want := range tests {
		if got := safePDFName(title); got != want {
			t.Fatalf("safePDFName(%q)=%q, want %q", title, got, want)
		}
	}
}

func TestValidatePDFArtifactRejectsMalformedStructure(t *testing.T) {
	valid := validTestPDF()
	for _, pdf := range [][]byte{
		[]byte("%PDF-1.4\n%%EOF\n"),
		[]byte("%PDF-1.4\nxref\ntrailer\nstartxref\n999999\n%%EOF\n"),
		[]byte("%PDF-1.4\nxref\nstartxref\n9\n%%EOF\n"),
		append(append([]byte(nil), valid...), []byte("trailing")...),
		bytes.Replace(valid, []byte("0000000009 00000 n"), []byte("0000000000 00000 n"), 1),
		bytes.Replace(valid, []byte("<< /Size 2 /Root 1 0 R >>"), []byte("<< /Info (/Size 2 /Root 1 0 R) >>"), 1),
		bytes.Replace(valid, []byte("0 2\n"), []byte("0 3\n"), 1),
		bytes.Replace(valid, []byte("\n%%EOF"), []byte("\nunexpected\n%%EOF"), 1),
	} {
		if errorCategoryOf(validatePDFArtifact(pdf, 1024*1024)) != ErrorRendererFailed {
			t.Fatalf("malformed PDF accepted: %q", pdf)
		}
	}
}

func validTestPDF() []byte {
	prefix := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n"
	xref := len(prefix)
	return []byte(prefix + "xref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer\n<< /Size 2 /Root 1 0 R >>\nstartxref\n" + fmt.Sprintf("%d", xref) + "\n%%EOF\n")
}

func TestPDFRendererRejectsMissingDescriptorAndInvalidOutput(t *testing.T) {
	renderer := PDFRenderer{Backend: &fakePDFBackend{}, Timeout: time.Second, MaxOutputBytes: 1024}
	if _, err := renderer.Render(context.Background(), PDFRenderRequest{Title: "x", HTML: "safe"}); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("descriptor error=%v category=%q", err, errorCategoryOf(err))
	}
	backend := &fakePDFBackend{descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"}, result: []byte("not pdf")}
	renderer.Backend = backend
	if _, err := renderer.Render(context.Background(), PDFRenderRequest{Title: "x", HTML: "safe"}); errorCategoryOf(err) != ErrorRendererFailed {
		t.Fatalf("invalid output error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestPDFRendererEnforcesOneContextTimeout(t *testing.T) {
	backend := &fakePDFBackend{descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"}, wait: true}
	renderer := PDFRenderer{Backend: backend, Timeout: 10 * time.Millisecond, MaxOutputBytes: 1024}
	_, err := renderer.Render(context.Background(), PDFRenderRequest{Title: "x", HTML: "safe"})
	if errorCategoryOf(err) != ErrorTimeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestPDFRendererClassifiesCallerCancellationAsTimeout(t *testing.T) {
	backend := &fakePDFBackend{descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"}, wait: true}
	renderer := PDFRenderer{Backend: backend, Timeout: time.Second, MaxOutputBytes: 1024}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := renderer.Render(ctx, PDFRenderRequest{Title: "x", HTML: "safe"})
	if errorCategoryOf(err) != ErrorTimeout || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v category=%q", err, errorCategoryOf(err))
	}
}
