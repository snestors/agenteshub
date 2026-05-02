package server

import (
	"net/http"
	"os"
	"path/filepath"
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
