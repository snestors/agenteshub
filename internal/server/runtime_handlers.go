package server

import (
	"net/http"
	"strconv"
)

func (s *Server) handleAgentRuntime(w http.ResponseWriter, r *http.Request) {
	engine, model := s.currentEngineModel(r.Context())
	key := mainAgentSessionName(engine, model)
	run, err := s.repos.ConversationRuns.Get(r.Context(), "main", key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if run == nil {
		writeJSON(w, http.StatusOK, map[string]any{"run": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": conversationRunToWire(run)})
}

func (s *Server) handleProjectSessionRuntime(w http.ResponseWriter, r *http.Request) {
	_, sess, ok := s.projectSessionFromRequest(w, r)
	if !ok {
		return
	}
	run, err := s.repos.ConversationRuns.Get(r.Context(), "project", strconv.FormatInt(sess.ID, 10))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if run == nil {
		writeJSON(w, http.StatusOK, map[string]any{"run": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": conversationRunToWire(run)})
}
