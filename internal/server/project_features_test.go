package server

import (
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
