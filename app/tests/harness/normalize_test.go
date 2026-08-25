package harness

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeBodyPreservesJSONOrderAndContentCanary(t *testing.T) {
	body := []byte(`{"NoteId":"507f1f77bcf86cd799439011","CreatedTime":"2015-01-20T11:13:41.34+08:00","Content":"keep 507f1f77bcf86cd799439011","Usn":42}`)

	got, err := NormalizeBody(body)
	if err != nil {
		t.Fatalf("NormalizeBody() error = %v", err)
	}

	want := `{"NoteId":"OID_TOKEN","CreatedTime":"TIME_TOKEN","Content":"keep 507f1f77bcf86cd799439011","Usn":42}`
	if string(got) != want {
		t.Fatalf("NormalizeBody() = %s, want %s", got, want)
	}
}

func TestNormalizeBodyRejectsExtendedObjectIDForContractField(t *testing.T) {
	body := []byte(`{"NoteId":{"$oid":"507f1f77bcf86cd799439011"}}`)

	_, err := NormalizeBody(body)
	if err == nil || !strings.Contains(err.Error(), "NoteId") {
		t.Fatalf("NormalizeBody() error = %v, want NoteId contract error", err)
	}
}

func TestNormalizeBodyReplacesDynamicSyncTimestampWithoutTouchingUsn(t *testing.T) {
	body := []byte(`{"LastSyncTime":1769400000,"Usn":42}`)

	got, err := NormalizeBody(body)
	if err != nil {
		t.Fatalf("NormalizeBody() error = %v", err)
	}
	want := `{"LastSyncTime":"UNIX_TIME_TOKEN","Usn":42}`
	if string(got) != want {
		t.Fatalf("NormalizeBody() = %s, want %s", got, want)
	}
}

func TestNormalizeBodyRejectsInvalidTimestampContractValue(t *testing.T) {
	_, err := NormalizeBody([]byte(`{"CreatedTime":"not-a-timestamp"}`))
	if err == nil {
		t.Fatal("NormalizeBody() error = nil, want invalid timestamp rejection")
	}
}

func TestNormalizeBodyReplacesOnlyGeneratedLogoPath(t *testing.T) {
	body := []byte(`{"Logo":"http://127.0.0.1:28017/public/upload/user/images/logo/9d963be1.png","Fallback":"/images/blog/default_avatar.png"}`)
	got, err := NormalizeBody(body)
	if err != nil {
		t.Fatalf("NormalizeBody() error = %v", err)
	}
	want := `{"Logo":"LOGO_TOKEN","Fallback":"/images/blog/default_avatar.png"}`
	if string(got) != want {
		t.Fatalf("NormalizeBody() = %s, want %s", got, want)
	}
}

func TestNormalizeBodyAllowsEmptyOptionalObjectIDFields(t *testing.T) {
	body := []byte(`{"NoteId":"507f1f77bcf86cd799439011","UserId":"507f1f77bcf86cd799439012","NotebookId":""}`)
	got, err := NormalizeBody(body)
	if err != nil {
		t.Fatalf("NormalizeBody() error = %v", err)
	}
	want := `{"NoteId":"OID_TOKEN","UserId":"OID_TOKEN","NotebookId":""}`
	if string(got) != want {
		t.Fatalf("NormalizeBody() = %s, want %s", got, want)
	}
}

func TestNormalizeHeadersRejectsUnknownHeaders(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("X-Framework-Version", "1")

	_, err := NormalizeHeaders(headers, false)
	if err == nil || !strings.Contains(err.Error(), "X-Framework-Version") {
		t.Fatalf("NormalizeHeaders() error = %v, want unknown header error", err)
	}
}

func TestNormalizeHeadersKeepsOnlyComparableHeaders(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("Location", "/login")
	headers.Set("Date", "Mon, 02 Jan 2006 15:04:05 GMT")
	headers.Set("Set-Cookie", "LEANOTE=a")
	headers.Set("Content-Length", "123")

	got, err := NormalizeHeaders(headers, false)
	if err != nil {
		t.Fatalf("NormalizeHeaders() error = %v", err)
	}

	if len(got) != 2 || got["Content-Type"] != "application/json; charset=utf-8" || got["Location"] != "/login" {
		t.Fatalf("NormalizeHeaders() = %#v, want Content-Type and Location", got)
	}
}

func TestNormalizeBinaryHeadersKeepsDispositionAndRange(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Content-Type", "image/png")
	headers.Set("Accept-Ranges", "bytes")
	headers.Set("Content-Disposition", `inline; filename="seed.png"`)
	headers.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
	headers.Set("Date", "Mon, 02 Jan 2006 15:04:05 GMT")
	headers.Set("Content-Length", "3")

	got, err := NormalizeHeaders(headers, true)
	if err != nil {
		t.Fatalf("NormalizeHeaders() error = %v", err)
	}

	if len(got) != 3 || got["Accept-Ranges"] != "bytes" || got["Content-Disposition"] != `inline; filename="seed.png"` {
		t.Fatalf("NormalizeHeaders() = %#v, want binary comparable headers", got)
	}
}
