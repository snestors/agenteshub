// Package harness implements the BettaTech harness primitives shared by the
// HTTP server and the MCP server: parsing/writing feature_list.json, reading
// the canonical state files (current.md, history.md, CHECKPOINTS.md), and
// running init.sh in a project's repo.
//
// Both server and mcp depend on this package; this package depends on neither
// to keep the dependency graph simple.
package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FeatureListFile is the canonical filename inside a project's repo. The
// BettaTech harness expects this exact path; do not parameterize.
const FeatureListFile = "feature_list.json"

// validFeatureStatuses are the values accepted in FeatureItem.Status. Anything
// else makes ParseFeatureList reject the file with a clear error so a typo
// surfaces immediately instead of silently rendering the row as "unknown".
var validFeatureStatuses = map[string]struct{}{
	"pending":     {},
	"in_progress": {},
	"done":        {},
	"blocked":     {},
}

// FeatureItem is one entry in feature_list.json. Only ID/Name/Status are
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
// invariants (version >= 1, every feature has id/name/status, status is in
// the allowed set, no duplicate ids). Returns a wrapped error pointing at the
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

// WriteFeatureListAtomic encodes fl as pretty JSON and writes it to dst via
// a sibling tmp file + rename. The tmp file lives in the same directory so
// rename is a pure metadata op (no cross-device move). On failure, the tmp
// file is cleaned up; the original dst is left untouched.
func WriteFeatureListAtomic(dst string, fl FeatureList) error {
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
