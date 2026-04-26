// Package cliengine spawns Claude/Codex/Ollama CLIs with --resume and persists
// turns + restores JSONL snapshots before each spawn.
package cliengine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// RunOpts captures everything needed to spawn a CLI for one turn.
type RunOpts struct {
	Prompt    string
	SessionID string // resume id; empty = first run, engine assigns one
	Channel   string // 'wa' | 'web' | 'topic' | 'project' | 'agent' | 'mini-agent'
	Cwd       string // working dir (where .claude/skills/ is discovered)
	Engine    string // 'claude' | 'codex' | 'ollama'
	Model     string // 'opus-4-7' | 'sonnet' | 'haiku' (model selection per engine)
	OutputFmt string // 'json' (final result) | 'stream-json' (chunks)

	// scope identifiers for persisting session_messages
	Scope          string // 'main' | 'topic' | 'project' | 'agent'
	TopicID        int64
	ProjectID      int64
	ProjectSessID  int64
	AgentName      string
}

// Result holds the outcome of a single turn.
type Result struct {
	Text       string
	SessionID  string // resume id (echo back; may have been newly assigned)
	CostTokens int64
	ToolsUsed  []string
}

// Engine is the abstraction implemented by Claude/Codex/Ollama.
type Engine interface {
	Name() string
	Run(ctx context.Context, opts RunOpts) (*Result, error)
}

// Manager holds engine implementations and shared deps.
type Manager struct {
	cfg       *config.Config
	repos     *store.Repos
	log       *slog.Logger
	engines   map[string]Engine
	defaultEn string
}

// New constructs a Manager with claude/codex/ollama wired.
func New(cfg *config.Config, repos *store.Repos, log *slog.Logger) *Manager {
	m := &Manager{
		cfg:       cfg,
		repos:     repos,
		log:       log,
		engines:   map[string]Engine{},
		defaultEn: cfg.DefaultEngine,
	}
	m.engines["claude"] = &ClaudeEngine{cfg: cfg, repos: repos, log: log.With("engine", "claude")}
	m.engines["codex"] = &CodexEngine{cfg: cfg, repos: repos, log: log.With("engine", "codex")}
	m.engines["ollama"] = &OllamaEngine{cfg: cfg, log: log.With("engine", "ollama")}
	return m
}

// Run dispatches to the right engine, ensures JSONL is present (restore from
// snapshot if missing), runs it, persists the user/assistant turns to
// session_messages, and triggers a snapshot post-turn (best effort).
func (m *Manager) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	if opts.Engine == "" {
		opts.Engine = m.defaultEn
	}
	if opts.OutputFmt == "" {
		opts.OutputFmt = "json"
	}
	if opts.Cwd == "" {
		opts.Cwd = "."
	}
	eng, ok := m.engines[opts.Engine]
	if !ok {
		return nil, fmt.Errorf("unknown engine %q", opts.Engine)
	}

	// 1. Restore JSONL if missing and we have a snapshot.
	if opts.SessionID != "" && opts.Engine == "claude" {
		if err := m.ensureJSONL(ctx, opts.SessionID, opts.Cwd); err != nil {
			m.log.Warn("ensure jsonl", "session", opts.SessionID, "err", err)
		}
	}

	// 2. Persist user turn.
	_, _ = m.repos.Sessions.AppendMessage(ctx, store.SessionMessage{
		Scope:         opts.Scope,
		TopicID:       sqlInt(opts.TopicID),
		ProjectID:     sqlInt(opts.ProjectID),
		ProjectSessID: sqlInt(opts.ProjectSessID),
		SessionID:     opts.SessionID,
		Role:          "user",
		Body:          sqlStr(opts.Prompt),
	})

	// 3. Run the engine.
	res, err := eng.Run(ctx, opts)
	if err != nil {
		return nil, err
	}
	if res.SessionID == "" {
		res.SessionID = opts.SessionID
	}

	// 4. Persist assistant turn.
	_, _ = m.repos.Sessions.AppendMessage(ctx, store.SessionMessage{
		Scope:         opts.Scope,
		TopicID:       sqlInt(opts.TopicID),
		ProjectID:     sqlInt(opts.ProjectID),
		ProjectSessID: sqlInt(opts.ProjectSessID),
		SessionID:     res.SessionID,
		Role:          "assistant",
		Body:          sqlStr(res.Text),
		CostTokens:    res.CostTokens,
	})

	// 5. Snapshot post-turn (best effort, async).
	if res.SessionID != "" && opts.Engine == "claude" {
		go m.snapshot(context.Background(), res.SessionID, opts.Cwd)
	}
	return res, nil
}

// ensureJSONL creates the JSONL from session_snapshots if missing.
func (m *Manager) ensureJSONL(ctx context.Context, sessionID, cwd string) error {
	jsonlPath := claudeJSONLPath(m.cfg.ClaudeProjectsDir, cwd, sessionID)
	if _, err := os.Stat(jsonlPath); err == nil {
		return nil
	}
	snap, err := m.repos.Snapshots.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if snap == nil {
		return errors.New("no snapshot")
	}
	if err := os.MkdirAll(filepath.Dir(jsonlPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(jsonlPath, snap.JsonlData, 0o600); err != nil {
		return err
	}
	m.log.Info("session restored from snapshot", "session", sessionID, "size", snap.JsonlSize)
	return nil
}

// snapshot reads the JSONL and upserts it as a BLOB in session_snapshots.
func (m *Manager) snapshot(ctx context.Context, sessionID, cwd string) {
	jsonlPath := claudeJSONLPath(m.cfg.ClaudeProjectsDir, cwd, sessionID)
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		// JSONL may not be flushed yet; best effort
		return
	}
	cwdDir := encodedCwdDir(cwd)
	_ = m.repos.Snapshots.Upsert(ctx, store.Snapshot{
		SessionID: sessionID,
		CWDDir:    cwdDir,
		JsonlData: data,
		JsonlSize: int64(len(data)),
	})
}

// claudeJSONLPath returns ~/.claude/projects/<encoded_cwd>/<session>.jsonl
func claudeJSONLPath(projectsDir, cwd, session string) string {
	return filepath.Join(projectsDir, encodedCwdDir(cwd), session+".jsonl")
}

// encodedCwdDir converts /home/nestor/grid-bot → -home-nestor-grid-bot.
func encodedCwdDir(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	return strings.ReplaceAll(abs, string(os.PathSeparator), "-")
}

// sqlInt / sqlStr helpers (avoid import cycle on database/sql at call sites)
func sqlInt(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}
func sqlStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
