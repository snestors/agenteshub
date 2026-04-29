package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/store"
)

type runtimeToolEntry struct {
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name"`
	Args          any            `json:"args,omitempty"`
	Status        string         `json:"status"`
	ResultPreview string         `json:"result_preview,omitempty"`
	StartedAt     int64          `json:"started_at,omitempty"`
	FinishedAt    int64          `json:"finished_at,omitempty"`
	SubagentStats map[string]any `json:"subagent_stats,omitempty"`
}

type runtimeSnapshot struct {
	Text     string             `json:"text,omitempty"`
	Thinking string             `json:"thinking,omitempty"`
	Tools    []runtimeToolEntry `json:"tools,omitempty"`
}

type runtimeRunRef struct {
	Scope     string
	ScopeKey  string
	Topic     string
	Engine    string
	Model     string
	SessionID string
}

func (s *Server) beginRuntimeRun(ctx context.Context, ref runtimeRunRef) {
	if s.repos == nil || s.repos.ConversationRuns == nil || ref.Scope == "" || ref.ScopeKey == "" {
		return
	}
	now := time.Now().Unix()
	_ = s.repos.ConversationRuns.Upsert(ctx, store.ConversationRun{
		Scope:           ref.Scope,
		ScopeKey:        ref.ScopeKey,
		Topic:           sqlStr(ref.Topic),
		Engine:          ref.Engine,
		Model:           sqlStr(ref.Model),
		SessionID:       sqlStr(ref.SessionID),
		Status:          "running",
		StartedAt:       now,
		UpdatedAt:       now,
		TextPartial:     sql.NullString{},
		ThinkingPartial: sql.NullString{},
		ActivityJSON:    sql.NullString{},
		ResultText:      sql.NullString{},
		LastSeq:         0,
	})
}

func (s *Server) persistRuntimeSnapshot(ctx context.Context, ref runtimeRunRef, status string, snap runtimeSnapshot, sessionID, resultText, lastErr string, seq int, finished bool) {
	if s.repos == nil || s.repos.ConversationRuns == nil || ref.Scope == "" || ref.ScopeKey == "" {
		return
	}
	if sessionID != "" {
		ref.SessionID = sessionID
	}
	now := time.Now().Unix()
	run := store.ConversationRun{
		Scope:           ref.Scope,
		ScopeKey:        ref.ScopeKey,
		Topic:           sqlStr(ref.Topic),
		Engine:          ref.Engine,
		Model:           sqlStr(ref.Model),
		SessionID:       sqlStr(ref.SessionID),
		Status:          status,
		StartedAt:       now,
		UpdatedAt:       now,
		TextPartial:     sqlStr(snap.Text),
		ThinkingPartial: sqlStr(snap.Thinking),
		ActivityJSON:    store.MustJSON(snap),
		ResultText:      sqlStr(resultText),
		LastError:       sqlStr(lastErr),
		LastSeq:         seq,
	}
	if prev, err := s.repos.ConversationRuns.Get(ctx, ref.Scope, ref.ScopeKey); err == nil && prev != nil && prev.StartedAt > 0 {
		run.StartedAt = prev.StartedAt
	}
	if finished {
		run.FinishedAt = sql.NullInt64{Int64: now, Valid: true}
	}
	_ = s.repos.ConversationRuns.Upsert(ctx, run)
}

func (s *Server) finalizeRuntimeRun(ctx context.Context, ref runtimeRunRef, status, sessionID, resultText, lastErr string) {
	if s.repos == nil || s.repos.ConversationRuns == nil || ref.Scope == "" || ref.ScopeKey == "" {
		return
	}
	existing, _ := s.repos.ConversationRuns.Get(ctx, ref.Scope, ref.ScopeKey)
	snap := runtimeSnapshot{}
	seq := 0
	if existing != nil {
		if existing.ActivityJSON.Valid && existing.ActivityJSON.String != "" {
			_ = json.Unmarshal([]byte(existing.ActivityJSON.String), &snap)
		}
		if snap.Text == "" {
			snap.Text = nullString(existing.TextPartial)
		}
		if snap.Thinking == "" {
			snap.Thinking = nullString(existing.ThinkingPartial)
		}
		seq = existing.LastSeq
	}
	s.persistRuntimeSnapshot(ctx, ref, status, snap, sessionID, resultText, lastErr, seq, true)
}

func applyRuntimeEvent(snap *runtimeSnapshot, ev cliengine.StreamEvent) {
	switch ev.Kind {
	case "text":
		if ev.Text != "" {
			snap.Text += ev.Text
		}
	case "thinking":
		if ev.Text != "" {
			snap.Thinking += ev.Text
		}
	case "tool_use":
		snap.Tools = append(snap.Tools, runtimeToolEntry{
			ID:        ev.ToolID,
			Name:      ev.ToolName,
			Args:      ev.ToolArgs,
			Status:    "running",
			StartedAt: time.Now().UnixMilli(),
		})
	case "tool_result":
		idx := -1
		if ev.ToolID != "" {
			for i := range snap.Tools {
				if snap.Tools[i].ID == ev.ToolID {
					idx = i
					break
				}
			}
		}
		if idx == -1 {
			for i := len(snap.Tools) - 1; i >= 0; i-- {
				if snap.Tools[i].Status == "running" {
					idx = i
					break
				}
			}
		}
		if idx >= 0 {
			snap.Tools[idx].Status = "ok"
			snap.Tools[idx].ResultPreview = truncate(ev.ToolResult, 1000)
			snap.Tools[idx].FinishedAt = time.Now().UnixMilli()
		}
	case "subagent_stats":
		if ev.ToolID == "" || ev.Meta == nil {
			return
		}
		for i := range snap.Tools {
			if snap.Tools[i].ID == ev.ToolID {
				snap.Tools[i].SubagentStats = ev.Meta
				break
			}
		}
	}
}

type runtimeStateWire struct {
	Scope      string             `json:"scope"`
	ScopeKey   string             `json:"scope_key"`
	Topic      string             `json:"topic,omitempty"`
	Engine     string             `json:"engine"`
	Model      string             `json:"model,omitempty"`
	SessionID  string             `json:"session_id,omitempty"`
	Status     string             `json:"status"`
	StartedAt  int64              `json:"started_at"`
	UpdatedAt  int64              `json:"updated_at"`
	FinishedAt int64              `json:"finished_at,omitempty"`
	LastError  string             `json:"last_error,omitempty"`
	Text       string             `json:"text,omitempty"`
	Thinking   string             `json:"thinking,omitempty"`
	Tools      []runtimeToolEntry `json:"tools,omitempty"`
	ResultText string             `json:"result_text,omitempty"`
	LastSeq    int                `json:"last_seq"`
}

func conversationRunToWire(run *store.ConversationRun) runtimeStateWire {
	wire := runtimeStateWire{
		Scope:      run.Scope,
		ScopeKey:   run.ScopeKey,
		Topic:      nullString(run.Topic),
		Engine:     run.Engine,
		Model:      nullString(run.Model),
		SessionID:  nullString(run.SessionID),
		Status:     run.Status,
		StartedAt:  run.StartedAt,
		UpdatedAt:  run.UpdatedAt,
		LastError:  nullString(run.LastError),
		Text:       nullString(run.TextPartial),
		Thinking:   nullString(run.ThinkingPartial),
		ResultText: nullString(run.ResultText),
		LastSeq:    run.LastSeq,
	}
	if run.FinishedAt.Valid {
		wire.FinishedAt = run.FinishedAt.Int64
	}
	if run.ActivityJSON.Valid && run.ActivityJSON.String != "" {
		var snap runtimeSnapshot
		if err := json.Unmarshal([]byte(run.ActivityJSON.String), &snap); err == nil {
			if wire.Text == "" {
				wire.Text = snap.Text
			}
			if wire.Thinking == "" {
				wire.Thinking = snap.Thinking
			}
			wire.Tools = snap.Tools
		}
	}
	return wire
}
