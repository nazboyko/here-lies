// Package server serves the page and the two API routes.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/nazboyko/here-lies/internal/obit"
	"github.com/nazboyko/here-lies/internal/ollama"
	"github.com/nazboyko/here-lies/internal/photo"
)

const maxUpload = 25 << 20

// Messages for upload problems.
const (
	msgNoPhoto = "No photo arrived. Choose a photo and try again."
	msgTooBig  = "This photo is too large. Try a smaller picture."
	msgBusy    = "Another obituary is being written. Try again in a moment."
)

// Health reports whether the model is ready.
type Health interface {
	Health(ctx context.Context) ollama.Health
}

// Server holds what the routes need.
type Server struct {
	Pipeline *obit.Pipeline
	Health   Health
	Static   fs.FS // the built page; may be nil in tests
	Log      *log.Logger

	mu sync.Mutex // one obituary at a time: the model is local and serial
}

// Handler returns the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/obituary", s.obituary)
	if s.Static != nil {
		mux.Handle("GET /", staticHandler(s.Static))
	}
	return securityHeaders(mux)
}

func (s *Server) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log.Printf(format, args...)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Health.Health(r.Context()))
}

type event struct {
	Step   string       `json:"step,omitempty"`
	Result *obit.Result `json:"result,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// obituary takes a multipart "photo". With Accept: application/x-ndjson it
// streams one line per step and then the result; otherwise it answers with
// the result as plain JSON.
func (s *Server) obituary(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	file, _, err := r.FormFile("photo")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, event{Error: msgTooBig})
			return
		}
		writeJSON(w, http.StatusBadRequest, event{Error: msgNoPhoto})
		return
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, event{Error: msgTooBig})
		return
	}
	jpg, err := photo.Prepare(data)
	if err != nil {
		var p photo.Problem
		if errors.As(err, &p) {
			writeJSON(w, http.StatusUnsupportedMediaType, event{Error: string(p)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, event{Error: msgNoPhoto})
		return
	}

	if !s.mu.TryLock() {
		writeJSON(w, http.StatusTooManyRequests, event{Error: msgBusy})
		return
	}
	defer s.mu.Unlock()

	stream := strings.Contains(r.Header.Get("Accept"), "application/x-ndjson")
	var progress func(string)
	if stream {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		enc := json.NewEncoder(w)
		progress = func(step string) {
			enc.Encode(event{Step: step})
			if flusher != nil {
				flusher.Flush()
			}
		}
	}

	res, err := s.Pipeline.Run(r.Context(), jpg, progress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		s.logf("obituary failed: %v", err)
		msg := ollama.Explain(err)
		if stream {
			json.NewEncoder(w).Encode(event{Error: msg})
			return
		}
		writeJSON(w, statusFor(err), event{Error: msg})
		return
	}
	// The log carries kinds and timings only, never image data or text.
	s.logf("%s in %d ms (look %d, write %d)", res.Kind, res.Timings.TotalMS, res.Timings.LookMS, res.Timings.WriteMS)
	if stream {
		json.NewEncoder(w).Encode(event{Result: res})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func statusFor(err error) int {
	var missing ollama.MissingModel
	switch {
	case errors.As(err, &missing), errors.Is(err, ollama.ErrDown):
		return http.StatusServiceUnavailable
	case errors.Is(err, ollama.ErrSlow):
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// staticHandler serves the built page and falls back to index.html.
func staticHandler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(files, name); err != nil {
			if _, err := fs.Stat(files, "index.html"); err != nil {
				http.Error(w, "The page is not built yet. Run: make build", http.StatusNotFound)
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// securityHeaders keeps the page from loading anything from another host.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
