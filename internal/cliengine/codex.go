package cliengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// CodexEngine spawns OpenAI Codex CLI with --json + resume.
type CodexEngine struct {
	cfg   *config.Config
	repos *store.Repos
	log   *slog.Logger
}

func (e *CodexEngine) Name() string { return "codex" }

// codexEvent is a JSONL line from `codex exec --json`.
type codexEvent struct {
	Type      string `json:"type"`
	ThreadID  string `json:"thread_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Item      struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item,omitempty"`
	Text   string `json:"text,omitempty"`
	Result string `json:"result,omitempty"`
	Usage  struct {
		InputTokens       int64 `json:"input_tokens"`
		CachedInputTokens int64 `json:"cached_input_tokens"`
		OutputTokens      int64 `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

func (e *CodexEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	args := []string{"exec"}
	if opts.SessionID != "" {
		args = append(args, "resume", opts.SessionID)
	}
	args = append(args,
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"--json",
	)
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}
	if opts.Cwd != "" && opts.SessionID == "" {
		args = append(args, "-C", opts.Cwd)
	}
	args = append(args, opts.Prompt)

	bin := e.cfg.CodexBinPath
	if bin == "" {
		bin = "codex"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex run: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}

	// Parse JSONL stream
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	var sessionID, finalText string
	var tokens int64
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.ThreadID != "" {
			sessionID = ev.ThreadID
		}
		if ev.SessionID != "" {
			sessionID = ev.SessionID
		}
		if ev.Type == "item.completed" && ev.Item.Type == "agent_message" && ev.Item.Text != "" {
			finalText = ev.Item.Text
		} else if ev.Result != "" {
			finalText = ev.Result
		} else if ev.Text != "" {
			finalText = ev.Text
		}
		if ev.Usage.InputTokens > 0 || ev.Usage.CachedInputTokens > 0 || ev.Usage.OutputTokens > 0 {
			tokens = ev.Usage.InputTokens + ev.Usage.CachedInputTokens + ev.Usage.OutputTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("codex parse: %w", err)
	}
	if finalText == "" {
		finalText = strings.TrimSpace(stdout.String())
	}
	return &Result{Text: finalText, SessionID: sessionID, CostTokens: tokens}, nil
}
