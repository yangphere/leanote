package service

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
)

func TestThemeActivationStateCycleCreatesNewReceipt(t *testing.T) {
	useNotebookReceiptTestDatabase(t)
	for _, collection := range []*db.Collection{db.Themes, db.UserBlogs} {
		if _, err := collection.RemoveAll(map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, collection := range []*db.Collection{db.Themes, db.UserBlogs} {
			if _, err := collection.RemoveAll(map[string]any{}); err != nil {
				t.Errorf("clean theme collection: %v", err)
			}
		}
	})
	basePath, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	previousBasePath := revel.BasePath
	revel.BasePath = basePath
	t.Cleanup(func() { revel.BasePath = previousBasePath })
	owner, _ := domain.ParseObjectID("507f1f77bcf86cd799439011")
	first, _ := domain.ParseObjectID("507f1f77bcf86cd799439012")
	second, _ := domain.ParseObjectID("507f1f77bcf86cd799439013")
	if err := db.Themes.Insert(
		info.Theme{ThemeId: first, UserId: owner, Path: "public/blog/themes/default", IsActive: true},
		info.Theme{ThemeId: second, UserId: owner, Path: "public/blog/themes/elegant"},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.UserBlogs.Insert(info.UserBlog{UserId: owner, ThemeId: first}); err != nil {
		t.Fatal(err)
	}
	service := &ThemeService{}
	for _, target := range []domain.ObjectID{second, first, second} {
		if !service.ActiveTheme(owner.Hex(), target.Hex()) {
			t.Fatalf("theme activation to %s failed", target.Hex())
		}
	}
	if !service.ActiveTheme(owner.Hex(), second.Hex()) {
		t.Fatal("same-intent theme retry failed")
	}
	count, err := db.WorkspaceOperations.Find(map[string]any{"OwnerId": owner, "Kind": "theme_activate"}).Count()
	if err != nil || count != 3 {
		t.Fatalf("theme activation receipts=%d err=%v, want three", count, err)
	}
	var blog info.UserBlog
	if err := db.UserBlogs.FindId(owner).One(&blog); err != nil || blog.ThemeId != second {
		t.Fatalf("active blog theme=%s err=%v", blog.ThemeId.Hex(), err)
	}
}

func TestParseConfigRejectsWrongMetadataTypes(t *testing.T) {
	service := &ThemeService{}

	for _, field := range []string{"Name", "Version", "Author", "AuthorUrl"} {
		t.Run(field, func(t *testing.T) {
			_, err := service.parseConfig(`{"` + field + `": 42}`)
			if err == nil {
				t.Fatal("parseConfig accepted a non-string metadata field")
			}
			if !errors.Is(err, ErrThemeValidation) {
				t.Fatalf("parseConfig error = %v, want ErrThemeValidation", err)
			}
		})
	}
}

func TestValidateThemePathRejectsTraversalAndForeignRoots(t *testing.T) {
	userID := "507f1f77bcf86cd799439011"
	themeID := "507f1f77bcf86cd799439012"

	valid := "public/upload/" + Digest3(userID) + "/" + userID + "/themes/" + themeID
	if !validateThemePath(userID, themeID, valid) {
		t.Fatal("validateThemePath rejected the owner-scoped theme path")
	}

	for _, path := range []string{
		"public/upload/" + Digest3(userID) + "/" + userID + "/themes/" + themeID + "/../../other",
		"public/upload/999/" + userID + "/themes/" + themeID,
		"public/upload/" + Digest3(userID) + "/" + userID + "/themes/other",
		"C:/outside/theme",
	} {
		if validateThemePath(userID, themeID, path) {
			t.Fatalf("validateThemePath accepted unsafe path %q", path)
		}
	}
}

func TestThemeValidationErrorsRemainRecognizable(t *testing.T) {
	service := &ThemeService{}
	_, err := service.parseConfig(`{"Name": {"nested": true}}`)
	if err == nil || !strings.Contains(err.Error(), "Name") {
		t.Fatalf("parseConfig error = %v, want field name", err)
	}
}

func TestValidateFilenameRejectsEscapingPaths(t *testing.T) {
	valid := []string{"header.html", "nested/style.css", "foo..bar.js"}
	for _, filename := range valid {
		if !validateFilename(filename) {
			t.Fatalf("validateFilename(%q) = false, want true", filename)
		}
	}

	invalid := []string{"", "../secret", `..\secret`, "/etc/passwd", `C:/outside`, "nested/../secret", "nested//file", "foo:bar"}
	for _, filename := range invalid {
		if validateFilename(filename) {
			t.Fatalf("validateFilename(%q) = true, want false", filename)
		}
	}
}

func TestValidateThemeImageFilenameRejectsNestedNames(t *testing.T) {
	if !validateThemeImageFilename("cover.PNG") {
		t.Fatal("validateThemeImageFilename rejected a normal image filename")
	}
	for _, filename := range []string{"../cover.png", "nested/cover.png", `nested\cover.png`} {
		if validateThemeImageFilename(filename) {
			t.Fatalf("validateThemeImageFilename(%q) = true, want false", filename)
		}
	}
}

func TestThemeImageValidationUsesContentFormat(t *testing.T) {
	var encoded bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	format, err := imageFormat(encoded.Bytes())
	if err != nil || format != "png" {
		t.Fatalf("imageFormat() = %q, %v; want png", format, err)
	}
	if !isSupportedThemeImage(format, ".png") {
		t.Fatal("valid PNG was rejected")
	}
	if isSupportedThemeImage(format, ".jpg") {
		t.Fatal("PNG content was accepted as JPEG")
	}
	if _, err := imageFormat([]byte("not an image")); err == nil {
		t.Fatal("invalid image content was accepted")
	}
}

func TestSafeThemeArchiveNameRemovesPathComponents(t *testing.T) {
	if got := safeThemeArchiveName(`../release\theme`, "507f1f77bcf86cd799439012"); got != "theme.zip" {
		t.Fatalf("safeThemeArchiveName() = %q, want theme.zip", got)
	}
	if got := safeThemeArchiveName("...", "507f1f77bcf86cd799439012"); got != "theme-507f1f77bcf86cd799439012.zip" {
		t.Fatalf("safeThemeArchiveName() = %q, want fallback name", got)
	}
}

func TestThemeOperationsRejectInvalidIdentifiersWithoutPanicking(t *testing.T) {
	service := &ThemeService{}
	if service.DeleteTheme("not-an-id", "also-not-an-id") {
		t.Fatal("DeleteTheme accepted invalid identifiers")
	}
	if ok, _ := service.ExportTheme("not-an-id", "also-not-an-id"); ok {
		t.Fatal("ExportTheme accepted invalid identifiers")
	}
	if _, ok := service.ReadTplContent("not-an-id", "also-not-an-id", "header.html"); ok {
		t.Fatal("ReadTplContent accepted invalid identifiers")
	}
}

func TestPublicThemePathRequiresConfiguredOwnerAndCanonicalSource(t *testing.T) {
	service := &ThemeService{}
	adminID := db.MustObjectIDFromHex("507f1f77bcf86cd799439011")
	themeID := db.MustObjectIDFromHex("507f1f77bcf86cd799439012")
	tests := []struct {
		name  string
		theme info.Theme
		want  bool
	}{
		{name: "built in", theme: info.Theme{ThemeId: themeID, UserId: adminID, Path: "public/blog/themes/default"}, want: true},
		{name: "admin upload", theme: info.Theme{ThemeId: themeID, UserId: adminID, Path: "public/upload/" + Digest3(adminID.Hex()) + "/" + adminID.Hex() + "/themes/" + themeID.Hex()}, want: true},
		{name: "foreign owner", theme: info.Theme{ThemeId: themeID, UserId: db.MustObjectIDFromHex("507f1f77bcf86cd799439013"), Path: "public/blog/themes/default"}},
		{name: "foreign path", theme: info.Theme{ThemeId: themeID, UserId: adminID, Path: "public/upload/other/theme"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := service.publicThemePath(test.theme, adminID); got != test.want {
				t.Fatalf("publicThemePath()=%t, want %t", got, test.want)
			}
		})
	}
}

func TestDeleteThemeStagedRestoresMetadataAndPathOnCleanupFailure(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme")
	backupPath := filepath.Join(root, ".theme.deleting")
	if err := os.Mkdir(themePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themePath, "theme.json"), []byte(`{"Name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}
	metadataPresent := true
	cleanupErr := errors.New("cleanup failed")
	err := deleteThemeStaged(themePath, backupPath, func() error {
		metadataPresent = false
		return nil
	}, func() error {
		metadataPresent = true
		return nil
	}, func(string) error { return cleanupErr })
	if !errors.Is(err, cleanupErr) || !metadataPresent {
		t.Fatalf("delete result=%v metadataPresent=%t", err, metadataPresent)
	}
	if _, err := os.Stat(filepath.Join(themePath, "theme.json")); err != nil {
		t.Fatalf("original theme was not restored: %v", err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("staged path remains after restore: %v", err)
	}
}

func TestDeleteThemeStagedRestoresPathWhenMetadataDeleteFails(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme")
	backupPath := filepath.Join(root, ".theme.deleting")
	if err := os.Mkdir(themePath, 0755); err != nil {
		t.Fatal(err)
	}
	metadataErr := errors.New("metadata delete failed")
	err := deleteThemeStaged(themePath, backupPath, func() error { return metadataErr }, func() error {
		t.Fatal("metadata restore should not run")
		return nil
	}, os.RemoveAll)
	if !errors.Is(err, metadataErr) {
		t.Fatalf("delete result=%v, want metadata failure", err)
	}
	if _, err := os.Stat(themePath); err != nil {
		t.Fatalf("original theme was not restored: %v", err)
	}
}
