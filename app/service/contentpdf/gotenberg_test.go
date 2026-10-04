package contentpdf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

func gotenbergTestDocument(t *testing.T) []byte {
	t.Helper()
	document, err := application.SerializeSelfContainedPDF(application.PDFDocumentRequest{
		Title: "标题", HTML: "<p>中文正文</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func gotenbergTestPDF() []byte {
	prefix := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n"
	return []byte(prefix + "xref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer\n<< /Size 2 /Root 1 0 R >>\nstartxref\n" + fmt.Sprintf("%d", len(prefix)) + "\n%%EOF\n")
}

func newGotenbergTestBackend(t *testing.T, server *httptest.Server) *GotenbergBackend {
	t.Helper()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewGotenbergBackend(endpoint, "leanote-pdf-v1")
	if err != nil {
		t.Fatal(err)
	}
	return backend
}

func renderWithGotenberg(t *testing.T, backend *GotenbergBackend, ctx context.Context, document []byte, max int64) ([]byte, error) {
	t.Helper()
	descriptor, err := backend.Descriptor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return backend.Render(ctx, descriptor, document, max)
}

func requireGotenbergError(t *testing.T, err error, category application.ErrorCategory, code string) *application.Error {
	t.Helper()
	var contentErr *application.Error
	if !errors.As(err, &contentErr) {
		t.Fatalf("error = %v, want content error %s/%s", err, category, code)
	}
	if contentErr.Category != category || contentErr.Code != code {
		t.Fatalf("error = %s/%s, want %s/%s", contentErr.Category, contentErr.Code, category, code)
	}
	return contentErr
}

func TestGotenbergBackendSendsExpectedMultipartRequest(t *testing.T) {
	document := gotenbergTestDocument(t)
	var path, method, accept string
	var filename, fileContent string
	fields := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method, accept = r.URL.Path, r.Method, r.Header.Get("Accept")
		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("multipart reader: %v", err)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		for {
			part, nextErr := reader.NextPart()
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if nextErr != nil {
				t.Errorf("next part: %v", nextErr)
				return
			}
			data, _ := io.ReadAll(part)
			if part.FormName() == "files" {
				filename, fileContent = part.FileName(), string(data)
				continue
			}
			fields[part.FormName()] = string(data)
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(gotenbergTestPDF())
	}))
	defer server.Close()

	pdf, err := renderWithGotenberg(t, newGotenbergTestBackend(t, server), context.Background(), document, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdf) != string(gotenbergTestPDF()) {
		t.Fatalf("pdf bytes changed")
	}
	if method != http.MethodPost || path != "/forms/chromium/convert/html" || accept != "application/pdf" {
		t.Fatalf("request = %s %s accept=%q", method, path, accept)
	}
	if filename != "index.html" || fileContent != string(document) {
		t.Fatalf("file part = %q (%d bytes), want index.html with the exact document", filename, len(fileContent))
	}
	want := map[string]string{
		"waitForExpression": `window.status === "done"`,
		"printBackground":   "true",
		"paperWidth":        "21cm", "paperHeight": "29.7cm",
		"marginTop": "1cm", "marginBottom": "1cm", "marginLeft": "1cm", "marginRight": "1cm",
	}
	if len(fields) != len(want) {
		t.Fatalf("fields = %v, want exactly %v", fields, want)
	}
	for name, value := range want {
		if fields[name] != value {
			t.Fatalf("field %s = %q, want %q", name, fields[name], value)
		}
	}
}

func TestGotenbergBackendOnlyContactsConfiguredAddress(t *testing.T) {
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { otherHits.Add(1) }))
	defer other.Close()
	var configuredHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configuredHits.Add(1)
		http.Redirect(w, r, other.URL+"/forms/chromium/convert/html", http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	_, err := renderWithGotenberg(t, newGotenbergTestBackend(t, server), context.Background(), gotenbergTestDocument(t), 1<<20)
	requireGotenbergError(t, err, application.ErrorRendererFailed, "renderer_process_failed")
	if configuredHits.Load() != 1 || otherHits.Load() != 0 {
		t.Fatalf("hits configured=%d other=%d, redirect must not be followed", configuredHits.Load(), otherHits.Load())
	}
}

func TestGotenbergBackendMapsNonOKStatusWithoutLeakingDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Gotenberg-Trace", "trace-1")
		http.Error(w, "chromium crashed", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := renderWithGotenberg(t, newGotenbergTestBackend(t, server), context.Background(), gotenbergTestDocument(t), 1<<20)
	contentErr := requireGotenbergError(t, err, application.ErrorRendererFailed, "renderer_process_failed")
	if !strings.Contains(contentErr.Cause.Error(), "chromium crashed") || !strings.Contains(contentErr.Cause.Error(), "500") {
		t.Fatalf("cause = %v, want status and bounded diagnostic", contentErr.Cause)
	}
	if strings.Contains(contentErr.Error(), "chromium") {
		t.Fatalf("user-facing error leaks diagnostics: %q", contentErr.Error())
	}
}

func TestGotenbergBackendBoundsDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", maxRendererDiagnosticBytes*4)))
	}))
	defer server.Close()

	_, err := renderWithGotenberg(t, newGotenbergTestBackend(t, server), context.Background(), gotenbergTestDocument(t), 1<<20)
	contentErr := requireGotenbergError(t, err, application.ErrorRendererFailed, "renderer_process_failed")
	if len(contentErr.Cause.Error()) > maxRendererDiagnosticBytes+256 {
		t.Fatalf("diagnostic length = %d, want bounded", len(contentErr.Cause.Error()))
	}
}

func TestGotenbergBackendMapsUnavailableDependency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	backend := newGotenbergTestBackend(t, server)
	server.Close()

	_, err := renderWithGotenberg(t, backend, context.Background(), gotenbergTestDocument(t), 1<<20)
	requireGotenbergError(t, err, application.ErrorDependency, "renderer_unavailable")
}

func TestGotenbergBackendHonorsDeadlineAndCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	backend := newGotenbergTestBackend(t, server)

	deadline, cancelDeadline := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelDeadline()
	_, err := renderWithGotenberg(t, backend, deadline, gotenbergTestDocument(t), 1<<20)
	requireGotenbergError(t, err, application.ErrorTimeout, "renderer_process_timeout")

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	descriptor, descriptorErr := backend.Descriptor(context.Background())
	if descriptorErr != nil {
		t.Fatal(descriptorErr)
	}
	_, err = backend.Render(canceled, descriptor, gotenbergTestDocument(t), 1<<20)
	requireGotenbergError(t, err, application.ErrorTimeout, "renderer_process_timeout")
	if _, err = backend.Descriptor(canceled); err == nil {
		t.Fatal("canceled descriptor request succeeded")
	}
}

func TestGotenbergBackendEnforcesOutputLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 65)))
	}))
	defer server.Close()
	backend := newGotenbergTestBackend(t, server)

	_, err := renderWithGotenberg(t, backend, context.Background(), gotenbergTestDocument(t), 64)
	requireGotenbergError(t, err, application.ErrorTooLarge, "renderer_output_limit")
	body, err := renderWithGotenberg(t, backend, context.Background(), gotenbergTestDocument(t), 65)
	if err != nil || len(body) != 65 {
		t.Fatalf("exact limit body = %d err=%v, want accepted", len(body), err)
	}
	_, err = renderWithGotenberg(t, backend, context.Background(), gotenbergTestDocument(t), 0)
	requireGotenbergError(t, err, application.ErrorValidation, "renderer_output_limit")
}

func TestGotenbergBackendRejectsChangedDescriptorAndUnsafeDocumentBeforeNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	backend := newGotenbergTestBackend(t, server)
	descriptor, err := backend.Descriptor(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, changed := range []application.RendererDescriptor{
		{PolicyID: "other", ExecutableID: descriptor.ExecutableID},
		{PolicyID: descriptor.PolicyID, ExecutableID: "other"},
	} {
		_, err = backend.Render(context.Background(), changed, gotenbergTestDocument(t), 1<<20)
		requireGotenbergError(t, err, application.ErrorConflict, "renderer_descriptor_changed")
	}
	if _, err = backend.Render(context.Background(), descriptor, []byte(`<html><img src="https://example.com/x.png"></html>`), 1<<20); err == nil {
		t.Fatal("document with external resource was sent to the renderer")
	}
	if hits.Load() != 0 {
		t.Fatalf("renderer received %d requests for rejected input", hits.Load())
	}
}

func TestGotenbergDescriptorDependsOnPolicyAndEndpoint(t *testing.T) {
	parse := func(raw string) *url.URL {
		endpoint, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return endpoint
	}
	first, err := NewGotenbergBackend(parse("http://gotenberg:3000"), "leanote-pdf-v1")
	if err != nil {
		t.Fatal(err)
	}
	same, _ := NewGotenbergBackend(parse("http://gotenberg:3000/"), "leanote-pdf-v1")
	otherHost, _ := NewGotenbergBackend(parse("http://other:3000"), "leanote-pdf-v1")
	otherPolicy, _ := NewGotenbergBackend(parse("http://gotenberg:3000"), "leanote-pdf-v2")
	descriptor := func(backend *GotenbergBackend) application.RendererDescriptor {
		value, descriptorErr := backend.Descriptor(context.Background())
		if descriptorErr != nil {
			t.Fatal(descriptorErr)
		}
		return value
	}
	if descriptor(first) != descriptor(same) {
		t.Fatal("trailing slash changed the descriptor")
	}
	if descriptor(first) == descriptor(otherHost) || descriptor(first) == descriptor(otherPolicy) {
		t.Fatal("descriptor did not change with endpoint or policy")
	}
	if len(descriptor(first).ExecutableID) != 32 {
		t.Fatalf("executable id = %q, want 32 hex characters", descriptor(first).ExecutableID)
	}
}

func TestValidateGotenbergEndpointRejectsUnsafeValues(t *testing.T) {
	for _, raw := range []string{
		"ftp://gotenberg:3000", "gotenberg:3000", "http://", "http://user:pass@gotenberg:3000",
		"http://gotenberg:3000?x=1", "http://gotenberg:3000/?", "http://gotenberg:3000#frag",
		"http://gotenberg:3000/api", "mailto:a@b",
	} {
		endpoint, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if err := ValidateGotenbergEndpoint(endpoint); err == nil {
			t.Fatalf("endpoint %q accepted", raw)
		}
	}
	for _, raw := range []string{"http://gotenberg:3000", "https://gotenberg.internal/", "http://127.0.0.1:3000"} {
		endpoint, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateGotenbergEndpoint(endpoint); err != nil {
			t.Fatalf("endpoint %q rejected: %v", raw, err)
		}
	}
	if err := ValidateGotenbergEndpoint(nil); err == nil {
		t.Fatal("nil endpoint accepted")
	}
	if _, err := NewGotenbergBackend(&url.URL{Scheme: "http", Host: "gotenberg:3000"}, " "); err == nil {
		t.Fatal("empty policy accepted")
	}
}

// 后端接入完整 PDFRenderer 调用链：有效 PDF 通过 validatePDFArtifact，非法 PDF 被拒绝。
func TestGotenbergBackendThroughPDFRendererValidatesArtifact(t *testing.T) {
	respond := gotenbergTestPDF()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(respond)
	}))
	defer server.Close()
	renderer := application.PDFRenderer{
		Backend: newGotenbergTestBackend(t, server), Timeout: 5 * time.Second, MaxOutputBytes: 1 << 20,
	}

	artifact, err := renderer.Render(context.Background(), application.PDFRenderRequest{Title: "笔记", HTML: "<p>中文正文</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ContentType != "application/pdf" || string(artifact.Data) != string(gotenbergTestPDF()) {
		t.Fatalf("artifact = %q (%d bytes)", artifact.ContentType, len(artifact.Data))
	}

	respond = []byte("%PDF-1.4\nnot a real pdf\n%%EOF\n")
	_, err = renderer.Render(context.Background(), application.PDFRenderRequest{Title: "笔记", HTML: "<p>x</p>"})
	requireGotenbergError(t, err, application.ErrorRendererFailed, "pdf_startxref")
}
