package contentpdf

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	application "github.com/yangphere/leanote/app/application/content"
)

// 真实 Gotenberg 集成门禁（R1）：只有设置 LEANOTE_GOTENBERG_URL 才运行。
// 它让 Chromium 的真实输出走完整的 application.PDFRenderer 调用链，
// 即必须通过 validatePDFArtifact；仅 qpdf 或 %PDF- 检查不算通过。
func TestGotenbergBackendRealOutputPassesPDFRenderer(t *testing.T) {
	raw := os.Getenv("LEANOTE_GOTENBERG_URL")
	if raw == "" {
		t.Skip("LEANOTE_GOTENBERG_URL is not set; real Gotenberg integration unrun")
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewGotenbergBackend(endpoint, "leanote-pdf-v1")
	if err != nil {
		t.Fatal(err)
	}
	renderer := application.PDFRenderer{Backend: backend, Timeout: 30 * time.Second, MaxOutputBytes: 64 << 20}

	for name, request := range map[string]application.PDFRenderRequest{
		"html": {Title: "中文标题", HTML: "<h1>中文标题</h1><p>这是一段中文正文，用于验证字体可见。</p><table><tr><td>表格</td><td>单元格</td></tr></table>"},
		"markdown": {
			Title: "Markdown 笔记", Markdown: true,
			HTML: "# 标题\n\n中文正文 **加粗**\n\n- 列表一\n- 列表二\n\n```go\nfmt.Println(\"你好\")\n```\n",
		},
		// 外部 URL 必须被 deny-list 或文档净化拦截，但渲染本身仍应成功。
		"external-url": {Title: "外链", HTML: `<p>正文</p><img src="https://example.com/x.png">`},
	} {
		t.Run(name, func(t *testing.T) {
			artifact, renderErr := renderer.Render(context.Background(), request)
			if renderErr != nil {
				t.Fatalf("render through PDFRenderer: %v", renderErr)
			}
			if artifact.ContentType != "application/pdf" || !strings.HasPrefix(string(artifact.Data), "%PDF-") {
				t.Fatalf("artifact = %q, prefix %q", artifact.ContentType, string(artifact.Data[:min(8, len(artifact.Data))]))
			}
			t.Logf("%s: %d bytes, header %q", name, len(artifact.Data), string(artifact.Data[:8]))
			if dir := os.Getenv("LEANOTE_GOTENBERG_OUTPUT_DIR"); dir != "" {
				if writeErr := os.WriteFile(dir+string(os.PathSeparator)+name+".pdf", artifact.Data, 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
		})
	}
}
