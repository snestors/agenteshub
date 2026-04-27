package server

import (
	"encoding/json"
	"net/http"

	"github.com/agenteshub/agenteshub/internal/skillsync"
	"github.com/agenteshub/agenteshub/internal/store"
)

type skillWire struct {
	Name        string         `json:"name"`
	Source      string         `json:"source"`
	Description string         `json:"description,omitempty"`
	RoleHint    string         `json:"role_hint,omitempty"`
	Version     string         `json:"version,omitempty"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Body        string         `json:"body"`
	Path        string         `json:"path"`
	PulledAt    int64          `json:"pulled_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

func skillToWire(s store.Skill) skillWire {
	w := skillWire{
		Name:      s.Name,
		Source:    s.Source,
		Body:      s.Body,
		Path:      s.Path,
		PulledAt:  s.PulledAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.Description.Valid {
		w.Description = s.Description.String
	}
	if s.RoleHint.Valid {
		w.RoleHint = s.RoleHint.String
	}
	if s.Version.Valid {
		w.Version = s.Version.String
	}
	if s.Frontmatter.Valid && s.Frontmatter.String != "" {
		var fm map[string]any
		if err := json.Unmarshal([]byte(s.Frontmatter.String), &fm); err == nil {
			w.Frontmatter = fm
		}
	}
	return w
}

func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	rows, err := s.repos.Skills.List(r.Context(), source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]skillWire, 0, len(rows))
	for _, row := range rows {
		out = append(out, skillToWire(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": out})
}

// handleSkillsSync triggers a fresh scan of every configured source.
// For v1 only the local ~/.claude/skills/ root is scanned; remote
// registries are stubbed.
func (s *Server) handleSkillsSync(w http.ResponseWriter, r *http.Request) {
	syncer := skillsync.New(s.repos)
	res, err := syncer.Sync(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
