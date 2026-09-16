package content

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"golang.org/x/net/html"
)

const maxPDFResourceBytes = 128 * 1024 * 1024

const builtinPDFCSS = `
body { margin: 30px; color: #111; font-family: Georgia, "Times New Roman", serif; }
h1 { line-height: 1.15; }
img { max-width: 100%; height: auto; }
pre { white-space: pre-wrap; }
table { border-collapse: collapse; }
th, td { border: 1px solid #ddd; padding: 6px 13px; }
`

const builtinPDFCompletionScript = `window.status = "done";`

type PDFResource struct {
	MIME string
	Data []byte
}

// PDFDocumentRequest contains already-authorized, bounded resources.  It does
// not contain a URL fetcher or a secret; content rendering is intentionally
// incapable of making callback requests.
type PDFDocumentRequest struct {
	Title     string
	HTML      string
	Markdown  bool
	Resources map[string]PDFResource
}

func SerializeSelfContainedPDF(request PDFDocumentRequest) ([]byte, error) {
	if len(request.HTML) > 32*1024*1024 {
		return nil, contentError(ErrorTooLarge, "pdf_html_limit", nil)
	}
	contentHTML, err := renderPDFContent(request.HTML, request.Markdown)
	if err != nil {
		return nil, err
	}
	parsedFragment, err := html.Parse(strings.NewReader("<body>" + contentHTML + "</body>"))
	if err != nil {
		return nil, contentError(ErrorUnsupportedMedia, "pdf_html_parse", err)
	}
	body := &html.Node{Type: html.ElementNode, Data: "body"}
	var sourceBody *html.Node
	var findBody func(*html.Node)
	findBody = func(node *html.Node) {
		if sourceBody != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "body" {
			sourceBody = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			findBody(child)
		}
	}
	findBody(parsedFragment)
	if sourceBody == nil {
		return nil, contentError(ErrorUnsupportedMedia, "pdf_html_body_missing", nil)
	}
	for child := sourceBody.FirstChild; child != nil; {
		next := child.NextSibling
		sourceBody.RemoveChild(child)
		body.AppendChild(child)
		child = next
	}
	if err := sanitizePDFTree(body, request.Resources); err != nil {
		return nil, err
	}

	document := &html.Node{Type: html.DocumentNode}
	document.AppendChild(&html.Node{Type: html.DoctypeNode, Data: "html"})
	root := &html.Node{Type: html.ElementNode, Data: "html"}
	document.AppendChild(root)
	head := &html.Node{Type: html.ElementNode, Data: "head"}
	root.AppendChild(head)
	title := &html.Node{Type: html.ElementNode, Data: "title"}
	title.AppendChild(&html.Node{Type: html.TextNode, Data: strings.TrimSpace(request.Title)})
	head.AppendChild(title)
	style := &html.Node{Type: html.ElementNode, Data: "style"}
	style.AppendChild(&html.Node{Type: html.TextNode, Data: builtinPDFCSS})
	head.AppendChild(style)
	script := &html.Node{Type: html.ElementNode, Data: "script", Attr: []html.Attribute{{Key: "data-leanote-pinned", Val: "completion-v1"}}}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: builtinPDFCompletionScript})
	head.AppendChild(script)
	root.AppendChild(body)
	var output bytes.Buffer
	if err := html.Render(&output, document); err != nil {
		return nil, contentError(ErrorStorageUnavailable, "pdf_html_render", err)
	}
	result := output.Bytes()
	if err := ValidateSelfContainedPDFDocument(result); err != nil {
		return nil, err
	}
	return result, nil
}

func renderPDFContent(value string, markdown bool) (string, error) {
	if !markdown {
		return value, nil
	}
	var rendered bytes.Buffer
	if err := goldmark.Convert([]byte(value), &rendered); err != nil {
		return "", contentError(ErrorUnsupportedMedia, "pdf_markdown_render", err)
	}
	return rendered.String(), nil
}

func sanitizePDFTree(node *html.Node, resources map[string]PDFResource) error {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode {
			tag := strings.ToLower(child.Data)
			switch tag {
			case "script", "style", "iframe", "object", "embed", "frame", "frameset", "base", "link", "form", "input", "button", "video", "audio", "source", "applet", "svg":
				node.RemoveChild(child)
				child = next
				continue
			case "meta":
				if hasRefreshMeta(child) {
					node.RemoveChild(child)
					child = next
					continue
				}
			}
			attrs := make([]html.Attribute, 0, len(child.Attr))
			for _, attr := range child.Attr {
				key := strings.ToLower(attr.Key)
				if strings.HasPrefix(key, "on") || key == "srcset" || key == "style" || key == "action" || key == "formaction" {
					continue
				}
				if key == "src" {
					resource, ok := resources[attr.Val]
					if !ok {
						continue
					}
					dataURL, err := inlinePDFResource(resource)
					if err != nil {
						return err
					}
					attr.Val = dataURL
				}
				if key == "href" && !strings.HasPrefix(attr.Val, "#") {
					continue
				}
				if isPDFExternalAttribute(key) && key != "src" && key != "href" {
					continue
				}
				attrs = append(attrs, attr)
			}
			child.Attr = attrs
		}
		if err := sanitizePDFTree(child, resources); err != nil {
			return err
		}
		child = next
	}
	return nil
}

func inlinePDFResource(resource PDFResource) (string, error) {
	if len(resource.Data) == 0 || len(resource.Data) > maxPDFResourceBytes {
		return "", contentError(ErrorTooLarge, "pdf_resource_limit", nil)
	}
	mime := strings.ToLower(strings.TrimSpace(resource.MIME))
	if !strings.HasPrefix(mime, "image/") || strings.ContainsAny(mime, "\r\n; ") {
		return "", contentError(ErrorUnsupportedMedia, "pdf_resource_mime", nil)
	}
	extension := "." + strings.TrimPrefix(strings.TrimPrefix(mime, "image/"), "jpeg")
	if mime == "image/jpeg" {
		extension = ".jpg"
	}
	if _, err := ValidateImage(resource.Data, extension, HardImageBudget()); err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(resource.Data), nil
}

func ValidateSelfContainedPDFDocument(document []byte) error {
	if len(document) == 0 || len(document) > 64*1024*1024 {
		return contentError(ErrorTooLarge, "pdf_document_limit", nil)
	}
	parsed, err := html.Parse(bytes.NewReader(document))
	if err != nil {
		return contentError(ErrorUnsafePath, "pdf_document_parse", err)
	}
	var walk func(*html.Node) error
	walk = func(node *html.Node) error {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "script" {
				pinned := false
				for _, attr := range node.Attr {
					if attr.Key == "data-leanote-pinned" && attr.Val == "completion-v1" {
						pinned = true
					}
				}
				if !pinned || node.FirstChild == nil || node.FirstChild != node.LastChild || node.FirstChild.Type != html.TextNode || node.FirstChild.Data != builtinPDFCompletionScript {
					return unsafePathError("pdf_active_script", nil)
				}
			} else if tag == "iframe" || tag == "object" || tag == "embed" || tag == "link" || tag == "base" || tag == "form" {
				return unsafePathError("pdf_active_element", nil)
			}
			if hasRefreshMeta(node) {
				return unsafePathError("pdf_meta_refresh", nil)
			}
			for _, attr := range node.Attr {
				key := strings.ToLower(attr.Key)
				if strings.HasPrefix(key, "on") || key == "srcset" || key == "style" || key == "action" || key == "formaction" {
					return unsafePathError("pdf_active_attribute", nil)
				}
				if isPDFExternalAttribute(key) {
					if strings.HasPrefix(attr.Val, "#") && key == "href" {
						continue
					}
					if key != "src" || !strings.HasPrefix(strings.ToLower(attr.Val), "data:image/") {
						return unsafePathError("pdf_external_resource", nil)
					}
					if _, err := url.Parse(attr.Val); err != nil {
						return unsafePathError("pdf_resource_url", err)
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
		return err
	}
	return nil
}

func isPDFExternalAttribute(key string) bool {
	switch strings.ToLower(key) {
	case "src", "href", "xlink:href", "background", "poster", "ping", "manifest", "codebase", "archive", "longdesc", "cite":
		return true
	default:
		return false
	}
}

func hasRefreshMeta(node *html.Node) bool {
	if strings.ToLower(node.Data) != "meta" {
		return false
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "http-equiv") && strings.EqualFold(strings.TrimSpace(attr.Val), "refresh") {
			return true
		}
	}
	return false
}
