package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadBytes = 50 * 1024 * 1024 // 50 MB per file

// Upload metadata returned to the client and persisted alongside the file.
type uploadMeta struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	CreatedAt int64  `json:"created_at"`
}

// handleUpload accepts a single multipart "file" and stores it under data/uploads/.
// Returns the metadata so the client can reference it on the next /api/messages call.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "bad multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > maxUploadBytes {
		http.Error(w, fmt.Sprintf("file too big (max %d MB)", maxUploadBytes/(1024*1024)), http.StatusRequestEntityTooLarge)
		return
	}

	dir := s.cfg.UploadDir
	if dir == "" {
		dir = "data/uploads"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "ensure dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id := uuid.NewString()
	safeName := sanitizeFilename(header.Filename)
	final := id + "-" + safeName
	path := filepath.Join(dir, final)

	out, err := os.Create(path)
	if err != nil {
		http.Error(w, "create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		http.Error(w, "write: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if written > maxUploadBytes {
		_ = os.Remove(path)
		http.Error(w, "file too big", http.StatusRequestEntityTooLarge)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta := uploadMeta{
		ID:        id,
		Name:      header.Filename,
		Size:      written,
		Type:      contentType,
		Path:      path,
		CreatedAt: time.Now().Unix(),
	}
	// Save sidecar JSON so handleDeleteUpload can resolve id → path
	sidecar := filepath.Join(dir, id+".json")
	raw, _ := json.Marshal(meta)
	_ = os.WriteFile(sidecar, raw, 0o600)

	writeJSON(w, http.StatusOK, meta)
}

// handleDeleteUpload removes a previously uploaded file by id.
func (s *Server) handleDeleteUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	dir := s.cfg.UploadDir
	if dir == "" {
		dir = "data/uploads"
	}
	sidecar := filepath.Join(dir, id+".json")
	raw, err := os.ReadFile(sidecar)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var meta uploadMeta
	_ = json.Unmarshal(raw, &meta)
	_ = os.Remove(meta.Path)
	_ = os.Remove(sidecar)
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeFilename keeps the name human-readable but strips path components.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	// allow letters, digits, common punctuation; collapse the rest to underscore
	out := strings.Builder{}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_', r == ' ':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
	}
	clean := out.String()
	if clean == "" {
		clean = "upload"
	}
	return clean
}
