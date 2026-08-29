// Command leanote is the plain Go entry point that replaces the Revel CLI
// run/package flow. It loads conf/app.conf, validates the prod secret and
// serves until SIGTERM/SIGINT or a shutdown timeout.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yangphere/leanote/app/controllers"
	api "github.com/yangphere/leanote/app/controllers/api"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/httpserver"
	"github.com/yangphere/leanote/app/lea/i18n"
	"github.com/yangphere/leanote/app/service"
)

// defaultPublicSecret mirrors the app.secret shipped in
// conf/app.conf-default; keep the two in sync. prod must never start with
// this value or an empty secret.
const defaultPublicSecret = "V85ZzBeTnzpsHyjQX4zukbQ8qqtju9y2aDM55VWxAH9Qop19poekx3xkcDVvrD0y"

func main() {
	confPath := flag.String("conf", "conf/app.conf", "path to app.conf")
	runMode := flag.String("runMode", "dev", "active app.conf section ([dev]/[prod]/[test])")
	port := flag.Int("port", 0, "override http.port from app.conf")
	flag.Parse()

	cfg, err := httpserver.LoadConfigFile(*confPath, *runMode)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	secret, _ := cfg.String("app.secret")
	if err := validateProdSecret(*runMode, secret); err != nil {
		log.Fatalf("app.conf: %v", err)
	}
	appBase := applicationBase(*confPath)
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

	initDatabase(cfg)

	// Wire the first-party stack: conf/routes table + registered actions +
	// sessions + static file roots (module.static equivalents). db must be
	// initialised before serving; run-mode is injected into controllers.
	confDir := filepath.Dir(*confPath)
	routesData, err := os.ReadFile(filepath.Join(confDir, "routes"))
	if err != nil {
		log.Fatalf("load routes: %v", err)
	}
	routes, err := httpserver.ParseRoutes(routesData)
	if err != nil {
		log.Fatalf("parse routes: %v", err)
	}
	service.InitService()
	controllers.InitService()
	api.InitService()
	registry := httpserver.NewRegistry()
	controllers.RegisterHTTP(registry, *runMode)
	api.RegisterHTTP(registry, *runMode)

	app := &httpserver.App{
		Routes:        httpserver.CompileRoutes(routes),
		Registry:      registry,
		Sessions:      httpserver.NewSessionCodec(cfg),
		StaticHandler: func(base string) http.Handler { return http.FileServer(http.Dir(base)) },
		OnRequest:     db.CheckMongoSessionLost,
	}

	log.Printf("leanote starting: addr=%s runMode=%s shutdownTimeout=%s", addr, *runMode, shutdownTimeout)
	srv := httpserver.NewServer(addr, app, shutdownTimeout)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	if err := srv.Run(signals, nil); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Printf("leanote stopped cleanly")
}

func applicationBase(confPath string) string {
	confDir := filepath.Dir(filepath.Clean(confPath))
	if strings.EqualFold(filepath.Base(confDir), "conf") {
		return filepath.Dir(confDir)
	}
	return confDir
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

// validateProdSecret rejects an empty or still-public app.secret before a
// prod server starts, with an actionable message.
func validateProdSecret(runMode, secret string) error {
	if runMode != "prod" {
		return nil
	}
	if secret == "" {
		return fmt.Errorf("app.secret is empty; set a real secret in app.conf before running in prod")
	}
	if secret == defaultPublicSecret {
		return fmt.Errorf("app.secret is still the public repository default; set a real secret in app.conf before running in prod")
	}
	return nil
}

// initDatabase mirrors db.Init's URL derivation using the first-party
// config (db.url → db.urlEnv → db.host/port/user/pass), so the plain-Go
// process never needs revel.Config. db timeouts fall back to their
// defaults until the db config seam wires the new keys (Task 6).
func initDatabase(cfg *httpserver.Config) {
	url := cfg.StringDefault("db.url", "")
	if url == "" {
		url = cfg.StringDefault("db.urlEnv", "")
	}
	dbname := cfg.StringDefault("db.dbname", "leanote")
	if url == "" {
		host := cfg.StringDefault("db.host", "127.0.0.1")
		port := cfg.StringDefault("db.port", "27017")
		user := cfg.StringDefault("db.username", "")
		pass := cfg.StringDefault("db.password", "")
		userPass := ""
		if user != "" && pass != "" {
			userPass = user + ":" + pass + "@"
		}
		url = "mongodb://" + userPass + host + ":" + port + "/" + dbname
	}
	db.Init(url, dbname)
}
