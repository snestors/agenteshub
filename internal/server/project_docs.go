package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// canonicalDocs lists every markdown file we expose under
// GET /api/projects/{id}/docs/{doc}. The map keys are the URL slugs;
// values are the relative paths inside the project.
var canonicalDocs = map[string]string{
	"claude":  ".claude/CLAUDE.md",
	"specs":   "SPECS.md",
	"design":  "DESIGN.md",
	"agents":  "AGENTS.md",
	"release": "RELEASE_NOTES.md",
}

// handleProjectDocGet returns the contents of one canonical project doc.
// Doc slug ∈ keys of canonicalDocs. Empty file → returns content "" not 404.
func (s *Server) handleProjectDocGet(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	slug := chi.URLParam(r, "doc")
	rel, ok := canonicalDocs[slug]
	if !ok {
		http.Error(w, "unknown doc", http.StatusBadRequest)
		return
	}
	full := filepath.Join(project.Path, rel)
	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{
				"doc":     slug,
				"path":    rel,
				"content": "",
				"exists":  false,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"doc":     slug,
		"path":    rel,
		"content": string(raw),
		"exists":  true,
	})
}

// handleProjectDocsList returns the list of available doc slugs (so the UI
// can render tabs without hardcoding).
func (s *Server) handleProjectDocsList(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	out := make([]map[string]any, 0, len(canonicalDocs))
	for slug, rel := range canonicalDocs {
		full := filepath.Join(project.Path, rel)
		_, err := os.Stat(full)
		out = append(out, map[string]any{
			"doc":    slug,
			"path":   rel,
			"exists": err == nil,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"docs": out})
}
