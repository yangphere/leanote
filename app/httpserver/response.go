package httpserver

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
)

// Result is what a controller action returns; the server applies it to the
// response. It replaces the revel.Result interface.
type Result interface {
	Apply(w http.ResponseWriter, r *http.Request)
}

// TemplateRenderer renders a template by name with ViewArgs, mirroring what
// the template registry wires up at startup. It is an install point so the
// response types stay independent of html/template loading.
var TemplateRenderer = func(name string, args map[string]interface{}) ([]byte, error) {
	return nil, fmt.Errorf("no template renderer installed")
}

// newStatusWriter wraps a ResponseWriter with the single response invariant:
// once a status is written it cannot change, and an implicit 200 is applied
// on first body write.
func newStatusWriter(w http.ResponseWriter) *statusWriter {
	return &statusWriter{ResponseWriter: w}
}

type statusWriter struct {
	http.ResponseWriter
	wroteHeader bool
	status      int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// JSONResult serialises v exactly like Revel's RenderJSON: compact
// json.Marshal output with no trailing newline and the
// "application/json; charset=utf-8" content type.
func JSONResult(status int, v interface{}) Result {
	return jsonResult{status: status, value: v}
}

// JSONLineResult is the fixed-line JSON response used by readiness probes.
// It deliberately appends exactly one newline to the compact JSON payload.
func JSONLineResult(status int, v interface{}) Result {
	return jsonLineResult{status: status, value: v}
}

type jsonLineResult struct {
	status int
	value  interface{}
}

func (r jsonLineResult) Apply(w http.ResponseWriter, req *http.Request) {
	body, err := json.Marshal(r.value)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(r.status)
	w.Write(append(body, '\n'))
}

type jsonResult struct {
	status int
	value  interface{}
}

func (r jsonResult) Apply(w http.ResponseWriter, req *http.Request) {
	body, err := json.Marshal(r.value)
	if err != nil {
		// A value that cannot marshal is a programming error; fail loudly
		// instead of writing a wrong 200.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(r.status)
	w.Write(body)
}

// JSONPResult renders the Revel RenderJsonP shape: an
// `application/javascript; charset=utf-8` body of `callback(json);`
// (revel results.go renderJsonP).
func JSONPResult(callback string, v interface{}) Result {
	return jsonpResult{callback: callback, value: v}
}

type jsonpResult struct {
	callback string
	value    interface{}
}

func (r jsonpResult) Apply(w http.ResponseWriter, req *http.Request) {
	body, err := json.Marshal(r.value)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s(%s);", r.callback, body)
}

// TextResult writes plain text.
func TextResult(status int, text string) Result {
	return textResult{status: status, text: text}
}

type textResult struct {
	status int
	text   string
}

func (r textResult) Apply(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(r.status)
	w.Write([]byte(r.text))
}

// HTMLResult writes an inline HTML body with the html content type.
func HTMLResult(status int, html string) Result {
	return htmlResult{status: status, html: html}
}

type htmlResult struct {
	status int
	html   string
}

func (r htmlResult) Apply(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(r.status)
	w.Write([]byte(r.html))
}

// TemplateResult renders a template through the installed TemplateRenderer.
func TemplateResult(status int, name string, args map[string]interface{}) Result {
	return templateResult{status: status, name: name, args: args}
}

type templateResult struct {
	status int
	name   string
	args   map[string]interface{}
}

func (r templateResult) Apply(w http.ResponseWriter, req *http.Request) {
	body, err := TemplateRenderer(r.name, r.args)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "template %s: %v", r.name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(r.status)
	w.Write(body)
}

// BinaryResult writes raw bytes with an explicit content type (image and
// download endpoints).
func BinaryResult(status int, contentType string, data []byte) Result {
	return binaryResult{status: status, contentType: contentType, data: data}
}

type binaryResult struct {
	status      int
	contentType string
	data        []byte
}

func (r binaryResult) Apply(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", r.contentType)
	w.WriteHeader(r.status)
	w.Write(r.data)
}

// FileResult streams a file from disk as an attachment download, with the
// deterministic content-type mapping Revel applies (revel
// ContentTypeByFilename: known extension wins, text/* gains a charset,
// unknown extension degrades to application/octet-stream).
func FileResult(path, downloadName string) Result {
	return fileResult{path: path, name: downloadName}
}

type fileResult struct {
	path string
	name string
}

func (r fileResult) Apply(w http.ResponseWriter, req *http.Request) {
	f, err := os.Open(r.path)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	w.Header().Set("Content-Type", contentTypeByFilename(r.name))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", r.name))
	http.ServeContent(w, req, r.name, info.ModTime(), f)
}

// fileContentTypes pins the content types for the extensions this project
// actually serves, independent of the host OS registry that
// mime.TypeByExtension consults on Windows.
var fileContentTypes = map[string]string{
	"html":  "text/html; charset=utf-8",
	"css":   "text/css; charset=utf-8",
	"js":    "text/javascript; charset=utf-8",
	"mjs":   "text/javascript; charset=utf-8",
	"json":  "application/json",
	"txt":   "text/plain; charset=utf-8",
	"xml":   "text/xml; charset=utf-8",
	"png":   "image/png",
	"jpg":   "image/jpeg",
	"jpeg":  "image/jpeg",
	"gif":   "image/gif",
	"svg":   "image/svg+xml",
	"ico":   "image/x-icon",
	"webp":  "image/webp",
	"woff":  "font/woff",
	"woff2": "font/woff2",
	"ttf":   "font/ttf",
	"otf":   "font/otf",
	"pdf":   "application/pdf",
	"wasm":  "application/wasm",
	"mp4":   "video/mp4",
}

// contentTypeByFilename mirrors revel.ContentTypeByFilename: a known
// extension wins, text/* gains a charset, anything unknown degrades to
// application/octet-stream.
func contentTypeByFilename(filename string) string {
	dot := strings.LastIndex(filename, ".")
	if dot == -1 || dot+1 >= len(filename) {
		return "application/octet-stream"
	}
	ext := strings.ToLower(filename[dot+1:])
	if ct, ok := fileContentTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension("." + ext); ct != "" {
		if strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "charset") {
			return ct + "; charset=utf-8"
		}
		return ct
	}
	return "application/octet-stream"
}

// RedirectResult issues a 302 to url.
func RedirectResult(url string) Result {
	return redirectResult{url: url}
}

type redirectResult struct {
	url string
}

func (r redirectResult) Apply(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, r.url, http.StatusFound)
}
