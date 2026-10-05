package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/service"
)

func TestValidateProductionSecretEnforcesContract(t *testing.T) {
	valid := strings.Repeat("a", 32)
	for name, secret := range map[string]string{
		"valid":          valid,
		"minimum length": strings.Repeat("b", 32),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProductionSecret(secret); err != nil {
				t.Fatalf("validateProductionSecret() error = %v", err)
			}
		})
	}
	for name, secret := range map[string]string{
		"short":     strings.Repeat("a", 31),
		"non ascii": strings.Repeat("a", 31) + "é",
		"control":   strings.Repeat("a", 31) + "\n",
		"public":    "V85ZzBeTnzpsHyjQX4zukbQ8qqtju9y2aDM55VWxAH9Qop19poekx3xkcDVvrD0y",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProductionSecret(secret); err == nil {
				t.Fatal("validateProductionSecret() accepted an invalid secret")
			}
		})
	}
}

func TestValidateMongoURLEnforcesDatabaseAndHostContract(t *testing.T) {
	if err := validateMongoURL("mongodb://db.example/leanote?retryWrites=true", "leanote"); err != nil {
		t.Fatalf("validateMongoURL() valid URI error = %v", err)
	}
	for name, raw := range map[string]string{
		"localhost":       "mongodb://localhost/leanote",
		"localhost fqdn":  "mongodb://localhost./leanote",
		"loopback":        "mongodb://127.0.0.1/leanote",
		"mapped loopback": "mongodb://[::ffff:127.0.0.1]/leanote",
		"test db":         "mongodb://db.example/leanote_test",
		"mismatch":        "mongodb://db.example/other",
		"extra path":      "mongodb://db.example/leanote/extra",
		"zero port":       "mongodb://db.example:0/leanote",
		"fragment":        "mongodb://db.example/leanote#fragment",
		"opaque":          "mongodb:leanote",
		"wrong scheme":    "http://db.example/leanote",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateMongoURL(raw, "leanote"); err == nil {
				t.Fatal("validateMongoURL() accepted an invalid URI")
			}
		})
	}
}

func TestParseProductionSectionsRejectsMongoURIFragment(t *testing.T) {
	data := []byte("[prod]\n" +
		"db.urlEnv=${MONGODB_URL}\n" +
		"db.dbname=leanote\n" +
		"app.secret=${LEANOTE_APP_SECRET}\n")
	sections, err := parseProductionSections(data)
	if err != nil {
		t.Fatalf("parseProductionSections() error = %v", err)
	}
	if err := validateProductionSecret("a-valid-secret-that-is-long-enough-012345"); err != nil {
		t.Fatalf("validateProductionSecret() error = %v", err)
	}

	t.Setenv("MONGODB_URL", "mongodb://db.example/leanote#fragment")
	t.Setenv("LEANOTE_APP_SECRET", "a-valid-secret-that-is-long-enough-012345")
	path := filepath.Join(t.TempDir(), "app.conf")
	if err := os.WriteFile(path, data, 0440); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0440); err != nil {
		t.Fatal(err)
	}

	if err := validateMongoURL(os.Getenv("MONGODB_URL"), sections["prod"]["db.dbname"]); err == nil {
		t.Fatal("validateMongoURL() accepted a URI fragment")
	}
}

func TestParseProductionSectionsAcceptsCommentLines(t *testing.T) {
	data := []byte("; production settings\n# keep this file non-sensitive\n[prod]\n" +
		"db.urlEnv=${MONGODB_URL}\n" +
		"db.dbname=leanote\n" +
		"app.secret=${LEANOTE_APP_SECRET}\n")
	if _, err := parseProductionSections(data); err != nil {
		t.Fatalf("parseProductionSections() rejected comment lines: %v", err)
	}
}

func TestProductionSectionsRejectForbiddenInheritedKey(t *testing.T) {
	data := []byte("db.host=internal-db\n[prod]\n" +
		"db.urlEnv=${MONGODB_URL}\n" +
		"db.dbname=leanote\n" +
		"app.secret=${LEANOTE_APP_SECRET}\n")
	sections, err := parseProductionSections(data)
	if err != nil {
		t.Fatalf("parseProductionSections() error = %v", err)
	}
	if !forbiddenProductionKey("db.host") || sections["DEFAULT"]["db.host"] != "internal-db" {
		t.Fatal("test fixture did not include the inherited forbidden key")
	}
}

func TestValidateProductionContentRootKeysRequiresProdValues(t *testing.T) {
	values := map[string]string{}
	for _, key := range productionContentRootKeys {
		values[key] = "/var/lib/leanote/" + strings.ReplaceAll(key, ".", "-")
	}
	if err := validateProductionContentRootKeys(values); err != nil {
		t.Fatalf("complete content root keys rejected: %v", err)
	}

	delete(values, "content.public.quarantine")
	err := validateProductionContentRootKeys(values)
	configErr, ok := err.(*ConfigError)
	if !ok || configErr.Code != "CONFIG_CONTENT_ROOT_MISSING" || configErr.Key != "content.public.quarantine" {
		t.Fatalf("missing prod root error = %v, want CONFIG_CONTENT_ROOT_MISSING/content.public.quarantine", err)
	}
}

func TestValidateSiteURL(t *testing.T) {
	for _, candidate := range []string{
		"http://example.com",
		"https://example.com:8443",
		"http://127.0.0.1",
		"https://[::1]:443",
		"http://localhost:9000",
	} {
		t.Run(candidate, func(t *testing.T) {
			if err := validateSiteURL(candidate); err != nil {
				t.Fatalf("validateSiteURL(%q) error = %v", candidate, err)
			}
		})
	}
	for _, candidate := range []string{
		"example.com",
		"ftp://example.com",
		"HTTP://example.com",
		"http://example.com/",
		"http://example.com/path",
		"http://example.com?query",
		"http://example.com#fragment",
		"http://user@example.com",
		"http://",
		"http://example.com:0",
		"http://example.com:65536",
		" http://example.com",
		"http://example.com ",
		"http://example.com?",
		"http://example.com#",
	} {
		t.Run(candidate, func(t *testing.T) {
			if err := validateSiteURL(candidate); err == nil {
				t.Fatalf("validateSiteURL(%q) accepted an invalid URL", candidate)
			}
		})
	}
}

func TestValidateProductionSiteURLSourceContract(t *testing.T) {
	originalEnv, hadOriginalEnv := os.LookupEnv("LEANOTE_SITE_URL")
	t.Cleanup(func() {
		if hadOriginalEnv {
			_ = os.Setenv("LEANOTE_SITE_URL", originalEnv)
		} else {
			_ = os.Unsetenv("LEANOTE_SITE_URL")
		}
	})
	tests := []struct {
		name string
		prod map[string]string
		env  *string
		code string
		key  string
	}{
		{name: "missing", prod: map[string]string{}, code: "CONFIG_KEY_INVALID", key: "site.url"},
		{name: "other placeholder", prod: map[string]string{"site.url": "${OTHER_URL}"}, code: "CONFIG_SOURCE_CONFLICT", key: "site.url"},
		{name: "env missing", prod: map[string]string{"site.url": "${LEANOTE_SITE_URL}"}, code: "CONFIG_VALUE_MISSING", key: "LEANOTE_SITE_URL"},
		{name: "env empty", prod: map[string]string{"site.url": "${LEANOTE_SITE_URL}"}, env: stringPtr("  \t"), code: "CONFIG_VALUE_EMPTY", key: "LEANOTE_SITE_URL"},
		{name: "literal invalid", prod: map[string]string{"site.url": "https://example.com/"}, code: "CONFIG_SITE_URL_INVALID", key: "site.url"},
		{name: "env invalid", prod: map[string]string{"site.url": "${LEANOTE_SITE_URL}"}, env: stringPtr("https://example.com/"), code: "CONFIG_SITE_URL_INVALID", key: "LEANOTE_SITE_URL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.env == nil {
				if err := os.Unsetenv("LEANOTE_SITE_URL"); err != nil {
					t.Fatalf("unset LEANOTE_SITE_URL: %v", err)
				}
			} else {
				t.Setenv("LEANOTE_SITE_URL", *test.env)
			}
			err := validateProductionSiteURL(test.prod)
			configErr, ok := err.(*ConfigError)
			if !ok || configErr.Code != test.code || configErr.Key != test.key {
				t.Fatalf("error = %#v, want %s/%s", err, test.code, test.key)
			}
			if strings.Contains(err.Error(), "example.com") || strings.Contains(err.Error(), "OTHER_URL") {
				t.Fatalf("configuration error leaked a value: %v", err)
			}
		})
	}
	t.Setenv("LEANOTE_SITE_URL", "https://note.example.com")
	if err := validateProductionSiteURL(map[string]string{"site.url": "${LEANOTE_SITE_URL}"}); err != nil {
		t.Fatalf("valid env site URL rejected: %v", err)
	}
}

func stringPtr(value string) *string { return &value }

func TestProductionSiteURLExpandsFromEnvironment(t *testing.T) {
	t.Setenv("LEANOTE_SITE_URL", "https://note.example.com")
	cfg, err := ParseConfig([]byte("[prod]\nsite.url=${LEANOTE_SITE_URL}\n"), "prod")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if got, ok := cfg.String("site.url"); !ok || got != "https://note.example.com" {
		t.Fatalf("site.url = %q, ok=%t", got, ok)
	}
	service.SetAppConfigSource(cfg)
	t.Cleanup(func() { service.SetAppConfigSource(nil) })
	if got := (&service.ConfigService{}).GetDefaultDomain(); got != "note.example.com" {
		t.Fatalf("default domain = %q, want note.example.com", got)
	}
}
