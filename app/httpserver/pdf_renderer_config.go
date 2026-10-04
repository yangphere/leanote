package httpserver

import (
	"net/url"
	"strings"

	"github.com/yangphere/leanote/app/service"
	"github.com/yangphere/leanote/app/service/contentpdf"
)

const (
	pdfRendererKey     = "pdf.renderer"
	pdfGotenbergURLKey = "pdf.gotenberg.url"
)

// parsePDFRendererConfig 是 prod/dev/test 共用的渲染器选择规则（fail closed）：
// 缺省保持进程模式；gotenberg 必须配置合法地址；未知值启动失败。
// 渲染器只能由部署配置选择，管理后台与请求参数不参与。
func parsePDFRendererConfig(cfg *Config) (service.PDFRendererConfig, error) {
	raw, _ := cfg.String(pdfRendererKey)
	switch strings.TrimSpace(raw) {
	case "", string(service.PDFRendererProcess):
		return service.PDFRendererConfig{Kind: service.PDFRendererProcess}, nil
	case string(service.PDFRendererGotenberg):
		rawURL, ok := cfg.String(pdfGotenbergURLKey)
		if !ok || strings.TrimSpace(rawURL) == "" {
			return service.PDFRendererConfig{}, configError("CONFIG_VALUE_MISSING", pdfGotenbergURLKey)
		}
		endpoint, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || contentpdf.ValidateGotenbergEndpoint(endpoint) != nil {
			return service.PDFRendererConfig{}, configError("CONFIG_PDF_RENDERER_URL_INVALID", pdfGotenbergURLKey)
		}
		return service.PDFRendererConfig{Kind: service.PDFRendererGotenberg, GotenbergURL: endpoint}, nil
	default:
		return service.PDFRendererConfig{}, configError("CONFIG_PDF_RENDERER_INVALID", pdfRendererKey)
	}
}
