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
	"time"

	"seestaralign/internal/alpaca"
	"seestaralign/internal/session"
)

//go:embed static
var static embed.FS

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
	sub, _ := fs.Sub(static, "static")
	mux.Handle("GET /", http.FileServerFS(sub))

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

	mux.HandleFunc("POST /api/sim/knob", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Alt, Az float64 }
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
			fail(w, err)
			return
		}
		if err := s.SimKnob(req.Alt, req.Az); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

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
