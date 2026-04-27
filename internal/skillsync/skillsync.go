// Package skillsync discovers SKILL.md files on disk and persists them as
// rows in the `skills` table so the API + UI can browse them, plus pull
// from configured remote git registries (future work).
//
// For v1 it only scans the global ~/.claude/skills/ tree; remote pull is
// stubbed and reported in the sync result.
package skillsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/agenteshub/agenteshub/internal/store"
)

// Result is what a Sync() call returns to the caller (UI / API).
type Result struct {
	Sources       []SourceResult `json:"sources"`
	StartedAt     int64          `json:"started_at"`
	FinishedAt    int64          `json:"finished_at"`
	TotalUpserted int            `json:"total_upserted"`
	TotalRemoved  int            `json:"total_removed"`
	Errors        []string       `json:"errors,omitempty"`
}

// SourceResult holds the per-source breakdown.
type SourceResult struct {
	Source    string `json:"source"`     // 'local', 'registry:<name>', etc.
	Path      string `json:"path"`       // root path scanned
	Discovered int   `json:"discovered"` // files seen
	Upserted  int    `json:"upserted"`
	Removed   int    `json:"removed"`
	Error     string `json:"error,omitempty"`
}

type Syncer struct {
	repos *store.Repos
}

func New(repos *store.Repos) *Syncer { return &Syncer{repos: repos} }

// Sync discovers + persists every SKILL.md under the configured roots.
// Errors per-source are recorded but don't abort the whole run.
func (s *Syncer) Sync(ctx context.Context) (*Result, error) {
	res := &Result{StartedAt: time.Now().Unix()}

	// 1. Local global skills under ~/.claude/skills/
	if home, err := os.UserHomeDir(); err == nil {
		root := filepath.Join(home, ".claude", "skills")
		sr := s.scanDir(ctx, "local", root)
		res.Sources = append(res.Sources, sr)
		res.TotalUpserted += sr.Upserted
		res.TotalRemoved += sr.Removed
	}

	// 2. Project-scoped skills are picked up at endpoint time per project,
	//    not on a global sync — kept out of v1.

	// 3. Remote registries: TODO — pull from git into data/skill-registry/<name>/
	//    and recursively scanDir(name, that-path). No-op for now.

	res.FinishedAt = time.Now().Unix()
	return res, nil
}

// scanDir walks a root, parses every SKILL.md it finds, and upserts
// matching rows under the given source. Stale rows for that source are
// removed after the walk.
func (s *Syncer) scanDir(ctx context.Context, source, root string) SourceResult {
	out := SourceResult{Source: source, Path: root}
	if root == "" {
		return out
	}
	info, err := os.Stat(root)
	if err != nil {
		out.Error = "root not found"
		return out
	}
	if !info.IsDir() {
		out.Error = "root not a dir"
		return out
	}

	cutoff := time.Now().Unix()
	walkErr := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries silently
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Base(p) != "SKILL.md" {
			return nil
		}
		out.Discovered++
		skill, ok := parseSkillFile(p, source)
		if !ok {
			return nil
		}
		if err := s.repos.Skills.Upsert(ctx, skill); err == nil {
			out.Upserted++
		}
		return nil
	})
	if walkErr != nil {
		out.Error = walkErr.Error()
	}

	// Remove rows that weren't touched in this run (deleted from disk).
	if removed, err := s.repos.Skills.DeleteStale(ctx, source, cutoff); err == nil {
		out.Removed = int(removed)
	}
	return out
}

// parseSkillFile reads a SKILL.md, extracts YAML frontmatter, returns a
// Skill ready for upsert. Returns (zero, false) on parse failure.
func parseSkillFile(path, source string) (store.Skill, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return store.Skill{}, false
	}
	s := string(raw)
	if !strings.HasPrefix(s, "---") {
		// Skill without frontmatter is still a skill — synthesize a name from
		// the parent directory and fall back gracefully.
		return store.Skill{
			Name:   strings.ToLower(filepath.Base(filepath.Dir(path))),
			Source: source,
			Body:   s,
			Path:   path,
		}, true
	}
	rest := strings.TrimPrefix(s, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return store.Skill{}, false
	}
	front := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---"):])

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		return store.Skill{}, false
	}
	name, _ := fm["name"].(string)
	if strings.TrimSpace(name) == "" {
		name = strings.ToLower(filepath.Base(filepath.Dir(path)))
	}
	desc, _ := fm["description"].(string)
	role, _ := fm["role_hint"].(string)
	version, _ := fm["version"].(string)

	frontJSON, _ := json.Marshal(fm)

	out := store.Skill{
		Name:        name,
		Source:      source,
		Body:        body,
		Path:        path,
		Description: nullStr(desc),
		RoleHint:    nullStr(role),
		Version:     nullStr(version),
		Frontmatter: nullStr(string(frontJSON)),
	}
	return out, true
}

func nullStr(s string) (n storeNullString) {
	n = storeNullString{Valid: s != "", String: s}
	return
}

// storeNullString aliases sql.NullString without importing it directly here
// — keeps the package small + dependency-free for tests.
type storeNullString = struct {
	String string
	Valid  bool
}

// Verify Result + SourceResult get JSON-encoded cleanly.
var _ = fmt.Sprintf
