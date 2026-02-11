package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgallion1/docgest/internal/api"
	"github.com/dgallion1/docgest/internal/config"
	"github.com/dgallion1/docgest/internal/extract"
	"github.com/dgallion1/docgest/internal/pathstore"
	"github.com/dgallion1/docgest/internal/pipeline"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize clients.
	ps := pathstore.NewClient(cfg.PathstoreURL, cfg.PathstoreAPIKey)
	claude := extract.NewClaudeClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)

	// Initialize pipeline.
	orch := pipeline.NewOrchestrator(cfg, claude, ps, log)
	orch.Start(ctx)

	// Initialize HTTP server.
	srv := api.NewServer(orch, claude, log, cfg)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down...")

		orch.Stop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)

		claude.Close()
		ps.Close()
	}()

	log.Info("starting docgest", "port", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}
