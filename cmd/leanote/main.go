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

	"github.com/revel/revel"
	"github.com/yangphere/leanote/app/controllers"
	api "github.com/yangphere/leanote/app/controllers/api"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/lea/i18n"
	"github.com/yangphere/leanote/app/service"
)

var initContentRuntime = service.InitContentRuntime

func main() {
	confPath := flag.String("conf", "", "path to the canonical production app.conf")
	runMode := flag.String("runMode", "", "active app.conf section (must be prod)")
	port := flag.Int("port", 0, "override http.port from app.conf")
	flag.Parse()
	if err := validateCLIOptions(*runMode, hasCLIFlag("runMode"), hasCLIFlag("conf")); err != nil {
		logConfigError(err)
	}

	cfg, err := httpserver.ValidateProductionConfig(*confPath)
	if err != nil {
		logConfigError(err)
	}
	appBase := applicationBase(*confPath)
	revel.BasePath = appBase
	if err := setupPresentation(
		cfg,
		filepath.Join(appBase, "app", "views"),
		filepath.Join(appBase, "messages"),
	); err != nil {
		log.Fatalf("load presentation assets: %v", err)
	}

	addr := fmt.Sprintf("%s:%d",
		cfg.StringDefault("http.addr", "0.0.0.0"),
		orInt(*port, cfg.IntDefault("http.port", 9000)))
	shutdownTimeout := httpserver.ShutdownTimeout(cfg)

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
	// sessions + static file roots (module.static equivalents). db must be
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
	if databaseReady && !service.ConfigS.InitGlobalConfigs() {
		log.Printf("global configuration unavailable; email delivery will retry through outbox")
	}
	if err := initializeContentRuntime(appBase); err != nil {
		log.Fatalf("initialize content runtime: %v", err)
	}
	controllers.InitService()
	api.InitService()
	registry := httpserver.NewRegistry()
	controllers.RegisterHTTP(registry, *runMode, cfg)
	api.RegisterHTTP(registry, *runMode)

	app := &httpserver.App{
		Routes:   httpserver.CompileRoutes(routes),
		Registry: registry,
		Sessions: httpserver.NewSessionCodec(cfg),
		StaticHandler: func(base string) http.Handler {
			return staticHandler(appBase, base)
		},
		OnRequest:   db.CheckMongoSessionLost,
		HealthCheck: db.Ping,
	}

	log.Printf("leanote starting: addr=%s runMode=%s shutdownTimeout=%s", addr, *runMode, shutdownTimeout)
	srv := httpserver.NewServer(addr, app, shutdownTimeout)
	var stopOutbox context.CancelFunc
	var outboxDone chan struct{}
	if databaseReady {
		worker := service.NewOutboxWorker(service.EmailS.DeliverOutbox)
		worker.SetErrorHandler(logOutboxDeliveryError)
		workerCtx, cancel := context.WithCancel(context.Background())
		stopOutbox = cancel
		outboxDone = make(chan struct{})
		go func() {
			defer close(outboxDone)
			worker.Run(workerCtx)
		}()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	runErr := srv.Run(signals, nil)
	if stopOutbox != nil {
		stopOutbox()
		select {
		case <-outboxDone:
		case <-time.After(shutdownTimeout):
			runErr = errors.Join(runErr, errors.New("outbox worker shutdown timed out"))
		}
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

func initializeContentRuntime(appBase string) error {
	return initContentRuntime(appBase)
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

func orInt(override, base int) int {
	if override != 0 {
		return override
	}
	return base
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
	if !hasRunMode || runMode != "prod" {
		return &httpserver.ConfigError{Code: "CONFIG_RUN_MODE_INVALID"}
	}
	if !hasConf {
		return &httpserver.ConfigError{Code: "CONFIG_PATH_INVALID", Key: "conf"}
	}
	return nil
}

func logConfigError(err error) {
	log.Printf("%v", err)
	os.Exit(78)
}

// initDatabase consumes the validated production placeholders. No alternate
// URL, host/port or database-name source is accepted here.
func initDatabase(cfg *httpserver.Config, runMode string) error {
	if runMode != "prod" {
		return &httpserver.ConfigError{Code: "CONFIG_RUN_MODE_INVALID"}
	}
	url, ok := cfg.String("db.urlEnv")
	if !ok || url == "" {
		return &httpserver.ConfigError{Code: "CONFIG_VALUE_MISSING", Key: "MONGODB_URL"}
	}
	dbname, ok := cfg.String("db.dbname")
	if !ok || dbname == "" {
		return &httpserver.ConfigError{Code: "CONFIG_KEY_INVALID", Key: "db.dbname"}
	}
	return db.InitWithError(url, dbname)
}
