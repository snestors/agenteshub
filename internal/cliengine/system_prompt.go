package cliengine

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/store"
)

// systemPromptResolver builds the global+project system prompt that engines
// hand to the underlying CLI. Cached once for the global file because it
// rarely changes during a daemon's lifetime.
type systemPromptResolver struct {
	cfg   *config.Config
	repos *store.Repos
	log   *slog.Logger

	once   sync.Once
	cached string
}

func newSystemPromptResolver(cfg *config.Config, repos *store.Repos, log *slog.Logger) *systemPromptResolver {
	return &systemPromptResolver{cfg: cfg, repos: repos, log: log}
}

// Resolve returns the global agent prompt joined with the project-local
// .claude/CLAUDE.md (when cwd matches a registered project). Empty string
// when neither source is available.
func (r *systemPromptResolver) Resolve(ctx context.Context, cwd string) string {
	parts := []string{}
	if g := r.global(); g != "" {
		parts = append(parts, g)
	}
	if p := r.project(ctx, cwd); p != "" {
		parts = append(parts, p)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func (r *systemPromptResolver) global() string {
	r.once.Do(func() {
		path := r.cfg.SystemPromptPath
		if path == "" {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		r.cached = strings.TrimSpace(string(raw))
	})
	return r.cached
}

func (r *systemPromptResolver) project(ctx context.Context, cwd string) string {
	if r.repos == nil || r.repos.Projects == nil || strings.TrimSpace(cwd) == "" {
		return ""
	}
	project, err := r.repos.Projects.GetByPath(ctx, cwd)
	if errors.Is(err, sql.ErrNoRows) {
		clean := filepath.Clean(cwd)
		if clean != cwd {
			project, err = r.repos.Projects.GetByPath(ctx, clean)
		}
	}
	if err != nil || project == nil {
		return ""
	}
	for _, rel := range []string{filepath.Join(".claude", "CLAUDE.md"), "CLAUDE.md"} {
		raw, err := os.ReadFile(filepath.Join(project.Path, rel))
		if err == nil {
			if prompt := strings.TrimSpace(string(raw)); prompt != "" {
				return prompt
			}
		}
	}
	return ""
}
