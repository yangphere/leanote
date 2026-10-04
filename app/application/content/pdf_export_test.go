package content

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/yangphere/leanote/app/domain"
)

type fakePDFNotePort struct {
	snapshot PDFNoteSnapshot
	err      error
}

type blockingPDFNotePort struct{}

func (blockingPDFNotePort) LoadAuthorized(ctx context.Context, _, _ domain.ObjectID) (PDFNoteSnapshot, error) {
	<-ctx.Done()
	return PDFNoteSnapshot{}, ctx.Err()
}

func (port fakePDFNotePort) LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID) (PDFNoteSnapshot, error) {
	return port.snapshot, port.err
}

type fakePDFResourcePort struct {
	resources map[domain.ObjectID]PDFResource
	owners    []domain.ObjectID
	limits    []int64
}

func (port *fakePDFResourcePort) LoadAuthorized(_ context.Context, ownerID, fileID domain.ObjectID, maxBytes int64) (PDFResource, error) {
	port.owners = append(port.owners, ownerID)
	port.limits = append(port.limits, maxBytes)
	resource, ok := port.resources[fileID]
	if !ok {
		return PDFResource{}, errors.New("not found")
	}
	return resource, nil
}

type failingPDFResourcePort struct{ err error }

func (port failingPDFResourcePort) LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID, int64) (PDFResource, error) {
	return PDFResource{}, port.err
}

type fakePDFRemoteResourcePort struct {
	resources map[string]PDFResource
	urls      []string
	limits    []int64
}

func (port *fakePDFRemoteResourcePort) LoadPublic(_ context.Context, rawURL string, maxBytes int64) (PDFResource, error) {
	port.urls = append(port.urls, rawURL)
	port.limits = append(port.limits, maxBytes)
	resource, ok := port.resources[rawURL]
	if !ok {
		return PDFResource{}, errors.New("not found")
	}
	return resource, nil
}

func TestPDFExportServiceLoadsOnlyAuthorizedLocalResources(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	fileID, _ := domain.ParseObjectID("507f1f77bcf86cd799439014")
	backend := &fakePDFBackend{
		descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"},
		result:     validTestPDF(),
	}
	resources := &fakePDFResourcePort{resources: map[domain.ObjectID]PDFResource{
		fileID: {MIME: "image/png", Data: encodePNG(t, 1, 1)},
	}}
	service := PDFExportService{
		Notes: fakePDFNotePort{snapshot: PDFNoteSnapshot{
			NoteID: noteID, OwnerID: owner, Title: "note",
			HTML: `<img src="/file/outputImage?fileId=507f1f77bcf86cd799439014"><img src="file:///not-authorized.png">`,
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
	if string(backend.document) == "" || containsAny(string(backend.document), "file:///", "/file/outputImage") {
		t.Fatalf("backend document contains unresolved URL: %s", backend.document)
	}
}

func TestPDFExportServiceLoadsPublicRemoteResourcesOnceWithinProjectedBudget(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	image := encodePNG(t, 1, 1)
	remote := &fakePDFRemoteResourcePort{resources: map[string]PDFResource{
		"https://cdn.example/image.png": {MIME: "image/png", Data: image},
	}}
	backend := &fakePDFBackend{descriptor: RendererDescriptor{PolicyID: "p", ExecutableID: "e"}, result: validTestPDF()}
	service := PDFExportService{
		Notes: fakePDFNotePort{snapshot: PDFNoteSnapshot{
			NoteID: noteID, OwnerID: owner, Title: "remote",
			HTML: `<img src="https://cdn.example/image.png"><img src="https://CDN.EXAMPLE:443/image.png"><img src="https://cdn.example/image.png">`,
		}},
		RemoteResources: remote,
		Renderer:        PDFRenderer{Backend: backend, Timeout: time.Second, MaxOutputBytes: 64 * 1024 * 1024},
	}
	artifact, err := service.Export(context.Background(), actor, noteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remote.urls) != 1 || remote.urls[0] != "https://cdn.example/image.png" || len(remote.limits) != 1 || remote.limits[0] <= 0 || remote.limits[0] >= 64*1024*1024 {
		t.Fatalf("remote calls=%v limits=%v", remote.urls, remote.limits)
	}
	if strings.Contains(string(backend.document), "https://") || strings.Count(string(backend.document), "data:image/png;base64,") != 3 || artifact.ContentType != "application/pdf" {
		t.Fatalf("document=%s artifact=%+v", backend.document, artifact)
	}
}

func TestProjectedPDFResourceCostRejectsIntegerOverflow(t *testing.T) {
	for _, test := range []struct {
		raw         int64
		mimeBytes   int
		occurrences int64
	}{
		{raw: math.MaxInt64, mimeBytes: len("image/png"), occurrences: 1},
		{raw: math.MaxInt64 / 2, mimeBytes: len("image/png"), occurrences: 3},
		{raw: 1, mimeBytes: len("image/png"), occurrences: math.MaxInt64},
	} {
		if cost, ok := projectedPDFResourceCost(test.raw, test.mimeBytes, test.occurrences); ok || cost != 0 {
			t.Fatalf("projectedPDFResourceCost(%d, %d, %d)=(%d, %v), want overflow rejection", test.raw, test.mimeBytes, test.occurrences, cost, ok)
		}
	}
}

func TestPDFExportServiceStopsBeforeWorkWhenCallerCanceled(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (PDFExportService{Notes: fakePDFNotePort{}}).Export(ctx, actor, noteID)
	if errorCategoryOf(err) != ErrorTimeout || !errors.Is(err, context.Canceled) {
		t.Fatalf("Export() cancellation error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestPDFExportServiceAppliesOneTimeoutAcrossNoteLoadAndRender(t *testing.T) {
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	service := PDFExportService{
		Notes:    blockingPDFNotePort{},
		Renderer: PDFRenderer{Timeout: 10 * time.Millisecond},
	}
	_, err := service.Export(context.Background(), actor, noteID)
	if errorCategoryOf(err) != ErrorTimeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Export() timeout error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestPDFExportServiceRejectsInputOverHardCapBeforeResourceLoading(t *testing.T) {
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	actor, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	noteID, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	remote := &fakePDFRemoteResourcePort{}
	service := PDFExportService{
		Notes:           fakePDFNotePort{snapshot: PDFNoteSnapshot{NoteID: noteID, OwnerID: owner, HTML: strings.Repeat("x", 32*1024*1024+1)}},
		RemoteResources: remote,
	}
	if _, err := service.Export(context.Background(), actor, noteID); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("Export() error=%v category=%q", err, errorCategoryOf(err))
	}
	if len(remote.urls) != 0 {
		t.Fatalf("remote resources loaded after input cap: %v", remote.urls)
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
