package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/revel/config"
	"github.com/revel/revel"
	"github.com/revel/revel/session"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/controllers"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/service"
)

type recordingAPIPDFExporter struct {
	actorID domain.ObjectID
	noteID  domain.ObjectID
	err     error
}

func (exporter *recordingAPIPDFExporter) Export(_ context.Context, actorID, noteID domain.ObjectID) (applicationcontent.PDFArtifact, error) {
	exporter.actorID = actorID
	exporter.noteID = noteID
	if exporter.err != nil {
		return applicationcontent.PDFArtifact{}, exporter.err
	}
	return applicationcontent.PDFArtifact{Filename: "api.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4\n%%EOF\n")}, nil
}

func TestAPIExportPDFPreservesLegacyFailureBodies(t *testing.T) {
	saved := service.ContentPDF
	t.Cleanup(func() { service.ContentPDF = saved })
	savedConfig := revel.Config
	revel.Config = config.NewContext()
	t.Cleanup(func() { revel.Config = savedConfig })
	tests := []struct {
		name   string
		noteID string
		err    error
		msg    string
	}{
		{name: "invalid identity", noteID: "invalid", msg: "noteNotExists"},
		{name: "not found", noteID: "507f1f77bcf86cd799439012", err: applicationcontent.NewError(applicationcontent.ErrorNotFound, "missing", nil), msg: "noteNotExists"},
		{name: "unauthorized", noteID: "507f1f77bcf86cd799439012", err: applicationcontent.NewError(applicationcontent.ErrorUnauthorized, "denied", nil), msg: "noteNotExists"},
		{name: "renderer failure", noteID: "507f1f77bcf86cd799439012", err: applicationcontent.NewError(applicationcontent.ErrorRendererFailed, "failed", nil), msg: "sysError"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service.ContentPDF = &recordingAPIPDFExporter{err: test.err}
			ctx := revel.NewGoContext(nil)
			ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/api/note/exportPdf", nil))
			recorder := httptest.NewRecorder()
			ctx.Response.Original = recorder
			ctx.Response.SetWriter(recorder)
			controller := revel.NewController(ctx)
			controller.Session = session.NewSession()
			controller.Session["_userId"] = "507f1f77bcf86cd799439011"
			action := ApiNote{ApiBaseContrller: ApiBaseContrller{BaseController: controllers.BaseController{Controller: controller}}}
			result := action.ExportPdf(test.noteID)
			result.Apply(controller.Request, controller.Response)
			var payload struct{ Msg string }
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || payload.Msg != test.msg {
				t.Fatalf("failure body=%q msg=%q want=%q err=%v", recorder.Body.String(), payload.Msg, test.msg, err)
			}
		})
	}
}

func TestAPIExportPDFUsesSharedApplicationService(t *testing.T) {
	saved := service.ContentPDF
	exporter := &recordingAPIPDFExporter{}
	service.ContentPDF = exporter
	t.Cleanup(func() { service.ContentPDF = saved })

	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/api/note/exportPdf", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["_userId"] = "507f1f77bcf86cd799439011"
	action := ApiNote{ApiBaseContrller: ApiBaseContrller{BaseController: controllers.BaseController{Controller: controller}}}

	result := action.ExportPdf("507f1f77bcf86cd799439012")
	binary, ok := result.(*revel.BinaryResult)
	if !ok {
		t.Fatalf("ExportPdf() result = %T, want *revel.BinaryResult", result)
	}
	data, err := io.ReadAll(binary.Reader)
	if err != nil || string(data) != "%PDF-1.4\n%%EOF\n" || binary.Name != "api.pdf" {
		t.Fatalf("binary name=%q data=%q err=%v", binary.Name, data, err)
	}
	binary.Reader = bytes.NewReader(data)
	savedConfig := revel.Config
	revel.Config = config.NewContext()
	t.Cleanup(func() { revel.Config = savedConfig })
	mimeRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(mimeRoot, "mime-types.conf"), []byte("[DEFAULT]\npdf = application/pdf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	savedConfPaths := revel.ConfPaths
	revel.ConfPaths = []string{mimeRoot}
	revel.LoadMimeConfig()
	t.Cleanup(func() { revel.ConfPaths = savedConfPaths })
	binary.Apply(controller.Request, controller.Response)
	if binary.Delivery != revel.Attachment || binary.Name != "api.pdf" || recorder.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("binary name=%q delivery=%q content-type=%q", binary.Name, binary.Delivery, recorder.Header().Get("Content-Type"))
	}
	if exporter.actorID.Hex() != "507f1f77bcf86cd799439011" || exporter.noteID.Hex() != "507f1f77bcf86cd799439012" {
		t.Fatalf("export identity actor=%s note=%s", exporter.actorID.Hex(), exporter.noteID.Hex())
	}
}
