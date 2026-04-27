package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/agenteshub/agenteshub/internal/ws"
)

// Notification is what the frontend toast / badge consumes.
//
// Kind is the discriminator the UI uses to pick an icon and decide
// which sidebar item gets the unread-count bump.
type Notification struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`     // 'agent_run_done' | 'agent_run_failed' | ...
	Severity string         `json:"severity"` // 'info' | 'warn' | 'error'
	Title    string         `json:"title"`
	Body     string         `json:"body,omitempty"`
	Context  map[string]any `json:"context,omitempty"`
	TS       int64          `json:"ts"`
}

// broadcastNotification emits a notification envelope on the global
// 'notifications' topic so any subscriber (NotificationProvider) sees it.
func (s *Server) broadcastNotification(n Notification) {
	if s.hub == nil {
		return
	}
	if n.ID == "" {
		n.ID = "n-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	if n.TS == 0 {
		n.TS = time.Now().Unix()
	}
	if n.Severity == "" {
		n.Severity = "info"
	}
	raw, err := json.Marshal(n)
	if err != nil {
		return
	}
	s.hub.Broadcast(ws.Envelope{Type: "notification", Topic: "notifications", Payload: raw})
}

// NotifyAgentRunFinished is the callback shape consumed by the scheduler.
// Called after FinishRun on any cron-triggered run.
func (s *Server) NotifyAgentRunFinished(_ context.Context, agentName string, agentID, runID int64, status, result, errStr string) {
	kind := "agent_run_done"
	severity := "info"
	body := truncate(result, 280)
	if status != "ok" {
		kind = "agent_run_failed"
		severity = "error"
		body = truncate(errStr, 280)
	}
	s.broadcastNotification(Notification{
		Kind:     kind,
		Severity: severity,
		Title:    "Agent · " + agentName,
		Body:     body,
		Context: map[string]any{
			"agent_id": agentID,
			"run_id":   runID,
			"trigger":  "cron",
			"status":   status,
		},
	})
}

// isShutdownError returns true when the error is caused by the daemon shutting
// down (context cancelled or the claude CLI receiving SIGTERM/SIGKILL from the
// OS as a child-process). These are not real errors — the turn will resume
// naturally on the next user message.
func isShutdownError(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "exit status 143") ||
		strings.Contains(msg, "signal: killed") ||
		strings.Contains(msg, "signal: terminated")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
