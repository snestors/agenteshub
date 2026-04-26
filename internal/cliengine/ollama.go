package cliengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/snestors/agenthub/internal/config"
)

// OllamaEngine talks to a local ollama server (default :11434).
type OllamaEngine struct {
	cfg *config.Config
	log *slog.Logger
}

func (e *OllamaEngine) Name() string { return "ollama" }

type ollamaReq struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Stream   bool   `json:"stream"`
}
type ollamaResp struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	EvalCount int   `json:"eval_count"`
}

func (e *OllamaEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	body, _ := json.Marshal(ollamaReq{Model: e.cfg.OllamaModel, Prompt: opts.Prompt, Stream: false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.OllamaURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r ollamaResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}
	return &Result{Text: r.Response, CostTokens: int64(r.EvalCount)}, nil
}
