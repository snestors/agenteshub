package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// featureListFile is the canonical filename inside a project's repo. The
// BettaTech harness expects this exact path; do not parameterize.
const featureListFile = "feature_list.json"

// validFeatureStatuses are the values accepted in FeatureItem.Status. Anything
// else makes ParseFeatureList reject the file with a clear error so a typo
// surfaces immediately instead of silently rendering the row as "unknown".
var validFeatureStatuses = map[string]struct{}{
	"pending":     {},
	"in_progress": {},
	"done":        {},
	"blocked":     {},
}

// FeatureItem is one entry in feature_list.json. Only Id/Name/Status are
// required — the rest is optional metadata that the leader/implementer/reviewer
// can fill in as the feature progresses.
type FeatureItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Description   string   `json:"description,omitempty"`
	DependsOn     []string `json:"depends_on,omitempty"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	CompletedAt   string   `json:"completed_at,omitempty"`
}

// FeatureList is the top-level shape of feature_list.json.
type FeatureList struct {
	Version   int           `json:"version"`
	UpdatedAt string        `json:"updated_at,omitempty"`
	Features  []FeatureItem `json:"features"`
}

// ParseFeatureList decodes raw bytes into FeatureList and validates required
// invariants (version >= 1, every feature has id/name/status, status is in the
// allowed set, no duplicate ids). Returns a wrapped error pointing at the
// offending entry so the operator can fix it without grepping.
func ParseFeatureList(raw []byte) (FeatureList, error) {
	var fl FeatureList
	if err := json.Unmarshal(raw, &fl); err != nil {
		return FeatureList{}, fmt.Errorf("parse feature_list.json: %w", err)
	}
	if fl.Version < 1 {
		return FeatureList{}, errors.New("feature_list.json: version must be >= 1")
	}
	seen := map[string]struct{}{}
	for i, f := range fl.Features {
		if f.ID == "" {
			return FeatureList{}, fmt.Errorf("feature_list.json: features[%d] missing id", i)
		}
		if _, dup := seen[f.ID]; dup {
			return FeatureList{}, fmt.Errorf("feature_list.json: duplicate id %q", f.ID)
		}
		seen[f.ID] = struct{}{}
		if f.Name == "" {
			return FeatureList{}, fmt.Errorf("feature_list.json: %q missing name", f.ID)
		}
		if _, ok := validFeatureStatuses[f.Status]; !ok {
			return FeatureList{}, fmt.Errorf("feature_list.json: %q has invalid status %q", f.ID, f.Status)
		}
	}
	if fl.Features == nil {
		fl.Features = []FeatureItem{}
	}
	return fl, nil
}

// handleProjectFeaturesGet returns the parsed feature_list.json for a project.
//
//	GET /api/projects/{id}/features
//
// Response shape:
//
//	{
//	  "exists":  true|false,                        // false when the file is missing
//	  "path":    "feature_list.json",
//	  "version": 1,
//	  "updated_at": "...",
//	  "features": [...]
//	}
//
// A missing file is NOT an error — it's the expected state of a project that
// hasn't been scaffolded with the harness yet, so the UI can render an empty
// state with a "scaffold harness" CTA. A file that exists but is invalid JSON
// or fails validation returns 502 with the validator error so the operator
// can fix it without guessing.
func (s *Server) handleProjectFeaturesGet(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	full := filepath.Join(project.Path, featureListFile)

	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{
				"exists":   false,
				"path":     featureListFile,
				"version":  0,
				"features": []FeatureItem{},
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fl, err := ParseFeatureList(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":     true,
		"path":       featureListFile,
		"version":    fl.Version,
		"updated_at": fl.UpdatedAt,
		"features":   fl.Features,
	})
}

// handleProjectFeaturesPut overwrites feature_list.json with the validated
// payload. Atomic via tmp + rename so a crash mid-write can't corrupt the file.
//
//	PUT /api/projects/{id}/features
//	Content-Type: application/json
//	{ "version": 1, "features": [...] }
//
// updated_at is rewritten server-side regardless of what the client sent —
// the field is meant to be a "last server write" stamp, not a user-editable
// metadata field. Validation is the same as the GET parser; an invalid payload
// returns 400 with the validator error.
//
// The response shape mirrors GET so the UI can replace its cached copy with
// the response without an extra round-trip.
func (s *Server) handleProjectFeaturesPut(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}

	raw, err := readLimited(r, 512*1024) // 512 KiB cap — feature_list shouldn't get bigger
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	fl, err := ParseFeatureList(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fl.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	full := filepath.Join(project.Path, featureListFile)
	if err := writeFileAtomic(full, fl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":     true,
		"path":       featureListFile,
		"version":    fl.Version,
		"updated_at": fl.UpdatedAt,
		"features":   fl.Features,
	})
}

// readLimited reads up to maxBytes from r.Body and returns an error if the body
// exceeds the cap. The error message is generic on purpose — the caller maps
// it to 413 Request Entity Too Large.
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

// writeFileAtomic encodes fl as pretty JSON and writes it to dst via a sibling
// tmp file + rename. The tmp file lives in the same directory so rename is a
// pure metadata op (no cross-device move). On failure, the tmp file is cleaned
// up; the original dst is left untouched.
func writeFileAtomic(dst string, fl FeatureList) error {
	enc, err := json.MarshalIndent(fl, "", "  ")
	if err != nil {
		return fmt.Errorf("encode feature_list: %w", err)
	}
	enc = append(enc, '\n')

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".feature_list-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(enc); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("chmod tmp: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("rename tmp -> dst: %w", err)
	}
	cleanup = false
	return nil
}
