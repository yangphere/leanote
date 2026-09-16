package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
}

func (exporter *recordingAPIPDFExporter) Export(_ context.Context, actorID, noteID domain.ObjectID) (applicationcontent.PDFArtifact, error) {
	exporter.actorID = actorID
	exporter.noteID = noteID
	return applicationcontent.PDFArtifact{Filename: "api.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4\n%%EOF\n")}, nil
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
	if exporter.actorID.Hex() != "507f1f77bcf86cd799439011" || exporter.noteID.Hex() != "507f1f77bcf86cd799439012" {
		t.Fatalf("export identity actor=%s note=%s", exporter.actorID.Hex(), exporter.noteID.Hex())
	}
}
