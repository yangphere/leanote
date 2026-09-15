package api

import (
	"net/url"
	"testing"

	"github.com/yangphere/leanote/app/info"
)

func TestAPINoteFilesPresenceLeavesAssetsUnchangedWhenFilesAreAbsent(t *testing.T) {
	values := url.Values{
		"Title":                    {"updated"},
		"FileDatas[local-file-id]": {"upload bytes are bound separately"},
	}

	if apiNoteFilesPresent(values, nil) {
		t.Fatal("unrelated fields were treated as a complete files collection")
	}
}

func TestAPINoteFilesPresenceAcceptsExplicitEmptyMarkers(t *testing.T) {
	for _, marker := range []string{"FilesPresent", "HasFiles"} {
		t.Run(marker, func(t *testing.T) {
			if !apiNoteFilesPresent(url.Values{marker: {"1"}}, nil) {
				t.Fatalf("%s did not mark an explicit empty files collection as present", marker)
			}
		})
	}
}

func TestAPINoteFilesPresenceRejectsDisabledEmptyMarkers(t *testing.T) {
	for _, marker := range []string{"FilesPresent", "HasFiles"} {
		t.Run(marker, func(t *testing.T) {
			if apiNoteFilesPresent(url.Values{marker: {"0"}}, nil) {
				t.Fatalf("%s=0 was treated as an explicit empty files collection", marker)
			}
		})
	}
}

func TestAPINoteFilesPresenceAcceptsAnyIndexedFilesField(t *testing.T) {
	values := url.Values{"Files[3][Title]": {"attachment.txt"}}

	if !apiNoteFilesPresent(values, nil) {
		t.Fatal("an indexed Files field did not mark the complete collection as present")
	}
}

func TestAPINoteFilesPresenceAcceptsNonEmptyDecodedCollection(t *testing.T) {
	files := []info.NoteFile{{LocalFileId: "local-file-id"}}

	if !apiNoteFilesPresent(nil, files) {
		t.Fatal("a non-empty decoded files collection was treated as absent")
	}
}
