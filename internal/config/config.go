package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

	// Pathstore connection
	PathstoreURL    string
	PathstoreAPIKey string

	// Auth
	DocgestAPIKey string

	// Claude extraction
	AnthropicAPIKey string
	AnthropicModel  string

	// Worker pool
	WorkerCount          int
	MaxQueueSize         int
	MaxConcurrentExtract int
	MaxConcurrentStore   int

	// Upload limits
	MaxUploadBytes int64

	// Chunking defaults
	DefaultChunkSize    int
	DefaultChunkOverlap int

	// Job state
	JobTTL time.Duration

	// PDF
	PDFFallbackPdftotext bool
}

func Load() Config {
	cfg := Config{
		Port: envOr("PORT", "8090"),

		PathstoreURL:    envOr("PATHSTORE_URL", "http://localhost:8080"),
		PathstoreAPIKey: os.Getenv("PATHSTORE_API_KEY"),

		DocgestAPIKey: os.Getenv("DOCGEST_API_KEY"),

		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:  envOr("ANTHROPIC_MODEL", "claude-sonnet-4-5-20250929"),

		WorkerCount:          envInt("WORKER_COUNT", 4),
		MaxQueueSize:         envInt("MAX_QUEUE_SIZE", 100),
		MaxConcurrentExtract: envInt("MAX_CONCURRENT_EXTRACT", 5),
		MaxConcurrentStore:   envInt("MAX_CONCURRENT_STORE", 10),

		MaxUploadBytes: envInt64("MAX_UPLOAD_BYTES", 52428800), // 50MB

		DefaultChunkSize:    envInt("DEFAULT_CHUNK_SIZE", 1500),
		DefaultChunkOverlap: envInt("DEFAULT_CHUNK_OVERLAP", 200),

		JobTTL: envDuration("JOB_TTL", 1*time.Hour),

		PDFFallbackPdftotext: envBool("PDF_FALLBACK_PDFTOTEXT", true),
	}

	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = 100
	}
	if cfg.MaxConcurrentExtract <= 0 {
		cfg.MaxConcurrentExtract = 5
	}
	if cfg.MaxConcurrentStore <= 0 {
		cfg.MaxConcurrentStore = 10
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = 52428800
	}
	if cfg.DefaultChunkSize <= 0 {
		cfg.DefaultChunkSize = 1500
	}
	if cfg.DefaultChunkOverlap <= 0 {
		cfg.DefaultChunkOverlap = 200
	}
	if cfg.JobTTL <= 0 {
		cfg.JobTTL = 1 * time.Hour
	}

	return cfg
}

func (c Config) Validate() error {
	if c.PathstoreAPIKey == "" {
		return fmt.Errorf("PATHSTORE_API_KEY is required")
	}
	if c.DocgestAPIKey == "" {
		return fmt.Errorf("DOCGEST_API_KEY is required")
	}
	if c.AnthropicAPIKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
