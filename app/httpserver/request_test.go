package httpserver

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParamsParsesMultipartFieldsBeforeFileAccess(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("Title", "hello"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("FileDatas[local-1]", "image.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	params := newParams(request, nil)

	if got := params.String("Title"); got != "hello" {
		t.Fatalf("multipart field = %q, want hello", got)
	}
	file, header, err := params.FormFile("FileDatas[local-1]")
	if err != nil {
		t.Fatalf("FormFile: %v", err)
	}
	defer file.Close()
	if header.Filename != "image.png" {
		t.Fatalf("multipart filename = %q, want image.png", header.Filename)
	}
}

func TestParamsFormFileParsesBoundedFormWhenCalledFirst(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("Title", "first"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "payload.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	params := newParams(request, nil)

	file, header, err := params.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile: %v", err)
	}
	defer file.Close()
	if header.Filename != "payload.txt" {
		t.Fatalf("filename = %q, want payload.txt", header.Filename)
	}
	values, found := params.Form["Title"]
	if !params.formOK || !found || len(values) != 1 || values[0] != "first" {
		t.Fatalf("FormFile did not run the bounded form parser: formOK=%v form=%v", params.formOK, params.Form)
	}
	if got := params.String("Title"); got != "first" {
		t.Fatalf("form field after FormFile = %q, want first", got)
	}
}

func TestParamsPreservesPresenceAndRepeatedValues(t *testing.T) {
	request := httptest.NewRequest("POST", "/?Tags=query&Tags=query2&empty=", strings.NewReader("Tags=form&Tags=form2&Tags[0]=nested&Files[0][LocalFileId]=local"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	params := newParams(request, map[string]string{"path": "value"})

	emptyValue, emptyFound := params.Get("empty")
	if !params.Has("empty") || !emptyFound || emptyValue != "" {
		t.Fatalf("empty parameter presence/value = %v/%q/%v, want true/empty/true Get", params.Has("empty"), emptyValue, emptyFound)
	}
	if got := params.Strings("Tags"); !reflect.DeepEqual(got, []string{"query", "query2"}) {
		t.Fatalf("query strings = %#v, want query values", got)
	}
	if got, ok := params.Get("Tags[0]"); !ok || got != "nested" {
		t.Fatalf("nested value = %q/%v, want nested/true", got, ok)
	}
	if got, ok := params.NestedString("Files", 0, "LocalFileId"); !ok || got != "local" {
		t.Fatalf("nested helper = %q/%v, want local/true", got, ok)
	}
}

func TestParamsBoolUsesLegacyAtobVocabulary(t *testing.T) {
	request := httptest.NewRequest("GET", "/?yes=yes&no=off&bad=maybe", nil)
	params := newParams(request, nil)
	if !params.Bool("yes", false) || params.Bool("no", true) || !params.Bool("missing", true) || !params.Bool("bad", true) {
		t.Fatal("Bool did not preserve Atob vocabulary and defaults")
	}
}

func TestParamsStrictReadersRejectMissingMalformedAndOverflow(t *testing.T) {
	request := httptest.NewRequest("GET", "/?count=999999999999999999999999&bad=not-an-id&id=507f1f77bcf86cd799439011", nil)
	params := newParams(request, nil)
	if _, err := params.StrictInt("count"); err == nil {
		t.Fatal("StrictInt accepted overflow")
	}
	if _, err := params.StrictObjectID("bad"); err == nil {
		t.Fatal("StrictObjectID accepted malformed id")
	}
	if id, err := params.StrictObjectID("id"); err != nil || id.Hex() != "507f1f77bcf86cd799439011" {
		t.Fatalf("StrictObjectID valid id = %s/%v", id.Hex(), err)
	}
	if _, err := params.StrictObjectID("missing"); err == nil {
		t.Fatal("StrictObjectID accepted missing id")
	}
}
