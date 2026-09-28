package httpserver

import (
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/yangphere/leanote/app/domain"
)

const (
	// maxFormBodyBytes bounds request bodies before the standard library parses
	// urlencoded or multipart form fields.  Multipart files are still streamed
	// through the request's temporary-file handling after this boundary.
	maxFormBodyBytes   int64 = 64 << 20
	maxFormMemoryBytes       = 32 << 20
)

// Params carries the parameter sources a legacy action could see, in binding
// priority order: path route params, then query, then form.
type Params struct {
	request *http.Request
	Path    map[string]string
	Query   map[string][]string
	Form    map[string][]string
	formOK  bool
}

func newParams(r *http.Request, path map[string]string) *Params {
	p := &Params{request: r, Path: path}
	if r.URL != nil {
		p.Query = r.URL.Query()
	}
	return p
}

// ensureForm parses the urlencoded/multipart form body once, on demand.
func (p *Params) ensureForm() {
	if p.formOK {
		return
	}
	p.formOK = true
	if p.request != nil && p.request.Method != http.MethodGet && p.request.Method != http.MethodHead {
		// ParseForm intentionally does not parse multipart fields.  Detect the
		// media type first so ordinary fields and file metadata are available to
		// the same adapter invocation, matching the old binder's semantics.
		if p.request.Body != nil {
			p.request.Body = http.MaxBytesReader(nil, p.request.Body, maxFormBodyBytes)
		}
		contentType, _, _ := mime.ParseMediaType(p.request.Header.Get("Content-Type"))
		if contentType == "multipart/form-data" {
			_ = p.request.ParseMultipartForm(maxFormMemoryBytes)
		} else {
			_ = p.request.ParseForm()
		}
		p.Form = p.request.PostForm
	}
}

// Get returns the first value for name across path/query/form.
func (p *Params) Get(name string) (string, bool) {
	if v, ok := p.Path[name]; ok && v != "" {
		return v, true
	}
	if vs, ok := p.Query[name]; ok && len(vs) > 0 {
		return vs[0], true
	}
	p.ensureForm()
	if vs, ok := p.Form[name]; ok && len(vs) > 0 {
		return vs[0], true
	}
	return "", false
}

// Has reports whether a parameter key is present in any request source. An
// explicitly supplied empty value still counts as present, matching the
// previous runtime's Params.Has semantics.
func (p *Params) Has(name string) bool {
	if _, ok := p.Path[name]; ok {
		return true
	}
	if _, ok := p.Query[name]; ok {
		return true
	}
	p.ensureForm()
	_, ok := p.Form[name]
	return ok
}

// Strings returns all values from the highest-priority source containing the
// key. Path parameters are scalar, while query/form values preserve repeated
// fields (including both Tags=x and Tags[]=x wire shapes).
func (p *Params) Strings(name string) []string {
	if value, ok := p.Path[name]; ok {
		return []string{value}
	}
	if values, ok := p.Query[name]; ok {
		return append([]string(nil), values...)
	}
	p.ensureForm()
	if values, ok := p.Form[name]; ok {
		return append([]string(nil), values...)
	}
	return nil
}

// NestedString reads a nested form/query key such as Files[0][LocalFileId].
// The caller supplies the collection name, zero-based index and field name;
// callers that already have a complete key can continue using Get directly.
func (p *Params) NestedString(collection string, index int, field string) (string, bool) {
	if index < 0 {
		return "", false
	}
	return p.Get(fmt.Sprintf("%s[%d][%s]", collection, index, field))
}

// Bool applies the legacy Atob vocabulary. Missing or unrecognised values return
// the caller's default, preserving the historical zero/default behaviour for
// fields that are not explicitly strict in an upstream contract.
func (p *Params) Bool(name string, def bool) bool {
	value, ok := p.Get(name)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t", "true", "y", "yes", "on", "1":
		return true
	case "f", "false", "n", "no", "off", "0":
		return false
	default:
		return def
	}
}

// StrictInt parses a required integer without falling back to a default.
func (p *Params) StrictInt(name string) (int, error) {
	value, ok := p.Get(name)
	if !ok || strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("parameter %q is required", name)
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("parameter %q must be an integer: %w", name, err)
	}
	return parsed, nil
}

// StrictObjectID parses a required 24-character hexadecimal ObjectID. The
// domain package owns the canonical codec and error classification.
func (p *Params) StrictObjectID(name string) (domain.ObjectID, error) {
	value, ok := p.Get(name)
	if !ok || strings.TrimSpace(value) == "" {
		return domain.ObjectID{}, fmt.Errorf("parameter %q is required", name)
	}
	id, err := domain.ParseObjectID(strings.TrimSpace(value))
	if err != nil || id.IsZero() {
		if err == nil {
			err = domain.ErrInvalidObjectID
		}
		return domain.ObjectID{}, fmt.Errorf("parameter %q must be a valid ObjectID: %w", name, err)
	}
	return id, nil
}

// String returns the first value or "".
func (p *Params) String(name string) string {
	v, _ := p.Get(name)
	return v
}

// Int returns the parsed integer value or def when missing/unparseable.
func (p *Params) Int(name string, def int) int {
	v, ok := p.Get(name)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// FormFile returns the first uploaded file for name (multipart bodies).
func (p *Params) FormFile(name string) (multipart.File, *multipart.FileHeader, error) {
	if p.request == nil {
		return nil, nil, http.ErrNotMultipart
	}
	// Route handlers may need the file before reading an ordinary field. Run
	// the same bounded form parser in that order too; Request.FormFile by
	// itself would parse the body without the request-wide limit.
	p.ensureForm()
	return p.request.FormFile(name)
}
