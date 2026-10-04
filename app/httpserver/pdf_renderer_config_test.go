package httpserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yangphere/leanote/app/service"
)

func pdfRendererTestConfig(t *testing.T, lines string) *Config {
	t.Helper()
	cfg, err := ParseConfig([]byte("db.dbname=leanote_test\ndb.host=127.0.0.1\ndb.port=27017\n[test]\n"+lines), "test")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	return cfg
}

func TestParsePDFRendererConfigDefaultsToProcess(t *testing.T) {
	for _, lines := range []string{"", "pdf.renderer=\n", "pdf.renderer=process\n"} {
		got, err := parsePDFRendererConfig(pdfRendererTestConfig(t, lines))
		if err != nil {
			t.Fatalf("%q: %v", lines, err)
		}
		if got.Kind != service.PDFRendererProcess || got.GotenbergURL != nil {
			t.Fatalf("%q: config = %+v, want process mode without a Gotenberg URL", lines, got)
		}
	}
}

func TestParsePDFRendererConfigAcceptsGotenberg(t *testing.T) {
	got, err := parsePDFRendererConfig(pdfRendererTestConfig(t, "pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != service.PDFRendererGotenberg || got.GotenbergURL == nil || got.GotenbergURL.String() != "http://gotenberg:3000" {
		t.Fatalf("config = %+v", got)
	}
}

func TestParsePDFRendererConfigFailsClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		lines string
		code  string
		key   string
	}{
		"unknown renderer": {"pdf.renderer=chromium\n", "CONFIG_PDF_RENDERER_INVALID", "pdf.renderer"},
		"case sensitive":   {"pdf.renderer=Gotenberg\npdf.gotenberg.url=http://gotenberg:3000\n", "CONFIG_PDF_RENDERER_INVALID", "pdf.renderer"},
		"missing url":      {"pdf.renderer=gotenberg\n", "CONFIG_VALUE_MISSING", "pdf.gotenberg.url"},
		"blank url":        {"pdf.renderer=gotenberg\npdf.gotenberg.url=\n", "CONFIG_VALUE_MISSING", "pdf.gotenberg.url"},
		"non http scheme":  {"pdf.renderer=gotenberg\npdf.gotenberg.url=ftp://gotenberg:3000\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"no scheme":        {"pdf.renderer=gotenberg\npdf.gotenberg.url=gotenberg:3000\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"userinfo":         {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://user:pw@gotenberg:3000\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"query":            {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000?a=b\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"fragment":         {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000#x\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"path":             {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000/forms\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"empty host":       {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://:3000\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
		"unparseable url":  {"pdf.renderer=gotenberg\npdf.gotenberg.url=http://%zz\n", "CONFIG_PDF_RENDERER_URL_INVALID", "pdf.gotenberg.url"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parsePDFRendererConfig(pdfRendererTestConfig(t, tc.lines))
			var configErr *ConfigError
			if !errors.As(err, &configErr) || configErr.Code != tc.code || configErr.Key != tc.key {
				t.Fatalf("error = %v, want code=%s key=%s", err, tc.code, tc.key)
			}
		})
	}
}

// dev/test 与 prod 共用同一解析入口：校验失败必须阻止 ProductionConfig 交接。
func TestValidateLocalRuntimeConfigHandsOffPDFRenderer(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	runtime, err := ValidateLocalRuntimeConfig(pdfRendererTestConfig(t, ""), "test", base)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.PDFRenderer.Kind != service.PDFRendererProcess {
		t.Fatalf("default renderer = %q, want process", runtime.PDFRenderer.Kind)
	}
	runtime, err = ValidateLocalRuntimeConfig(pdfRendererTestConfig(t, "pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000\n"), "test", base)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.PDFRenderer.Kind != service.PDFRendererGotenberg || runtime.PDFRenderer.GotenbergURL.Host != "gotenberg:3000" {
		t.Fatalf("renderer = %+v, want Gotenberg handoff", runtime.PDFRenderer)
	}
	if _, err = ValidateLocalRuntimeConfig(pdfRendererTestConfig(t, "pdf.renderer=gotenberg\n"), "test", base); err == nil {
		t.Fatal("Gotenberg without URL accepted")
	}
	if _, err = ValidateLocalRuntimeConfig(pdfRendererTestConfig(t, "pdf.renderer=bogus\n"), "test", base); err == nil {
		t.Fatal("unknown renderer accepted")
	}
}

// 生产路径同样走共用解析入口：Gotenberg 地址原样交接，非法配置阻止启动。
func TestValidateProductionRuntimeConfigHandsOffPDFRenderer(t *testing.T) {
	base := t.TempDir()
	mkdir := func(name string) string {
		path := filepath.Join(base, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	roots := fmt.Sprintf("content.private.data=%s\ncontent.private.quarantine=%s\ncontent.public.data=%s\ncontent.public.quarantine=%s\ncontent.temporary=%s\nadmin.backup.root=%s\n",
		mkdir("private-files"), mkdir("private-quarantine"), mkdir("public-upload"), mkdir("public-quarantine"), mkdir("temporary"), mkdir("backup"))
	publicStatic := mkdir("public-static")
	t.Setenv("MONGODB_URL", "mongodb://db.example/leanote")
	build := func(extra string) (*ProductionConfig, error) {
		cfg, err := ParseConfig([]byte("[prod]\ndb.dbname=leanote\n"+roots+extra), "prod")
		if err != nil {
			t.Fatal(err)
		}
		return ValidateProductionRuntimeConfig(cfg, publicStatic)
	}

	runtime, err := build("")
	if err != nil || runtime.PDFRenderer.Kind != service.PDFRendererProcess {
		t.Fatalf("default = %+v err=%v, want process", runtime, err)
	}
	runtime, err = build("pdf.renderer=gotenberg\npdf.gotenberg.url=http://gotenberg:3000\n")
	if err != nil || runtime.PDFRenderer.Kind != service.PDFRendererGotenberg || runtime.PDFRenderer.GotenbergURL.String() != "http://gotenberg:3000" {
		t.Fatalf("gotenberg = %+v err=%v", runtime, err)
	}
	for _, extra := range []string{"pdf.renderer=gotenberg\n", "pdf.renderer=nope\n", "pdf.renderer=gotenberg\npdf.gotenberg.url=http://u:p@gotenberg:3000\n"} {
		if _, err = build(extra); err == nil {
			t.Fatalf("config %q accepted", extra)
		}
	}
}
