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
	"syscall"

	"github.com/yangphere/leanote/app/httpserver"
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

	addr := fmt.Sprintf("%s:%d",
		cfg.StringDefault("http.addr", "0.0.0.0"),
		orInt(*port, cfg.IntDefault("http.port", 9000)))
	shutdownTimeout := httpserver.ShutdownTimeout(cfg)

	log.Printf("leanote starting: addr=%s runMode=%s shutdownTimeout=%s", addr, *runMode, shutdownTimeout)
	srv := httpserver.NewServer(addr, http.NotFoundHandler(), shutdownTimeout)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	if err := srv.Run(signals, nil); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Printf("leanote stopped cleanly")
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
