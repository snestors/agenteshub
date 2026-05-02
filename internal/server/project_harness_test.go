package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHarnessFile_MissingReportsExistsFalse(t *testing.T) {
	dir := t.TempDir()
	out := readHarnessFile(filepath.Join(dir, "nope.md"), "nope.md", harnessFileMaxBytes)
	if out["exists"].(bool) {
		t.Errorf("missing file should report exists=false")
	}
	if out["content"].(string) != "" {
		t.Errorf("missing file should have empty content")
	}
	if out["size"].(int64) != 0 {
		t.Errorf("missing file size should be 0")
	}
}

func TestReadHarnessFile_SmallFileFullContent(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "current.md")
	body := "session start: x\n- did A\n- did B\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := readHarnessFile(full, "progress/current.md", harnessFileMaxBytes)
	if !out["exists"].(bool) {
		t.Errorf("file should exist")
	}
	if out["truncated"].(bool) {
		t.Errorf("small file should not be truncated")
	}
	if out["content"].(string) != body {
		t.Errorf("content mismatch: %q vs %q", out["content"], body)
	}
}

func TestReadHarnessFile_TruncationCap(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "history.md")
	body := strings.Repeat("a", 2000)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := readHarnessFile(full, "progress/history.md", 1000)
	if !out["truncated"].(bool) {
		t.Errorf("file larger than cap should be truncated")
	}
	if len(out["content"].(string)) != 1000 {
		t.Errorf("content len = %d, want 1000", len(out["content"].(string)))
	}
	if out["size"].(int64) != 2000 {
		t.Errorf("size = %d, want 2000 (real size, not truncated len)", out["size"])
	}
}
