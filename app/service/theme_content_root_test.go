package service

import (
	"path/filepath"
	"testing"

	. "github.com/yangphere/leanote/app/lea"
)

func TestThemeServiceUsesConfiguredPublicUploadRoot(t *testing.T) {
	previousRoots := configuredContentRoots
	t.Cleanup(func() {
		configuredContentRoots = previousRoots
	})

	configuredContentRoots = ContentRoots{PublicUpload: ContentRootPair{Data: filepath.Join(t.TempDir(), "public-upload")}}
	service := &ThemeService{}
	userID := "507f1f77bcf86cd799439011"
	want := filepath.Join(configuredContentRoots.PublicUpload.Data, Digest3(userID), userID, "themes")
	if got := service.getUserThemeBasePath(userID); got != want {
		t.Fatalf("theme base path = %q, want configured content root %q", got, want)
	}
}
