package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFeatureList_Valid(t *testing.T) {
	raw := []byte(`{
		"version": 1,
		"updated_at": "2026-05-02T12:00:00Z",
		"features": [
			{"id": "f-001", "name": "First", "status": "pending"},
			{"id": "f-002", "name": "Second", "status": "in_progress", "depends_on": ["f-001"]},
			{"id": "f-003", "name": "Third", "status": "blocked", "blocked_reason": "waiting on API key"},
			{"id": "f-004", "name": "Fourth", "status": "done", "completed_at": "2026-05-02"}
		]
	}`)
	fl, err := ParseFeatureList(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fl.Version != 1 {
		t.Errorf("version = %d, want 1", fl.Version)
	}
	if len(fl.Features) != 4 {
		t.Fatalf("features len = %d, want 4", len(fl.Features))
	}
	if fl.Features[2].BlockedReason != "waiting on API key" {
		t.Errorf("blocked_reason not preserved")
	}
}

func TestParseFeatureList_EmptyFeaturesNormalized(t *testing.T) {
	raw := []byte(`{"version": 1}`)
	fl, err := ParseFeatureList(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fl.Features == nil {
		t.Errorf("features should be normalized to empty slice, got nil")
	}
}

func TestParseFeatureList_Errors(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantErrPart string
	}{
		{"garbage json", `not even json`, "parse feature_list.json"},
		{"missing version", `{"features": []}`, "version must be >= 1"},
		{"missing id", `{"version": 1, "features": [{"name": "x", "status": "pending"}]}`, "missing id"},
		{"missing name", `{"version": 1, "features": [{"id": "f-001", "status": "pending"}]}`, "missing name"},
		{"invalid status", `{"version": 1, "features": [{"id": "f-001", "name": "x", "status": "halfway"}]}`, `invalid status "halfway"`},
		{"duplicate id", `{"version": 1, "features": [{"id":"f-001","name":"a","status":"pending"},{"id":"f-001","name":"b","status":"done"}]}`, `duplicate id "f-001"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFeatureList([]byte(tc.raw))
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErrPart)
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErrPart)
			}
		})
	}
}

func TestWriteFeatureListAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "feature_list.json")

	fl := FeatureList{
		Version: 1,
		Features: []FeatureItem{
			{ID: "f-001", Name: "First", Status: "pending"},
		},
	}
	if err := WriteFeatureListAtomic(dst, fl); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := os.ReadFile(dst)
	parsed, err := ParseFeatureList(raw)
	if err != nil {
		t.Fatalf("parse round-trip: %v", err)
	}
	if len(parsed.Features) != 1 || parsed.Features[0].ID != "f-001" {
		t.Errorf("round-trip mismatch: %+v", parsed)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

func TestWriteFeatureListAtomic_Overwrite(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "feature_list.json")
	if err := os.WriteFile(dst, []byte(`{"version":1,"features":[{"id":"old","name":"old","status":"pending"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fl := FeatureList{Version: 1, Features: []FeatureItem{{ID: "new", Name: "new", Status: "done"}}}
	if err := WriteFeatureListAtomic(dst, fl); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(dst)
	parsed, _ := ParseFeatureList(raw)
	if parsed.Features[0].ID != "new" {
		t.Errorf("overwrite did not replace content")
	}
}
