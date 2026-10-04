package contentpdf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	application "github.com/yangphere/leanote/app/application/content"
)

const (
	// gotenbergConvertPath 是 Gotenberg 8 的 Chromium HTML 转换路由。
	gotenbergConvertPath = "forms/chromium/convert/html"
	// gotenbergWaitExpression 与自包含文档内置脚本设置的 window.status 对应，
	// 等价于 wkhtmltopdf 的 --window-status done。
	gotenbergWaitExpression = `window.status === "done"`
)

// GotenbergBackend 通过一次 HTTP 调用把自包含 HTML 交给 Gotenberg 渲染。
// 它只访问部署配置给出的地址，不读取管理后台配置，也不回退到其他后端。
type GotenbergBackend struct {
	endpoint     string
	convertURL   string
	policyID     string
	executableID string
	client       *http.Client
}

// NewGotenbergBackend 校验部署配置的 Gotenberg 地址并构造后端。
// 地址必须是 http/https、带主机、无 userinfo/query/fragment，path 只能为空或 "/"。
func NewGotenbergBackend(endpoint *url.URL, policyID string) (*GotenbergBackend, error) {
	if strings.TrimSpace(policyID) == "" {
		return nil, application.NewError(application.ErrorValidation, "renderer_policy_missing", nil)
	}
	if err := ValidateGotenbergEndpoint(endpoint); err != nil {
		return nil, err
	}
	base := &url.URL{Scheme: endpoint.Scheme, Host: endpoint.Host}
	convert := base.JoinPath(gotenbergConvertPath)
	digest := sha256.Sum256([]byte(strings.Join([]string{policyID, "gotenberg", base.String()}, "\x00")))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	// 不读取代理环境变量，避免渲染请求被环境中的代理转发到其他地址。
	transport.Proxy = nil
	client := &http.Client{
		Transport: transport,
		// 超时由调用方 ctx 控制；禁止重定向，保证只访问配置的地址。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &GotenbergBackend{
		endpoint: base.String(), convertURL: convert.String(), policyID: policyID,
		executableID: hex.EncodeToString(digest[:])[:32], client: client,
	}, nil
}

// ValidateGotenbergEndpoint 是 Gotenberg 地址的唯一校验规则，启动配置与后端共用。
func ValidateGotenbergEndpoint(endpoint *url.URL) error {
	invalid := func() error {
		return application.NewError(application.ErrorValidation, "renderer_endpoint_invalid", nil)
	}
	if endpoint == nil {
		return invalid()
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return invalid()
	}
	if endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery ||
		endpoint.Fragment != "" || endpoint.Opaque != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return invalid()
	}
	return nil
}

func (backend *GotenbergBackend) Descriptor(ctx context.Context) (application.RendererDescriptor, error) {
	if backend == nil {
		return application.RendererDescriptor{}, application.NewError(application.ErrorDependency, "renderer_config_unavailable", nil)
	}
	if err := processContextError(ctx, "renderer_descriptor_canceled"); err != nil {
		return application.RendererDescriptor{}, err
	}
	// 描述符只由策略与地址决定，不在每次导出前访问网络；可用性由 Render 的错误反映。
	return application.RendererDescriptor{PolicyID: backend.policyID, ExecutableID: backend.executableID}, nil
}

func (backend *GotenbergBackend) Render(ctx context.Context, descriptor application.RendererDescriptor, document []byte, maxOutputBytes int64) ([]byte, error) {
	if backend == nil {
		return nil, application.NewError(application.ErrorDependency, "renderer_config_unavailable", nil)
	}
	if err := processContextError(ctx, "renderer_process_timeout"); err != nil {
		return nil, err
	}
	if descriptor.PolicyID != backend.policyID || descriptor.ExecutableID != backend.executableID {
		return nil, application.NewError(application.ErrorConflict, "renderer_descriptor_changed", nil)
	}
	if maxOutputBytes <= 0 {
		return nil, application.NewError(application.ErrorValidation, "renderer_output_limit", nil)
	}
	if err := application.ValidateSelfContainedPDFDocumentContext(ctx, document); err != nil {
		return nil, err
	}
	body, contentType, err := buildGotenbergForm(document)
	if err != nil {
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_request_build", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, backend.convertURL, body)
	if err != nil {
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_request_build", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/pdf")

	response, err := backend.client.Do(request)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, application.NewError(application.ErrorTimeout, "renderer_process_timeout", errors.Join(err, ctx.Err()))
		}
		return nil, application.NewError(application.ErrorDependency, "renderer_unavailable", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		// 诊断只进入错误链，不进入用户响应。
		diagnostic, _ := io.ReadAll(io.LimitReader(response.Body, maxRendererDiagnosticBytes))
		cause := fmt.Errorf("gotenberg status %d trace=%q: %s", response.StatusCode,
			response.Header.Get("Gotenberg-Trace"), strings.TrimSpace(string(diagnostic)))
		if ctx.Err() != nil {
			return nil, application.NewError(application.ErrorTimeout, "renderer_process_timeout", errors.Join(cause, ctx.Err()))
		}
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_process_failed", cause)
	}

	pdf, err := io.ReadAll(io.LimitReader(response.Body, maxOutputBytes+1))
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, application.NewError(application.ErrorTimeout, "renderer_process_timeout", errors.Join(err, ctx.Err()))
		}
		return nil, application.NewError(application.ErrorRendererFailed, "renderer_process_failed", err)
	}
	if int64(len(pdf)) > maxOutputBytes {
		return nil, application.NewError(application.ErrorTooLarge, "renderer_output_limit", nil)
	}
	return pdf, nil
}

// buildGotenbergForm 构造 multipart 请求体：文档固定命名为 index.html，
// 并显式给出 A4 与四边 1cm 边距，保持与 wkhtmltopdf 默认页面一致。
func buildGotenbergForm(document []byte) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="files"; filename="index.html"`)
	header.Set("Content-Type", "text/html; charset=utf-8")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(document); err != nil {
		return nil, "", err
	}
	for _, field := range [][2]string{
		{"waitForExpression", gotenbergWaitExpression},
		{"printBackground", "true"},
		{"paperWidth", "21cm"},
		{"paperHeight", "29.7cm"},
		{"marginTop", "1cm"},
		{"marginBottom", "1cm"},
		{"marginLeft", "1cm"},
		{"marginRight", "1cm"},
	} {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}
