package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/snestors/agenteshub/internal/usage"
)

// handleUsageList handles GET /api/usage
//
// Query params:
//   - since  — unix epoch or ISO date (default: 7 days ago)
//   - until  — unix epoch or ISO date (default: now)
//   - source — "claude" | "codex" | "" (all)
//   - model  — exact model filter or ""
//   - group_by — "day" | "model" | "source" | "session" (default: "day")
func (s *Server) handleUsageList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	now := time.Now().Unix()
	since := parseTimeParam(q.Get("since"), now-7*24*3600)
	until := parseTimeParam(q.Get("until"), now)
	source := strings.TrimSpace(q.Get("source"))
	model := strings.TrimSpace(q.Get("model"))
	groupBy := strings.TrimSpace(q.Get("group_by"))
	if groupBy == "" {
		groupBy = "day"
	}

	// Validate source.
	if source != "" && source != "claude" && source != "codex" {
		http.Error(w, "source must be 'claude', 'codex', or empty", http.StatusBadRequest)
		return
	}
	// Validate group_by.
	switch groupBy {
	case "day", "model", "source", "session":
	default:
		http.Error(w, "group_by must be 'day', 'model', 'source', or 'session'", http.StatusBadRequest)
		return
	}

	params := usage.QueryParams{
		Since:   since,
		Until:   until,
		Source:  source,
		Model:   model,
		GroupBy: groupBy,
	}

	totals, buckets, err := s.usageRepo.AggregateBy(r.Context(), params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"since":  since,
		"until":  until,
		"totals": totals,
		"buckets": buckets,
	})
}

// handleInternalUsage handles GET /api/internal/usage.
//
// Public-but-loopback-only endpoint consumed by bin/budget-alert.sh + cron.
// The script needs to read today/month spend without managing a JWT cookie.
// Exposing this route requires NO auth, so we gate it on RemoteAddr being
// loopback in addition to the daemon binding to 127.0.0.1 in prod — defense
// in depth in case the bind address changes.
//
// Response shape:
//
//	{
//	  "ok": true,
//	  "now":      <unix>,
//	  "today":    {"events": int, "cost_usd": float},
//	  "month":    {"events": int, "cost_usd": float},
//	  "lifetime": {"events": int, "cost_usd": float}
//	}
func (s *Server) handleInternalUsage(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackAddr(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	today, err := s.usageRepo.SumSince(ctx, todayStart)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	month, err := s.usageRepo.SumSince(ctx, monthStart)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	life, err := s.usageRepo.SumSince(ctx, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"now":      now.Unix(),
		"today":    today,
		"month":    month,
		"lifetime": life,
	})
}

// isLoopbackAddr returns true when the http.Request RemoteAddr corresponds to
// 127.0.0.0/8 or ::1. It tolerates inputs without a port.
func isLoopbackAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// parseTimeParam converts a string to unix epoch.
// Accepts unix epoch (integer string) or ISO date (YYYY-MM-DD or RFC3339).
// Returns defaultVal on empty or parse failure.
func parseTimeParam(s string, defaultVal int64) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	// Try unix epoch (all digits).
	if isAllDigits(s) {
		if n := parseInt64Default(s, -1); n >= 0 {
			return n
		}
	}
	// Try ISO date YYYY-MM-DD.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix()
	}
	// Try RFC3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	return defaultVal
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
