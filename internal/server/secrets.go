package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenteshub/internal/auth"
	"github.com/snestors/agenteshub/internal/store"
)

// secretWire is the metadata-only shape returned by listings. Plaintext
// values never travel over /api/secrets — caller has to use the dedicated
// reveal endpoint and confirm.
type secretWire struct {
	Key            string `json:"key"`
	Description    string `json:"description,omitempty"`
	Scope          string `json:"scope"`
	ExpiresAt      *int64 `json:"expires_at,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	LastAccessedAt *int64 `json:"last_accessed_at,omitempty"`
}

func secretToWire(s store.Secret) secretWire {
	w := secretWire{
		Key:       s.Key,
		Scope:     s.Scope,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
	if s.Description.Valid {
		w.Description = s.Description.String
	}
	if s.ExpiresAt.Valid {
		v := s.ExpiresAt.Int64
		w.ExpiresAt = &v
	}
	if s.LastAccessedAt.Valid {
		v := s.LastAccessedAt.Int64
		w.LastAccessedAt = &v
	}
	return w
}

func (s *Server) handleSecretsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.repos.Secrets.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]secretWire, 0, len(rows))
	for _, row := range rows {
		out = append(out, secretToWire(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": out})
}

type secretUpsertReq struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresAt   int64  `json:"expires_at,omitempty"`
}

func (s *Server) handleSecretsCreate(w http.ResponseWriter, r *http.Request) {
	var req secretUpsertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(req.Key)
	if key == "" || strings.TrimSpace(req.Value) == "" {
		http.Error(w, "key and value required", http.StatusBadRequest)
		return
	}
	enc, err := auth.EncryptAESGCM(s.cfg.SecretKey, []byte(req.Value))
	if err != nil {
		http.Error(w, "encrypt failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sec := store.Secret{
		Key:      key,
		ValueEnc: enc,
		Scope:    strings.TrimSpace(req.Scope),
	}
	if d := strings.TrimSpace(req.Description); d != "" {
		sec.Description = sql.NullString{String: d, Valid: true}
	}
	if req.ExpiresAt > 0 {
		sec.ExpiresAt = sql.NullInt64{Int64: req.ExpiresAt, Valid: true}
	}
	if err := s.repos.Secrets.Upsert(r.Context(), sec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": key})
}

// handleSecretReveal returns the plaintext for a single secret. This is the
// only path that surfaces the value, intended for the UI's reveal-with-confirm
// affordance and for ad-hoc copies. Audited via last_accessed_at.
func (s *Server) handleSecretReveal(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	enc, err := s.repos.Secrets.GetValue(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if enc == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	plain, err := auth.DecryptAESGCM(s.cfg.SecretKey, enc)
	if err != nil {
		http.Error(w, "decrypt failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":   key,
		"value": string(plain),
		"ts":    time.Now().Unix(),
	})
}

func (s *Server) handleSecretDelete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "key required", http.StatusBadRequest)
		return
	}
	if err := s.repos.Secrets.Delete(r.Context(), key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
