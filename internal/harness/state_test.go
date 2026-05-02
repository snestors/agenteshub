package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadStateFile_Missing(t *testing.T) {
	dir := t.TempDir()
	out := ReadStateFile(filepath.Join(dir, "nope.md"), "nope.md", FileMaxBytes)
	if out.Exists {
		t.Errorf("missing file should report exists=false")
	}
	if out.Content != "" {
		t.Errorf("missing file should have empty content")
	}
	if out.Size != 0 {
		t.Errorf("missing file size should be 0")
	}
}

func TestReadStateFile_Small(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "current.md")
	body := "session start: x\n- did A\n- did B\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := ReadStateFile(full, "progress/current.md", FileMaxBytes)
	if !out.Exists || out.Truncated || out.Content != body {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestReadStateFile_Truncated(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "history.md")
	body := strings.Repeat("a", 2000)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := ReadStateFile(full, "progress/history.md", 1000)
	if !out.Truncated {
		t.Errorf("expected truncated=true")
	}
	if len(out.Content) != 1000 {
		t.Errorf("content len = %d, want 1000", len(out.Content))
	}
	if out.Size != 2000 {
		t.Errorf("size = %d, want 2000 (real size on disk)", out.Size)
	}
}

func TestReadAllState(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "progress"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "progress/current.md"), []byte("cur"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CHECKPOINTS.md"), []byte("checks"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := ReadAllState(dir, FileMaxBytes)
	if !out["current"].Exists {
		t.Errorf("current should exist")
	}
	if out["history"].Exists {
		t.Errorf("history should not exist (we didn't create it)")
	}
	if out["checkpoints"].Content != "checks" {
		t.Errorf("checkpoints content mismatch: %q", out["checkpoints"].Content)
	}
}
