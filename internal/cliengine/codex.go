package cliengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/store"
)

// CodexEngine spawns OpenAI Codex CLI with --json + resume.
type CodexEngine struct {
	cfg    *config.Config
	repos  *store.Repos
	log    *slog.Logger
	prompt *systemPromptResolver
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
	if isNVIDIACodexModel(opts.Model) {
		return e.runNVIDIAChat(ctx, opts)
	}

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
	args = append(args, e.augmentedPrompt(ctx, opts))

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

func isNVIDIACodexModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "glm-5.1", "z-ai/glm-5.1":
		return true
	default:
		return false
	}
}

func nvidiaCodexModelID(model string) string {
	model = strings.TrimSpace(model)
	if strings.EqualFold(model, "glm-5.1") {
		return "z-ai/glm-5.1"
	}
	return model
}

type nvidiaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (e *CodexEngine) runNVIDIAChat(ctx context.Context, opts RunOpts) (*Result, error) {
	apiKey := strings.TrimSpace(e.cfg.NVIDIAAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("nvidia: NVIDIA_API_KEY no está configurada; agrega la key en .env o en el entorno del servicio")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(e.cfg.NVIDIABaseURL), "/")
	if baseURL == "" {
		baseURL = "https://integrate.api.nvidia.com/v1"
	}

	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = "nvidia-" + uuid.NewString()
	}

	messages := e.nvidiaMessages(ctx, opts)
	reqBody := map[string]any{
		"model":       nvidiaCodexModelID(opts.Model),
		"messages":    messages,
		"temperature": 0.2,
		"stream":      false,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("nvidia request: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("nvidia request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nvidia: %w", err)
	}
	defer resp.Body.Close()
	respRaw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("nvidia response: %w", err)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respRaw, &out); err != nil {
		return nil, fmt.Errorf("nvidia decode: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(out.Error.Message)
		if msg == "" {
			msg = strings.TrimSpace(string(respRaw))
		}
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		return nil, fmt.Errorf("nvidia: status %d: %s", resp.StatusCode, msg)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("nvidia: respuesta sin choices")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		text = strings.TrimSpace(out.Choices[0].Text)
	}
	if text == "" {
		return nil, fmt.Errorf("nvidia: respuesta vacía")
	}

	if opts.OnEvent != nil {
		opts.OnEvent(StreamEvent{Kind: "text", Text: text, SessionID: sessionID, Seq: 1})
		opts.OnEvent(StreamEvent{Kind: "final", Final: true, SessionID: sessionID, Seq: 2})
	}
	tokens := out.Usage.TotalTokens
	if tokens == 0 {
		tokens = out.Usage.PromptTokens + out.Usage.CompletionTokens
	}
	return &Result{Text: text, SessionID: sessionID, CostTokens: tokens}, nil
}

func (e *CodexEngine) nvidiaMessages(ctx context.Context, opts RunOpts) []nvidiaChatMessage {
	persisted := e.persistedMessages(ctx, opts)
	system := e.systemPrompt(ctx, opts)

	if len(persisted) == 0 {
		msgs := make([]nvidiaChatMessage, 0, 2)
		if system != "" {
			msgs = append(msgs, nvidiaChatMessage{Role: "system", Content: system})
		}
		msgs = append(msgs, nvidiaChatMessage{Role: "user", Content: opts.Prompt})
		return msgs
	}
	out := make([]nvidiaChatMessage, 0, len(persisted)+1)
	if system != "" {
		out = append(out, nvidiaChatMessage{Role: "system", Content: system})
	}
	for _, m := range persisted {
		if !m.Body.Valid || strings.TrimSpace(m.Body.String) == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "assistant", "system":
		default:
			role = "user"
		}
		out = append(out, nvidiaChatMessage{Role: role, Content: m.Body.String})
	}
	if len(out) == 0 || (len(out) == 1 && out[0].Role == "system") {
		out = append(out, nvidiaChatMessage{Role: "user", Content: opts.Prompt})
	}
	return out
}

// augmentedPrompt prepends the global+project system prompt to the user's
// prompt for the codex CLI path. Codex CLI has no --system flag, so we wrap
// the system context as an explicit block at the top of the first turn.
// Resumed sessions skip the prefix because codex retains prior instructions
// in its session state.
func (e *CodexEngine) augmentedPrompt(ctx context.Context, opts RunOpts) string {
	if opts.SessionID != "" {
		return opts.Prompt
	}
	system := e.systemPrompt(ctx, opts)
	if system == "" {
		return opts.Prompt
	}
	var b strings.Builder
	b.WriteString("<system_instructions>\n")
	b.WriteString(system)
	b.WriteString("\n</system_instructions>\n\n")
	b.WriteString(opts.Prompt)
	return b.String()
}

// systemPrompt resolves the global+project prompt. Matches ClaudeEngine: the
// global prompt is added for every scope. Mini-agents already embed their own
// system_prompt in opts.Prompt; the global one stacks on top of it, same as
// claude's --append-system-prompt behavior.
func (e *CodexEngine) systemPrompt(ctx context.Context, opts RunOpts) string {
	if e.prompt == nil {
		return ""
	}
	return e.prompt.Resolve(ctx, opts.Cwd)
}

func (e *CodexEngine) persistedMessages(ctx context.Context, opts RunOpts) []store.SessionMessage {
	if e.repos == nil || e.repos.Sessions == nil {
		return nil
	}
	if opts.Scope == "project" && opts.ProjectSessID != 0 {
		msgs, err := e.repos.Sessions.MessagesForProjectSession(ctx, opts.ProjectSessID, 60)
		if err == nil {
			return msgs
		}
		e.log.Warn("nvidia history lookup failed", "scope", opts.Scope, "project_session", opts.ProjectSessID, "err", err)
	}
	if strings.TrimSpace(opts.SessionID) != "" {
		msgs, err := e.repos.Sessions.MessagesForSession(ctx, opts.SessionID, 60)
		if err == nil {
			return msgs
		}
		e.log.Warn("nvidia history lookup failed", "session", opts.SessionID, "err", err)
	}
	return nil
}
