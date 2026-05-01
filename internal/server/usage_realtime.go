package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/snestors/agenteshub/internal/usage/realtime"
)

// realtimeCache is a package-level cache shared across requests.
// Populated on-demand; no background worker needed.
var realtimeCache = realtime.NewCache()

// handleUsageRealtime handles GET /api/usage/realtime
//
// Query params:
//   - source — "claude" | "codex" | "" (both)
//
// Response: { "claude": RealtimeUsage, "codex": RealtimeUsage, "fetched_at": unix }
// Partial failure: a source that errors still returns its entry with error field;
// the overall HTTP status is always 200 as long as the handler itself runs.
func (s *Server) handleUsageRealtime(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if source != "" && source != "claude" && source != "codex" {
		http.Error(w, "source must be 'claude', 'codex', or empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	out := map[string]any{
		"fetched_at": time.Now().Unix(),
	}

	if source == "" || source == "claude" {
		u, _ := realtime.FetchAnthropicUsage(ctx, realtimeCache)
		out["claude"] = u
	}

	if source == "" || source == "codex" {
		u, _ := realtime.FetchCodexUsage(ctx, realtimeCache)
		out["codex"] = u
	}

	writeJSON(w, http.StatusOK, out)
}
