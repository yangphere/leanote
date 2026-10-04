package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/domain"
	"github.com/yangphere/leanote/app/info"
)

func TestRenderBlogTemplateSelectsTheBuiltInStyleDirectory(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	styles := map[string]string{
		"default":   "default-template",
		"elegant":   "elegant-template",
		"nav_fixed": "fixed-template",
	}
	for directory, body := range styles {
		root := filepath.Join(publicRoot, "blog", "themes", directory)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	previous := configuredContentRoots
	configuredContentRoots = ContentRoots{ServedRoots: []string{publicRoot}}
	t.Cleanup(func() { configuredContentRoots = previous })

	service := &ThemeService{}
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		style string
		want  string
	}{
		{name: "default", style: defaultStyle, want: styles["default"]},
		{name: "elegant", style: elegantStyle, want: styles["elegant"]},
		{name: "fixed", style: fixedStyle, want: styles["nav_fixed"]},
	} {
		t.Run(test.name, func(t *testing.T) {
			blog := info.UserBlog{UserId: owner, Style: test.style, ThemePath: service.GetDefaultThemePath(test.style)}
			body, err := service.RenderBlogTemplate(blog, "index.html", nil)
			if err != nil {
				t.Fatalf("RenderBlogTemplate() error = %v", err)
			}
			if got := strings.TrimSpace(string(body)); got != test.want {
				t.Fatalf("rendered template = %q, want %q", got, test.want)
			}
		})
	}
}
