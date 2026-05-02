package server

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
	if len(fl.Features) != 0 {
		t.Errorf("features len = %d, want 0", len(fl.Features))
	}
}

func TestParseFeatureList_Errors(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantErrPart string
	}{
		{
			name:        "garbage json",
			raw:         `not even json`,
			wantErrPart: "parse feature_list.json",
		},
		{
			name:        "missing version",
			raw:         `{"features": []}`,
			wantErrPart: "version must be >= 1",
		},
		{
			name:        "missing id",
			raw:         `{"version": 1, "features": [{"name": "x", "status": "pending"}]}`,
			wantErrPart: "missing id",
		},
		{
			name:        "missing name",
			raw:         `{"version": 1, "features": [{"id": "f-001", "status": "pending"}]}`,
			wantErrPart: "missing name",
		},
		{
			name:        "invalid status",
			raw:         `{"version": 1, "features": [{"id": "f-001", "name": "x", "status": "halfway"}]}`,
			wantErrPart: `invalid status "halfway"`,
		},
		{
			name: "duplicate id",
			raw: `{"version": 1, "features": [
				{"id": "f-001", "name": "a", "status": "pending"},
				{"id": "f-001", "name": "b", "status": "done"}
			]}`,
			wantErrPart: `duplicate id "f-001"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFeatureList([]byte(tc.raw))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrPart)
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErrPart)
			}
		})
	}
}

func TestWriteFileAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "feature_list.json")

	fl := FeatureList{
		Version:   1,
		UpdatedAt: "2026-05-02T12:00:00Z",
		Features: []FeatureItem{
			{ID: "f-001", Name: "First", Status: "pending"},
		},
	}
	if err := writeFileAtomic(dst, fl); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	parsed, err := ParseFeatureList(raw)
	if err != nil {
		t.Fatalf("parse round-trip: %v", err)
	}
	if len(parsed.Features) != 1 || parsed.Features[0].ID != "f-001" {
		t.Errorf("round-trip mismatch: %+v", parsed)
	}

	// Atomic write should not leave .tmp siblings around on success.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

func TestWriteFileAtomic_OverwriteAtomic(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "feature_list.json")

	// Pre-existing content.
	if err := os.WriteFile(dst, []byte(`{"version":1,"features":[{"id":"old","name":"old","status":"pending"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fl := FeatureList{Version: 1, Features: []FeatureItem{{ID: "new", Name: "new", Status: "done"}}}
	if err := writeFileAtomic(dst, fl); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	raw, _ := os.ReadFile(dst)
	parsed, err := ParseFeatureList(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Features) != 1 || parsed.Features[0].ID != "new" {
		t.Errorf("overwrite did not replace content: %+v", parsed)
	}
}
