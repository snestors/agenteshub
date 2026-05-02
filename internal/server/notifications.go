package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenteshub/internal/store"
	"github.com/snestors/agenteshub/internal/ws"
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

	// Persist before broadcasting so a subscriber that immediately fetches
	// /api/notifications doesn't race the WS event. Failure to persist is
	// logged but doesn't block the live WS broadcast.
	if s.repos != nil && s.repos.Notifications != nil {
		var ctxJSON sql.NullString
		if len(n.Context) > 0 {
			if b, err := json.Marshal(n.Context); err == nil {
				ctxJSON = sql.NullString{String: string(b), Valid: true}
			}
		}
		row := store.Notification{
			ID:          n.ID,
			Kind:        n.Kind,
			Severity:    n.Severity,
			Title:       n.Title,
			Body:        sql.NullString{String: n.Body, Valid: n.Body != ""},
			ContextJSON: ctxJSON,
			CreatedAt:   n.TS,
		}
		if err := s.repos.Notifications.Insert(context.Background(), row); err != nil {
			s.log.Warn("notif persist", "id", n.ID, "err", err)
		}
	}

	s.hub.Broadcast(ws.Envelope{Type: "notification", Topic: "notifications", Payload: raw})
	go s.sendPushNotification(context.Background(), n)
}

// notificationToWire reshapes a stored row to the JSON shape consumed by the
// frontend toast / drawer (matches the WS envelope payload).
func notificationToWire(n store.Notification) map[string]any {
	out := map[string]any{
		"id":       n.ID,
		"kind":     n.Kind,
		"severity": n.Severity,
		"title":    n.Title,
		"ts":       n.CreatedAt,
		"read":     n.ReadAt.Valid,
	}
	if n.Body.Valid {
		out["body"] = n.Body.String
	}
	if n.ContextJSON.Valid {
		var ctx map[string]any
		if err := json.Unmarshal([]byte(n.ContextJSON.String), &ctx); err == nil {
			out["context"] = ctx
		}
	}
	if n.ReadAt.Valid {
		out["read_at"] = n.ReadAt.Int64
	}
	return out
}

// handleNotificationsList serves GET /api/notifications.
//
//	?unread=true  — only rows with read_at IS NULL
//	?limit=N      — cap on rows returned (default 50, hard ceiling 200)
//
// Response: {"items": [...], "unread_count": N}
func (s *Server) handleNotificationsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	unreadOnly := q.Get("unread") == "true" || q.Get("unread") == "1"
	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := s.repos.Notifications.List(r.Context(), unreadOnly, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	unread, _ := s.repos.Notifications.CountUnread(r.Context())
	items := make([]map[string]any, 0, len(rows))
	for _, n := range rows {
		items = append(items, notificationToWire(n))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":        items,
		"unread_count": unread,
	})
}

// handleNotificationRead marks one notification as read.
//
//	POST /api/notifications/{id}/read
func (s *Server) handleNotificationRead(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.repos.Notifications.MarkRead(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	unread, _ := s.repos.Notifications.CountUnread(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "unread_count": unread})
}

// handleNotificationsReadAll marks every unread row as read.
func (s *Server) handleNotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	if err := s.repos.Notifications.MarkAllRead(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "unread_count": 0})
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
