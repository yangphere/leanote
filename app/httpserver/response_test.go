package httpserver

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func apply(t *testing.T, r Result) (*httptest.ResponseRecorder, *statusWriter) {
	t.Helper()
	rec := httptest.NewRecorder()
	sw := newStatusWriter(rec)
	r.Apply(sw, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec, sw
}

func TestJSONResult(t *testing.T) {
	rec, sw := apply(t, JSONResult(http.StatusOK, map[string]interface{}{"Ok": true, "Msg": ""}))
	if sw.status != http.StatusOK {
		t.Fatalf("status = %d", sw.status)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	if strings.HasSuffix(body, "\n") {
		t.Fatal("Revel RenderJSON writes no trailing newline")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("body not json: %q", body)
	}
}

func TestJSONPResultMatchesRevelRenderJsonP(t *testing.T) {
	rec, sw := apply(t, JSONPResult("cb", map[string]interface{}{"Ok": true}))
	if sw.status != http.StatusOK {
		t.Fatalf("status = %d", sw.status)
	}
	// Revel results.go renderJsonP: application/javascript; charset=utf-8,
	// body `callback(json);`.
	if ct := rec.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/javascript; charset=utf-8", ct)
	}
	if got := rec.Body.String(); got != `cb({"Ok":true});` {
		t.Fatalf("body = %q", got)
	}
}

func TestJSONPResultCallbackIsVerbatim(t *testing.T) {
	// Revel concatenates the callback verbatim (no identifier escaping);
	// pin that so any future hardening is a conscious contract change.
	rec, _ := apply(t, JSONPResult("cb(1)", map[string]interface{}{"Ok": true}))
	if got := rec.Body.String(); got != `cb(1)({"Ok":true});` {
		t.Fatalf("body = %q", got)
	}
}

func TestTextResult(t *testing.T) {
	rec, sw := apply(t, TextResult(http.StatusOK, "hello"))
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Body.String() != "hello" || sw.status != http.StatusOK {
		t.Fatalf("body=%q status=%d", rec.Body.String(), sw.status)
	}
}

func TestHTMLResult(t *testing.T) {
	rec, sw := apply(t, HTMLResult(http.StatusOK, "<html></html>"))
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if sw.status != http.StatusOK {
		t.Fatalf("status = %d", sw.status)
	}
}

func TestBinaryResult(t *testing.T) {
	rec, swBinary := apply(t, BinaryResult(http.StatusOK, "image/png", []byte{0x89, 0x50}))
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Body.Len() != 2 || swBinary.status != http.StatusOK {
		t.Fatalf("body len=%d status=%d", rec.Body.Len(), swBinary.status)
	}
}

func TestFileResultAttachment(t *testing.T) {
	path := t.TempDir() + "/report.txt"
	if err := os.WriteFile(path, []byte("report-bytes"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	rec, _ := apply(t, FileResult(path, "report.txt"))
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="report.txt"`) {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if rec.Body.String() != "report-bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRedirectResult(t *testing.T) {
	rec, sw := apply(t, RedirectResult("/login"))
	if sw.status != http.StatusFound {
		t.Fatalf("status = %d, want 302", sw.status)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestTemplateResultUsesRenderer(t *testing.T) {
	saved := TemplateRenderer
	TemplateRenderer = func(name string, args map[string]interface{}) ([]byte, error) {
		return []byte("<p>" + name + ":" + args["v"].(string) + "</p>"), nil
	}
	defer func() { TemplateRenderer = saved }()

	rec, sw := apply(t, TemplateResult(http.StatusOK, "Home/Index.html", map[string]interface{}{"v": "1"}))
	if sw.status != http.StatusOK {
		t.Fatalf("status = %d", sw.status)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Home/Index.html:1") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStatusWriterImplicit200OnFirstWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := newStatusWriter(rec)
	sw.Write([]byte("implicit"))
	if sw.status != http.StatusOK || rec.Code != http.StatusOK {
		t.Fatalf("implicit 200 not applied: sw=%d rec=%d", sw.status, rec.Code)
	}
	if rec.Body.String() != "implicit" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStatusWriterIgnoresLateHeaderWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := newStatusWriter(rec)
	sw.WriteHeader(http.StatusInternalServerError)
	sw.WriteHeader(http.StatusOK) // late attempt must be ignored
	if sw.status != http.StatusInternalServerError || rec.Code != http.StatusInternalServerError {
		t.Fatalf("late header write changed status: sw=%d rec=%d", sw.status, rec.Code)
	}
	sw.Write([]byte("x"))
	if sw.status != http.StatusInternalServerError {
		t.Fatal("body write must not change a written status")
	}
}

func TestFileResultContentTypeByExtension(t *testing.T) {
	cases := map[string]string{
		"style.css":  "text/css; charset=utf-8",
		"app.js":     "text/javascript; charset=utf-8",
		"logo.png":   "image/png",
		"noext":      "application/octet-stream",
		"doc.pdf":    "application/pdf",
		"font.woff2": "font/woff2",
		"page.html":  "text/html; charset=utf-8",
		"data.json":  "application/json",
		"readme":     "application/octet-stream",
		"upper.PNG":  "image/png",
		"note.txt":   "text/plain; charset=utf-8",
	}
	for name, want := range cases {
		if got := contentTypeByFilename(name); got != want {
			t.Errorf("contentTypeByFilename(%q) = %q, want %q", name, got, want)
		}
	}
	// An extension outside the pinned map falls through to
	// mime.TypeByExtension, whose answer is machine-dependent (registry on
	// Windows); only assert octet-stream when the host has no mapping.
	if mime.TypeByExtension(".xyz") == "" {
		if got := contentTypeByFilename("unknown.xyz"); got != "application/octet-stream" {
			t.Errorf("contentTypeByFilename(unknown.xyz) = %q, want octet-stream on an unmapped host", got)
		}
	} else {
		t.Logf("host maps .xyz to %q; skipping machine-dependent assertion", mime.TypeByExtension(".xyz"))
	}
}
