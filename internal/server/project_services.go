package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	projectsvc "github.com/snestors/agenteshub/internal/projects"
)

func (s *Server) handleProjectServices(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	manifest, err := projectsvc.LoadManifest(project.Path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{"services": []projectsvc.ServiceStatus{}})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": projectsvc.BuildStatus(r.Context(), manifest)})
}

func (s *Server) handleProjectServicesReload(w http.ResponseWriter, r *http.Request) {
	// The daemon intentionally does not cache manifests yet; "reload" means
	// validate/re-read from disk and return the fresh live status.
	s.handleProjectServices(w, r)
}

func (s *Server) handleProjectServiceAction(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	idx, err := strconv.Atoi(chi.URLParam(r, "idx"))
	if err != nil || idx < 0 {
		http.Error(w, "bad service index", http.StatusBadRequest)
		return
	}
	action := strings.TrimSpace(chi.URLParam(r, "action"))
	switch action {
	case "start", "stop", "restart":
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}
	manifest, err := projectsvc.LoadManifest(project.Path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "service manifest not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if idx >= len(manifest.Services) {
		http.Error(w, "service index out of range", http.StatusBadRequest)
		return
	}
	entry := manifest.Services[idx]
	switch entry.Kind {
	case "docker":
		if entry.Container == "" {
			http.Error(w, "docker container required", http.StatusBadRequest)
			return
		}
		if err := dockerAction(r.Context(), entry.Container, action); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "systemd":
		if entry.Unit == "" {
			http.Error(w, "systemd unit required", http.StatusBadRequest)
			return
		}
		if _, err := s.serviceAction(r.Context(), entry.Unit, action); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errServiceActionRequired) || isWhitelistError(err.Error()) {
				status = http.StatusBadRequest
			} else if isPermissionDenied(err.Error()) {
				status = http.StatusForbidden
			}
			http.Error(w, err.Error(), status)
			return
		}
	case "cloudflare-tunnel":
		if action != "restart" {
			http.Error(w, "cloudflare-tunnel supports restart only", http.StatusBadRequest)
			return
		}
		if _, err := s.serviceAction(r.Context(), "cloudflared", action); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "service kind is read-only in v1", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "index": idx, "action": action})
}

func dockerAction(ctx context.Context, container, action string) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "docker", action, container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker %s %s: %s", action, container, msg)
	}
	return nil
}
