// Command gitorgasm runs the API, worker, and migration roles from one image.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/upayanmazumder/gitorgasm/core/internal/obs"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gitorgasm <api|worker|migrate>")
		os.Exit(2)
	}
	role := os.Args[1]
	log := obs.NewLogger(role).With("app_version", version)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch role {
	case "api":
		err = runAPI(ctx, log)
	case "worker":
		err = runWorker(ctx, log)
	case "migrate":
		err = runMigrate(ctx, log)
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		log.Error("exiting with error", "error", err)
		os.Exit(1)
	}
}
