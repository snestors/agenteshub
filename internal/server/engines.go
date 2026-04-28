package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// engineDef describes a CLI engine and its supported models with their context windows.
type engineDef struct {
	Engine           string         `json:"engine"`
	Models           []string       `json:"models"`
	CtxWindows       map[string]int `json:"ctx_windows"`
	ReasoningEfforts []string       `json:"reasoning_efforts,omitempty"`
}

// staticEngines is the always-present part of the catalogue. Ollama is
// added on top of this dynamically — see resolveEngines below.
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

// availableEngines preserves the symbol used by validation helpers — it
// holds the static catalogue. Dynamic ollama discovery is layered on top
// at the HTTP boundary (handleListEngines) and validated separately.
var availableEngines = staticEngines

// ollamaCache memoises the result of /api/tags for a short window so that
// every picker open doesn't poke the local server.
var (
	ollamaCacheMu  sync.Mutex
	ollamaCacheAt  time.Time
	ollamaCacheVal *engineDef
)

// resolveOllamaEngine returns an engineDef for the local ollama daemon if
// it's reachable and has at least one chat-capable model. Returns nil
// otherwise. Cached for 30s.
func (s *Server) resolveOllamaEngine(ctx context.Context) *engineDef {
	ollamaCacheMu.Lock()
	if time.Since(ollamaCacheAt) < 30*time.Second {
		v := ollamaCacheVal
		ollamaCacheMu.Unlock()
		return v
	}
	ollamaCacheMu.Unlock()

	def := fetchOllamaModels(ctx, s.cfg.OllamaURL)

	ollamaCacheMu.Lock()
	ollamaCacheAt = time.Now()
	ollamaCacheVal = def
	ollamaCacheMu.Unlock()
	return def
}

func fetchOllamaModels(ctx context.Context, baseURL string) *engineDef {
	if baseURL == "" {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var body struct {
		Models []struct {
			Name    string `json:"name"`
			Details struct {
				Family string `json:"family"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	models := make([]string, 0, len(body.Models))
	ctxWindows := map[string]int{}
	for _, m := range body.Models {
		// Embedding-only models don't make sense in a chat picker.
		if isEmbeddingModel(m.Name, m.Details.Family) {
			continue
		}
		models = append(models, m.Name)
		ctxWindows[m.Name] = 8_192 // sane default; ollama doesn't tell us up front
	}
	if len(models) == 0 {
		return nil
	}
	return &engineDef{Engine: "ollama", Models: models, CtxWindows: ctxWindows, ReasoningEfforts: reasoningEfforts()}
}

func isEmbeddingModel(name, family string) bool {
	low := strings.ToLower(name + " " + family)
	for _, marker := range []string{"embed", "nomic", "bge-", "mxbai", "snowflake-arctic"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

// handleListEngines returns the catalogue (static engines + dynamic ollama).
func (s *Server) handleListEngines(w http.ResponseWriter, r *http.Request) {
	out := append([]engineDef(nil), staticEngines...)
	if def := s.resolveOllamaEngine(r.Context()); def != nil {
		out = append(out, *def)
	}
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
	// Ollama: any non-empty model is accepted at validation time. The actual
	// model presence is enforced by the engine when it runs (it'll error if
	// the model isn't pulled).
	if engine == "ollama" && strings.TrimSpace(model) != "" {
		return true
	}
	return false
}

func defaultModelForEngine(engine, ollamaDefault string) string {
	for _, e := range staticEngines {
		if e.Engine == engine && len(e.Models) > 0 {
			return e.Models[0]
		}
	}
	if engine == "ollama" {
		if strings.TrimSpace(ollamaDefault) != "" {
			return strings.TrimSpace(ollamaDefault)
		}
		return "gemma:2b"
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
	return engine == "ollama"
}
