package httpserver

import (
	"mime/multipart"
	"net/http"
	"strconv"
)

// Params carries the parameter sources a Revel action could see, in binding
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
		_ = p.request.ParseForm()
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
	return p.request.FormFile(name)
}
