// Command leanote is the plain Go production entry point. It requires the
// canonical production configuration interface before binding or dialing.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yangphere/leanote/app/controllers"
	adminControllers "github.com/yangphere/leanote/app/controllers/admin"
	api "github.com/yangphere/leanote/app/controllers/api"
	memberControllers "github.com/yangphere/leanote/app/controllers/member"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/lea/i18n"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"
)

var initContentRuntime = service.InitContentRuntime

func main() {
	confPath := flag.String("conf", "", "path to the canonical production app.conf")
	runMode := flag.String("runMode", "", "active app.conf section (dev, test, or prod)")
	flag.Parse()
	if err := validateCLIOptions(*runMode, hasCLIFlag("runMode"), hasCLIFlag("conf")); err != nil {
		logConfigError(err)
	}
	configPath, err := configPathForRunMode(*confPath, *runMode)
	if err != nil {
		logConfigError(err)
	}
	var cfg *httpserver.Config
	if *runMode == "prod" {
		cfg, err = httpserver.ValidateProductionConfig(configPath)
	} else {
		cfg, err = httpserver.LoadConfigFile(configPath, *runMode)
	}
	if err != nil {
		logConfigError(err)
	}
	appBase := applicationBase(configPath)
	var runtimeCfg *httpserver.ProductionConfig
	if *runMode == "prod" {
		runtimeCfg, err = httpserver.ValidateProductionRuntimeConfig(cfg, filepath.Join(appBase, "public"))
	} else {
		runtimeCfg, err = httpserver.ValidateLocalRuntimeConfig(cfg, *runMode, appBase, filepath.Join(appBase, "public"))
	}
	if err != nil {
		logConfigError(err)
	}
	// First-party application-config seam (adminUsername, site.url) from the
	// already-validated configuration, installed before the registry is
	// wired. The global configuration snapshot never reads framework globals.
	service.SetAppConfigSource(cfg)
	service.SetRuntimeConfig(service.RuntimeConfig{
		BackupRoot:       runtimeCfg.BackupRoot,
		DatabaseName:     runtimeCfg.DatabaseName,
		DatabaseIdentity: runtimeCfg.ConfiguredDatabaseIdentity,
		DatabaseProvider: databaseConnectionProvider(cfg, *runMode, runtimeCfg),
	})
	if err := setupPresentation(
		cfg,
		filepath.Join(appBase, "app", "views"),
		filepath.Join(appBase, "messages"),
	); err != nil {
		log.Fatalf("load presentation assets: %v", err)
	}
	service.SetThemeTemplateFuncs(httpserver.TemplateFuncs())

	addr := runtimeCfg.Addr
	shutdownTimeout := runtimeCfg.ShutdownTimeout
	if err := db.ConfigureTimeouts(cfg); err != nil {
		logConfigError(err)
	}

	databaseReady := true
	if err := initDatabase(cfg, *runMode); err != nil {
		databaseReady = false
		var configErr *httpserver.ConfigError
		if errors.As(err, &configErr) {
			logConfigError(err)
		}
		// A valid configuration with a temporarily unavailable MongoDB keeps
		// serving /healthz so orchestrators receive the required 503 response.
		log.Printf("mongo readiness unavailable; healthz will return not_ready")
	}

	// Wire the first-party stack: conf/routes table + registered actions +
	// sessions + static file roots. db must be
	// initialised before serving; run-mode is injected into controllers.
	// The production config is mounted outside the application tree. Resolve
	// routes from the packaged application root, alongside views/messages.
	routesData, err := os.ReadFile(filepath.Join(appBase, "conf", "routes"))
	if err != nil {
		log.Fatalf("load routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(routesData)
	if err != nil {
		log.Fatalf("parse routes: %v", err)
	}
	service.InitService()
	lea.InitEmail(cfg)
	lea.InitVd()
	// D-H9: the app is not ready until the global configuration snapshot has
	// loaded. Try once now when MongoDB is up (keeping the OnAppStart order
	// db → service → global config → api → registry); otherwise the
	// background loop below reconnects and reloads with backoff.
	readiness := newStartupReadiness(databaseReady,
		func() error { return initDatabase(cfg, *runMode) },
		service.ConfigS.InitGlobalConfigsWithError,
		log.Printf,
	)
	if databaseReady {
		if err := readiness.attempt(); err != nil {
			log.Printf("global configuration unavailable: %v; not ready, retrying in background", err)
		}
	}
	if err := initContentRuntime(runtimeCfg.ContentRoots); err != nil {
		log.Fatalf("initialize content runtime: %v", err)
	}
	controllers.InitService()
	api.InitService()
	memberControllers.InitService()
	adminControllers.InitService()
	registry := httpserver.NewRegistry()
	controllers.RegisterHTTP(registry, *runMode, cfg)
	memberControllers.RegisterHTTP(registry, controllers.WebSessionBefore)
	adminControllers.RegisterHTTP(registry, adminControllers.HTTPDeps{Production: runtimeCfg, SessionBefore: controllers.WebSessionBefore})
	api.RegisterHTTP(registry, *runMode)

	app := &httpserver.App{
		Routes:          httpserver.CompileRoutes(routes),
		Registry:        registry,
		Sessions:        httpserver.NewSessionCodec(cfg),
		PrincipalPolicy: api.PrincipalPolicyFromConfig(),
		SessionReaderFactory: func(session map[string]string) httpserver.SessionReader {
			return sessionReader{values: session}
		},
		SessionWriterFactory: func(ctx *httpserver.Context) httpserver.SessionWriter { return ctx },
		LocaleResolver:       httpserver.LocaleResolverFromConfig(cfg),
		StaticHandler: func(base string) http.Handler {
			return staticHandlerWithContent(appBase, base, runtimeCfg.ContentRoots.PublicUpload.Data)
		},
		OnRequest:   db.CheckMongoSessionLost,
		HealthCheck: db.Ping,
		Ready:       readiness.Ready,
	}

	log.Printf("leanote starting: addr=%s runMode=%s shutdownTimeout=%s", addr, *runMode, shutdownTimeout)
	srv := httpserver.NewServer(addr, app, shutdownTimeout)
	// Background work shares one shutdown context: readiness retries until
	// the snapshot loads, then the outbox worker runs. The outbox never
	// delivers on an empty snapshot.
	backgroundCtx, stopBackground := context.WithCancel(context.Background())
	backgroundDone := make(chan struct{})
	go func() {
		defer close(backgroundDone)
		if !readiness.run(backgroundCtx) {
			return
		}
		worker := service.NewOutboxWorker(service.EmailS.DeliverOutbox)
		worker.SetErrorHandler(logOutboxDeliveryError)
		worker.Run(backgroundCtx)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	runErr := srv.Run(signals, nil)
	stopBackground()
	select {
	case <-backgroundDone:
	case <-time.After(shutdownTimeout):
		runErr = errors.Join(runErr, errors.New("readiness/outbox worker shutdown timed out"))
	}
	if runErr != nil {
		log.Fatalf("shutdown: %v", runErr)
	}
	log.Printf("leanote stopped cleanly")
}

func logOutboxDeliveryError(error) {
	// SMTP responses can echo recipient data. The durable outbox retains the
	// diagnostic state; the process log emits only a stable, redacted signal.
	log.Printf("outbox delivery pending retry")
}

func applicationBase(confPath string) string {
	if filepath.Clean(confPath) == filepath.Clean(httpserver.CanonicalProductionConfigPath()) {
		if executable, err := os.Executable(); err == nil {
			return filepath.Dir(filepath.Dir(executable))
		}
	}
	confDir := filepath.Dir(filepath.Clean(confPath))
	if strings.EqualFold(filepath.Base(confDir), "conf") {
		return filepath.Dir(confDir)
	}
	return confDir
}

func staticAssetRoot(appBase, base string) string {
	return filepath.Join(appBase, base)
}

func staticHandler(appBase, base string) http.Handler {
	assetPath := staticAssetRoot(appBase, base)
	if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, assetPath)
		})
	}
	return http.FileServer(http.Dir(assetPath))
}

type sessionReader struct{ values map[string]string }

func (r sessionReader) Get(key string) (string, bool, error) {
	value, ok := r.values[key]
	return value, ok, nil
}

func staticHandlerWithContent(appBase, base, publicUploadRoot string) http.Handler {
	if base == "public/upload" {
		return http.FileServer(http.Dir(publicUploadRoot))
	}
	if base == "public" {
		assets := http.FileServer(http.Dir(filepath.Join(appBase, base)))
		uploads := http.StripPrefix("/upload", http.FileServer(http.Dir(publicUploadRoot)))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/upload/") || r.URL.Path == "/upload" {
				uploads.ServeHTTP(w, r)
				return
			}
			assets.ServeHTTP(w, r)
		})
	}
	return staticHandler(appBase, base)
}

// setupPresentation installs the first-party template renderer and loads the
// message catalog before any request can reach a controller. The paths are
// explicit so resource lookup follows the selected configuration root.
func setupPresentation(cfg *httpserver.Config, viewsDir, messagesDir string) error {
	templates, err := httpserver.LoadTemplates(viewsDir)
	if err != nil {
		return err
	}
	if err := i18n.LoadMessages(messagesDir); err != nil {
		return fmt.Errorf("load messages: %w", err)
	}
	i18n.DefaultLanguage = cfg.StringDefault("i18n.default_language", "en-us")
	httpserver.TemplateRenderer = httpserver.TemplateSetRenderer(templates)
	return nil
}

func hasCLIFlag(name string) bool {
	prefix := "-" + name
	for _, arg := range os.Args[1:] {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
}

func validateCLIOptions(runMode string, hasRunMode, hasConf bool) error {
	if !hasRunMode || (runMode != "prod" && runMode != "dev" && runMode != "test") {
		return &httpserver.ConfigError{Code: "CONFIG_RUN_MODE_INVALID"}
	}
	if runMode == "prod" && !hasConf {
		return &httpserver.ConfigError{Code: "CONFIG_PATH_INVALID", Key: "conf"}
	}
	return nil
}

func configPathForRunMode(confPath, runMode string) (string, error) {
	path := confPath
	if runMode != "prod" && path == "" {
		path = filepath.Join("conf", "app.conf")
	}
	if runMode != "prod" && filepath.Clean(path) == httpserver.CanonicalProductionConfigPath() {
		return "", &httpserver.ConfigError{Code: "CONFIG_PATH_INVALID", Key: "conf"}
	}
	return path, nil
}

func logConfigError(err error) {
	log.Printf("%v", err)
	os.Exit(78)
}

// initDatabase consumes the validated production placeholders. No alternate
// URL, host/port or database-name source is accepted here.
func initDatabase(cfg *httpserver.Config, runMode string) error {
	if cfg == nil || (runMode != "prod" && runMode != "dev" && runMode != "test") {
		return &httpserver.ConfigError{Code: "CONFIG_RUN_MODE_INVALID"}
	}
	url, err := databaseURL(cfg, runMode)
	if err != nil {
		return err
	}
	dbname, ok := cfg.String("db.dbname")
	if !ok || dbname == "" {
		return &httpserver.ConfigError{Code: "CONFIG_KEY_INVALID", Key: "db.dbname"}
	}
	return db.InitWithError(url, dbname)
}

func databaseURL(cfg *httpserver.Config, runMode string) (string, error) {
	if runMode == "prod" {
		value, ok := cfg.String("db.urlEnv")
		if !ok || strings.TrimSpace(value) == "" {
			return "", &httpserver.ConfigError{Code: "CONFIG_VALUE_MISSING", Key: "MONGODB_URL"}
		}
		return strings.TrimSpace(value), nil
	}
	for _, key := range []string{"db.url", "db.urlEnv"} {
		if value, ok := cfg.String(key); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	}
	dbname := cfg.StringDefault("db.dbname", "")
	if dbname == "" {
		return "", &httpserver.ConfigError{Code: "CONFIG_MONGO_INVALID", Key: "db.host"}
	}
	return httpserver.MongoURL(cfg, dbname)
}

func databaseConnectionProvider(cfg *httpserver.Config, runMode string, runtimeCfg *httpserver.ProductionConfig) service.DatabaseConnectionProvider {
	return service.DatabaseConnectionProviderFunc(func() (service.DatabaseConnection, error) {
		if runtimeCfg == nil {
			return service.DatabaseConnection{}, errors.New("database runtime configuration is unavailable")
		}
		raw, err := databaseURL(cfg, runMode)
		if err != nil {
			return service.DatabaseConnection{}, err
		}
		parsed, err := connstring.ParseAndValidate(raw)
		if err != nil {
			return service.DatabaseConnection{}, fmt.Errorf("parse database connection: %w", err)
		}
		identity := runtimeCfg.ConfiguredDatabaseIdentity
		return service.DatabaseConnection{
			Scheme:       identity.Scheme,
			Host:         identity.ClusterHost,
			Port:         identity.ClusterPort,
			DatabaseName: runtimeCfg.DatabaseName,
			Username:     parsed.Username,
			Password:     parsed.Password,
			AuthSource:   parsed.AuthSource,
			TLSMode:      identity.TLSMode,
		}, nil
	})
}
