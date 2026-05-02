package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/snestors/agenteshub/internal/harness"
)

// handleProjectHarnessState returns a snapshot of the three harness state
// files in one round-trip:
//
//	GET /api/projects/{id}/harness/state
//
// Missing files report exists=false (NOT an error) — they're the expected
// state of a project that hasn't been scaffolded or hasn't run a session yet.
// The HUD treats exists=false as "show empty placeholder".
func (s *Server) handleProjectHarnessState(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	out := harness.ReadAllState(project.Path, harness.FileMaxBytes)
	writeJSON(w, http.StatusOK, out)
}

// handleProjectHarnessScaffold materializes the embedded template into a
// project's repo. Idempotent: re-running on an already-scaffolded project
// re-applies template-managed files but leaves project-owned and
// template-seed files alone. The response shape mirrors ScaffoldResult so the
// UI can render created/skipped lists directly.
//
//	POST /api/projects/{id}/harness/scaffold
func (s *Server) handleProjectHarnessScaffold(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	res, err := harness.Scaffold(project.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleProjectHarnessInit runs init.sh in the project's repo synchronously.
//
//	POST /api/projects/{id}/harness/init?timeout_s=120
//
// The endpoint is intentionally synchronous — init.sh is meant to be a fast
// validator (go test + pnpm build + smoke), not a long-running job. Async/
// streaming flavors will arrive when there's a real use case (probably via
// the existing run-tracker + WS).
func (s *Server) handleProjectHarnessInit(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}

	initSh := filepath.Join(project.Path, "init.sh")
	info, err := os.Stat(initSh)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "init.sh missing — scaffold the harness first", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if info.Mode().Perm()&0o111 == 0 {
		http.Error(w, "init.sh exists but is not executable", http.StatusBadGateway)
		return
	}

	timeout := harness.InitDefaultTimeout
	if q := r.URL.Query().Get("timeout_s"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	if timeout > harness.InitMaxTimeout {
		timeout = harness.InitMaxTimeout
	}

	res := harness.RunInit(r.Context(), project.Path, []string{
		"AGENTHUB_HARNESS=1",
		"AGENTHUB_PROJECT_ID=" + strconv.FormatInt(project.ID, 10),
		"AGENTHUB_PROJECT_NAME=" + project.Name,
	}, timeout, harness.InitOutputCap)

	writeJSON(w, http.StatusOK, res)
}
