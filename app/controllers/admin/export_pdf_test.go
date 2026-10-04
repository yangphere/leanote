package admin

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/service"
)

func setPDFRendererKindForTest(t *testing.T, kind service.PDFRendererKind) {
	t.Helper()
	previous := pdfRendererKind
	pdfRendererKind = func() service.PDFRendererKind { return kind }
	t.Cleanup(func() { pdfRendererKind = previous })
}

func TestViewArgsExposeReadOnlyPDFRendererState(t *testing.T) {
	for kind, want := range map[service.PDFRendererKind]bool{
		service.PDFRendererGotenberg: true,
		service.PDFRendererProcess:   false,
	} {
		setPDFRendererKindForTest(t, kind)
		args := (&server{}).viewArgs(&httpserver.Context{})
		if got, _ := args["pdfRendererGotenberg"].(bool); got != want {
			t.Fatalf("kind %q: pdfRendererGotenberg = %v, want %v", kind, args["pdfRendererGotenberg"], want)
		}
	}
}

// Gotenberg 模式下即使提交了合法的可执行文件路径，也必须被拒绝且不触达配置存储。
func TestExportPdfWriteRejectedInGotenbergMode(t *testing.T) {
	setPDFRendererKindForTest(t, service.PDFRendererGotenberg)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	previous := configService
	// 空的 ConfigService 没有 Mongo 连接；若拒绝逻辑缺失，写入路径会在此处失败或 panic。
	configService = &service.ConfigService{}
	t.Cleanup(func() { configService = previous })

	c := &httpserver.Context{
		Action: "ExportPdf",
		Params: &httpserver.Params{Form: map[string][]string{"path": {executable}}},
	}
	recorder := httptest.NewRecorder()
	(&server{}).settingWrite(c).Apply(recorder, httptest.NewRequest("POST", "/adminSetting/exportPdf", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `"Ok":false`) || !strings.Contains(body, `"Msg":"admin.validation"`) {
		t.Fatalf("response = %s, want admin.validation rejection", body)
	}
}

func renderExportPDFTemplate(t *testing.T, kind service.PDFRendererKind) string {
	t.Helper()
	const viewsDir = "../../views"
	if _, err := os.Stat(viewsDir); err != nil {
		t.Skipf("views dir not reachable: %v", err)
	}
	templates, err := httpserver.LoadTemplates(viewsDir)
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	setPDFRendererKindForTest(t, kind)
	previous := configService
	configService = &service.ConfigService{GlobalStringConfigs: map[string]string{"exportPdfBinPath": "/usr/bin/wkhtmltopdf"}}
	t.Cleanup(func() { configService = previous })
	args := (&server{}).viewArgs(&httpserver.Context{Locale: "en-us"})
	out, err := httpserver.TemplateSetRenderer(templates)("admin/setting/export_pdf.html", args)
	if err != nil {
		t.Fatalf("render export_pdf.html: %v", err)
	}
	return string(out)
}

func TestExportPdfTemplateShowsReadOnlyStatusInGotenbergMode(t *testing.T) {
	html := renderExportPDFTemplate(t, service.PDFRendererGotenberg)
	if !strings.Contains(html, "Gotenberg") || !strings.Contains(html, "read-only") {
		t.Fatalf("Gotenberg mode page lacks read-only status")
	}
	for _, forbidden := range []string{`name="path"`, "add_user_form", "/adminSetting/exportPdf", "exportPdfBinPath", "/usr/bin/wkhtmltopdf"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("Gotenberg mode page still contains %q", forbidden)
		}
	}
}

func TestExportPdfTemplateKeepsProcessFormInProcessMode(t *testing.T) {
	html := renderExportPDFTemplate(t, service.PDFRendererProcess)
	for _, required := range []string{`name="path"`, "add_user_form", "/adminSetting/exportPdf", "/usr/bin/wkhtmltopdf"} {
		if !strings.Contains(html, required) {
			t.Fatalf("process mode page lost %q", required)
		}
	}
	if strings.Contains(html, "read-only") {
		t.Fatal("process mode page shows the read-only Gotenberg notice")
	}
}
