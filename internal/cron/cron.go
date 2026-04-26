// Package cron runs the daemon's internal scheduled jobs.
//
// Internal jobs only — mini-agent schedules live in agent_schedules.
package cron

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// Runner owns the internal cron scheduler.
type Runner struct {
	cfg   *config.Config
	repos *store.Repos
	log   *slog.Logger
	c     *cron.Cron
}

// New constructs the runner with all internal jobs registered.
func New(cfg *config.Config, repos *store.Repos, log *slog.Logger) *Runner {
	r := &Runner{cfg: cfg, repos: repos, log: log, c: cron.New()}

	// Snapshot of Claude JSONLs — defaults to every 5 min.
	_, _ = r.c.AddFunc("@every 5m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		r.snapshotJSONLs(ctx)
	})

	// JWT revocations cleanup — once a day.
	_, _ = r.c.AddFunc("@every 24h", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = repos.Auth.DeleteExpiredRevocations(ctx)
	})

	// Local Claude usage estimate — every 30 min.
	_, _ = r.c.AddFunc("@every 30m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		r.updateUsageSettings(ctx)
	})

	return r
}

// Start kicks off the cron loop. Stop on Stop().
func (r *Runner) Start() {
	r.c.Start()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		r.updateUsageSettings(ctx)
	}()
}

// Stop halts the scheduler.
func (r *Runner) Stop() { r.c.Stop() }

// snapshotJSONLs walks ~/.claude/projects/ and upserts session JSONLs as BLOBs.
func (r *Runner) snapshotJSONLs(ctx context.Context) {
	root := r.cfg.ClaudeProjectsDir
	if _, err := os.Stat(root); err != nil {
		return
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return
	}
	count := 0
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		cwdEnc := d.Name()
		entries, err := os.ReadDir(filepath.Join(root, cwdEnc))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
			path := filepath.Join(root, cwdEnc, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			_ = r.repos.Snapshots.Upsert(ctx, store.Snapshot{
				SessionID: sessionID,
				CWDDir:    cwdEnc,
				JsonlData: data,
				JsonlSize: int64(len(data)),
			})
			count++
		}
	}
	if count > 0 {
		r.log.Info("session snapshots upserted", "count", count)
	}
}
