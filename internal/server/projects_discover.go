package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// candidateProject is a directory under the discovery root that looks like a
// project (has .git or one of the well-known stack markers) and is NOT yet
// enrolled in the projects table.
type candidateProject struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Stack     []string `json:"stack"`
	HasGit    bool     `json:"has_git"`
	HasClaude bool     `json:"has_claude"`
}

// stackMarkers maps a marker filename to the stack tag it implies. Keep this
// list small — it is heuristics, not detection. Front-end shows the tags so
// the user can decide whether to enroll.
var stackMarkers = map[string]string{
	"go.mod":           "go",
	"package.json":     "node",
	"pnpm-lock.yaml":   "node-pnpm",
	"requirements.txt": "python",
	"pyproject.toml":   "python",
	"Cargo.toml":       "rust",
	"composer.json":    "php",
	"Gemfile":          "ruby",
	"pubspec.yaml":     "flutter",
	"wrangler.toml":    "cloudflare",
	"astro.config.mjs": "astro",
	"astro.config.ts":  "astro",
	"next.config.js":   "next",
	"next.config.ts":   "next",
	"vite.config.ts":   "vite",
	"vite.config.js":   "vite",
}

// handleProjectsDiscover scans a fixed root (default /home/nestor) for
// directories that look like projects and are NOT yet registered. Returns
// candidates the UI can offer to enroll in one click.
//
// Scope: top-level only (depth 1) — we don't recurse to avoid scanning
// node_modules / vendor / .git internals. If a project lives nested (e.g.
// monorepo), the user enrolls it manually with POST /api/projects.
func (s *Server) handleProjectsDiscover(w http.ResponseWriter, r *http.Request) {
	root := strings.TrimSpace(r.URL.Query().Get("root"))
	if root == "" {
		root = "/home/nestor"
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		http.Error(w, "discover root not a directory", http.StatusBadRequest)
		return
	}

	// Build a set of already-enrolled paths (canonicalized) so we skip them.
	existing := map[string]struct{}{}
	if projects, err := s.repos.Projects.List(r.Context()); err == nil {
		for _, p := range projects {
			existing[filepath.Clean(p.Path)] = struct{}{}
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]candidateProject, 0, 16)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip dotfolders, system mounts, snap caches and bin/data dirs of agenthub itself.
		if strings.HasPrefix(name, ".") || name == "snap" || name == "Downloads" || name == "Desktop" {
			continue
		}
		path := filepath.Join(root, name)
		if _, dupe := existing[filepath.Clean(path)]; dupe {
			continue
		}

		c := candidateProject{Name: name, Path: path, Stack: []string{}}

		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			c.HasGit = true
		}
		if _, err := os.Stat(filepath.Join(path, "CLAUDE.md")); err == nil {
			c.HasClaude = true
		}
		if _, err := os.Stat(filepath.Join(path, ".claude")); err == nil {
			c.HasClaude = true
		}
		stackSet := map[string]struct{}{}
		for marker, tag := range stackMarkers {
			if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
				stackSet[tag] = struct{}{}
			}
		}
		for tag := range stackSet {
			c.Stack = append(c.Stack, tag)
		}
		sort.Strings(c.Stack)

		// Filter: only return directories that look like projects. We need at
		// least one signal — git repo, CLAUDE.md, or a stack marker.
		if !c.HasGit && !c.HasClaude && len(c.Stack) == 0 {
			continue
		}
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	writeJSON(w, http.StatusOK, map[string]any{
		"root":       root,
		"candidates": out,
	})
}
