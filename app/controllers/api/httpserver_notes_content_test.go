package api

import (
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/info"
)

func TestRewriteAPINoteContentLinksUsesServerAssetIDs(t *testing.T) {
	content := `<p><img src="https://client.example/api/file/getImage?fileId=local-image"><a href="http://client.example/api/file/getAttach?fileId=local-attach">download</a></p>`
	files := []info.NoteFile{
		{LocalFileId: "local-image", FileId: "507f1f77bcf86cd799439011"},
		{LocalFileId: "local-attach", FileId: "507f1f77bcf86cd799439012", IsAttach: true},
	}

	got := rewriteAPINoteContentLinks(content, files)
	for _, want := range []string{
		`/api/file/getImage?fileId=507f1f77bcf86cd799439011`,
		`/api/file/getAttach?fileId=507f1f77bcf86cd799439012`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten content = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "local-image") || strings.Contains(got, "local-attach") {
		t.Fatalf("rewritten content kept local asset IDs: %q", got)
	}
}

func TestRewriteAPINoteContentLinksDoesNotRewriteUnrelatedHosts(t *testing.T) {
	content := `https://client.example/other/getImage?fileId=local-image https://client.example/api/file/getImage?fileId=local-image-extra`
	got := rewriteAPINoteContentLinks(content, []info.NoteFile{{LocalFileId: "local-image", FileId: "507f1f77bcf86cd799439011"}})
	if got != content {
		t.Fatalf("unrelated content changed: got %q want %q", got, content)
	}
}
