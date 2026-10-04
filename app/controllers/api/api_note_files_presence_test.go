package api

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
)

func TestAPINoteFilesPresenceLeavesAssetsUnchangedWhenFilesAreAbsent(t *testing.T) {
	values := url.Values{
		"Title":                    {"updated"},
		"FileDatas[local-file-id]": {"upload bytes are bound separately"},
	}

	request := httptest.NewRequest("POST", "/api/note/update", nil)
	request.Form = values
	context := &httpserver.Context{Request: request}
	if apiNoteAssetsSupplied(context) {
		t.Fatal("unrelated fields were treated as a complete files collection")
	}
}

func TestAPINoteFilesPresenceAcceptsExplicitEmptyMarkers(t *testing.T) {
	for _, values := range []url.Values{
		{"Files": {""}},
		{"Files[0][Title]": {""}},
	} {
		request := httptest.NewRequest("POST", "/api/note/update", nil)
		request.Form = values
		if !apiNoteAssetsSupplied(&httpserver.Context{Request: request}) {
			t.Fatalf("values %v did not mark an explicit files collection as present", values)
		}
	}
}

func TestAPINoteFilesPresenceAcceptsAnyIndexedFilesField(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/note/update", nil)
	request.Form = url.Values{"Files[3][Title]": {"attachment.txt"}}
	if !apiNoteAssetsSupplied(&httpserver.Context{Request: request}) {
		t.Fatal("an indexed Files field did not mark the complete collection as present")
	}
}

func TestAPINoteFilesPresenceRequiresRequest(t *testing.T) {
	if apiNoteAssetsSupplied(nil) {
		t.Fatal("nil request context reported supplied assets")
	}
}
