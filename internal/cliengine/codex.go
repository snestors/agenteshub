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
	"syscall"

	"github.com/agenteshub/agenteshub/internal/config"
	"github.com/agenteshub/agenteshub/internal/store"
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
	Text    string `json:"text,omitempty"`
	Result  string `json:"result,omitempty"`
	Message string `json:"message,omitempty"` // for type=='error'
	Error   struct {
		Message string `json:"message"`
	} `json:"error,omitempty"` // for type=='turn.failed'
	Usage struct {
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
	if opts.ReasoningEffort != "" {
		args = append(args, "-c", "model_reasoning_effort="+opts.ReasoningEffort)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	// Closed stdin so the CLI doesn't sit waiting on its banner
	// "Reading additional input from stdin..." prompt.
	cmd.Stdin = bytes.NewReader(nil)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// We always parse stdout — even on non-zero exit — because the JSONL
	// usually contains the real reason (rate limit, model error, etc.).
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()
	runErr := cmd.Run()
	close(done)

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	var sessionID, finalText, codexErr string
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
		switch {
		case ev.Type == "item.completed" && ev.Item.Type == "agent_message" && ev.Item.Text != "":
			finalText = ev.Item.Text
		case ev.Result != "":
			finalText = ev.Result
		case ev.Type == "error" && ev.Message != "":
			codexErr = ev.Message
		case ev.Type == "turn.failed" && ev.Error.Message != "":
			codexErr = ev.Error.Message
		case ev.Text != "":
			finalText = ev.Text
		}
		if ev.Usage.InputTokens > 0 || ev.Usage.CachedInputTokens > 0 || ev.Usage.OutputTokens > 0 {
			tokens = ev.Usage.InputTokens + ev.Usage.CachedInputTokens + ev.Usage.OutputTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("codex parse: %w", err)
	}

	// If codex told us a meaningful error, surface that instead of the bare
	// exit-code message.
	if codexErr != "" {
		return nil, fmt.Errorf("codex: %s", codexErr)
	}
	if runErr != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		// strip the noisy banner so the message is actionable
		stderrStr = strings.TrimSpace(strings.ReplaceAll(stderrStr, "Reading additional input from stdin...", ""))
		if stderrStr == "" {
			stderrStr = "exit " + runErr.Error()
		}
		return nil, fmt.Errorf("codex: %s", stderrStr)
	}
	if finalText == "" {
		finalText = strings.TrimSpace(stdout.String())
	}
	return &Result{Text: finalText, SessionID: sessionID, CostTokens: tokens}, nil
}
