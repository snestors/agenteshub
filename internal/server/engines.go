package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// engineDef describes a CLI engine and its supported models with their context windows.
type engineDef struct {
	Engine           string         `json:"engine"`
	Models           []string       `json:"models"`
	CtxWindows       map[string]int `json:"ctx_windows"`
	ReasoningEfforts []string       `json:"reasoning_efforts,omitempty"`
}

// staticEngines is the catalogue of supported engines.
//
// El "model" del JSON es el alias que el frontend muestra; algunos pasan por
// resolveClaudeAlias() para convertirse en el ID que el CLI realmente acepta
// (ej. opus-1m → claude-opus-4-7[1m]).
var staticEngines = []engineDef{
	{
		Engine: "claude",
		// Non-Anthropic models served via the claude CLI by pointing
		// ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY at DeepSeek's
		// Anthropic-compatible endpoint. Faster than the previous Ollama
		// Cloud wrapper; cost goes against the user's DeepSeek balance.
		Models: []string{"sonnet", "opus", "haiku", "opus-1m", "deepseek-v4-pro", "deepseek-v4-flash"},
		CtxWindows: map[string]int{
			"sonnet":            200_000,
			"opus":              200_000,
			"haiku":             200_000,
			"opus-1m":           1_000_000,
			"deepseek-v4-pro":   128_000,
			"deepseek-v4-flash": 128_000,
		},
		ReasoningEfforts: reasoningEfforts(),
	},
	{
		Engine: "codex",
		Models: []string{"gpt-5.5", "gpt-5.4", "glm-5.1"},
		CtxWindows: map[string]int{
			"gpt-5.5": 400_000,
			"gpt-5.4": 400_000,
			"glm-5.1": 128_000,
		},
		ReasoningEfforts: reasoningEfforts(),
	},
}

// availableEngines preserves the symbol used by validation helpers.
var availableEngines = staticEngines

// handleListEngines returns the engine catalogue.
func (s *Server) handleListEngines(w http.ResponseWriter, r *http.Request) {
	out := append([]engineDef(nil), staticEngines...)
	writeJSON(w, http.StatusOK, map[string]any{"engines": out})
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
	for _, e := range staticEngines {
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

func defaultModelForEngine(engine string) string {
	for _, e := range staticEngines {
		if e.Engine == engine && len(e.Models) > 0 {
			return e.Models[0]
		}
	}
	return ""
}

func reasoningEfforts() []string {
	return []string{"low", "medium", "high", "xhigh"}
}

func defaultReasoningEffort() string { return "medium" }

func normalizeReasoningEffort(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func validReasoningEffort(v string) bool {
	v = normalizeReasoningEffort(v)
	for _, e := range reasoningEfforts() {
		if e == v {
			return true
		}
	}
	return false
}

func validEngine(engine string) bool {
	for _, e := range staticEngines {
		if e.Engine == engine {
			return true
		}
	}
	return false
}
