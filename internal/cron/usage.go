package cron

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	usageSessionPctKey   = "usage_session_pct"
	usageWeekPctKey      = "usage_week_pct"
	usageCalculatedAtKey = "usage_calculated_at"
	usageSessionTokenKey = "usage_session_tokens"
	usageWeekTokenKey    = "usage_week_tokens"
)

type usageSnapshot struct {
	SessionTokens int64
	WeekTokens    int64
	SessionPct    float64
	WeekPct       float64
	CalculatedAt  int64
}

type usageJSONLEntry struct {
	Timestamp string `json:"timestamp"`
	Message   struct {
		Usage usageBlock `json:"usage"`
	} `json:"message"`
}

type usageBlock struct {
	InputTokens              int64 `json:"input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func (r *Runner) updateUsageSettings(ctx context.Context) {
	snap, err := r.calculateUsage(ctx, time.Now())
	if err != nil {
		r.log.Warn("usage calculation failed", "err", err)
		return
	}
	_ = r.repos.Settings.Set(ctx, usageSessionTokenKey, strconv.FormatInt(snap.SessionTokens, 10))
	_ = r.repos.Settings.Set(ctx, usageWeekTokenKey, strconv.FormatInt(snap.WeekTokens, 10))
	_ = r.repos.Settings.Set(ctx, usageSessionPctKey, strconv.FormatFloat(snap.SessionPct, 'f', 6, 64))
	_ = r.repos.Settings.Set(ctx, usageWeekPctKey, strconv.FormatFloat(snap.WeekPct, 'f', 6, 64))
	_ = r.repos.Settings.Set(ctx, usageCalculatedAtKey, strconv.FormatInt(snap.CalculatedAt, 10))
	r.log.Info("usage estimates updated", "session_tokens", snap.SessionTokens, "week_tokens", snap.WeekTokens)
}

func (r *Runner) calculateUsage(ctx context.Context, now time.Time) (usageSnapshot, error) {
	root := r.cfg.ClaudeProjectsDir
	if _, err := os.Stat(root); err != nil {
		return usageSnapshot{CalculatedAt: now.Unix()}, nil
	}
	sessionStart := now.Add(-5 * time.Hour)
	weekStart := now.Add(-7 * 24 * time.Hour)
	var sessionTokens, weekTokens int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		st, wt := scanUsageJSONL(path, sessionStart, weekStart)
		sessionTokens += st
		weekTokens += wt
		return nil
	})
	if err != nil {
		return usageSnapshot{}, err
	}
	return usageSnapshot{
		SessionTokens: sessionTokens,
		WeekTokens:    weekTokens,
		SessionPct:    usagePct(sessionTokens, r.cfg.UsageSessionTokenLimit),
		WeekPct:       usagePct(weekTokens, r.cfg.UsageWeekTokenLimit),
		CalculatedAt:  now.Unix(),
	}, nil
}

func scanUsageJSONL(path string, sessionStart, weekStart time.Time) (int64, int64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	var sessionTokens, weekTokens int64
	for scanner.Scan() {
		line := scanner.Bytes()
		if !strings.Contains(string(line), `"usage"`) || !strings.Contains(string(line), `"input_tokens"`) {
			continue
		}
		var entry usageJSONLEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			continue
		}
		tokens := usageTotal(entry.Message.Usage)
		if tokens <= 0 || ts.Before(weekStart) {
			continue
		}
		weekTokens += tokens
		if !ts.Before(sessionStart) {
			sessionTokens += tokens
		}
	}
	return sessionTokens, weekTokens
}

func usageTotal(u usageBlock) int64 {
	// Task #36 intentionally tracks usage.input_tokens. Cache fields remain in
	// the struct so we can refine the estimate later if Anthropic publishes a
	// precise Max usage formula.
	return u.InputTokens
}

func usagePct(tokens, limit int64) float64 {
	if tokens <= 0 || limit <= 0 {
		return 0
	}
	pct := float64(tokens) / float64(limit)
	if pct > 1 {
		return 1
	}
	return pct
}
