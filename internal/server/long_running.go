package server

import (
	"context"
	"fmt"
	"time"
)

// longRunningThreshold is when we start nagging the user about a turn that
// won't end. Users explicitly asked for a heads-up at one hour instead of
// hard-killing the run, so we just keep beating the drum every hour until the
// run finishes or they cancel it manually.
const longRunningThreshold = time.Hour

// watchLongRunning fires a notification every `longRunningThreshold` while the
// given context is alive. Designed to be invoked as a goroutine right after a
// turn starts; it returns when the context is cancelled (turn finished or
// user-cancelled). Lightweight — one timer per concurrent turn.
//
// `scope` is one of "main" / "project" / "agent" / "openspec" — used as the
// scope key in the notification context AND in the cancel registry, so the
// frontend toast can POST {scope, id} to /api/runs/cancel without translating.
// `id` is the same id the handler used in RegisterCancel (engine name for
// main, project_session_id stringified for project, run_id for agent, etc).
// `displayLabel` is shown to the user (e.g. "Project · agenthub").
// `cancelHint` is a short string explaining how to cancel from the UI.
func (s *Server) watchLongRunning(ctx context.Context, scope, id, displayLabel, cancelHint string) {
	startedAt := time.Now()
	ticker := time.NewTicker(longRunningThreshold)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startedAt).Round(time.Minute)
			s.broadcastNotification(Notification{
				Kind:     "long_running_turn",
				Severity: "warn",
				Title:    displayLabel + " · turn lleva " + formatElapsed(elapsed),
				Body:     "El agente sigue trabajando. " + cancelHint,
				Context: map[string]any{
					"scope":        scope,
					"id":           id,
					"label":        displayLabel,
					"elapsed_secs": int64(elapsed.Seconds()),
					"started_at":   startedAt.Unix(),
					"cancel_hint":  cancelHint,
					"actions": []map[string]string{
						{"label": "Cancelar", "kind": "cancel"},
						{"label": "Continuar", "kind": "dismiss"},
					},
				},
			})
		}
	}
}

func formatElapsed(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%02dm", h, m)
}
