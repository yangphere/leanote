package content

import (
	"context"
	"errors"
	"math"
	"net"
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
	LoadAuthorized(context.Context, domain.ObjectID, domain.ObjectID, int64) (PDFResource, error)
}

type PDFRemoteResourcePort interface {
	LoadPublic(context.Context, string, int64) (PDFResource, error)
}

type PDFExportService struct {
	Notes           PDFNotePort
	Resources       PDFResourcePort
	RemoteResources PDFRemoteResourcePort
	Renderer        PDFRenderer
}

func (service PDFExportService) Export(ctx context.Context, actorID, noteID domain.ObjectID) (PDFArtifact, error) {
	if err := pdfContextError(ctx, "pdf_export_canceled"); err != nil {
		return PDFArtifact{}, err
	}
	if service.Notes == nil || actorID.IsZero() || noteID.IsZero() {
		return PDFArtifact{}, validationError("invalid_pdf_export_identity", nil)
	}
	if service.Renderer.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, service.Renderer.Timeout)
		defer cancel()
	}
	snapshot, err := service.Notes.LoadAuthorized(ctx, actorID, noteID)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return PDFArtifact{}, contentError(ErrorTimeout, "load_pdf_note_canceled", errors.Join(err, ctx.Err()))
		}
		var contentErr *Error
		if errors.As(err, &contentErr) {
			return PDFArtifact{}, err
		}
		return PDFArtifact{}, contentError(ErrorDependency, "load_pdf_note", err)
	}
	if snapshot.NoteID != noteID || snapshot.OwnerID.IsZero() {
		return PDFArtifact{}, contentError(ErrorConflict, "pdf_note_snapshot_identity", nil)
	}
	if len(snapshot.HTML) > maxPDFInputBytes {
		return PDFArtifact{}, contentError(ErrorTooLarge, "pdf_html_limit", nil)
	}
	references, err := referencedPDFResources(ctx, snapshot.HTML, snapshot.Markdown)
	if err != nil {
		return PDFArtifact{}, err
	}
	baseDocument, err := serializeSelfContainedPDF(ctx, PDFDocumentRequest{
		Title: snapshot.Title, HTML: snapshot.HTML, Markdown: snapshot.Markdown,
	})
	if err != nil {
		return PDFArtifact{}, err
	}
	remaining := int64(maxPDFDocumentBytes - len(baseDocument))
	resources := make(map[string]PDFResource, len(references))
	for _, reference := range references {
		if err := ctx.Err(); err != nil {
			return PDFArtifact{}, contentError(ErrorTimeout, "pdf_resource_canceled", err)
		}
		maxRaw := maxPDFRawBytesForBudget(remaining, reference.occurrences)
		if maxRaw <= 0 {
			return PDFArtifact{}, contentError(ErrorTooLarge, "pdf_resource_budget", nil)
		}
		var resource PDFResource
		if !reference.localID.IsZero() {
			if service.Resources == nil {
				return PDFArtifact{}, contentError(ErrorDependency, "pdf_resource_port_missing", nil)
			}
			resource, err = service.Resources.LoadAuthorized(ctx, snapshot.OwnerID, reference.localID, maxRaw)
		} else {
			if service.RemoteResources == nil {
				return PDFArtifact{}, contentError(ErrorDependency, "pdf_remote_resource_port_missing", nil)
			}
			resource, err = service.RemoteResources.LoadPublic(ctx, reference.fetchURL, maxRaw)
		}
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return PDFArtifact{}, contentError(ErrorTimeout, "load_pdf_resource_canceled", errors.Join(err, ctx.Err()))
			}
			var contentErr *Error
			if errors.As(err, &contentErr) {
				return PDFArtifact{}, err
			}
			return PDFArtifact{}, contentError(ErrorDependency, "load_pdf_resource", err)
		}
		cost, ok := projectedPDFResourceCost(int64(len(resource.Data)), len(resource.MIME), reference.occurrences)
		if !ok || cost > remaining {
			return PDFArtifact{}, contentError(ErrorTooLarge, "pdf_resource_budget", nil)
		}
		remaining -= cost
		for raw := range reference.rawValues {
			resources[raw] = resource
		}
	}
	return service.Renderer.Render(ctx, PDFRenderRequest{
		Title: snapshot.Title, HTML: snapshot.HTML, Markdown: snapshot.Markdown, Resources: resources,
	})
}

type pdfResourceReference struct {
	key         string
	fetchURL    string
	localID     domain.ObjectID
	occurrences int64
	rawValues   map[string]struct{}
}

func referencedPDFResources(ctx context.Context, value string, markdown bool) ([]pdfResourceReference, error) {
	if err := pdfContextError(ctx, "pdf_resource_scan_canceled"); err != nil {
		return nil, err
	}
	contentHTML, err := renderPDFContent(value, markdown)
	if err != nil {
		return nil, err
	}
	parsed, err := html.Parse(strings.NewReader("<body>" + contentHTML + "</body>"))
	if err != nil {
		return nil, contentError(ErrorUnsupportedMedia, "pdf_resource_html_parse", err)
	}
	references := make([]pdfResourceReference, 0)
	indices := make(map[string]int)
	var walk func(*html.Node) error
	walk = func(node *html.Node) error {
		if err := pdfContextError(ctx, "pdf_resource_scan_canceled"); err != nil {
			return err
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "img") {
			for _, attr := range node.Attr {
				if !strings.EqualFold(attr.Key, "src") {
					continue
				}
				reference, ok, err := classifyPDFResource(attr.Val)
				if err != nil {
					return err
				}
				if ok {
					if index, exists := indices[reference.key]; exists {
						references[index].occurrences++
						references[index].rawValues[attr.Val] = struct{}{}
					} else {
						reference.occurrences = 1
						reference.rawValues = map[string]struct{}{attr.Val: {}}
						indices[reference.key] = len(references)
						references = append(references, reference)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(parsed); err != nil {
		return nil, err
	}
	return references, nil
}

func classifyPDFResource(value string) (pdfResourceReference, bool, error) {
	if fileID, ok := parseLocalPDFImage(value); ok {
		return pdfResourceReference{key: "local:" + fileID.Hex(), localID: fileID}, true, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return pdfResourceReference{}, false, nil
	}
	validated, err := ValidateRemoteURL(value)
	if err != nil {
		return pdfResourceReference{}, false, err
	}
	validated.Scheme = strings.ToLower(validated.Scheme)
	hostname := strings.ToLower(validated.Hostname())
	port := validated.Port()
	if (validated.Scheme == "http" && port == "80") || (validated.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		validated.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		validated.Host = "[" + hostname + "]"
	} else {
		validated.Host = hostname
	}
	if validated.Path == "" {
		validated.Path = "/"
	}
	canonical := validated.String()
	return pdfResourceReference{key: "remote:" + canonical, fetchURL: canonical}, true, nil
}

func maxPDFRawBytesForBudget(remaining, occurrences int64) int64 {
	if remaining <= 0 || occurrences <= 0 {
		return 0
	}
	low, high := int64(0), remaining
	for low < high {
		mid := low + (high-low+1)/2
		cost, ok := projectedPDFResourceCost(mid, len("image/jpeg"), occurrences)
		if ok && cost <= remaining {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return low
}

func projectedPDFResourceCost(raw int64, mimeBytes int, occurrences int64) (int64, bool) {
	if raw <= 0 || mimeBytes <= 0 || occurrences <= 0 {
		return 0, false
	}
	if raw > math.MaxInt64-2 {
		return 0, false
	}
	encodedUnits := (raw + 2) / 3
	if encodedUnits > math.MaxInt64/4 {
		return 0, false
	}
	encoded := encodedUnits * 4
	prefixBytes := int64(len(` src="data:`) + mimeBytes + len(`;base64,"`))
	if encoded > math.MaxInt64-prefixBytes {
		return 0, false
	}
	perReference := prefixBytes + encoded
	if perReference > math.MaxInt64/occurrences {
		return 0, false
	}
	projected := perReference * occurrences
	if raw > math.MaxInt64-projected {
		return 0, false
	}
	return raw + projected, true
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
