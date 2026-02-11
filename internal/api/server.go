package api

import (
	"log/slog"
	"net/http"

	"github.com/dgallion1/docgest/internal/config"
	"github.com/dgallion1/docgest/internal/extract"
	"github.com/dgallion1/docgest/internal/pipeline"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the HTTP API server for docgest.
type Server struct {
	router       chi.Router
	orchestrator *pipeline.Orchestrator
	claude       *extract.ClaudeClient
	log          *slog.Logger
	cfg          config.Config
}

// NewServer creates and configures the HTTP server.
func NewServer(orch *pipeline.Orchestrator, claude *extract.ClaudeClient, log *slog.Logger, cfg config.Config) *Server {
	s := &Server{
		orchestrator: orch,
		claude:       claude,
		log:          log,
		cfg:          cfg,
	}
	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(RequestLogger(s.log))

	// Public endpoints.
	r.Get("/health", s.handleHealth)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(s.cfg.DocgestAPIKey, s.log))

		r.Post("/api/ingest", s.handleIngest)
		r.Get("/api/ingest/{jobID}/status", s.handleIngestStatus)
		r.Post("/api/ingest/batch", s.handleBatchIngest)
		r.Get("/api/stats/llm", s.handleLLMStats)

		r.Get("/api/documents", s.handleListDocuments)
		r.Delete("/api/documents/{docID}", s.handleDeleteDocument)
	})

	s.router = r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
