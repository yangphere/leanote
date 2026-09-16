package content

import (
	"bytes"
	"context"
	"errors"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

// RendererDescriptor is an opaque capability issued by the administrator-
// owned configuration provider.  It deliberately contains no executable path.
type RendererDescriptor struct {
	PolicyID     string
	ExecutableID string
}

// PDFBackend consumes an opaque descriptor and owns process execution.  A
// backend must revalidate the descriptor immediately before execution and
// return at most maxOutputBytes.
type PDFBackend interface {
	Descriptor(context.Context) (RendererDescriptor, error)
	Render(context.Context, RendererDescriptor, []byte, int64) ([]byte, error)
}

type PDFRenderRequest struct {
	Title     string
	HTML      string
	Markdown  bool
	Resources map[string]PDFResource
}

type PDFArtifact struct {
	Filename    string
	ContentType string
	Data        []byte
}

type PDFRenderer struct {
	Backend        PDFBackend
	Timeout        time.Duration
	MaxOutputBytes int64
}

func (renderer PDFRenderer) Render(ctx context.Context, request PDFRenderRequest) (PDFArtifact, error) {
	if renderer.Backend == nil || renderer.Timeout <= 0 || renderer.MaxOutputBytes <= 0 {
		return PDFArtifact{}, contentError(ErrorDependency, "pdf_renderer_unconfigured", nil)
	}
	document, err := SerializeSelfContainedPDF(PDFDocumentRequest{
		Title: request.Title, HTML: request.HTML, Markdown: request.Markdown, Resources: request.Resources,
	})
	if err != nil {
		return PDFArtifact{}, err
	}
	descriptor, err := renderer.Backend.Descriptor(ctx)
	if err != nil {
		return PDFArtifact{}, contentError(ErrorDependency, "pdf_descriptor", err)
	}
	if descriptor.PolicyID == "" || descriptor.ExecutableID == "" {
		return PDFArtifact{}, contentError(ErrorDependency, "pdf_descriptor_invalid", nil)
	}

	renderContext, cancel := context.WithTimeout(ctx, renderer.Timeout)
	defer cancel()
	pdf, err := renderer.Backend.Render(renderContext, descriptor, document, renderer.MaxOutputBytes)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(renderContext.Err(), context.DeadlineExceeded) {
			return PDFArtifact{}, contentError(ErrorTimeout, "pdf_render_timeout", errors.Join(err, renderContext.Err()))
		}
		var contentErr *Error
		if errors.As(err, &contentErr) {
			return PDFArtifact{}, err
		}
		return PDFArtifact{}, contentError(ErrorRendererFailed, "pdf_process", err)
	}
	if err := validatePDFArtifact(pdf, renderer.MaxOutputBytes); err != nil {
		return PDFArtifact{}, err
	}
	return PDFArtifact{
		Filename: safePDFName(request.Title), ContentType: "application/pdf", Data: append([]byte(nil), pdf...),
	}, nil
}

func validatePDFArtifact(pdf []byte, maxBytes int64) error {
	if len(pdf) == 0 || int64(len(pdf)) > maxBytes {
		return contentError(ErrorTooLarge, "pdf_output_limit", nil)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return contentError(ErrorRendererFailed, "pdf_magic", nil)
	}
	tail := pdf
	if len(tail) > 1024 {
		tail = tail[len(tail)-1024:]
	}
	if !bytes.Contains(tail, []byte("%%EOF")) {
		return contentError(ErrorRendererFailed, "pdf_eof", nil)
	}
	return nil
}

func safePDFName(title string) string {
	title = strings.ReplaceAll(title, "\\", "/")
	title = path.Base(title)
	var builder strings.Builder
	for _, r := range title {
		if r == 0 || r < 0x20 || r == 0x7f {
			continue
		}
		if r == ',' {
			r = '-'
		}
		builder.WriteRune(r)
	}
	title = strings.Trim(builder.String(), " .")
	if utf8.RuneCountInString(title) > 120 {
		runes := []rune(title)
		title = string(runes[:120])
	}
	if title == "" || title == "." {
		title = "Untitled"
	}
	return title + ".pdf"
}
