package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplySpecDeltas_NoDeltasIsNoop(t *testing.T) {
	tmp := t.TempDir()
	deltaRoot := filepath.Join(tmp, "changes", "x", "specs")
	specsRoot := filepath.Join(tmp, "specs")
	if err := os.MkdirAll(specsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// deltaRoot does not exist → must succeed silently.
	if err := applySpecDeltas(deltaRoot, specsRoot, "x", time.Now()); err != nil {
		t.Fatalf("expected nil error when delta root missing, got %v", err)
	}

	// Spec dir should still be empty.
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty specs dir, got %d entries", len(entries))
	}
}

func TestApplySpecDeltas_FirstTimeCopiesDeltaVerbatim(t *testing.T) {
	tmp := t.TempDir()
	deltaRoot := filepath.Join(tmp, "delta")
	specsRoot := filepath.Join(tmp, "specs")
	deltaCap := filepath.Join(deltaRoot, "openspec-flow")
	if err := os.MkdirAll(deltaCap, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# capability: openspec-flow\n\noriginal contents\n"
	if err := os.WriteFile(filepath.Join(deltaCap, "spec.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applySpecDeltas(deltaRoot, specsRoot, "bootstrap", time.Now()); err != nil {
		t.Fatalf("applySpecDeltas: %v", err)
	}

	dst := filepath.Join(specsRoot, "openspec-flow", "spec.md")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != body {
		t.Fatalf("expected verbatim copy on first apply, got:\n%s", got)
	}
}

func TestApplySpecDeltas_SecondTimeAppendsWithHeader(t *testing.T) {
	tmp := t.TempDir()
	deltaRoot := filepath.Join(tmp, "delta")
	specsRoot := filepath.Join(tmp, "specs")
	dstCap := filepath.Join(specsRoot, "openspec-flow")
	if err := os.MkdirAll(dstCap, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "# capability: openspec-flow\n\noriginal contents\n"
	if err := os.WriteFile(filepath.Join(dstCap, "spec.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	deltaCap := filepath.Join(deltaRoot, "openspec-flow")
	if err := os.MkdirAll(deltaCap, 0o755); err != nil {
		t.Fatal(err)
	}
	delta := "## new section\n\nadded by change\n"
	if err := os.WriteFile(filepath.Join(deltaCap, "spec.md"), []byte(delta), 0o644); err != nil {
		t.Fatal(err)
	}

	when := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	if err := applySpecDeltas(deltaRoot, specsRoot, "archive-merges-deltas", when); err != nil {
		t.Fatalf("applySpecDeltas: %v", err)
	}

	merged, err := os.ReadFile(filepath.Join(dstCap, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(merged)

	if !strings.HasPrefix(got, original) {
		t.Fatalf("merged file must start with original content; got:\n%s", got)
	}
	if !strings.Contains(got, "## Delta from change: `archive-merges-deltas` (archived 2026-05-01)") {
		t.Fatalf("merged file missing expected delta header; got:\n%s", got)
	}
	if !strings.Contains(got, "added by change") {
		t.Fatalf("merged file missing delta body; got:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		tail := got
		if len(tail) > 20 {
			tail = tail[len(tail)-20:]
		}
		t.Fatalf("merged file must end with newline; got tail %q", tail)
	}
}

func TestApplySpecDeltas_NewCapabilityAlongsideExisting(t *testing.T) {
	tmp := t.TempDir()
	deltaRoot := filepath.Join(tmp, "delta")
	specsRoot := filepath.Join(tmp, "specs")
	if err := os.MkdirAll(filepath.Join(specsRoot, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specsRoot, "old", "spec.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	newCap := filepath.Join(deltaRoot, "brand-new")
	if err := os.MkdirAll(newCap, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newCap, "spec.md"), []byte("# brand-new capability\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applySpecDeltas(deltaRoot, specsRoot, "introduce-brand-new", time.Now()); err != nil {
		t.Fatalf("applySpecDeltas: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(specsRoot, "old", "spec.md")); string(got) != "old\n" {
		t.Fatalf("existing untouched capability must remain intact, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(specsRoot, "brand-new", "spec.md")); err != nil {
		t.Fatalf("brand-new capability not created: %v", err)
	}
}

func TestArchiveChange_MovesAndMergesAtomically(t *testing.T) {
	tmp := t.TempDir()
	if err := EnsureOpenSpecLayout(tmp); err != nil {
		t.Fatal(err)
	}
	changeName := "demo-change"
	changeDir := ChangeDir(tmp, changeName)
	if err := os.MkdirAll(filepath.Join(changeDir, "specs", "openspec-flow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "specs", "openspec-flow", "spec.md"), []byte("delta body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ArchiveChange(tmp, changeName); err != nil {
		t.Fatalf("ArchiveChange: %v", err)
	}

	// Change folder is gone, archive folder exists.
	if _, err := os.Stat(changeDir); !os.IsNotExist(err) {
		t.Fatalf("expected change dir to be moved, stat err=%v", err)
	}
	archiveDir := filepath.Join(tmp, "openspec", "archive", changeName)
	if _, err := os.Stat(archiveDir); err != nil {
		t.Fatalf("expected archive dir to exist: %v", err)
	}
	// Spec was created in specs/.
	merged, err := os.ReadFile(filepath.Join(tmp, "openspec", "specs", "openspec-flow", "spec.md"))
	if err != nil {
		t.Fatalf("read merged spec: %v", err)
	}
	if string(merged) != "delta body\n" {
		t.Fatalf("first archive should copy delta verbatim, got %q", merged)
	}
}

