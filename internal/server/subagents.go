package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/agenteshub/agenteshub/internal/store"
)

// subagentWire is the JSON shape consumed by the frontend.
type subagentWire struct {
	ID                  int64  `json:"id"`
	ParentSessionID     string `json:"parent_session_id"`
	ParentScope         string `json:"parent_scope"`
	ParentTopicID       *int64 `json:"parent_topic_id,omitempty"`
	ParentProjectSessID *int64 `json:"parent_project_session_id,omitempty"`
	AgentType           string `json:"agent_type,omitempty"`
	Description         string `json:"description,omitempty"`
	Prompt              string `json:"prompt,omitempty"`
	Result              string `json:"result,omitempty"`
	Status              string `json:"status"`
	StartedAt           int64  `json:"started_at"`
	FinishedAt          *int64 `json:"finished_at,omitempty"`
	CostTokens          int64  `json:"cost_tokens"`
	ToolsUsed           string `json:"tools_used,omitempty"`
	WorktreePath        string `json:"worktree_path,omitempty"`
}

func subagentToWire(s store.SubagentRun) subagentWire {
	w := subagentWire{
		ID:              s.ID,
		ParentSessionID: s.ParentSessionID,
		ParentScope:     s.ParentScope,
		Status:          s.Status,
		StartedAt:       s.StartedAt,
		CostTokens:      s.CostTokens,
	}
	if s.ParentTopicID.Valid {
		v := s.ParentTopicID.Int64
		w.ParentTopicID = &v
	}
	if s.ParentProjectSessID.Valid {
		v := s.ParentProjectSessID.Int64
		w.ParentProjectSessID = &v
	}
	if s.AgentType.Valid {
		w.AgentType = s.AgentType.String
	}
	if s.Description.Valid {
		w.Description = s.Description.String
	}
	if s.Prompt.Valid {
		w.Prompt = s.Prompt.String
	}
	if s.Result.Valid {
		w.Result = s.Result.String
	}
	if s.FinishedAt.Valid {
		v := s.FinishedAt.Int64
		w.FinishedAt = &v
	}
	if s.ToolsUsed.Valid {
		w.ToolsUsed = s.ToolsUsed.String
	}
	if s.WorktreePath.Valid {
		w.WorktreePath = s.WorktreePath.String
	}
	return w
}

func (s *Server) handleSubagentsList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	rows, err := s.repos.Subagents.Recent(r.Context(), status, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]subagentWire, 0, len(rows))
	for _, x := range rows {
		out = append(out, subagentToWire(x))
	}
	writeJSON(w, http.StatusOK, map[string]any{"subagents": out})
}

func (s *Server) handleSubagentGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad subagent id", http.StatusBadRequest)
		return
	}
	// Cheap: fetch a recent window and look up. Adding a dedicated GetByID is
	// trivial but not needed for v1 traffic.
	rows, err := s.repos.Subagents.Recent(r.Context(), "", 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, x := range rows {
		if x.ID == id {
			writeJSON(w, http.StatusOK, subagentToWire(x))
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}
