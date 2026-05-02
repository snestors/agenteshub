package harness

import (
	"os"
	"path/filepath"
)

// StateFiles lists the BettaTech harness files surfaced as "harness state".
// Each entry is (URL key, path relative to the project root). Order is the
// order they appear in API responses — keep it stable so the UI can memoize.
var StateFiles = []struct {
	Key  string
	Path string
}{
	{"current", "progress/current.md"},
	{"history", "progress/history.md"},
	{"checkpoints", "CHECKPOINTS.md"},
}

// FileMaxBytes caps how much we read per state file. history.md is the only
// realistic offender; bumping this is fine but the consumer (HTTP body, MCP
// tool result) grows accordingly. Files larger than this keep the FIRST
// FileMaxBytes bytes; the truncation flag in the response makes that explicit.
const FileMaxBytes = 256 * 1024

// FileSnapshot is the standard shape returned by ReadStateFile. We use a
// concrete type (not map[string]any) because both the HTTP server and the MCP
// server consume it — keeping it typed avoids stringly-typed bugs.
type FileSnapshot struct {
	Exists    bool   `json:"exists"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Size      int64  `json:"size"`
	Error     string `json:"error,omitempty"`
}

// ReadStateFile returns a FileSnapshot for full = projectRoot + rel. A missing
// file is reported as exists=false (NOT a read error) — the harness allows
// projects without these files yet. Other read errors fall through to
// exists=true with empty content + the error string in Error.
func ReadStateFile(full, rel string, maxBytes int64) FileSnapshot {
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return FileSnapshot{Path: rel}
		}
		return FileSnapshot{Path: rel, Error: err.Error()}
	}

	size := info.Size()
	f, err := os.Open(full)
	if err != nil {
		return FileSnapshot{Exists: true, Path: rel, Size: size, Error: err.Error()}
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
		return FileSnapshot{Exists: true, Path: rel, Size: size, Error: err.Error()}
	}
	return FileSnapshot{
		Exists:    true,
		Path:      rel,
		Content:   string(buf[:n]),
		Truncated: truncated,
		Size:      size,
	}
}

// ReadAllState returns one FileSnapshot per entry in StateFiles, keyed by Key.
// Used by both /api/projects/{id}/harness/state and the query_project_state
// MCP tool.
func ReadAllState(projectRoot string, maxBytes int64) map[string]FileSnapshot {
	out := make(map[string]FileSnapshot, len(StateFiles))
	for _, f := range StateFiles {
		out[f.Key] = ReadStateFile(filepath.Join(projectRoot, f.Path), f.Path, maxBytes)
	}
	return out
}
