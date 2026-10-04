package httpserver

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yangphere/leanote/app/service"
	"github.com/yangphere/leanote/app/service/contentfs"
)

// ValidateLocalRuntimeConfig builds the same typed runtime handoff used by
// production for the dev/test modes. Local modes deliberately permit loopback
// Mongo and repository-relative content roots, but still run the content root
// validator before the listener or database is opened.
func ValidateLocalRuntimeConfig(cfg *Config, runMode, appBase string, publicStaticRoots ...string) (*ProductionConfig, error) {
	if cfg == nil || (runMode != "dev" && runMode != "test") {
		return nil, configError("CONFIG_RUN_MODE_INVALID", runMode)
	}
	pdfRenderer, err := parsePDFRendererConfig(cfg)
	if err != nil {
		return nil, err
	}
	databaseName := cfg.StringDefault("db.dbname", "")
	if runMode == "test" && databaseName != "leanote_test" {
		return nil, configError("CONFIG_TEST_DATABASE_INVALID", "db.dbname")
	}
	if strings.TrimSpace(databaseName) == "" {
		return nil, configError("CONFIG_KEY_INVALID", "db.dbname")
	}
	base, err := filepath.Abs(appBase)
	if err != nil {
		return nil, configError("CONFIG_PATH_INVALID", "appBase")
	}
	values := make(map[string]string, len(productionContentRootKeys))
	defaults := map[string]string{
		"content.private.data":       filepath.Join(base, ".leanote-data", "private", "files"),
		"content.private.quarantine": filepath.Join(base, ".leanote-data", "private", "quarantine"),
		"content.public.data":        filepath.Join(base, ".leanote-data", "public", "upload"),
		"content.public.quarantine":  filepath.Join(base, ".leanote-data", "public", "quarantine"),
		"content.temporary":          filepath.Join(base, ".leanote-data", "tmp"),
		"admin.backup.root":          filepath.Join(base, ".leanote-data", "backup"),
	}
	for _, key := range productionContentRootKeys {
		value := cfg.StringDefault(key, defaults[key])
		if strings.TrimSpace(value) == "" {
			return nil, configError("CONFIG_CONTENT_ROOT_MISSING", key)
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(base, value)
		}
		value, err = filepath.Abs(filepath.Clean(value))
		if err != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_INVALID", key)
		}
		if err := os.MkdirAll(value, 0o755); err != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_UNWRITABLE", key)
		}
		values[key] = value
	}
	publicStaticRoot := filepath.Join(base, "public")
	if len(publicStaticRoots) > 0 && strings.TrimSpace(publicStaticRoots[0]) != "" {
		publicStaticRoot = publicStaticRoots[0]
	}
	publicStaticRoot, err = filepath.Abs(publicStaticRoot)
	if err != nil {
		return nil, configError("CONFIG_CONTENT_ROOT_INVALID", "content.public.data")
	}
	if info, statErr := os.Stat(publicStaticRoot); statErr != nil || !info.IsDir() {
		return nil, configError("CONFIG_CONTENT_ROOT_MISSING", "content.public.data")
	}
	servedRoots := runtimeServedRoots(values["content.public.data"], publicStaticRoot)
	if err = validateContentRootPairs(values, servedRoots); err != nil {
		return nil, err
	}
	backup, err := canonicalBackupRoot(values["admin.backup.root"])
	if err != nil {
		return nil, configError("CONFIG_CONTENT_ROOT_UNWRITABLE", "admin.backup.root")
	}
	for _, root := range append([]string(nil), values["content.private.data"], values["content.private.quarantine"], values["content.public.quarantine"], values["content.temporary"]) {
		if runtimePathsOverlap(backup, root) {
			return nil, configError("CONFIG_CONTENT_ROOT_OVERLAP", "admin.backup.root")
		}
	}
	for _, served := range servedRoots {
		if runtimePathsOverlap(backup, served) {
			return nil, configError("CONFIG_CONTENT_ROOT_PUBLIC", "admin.backup.root")
		}
	}
	mongoURL, err := localMongoURL(cfg, databaseName)
	if err != nil {
		return nil, err
	}
	identity, err := databaseIdentity(mongoURL, databaseName)
	if err != nil {
		return nil, err
	}
	digest, err := identity.Digest()
	if err != nil {
		return nil, configError("CONFIG_MONGO_INVALID", "db.url")
	}
	canonicalValues := make(map[string]string, len(values))
	for key, value := range values {
		canonical, evalErr := filepath.EvalSymlinks(value)
		if evalErr != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_MISSING", key)
		}
		canonicalValues[key] = filepath.Clean(canonical)
	}
	return &ProductionConfig{
		Addr:            fmt.Sprintf("%s:%d", cfg.StringDefault("http.addr", "127.0.0.1"), cfg.IntDefault("http.port", 9000)),
		ShutdownTimeout: ShutdownTimeout(cfg), DatabaseName: databaseName,
		ConfiguredDatabaseIdentity: identity, DatabaseIdentityDigest: digest,
		CredentialProviderRef: service.CredentialProviderRef{ProviderKind: "config", OpaqueHandle: "local", Owner: "interface-http"},
		ContentRoots: service.ContentRoots{
			PrivateFiles: service.ContentRootPair{Data: canonicalValues["content.private.data"], Quarantine: canonicalValues["content.private.quarantine"]},
			PublicUpload: service.ContentRootPair{Data: canonicalValues["content.public.data"], Quarantine: canonicalValues["content.public.quarantine"]},
			Temporary:    canonicalValues["content.temporary"], ServedRoots: servedRoots,
		}, BackupRoot: backup, PDFRenderer: pdfRenderer,
	}, nil
}

func validateContentRootPairs(values map[string]string, servedRoots []string) error {
	_, err := contentfs.ValidateContentRoots(contentfs.ContentRootsConfig{
		PrivateFiles: contentfs.DurableRootConfig{Data: values["content.private.data"], Quarantine: values["content.private.quarantine"]},
		PublicUpload: contentfs.DurableRootConfig{Data: values["content.public.data"], Quarantine: values["content.public.quarantine"]},
		Temporary:    values["content.temporary"], ServedRoots: servedRoots,
	})
	if err != nil {
		return mapContentRootError(err)
	}
	return nil
}

// MongoURL builds the local MongoDB URL from the same config keys used by the
// command entrypoint. Credentials are escaped once here so development and
// test startup cannot disagree about passwords containing URI delimiters.
func MongoURL(cfg *Config, databaseName string) (string, error) {
	for _, key := range []string{"db.url", "db.urlEnv"} {
		if value, ok := cfg.String(key); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	host := cfg.StringDefault("db.host", "127.0.0.1")
	port := cfg.StringDefault("db.port", "27017")
	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return "", configError("CONFIG_MONGO_INVALID", "db.host")
	}
	user := cfg.StringDefault("db.username", "")
	password := cfg.StringDefault("db.password", "")
	credentials := ""
	if user != "" || password != "" {
		credentials = url.QueryEscape(user) + ":" + url.QueryEscape(password) + "@"
	}
	return "mongodb://" + credentials + host + ":" + port + "/" + databaseName, nil
}

func localMongoURL(cfg *Config, databaseName string) (string, error) {
	return MongoURL(cfg, databaseName)
}
