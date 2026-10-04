package contentremote

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"path"
	"strings"

	application "github.com/yangphere/leanote/app/application/content"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type DialContext func(context.Context, string, string) (net.Conn, error)

type Fetcher struct {
	resolver Resolver
	dial     DialContext
}

type Result struct {
	Data        []byte
	Metadata    application.ImageMetadata
	DisplayName string
}

func New(resolver Resolver, dial DialContext) *Fetcher {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dial == nil {
		networkDialer := &net.Dialer{}
		dial = networkDialer.DialContext
	}
	return &Fetcher{resolver: resolver, dial: dial}
}

func (fetcher *Fetcher) Fetch(ctx context.Context, value string, uploadImageBytes int64) (result Result, resultErr error) {
	if fetcher == nil || fetcher.resolver == nil || fetcher.dial == nil {
		return Result{}, application.NewError(application.ErrorDependency, "remote_fetcher_unavailable", nil)
	}
	profile, err := application.ResolveRemoteFetchProfile(uploadImageBytes)
	if err != nil {
		return Result{}, err
	}
	parsed, err := application.ValidateRemoteURL(value)
	if err != nil {
		return Result{}, err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DisableCompression:    true,
		TLSHandshakeTimeout:   profile.TLSHandshakeTimeout,
		ResponseHeaderTimeout: profile.ResponseHeaderTimeout,
	}
	defer transport.CloseIdleConnections()
	transport.DialContext = func(parent context.Context, network, address string) (net.Conn, error) {
		dialCtx, cancel := context.WithTimeout(parent, profile.DNSDialTimeout)
		defer cancel()
		return fetcher.dialPublic(dialCtx, network, address)
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > profile.MaxRedirects {
				return application.NewError(application.ErrorValidation, "remote_redirect_limit", nil)
			}
			_, err := application.ValidateRemoteURL(request.URL.String())
			return err
		},
	}
	requestCtx, cancel := context.WithTimeout(ctx, profile.OverallTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Result{}, application.NewError(application.ErrorValidation, "invalid_remote_request", err)
	}
	request.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/bmp")
	response, err := client.Do(request)
	if err != nil {
		if requestCtx.Err() != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Result{}, application.NewError(application.ErrorTimeout, "remote_fetch_timeout", errors.Join(err, requestCtx.Err()))
		}
		var contentErr *application.Error
		if errors.As(err, &contentErr) {
			return Result{}, err
		}
		return Result{}, application.NewError(application.ErrorDependency, "remote_fetch_failed", nil)
	}
	defer func() {
		if closeErr := response.Body.Close(); resultErr == nil && closeErr != nil {
			result = Result{}
			resultErr = application.NewError(application.ErrorDependency, "remote_body_close", closeErr)
		}
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Result{}, application.NewError(application.ErrorDependency, "remote_status", nil)
	}
	data, err := readResponseBody(response, profile.MaxCompressedBytes)
	if err != nil {
		return Result{}, err
	}
	finalURL := response.Request.URL
	extension := strings.ToLower(path.Ext(finalURL.Path))
	metadata, err := application.ValidateImage(data, extension, application.HardImageBudget())
	if err != nil {
		return Result{}, err
	}
	displayName, err := application.CleanVisibleText(path.Base(finalURL.Path), true)
	if err != nil {
		return Result{}, err
	}
	if displayName == "" || displayName == "." || displayName == "/" {
		displayName = "remote" + metadata.Extension
	}
	return Result{Data: data, Metadata: metadata, DisplayName: displayName}, nil
}

func (fetcher *Fetcher) dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, application.NewError(application.ErrorValidation, "remote_address_invalid", err)
	}
	addresses, err := fetcher.resolve(ctx, network, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		connection, err := fetcher.dial(ctx, network, net.JoinHostPort(resolved.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		connected, err := connectedAddress(connection.RemoteAddr())
		if err != nil || application.ValidatePublicRemoteAddress(connected) != nil {
			_ = connection.Close()
			return nil, application.NewError(application.ErrorValidation, "remote_connected_address_not_public", err)
		}
		return connection, nil
	}
	return nil, application.NewError(application.ErrorDependency, "remote_dial_failed", lastErr)
}

func (fetcher *Fetcher) resolve(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if err := application.ValidatePublicRemoteAddress(literal); err != nil {
			return nil, err
		}
		return []netip.Addr{literal.Unmap()}, nil
	}
	addresses, err := fetcher.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, application.NewError(application.ErrorDependency, "remote_dns_failed", err)
	}
	if len(addresses) == 0 {
		return nil, application.NewError(application.ErrorDependency, "remote_dns_empty", nil)
	}
	for _, address := range addresses {
		if err := application.ValidatePublicRemoteAddress(address); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func connectedAddress(value net.Addr) (netip.Addr, error) {
	switch address := value.(type) {
	case *net.TCPAddr:
		result, ok := netip.AddrFromSlice(address.IP)
		if !ok {
			return netip.Addr{}, fmt.Errorf("invalid TCP address")
		}
		return result.Unmap(), nil
	default:
		host, _, err := net.SplitHostPort(value.String())
		if err != nil {
			return netip.Addr{}, err
		}
		return netip.ParseAddr(strings.Trim(host, "[]"))
	}
}

func readResponseBody(response *http.Response, limit int64) ([]byte, error) {
	compressed := &io.LimitedReader{R: response.Body, N: limit + 1}
	var reader io.Reader = compressed
	var gzipReader *gzip.Reader
	switch strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip":
		var err error
		gzipReader, err = gzip.NewReader(compressed)
		if err != nil {
			return nil, application.NewError(application.ErrorUnsupportedMedia, "remote_gzip_header", err)
		}
		reader = gzipReader
	default:
		return nil, application.NewError(application.ErrorUnsupportedMedia, "remote_content_encoding", nil)
	}
	data, err := application.ReadBounded(reader, limit)
	if gzipReader != nil {
		if closeErr := gzipReader.Close(); err == nil && closeErr != nil {
			err = application.NewError(application.ErrorUnsupportedMedia, "remote_gzip_close", closeErr)
		}
	}
	if compressed.N == 0 {
		return nil, application.NewError(application.ErrorTooLarge, "remote_compressed_size_limit", nil)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
