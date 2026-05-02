package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// harnessFiles lists the BettaTech harness files surfaced by /harness/state.
// Each entry is (URL key, path relative to the project root). The order is the
// order they appear in the JSON response — keep it stable so the UI can
// memoize.
var harnessFiles = []struct {
	Key  string
	Path string
}{
	{"current", "progress/current.md"},
	{"history", "progress/history.md"},
	{"checkpoints", "CHECKPOINTS.md"},
}

// harnessFileMaxBytes caps how much we read per file. history.md is the only
// realistic offender; bumping this is fine but the JSON response also grows.
// Files larger than this are truncated from the END (we keep the *first*
// maxBytes) and the response sets truncated=true with the real size.
const harnessFileMaxBytes = 256 * 1024

// handleProjectHarnessState returns a snapshot of the three harness state
// files in one round-trip:
//
//	GET /api/projects/{id}/harness/state
//
//	{
//	  "current":     {"exists": true,  "path": "progress/current.md", "content": "...", "truncated": false, "size": 1234},
//	  "history":     {"exists": true,  "path": "progress/history.md", "content": "...", "truncated": true,  "size": 999999},
//	  "checkpoints": {"exists": false, "path": "CHECKPOINTS.md",      "content": "",    "truncated": false, "size": 0}
//	}
//
// Missing files are NOT errors — they're the expected state of a project that
// hasn't run a session yet (current/history) or that hasn't been scaffolded
// (checkpoints). The HUD treats exists=false as "show empty placeholder".
func (s *Server) handleProjectHarnessState(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}

	out := map[string]any{}
	for _, f := range harnessFiles {
		full := filepath.Join(project.Path, f.Path)
		out[f.Key] = readHarnessFile(full, f.Path, harnessFileMaxBytes)
	}
	writeJSON(w, http.StatusOK, out)
}

// readHarnessFile returns the standard harness-file response shape. A missing
// file is reported as exists=false, NOT a read error — the harness allows
// projects without these files yet. Other read errors fall through to
// exists=true with empty content + the error string in "error", so the UI can
// surface "could not read" instead of silently rendering empty.
func readHarnessFile(full, rel string, maxBytes int64) map[string]any {
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"exists":    false,
				"path":      rel,
				"content":   "",
				"truncated": false,
				"size":      int64(0),
			}
		}
		return map[string]any{
			"exists":    false,
			"path":      rel,
			"content":   "",
			"truncated": false,
			"size":      int64(0),
			"error":     err.Error(),
		}
	}

	size := info.Size()
	f, err := os.Open(full)
	if err != nil {
		return map[string]any{
			"exists":    true,
			"path":      rel,
			"content":   "",
			"truncated": false,
			"size":      size,
			"error":     err.Error(),
		}
	}
	defer f.Close()

	toRead := size
	truncated := false
	if maxBytes > 0 && toRead > maxBytes {
		toRead = maxBytes
		truncated = true
	}
	buf := make([]byte, toRead)
	n, err := f.Read(buf)
	if err != nil && int64(n) != toRead {
		return map[string]any{
			"exists":    true,
			"path":      rel,
			"content":   "",
			"truncated": false,
			"size":      size,
			"error":     err.Error(),
		}
	}
	return map[string]any{
		"exists":    true,
		"path":      rel,
		"content":   string(buf[:n]),
		"truncated": truncated,
		"size":      size,
	}
}

const (
	// harnessInitDefaultTimeout is the wall-clock cap when ?timeout_s= is not set.
	harnessInitDefaultTimeout = 300 * time.Second
	// harnessInitMaxTimeout is the hard ceiling regardless of ?timeout_s= — a
	// stuck init.sh shouldn't tie up an HTTP worker indefinitely.
	harnessInitMaxTimeout = 30 * time.Minute
	// harnessInitOutputCap caps combined stdout+stderr returned in the body. The
	// real output is preserved up to this size; anything past it is dropped.
	harnessInitOutputCap = 256 * 1024
)

// handleProjectHarnessInit runs init.sh in the project's repo and returns its
// output synchronously.
//
//	POST /api/projects/{id}/harness/init?timeout_s=120
//
// Response shape:
//
//	{
//	  "exit_code":   0,                  // process exit code, or -1 (spawn fail), -2 (timeout)
//	  "combined":    "...",              // stdout + stderr merged
//	  "truncated":   false,
//	  "duration_ms": 1234,
//	  "timeout":     false               // true if killed by the timeout
//	}
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

	timeout := harnessInitDefaultTimeout
	if q := r.URL.Query().Get("timeout_s"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	if timeout > harnessInitMaxTimeout {
		timeout = harnessInitMaxTimeout
	}

	res := runHarnessInit(r.Context(), project.Path, []string{
		"AGENTHUB_HARNESS=1",
		"AGENTHUB_PROJECT_ID=" + strconv.FormatInt(project.ID, 10),
		"AGENTHUB_PROJECT_NAME=" + project.Name,
	}, timeout, harnessInitOutputCap)

	writeJSON(w, http.StatusOK, map[string]any{
		"exit_code":   res.ExitCode,
		"combined":    res.Combined,
		"truncated":   res.Truncated,
		"duration_ms": res.DurationMs,
		"timeout":     res.TimedOut,
	})
}

// HarnessInitResult captures the outcome of runHarnessInit. Exposed so tests
// can introspect without going through HTTP.
type HarnessInitResult struct {
	ExitCode   int
	Combined   string
	Truncated  bool
	DurationMs int64
	TimedOut   bool
}

// runHarnessInit runs `bash init.sh` in dir with the given extra env, capping
// wall-clock time at timeout and combined output at outputCap bytes. The shape
// of ExitCode mirrors what the HTTP handler returns: 0 on success, the process
// exit code on a clean failure, -1 on spawn failure, -2 on timeout.
//
// The caller is expected to have already verified that init.sh exists and is
// executable; runHarnessInit will still surface a non-zero exit if the file
// disappears between the check and the spawn (race), but it doesn't pre-stat.
//
// Caveat: when init.sh spawns long-running child processes (e.g. a literal
// `sleep 60`), CommandContext kills bash on timeout but the orphans run to
// completion. The HTTP request returns once bash exits, which can be later
// than `timeout` suggests. For 0.4.0 this is acceptable — init.sh is meant to
// be a quick validator. If we hit a real-world stall we'll switch to a process
// group + kill -KILL on the pgid.
func runHarnessInit(parent context.Context, dir string, extraEnv []string, timeout time.Duration, outputCap int) HarnessInitResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "init.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	duration := time.Since(start)

	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)

	truncated := false
	if outputCap > 0 && len(out) > outputCap {
		out = out[:outputCap]
		truncated = true
	}

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case timedOut:
			exitCode = -2
		case errors.As(runErr, &exitErr):
			exitCode = exitErr.ExitCode()
		default:
			exitCode = -1
		}
	}

	return HarnessInitResult{
		ExitCode:   exitCode,
		Combined:   string(out),
		Truncated:  truncated,
		DurationMs: duration.Milliseconds(),
		TimedOut:   timedOut,
	}
}
