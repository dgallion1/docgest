package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dgallion1/docgest/internal/parser"
	"github.com/dgallion1/docgest/internal/pipeline"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	// Limit total request size.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+1024*1024) // extra 1MB for form overhead

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonError(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	userID := r.FormValue("user_id")
	if userID == "" {
		jsonError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := sanitizeFilename(header.Filename)
	if !parser.IsSupportedExtension(filename) {
		jsonError(w, fmt.Sprintf("unsupported file type: %s", filepath.Ext(filename)), http.StatusBadRequest)
		return
	}

	// Read file data.
	data, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxUploadBytes+1))
	if err != nil {
		jsonError(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	if int64(len(data)) > s.cfg.MaxUploadBytes {
		jsonError(w, fmt.Sprintf("file exceeds max size (%d bytes)", s.cfg.MaxUploadBytes), http.StatusRequestEntityTooLarge)
		return
	}

	docID := r.FormValue("doc_id")
	if docID == "" {
		docID = pipeline.ContentHashHex(data)[:16]
	}
	title := r.FormValue("title")

	// Parse optional chunk config overrides.
	chunkSize := s.cfg.DefaultChunkSize
	if v := r.FormValue("chunk_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			chunkSize = n
		}
	}
	chunkOverlap := s.cfg.DefaultChunkOverlap
	if v := r.FormValue("overlap"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			chunkOverlap = n
		}
	}

	force := r.FormValue("force") == "true"

	now := time.Now()
	job := &pipeline.Job{
		ID:        pipeline.ContentHashHex([]byte(fmt.Sprintf("%s-%s-%d", userID, filename, now.UnixNano())))[:20],
		DocID:     docID,
		UserID:    userID,
		Status:    pipeline.StatusQueued,
		Phase:     "queued",
		Filename:  filename,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	// Set internal fields via exported setter or direct. Since fileData is unexported,
	// we need a method on Job. Let's use the Submit method which passes data.

	_ = chunkSize
	_ = chunkOverlap
	_ = force

	// We need to set fileData on the job. Since it's unexported, add a setter.
	job.SetFileData(data)

	if err := s.orchestrator.Submit(job); err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"job_id":   job.ID,
		"doc_id":   job.DocID,
		"status":   job.Status,
		"poll_url": fmt.Sprintf("/api/ingest/%s/status", job.ID),
	})
}

func (s *Server) handleIngestStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job := s.orchestrator.GetJob(jobID)
	if job == nil {
		jsonError(w, "job not found", http.StatusNotFound)
		return
	}
	snap := job.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"job_id":   snap.ID,
		"doc_id":   snap.DocID,
		"status":   snap.Status,
		"phase":    snap.Phase,
		"progress": snap.Progress,
	})
}

func (s *Server) handleBatchIngest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes*10+10*1024*1024)

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		jsonError(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	userID := r.FormValue("user_id")
	if userID == "" {
		jsonError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "at least one file is required", http.StatusBadRequest)
		return
	}

	var results []map[string]any
	for _, fh := range files {
		filename := sanitizeFilename(fh.Filename)
		if !parser.IsSupportedExtension(filename) {
			results = append(results, map[string]any{
				"filename": filename,
				"error":    fmt.Sprintf("unsupported file type: %s", filepath.Ext(filename)),
			})
			continue
		}

		f, err := fh.Open()
		if err != nil {
			results = append(results, map[string]any{
				"filename": filename,
				"error":    "failed to open file",
			})
			continue
		}

		data, err := io.ReadAll(io.LimitReader(f, s.cfg.MaxUploadBytes+1))
		f.Close()
		if err != nil || int64(len(data)) > s.cfg.MaxUploadBytes {
			results = append(results, map[string]any{
				"filename": filename,
				"error":    "file too large or read error",
			})
			continue
		}

		now := time.Now()
		docID := pipeline.ContentHashHex(data)[:16]
		job := &pipeline.Job{
			ID:        pipeline.ContentHashHex([]byte(fmt.Sprintf("%s-%s-%d", userID, filename, now.UnixNano())))[:20],
			DocID:     docID,
			UserID:    userID,
			Status:    pipeline.StatusQueued,
			Phase:     "queued",
			Filename:  filename,
			CreatedAt: now,
			UpdatedAt: now,
		}
		job.SetFileData(data)

		if err := s.orchestrator.Submit(job); err != nil {
			results = append(results, map[string]any{
				"filename": filename,
				"error":    err.Error(),
			})
			continue
		}

		results = append(results, map[string]any{
			"filename": filename,
			"job_id":   job.ID,
			"doc_id":   job.DocID,
			"status":   job.Status,
			"poll_url": fmt.Sprintf("/api/ingest/%s/status", job.ID),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{"jobs": results})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func sanitizeFilename(name string) string {
	// Strip path components, keep only the base name.
	name = filepath.Base(name)
	// Remove any path separators that might have survived.
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	if name == "" || name == "." {
		name = "unnamed"
	}
	return name
}
