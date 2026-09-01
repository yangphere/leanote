package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	i18n "github.com/yangphere/leanote/app/lea/i18n"
)

func TestSetupPresentationRendersTemplatesWithConfiguredMessages(t *testing.T) {
	root := t.TempDir()
	viewsDir := filepath.Join(root, "app", "views", "home")
	messagesDir := filepath.Join(root, "messages", "en-us")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("create views directory: %v", err)
	}
	if err := os.MkdirAll(messagesDir, 0o755); err != nil {
		t.Fatalf("create messages directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "login.html"), []byte("[{{leaMsg . \"greeting\"}} {{.Name}}]"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(messagesDir, "messages.conf"), []byte("greeting=Hello\n"), 0o644); err != nil {
		t.Fatalf("write messages: %v", err)
	}

	cfg, err := httpserver.ParseConfig([]byte("i18n.default_language=en-us\n"), "")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	savedRenderer := httpserver.TemplateRenderer
	savedDefaultLanguage := i18n.DefaultLanguage
	t.Cleanup(func() {
		httpserver.TemplateRenderer = savedRenderer
		i18n.DefaultLanguage = savedDefaultLanguage
	})

	if err := setupPresentation(cfg, filepath.Dir(viewsDir), filepath.Dir(messagesDir)); err != nil {
		t.Fatalf("setup presentation: %v", err)
	}
	rec := httptest.NewRecorder()
	httpserver.TemplateResult(http.StatusOK, "home/login.html", map[string]interface{}{
		"currentLocale": "fr-fr",
		"Name":          "leanote",
	}).Apply(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("render status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[Hello leanote]" {
		t.Fatalf("rendered body = %q, want configured message and template data", got)
	}
}

func TestApplicationBaseUsesConfigParentUnlessItIsConfDirectory(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "conventional conf", path: filepath.Join("workspace", "conf", "app.conf"), want: filepath.Clean("workspace")},
		{name: "uppercase conf on Windows", path: filepath.Join("workspace", "CONF", "app.conf"), want: filepath.Clean("workspace")},
		{name: "custom config directory", path: filepath.Join("workspace", "config", "app.conf"), want: filepath.Clean(filepath.Join("workspace", "config"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := applicationBase(test.path); got != test.want {
				t.Fatalf("applicationBase(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestStaticAssetRootIsRelativeToApplicationBase(t *testing.T) {
	root := filepath.Join("workspace", "release")
	if got, want := staticAssetRoot(root, "public"), filepath.Join(root, "public"); got != want {
		t.Fatalf("static asset root = %q, want %q", got, want)
	}
}

func TestStaticHandlerServesExactFiles(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "public", "images", "favicon.ico")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("create asset directory: %v", err)
	}
	if err := os.WriteFile(file, []byte("icon"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	handler := staticHandler(root, "public/images/favicon.ico")
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "icon" {
		t.Fatalf("exact static file: status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestSetupPresentationRejectsMissingMessagesDirectory(t *testing.T) {
	root := t.TempDir()
	viewsDir := filepath.Join(root, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("create views directory: %v", err)
	}
	cfg, err := httpserver.ParseConfig(nil, "")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if err := setupPresentation(cfg, viewsDir, filepath.Join(root, "missing-messages")); err == nil {
		t.Fatal("setupPresentation succeeded with a missing messages directory")
	}
}

func TestSetupPresentationRejectsMalformedMessageFile(t *testing.T) {
	root := t.TempDir()
	viewsDir := filepath.Join(root, "views")
	messagesDir := filepath.Join(root, "messages", "en-us")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("create views directory: %v", err)
	}
	if err := os.MkdirAll(messagesDir, 0o755); err != nil {
		t.Fatalf("create messages directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(messagesDir, "broken.conf"), []byte("not a config entry\n"), 0o644); err != nil {
		t.Fatalf("write malformed message file: %v", err)
	}
	cfg, err := httpserver.ParseConfig(nil, "")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if err := setupPresentation(cfg, viewsDir, filepath.Join(root, "messages")); err == nil {
		t.Fatal("setupPresentation succeeded with a malformed message file")
	}
}

func TestValidateCLIOptionsRequiresProductionMode(t *testing.T) {
	if err := validateCLIOptions("dev", true, true); err == nil {
		t.Fatal("validateCLIOptions() accepted non-production run mode")
	}
}

func TestValidateCLIOptionsRequiresExplicitCanonicalArguments(t *testing.T) {
	if err := validateCLIOptions("", false, false); err == nil {
		t.Fatal("validateCLIOptions() accepted missing production arguments")
	} else if cfgErr, ok := err.(*httpserver.ConfigError); !ok || cfgErr.Code != "CONFIG_RUN_MODE_INVALID" {
		t.Fatalf("missing run mode error = %v, want CONFIG_RUN_MODE_INVALID", err)
	}
	if err := validateCLIOptions("prod", true, false); err == nil {
		t.Fatal("validateCLIOptions() accepted missing config path")
	} else if cfgErr, ok := err.(*httpserver.ConfigError); !ok || cfgErr.Code != "CONFIG_PATH_INVALID" {
		t.Fatalf("missing config path error = %v, want CONFIG_PATH_INVALID", err)
	}
}
