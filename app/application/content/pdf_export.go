package content

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/yangphere/leanote/app/domain"
	"golang.org/x/net/html"
)

type PDFNoteSnapshot struct {
	NoteID   domain.ObjectID
	OwnerID  domain.ObjectID
	Title    string
	HTML     string
	Markdown bool
}

type PDFNotePort interface {
	LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID) (PDFNoteSnapshot, error)
}

type PDFResourcePort interface {
	LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID) (PDFResource, error)
}

type PDFExportService struct {
	Notes     PDFNotePort
	Resources PDFResourcePort
	Renderer  PDFRenderer
}

func (service PDFExportService) Export(ctx context.Context, actorID, noteID domain.ObjectID) (PDFArtifact, error) {
	if service.Notes == nil || actorID.IsZero() || noteID.IsZero() {
		return PDFArtifact{}, validationError("invalid_pdf_export_identity", nil)
	}
	snapshot, err := service.Notes.LoadAuthorized(ctx, actorID, noteID)
	if err != nil {
		var contentErr *Error
		if errors.As(err, &contentErr) {
			return PDFArtifact{}, err
		}
		return PDFArtifact{}, contentError(ErrorDependency, "load_pdf_note", err)
	}
	if snapshot.NoteID != noteID || snapshot.OwnerID.IsZero() {
		return PDFArtifact{}, contentError(ErrorConflict, "pdf_note_snapshot_identity", nil)
	}
	references, err := referencedPDFResources(snapshot.HTML, snapshot.Markdown)
	if err != nil {
		return PDFArtifact{}, err
	}
	resources := make(map[string]PDFResource, len(references))
	loaded := make(map[domain.ObjectID]PDFResource, len(references))
	for raw, fileID := range references {
		resource, ok := loaded[fileID]
		if !ok {
			if service.Resources == nil {
				return PDFArtifact{}, contentError(ErrorDependency, "pdf_resource_port_missing", nil)
			}
			resource, err = service.Resources.LoadAuthorized(ctx, snapshot.OwnerID, fileID)
			if err != nil {
				var contentErr *Error
				if errors.As(err, &contentErr) {
					return PDFArtifact{}, err
				}
				return PDFArtifact{}, contentError(ErrorDependency, "load_pdf_resource", err)
			}
			loaded[fileID] = resource
		}
		resources[raw] = resource
	}
	return service.Renderer.Render(ctx, PDFRenderRequest{
		Title: snapshot.Title, HTML: snapshot.HTML, Markdown: snapshot.Markdown, Resources: resources,
	})
}

func referencedPDFResources(value string, markdown bool) (map[string]domain.ObjectID, error) {
	contentHTML, err := renderPDFContent(value, markdown)
	if err != nil {
		return nil, err
	}
	parsed, err := html.Parse(strings.NewReader("<body>" + contentHTML + "</body>"))
	if err != nil {
		return nil, contentError(ErrorUnsupportedMedia, "pdf_resource_html_parse", err)
	}
	references := make(map[string]domain.ObjectID)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "img") {
			for _, attr := range node.Attr {
				if !strings.EqualFold(attr.Key, "src") {
					continue
				}
				if fileID, ok := parseLocalPDFImage(attr.Val); ok {
					references[attr.Val] = fileID
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(parsed)
	return references, nil
}

func parseLocalPDFImage(value string) (domain.ObjectID, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Fragment != "" {
		return domain.ObjectID{}, false
	}
	if parsed.Path != "/file/outputImage" && parsed.Path != "/api/file/getImage" {
		return domain.ObjectID{}, false
	}
	query := parsed.Query()
	if len(query) != 1 || len(query["fileId"]) != 1 {
		return domain.ObjectID{}, false
	}
	fileID, err := domain.ParseObjectID(query.Get("fileId"))
	if err != nil || fileID.IsZero() {
		return domain.ObjectID{}, false
	}
	return fileID, true
}
