package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/dgallion1/docgest/internal/pathstore"
	"github.com/go-chi/chi/v5"
)

// handleListDocuments lists all documents for a user.
func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		jsonError(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}

	prefix := fmt.Sprintf("memory/users/%s/documents", userID)
	children, err := s.orchestrator.PathstoreClient().ListChildren(r.Context(), prefix, 200)
	if err != nil {
		jsonError(w, "failed to list documents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter to only meta nodes.
	var docs []map[string]any
	for _, child := range children {
		if strings.HasSuffix(child.Key, ".meta") || strings.Contains(child.Key, ".meta") {
			docs = append(docs, map[string]any{
				"key":   child.Key,
				"value": child.Value,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"documents": docs})
}

// handleDeleteDocument deletes a document and all its stored facts.
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "docID")
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		jsonError(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	ps := s.orchestrator.PathstoreClient()
	docPrefix := fmt.Sprintf("memory/users/%s/documents/%s", userID, docID)

	// 1. Read manifest entries.
	manifestPrefix := docPrefix + "/facts"
	manifestEntries, err := ps.ListChildren(ctx, manifestPrefix, 10000)
	if err != nil {
		jsonError(w, "failed to read manifest: "+err.Error(), http.StatusInternalServerError)
		return
	}

	factsDeleted := 0
	missingPaths := 0

	// 2. Delete each referenced fact.
	for _, entry := range manifestEntries {
		factPath := extractFactPath(entry.Value)
		if factPath == "" {
			continue
		}
		err := ps.DeleteNode(ctx, factPath, false)
		if err != nil {
			missingPaths++
		} else {
			factsDeleted++
		}
	}

	// 3. Delete document meta and manifest.
	manifestDeleted := 0
	if err := ps.DeleteNode(ctx, docPrefix, true); err == nil {
		manifestDeleted = 1
	}

	// 4. Delete hash index entry.
	deleteHashIndex(ctx, ps, userID, docID, docPrefix)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"facts_deleted":      factsDeleted,
		"missing_fact_paths": missingPaths,
		"manifest_deleted":   manifestDeleted,
	})
}

func extractFactPath(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	path, _ := m["path"].(string)
	return path
}

func deleteHashIndex(ctx context.Context, ps *pathstore.Client, userID, docID, docPrefix string) {
	// Read the meta to get the content hash.
	meta, err := ps.GetNode(ctx, docPrefix+"/meta")
	if err != nil || meta == nil {
		return
	}
	metaMap, ok := meta.Value.(map[string]any)
	if !ok {
		return
	}
	hash, _ := metaMap["content_hash"].(string)
	if hash == "" {
		return
	}
	hashPath := fmt.Sprintf("memory/users/%s/documents/by_hash/%s/%s", userID, hash, docID)
	ps.DeleteNode(ctx, hashPath, false)
}
