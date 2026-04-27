package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenteshub/internal/ws"
)

// ---------- system endpoints (require JWT) ----------

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	st := s.buildSystemTick(r.Context())
	if st == nil {
		http.Error(w, "stats unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSystemServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := s.sysman.Services(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": svcs})
}

func (s *Server) handleSystemServiceAction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	action := chi.URLParam(r, "action")
	res, err := s.serviceAction(r.Context(), name, action)
	if err != nil {
		s.log.Warn("service action failed", "name", name, "action", action, "err", err)
		msg := err.Error()
		status := http.StatusInternalServerError
		if errors.Is(err, errServiceActionRequired) {
			status = http.StatusBadRequest
		} else if isPermissionDenied(msg) {
			status = http.StatusForbidden
		} else if isWhitelistError(msg) {
			status = http.StatusBadRequest
		}
		http.Error(w, msg, status)
		// also push the failure to subscribers so the UI can refresh state.
		s.broadcastServiceEvent(name, action, "error", msg)
		return
	}
	s.broadcastServiceEvent(name, action, "ok", "")
	writeJSON(w, http.StatusOK, res)
}

var errServiceActionRequired = errors.New("name and action required")

func (s *Server) serviceAction(ctx context.Context, name, action string) (map[string]any, error) {
	if name == "" || action == "" {
		return nil, errServiceActionRequired
	}
	if err := s.sysman.ServiceAction(ctx, name, action); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "name": name, "action": action}, nil
}

// broadcastServiceEvent emits a service_event envelope on the system topic so
// any UI subscribed sees the action result immediately (no need to wait for
// the next 2s stats tick).
func (s *Server) broadcastServiceEvent(name, action, result, errMsg string) {
	if s.hub == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"name":   name,
		"action": action,
		"result": result, // 'ok' | 'error'
		"error":  errMsg,
	})
	s.hub.Broadcast(ws.Envelope{Type: "service_event", Topic: "system", Payload: payload})
	// Force an immediate stats refresh too, so the service's `state` flips fast.
	go s.broadcastSystemStats()
}

func (s *Server) broadcastSystemStats() {
	if s.hub == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tick := s.buildSystemTick(ctx)
	if tick == nil {
		return
	}
	raw, _ := json.Marshal(tick)
	s.hub.Broadcast(ws.Envelope{Type: "stats", Topic: "system", Payload: raw})
}

func (s *Server) handleSystemProcesses(w http.ResponseWriter, r *http.Request) {
	top := 10
	if v := r.URL.Query().Get("top"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			top = n
		}
	}
	sortBy := r.URL.Query().Get("sort")
	procs, err := s.sysman.Processes(r.Context(), top, sortBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processes": procs})
}

func (s *Server) handleSystemConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := s.sysman.Connections(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Inject live WS clients count (sysman doesn't know about the hub).
	if s.hub != nil {
		conns.WSClients = s.hub.CountClients()
	}
	writeJSON(w, http.StatusOK, conns)
}

func isPermissionDenied(msg string) bool {
	// systemd / polkit / sudo error strings vary across distros — match the common ones.
	return contains(msg, "denied") ||
		contains(msg, "permission") ||
		contains(msg, "Permission") ||
		contains(msg, "Authentication") ||
		contains(msg, "Interactive authentication") ||
		contains(msg, "polkit") ||
		contains(msg, "Polkit") ||
		contains(msg, "no new privileges") ||
		contains(msg, "no_new_privs") ||
		contains(msg, "running as root")
}

func isWhitelistError(msg string) bool {
	return contains(msg, "whitelist") || contains(msg, "invalid action")
}

func contains(s, sub string) bool {
	// avoid pulling in strings just to keep this file lean; small helper
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
