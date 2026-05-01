package usage

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	workerInterval  = 5 * time.Minute
	maxFileAge      = 30 * 24 * time.Hour // only scan files modified in last 30 days
	claudeBaseDir   = "/home/nestor/.claude/projects"
	codexBaseDir    = "/home/nestor/.codex/sessions"
	pricingFilePath = "data/pricing.json"
)

// Worker scans Claude and Codex JSONL directories periodically, importing
// new usage events idempotently into the database.
type Worker struct {
	repo    *UsageRepo
	pricing *PricingTable
	log     *slog.Logger
}

// NewWorker creates a Worker. Call Start to run the background loop.
func NewWorker(repo *UsageRepo, log *slog.Logger) *Worker {
	pricing := LoadPricingTable(pricingFilePath, log)
	return &Worker{repo: repo, pricing: pricing, log: log}
}

// Start launches the background goroutine. Blocks until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	// Run immediately on startup, then every workerInterval.
	w.runOnce(ctx)
	tick := time.NewTicker(workerInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	w.importSource(ctx, "claude", claudeBaseDir, w.listClaudeFiles, ParseClaudeJSONL)
	w.importSource(ctx, "codex", codexBaseDir, w.listCodexFiles, ParseCodexJSONL)
}

type parserFn func(path string) ([]Event, error)
type listerFn func(root string) ([]string, error)

func (w *Worker) importSource(ctx context.Context, source, root string, lister listerFn, parser parserFn) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return // directory absent — skip silently
	}
	files, err := lister(root)
	if err != nil {
		w.log.Warn("usage worker: list files", "source", source, "err", err)
		return
	}

	var inserted, skipped int
	for _, path := range files {
		if ctx.Err() != nil {
			return
		}
		events, err := parser(path)
		if err != nil {
			w.log.Warn("usage worker: parse", "source", source, "path", path, "err", err)
			continue
		}
		for _, ev := range events {
			ev.CostUSD = w.pricing.Cost(ev.Model, ev.Input, ev.Output, ev.CacheCreate, ev.CacheRead)
			ok, err := w.repo.UpsertEvent(ctx, ev)
			if err != nil {
				w.log.Warn("usage worker: upsert", "source", source, "err", err)
				continue
			}
			if ok {
				inserted++
			} else {
				skipped++
			}
		}
	}
	w.log.Info("usage worker: batch done",
		"source", source,
		"files_scanned", len(files),
		"events_inserted", inserted,
		"events_skipped", skipped,
	)
}

// listClaudeFiles returns .jsonl files under ~/.claude/projects modified in last 30 days.
func (w *Worker) listClaudeFiles(root string) ([]string, error) {
	return listRecentJSONL(root, maxFileAge)
}

// listCodexFiles returns .jsonl files under ~/.codex/sessions modified in last 30 days.
func (w *Worker) listCodexFiles(root string) ([]string, error) {
	return listRecentJSONL(root, maxFileAge)
}

// listRecentJSONL walks root recursively and returns .jsonl paths modified within maxAge.
func listRecentJSONL(root string, maxAge time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-maxAge)
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out, err
}
