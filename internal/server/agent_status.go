package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/ws"
)

// agentStatus is what the StatusBar consumes.
type agentStatus struct {
	Engine    string  `json:"engine"`
	Model     string  `json:"model"`
	CtxWindow int     `json:"ctx_window"`
	CtxUsed   int64   `json:"ctx_used"` // input_tokens of the last assistant turn
	CtxPct    float64 `json:"ctx_pct"`
	SessionID string  `json:"session_id"`
	WAEnabled bool    `json:"wa_enabled"`

	// plan info read from ~/.claude/.credentials.json (best-effort).
	// Empty strings if the file is missing or unreadable.
	Plan     string `json:"plan"`      // 'max' | 'pro' | ...
	PlanTier string `json:"plan_tier"` // 'default_claude_max_5x' | ...
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
	case "gpt-5.5":
		return 400_000
	}
	return 200_000
}

// computeAgentStatus builds the same struct that handleAgentStatus returns —
// extracted so it can be broadcast over WS without an HTTP round-trip.
func (s *Server) computeAgentStatus(ctx context.Context) agentStatus {
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
	used := readLastInputTokens(s.cfg.ClaudeProjectsDir, sid)
	window := modelCtxWindow(model)
	pct := 0.0
	if window > 0 && used > 0 {
		pct = float64(used) / float64(window)
		if pct > 1 {
			pct = 1
		}
	}
	plan, tier := readClaudePlan()
	return agentStatus{
		Engine:    engine,
		Model:     model,
		CtxWindow: window,
		CtxUsed:   used,
		CtxPct:    pct,
		SessionID: sid,
		WAEnabled: s.cfg.WAEnabled,
		Plan:      plan,
		PlanTier:  tier,
	}
}

// broadcastAgentStatus emits a fresh status snapshot to every subscriber on the
// "agent_status" topic. Cheap (one DB read + one JSONL scan).
func (s *Server) broadcastAgentStatus(ctx context.Context) {
	if s.hub == nil {
		return
	}
	st := s.computeAgentStatus(ctx)
	raw, err := json.Marshal(st)
	if err != nil {
		return
	}
	s.hub.Broadcast(ws.Envelope{Type: "status", Topic: "agent_status", Payload: raw})
}

// handleAgentStatus returns model/engine/ctx-usage of the current main session.
//
// ctx_used = input_tokens of the LAST assistant turn (read from the JSONL).
// That's what represents how full Claude's context window is right now —
// next turn will receive ~the same number of tokens (history + system).
// We don't sum historical tokens because that's a meaningless growing number
// for sessions that resume.
func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.computeAgentStatus(r.Context()))
}

// readClaudePlan returns (subscriptionType, rateLimitTier) from
// ~/.claude/.credentials.json. Empty strings on any error.
func readClaudePlan() (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	raw, err := os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
	if err != nil {
		return "", ""
	}
	var creds struct {
		ClaudeAiOauth struct {
			SubscriptionType string `json:"subscriptionType"`
			RateLimitTier    string `json:"rateLimitTier"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(raw, &creds); err != nil {
		return "", ""
	}
	return creds.ClaudeAiOauth.SubscriptionType, creds.ClaudeAiOauth.RateLimitTier
}

// readLastInputTokens scans the Claude JSONL of `sessionID` and returns the
// most recent assistant `usage.input_tokens` it finds. 0 if the file is
// missing or has no usage block yet.
func readLastInputTokens(projectsDir, sessionID string) int64 {
	if sessionID == "" || projectsDir == "" {
		return 0
	}
	// We don't know the encoded cwd dir up front; try the most likely ones.
	// Walk projectsDir and look for any file named "<sessionID>.jsonl".
	var path string
	_ = filepath.Walk(projectsDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Base(p) == sessionID+".jsonl" {
			path = p
			return filepath.SkipDir // any match is good enough; stop searching
		}
		return nil
	})
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	var last int64
	for scanner.Scan() {
		line := scanner.Bytes()
		// fast pre-filter — most lines are tool_use / tool_result without usage
		if !strings.Contains(string(line), `"input_tokens"`) {
			continue
		}
		var entry struct {
			Message struct {
				Usage struct {
					InputTokens              int64 `json:"input_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		// Total context = direct + cache_read + cache_creation.
		u := entry.Message.Usage
		total := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		if total > 0 {
			last = total
		}
	}
	return last
}

// computeSessionUsage kept around in case we want a historical view later.
//
//nolint:unused
func computeSessionUsage(ctx context.Context, repos *store.Repos, sessionID string) int64 {
	if sessionID == "" {
		return 0
	}
	msgs, err := repos.Sessions.MessagesForSession(ctx, sessionID, 1000)
	if err != nil {
		return 0
	}
	var tokens int64
	for _, m := range msgs {
		tokens += m.CostTokens
	}
	return tokens
}
