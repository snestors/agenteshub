package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// engineDef describes a CLI engine and its supported models with their context windows.
type engineDef struct {
	Engine     string         `json:"engine"`
	Models     []string       `json:"models"`
	CtxWindows map[string]int `json:"ctx_windows"`
}

// availableEngines is the source of truth for the picker UI and POST validation.
// ollama está conceptualmente soportado (cliengine lo acepta) pero hoy no lo
// exponemos: requiere setup local. TODO: re-incluirlo cuando tenga smoke test
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
	{
		Engine: "codex",
		Models: []string{"gpt-5.5"},
		CtxWindows: map[string]int{
			"gpt-5.5": 400_000,
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
	res, err := s.setEngine(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errInvalidEngineModel) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	// Push fresh status to subscribers (engine/model changed → ctx_window may differ)
	go s.broadcastAgentStatus(context.Background())
	writeJSON(w, http.StatusOK, res)
}

var errInvalidEngineModel = errors.New("engine/model not in supported list")

func (s *Server) setEngine(ctx context.Context, req engineSetReq) (map[string]any, error) {
	if !validEngineModel(req.Engine, req.Model) {
		return nil, errInvalidEngineModel
	}
	if err := s.repos.Settings.Set(ctx, "engine", req.Engine); err != nil {
		return nil, err
	}
	if err := s.repos.Settings.Set(ctx, "model", req.Model); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "engine": req.Engine, "model": req.Model}, nil
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

func validEngine(engine string) bool {
	for _, e := range availableEngines {
		if e.Engine == engine {
			return true
		}
	}
	return false
}
