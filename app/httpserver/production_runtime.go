package httpserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/yangphere/leanote/app/service"
	"github.com/yangphere/leanote/app/service/contentfs"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

// ProductionConfig is the immutable handoff from interface startup to
// application consumers. It contains no secret or raw Mongo credentials.
type ProductionConfig struct {
	Addr                       string
	ShutdownTimeout            time.Duration
	CookieSecure               bool
	SessionTTL                 time.Duration
	DatabaseName               string
	ConfiguredDatabaseIdentity service.ConfiguredDatabaseIdentity
	DatabaseIdentityDigest     string
	CredentialProviderRef      service.CredentialProviderRef
	ContentRoots               service.ContentRoots
	PDFRenderer                service.PDFRendererConfig
	BackupRoot                 string
}

// ValidateProductionRuntime validates the production config and its typed
// filesystem contract before listeners or database connections are opened.
func ValidateProductionRuntime(path string) (*ProductionConfig, error) {
	cfg, err := ValidateProductionConfig(path)
	if err != nil {
		return nil, err
	}
	publicRoot, err := productionPublicRoot()
	if err != nil {
		return nil, err
	}
	return ValidateProductionRuntimeConfig(cfg, publicRoot)
}

// ValidateProductionRuntimeConfig builds the typed runtime handoff from the
// already validated configuration. Keeping the raw Config outside
// ProductionConfig prevents admin consumers from gaining a credential source.
func ValidateProductionRuntimeConfig(cfg *Config, publicStaticRoots ...string) (*ProductionConfig, error) {
	if cfg == nil {
		return nil, configError("CONFIG_KEY_INVALID", "prod")
	}
	pdfRenderer, err := parsePDFRendererConfig(cfg)
	if err != nil {
		return nil, err
	}
	sessionTTL, err := productionSessionTTL(cfg)
	if err != nil {
		return nil, err
	}
	publicStaticRoot := ""
	if len(publicStaticRoots) > 0 {
		publicStaticRoot = publicStaticRoots[0]
	} else {
		var err error
		publicStaticRoot, err = productionPublicRoot()
		if err != nil {
			return nil, err
		}
	}
	values := make(map[string]string, len(productionContentRootKeys))
	for _, key := range productionContentRootKeys {
		value, ok := cfg.String(key)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, configError("CONFIG_CONTENT_ROOT_MISSING", key)
		}
		if !filepath.IsAbs(value) {
			return nil, configError("CONFIG_CONTENT_ROOT_RELATIVE", key)
		}
		values[key] = filepath.Clean(value)
	}
	servedRoots := runtimeServedRoots(values["content.public.data"], publicStaticRoot)
	_, err = contentfs.ValidateContentRoots(contentfs.ContentRootsConfig{
		PrivateFiles: contentfs.DurableRootConfig{Data: values["content.private.data"], Quarantine: values["content.private.quarantine"]},
		PublicUpload: contentfs.DurableRootConfig{Data: values["content.public.data"], Quarantine: values["content.public.quarantine"]},
		Temporary:    values["content.temporary"],
		ServedRoots:  servedRoots,
	})
	if err != nil {
		return nil, mapContentRootError(err)
	}
	for _, key := range productionContentRootKeys {
		canonical, canonicalErr := filepath.EvalSymlinks(values[key])
		if canonicalErr != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_MISSING", key)
		}
		absolute, absoluteErr := filepath.Abs(canonical)
		if absoluteErr != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_UNWRITABLE", key)
		}
		values[key] = filepath.Clean(absolute)
	}
	canonicalPublicStatic, err := filepath.EvalSymlinks(publicStaticRoot)
	if err != nil {
		return nil, configError("CONFIG_CONTENT_ROOT_MISSING", "content.public.data")
	}
	canonicalPublicStatic, err = filepath.Abs(canonicalPublicStatic)
	if err != nil {
		return nil, configError("CONFIG_CONTENT_ROOT_UNWRITABLE", "content.public.data")
	}
	servedRoots = runtimeServedRoots(values["content.public.data"], filepath.Clean(canonicalPublicStatic))
	backup, err := canonicalBackupRoot(values["admin.backup.root"])
	if err != nil {
		code := "CONFIG_CONTENT_ROOT_UNWRITABLE"
		if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "missing") {
			code = "CONFIG_CONTENT_ROOT_MISSING"
		}
		return nil, configError(code, "admin.backup.root")
	}
	for _, served := range servedRoots {
		canonical, canonicalErr := filepath.EvalSymlinks(served)
		if canonicalErr != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_MISSING", "content.public.data")
		}
		if runtimePathsOverlap(backup, canonical) {
			return nil, configError("CONFIG_CONTENT_ROOT_PUBLIC", "admin.backup.root")
		}
	}
	for _, root := range []struct {
		key  string
		path string
	}{
		{key: "content.private.data", path: values["content.private.data"]},
		{key: "content.private.quarantine", path: values["content.private.quarantine"]},
		{key: "content.public.quarantine", path: values["content.public.quarantine"]},
		{key: "content.temporary", path: values["content.temporary"]},
	} {
		canonical, canonicalErr := filepath.EvalSymlinks(root.path)
		if canonicalErr != nil {
			return nil, configError("CONFIG_CONTENT_ROOT_UNWRITABLE", root.key)
		}
		if runtimePathsOverlap(backup, canonical) {
			return nil, configError("CONFIG_CONTENT_ROOT_OVERLAP", "admin.backup.root")
		}
	}
	mongoURL, _ := os.LookupEnv("MONGODB_URL")
	identity, err := databaseIdentity(mongoURL, cfg.StringDefault("db.dbname", ""))
	if err != nil {
		return nil, err
	}
	digest, err := identity.Digest()
	if err != nil {
		return nil, configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
	}
	return &ProductionConfig{
		Addr:                       fmt.Sprintf("%s:%d", cfg.StringDefault("http.addr", "0.0.0.0"), cfg.IntDefault("http.port", 9000)),
		ShutdownTimeout:            ShutdownTimeout(cfg),
		CookieSecure:               strings.HasPrefix(cfg.StringDefault("site.url", ""), "https://"),
		SessionTTL:                 sessionTTL,
		DatabaseName:               cfg.StringDefault("db.dbname", ""),
		ConfiguredDatabaseIdentity: identity,
		DatabaseIdentityDigest:     digest,
		CredentialProviderRef:      service.CredentialProviderRef{ProviderKind: "env", OpaqueHandle: "MONGODB_URL", Owner: "interface-http"},
		ContentRoots: service.ContentRoots{
			PrivateFiles: service.ContentRootPair{Data: values["content.private.data"], Quarantine: values["content.private.quarantine"]},
			PublicUpload: service.ContentRootPair{Data: values["content.public.data"], Quarantine: values["content.public.quarantine"]},
			Temporary:    values["content.temporary"], ServedRoots: servedRoots,
		},
		BackupRoot:  backup,
		PDFRenderer: pdfRenderer,
	}, nil
}

func canonicalBackupRoot(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("backup root missing: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup root is not a directory")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("backup root unavailable: %w", err)
	}
	probe, err := os.CreateTemp(canonical, ".leanote-backup-root-probe-")
	if err != nil {
		return "", fmt.Errorf("backup root is not writable: %w", err)
	}
	probeName := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(probeName)
	if closeErr != nil || removeErr != nil {
		return "", fmt.Errorf("backup root is not writable: %w", errors.Join(closeErr, removeErr))
	}
	return filepath.Clean(canonical), nil
}

func mapContentRootError(err error) error {
	message := strings.ToLower(err.Error())
	code := "CONFIG_CONTENT_ROOT_UNWRITABLE"
	if strings.Contains(message, "inaccessible") || strings.Contains(message, "unavailable") {
		code = "CONFIG_CONTENT_ROOT_MISSING"
	}
	if strings.Contains(message, "absolute path") {
		code = "CONFIG_CONTENT_ROOT_RELATIVE"
	}
	if strings.Contains(message, "served root") {
		code = "CONFIG_CONTENT_ROOT_PUBLIC"
	} else if strings.Contains(message, "overlap") {
		code = "CONFIG_CONTENT_ROOT_OVERLAP"
	}
	if strings.Contains(message, "filesystem") && errors.Is(err, syscall.EXDEV) {
		code = "CONFIG_CONTENT_ROOT_CROSS_DEVICE"
	}
	return configError(code, contentRootConfigKey(message))
}

func contentRootConfigKey(message string) string {
	switch {
	case strings.Contains(message, "private_files.quarantine"):
		return "content.private.quarantine"
	case strings.Contains(message, "private_files.data"), strings.Contains(message, "private files pair"):
		return "content.private.data"
	case strings.Contains(message, "public_upload.quarantine"):
		return "content.public.quarantine"
	case strings.Contains(message, "public_upload.data"), strings.Contains(message, "public upload pair"):
		return "content.public.data"
	case strings.Contains(message, "temporary"):
		return "content.temporary"
	case strings.Contains(message, "served_roots"):
		return "content.public.data"
	default:
		return "content.private.data"
	}
}

func runtimeServedRoots(publicDataRoot, publicStaticRoot string) []string {
	return []string{publicDataRoot, publicStaticRoot}
}

func productionPublicRoot() (string, error) {
	if executable, err := os.Executable(); err == nil {
		appBase := filepath.Dir(filepath.Dir(executable))
		publicRoot := filepath.Join(appBase, "public")
		if info, statErr := os.Stat(publicRoot); statErr == nil && info.IsDir() {
			if canonical, evalErr := filepath.EvalSymlinks(publicRoot); evalErr == nil {
				return filepath.Clean(canonical), nil
			}
		}
	}
	return "", configError("CONFIG_CONTENT_ROOT_MISSING", "content.public.data")
}

func runtimePathsOverlap(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}
	for _, pair := range [][2]string{{left, right}, {right, left}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))) {
			return true
		}
	}
	return false
}

func databaseIdentity(raw, databaseName string) (service.ConfiguredDatabaseIdentity, error) {
	cs, err := connstring.ParseAndValidate(raw)
	if err != nil || len(cs.Hosts) == 0 {
		return service.ConfiguredDatabaseIdentity{}, configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
	}
	host := cs.Hosts[0]
	port := ""
	if cs.Scheme == "mongodb" {
		if splitHost, splitPort, splitErr := splitMongoHost(host); splitErr == nil {
			host, port = splitHost, splitPort
		}
	}
	identity := service.ConfiguredDatabaseIdentity{Scheme: cs.Scheme, ClusterHost: host, ClusterPort: port, DatabaseName: databaseName, AuthSource: cs.AuthSource, TLSMode: "disabled"}
	if cs.SSL {
		identity.TLSMode = "enabled"
	}
	canonical, err := identity.Canonical()
	if err != nil {
		return service.ConfiguredDatabaseIdentity{}, configError("CONFIG_MONGO_INVALID", "MONGODB_URL")
	}
	return canonical, nil
}

func splitMongoHost(host string) (string, string, error) {
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		if len(parts) == 2 && parts[1] != "" {
			return strings.Trim(parts[0], "[]"), parts[1], nil
		}
	}
	return host, "27017", nil
}
