package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleLLMStats(w http.ResponseWriter, r *http.Request) {
	if s.claude == nil || s.claude.Stats == nil {
		jsonError(w, "llm stats unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model": s.claude.Model(),
		"stats": s.claude.Stats.Snapshot(),
	})
}
