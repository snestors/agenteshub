package cliengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/snestors/agenteshub/internal/config"
)

// OllamaEngine talks to a local ollama server (default :11434).
type OllamaEngine struct {
	cfg *config.Config
	log *slog.Logger
}

func (e *OllamaEngine) Name() string { return "ollama" }

type ollamaReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}
type ollamaResp struct {
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	EvalCount int    `json:"eval_count"`
	Error     string `json:"error,omitempty"` // populated by ollama on 4xx
}

func (e *OllamaEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	model := opts.Model
	if model == "" {
		model = e.cfg.OllamaModel
	}
	body, _ := json.Marshal(ollamaReq{Model: model, Prompt: opts.Prompt, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.OllamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: cannot reach %s (%w)", e.cfg.OllamaURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r ollamaResp
	_ = json.Unmarshal(raw, &r) // ignore decode err — we want the error first
	if resp.StatusCode >= 400 || r.Error != "" {
		msg := r.Error
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ollama: %s", msg)
	}
	if r.Response == "" {
		return nil, fmt.Errorf("ollama: empty response (model=%q)", model)
	}
	return &Result{Text: r.Response, CostTokens: int64(r.EvalCount)}, nil
}
