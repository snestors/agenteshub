package server

import (
	"encoding/json"
	"net/http"
)

// engineDef describes a CLI engine and its supported models with their context windows.
type engineDef struct {
	Engine     string         `json:"engine"`
	Models     []string       `json:"models"`
	CtxWindows map[string]int `json:"ctx_windows"`
}

// availableEngines is the source of truth for the picker UI and POST validation.
// codex + ollama están conceptualmente soportados (cliengine los acepta) pero
// hoy no los exponemos: codex requiere validar el flow --resume y ollama
// requiere setup local. TODO: re-incluirlos cuando cada uno tenga smoke test
// E2E pasando.
//
// El "model" del JSON es el alias que el frontend muestra; algunos pasan por
// resolveClaudeAlias() para convertirse en el ID que el CLI realmente acepta
// (ej. opus-1m → claude-opus-4-7[1m]).
var availableEngines = []engineDef{
	{
		Engine: "claude",
		Models: []string{"sonnet", "opus", "haiku", "opus-1m"},
		CtxWindows: map[string]int{
			"sonnet":  200_000,
			"opus":    200_000,
			"haiku":   200_000,
			"opus-1m": 1_000_000,
		},
	},
}

// handleListEngines returns the catalogue.
func (s *Server) handleListEngines(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"engines": availableEngines})
}

// engineSetReq is the body shape for POST /api/agent/engine.
type engineSetReq struct {
	Engine string `json:"engine"`
	Model  string `json:"model"`
}

// handleSetEngine persists engine+model in the settings table. The cliengine
// reads these from settings on every Run, so the change applies to the next
// turn.
func (s *Server) handleSetEngine(w http.ResponseWriter, r *http.Request) {
	var req engineSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !validEngineModel(req.Engine, req.Model) {
		http.Error(w, "engine/model not in supported list", http.StatusBadRequest)
		return
	}
	if err := s.repos.Settings.Set(r.Context(), "engine", req.Engine); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.repos.Settings.Set(r.Context(), "model", req.Model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "engine": req.Engine, "model": req.Model})
}

func validEngineModel(engine, model string) bool {
	for _, e := range availableEngines {
		if e.Engine != engine {
			continue
		}
		for _, m := range e.Models {
			if m == model {
				return true
			}
		}
		return false
	}
	return false
}
