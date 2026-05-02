package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/snestors/agenteshub/internal/store"
)

// handleInternalNotify enqueues a WhatsApp text message addressed to
// AGENTHUB_WA_NOTIFY_PHONE. Loopback-only (no JWT). Used by cron-driven scripts
// like bin/budget-alert.sh that need to ping the user when something crosses
// a threshold.
//
// Request:
//
//	POST /api/internal/notify
//	{ "body": "..." }
//
// Response:
//
//	{ "ok": true, "outbox_id": <int64> }
//
// The actual delivery is handled by the existing wa_outbox worker — this
// endpoint just enqueues the row.
func (s *Server) handleInternalNotify(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackAddr(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.cfg == nil || strings.TrimSpace(s.cfg.WANotifyPhone) == "" {
		http.Error(w, "AGENTHUB_WA_NOTIFY_PHONE not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}

	id, err := s.repos.WaOutbox.Enqueue(r.Context(), store.WaOutboxItem{
		JID:  s.cfg.WANotifyPhone,
		Kind: "text",
		Body: sql.NullString{String: body, Valid: true},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "outbox_id": id})
}
