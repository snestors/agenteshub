package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/snestors/agenteshub/internal/harness"
)

// handleProjectFeaturesGet returns the parsed feature_list.json for a project.
//
//	GET /api/projects/{id}/features
//
// Missing file → 200 {exists: false, features: []}, so the UI can render an
// empty state with a "scaffold harness" CTA. Invalid JSON or schema → 502 with
// the validator error so the operator can fix it without guessing.
func (s *Server) handleProjectFeaturesGet(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	full := filepath.Join(project.Path, harness.FeatureListFile)

	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{
				"exists":   false,
				"path":     harness.FeatureListFile,
				"version":  0,
				"features": []harness.FeatureItem{},
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fl, err := harness.ParseFeatureList(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":     true,
		"path":       harness.FeatureListFile,
		"version":    fl.Version,
		"updated_at": fl.UpdatedAt,
		"features":   fl.Features,
	})
}

// handleProjectFeaturesPut overwrites feature_list.json with a validated
// payload. Atomic via tmp + rename so a mid-write crash can't corrupt the
// file. updated_at is rewritten server-side regardless of what the client
// sent — the field is meant to be a server "last write" stamp, not user
// metadata. Body is capped at 512 KiB.
func (s *Server) handleProjectFeaturesPut(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}

	raw, err := readLimited(r, 512*1024)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	fl, err := harness.ParseFeatureList(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fl.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	full := filepath.Join(project.Path, harness.FeatureListFile)
	if err := harness.WriteFeatureListAtomic(full, fl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":     true,
		"path":       harness.FeatureListFile,
		"version":    fl.Version,
		"updated_at": fl.UpdatedAt,
		"features":   fl.Features,
	})
}

// readLimited reads up to maxBytes from r.Body and returns an error if the
// body exceeds the cap.
func readLimited(r *http.Request, maxBytes int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	defer r.Body.Close()
	buf := make([]byte, 0, 8*1024)
	tmp := make([]byte, 4*1024)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "EOF" {
				return buf, nil
			}
			if err.Error() == "http: request body too large" {
				return nil, fmt.Errorf("body exceeds %d bytes", maxBytes)
			}
			return buf, nil
		}
	}
}
