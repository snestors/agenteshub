package server

import (
	"context"
	"net/http"

	"github.com/snestors/agenthub/internal/store"
)

// agentStatus is what the StatusBar consumes.
type agentStatus struct {
	Engine      string  `json:"engine"`
	Model       string  `json:"model"`
	CtxWindow   int     `json:"ctx_window"`
	CtxUsed     int64   `json:"ctx_used"`
	CtxPct      float64 `json:"ctx_pct"`
	CostUSD     float64 `json:"cost_usd"`
	SessionID   string  `json:"session_id"`
	WAEnabled   bool    `json:"wa_enabled"`
	Permissions string  `json:"permissions"`
}

// modelCtxWindow returns the documented context window for a model alias.
// Defaults to 200K when the alias is unknown.
func modelCtxWindow(model string) int {
	switch model {
	case "opus-4-7-1m", "opus-4-7[1m]", "claude-opus-4-7[1m]":
		return 1_000_000
	case "haiku":
		return 200_000
	case "sonnet", "claude-sonnet-4-6", "sonnet-4-6":
		return 200_000
	case "opus", "opus-4-7", "claude-opus-4-7":
		return 200_000
	}
	return 200_000
}

// handleAgentStatus returns model/engine/ctx-usage of the current main session.
func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	engine := s.cfg.DefaultEngine
	model := s.cfg.DefaultModel
	if v, _ := s.repos.Settings.Get(ctx, "engine"); v != "" {
		engine = v
	}
	if v, _ := s.repos.Settings.Get(ctx, "model"); v != "" {
		model = v
	}

	mainSess, _ := s.repos.Sessions.GetAgentSession(ctx, "main-agent")
	sid := ""
	if mainSess != nil {
		sid = mainSess.SessionID
	}
	used, cost := computeSessionUsage(ctx, s.repos, sid)
	window := modelCtxWindow(model)
	pct := 0.0
	if window > 0 && used > 0 {
		pct = float64(used) / float64(window)
		if pct > 1 {
			pct = 1
		}
	}

	writeJSON(w, http.StatusOK, agentStatus{
		Engine:      engine,
		Model:       model,
		CtxWindow:   window,
		CtxUsed:     used,
		CtxPct:      pct,
		CostUSD:     cost,
		SessionID:   sid,
		WAEnabled:   s.cfg.WAEnabled,
		Permissions: "bypass",
	})
}

// computeSessionUsage sums cost_tokens of the assistant turns of a session.
// We approximate cost_usd as tokens * $3 / 1M (Sonnet output rate) — coarse but
// sufficient for the status bar; precise accounting comes later.
func computeSessionUsage(ctx context.Context, repos *store.Repos, sessionID string) (int64, float64) {
	if sessionID == "" {
		return 0, 0
	}
	msgs, err := repos.Sessions.MessagesForSession(ctx, sessionID, 1000)
	if err != nil {
		return 0, 0
	}
	var tokens int64
	for _, m := range msgs {
		tokens += m.CostTokens
	}
	cost := float64(tokens) * 3.0 / 1_000_000.0
	return tokens, cost
}
