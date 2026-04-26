package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ---------- system endpoints (require JWT) ----------

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.sysman.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if name == "" || action == "" {
		http.Error(w, "name and action required", http.StatusBadRequest)
		return
	}
	if err := s.sysman.ServiceAction(r.Context(), name, action); err != nil {
		// Heuristic: permission errors → 403
		s.log.Warn("service action failed", "name", name, "action", action, "err", err)
		msg := err.Error()
		status := http.StatusInternalServerError
		if isPermissionDenied(msg) {
			status = http.StatusForbidden
		} else if isWhitelistError(msg) {
			status = http.StatusBadRequest
		}
		http.Error(w, msg, status)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "action": action})
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
