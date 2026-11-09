// Command seestaralign serves a browser UI for three-point polar alignment of
// ZWO Seestar telescopes over ASCOM Alpaca.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"seestaralign/internal/session"
	"seestaralign/internal/web"
)

func defaultConfig() string {
	if p := os.Getenv("SEESTARALIGN_CONFIG"); p != "" {
		return p
	}
	if dir := os.Getenv("STATE_DIRECTORY"); dir != "" { // systemd StateDirectory=
		return filepath.Join(dir, "settings.json")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "seestaralign", "settings.json")
	}
	return "settings.json"
}

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	config := flag.String("config", defaultConfig(), "settings file")
	workdir := flag.String("workdir", filepath.Join(os.TempDir(), "seestaralign"), "scratch directory for frames")
	flag.Parse()

	s := session.New(*config, *workdir)
	srv := &http.Server{Addr: *listen, Handler: web.Handler(s), ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Print("shutting down")
		s.Stop() // never leave the mount moving
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	log.Printf("seestaralign listening on %s (settings: %s)", *listen, *config)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
