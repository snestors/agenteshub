package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateVersion_NotEmpty(t *testing.T) {
	v := TemplateVersion()
	if v == "" || v == "unknown" {
		t.Errorf("TemplateVersion() = %q, want non-empty semver", v)
	}
	// loose semver shape check: at least two dots.
	if strings.Count(v, ".") < 2 {
		t.Errorf("TemplateVersion() = %q, want semver-like (X.Y.Z)", v)
	}
}

func TestTemplateFiles_DotRenamed(t *testing.T) {
	files, err := TemplateFiles()
	if err != nil {
		t.Fatalf("TemplateFiles: %v", err)
	}
	// Must include core entries with dot-rename applied.
	want := map[string]string{
		"AGENTS.md":                       OwnerTemplateSeed,
		"init.sh":                         OwnerTemplateManaged,
		"feature_list.json":               OwnerProject,
		"CHECKPOINTS.md":                  OwnerTemplateManaged,
		"progress/current.md":             OwnerProject,
		"progress/history.md":             OwnerProject,
		"docs/architecture.md":            OwnerTemplateSeed,
		"docs/conventions.md":             OwnerTemplateSeed,
		"docs/verification.md":            OwnerTemplateManaged,
		".claude/agents/leader.md":        OwnerTemplateManaged,
		".claude/agents/implementer.md":   OwnerTemplateManaged,
		".claude/agents/reviewer.md":      OwnerTemplateManaged,
		".claude/settings.json":           OwnerTemplateManaged,
	}
	got := map[string]string{}
	for _, e := range files {
		got[e.RelPath] = e.Owner
	}
	for path, owner := range want {
		if g, ok := got[path]; !ok {
			t.Errorf("missing template entry: %s", path)
		} else if g != owner {
			t.Errorf("owner of %s = %s, want %s", path, g, owner)
		}
	}
	// Ensure no "dot_" prefix leaked through.
	for _, e := range files {
		if strings.Contains(e.RelPath, "dot_") {
			t.Errorf("dot_ prefix not rewritten: %s", e.RelPath)
		}
	}
	// Ensure version manifest itself is NOT in the output.
	for _, e := range files {
		if e.RelPath == "template_version.txt" {
			t.Errorf("template_version.txt should be excluded from project files")
		}
	}
}

func TestTemplateFiles_InitShIsExecutable(t *testing.T) {
	files, _ := TemplateFiles()
	for _, e := range files {
		if e.RelPath == "init.sh" {
			if e.Mode != 0o755 {
				t.Errorf("init.sh mode = %o, want 0o755", e.Mode)
			}
			return
		}
	}
	t.Errorf("init.sh not in template")
}

func TestScaffold_FreshProject(t *testing.T) {
	dir := t.TempDir()
	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.AlreadyExisted {
		t.Errorf("fresh project should not report already_existed=true")
	}
	if len(res.Created) == 0 {
		t.Errorf("fresh project should create files")
	}
	if len(res.Skipped) != 0 {
		t.Errorf("fresh project should skip nothing, got %d", len(res.Skipped))
	}

	// Manifest landed.
	if _, err := os.Stat(filepath.Join(dir, ".harness/manifest.json")); err != nil {
		t.Errorf("manifest missing: %v", err)
	}

	// init.sh exists and is executable.
	info, err := os.Stat(filepath.Join(dir, "init.sh"))
	if err != nil {
		t.Errorf("init.sh missing: %v", err)
	} else if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("init.sh not executable: mode=%o", info.Mode().Perm())
	}

	// .claude/agents/leader.md ended up at the right path.
	if _, err := os.Stat(filepath.Join(dir, ".claude/agents/leader.md")); err != nil {
		t.Errorf(".claude/agents/leader.md missing: %v", err)
	}
}

func TestScaffold_ProjectOwnedNotOverwritten(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate a project-owned file with custom content.
	custom := `{"version":1,"features":[{"id":"x","name":"existing","status":"pending"}]}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "feature_list.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// feature_list.json must be in Skipped.
	skippedFL := false
	for _, s := range res.Skipped {
		if s.Path == "feature_list.json" && s.Owner == OwnerProject {
			skippedFL = true
		}
	}
	if !skippedFL {
		t.Errorf("feature_list.json should be skipped, got skipped=%+v", res.Skipped)
	}

	// And the disk content is unchanged.
	got, _ := os.ReadFile(filepath.Join(dir, "feature_list.json"))
	if string(got) != custom {
		t.Errorf("project-owned feature_list.json was overwritten: got %q", string(got))
	}
}

func TestScaffold_TemplateSeedNotOverwritten(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate AGENTS.md (template-seed) with custom content.
	custom := "# Custom AGENTS\nThis is project-specific content.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	skippedSeed := false
	for _, s := range res.Skipped {
		if s.Path == "AGENTS.md" && s.Owner == OwnerTemplateSeed {
			skippedSeed = true
		}
	}
	if !skippedSeed {
		t.Errorf("AGENTS.md should be skipped, got skipped=%+v", res.Skipped)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(got) != custom {
		t.Errorf("template-seed AGENTS.md was overwritten: got %q", string(got)[:60])
	}
}

func TestScaffold_TemplateManagedOverwritten(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate init.sh (template-managed) with stale content.
	stale := "#!/bin/sh\necho stale\n"
	if err := os.WriteFile(filepath.Join(dir, "init.sh"), []byte(stale), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Scaffold(dir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "init.sh"))
	if string(got) == stale {
		t.Errorf("template-managed init.sh should be overwritten")
	}
	if !strings.Contains(string(got), "BettaTech harness validator") {
		t.Errorf("init.sh post-scaffold lacks template content: %q", string(got)[:80])
	}
}

func TestScaffold_SecondRunDetectsAlreadyExisted(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	res, err := Scaffold(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AlreadyExisted {
		t.Errorf("second scaffold should set already_existed=true")
	}
	if res.Manifest == nil || res.Manifest.LastUpdateAt == "" {
		t.Errorf("second scaffold should set last_update_at")
	}
}

func TestScaffold_ManifestRecordsHashes(t *testing.T) {
	dir := t.TempDir()
	res, err := Scaffold(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest == nil {
		t.Fatal("manifest nil")
	}
	mf, ok := res.Manifest.Files["init.sh"]
	if !ok {
		t.Fatal("init.sh missing from manifest")
	}
	if mf.Owner != OwnerTemplateManaged {
		t.Errorf("init.sh owner = %s", mf.Owner)
	}
	if len(mf.SHA256) != 64 {
		t.Errorf("init.sh sha256 length = %d, want 64 hex chars", len(mf.SHA256))
	}
}
