package cliengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/snestors/agenteshub/internal/auth"
	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/store"
)

// ClaudeEngine spawns the `claude` CLI with --resume, --output-format json.
type ClaudeEngine struct {
	cfg    *config.Config
	repos  *store.Repos
	log    *slog.Logger
	prompt *systemPromptResolver
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
	deepseekDirect := isDeepSeekDirectModel(opts.Model)
	// claude --model accepts Anthropic ids natively AND non-Anthropic ids
	// (deepseek-v4-pro, deepseek-v4-flash) when ANTHROPIC_BASE_URL points at
	// a compatible provider. Either way we pass --model through.
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

	claudeBin := e.cfg.ClaudeBinPath
	if claudeBin == "" {
		claudeBin = "claude"
	}
	cmd := exec.CommandContext(ctx, claudeBin, args...)
	if deepseekDirect {
		// claude CLI honours ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY to point at
		// any Anthropic-compatible provider. DeepSeek exposes one at
		// `/anthropic`; the API key lives in the encrypted vault under
		// DEEPSEEK_API_KEY.
		key, err := e.deepseekAPIKey(ctx)
		if err != nil {
			return nil, err
		}
		env := os.Environ()
		env = append(env,
			"ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic",
			"ANTHROPIC_API_KEY="+key,
		)
		cmd.Env = env
	}
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
	if e.prompt == nil {
		return ""
	}
	return e.prompt.Resolve(ctx, opts.Cwd)
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
	// streamedText accumulates text emitted via stream events (text_delta or
	// assistant text blocks). It's our fallback when the final `result` JSONL
	// line never arrives — has happened when claude exits cleanly without
	// flushing the result event, or when the daemon is racing a restart and
	// the CLI's last line is dropped. The user already saw the text via the
	// WebSocket, so persisting it from this buffer keeps the chat consistent.
	var streamedText strings.Builder
	// taskToolUseIDs collects every Agent (Task) tool_use the parent emits.
	// After cmd.Wait we look up the rich `toolUseResult` for each one in the
	// session's JSONL on disk and re-emit a `subagent_stats` event so the
	// frontend can decorate the SubAgentCard (duration, tokens, tool count
	// breakdown) with stats measured by claude itself.
	var taskToolUseIDs []string
	seq := 0
	emit := func(ev StreamEvent) {
		seq++
		ev.Seq = seq
		if ev.SessionID == "" {
			ev.SessionID = res.SessionID
		}
		if ev.Kind == "text" && ev.Text != "" {
			streamedText.WriteString(ev.Text)
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
					if b.Name == "Agent" && b.ID != "" {
						taskToolUseIDs = append(taskToolUseIDs, b.ID)
					}
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
			// `final` is deferred until after cmd.Wait so we can squeeze in
			// subagent_stats events first (read from the JSONL on disk once
			// claude has flushed it). Frontend then sees: stats → final, and
			// can decorate SubAgentCards before the ghost bubble closes.
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		_ = cmd.Wait()
		return nil, fmt.Errorf("stream scan: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		// If the CLI exited with a non-zero status but we still captured
		// streamed text (the user already saw it), keep that as the answer
		// instead of throwing it away. Cost stays at 0 since the result
		// event never delivered usage figures.
		if buf := strings.TrimSpace(streamedText.String()); buf != "" {
			res.Text = buf
			e.emitSubagentStats(opts, emit, res.SessionID, taskToolUseIDs)
			emit(StreamEvent{Kind: "final", Text: res.Text, Final: true})
			return res, nil
		}
		return nil, fmt.Errorf("claude wait: %w (stderr=%s)", err, strings.TrimSpace(stderr.String()))
	}
	if res.Text == "" {
		// Clean exit with no result event — fall back to the streamed text.
		// Empirically this happens when the daemon is racing a restart and
		// the CLI's last JSONL line is dropped.
		if buf := strings.TrimSpace(streamedText.String()); buf != "" {
			res.Text = buf
			e.emitSubagentStats(opts, emit, res.SessionID, taskToolUseIDs)
			emit(StreamEvent{Kind: "final", Text: res.Text, Final: true})
			return res, nil
		}
		return nil, fmt.Errorf("claude stream produced no result")
	}
	e.emitSubagentStats(opts, emit, res.SessionID, taskToolUseIDs)
	emit(StreamEvent{Kind: "final", Text: res.Text, Final: true})
	return res, nil
}

// subagentStats holds the slice of toolUseResult fields the JSONL exposes
// for each Task() invocation. Claude CLI doesn't stream sub-agent internals
// to stdout, but it does write a rich block to the session JSONL when each
// Task completes. We re-emit those blocks so the frontend can decorate.
type subagentStats struct {
	ToolUseID          string         `json:"tool_use_id"`
	AgentID            string         `json:"agent_id"`
	AgentType          string         `json:"agent_type"`
	Status             string         `json:"status"`
	TotalDurationMs    int64          `json:"total_duration_ms"`
	TotalTokens        int64          `json:"total_tokens"`
	TotalToolUseCount  int64          `json:"total_tool_use_count"`
	Usage              map[string]any `json:"usage,omitempty"`
	ToolStats          map[string]any `json:"tool_stats,omitempty"`
}

// emitSubagentStats parses the on-disk JSONL for the just-finished session
// and emits one stream event per Task tool_use_id we tracked. Best-effort:
// if the JSONL is missing or unreadable, we skip silently — these events
// are decoration, not critical.
func (e *ClaudeEngine) emitSubagentStats(opts RunOpts, emit func(StreamEvent), sessionID string, taskIDs []string) {
	if len(taskIDs) == 0 || sessionID == "" || opts.OnEvent == nil {
		return
	}
	jsonlPath := claudeJSONLPath(e.cfg.ClaudeProjectsDir, opts.Cwd, sessionID)
	stats, err := parseSubagentStats(jsonlPath, taskIDs)
	if err != nil || len(stats) == 0 {
		return
	}
	for _, s := range stats {
		emit(StreamEvent{
			Kind:   "subagent_stats",
			ToolID: s.ToolUseID,
			Meta: map[string]any{
				"agent_id":              s.AgentID,
				"agent_type":            s.AgentType,
				"status":                s.Status,
				"total_duration_ms":     s.TotalDurationMs,
				"total_tokens":          s.TotalTokens,
				"total_tool_use_count":  s.TotalToolUseCount,
				"usage":                 s.Usage,
				"tool_stats":            s.ToolStats,
			},
		})
	}
}

// parseSubagentStats walks a session JSONL once and returns one stats entry
// per requested tool_use_id (in claude's format, e.g. "call_abcdef"). Order
// in the return matches order of first occurrence in the JSONL.
func parseSubagentStats(jsonlPath string, taskIDs []string) ([]subagentStats, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	wanted := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		wanted[id] = true
	}
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type rawTUR struct {
		Status            string         `json:"status"`
		AgentID           string         `json:"agentId"`
		AgentType         string         `json:"agentType"`
		TotalDurationMs   int64          `json:"totalDurationMs"`
		TotalTokens       int64          `json:"totalTokens"`
		TotalToolUseCount int64          `json:"totalToolUseCount"`
		Usage             map[string]any `json:"usage"`
		ToolStats         map[string]any `json:"toolStats"`
	}
	type rawEntry struct {
		Type    string `json:"type"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		ToolUseResult *rawTUR `json:"toolUseResult"`
	}

	var out []subagentStats
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || !bytes.Contains(line, []byte("toolUseResult")) {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if e.Type != "user" || e.ToolUseResult == nil || len(e.Message.Content) == 0 {
			continue
		}
		// Each user/tool_result entry references its tool_use_id inside
		// message.content[*].tool_use_id. Find any that we're tracking.
		var blocks []struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
		}
		if err := json.Unmarshal(e.Message.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_result" || !wanted[b.ToolUseID] {
				continue
			}
			out = append(out, subagentStats{
				ToolUseID:          b.ToolUseID,
				AgentID:            e.ToolUseResult.AgentID,
				AgentType:          e.ToolUseResult.AgentType,
				Status:             e.ToolUseResult.Status,
				TotalDurationMs:    e.ToolUseResult.TotalDurationMs,
				TotalTokens:        e.ToolUseResult.TotalTokens,
				TotalToolUseCount:  e.ToolUseResult.TotalToolUseCount,
				Usage:              e.ToolUseResult.Usage,
				ToolStats:          e.ToolUseResult.ToolStats,
			})
			delete(wanted, b.ToolUseID)
		}
	}
	return out, scanner.Err()
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

// isDeepSeekDirectModel returns true for DeepSeek models served via the
// provider's Anthropic-compatible REST endpoint. The claude CLI is invoked
// directly (no wrapper); env vars steer it to DeepSeek instead of Anthropic.
func isDeepSeekDirectModel(m string) bool {
	return strings.HasPrefix(strings.TrimSpace(m), "deepseek-")
}

// deepseekAPIKey loads and decrypts the DEEPSEEK_API_KEY secret from the vault.
// Returns a clear error when the key is missing — surfaced to the user as the
// engine error so they know to set it via /api/secrets.
func (e *ClaudeEngine) deepseekAPIKey(ctx context.Context) (string, error) {
	if e.repos == nil || e.repos.Secrets == nil {
		return "", fmt.Errorf("deepseek: vault not wired in this build")
	}
	enc, err := e.repos.Secrets.GetValue(ctx, "DEEPSEEK_API_KEY")
	if err != nil {
		return "", fmt.Errorf("deepseek: vault lookup: %w", err)
	}
	if len(enc) == 0 {
		return "", fmt.Errorf("deepseek: DEEPSEEK_API_KEY not in vault — set it via POST /api/secrets")
	}
	plain, err := auth.DecryptAESGCM(e.cfg.SecretKey, enc)
	if err != nil {
		return "", fmt.Errorf("deepseek: decrypt: %w", err)
	}
	return strings.TrimSpace(string(plain)), nil
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
