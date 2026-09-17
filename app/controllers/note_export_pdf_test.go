package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/revel/config"
	"github.com/revel/revel"
	"github.com/revel/revel/session"
	applicationcontent "github.com/yangphere/leanote/app/application/content"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/service"
)

type recordingPDFExporter struct {
	actorID domain.ObjectID
	noteID  domain.ObjectID
	err     error
}

func (exporter *recordingPDFExporter) Export(_ context.Context, actorID, noteID domain.ObjectID) (applicationcontent.PDFArtifact, error) {
	exporter.actorID = actorID
	exporter.noteID = noteID
	if exporter.err != nil {
		return applicationcontent.PDFArtifact{}, exporter.err
	}
	return applicationcontent.PDFArtifact{Filename: "safe.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4\n%%EOF\n")}, nil
}

func TestNoteExportPDFPreservesLegacyFailureBodies(t *testing.T) {
	saved := service.ContentPDF
	t.Cleanup(func() { service.ContentPDF = saved })
	tests := []struct {
		name   string
		noteID string
		err    error
		body   string
	}{
		{name: "invalid identity", noteID: "invalid", body: "error"},
		{name: "unauthorized", noteID: "507f1f77bcf86cd799439012", err: applicationcontent.NewError(applicationcontent.ErrorUnauthorized, "denied", nil), body: "No Perm"},
		{name: "renderer failure", noteID: "507f1f77bcf86cd799439012", err: applicationcontent.NewError(applicationcontent.ErrorRendererFailed, "failed", nil), body: "error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service.ContentPDF = &recordingPDFExporter{err: test.err}
			controller, recorder := newPDFTestController(t, "507f1f77bcf86cd799439011")
			result := (Note{BaseController: BaseController{Controller: controller}}).ExportPdf(test.noteID)
			result.Apply(controller.Request, controller.Response)
			if recorder.Body.String() != test.body {
				t.Fatalf("failure body=%q, want %q", recorder.Body.String(), test.body)
			}
		})
	}
}

func TestNoteExportPDFUsesSharedApplicationService(t *testing.T) {
	saved := service.ContentPDF
	exporter := &recordingPDFExporter{}
	service.ContentPDF = exporter
	t.Cleanup(func() { service.ContentPDF = saved })

	controller, recorder := newPDFTestController(t, "507f1f77bcf86cd799439011")
	result := (Note{BaseController: BaseController{Controller: controller}}).ExportPdf("507f1f77bcf86cd799439012")
	binary, ok := result.(*revel.BinaryResult)
	if !ok {
		t.Fatalf("ExportPdf() result = %T, want *revel.BinaryResult", result)
	}
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
	if recorder.Body.String() != "%PDF-1.4\n%%EOF\n" || binary.Name != "safe.pdf" || binary.Delivery != revel.Attachment || recorder.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("binary name=%q delivery=%q content-type=%q data=%q", binary.Name, binary.Delivery, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
	if exporter.actorID.Hex() != "507f1f77bcf86cd799439011" || exporter.noteID.Hex() != "507f1f77bcf86cd799439012" {
		t.Fatalf("export identity actor=%s note=%s", exporter.actorID.Hex(), exporter.noteID.Hex())
	}
}

func newPDFTestController(t *testing.T, userID string) (*revel.Controller, *httptest.ResponseRecorder) {
	t.Helper()
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodGet, "/note/exportPdf", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["UserId"] = userID
	return controller, recorder
}
