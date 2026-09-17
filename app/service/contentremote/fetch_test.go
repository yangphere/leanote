package contentremote

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	application "github.com/yangphere/leanote/app/application/content"
)

type staticResolver []netip.Addr

func (resolver staticResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return resolver, nil
}

func TestFetchRejectsPrivateResolutionBeforeDial(t *testing.T) {
	dialed := false
	fetcher := New(staticResolver{netip.MustParseAddr("127.0.0.1")}, func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, nil
	})
	if _, err := fetcher.Fetch(context.Background(), "http://private.example/image.png", 1024); err == nil {
		t.Fatal("Fetch() accepted a private resolved address")
	}
	if dialed {
		t.Fatal("Fetch() dialed after private DNS resolution")
	}
}

func TestFetchClassifiesCallerCancellationAsTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("dial called for canceled request")
		return nil, nil
	})
	_, err := fetcher.Fetch(ctx, "http://images.example/photo.png", 1024)
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorTimeout {
		t.Fatalf("Fetch() error = %#v", err)
	}
}

func TestFetchRejectsPrivateConnectedAddressAfterPublicResolution(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			_, _ = io.Copy(io.Discard, connection)
		}
	}()
	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, listener.Addr().String())
	})
	if _, err := fetcher.Fetch(context.Background(), "http://images.example/photo.png", 1024); err == nil {
		t.Fatal("Fetch() accepted a private connected address")
	}
}

func TestFetchReturnsValidatedImageThroughPublicAddressPolicy(t *testing.T) {
	imageBytes := encodedPNG(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(imageBytes)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, listener.Addr().String())
		if err != nil {
			return nil, err
		}
		return &publicRemoteConn{Conn: conn}, nil
	})
	result, err := fetcher.Fetch(context.Background(), "http://images.example/photo.png", int64(len(imageBytes)+1))
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if string(result.Data) != string(imageBytes) || result.Metadata.MIME != "image/png" || result.DisplayName != "photo.png" {
		t.Fatalf("Fetch() result = %#v", result)
	}
}

func TestFetchRejectsBodyOverUploadLimit(t *testing.T) {
	imageBytes := encodedPNG(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write(imageBytes) })}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, listener.Addr().String())
		return &publicRemoteConn{Conn: conn}, err
	})
	if _, err := fetcher.Fetch(context.Background(), "http://images.example/photo.png", int64(len(imageBytes)-1)); err == nil {
		t.Fatal("Fetch() accepted an oversized response")
	}
}

func TestFetchClassifiesCompressedBodyLimitBeforeDecodeError(t *testing.T) {
	imageBytes := encodedPNG(t)
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if compressed.Len() <= len(imageBytes) {
		t.Skip("fixture did not produce gzip overhead")
	}
	response := &http.Response{Body: io.NopCloser(bytes.NewReader(compressed.Bytes())), Header: make(http.Header)}
	response.Header.Set("Content-Encoding", "gzip")
	_, err := readResponseBody(response, int64(len(imageBytes)))
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorTooLarge || contentErr.Code != "remote_compressed_size_limit" {
		t.Fatalf("readResponseBody() error = %#v", err)
	}
}

func TestFetchUsesFinalRedirectURLForImageMetadata(t *testing.T) {
	imageBytes := encodedPNG(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/photo.png", http.StatusFound)
			return
		}
		_, _ = writer.Write(imageBytes)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, listener.Addr().String())
		return &publicRemoteConn{Conn: conn}, err
	})
	result, err := fetcher.Fetch(context.Background(), "http://images.example/start", int64(len(imageBytes)+1))
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.DisplayName != "photo.png" || result.Metadata.Extension != ".png" {
		t.Fatalf("Fetch() metadata = %#v", result)
	}
}

func TestFetchRejectsRedirectBeyondProfileLimit(t *testing.T) {
	imageBytes := encodedPNG(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		remaining, parseErr := strconv.Atoi(strings.TrimPrefix(request.URL.Path, "/"))
		if parseErr != nil || remaining < 0 {
			http.Error(writer, "invalid hop", http.StatusBadRequest)
			return
		}
		if remaining > 0 {
			http.Redirect(writer, request, "/"+strconv.Itoa(remaining-1), http.StatusFound)
			return
		}
		_, _ = writer.Write(imageBytes)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, dialErr := dialer.DialContext(ctx, network, listener.Addr().String())
		return &publicRemoteConn{Conn: conn}, dialErr
	})
	if _, err := fetcher.Fetch(context.Background(), "http://images.example/5", int64(len(imageBytes)+1)); err != nil {
		t.Fatalf("Fetch() rejected the profile limit: %v", err)
	}
	_, err = fetcher.Fetch(context.Background(), "http://images.example/6", int64(len(imageBytes)+1))
	var contentErr *application.Error
	if !errors.As(err, &contentErr) || contentErr.Category != application.ErrorValidation || contentErr.Code != "remote_redirect_limit" {
		t.Fatalf("Fetch() error = %#v, want remote_redirect_limit validation error", err)
	}
}

func TestFetchRejectsOverlongRemoteDisplayName(t *testing.T) {
	imageBytes := encodedPNG(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write(imageBytes) })}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	dialer := &net.Dialer{}
	fetcher := New(staticResolver{netip.MustParseAddr("93.184.216.34")}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		conn, dialErr := dialer.DialContext(ctx, network, listener.Addr().String())
		return &publicRemoteConn{Conn: conn}, dialErr
	})
	name := strings.Repeat("a", application.MaxVisibleTextBytes) + ".png"
	if _, err := fetcher.Fetch(context.Background(), "http://images.example/"+name, int64(len(imageBytes)+1)); err == nil {
		t.Fatal("Fetch() accepted an overlong display name")
	}
}

type publicRemoteConn struct{ net.Conn }

func (conn *publicRemoteConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("93.184.216.34"), Port: 80}
}

func encodedPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.White)
	if err := png.Encode(&output, value); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return output.Bytes()
}
