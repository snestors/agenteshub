package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenteshub/internal/store"
)

type skillImpWire struct {
	ID         int64  `json:"id"`
	SkillName  string `json:"skill_name"`
	Source     string `json:"source"`
	ProposedBy string `json:"proposed_by,omitempty"`
	Rationale  string `json:"rationale"`
	Patch      string `json:"patch"`
	PatchKind  string `json:"patch_kind"`
	Status     string `json:"status"`
	ResolvedBy string `json:"resolved_by,omitempty"`
	ResolvedAt *int64 `json:"resolved_at,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

func skillImpToWire(x store.SkillImprovement) skillImpWire {
	w := skillImpWire{
		ID:        x.ID,
		SkillName: x.SkillName,
		Source:    x.Source,
		Rationale: x.Rationale,
		Patch:     x.Patch,
		PatchKind: x.PatchKind,
		Status:    x.Status,
		CreatedAt: x.CreatedAt,
	}
	if x.ProposedBy.Valid {
		w.ProposedBy = x.ProposedBy.String
	}
	if x.ResolvedBy.Valid {
		w.ResolvedBy = x.ResolvedBy.String
	}
	if x.ResolvedAt.Valid {
		v := x.ResolvedAt.Int64
		w.ResolvedAt = &v
	}
	return w
}

func (s *Server) handleSkillImprovementsList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	rows, err := s.repos.SkillImprovements.List(r.Context(), status, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pending, _ := s.repos.SkillImprovements.CountPending(r.Context())
	out := make([]skillImpWire, 0, len(rows))
	for _, x := range rows {
		out = append(out, skillImpToWire(x))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"improvements":  out,
		"pending_count": pending,
	})
}

type skillImpCreateReq struct {
	SkillName  string `json:"skill_name"`
	Source     string `json:"source"`
	ProposedBy string `json:"proposed_by,omitempty"`
	Rationale  string `json:"rationale"`
	Patch      string `json:"patch"`
	PatchKind  string `json:"patch_kind,omitempty"`
}

func (s *Server) handleSkillImprovementsCreate(w http.ResponseWriter, r *http.Request) {
	var req skillImpCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SkillName) == "" || strings.TrimSpace(req.Patch) == "" || strings.TrimSpace(req.Rationale) == "" {
		http.Error(w, "skill_name, rationale and patch are required", http.StatusBadRequest)
		return
	}
	imp := store.SkillImprovement{
		SkillName: strings.TrimSpace(req.SkillName),
		Source:    strings.TrimSpace(req.Source),
		Rationale: strings.TrimSpace(req.Rationale),
		Patch:     req.Patch,
		PatchKind: strings.TrimSpace(req.PatchKind),
	}
	if imp.Source == "" {
		imp.Source = "local"
	}
	if proposed := strings.TrimSpace(req.ProposedBy); proposed != "" {
		imp.ProposedBy = sqlStr(proposed)
	}
	id, err := s.repos.SkillImprovements.Create(r.Context(), imp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "pending"})
}

type skillImpResolveReq struct {
	Status     string `json:"status"`
	ResolvedBy string `json:"resolved_by,omitempty"`
}

func (s *Server) handleSkillImprovementResolve(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var req skillImpResolveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Status != "approved" && req.Status != "rejected" {
		http.Error(w, "status must be 'approved' or 'rejected'", http.StatusBadRequest)
		return
	}
	resolvedBy := strings.TrimSpace(req.ResolvedBy)
	if resolvedBy == "" {
		resolvedBy = "user"
	}
	if err := s.repos.SkillImprovements.Resolve(r.Context(), id, req.Status, resolvedBy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": req.Status})
}
