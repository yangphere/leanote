package content

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakePDFBackend struct {
	descriptor RendererDescriptor
	result     []byte
	err        error
	document   []byte
	wait       bool
}

func (backend *fakePDFBackend) Descriptor(context.Context) (RendererDescriptor, error) {
	return backend.descriptor, nil
}

func (backend *fakePDFBackend) Render(ctx context.Context, descriptor RendererDescriptor, document []byte, _ int64) ([]byte, error) {
	backend.document = append([]byte(nil), document...)
	if backend.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return append([]byte(nil), backend.result...), backend.err
}

func TestPDFRendererSerializesAndValidatesArtifact(t *testing.T) {
	backend := &fakePDFBackend{
		descriptor: RendererDescriptor{PolicyID: "approved-v1", ExecutableID: "renderer-one"},
		result:     []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF\n"),
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
