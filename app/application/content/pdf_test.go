package content

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestSerializeSelfContainedPDFRemovesActiveContentAndExternalResources(t *testing.T) {
	imageData := encodePNG(t, 1, 1)
	request := PDFDocumentRequest{
		Title:     "Untrusted <title>",
		HTML:      `<div onclick="fetch('/secret')"><script>fetch('https://evil.test')</script><iframe src="file:///etc/passwd"></iframe><style>@import url(https://evil.test/x); body{background:url(file:///x)}</style><img src="/authorized.png"><a href="https://evil.test">external</a><a href="#local">local</a></div>`,
		Resources: map[string]PDFResource{"/authorized.png": {MIME: "image/png", Data: imageData}},
	}
	document, err := SerializeSelfContainedPDF(request)
	if err != nil {
		t.Fatal(err)
	}
	text := string(document)
	for _, forbidden := range []string{"fetch(", "<iframe", "@import", "onclick", "https://evil.test", "file:///etc/passwd"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("serialized document contains %q: %s", forbidden, text)
		}
	}
	wantData := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData)
	if !strings.Contains(text, wantData) || !strings.Contains(text, "#local") || !strings.Contains(text, "font-family") {
		t.Fatalf("serialized document lost safe content/resources: %s", text)
	}
	if !strings.Contains(text, `data-leanote-pinned="completion-v1"`) || !strings.Contains(text, "window.status") {
		t.Fatalf("serialized document lost fixed completion signal: %s", text)
	}
	if err := ValidateSelfContainedPDFDocument(document); err != nil {
		t.Fatal(err)
	}
}

func TestSerializeSelfContainedPDFRejectsOversizedOrInvalidResource(t *testing.T) {
	request := PDFDocumentRequest{
		Title: "note", HTML: `<img src="/image.png">`,
		Resources: map[string]PDFResource{"/image.png": {MIME: "text/html", Data: []byte("<html>")}},
	}
	if _, err := SerializeSelfContainedPDF(request); errorCategoryOf(err) != ErrorUnsupportedMedia {
		t.Fatalf("invalid resource error=%v category=%q", err, errorCategoryOf(err))
	}
	request.Resources = map[string]PDFResource{"/image.png": {MIME: "image/png", Data: bytes.Repeat([]byte("x"), 129*1024*1024)}}
	if _, err := SerializeSelfContainedPDF(request); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("oversized resource error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestSerializeSelfContainedPDFRendersMarkdownBeforeSanitizing(t *testing.T) {
	imageData := encodePNG(t, 1, 1)
	document, err := SerializeSelfContainedPDF(PDFDocumentRequest{
		Title: "markdown", Markdown: true,
		HTML:      "# Heading\n\n![safe](/authorized.png)\n\n<script>fetch('https://evil.test')</script>",
		Resources: map[string]PDFResource{"/authorized.png": {MIME: "image/png", Data: imageData}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(document)
	if !strings.Contains(text, "<h1>Heading</h1>") || !strings.Contains(text, "data:image/png;base64,") || strings.Contains(text, "evil.test") {
		t.Fatalf("markdown document=%s", text)
	}
}

func TestValidateSelfContainedPDFDocumentRejectsNavigableURLsAndActiveNodes(t *testing.T) {
	for _, document := range []string{
		`<html><body><img src="file:///tmp/x"></body></html>`,
		`<html><body><img src="https://example.test/x"></body></html>`,
		`<html><body><img src="//example.test/x"></body></html>`,
		`<html><body background="https://example.test/x"></body></html>`,
		`<html><body><svg><image xlink:href="https://example.test/x"></image></svg></body></html>`,
		`<html><body><script>1</script></body></html>`,
		`<html><head><meta http-equiv="refresh" content="0;url=https://example.test"></head></html>`,
	} {
		if errorCategoryOf(ValidateSelfContainedPDFDocument([]byte(document))) != ErrorUnsafePath {
			t.Fatalf("document unexpectedly accepted: %s", document)
		}
	}
}
