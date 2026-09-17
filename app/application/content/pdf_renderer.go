package content

import (
	"bytes"
	"context"
	"errors"
	"path"
	"regexp"
	"strconv"
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
	renderContext, cancel := context.WithTimeout(ctx, renderer.Timeout)
	defer cancel()
	document, err := serializeSelfContainedPDF(renderContext, PDFDocumentRequest{
		Title: request.Title, HTML: request.HTML, Markdown: request.Markdown, Resources: request.Resources,
	})
	if err != nil {
		return PDFArtifact{}, err
	}
	descriptor, err := renderer.Backend.Descriptor(renderContext)
	if err != nil {
		return PDFArtifact{}, contentError(ErrorDependency, "pdf_descriptor", err)
	}
	if descriptor.PolicyID == "" || descriptor.ExecutableID == "" {
		return PDFArtifact{}, contentError(ErrorDependency, "pdf_descriptor_invalid", nil)
	}

	maxOutputBytes := renderer.MaxOutputBytes
	if maxOutputBytes > maxPDFDocumentBytes {
		maxOutputBytes = maxPDFDocumentBytes
	}
	pdf, err := renderer.Backend.Render(renderContext, descriptor, document, maxOutputBytes)
	if err != nil {
		if renderContext.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return PDFArtifact{}, contentError(ErrorTimeout, "pdf_render_timeout", errors.Join(err, renderContext.Err()))
		}
		var contentErr *Error
		if errors.As(err, &contentErr) {
			return PDFArtifact{}, err
		}
		return PDFArtifact{}, contentError(ErrorRendererFailed, "pdf_process", err)
	}
	if err := validatePDFArtifact(pdf, maxOutputBytes); err != nil {
		return PDFArtifact{}, err
	}
	return PDFArtifact{
		Filename: safePDFName(request.Title), ContentType: "application/pdf", Data: append([]byte(nil), pdf...),
	}, nil
}

func validatePDFArtifact(pdf []byte, maxBytes int64) error {
	if maxBytes <= 0 || maxBytes > maxPDFDocumentBytes {
		maxBytes = maxPDFDocumentBytes
	}
	if len(pdf) == 0 || int64(len(pdf)) > maxBytes {
		return contentError(ErrorTooLarge, "pdf_output_limit", nil)
	}
	if len(pdf) <= len("%PDF-1.0") || !bytes.HasPrefix(pdf, []byte("%PDF-")) || pdf[5] < '1' || pdf[5] > '2' || pdf[6] != '.' || pdf[7] < '0' || pdf[7] > '9' || (pdf[8] != '\r' && pdf[8] != '\n') {
		return contentError(ErrorRendererFailed, "pdf_magic", nil)
	}
	trimmed := bytes.TrimSpace(pdf)
	if !bytes.HasSuffix(trimmed, []byte("%%EOF")) {
		return contentError(ErrorRendererFailed, "pdf_eof", nil)
	}
	startMarker := []byte("startxref")
	startIndex := bytes.LastIndex(trimmed, startMarker)
	if startIndex < 0 {
		return contentError(ErrorRendererFailed, "pdf_startxref", nil)
	}
	offsetFields := bytes.Fields(trimmed[startIndex+len(startMarker) : len(trimmed)-len("%%EOF")])
	if len(offsetFields) != 1 {
		return contentError(ErrorRendererFailed, "pdf_startxref", nil)
	}
	offsetText := offsetFields[0]
	offset, err := strconv.ParseInt(string(offsetText), 10, 64)
	if err != nil || offset < int64(len("%PDF-1.0")) || offset >= int64(startIndex) {
		return contentError(ErrorRendererFailed, "pdf_startxref", err)
	}
	xref := trimmed[offset:startIndex]
	if !bytes.HasPrefix(xref, []byte("xref\n")) && !bytes.HasPrefix(xref, []byte("xref\r\n")) {
		return contentError(ErrorRendererFailed, "pdf_xref", nil)
	}
	trailerMatch := pdfTrailerLine.FindIndex(xref)
	if trailerMatch == nil || trailerMatch[0] <= len("xref") {
		return contentError(ErrorRendererFailed, "pdf_trailer", nil)
	}
	trailer := bytes.TrimSpace(xref[trailerMatch[1]:])
	if !bytes.HasPrefix(trailer, []byte("<<")) || !bytes.HasSuffix(trailer, []byte(">>")) {
		return contentError(ErrorRendererFailed, "pdf_trailer", nil)
	}
	trailerTokens, ok := scrubPDFTrailerLiterals(trailer)
	if !ok {
		return contentError(ErrorRendererFailed, "pdf_trailer", nil)
	}
	size, ok := singlePDFInteger(trailerTokens, pdfTrailerSize)
	if !ok || size <= 0 {
		return contentError(ErrorRendererFailed, "pdf_trailer", nil)
	}
	rootObject, rootGeneration, ok := singlePDFReference(trailerTokens, pdfTrailerRoot)
	if !ok || rootObject < 0 || rootObject >= size || rootGeneration < 0 {
		return contentError(ErrorRendererFailed, "pdf_trailer", nil)
	}
	if !validatePDFXRef(xref[:trailerMatch[0]], rootObject, rootGeneration, size, trimmed, offset) {
		return contentError(ErrorRendererFailed, "pdf_xref", nil)
	}
	return nil
}

var (
	pdfTrailerLine = regexp.MustCompile(`(?m)^trailer[ \t]*\r?$`)
	pdfTrailerSize = regexp.MustCompile(`(?:^|[\s<>\[\]])/Size[ \t\r\n]+([0-9]+)(?:[\s<>\[\]]|$)`)
	pdfTrailerRoot = regexp.MustCompile(`(?:^|[\s<>\[\]])/Root[ \t\r\n]+([0-9]+)[ \t\r\n]+([0-9]+)[ \t\r\n]+R(?:[\s<>\[\]]|$)`)
)

func singlePDFInteger(document []byte, pattern *regexp.Regexp) (int64, bool) {
	matches := pattern.FindAllSubmatch(document, -1)
	if len(matches) != 1 {
		return 0, false
	}
	value, err := strconv.ParseInt(string(matches[0][1]), 10, 64)
	return value, err == nil
}

func singlePDFReference(document []byte, pattern *regexp.Regexp) (int64, int64, bool) {
	matches := pattern.FindAllSubmatch(document, -1)
	if len(matches) != 1 {
		return 0, 0, false
	}
	object, objectErr := strconv.ParseInt(string(matches[0][1]), 10, 64)
	generation, generationErr := strconv.ParseInt(string(matches[0][2]), 10, 64)
	return object, generation, objectErr == nil && generationErr == nil
}

func scrubPDFTrailerLiterals(document []byte) ([]byte, bool) {
	result := append([]byte(nil), document...)
	for index := 0; index < len(result); index++ {
		switch result[index] {
		case '%':
			for index < len(result) && result[index] != '\r' && result[index] != '\n' {
				result[index] = ' '
				index++
			}
		case '(':
			depth := 1
			result[index] = ' '
			for index++; index < len(result) && depth > 0; index++ {
				if result[index] == '\\' {
					result[index] = ' '
					if index+1 < len(result) {
						index++
						result[index] = ' '
					}
					continue
				}
				if result[index] == '(' {
					depth++
				} else if result[index] == ')' {
					depth--
				}
				result[index] = ' '
			}
			if depth != 0 {
				return nil, false
			}
			index--
		case '<':
			if index+1 < len(result) && result[index+1] == '<' {
				index++
				continue
			}
			result[index] = ' '
			closed := false
			for index++; index < len(result); index++ {
				if result[index] == '>' {
					result[index] = ' '
					closed = true
					break
				}
				result[index] = ' '
			}
			if !closed {
				return nil, false
			}
		}
	}
	return result, true
}

func validatePDFXRef(section []byte, rootObject, rootGeneration, size int64, pdf []byte, xrefOffset int64) bool {
	section = bytes.ReplaceAll(section, []byte("\r\n"), []byte("\n"))
	section = bytes.ReplaceAll(section, []byte("\r"), []byte("\n"))
	lines := bytes.Split(section, []byte("\n"))
	if len(lines) == 0 || !bytes.Equal(bytes.TrimSpace(lines[0]), []byte("xref")) {
		return false
	}
	var rootOffset int64 = -1
	for line := 1; line < len(lines); {
		if len(bytes.TrimSpace(lines[line])) == 0 {
			line++
			continue
		}
		header := bytes.Fields(lines[line])
		if len(header) != 2 {
			return false
		}
		start, startErr := strconv.ParseInt(string(header[0]), 10, 64)
		count, countErr := strconv.ParseInt(string(header[1]), 10, 64)
		if startErr != nil || countErr != nil || start < 0 || count <= 0 || start >= size || count > size-start {
			return false
		}
		line++
		for entry := int64(0); entry < count; entry++ {
			if line >= len(lines) {
				return false
			}
			fields := bytes.Fields(lines[line])
			if len(fields) != 3 || (string(fields[2]) != "n" && string(fields[2]) != "f") {
				return false
			}
			objectOffset, offsetErr := strconv.ParseInt(string(fields[0]), 10, 64)
			generation, generationErr := strconv.ParseInt(string(fields[1]), 10, 64)
			if offsetErr != nil || generationErr != nil || objectOffset < 0 || generation < 0 {
				return false
			}
			if start+entry == rootObject && generation == rootGeneration && string(fields[2]) == "n" {
				rootOffset = objectOffset
			}
			line++
		}
	}
	if rootOffset < int64(len("%PDF-1.0")) || rootOffset >= xrefOffset {
		return false
	}
	objectPattern := regexp.MustCompile(`^` + strconv.FormatInt(rootObject, 10) + `[ \t\r\n]+` + strconv.FormatInt(rootGeneration, 10) + `[ \t\r\n]+obj(?:[ \t\r\n]|<)`)
	return objectPattern.Match(pdf[rootOffset:xrefOffset])
}

func safePDFName(title string) string {
	title = strings.ReplaceAll(title, "\\", "/")
	title = path.Base(title)
	var builder strings.Builder
	for _, r := range title {
		if r == 0 || r < 0x20 || r == 0x7f {
			continue
		}
		if r == ',' || r == '"' || r == ';' || r == '/' {
			r = '-'
		}
		builder.WriteRune(r)
	}
	title = strings.Trim(builder.String(), " .")
	if utf8.RuneCountInString(title) > 120 {
		runes := []rune(title)
		title = string(runes[:120])
	}
	if title == "" || title == "." || title == "-" {
		title = "Untitled"
	}
	return title + ".pdf"
}
