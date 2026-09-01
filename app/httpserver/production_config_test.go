package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
