// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed assets/index.html
var indexHTML string

// Server exposes the benchmark browser UI and JSON control API.
type Server struct {
	manager     *Manager
	openAPIPath string
}

// NewServer creates a benchmark HTTP server with a default OpenAPI spec path.
func NewServer(manager *Manager, openAPIPath string) *Server {
	return &Server{manager: manager, openAPIPath: openAPIPath}
}

// Router returns the HTTP routes for the benchmark UI and API.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/", s.handleIndex)
	r.Get("/favicon.ico", s.handleFavicon)
	r.Get("/api/templates", s.handleTemplates)
	r.Post("/api/runs", s.handleStartRun)
	r.Get("/api/runs", s.handleListRuns)
	r.Get("/api/runs/{id}", s.handleGetRun)
	r.Post("/api/runs/{id}/stop", s.handleStopRun)
	return r
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("spec")
	if path == "" {
		path = s.openAPIPath
	}
	if path == "" {
		writeError(w, http.StatusBadRequest, "BENCH-HTTP-TEMPLATESPEC: spec path is required")
		return
	}
	templates, err := LoadTemplatesFromFile(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	var cfg Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("BENCH-HTTP-DECODECONFIG: %v", err))
		return
	}
	run, err := s.manager.Start(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleListRuns(w http.ResponseWriter, _ *http.Request) {
	runs, err := s.manager.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.manager.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleStopRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.manager.Stop(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
