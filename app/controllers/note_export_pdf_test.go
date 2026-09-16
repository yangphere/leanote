package controllers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/revel"
	"github.com/revel/revel/session"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/service"
)

type recordingPDFExporter struct {
	actorID domain.ObjectID
	noteID  domain.ObjectID
}

func (exporter *recordingPDFExporter) Export(_ context.Context, actorID, noteID domain.ObjectID) (applicationcontent.PDFArtifact, error) {
	exporter.actorID = actorID
	exporter.noteID = noteID
	return applicationcontent.PDFArtifact{Filename: "safe.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4\n%%EOF\n")}, nil
}

func TestNoteExportPDFUsesSharedApplicationService(t *testing.T) {
	saved := service.ContentPDF
	exporter := &recordingPDFExporter{}
	service.ContentPDF = exporter
	t.Cleanup(func() { service.ContentPDF = saved })

	controller := newPDFTestController(t, "507f1f77bcf86cd799439011")
	result := (Note{BaseController: BaseController{Controller: controller}}).ExportPdf("507f1f77bcf86cd799439012")
	binary, ok := result.(*revel.BinaryResult)
	if !ok {
		t.Fatalf("ExportPdf() result = %T, want *revel.BinaryResult", result)
	}
	data, err := io.ReadAll(binary.Reader)
	if err != nil || string(data) != "%PDF-1.4\n%%EOF\n" || binary.Name != "safe.pdf" {
		t.Fatalf("binary name=%q data=%q err=%v", binary.Name, data, err)
	}
	if exporter.actorID.Hex() != "507f1f77bcf86cd799439011" || exporter.noteID.Hex() != "507f1f77bcf86cd799439012" {
		t.Fatalf("export identity actor=%s note=%s", exporter.actorID.Hex(), exporter.noteID.Hex())
	}
}

func newPDFTestController(t *testing.T, userID string) *revel.Controller {
	t.Helper()
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/note/exportPdf", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["UserId"] = userID
	return controller
}
