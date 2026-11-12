// Package web serves the UI and the JSON/SSE API.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"seestaralign/internal/alpaca"
	"seestaralign/internal/session"
)

// dist is the built frontend (web/ -> `npm run build`). A .gitkeep keeps the
// directory present so the Go code compiles before the frontend is built.
//
//go:embed all:dist
var dist embed.FS

// frontend serves the built single-page app. Hashed files under /assets/ are
// cached for good; index.html always revalidates so a new build shows up.
func frontend(fsys fs.FS) http.Handler {
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "This binary has no web UI built in. Build it with `just build` (needs Node.js), or run `just dev`.", http.StatusServiceUnavailable)
		})
	}
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func Handler(s *session.Session) http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(dist, "dist")
	mux.Handle("GET /", frontend(sub))

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		b, err := s.Snapshot()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no")
		ch, unsub := s.Subscribe()
		defer unsub()
		send := func() bool {
			b, err := s.Snapshot()
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return false
			}
			fl.Flush()
			return true
		}
		if !send() {
			return
		}
		ping := time.NewTicker(15 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ch:
				// coalesce bursts of updates
				time.Sleep(80 * time.Millisecond)
				select {
				case <-ch:
				default:
				}
				if !send() {
					return
				}
			case <-ping.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
				fl.Flush()
			}
		}
	})

	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			fail(w, err)
			return
		}
		if err := s.UpdateSettings(b); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	action := func(f func(ctx context.Context) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if err := f(r.Context()); err != nil {
				fail(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
		}
	}
	mux.HandleFunc("POST /api/connect", action(func(ctx context.Context) error {
		// don't tie the connection to the HTTP request's lifetime
		return s.Connect(context.WithoutCancel(ctx))
	}))
	mux.HandleFunc("POST /api/disconnect", action(func(context.Context) error { s.Disconnect(); return nil }))
	mux.HandleFunc("POST /api/start", action(func(context.Context) error { return s.Start() }))
	mux.HandleFunc("POST /api/stop", action(func(context.Context) error { s.Stop(); return nil }))

	mux.HandleFunc("GET /api/discover", func(w http.ResponseWriter, r *http.Request) {
		found, err := alpaca.Discover("255.255.255.255", 1500*time.Millisecond)
		if err != nil {
			fail(w, err)
			return
		}
		if found == nil {
			found = []alpaca.Found{}
		}
		writeJSON(w, 200, found)
	})

	mux.HandleFunc("GET /api/frame.jpg", func(w http.ResponseWriter, r *http.Request) {
		b := s.FrameJPEG()
		if b == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})
	return mux
}
