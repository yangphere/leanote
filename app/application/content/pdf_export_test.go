package content

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type fakePDFNotePort struct {
	snapshot PDFNoteSnapshot
	err      error
}

func (port fakePDFNotePort) LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID) (PDFNoteSnapshot, error) {
	return port.snapshot, port.err
}

type fakePDFResourcePort struct {
	resources map[domain.ObjectID]PDFResource
	owners    []domain.ObjectID
}

func (port *fakePDFResourcePort) LoadAuthorized(_ context.Context, ownerID, fileID domain.ObjectID) (PDFResource, error) {
	port.owners = append(port.owners, ownerID)
	resource, ok := port.resources[fileID]
	if !ok {
		return PDFResource{}, errors.New("not found")
	}
	return resource, nil
}

type failingPDFResourcePort struct{ err error }

func (port failingPDFResourcePort) LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID) (PDFResource, error) {
	return PDFResource{}, port.err
}

func TestPDFExportServiceLoadsOnlyAuthorizedLocalResources(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	fileID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	backend := &fakePDFBackend{
		descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"},
		result:     []byte("%PDF-1.4\n%%EOF\n"),
	}
	resources := &fakePDFResourcePort{resources: map[domain.ObjectID]PDFResource{
		fileID: {MIME: "image/png", Data: encodePNG(t, 1, 1)},
	}}
	service := PDFExportService{
		Notes: fakePDFNotePort{snapshot: PDFNoteSnapshot{
			NoteID: noteID, OwnerID: owner, Title: "note",
			HTML: `<img src="/file/outputImage?fileId=507f1f77bcf86cd799439014"><img src="https://evil.test/x.png">`,
		}},
		Resources: resources,
		Renderer:  PDFRenderer{Backend: backend, Timeout: time.Second, MaxOutputBytes: 1024},
	}
	artifact, err := service.Export(context.Background(), actor, noteID)
	if err != nil || artifact.Filename != "note.pdf" {
		t.Fatalf("artifact=%+v err=%v", artifact, err)
	}
	if len(resources.owners) != 1 || resources.owners[0] != owner {
		t.Fatalf("resource owner calls=%v", resources.owners)
	}
	if string(backend.document) == "" || containsAny(string(backend.document), "evil.test", "/file/outputImage") {
		t.Fatalf("backend document contains unresolved URL: %s", backend.document)
	}
}

func TestPDFExportServicePropagatesAuthorizationAndResourceFailures(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	service := PDFExportService{Notes: fakePDFNotePort{err: &Error{Category: ErrorUnauthorized, Code: "note"}}}
	if _, err := service.Export(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorUnauthorized {
		t.Fatalf("authorization error=%v", err)
	}
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	service = PDFExportService{
		Notes: fakePDFNotePort{snapshot: PDFNoteSnapshot{
			NoteID: noteID, OwnerID: owner, HTML: `<img src="/api/file/getImage?fileId=507f1f77bcf86cd799439014">`,
		}},
		Resources: &fakePDFResourcePort{resources: map[domain.ObjectID]PDFResource{}},
	}
	if _, err := service.Export(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorDependency {
		t.Fatalf("resource error=%v category=%q", err, errorCategoryOf(err))
	}
	service.Resources = failingPDFResourcePort{err: NewError(ErrorUnsafePath, "pdf_resource_path", nil)}
	if _, err := service.Export(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorUnsafePath {
		t.Fatalf("typed resource error=%v category=%q", err, errorCategoryOf(err))
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
