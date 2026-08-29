package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
	i18n "github.com/yangphere/leanote/app/lea/i18n"
)

const publicSecret = defaultPublicSecret

func TestValidateProdSecretAcceptsRealSecret(t *testing.T) {
	if err := validateProdSecret("prod", "a-real-production-secret"); err != nil {
		t.Fatalf("validateProdSecret(prod, real) = %v, want nil", err)
	}
}

func TestValidateProdSecretRejectsEmpty(t *testing.T) {
	err := validateProdSecret("prod", "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("validateProdSecret(prod, empty) = %v, want empty-secret error", err)
	}
}

func TestValidateProdSecretRejectsPublicDefault(t *testing.T) {
	err := validateProdSecret("prod", publicSecret)
	if err == nil || !strings.Contains(err.Error(), "public repository default") {
		t.Fatalf("validateProdSecret(prod, default) = %v, want public-default error", err)
	}
}

func TestValidateProdSecretSkipsNonProd(t *testing.T) {
	for _, mode := range []string{"dev", "test"} {
		if err := validateProdSecret(mode, ""); err != nil {
			t.Fatalf("validateProdSecret(%s, empty) = %v, want nil (non-prod skips check)", mode, err)
		}
	}
}

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
