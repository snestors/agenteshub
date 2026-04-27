package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/snestors/agenteshub/internal/wa"
)

// waState exposes the WhatsApp client + the path of the latest QR image to
// HTTP handlers without coupling the Server struct to the wa package.
//
// The PNG path is set at the call site (cmd/agenthub/main.go) — the consumer
// goroutine that writes the QR also writes the path here. When the client is
// connected, no QR is served (status 204).
type waState struct {
	mu     sync.RWMutex
	client *wa.Client
	qrPath string
}

// SetWAClient wires the WhatsApp client into the server so handlers can read
// connection state and serve the QR. Called from main.go after waClient.Connect.
func (s *Server) SetWAClient(c *wa.Client) {
	s.waState.mu.Lock()
	s.waState.client = c
	s.waState.mu.Unlock()
}

// SetWAQRPath records the on-disk path of the most recent pairing QR. Called
// by the QR consumer goroutine each time a new code is rendered.
func (s *Server) SetWAQRPath(path string) {
	s.waState.mu.Lock()
	s.waState.qrPath = path
	s.waState.mu.Unlock()
}

// waConnected returns true ONLY when the device is paired AND authenticated.
// We deliberately ignore the raw socket state — whatsmeow keeps an open
// socket during QR pairing too, so socket-up != usable. Callers that just
// want socket state can use waState.client.Connected() directly.
func (s *Server) waConnected() bool {
	s.waState.mu.RLock()
	c := s.waState.client
	s.waState.mu.RUnlock()
	if c == nil {
		return false
	}
	return c.LoggedIn()
}

// waQRPath returns the absolute (or daemon-relative) path of the latest QR
// PNG, or an empty string when no QR has been rendered yet.
func (s *Server) waQRPath() string {
	s.waState.mu.RLock()
	defer s.waState.mu.RUnlock()
	return s.waState.qrPath
}

// handleWaStatus reports whether the WA client is connected and, when not,
// where to fetch the pairing QR. Public on purpose: pairing has to happen
// before any user is logged in, and the JWT middleware would block it.
func (s *Server) handleWaStatus(w http.ResponseWriter, r *http.Request) {
	connected := s.waConnected()
	out := map[string]any{
		"enabled":   s.cfg != nil && s.cfg.WAEnabled,
		"connected": connected,
	}
	if !connected {
		if path := s.waQRPath(); path != "" {
			if _, err := os.Stat(path); err == nil {
				out["qr_url"] = "/api/wa/qr"
				out["qr_path"] = filepath.Clean(path)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleWaQR serves the latest pairing QR PNG. Returns 204 when the client is
// already paired (no QR needed) and 404 when WA is disabled or the consumer
// has not produced a QR yet.
func (s *Server) handleWaQR(w http.ResponseWriter, r *http.Request) {
	if s.waConnected() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	path := s.waQRPath()
	if path == "" {
		http.Error(w, "no qr available", http.StatusNotFound)
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.Error(w, "no qr available", http.StatusNotFound)
		return
	}
	// Disable caching — codes rotate every ~20s and an old PNG is useless.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, path)
}
