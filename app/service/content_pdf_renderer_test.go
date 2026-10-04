package service

import (
	"context"
	"net/url"
	"testing"

	"github.com/yangphere/leanote/app/service/contentpdf"
)

func TestNewContentPDFBackendSelectsByExplicitKind(t *testing.T) {
	endpoint, err := url.Parse("http://gotenberg:3000")
	if err != nil {
		t.Fatal(err)
	}

	for _, kind := range []PDFRendererKind{"", PDFRendererProcess} {
		backend, selected, selectErr := newContentPDFBackend(PDFRendererConfig{Kind: kind}, t.TempDir())
		if selectErr != nil || selected != PDFRendererProcess {
			t.Fatalf("kind %q: selected=%q err=%v, want process", kind, selected, selectErr)
		}
		if _, ok := backend.(*contentpdf.ConfiguredBackend); !ok {
			t.Fatalf("kind %q: backend = %T, want *contentpdf.ConfiguredBackend", kind, backend)
		}
		// 进程模式仍读取管理配置：未配置可执行文件时 Descriptor 失败，而不是切到别的后端。
		previous := ConfigS
		ConfigS = nil
		_, descriptorErr := backend.Descriptor(context.Background())
		ConfigS = previous
		if descriptorErr == nil {
			t.Fatalf("kind %q: process backend produced a descriptor without exportPdfBinPath", kind)
		}
	}

	backend, selected, err := newContentPDFBackend(PDFRendererConfig{Kind: PDFRendererGotenberg, GotenbergURL: endpoint}, t.TempDir())
	if err != nil || selected != PDFRendererGotenberg {
		t.Fatalf("selected=%q err=%v, want gotenberg", selected, err)
	}
	if _, ok := backend.(*contentpdf.GotenbergBackend); !ok {
		t.Fatalf("backend = %T, want *contentpdf.GotenbergBackend", backend)
	}
}

func TestNewContentPDFBackendFailsClosed(t *testing.T) {
	for name, config := range map[string]PDFRendererConfig{
		"gotenberg without url": {Kind: PDFRendererGotenberg},
		"gotenberg bad scheme":  {Kind: PDFRendererGotenberg, GotenbergURL: &url.URL{Scheme: "ftp", Host: "gotenberg:3000"}},
		"unknown kind":          {Kind: "chromium"},
	} {
		if backend, _, err := newContentPDFBackend(config, t.TempDir()); err == nil || backend != nil {
			t.Fatalf("%s: backend=%v err=%v, want startup failure without fallback", name, backend, err)
		}
	}
}
