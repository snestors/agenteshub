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
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Message   json.RawMessage `json:"message,omitempty"`
	Text      string          `json:"text,omitempty"`
	Result    string          `json:"result,omitempty"`
}

func (e *CodexEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"--json",
	}
	if opts.SessionID != "" {
		args = append([]string{"exec", "resume", opts.SessionID, "--json", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox"}, []string{}...)
	}
	if opts.Cwd != "" {
		args = append(args, "-C", opts.Cwd)
	}
	args = append(args, opts.Prompt)

	bin := e.cfg.CodexBinPath
	if bin == "" {
		bin = "codex"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex run: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}

	// Parse JSONL stream
	scanner := bufio.NewScanner(&stdout)
	var sessionID, finalText string
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.SessionID != "" {
			sessionID = ev.SessionID
		}
		if ev.Result != "" {
			finalText = ev.Result
		} else if ev.Text != "" {
			finalText = ev.Text
		}
	}
	if finalText == "" {
		finalText = strings.TrimSpace(stdout.String())
	}
	return &Result{Text: finalText, SessionID: sessionID}, nil
}
