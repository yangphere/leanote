package harness

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientExecutesFormRequestWithIdentityToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/api/note/addNote" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		if request.URL.Query().Get("token") != "admin-token" {
			t.Fatalf("token = %q", request.URL.Query().Get("token"))
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("Title") != "Regression note" {
			t.Fatalf("title = %q", request.Form.Get("Title"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.SetToken("admin", "admin-token")
	snapshot, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/note/addNote",
		Form:   map[string][]string{"Title": {"Regression note"}},
		Auth:   "admin",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if snapshot.Status != http.StatusOK || string(snapshot.Body) != `{"Ok":true}` {
		t.Fatalf("Do() snapshot = %#v", snapshot)
	}
}

func TestClientRejectsUnknownResponseHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Undocumented", "unexpected")
		_, _ = writer.Write([]byte(`{"Ok":true}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).Do(RequestSpec{Method: http.MethodGet, Path: "/api/auth/login"})
	if err == nil {
		t.Fatal("Do() error = nil, want unknown header rejection")
	}
}

func TestClientReadsBinaryResponseWithoutJSONNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Accept-Ranges", "bytes")
		writer.Header().Set("Content-Disposition", `inline; filename="seed.png"`)
		_, _ = writer.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer server.Close()

	snapshot, err := NewClient(server.URL).Do(RequestSpec{Method: http.MethodGet, Path: "/api/file/getImage", Binary: true})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if got := string(snapshot.Body); got != string([]byte{0x89, 0x50, 0x4e, 0x47}) {
		t.Fatalf("binary body = %q", got)
	}
}

func TestClientExecutesMultipartRequestWithIdentityToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("token") != "admin-token" {
			t.Fatalf("token = %q", request.URL.Query().Get("token"))
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("Content-Type = %q, want multipart/form-data", request.Header.Get("Content-Type"))
		}
		if err := request.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("caption") != "Regression logo" {
			t.Fatalf("caption = %q", request.FormValue("caption"))
		}
		file, header, err := request.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if header.Filename != "logo.png" || string(body) != "png-seed" {
			t.Fatalf("uploaded file = %q / %q", header.Filename, body)
		}
		if header.Header.Get("Content-Type") != "image/png" {
			t.Fatalf("file Content-Type = %q", header.Header.Get("Content-Type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.SetToken("admin", "admin-token")
	_, err := client.Do(RequestSpec{
		Method: http.MethodPost,
		Path:   "/api/user/updateLogo",
		Form:   map[string][]string{"caption": {"Regression logo"}},
		Files: map[string]FilePart{
			"file": {Filename: "logo.png", ContentType: "image/png", Body: []byte("png-seed")},
		},
		Auth: "admin",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}
