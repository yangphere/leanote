package httpserver

import (
	"html/template"

	i18n "github.com/yangphere/leanote/app/lea/i18n"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestTemplateFuncsNameSet freezes the template function name set: the 27
// active app/init.go registrations plus the three Revel builtins the views
// reference (set/append/pad) = 30.
func TestTemplateFuncsNameSet(t *testing.T) {
	want := []string{
		"raw", "trim", "add", "sub", "incr", "join", "concat", "concatStr",
		"decodeUrlValue", "json", "jsonJs", "datetime", "dateFormat",
		"unixDatetime", "has", "blogTags", "blogTagsForExport", "msg",
		"leaMsg", "blogTagsLea", "li", "urlConcat", "urlCond", "rawMsg",
		"sorterTh", "page", "N",
		"set", "append", "pad",
	}
	sort.Strings(want)
	funcs := TemplateFuncs()
	got := make([]string, 0, len(funcs))
	for name := range funcs {
		got = append(got, name)
	}
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("func count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("funcs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTemplateFuncsBasics(t *testing.T) {
	funcs := TemplateFuncs()
	call := func(name string, args ...interface{}) interface{} {
		return funcs[name].(func(...interface{}) interface{})
	}
	_ = call

	if got := funcs["add"].(func(int) string)(1); got != "2" {
		t.Fatalf("add(1) = %q", got)
	}
	if got := funcs["sub"].(func(int) int)(5); got != 4 {
		t.Fatalf("sub(5) = %d", got)
	}
	if got := funcs["incr"].(func(int, int) int)(3, 4); got != 7 {
		t.Fatalf("incr(3,4) = %d", got)
	}
	if got := funcs["decodeUrlValue"].(func(string) string)("a%20b"); got != "a b" {
		t.Fatalf("decodeUrlValue = %q", got)
	}
	if got := funcs["has"].(func(interface{}, string) bool)(struct{ Name string }{}, "Name"); !got {
		t.Fatal("has(Name) = false")
	}
	if got := funcs["urlConcat"].(func(string, ...interface{}) string)("x", "p", 1, "q", ""); got != "x?p=1" {
		t.Fatalf("urlConcat = %q", got)
	}
	if got := funcs["page"].(func(string, int, int, int) template.HTML)("/list", 1, 20, 0); got != "" {
		t.Fatalf("page(count=0) = %q, want empty", got)
	}
	now := time.Date(2026, 8, 29, 10, 30, 0, 0, time.UTC)
	if got := funcs["datetime"].(func(time.Time) template.HTML)(now); string(got) != "2026-08-29 10:30:00" {
		t.Fatalf("datetime = %q", got)
	}
}

func TestLoadTemplatesRealViews(t *testing.T) {
	const viewsDir = "../../app/views"
	if _, err := os.Stat(viewsDir); err != nil {
		t.Skipf("views dir not reachable: %v", err)
	}
	tpl, err := LoadTemplates(viewsDir)
	if err != nil {
		t.Fatalf("LoadTemplates(app/views): %v", err)
	}
	for _, name := range []string{
		"errors/404.html",
		"errors/500.html",
		"note/note.html",
	} {
		if tpl.Lookup(name) == nil {
			t.Errorf("template %q missing from set", name)
		}
	}
}

func TestTemplateSetRenderSmoke(t *testing.T) {
	i18n.DefaultLanguage = "en-us"
	defer func() { i18n.DefaultLanguage = "" }()
	dir := t.TempDir()
	file := filepath.Join(dir, "probe.html")
	content := `[{{leaMsg . "greeting"}}|{{.Name}}|{{urlConcat "/x" "p" 1}}]`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tpl, err := LoadTemplates(dir)
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	render := TemplateSetRenderer(tpl)
	out, err := render("probe.html", map[string]interface{}{
		currentLocaleViewArg: "en-us",
		"Name":               "leanote",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "greeting") || !strings.Contains(got, "leanote") || !strings.Contains(got, "/x?p=1") {
		t.Fatalf("unexpected render output: %q", got)
	}
}
