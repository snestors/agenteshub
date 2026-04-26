package cliengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// ClaudeEngine spawns the `claude` CLI with --resume, --output-format json.
type ClaudeEngine struct {
	cfg   *config.Config
	repos *store.Repos
	log   *slog.Logger
}

// Name implements Engine.
func (e *ClaudeEngine) Name() string { return "claude" }

// claudeJSONResp is the shape of `claude -p --output-format json` output.
type claudeJSONResp struct {
	Type      string `json:"type"`        // "result"
	Subtype   string `json:"subtype"`     // "success"
	Result    string `json:"result"`      // the assistant text
	SessionID string `json:"session_id"`  // resume id (echo back)
	IsError   bool   `json:"is_error"`
	Usage     struct {
		InputTokens     int `json:"input_tokens"`
		OutputTokens    int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// Run executes claude with the given prompt and returns the result.
func (e *ClaudeEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", "json",
	}
	if cfgPath, err := ensureMCPConfig(e.cfg); err == nil {
		args = append(args, "--mcp-config", cfgPath)
	}
	if model := chooseModel(opts.Model, e.cfg.DefaultModel); model != "" {
		args = append(args, "--model", model)
	}
	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}
	prompt := opts.Prompt
	if opts.Channel != "" {
		prompt = fmt.Sprintf("[canal: %s]\n%s", opts.Channel, prompt)
	}
	args = append(args, prompt)

	bin := e.cfg.ClaudeBinPath
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = opts.Cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude run: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}

	var resp claudeJSONResp
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		// fallback: maybe plain text on stdout
		text := strings.TrimSpace(stdout.String())
		if text == "" {
			return nil, fmt.Errorf("claude empty output: %s", stderr.String())
		}
		return &Result{Text: text, SessionID: opts.SessionID}, nil
	}
	if resp.IsError {
		return nil, fmt.Errorf("claude error: %s", resp.Result)
	}
	return &Result{
		Text:       resp.Result,
		SessionID:  resp.SessionID,
		CostTokens: int64(resp.Usage.InputTokens + resp.Usage.OutputTokens),
	}, nil
}

func chooseModel(opt, fallback string) string {
	if strings.TrimSpace(opt) != "" {
		return opt
	}
	return fallback
}

// ensureMCPConfig writes a JSON file pointing the Claude CLI to `agenthub mcp`,
// so spawned sessions can call our embedded tools. Returns the file path.
func ensureMCPConfig(cfg *config.Config) (string, error) {
	dir := filepath.Join(os.TempDir(), "agenthub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "mcp.json")
	bin, err := os.Executable()
	if err != nil {
		bin = "agenthub"
	}
	type mcpServer struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	body := struct {
		MCPServers map[string]mcpServer `json:"mcpServers"`
	}{
		MCPServers: map[string]mcpServer{
			"agenthub": {
				Command: bin,
				Args:    []string{"mcp"},
				Env: map[string]string{
					"AGENTHUB_DB_PATH":    cfg.DBPath,
					"AGENTHUB_DEV":        "true", // mcp mode reads same .env, but force minimal
					"AGENTHUB_SECRET_KEY": fmt.Sprintf("%x", cfg.SecretKey),
					"AGENTHUB_JWT_SECRET": string(cfg.JWTSecret),
				},
			},
		},
	}
	raw, _ := json.MarshalIndent(body, "", "  ")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
