package cliengine

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/store"
)

// ClaudeEngine spawns the `claude` CLI with --resume, --output-format json.
type ClaudeEngine struct {
	cfg      *config.Config
	repos    *store.Repos
	log      *slog.Logger
	spOnce   sync.Once
	spCached string
}

// Name implements Engine.
func (e *ClaudeEngine) Name() string { return "claude" }

// claudeJSONResp is the shape of `claude -p --output-format json` output.
type claudeJSONResp struct {
	Type      string `json:"type"`       // "result"
	Subtype   string `json:"subtype"`    // "success"
	Result    string `json:"result"`     // the assistant text
	SessionID string `json:"session_id"` // resume id (echo back)
	IsError   bool   `json:"is_error"`
	Usage     struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// Run executes claude with the given prompt. When opts.OnEvent is non-nil,
// it spawns with --output-format stream-json and pipes events through the
// callback in real time. Otherwise it stays in single-shot json mode.
func (e *ClaudeEngine) Run(ctx context.Context, opts RunOpts) (*Result, error) {
	streaming := opts.OnEvent != nil
	includePartialMessages := streaming

	outFmt := "json"
	if streaming {
		// stream-json requires --verbose; --include-partial-messages makes the
		// CLI emit stream_event/content_block_delta entries instead of only
		// complete assistant blocks at the end of the turn.
		outFmt = "stream-json"
	}
	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--output-format", outFmt,
	}
	if streaming {
		args = append(args, "--verbose")
	}
	if includePartialMessages {
		args = append(args, "--include-partial-messages")
	}
	if cfgPath, err := ensureMCPConfig(e.cfg); err == nil {
		args = append(args, "--mcp-config", cfgPath)
	}
	if sp := e.appendSystemPrompt(ctx, opts); sp != "" {
		args = append(args, "--append-system-prompt", sp)
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

	if streaming {
		return e.runStreaming(ctx, cmd, opts, includePartialMessages)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude run: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}

	var resp claudeJSONResp
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
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

// appendSystemPrompt returns the global AgentHub prompt plus, when opts.Cwd is
// a registered project path, the project-local .claude/CLAUDE.md prompt.
func (e *ClaudeEngine) appendSystemPrompt(ctx context.Context, opts RunOpts) string {
	parts := []string{}
	if global := e.globalSystemPrompt(); global != "" {
		parts = append(parts, global)
	}
	if project := e.projectSystemPrompt(ctx, opts.Cwd); project != "" {
		parts = append(parts, project)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// globalSystemPrompt reads cfg.SystemPromptPath and returns its contents.
// Cached after the first read; the file rarely changes during a daemon's
// lifetime and re-reading on every turn would only burn syscalls.
func (e *ClaudeEngine) globalSystemPrompt() string {
	e.spOnce.Do(func() {
		path := e.cfg.SystemPromptPath
		if path == "" {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		e.spCached = strings.TrimSpace(string(raw))
	})
	return e.spCached
}

func (e *ClaudeEngine) projectSystemPrompt(ctx context.Context, cwd string) string {
	if e.repos == nil || e.repos.Projects == nil || strings.TrimSpace(cwd) == "" {
		return ""
	}
	project, err := e.repos.Projects.GetByPath(ctx, cwd)
	if errors.Is(err, sql.ErrNoRows) {
		clean := filepath.Clean(cwd)
		if clean != cwd {
			project, err = e.repos.Projects.GetByPath(ctx, clean)
		}
	}
	if err != nil || project == nil {
		return ""
	}
	for _, rel := range []string{filepath.Join(".claude", "CLAUDE.md"), "CLAUDE.md"} {
		raw, err := os.ReadFile(filepath.Join(project.Path, rel))
		if err == nil {
			if prompt := strings.TrimSpace(string(raw)); prompt != "" {
				return prompt
			}
		}
	}
	return ""
}

// streamEvent is a line in `claude --output-format stream-json --verbose`.
type streamEvent struct {
	Type      string          `json:"type"`    // 'system' | 'stream_event' | 'assistant' | 'user' | 'result'
	Subtype   string          `json:"subtype"` // for 'system' / 'result'
	SessionID string          `json:"session_id"`
	Message   json.RawMessage `json:"message"` // for 'assistant' | 'user'
	Result    string          `json:"result"`  // for 'result'
	IsError   bool            `json:"is_error"`
	Usage     struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

type contentBlock struct {
	Type      string          `json:"type"` // 'text' | 'tool_use' | 'tool_result' | 'thinking'
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     map[string]any  `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"` // for tool_result; can be string or []block
}

type claudeStreamEvent struct {
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	} `json:"event"`
}

// runStreaming spawns the cmd, parses each JSONL event, fires opts.OnEvent for
// each useful one, and returns the final Result when 'result' arrives.
func (e *ClaudeEngine) runStreaming(ctx context.Context, cmd *exec.Cmd, opts RunOpts, includePartialMessages bool) (*Result, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	res := &Result{SessionID: opts.SessionID}
	seq := 0
	emit := func(ev StreamEvent) {
		seq++
		ev.Seq = seq
		if ev.SessionID == "" {
			ev.SessionID = res.SessionID
		}
		opts.OnEvent(ev)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}
		if ev.SessionID != "" {
			res.SessionID = ev.SessionID
		}
		switch ev.Type {
		case "system":
			emit(StreamEvent{Kind: "system", Text: ev.Subtype})
		case "stream_event":
			var sev claudeStreamEvent
			if err := json.Unmarshal(raw, &sev); err != nil {
				continue
			}
			if sev.Event.Type != "content_block_delta" {
				continue
			}
			switch sev.Event.Delta.Type {
			case "text_delta":
				if sev.Event.Delta.Text != "" {
					emit(StreamEvent{Kind: "text", Text: sev.Event.Delta.Text})
				}
			case "thinking_delta":
				if sev.Event.Delta.Thinking != "" {
					emit(StreamEvent{Kind: "thinking", Text: sev.Event.Delta.Thinking})
				}
			}
		case "assistant", "user":
			// Decode message.content as []contentBlock
			var msg struct {
				Content []contentBlock `json:"content"`
			}
			if err := json.Unmarshal(ev.Message, &msg); err != nil {
				continue
			}
			for _, b := range msg.Content {
				switch b.Type {
				case "text":
					if includePartialMessages {
						continue
					}
					if b.Text != "" {
						emit(StreamEvent{Kind: "text", Text: b.Text})
					}
				case "thinking":
					if includePartialMessages {
						continue
					}
					if b.Thinking != "" {
						emit(StreamEvent{Kind: "thinking", Text: b.Thinking})
					}
				case "tool_use":
					emit(StreamEvent{Kind: "tool_use", ToolName: b.Name, ToolID: b.ID, ToolArgs: b.Input})
				case "tool_result":
					emit(StreamEvent{Kind: "tool_result", ToolID: b.ToolUseID, ToolResult: rawToText(b.Content)})
				}
			}
		case "result":
			if ev.IsError {
				_ = cmd.Wait()
				return nil, fmt.Errorf("claude stream error: %s (stderr=%s)", ev.Result, strings.TrimSpace(stderr.String()))
			}
			res.Text = ev.Result
			res.CostTokens = int64(ev.Usage.InputTokens + ev.Usage.OutputTokens)
			emit(StreamEvent{Kind: "final", Text: ev.Result, Final: true})
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		_ = cmd.Wait()
		return nil, fmt.Errorf("stream scan: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("claude wait: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}
	if res.Text == "" {
		return nil, fmt.Errorf("claude stream produced no result")
	}
	return res, nil
}

// rawToText flattens tool_result content (string or []block) into a single string.
func rawToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// case 1: plain string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// case 2: array of {type:'text', text:'...'} blocks
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		out := strings.Builder{}
		for i, b := range blocks {
			if i > 0 {
				out.WriteString("\n")
			}
			if b.Text != "" {
				out.WriteString(b.Text)
			}
		}
		return out.String()
	}
	return string(raw)
}

func chooseModel(opt, fallback string) string {
	picked := opt
	if strings.TrimSpace(picked) == "" {
		picked = fallback
	}
	return resolveClaudeAlias(picked)
}

// resolveClaudeAlias converts AgentHub-friendly model names into the exact
// IDs the Claude CLI accepts. Claude Code's `--model` parser is finicky:
// 'sonnet'/'opus'/'haiku' work, full IDs like 'claude-opus-4-7' work, but
// '-1m' suffix variants only work as bracket-suffixed IDs.
func resolveClaudeAlias(m string) string {
	switch strings.TrimSpace(m) {
	case "opus-1m", "opus-4-7-1m":
		return "claude-opus-4-7[1m]"
	}
	return m
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
