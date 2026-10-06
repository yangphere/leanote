package httpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProductionCookieSecureIsForbidden(t *testing.T) {
	for _, section := range []string{"prod", "DEFAULT"} {
		t.Run(section, func(t *testing.T) {
			sections := map[string]map[string]string{
				"DEFAULT": {}, "prod": {},
			}
			sections[section]["cookie.secure"] = "do-not-log-cookie-value"
			err := validateProductionKeys(sections)
			want := "configuration error code=CONFIG_KEY_INVALID key=cookie.secure run_mode=prod"
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %s", err, want)
			}
		})
	}
	if err := validateProductionKeys(map[string]map[string]string{
		"prod": {}, "dev": {"cookie.secure": "true"},
	}); err != nil {
		t.Fatalf("local-only cookie.secure rejected: %v", err)
	}
}

func TestProductionSessionExpiresSourcesAndBounds(t *testing.T) {
	type expiresCase struct {
		name     string
		defaults string
		prod     string
		env      *string
		ttl      time.Duration
		code     string
		key      string
	}
	tests := []expiresCase{
		{name: "absent", ttl: 168 * time.Hour},
		{name: "env unset", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", ttl: 168 * time.Hour},
		{name: "env empty", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", env: stringPtr(""), ttl: 168 * time.Hour},
		{name: "env whitespace", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", env: stringPtr(" \t"), ttl: 168 * time.Hour},
		{name: "minimum", prod: "session.expires=5m\n", ttl: 5 * time.Minute},
		{name: "maximum", prod: "session.expires=8760h\n", ttl: 8760 * time.Hour},
		{name: "custom literal", prod: "session.expires=24h\n", ttl: 24 * time.Hour},
		{name: "quoted", prod: "session.expires=\"24h\"\n", ttl: 24 * time.Hour},
		{name: "inherited", defaults: "session.expires=48h\n", ttl: 48 * time.Hour},
		{name: "override", defaults: "session.expires=7d\n", prod: "session.expires=24h\n", ttl: 24 * time.Hour},
		{name: "env override defaults", defaults: "session.expires=48h\n", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", ttl: 168 * time.Hour},
		{name: "custom env", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", env: stringPtr(" 24h "), ttl: 24 * time.Hour},
		{name: "inherited env", defaults: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", env: stringPtr("5m"), ttl: 5 * time.Minute},
		{name: "other env", prod: "session.expires=${OTHER_TTL}\n", code: "CONFIG_SOURCE_CONFLICT", key: "session.expires"},
		{name: "embedded env", prod: "session.expires=1${LEANOTE_SESSION_EXPIRES}\n", code: "CONFIG_SOURCE_CONFLICT", key: "session.expires"},
		{name: "mixed env", prod: "session.expires=${LEANOTE_SESSION_EXPIRES}${OTHER_TTL}\n", code: "CONFIG_SOURCE_CONFLICT", key: "session.expires"},
		{name: "empty literal", prod: "session.expires=\n", code: "CONFIG_SESSION_EXPIRES_INVALID", key: "session.expires"},
	}
	for _, invalid := range []string{"4m59s", "8761h", "0s", "-1h", "7d", "abc", "999999999999999999999h"} {
		tests = append(tests,
			expiresCase{name: "literal " + invalid, prod: "session.expires=" + invalid + "\n", code: "CONFIG_SESSION_EXPIRES_INVALID", key: "session.expires"},
			expiresCase{name: "env " + invalid, prod: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", env: stringPtr(invalid), code: "CONFIG_SESSION_EXPIRES_INVALID", key: "LEANOTE_SESSION_EXPIRES"},
		)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LEANOTE_SESSION_EXPIRES", "")
			if test.env == nil {
				if err := os.Unsetenv("LEANOTE_SESSION_EXPIRES"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("LEANOTE_SESSION_EXPIRES", *test.env)
			}
			data := []byte(test.defaults + "[prod]\n" + test.prod)
			sections, err := parseProductionSections(data)
			if err != nil {
				t.Fatal(err)
			}
			err = validateProductionSessionExpires(sections)
			if test.code != "" {
				want := fmt.Sprintf("configuration error code=%s key=%s run_mode=prod", test.code, test.key)
				if err == nil || err.Error() != want {
					t.Fatalf("error = %v, want %s", err, want)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid expires rejected: %v", err)
			}
			cfg, err := ParseConfig(data, "prod")
			if err != nil {
				t.Fatal(err)
			}
			ttl, err := productionSessionTTL(cfg)
			if err != nil || ttl != test.ttl {
				t.Fatalf("runtime TTL = %v, error = %v, want %v", ttl, err, test.ttl)
			}
		})
	}
}

func TestProductionRuntimeSessionSettings(t *testing.T) {
	base := t.TempDir()
	configText := "[prod]\ndb.dbname=leanote\n"
	for _, key := range productionContentRootKeys {
		path := filepath.Join(base, key)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		configText += key + "=" + path + "\n"
	}
	publicStatic := filepath.Join(base, "static")
	if err := os.MkdirAll(publicStatic, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MONGODB_URL", "mongodb://db.example/leanote")
	for _, test := range []struct {
		name    string
		site    string
		expires string
		envTTL  string
		secure  bool
		ttl     time.Duration
	}{
		{name: "https literal", site: "https://note.example.com", secure: true, ttl: 168 * time.Hour},
		{name: "http literal", site: "http://127.0.0.1:9000", ttl: 168 * time.Hour},
		{name: "https env", site: "${LEANOTE_SITE_URL}", secure: true, ttl: 168 * time.Hour},
		{name: "custom literal", site: "https://note.example.com", expires: "session.expires=24h\n", secure: true, ttl: 24 * time.Hour},
		{name: "custom env", site: "${LEANOTE_SITE_URL}", expires: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", envTTL: "48h", secure: true, ttl: 48 * time.Hour},
		{name: "empty env", site: "https://note.example.com", expires: "session.expires=${LEANOTE_SESSION_EXPIRES}\n", secure: true, ttl: 168 * time.Hour},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LEANOTE_SITE_URL", "https://note.example.com")
			t.Setenv("LEANOTE_SESSION_EXPIRES", test.envTTL)
			cfg, err := ParseConfig([]byte(configText+"site.url="+test.site+"\n"+test.expires), "prod")
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := ValidateProductionRuntimeConfig(cfg, publicStatic)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.CookieSecure != test.secure || runtime.SessionTTL != test.ttl {
				t.Fatalf("session handoff = Secure %t TTL %v, want %t/%v", runtime.CookieSecure, runtime.SessionTTL, test.secure, test.ttl)
			}
		})
	}
	cfg, err := ParseConfig([]byte(configText+"session.expires=7d\n"), "prod")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateProductionRuntimeConfig(cfg, publicStatic)
	if cfgErr, ok := err.(*ConfigError); !ok || cfgErr.Code != "CONFIG_SESSION_EXPIRES_INVALID" || cfgErr.Key != "session.expires" {
		t.Fatalf("direct runtime invalid TTL error = %v", err)
	}
}
