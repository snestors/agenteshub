// Package mcp implements the embedded MCP server (stdio).
//
// Spawned by `agenthub mcp` and consumed by Claude/Codex CLIs via --mcp-config.
// Tools have direct access to the AgentHub DB (no HTTP loopback).
package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/robfig/cron/v3"

	"github.com/snestors/agenteshub/internal/auth"
	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/harness"
	"github.com/snestors/agenteshub/internal/store"
	"github.com/snestors/agenteshub/internal/sysman"
)

// Server wraps the MCP server bound to repos.
type Server struct {
	cfg     *config.Config
	repos   *store.Repos
	sysman  *sysman.Manager
	engines *cliengine.Manager
	srv     *server.MCPServer
}

// New constructs the MCP server with all tools registered.
//
// sm and engines may be nil — tools that depend on them will surface a
// helpful error at call time. This keeps `agenthub mcp` usable when the
// caller doesn't (yet) wire system/engine deps.
func New(cfg *config.Config, repos *store.Repos, sm *sysman.Manager, engines *cliengine.Manager) *Server {
	s := &Server{cfg: cfg, repos: repos, sysman: sm, engines: engines}
	s.srv = server.NewMCPServer("agenthub", "0.1.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	s.registerTools()
	return s
}

// ServeStdio blocks reading MCP requests from stdin and writing to stdout.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.srv)
}

func (s *Server) registerTools() {
	// ---------- output ----------
	s.srv.AddTool(mcp.NewTool("send_message",
		mcp.WithDescription("Send a text message to the user via the active channel (WhatsApp or Web)."),
		mcp.WithString("text", mcp.Required(), mcp.Description("Plain text body to send.")),
		mcp.WithString("jid", mcp.Description("Optional WA JID. When set, also queues an outgoing WA text — useful when responding outside the live conversation context.")),
		mcp.WithString("reply_to", mcp.Description("Optional WA stanza id to quote. Requires jid.")),
	), s.handleSendMessage)

	s.srv.AddTool(mcp.NewTool("send_image",
		mcp.WithDescription("Send an image. When jid is omitted inside a live chat turn, AgentHub posts it back into the current conversation; otherwise it queues the file for WhatsApp delivery."),
		mcp.WithString("jid", mcp.Description("Optional target JID (digits or full '<phone>@s.whatsapp.net'). Omit to use the current live chat when available.")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the image on the daemon's filesystem.")),
		mcp.WithString("caption", mcp.Description("Optional caption shown below the image.")),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to (use the 'reply_to' field of an incoming message).")),
	), s.handleSendImage)

	s.srv.AddTool(mcp.NewTool("send_voice",
		mcp.WithDescription("Send a voice note (PTT). File should be opus-encoded .ogg. When jid is omitted inside a live chat turn, AgentHub posts it back into the current conversation when possible."),
		mcp.WithString("jid"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the .ogg file.")),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to.")),
	), s.handleSendVoice)

	s.srv.AddTool(mcp.NewTool("send_audio",
		mcp.WithDescription("Send an audio file (music / non-PTT). When jid is omitted inside a live chat turn, AgentHub posts it back into the current conversation when possible."),
		mcp.WithString("jid"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to.")),
	), s.handleSendAudio)

	s.srv.AddTool(mcp.NewTool("send_document",
		mcp.WithDescription("Send a file as a document attachment. When jid is omitted inside a live chat turn, AgentHub posts it back into the current conversation when possible. Filename shown is the basename of the path."),
		mcp.WithString("jid"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("caption", mcp.Description("Optional caption.")),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to.")),
	), s.handleSendDocument)

	s.srv.AddTool(mcp.NewTool("send_video",
		mcp.WithDescription("Send a video file. When jid is omitted inside a live chat turn, AgentHub posts it back into the current conversation; in Web this makes the video clickable/playable in the chat UI."),
		mcp.WithString("jid"),
		mcp.WithString("path", mcp.Required()),
		mcp.WithString("caption", mcp.Description("Optional caption.")),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to.")),
	), s.handleSendVideo)

	s.srv.AddTool(mcp.NewTool("send_location",
		mcp.WithDescription("Send a static location pin to a WhatsApp JID."),
		mcp.WithString("jid", mcp.Required()),
		mcp.WithNumber("lat", mcp.Required(), mcp.Description("Latitude in decimal degrees.")),
		mcp.WithNumber("lng", mcp.Required(), mcp.Description("Longitude in decimal degrees.")),
		mcp.WithString("name", mcp.Description("Optional place label shown next to the pin.")),
		mcp.WithString("reply_to", mcp.Description("WA stanza id of the message you are replying to.")),
	), s.handleSendLocation)

	// ---------- input ----------
	s.srv.AddTool(mcp.NewTool("recent_messages",
		mcp.WithDescription("Returns recent messages from the user, optionally filtered by channel."),
		mcp.WithNumber("limit", mcp.Description("Max messages to return (default 50).")),
		mcp.WithString("channel", mcp.Description("'wa' | 'web' (empty = both).")),
	), s.handleRecentMessages)

	s.srv.AddTool(mcp.NewTool("mark_read",
		mcp.WithDescription("Mark a single message as read."),
		mcp.WithNumber("id", mcp.Required()),
	), s.handleMarkRead)

	// ---------- records (schema-on-read) ----------
	s.srv.AddTool(mcp.NewTool("record",
		mcp.WithDescription("Persist arbitrary JSON data under a topic."),
		mcp.WithString("topic", mcp.Required()),
		mcp.WithString("data", mcp.Required(), mcp.Description("JSON string with the payload.")),
	), s.handleRecord)

	s.srv.AddTool(mcp.NewTool("query_records",
		mcp.WithDescription("Query stored records of a topic."),
		mcp.WithString("topic", mcp.Required()),
		mcp.WithNumber("since", mcp.Description("Unix epoch — only records after this.")),
		mcp.WithNumber("limit", mcp.Description("Max rows (default 50).")),
	), s.handleQueryRecords)

	s.srv.AddTool(mcp.NewTool("list_topics_records",
		mcp.WithDescription("List distinct topics + their record counts."),
	), s.handleListRecordTopics)

	// ---------- topics (conversational contexts) ----------
	s.srv.AddTool(mcp.NewTool("list_topics",
		mcp.WithDescription("List conversational topics tracked by the main agent."),
	), s.handleListTopics)

	s.srv.AddTool(mcp.NewTool("read_topic_state",
		mcp.WithDescription("Read the JSON state of a topic (headline, pending, decisions, etc.)."),
		mcp.WithString("topic", mcp.Required()),
	), s.handleReadTopicState)

	s.srv.AddTool(mcp.NewTool("update_topic_state",
		mcp.WithDescription("Patch the JSON state of a topic. Pass any subset of fields."),
		mcp.WithString("topic", mcp.Required()),
		mcp.WithString("headline"),
		mcp.WithString("active_issues", mcp.Description("JSON array string.")),
		mcp.WithString("recent_decisions", mcp.Description("JSON array string.")),
		mcp.WithString("pending", mcp.Description("JSON array string.")),
		mcp.WithString("next_action_hint"),
	), s.handleUpdateTopicState)

	s.srv.AddTool(mcp.NewTool("topic_create",
		mcp.WithDescription("Create a new conversational topic. The session_id starts empty and is filled on the first run_in_topic."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Unique topic name, e.g. 'grid-bot'.")),
		mcp.WithString("description"),
		mcp.WithString("keywords", mcp.Description("JSON array of strings.")),
		mcp.WithString("engine", mcp.Description("'claude' (default) | 'codex' | 'ollama'.")),
	), s.handleTopicCreate)

	s.srv.AddTool(mcp.NewTool("run_in_topic",
		mcp.WithDescription("Delegate a turn to the topic's session. Resumes the topic's session_id if any, otherwise starts a fresh one and persists it. Returns the assistant's reply."),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic name.")),
		mcp.WithString("message", mcp.Required(), mcp.Description("Prompt to send to the topic session.")),
	), s.handleRunInTopic)

	// ---------- secrets vault ----------
	s.srv.AddTool(mcp.NewTool("secret_get",
		mcp.WithDescription("Read a stored credential by key from the encrypted vault. Returns the plaintext value. NEVER log or echo the result back to the user — use it inside the same turn (e.g. as a request header) and forget it."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Vault key, e.g. 'BBVA_API_KEY'.")),
	), s.handleSecretGet)

	s.srv.AddTool(mcp.NewTool("secret_list",
		mcp.WithDescription("List vault keys + metadata (description, scope, expiry). Never returns plaintext."),
	), s.handleSecretList)

	s.srv.AddTool(mcp.NewTool("secret_set",
		mcp.WithDescription("Save or rotate a credential in the encrypted vault (AES-GCM). Use whenever the user shares an API key, token, password, or anything that should survive across sessions instead of being repeated. UPPER_SNAKE_CASE keys by convention. Idempotent: same key overwrites the previous value."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Vault key, UPPER_SNAKE_CASE (e.g. 'CLOUDFLARE_API_TOKEN', 'BBVA_API_KEY').")),
		mcp.WithString("value", mcp.Required(), mcp.Description("Plaintext value to encrypt and store.")),
		mcp.WithString("description", mcp.Description("Optional human-readable note (what scope it has, when it expires, where it came from).")),
		mcp.WithString("scope", mcp.Description("Optional scope: 'global' (default) | 'project:<id>' | 'agent:<name>'.")),
	), s.handleSecretSet)

	s.srv.AddTool(mcp.NewTool("secret_delete",
		mcp.WithDescription("Permanently remove a credential from the vault. Use when a token expires, gets rotated, or the user asks to forget it."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Vault key to delete.")),
	), s.handleSecretDelete)

	// ---------- system ----------
	s.srv.AddTool(mcp.NewTool("get_status",
		mcp.WithDescription("Returns a quick status snapshot of AgentHub."),
	), s.handleGetStatus)

	// ---------- system manager ----------
	s.srv.AddTool(mcp.NewTool("get_system_stats",
		mcp.WithDescription("Returns host CPU, RAM, disk, temperature, load and uptime."),
	), s.handleGetSystemStats)

	s.srv.AddTool(mcp.NewTool("list_services",
		mcp.WithDescription("Lists whitelisted systemd services with state and resource usage."),
	), s.handleListServices)

	s.srv.AddTool(mcp.NewTool("service_action",
		mcp.WithDescription("Performs start|stop|restart on a whitelisted systemd service."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Service unit name (e.g. 'cloudflared.service').")),
		mcp.WithString("action", mcp.Required(), mcp.Description("'start' | 'stop' | 'restart'.")),
	), s.handleServiceAction)

	s.srv.AddTool(mcp.NewTool("list_processes",
		mcp.WithDescription("Top-N host processes sorted by CPU (default) or memory."),
		mcp.WithNumber("top", mcp.Description("Max processes to return (default 10).")),
		mcp.WithString("sort", mcp.Description("'cpu' (default) | 'mem'.")),
	), s.handleListProcesses)

	s.srv.AddTool(mcp.NewTool("list_tunnels",
		mcp.WithDescription("Lists known tunneling daemons (cloudflared, etc.) and their state."),
	), s.handleListTunnels)

	s.srv.AddTool(mcp.NewTool("list_cronjobs",
		mcp.WithDescription("Lists system cron jobs from the user crontab, /etc/crontab, /etc/cron.d and periodic cron directories."),
	), s.handleListCronjobs)

	// ---------- mini-agents ----------
	s.srv.AddTool(mcp.NewTool("agent_create",
		mcp.WithDescription("Creates a mini-agent with a system prompt."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("system_prompt", mcp.Required()),
		mcp.WithString("engine", mcp.Description("'claude' (default) | 'codex' | 'ollama'.")),
		mcp.WithString("description"),
	), s.handleAgentCreate)

	s.srv.AddTool(mcp.NewTool("agent_list",
		mcp.WithDescription("Lists all mini-agents."),
	), s.handleAgentList)

	s.srv.AddTool(mcp.NewTool("agent_run_now",
		mcp.WithDescription("Runs a mini-agent immediately, optionally with an extra prompt appended to the system prompt."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("prompt", mcp.Description("Optional user prompt appended to the agent's system prompt.")),
	), s.handleAgentRunNow)

	s.srv.AddTool(mcp.NewTool("agent_logs",
		mcp.WithDescription("Returns recent runs of a mini-agent."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max runs to return (default 50).")),
	), s.handleAgentLogs)

	s.srv.AddTool(mcp.NewTool("agent_schedule",
		mcp.WithDescription("Attaches a cron schedule to a mini-agent."),
		mcp.WithString("name", mcp.Required()),
		mcp.WithString("cron_expr", mcp.Required(), mcp.Description("Standard 5-field cron expression (e.g. '0 9 * * *') or @every form.")),
		mcp.WithString("prompt_template", mcp.Description("Optional prompt appended to the agent's system prompt at each firing.")),
		mcp.WithString("notify_target", mcp.Description("'main-agent' | 'wa:<jid>' | 'topic:<name>' | 'none'.")),
	), s.handleAgentSchedule)

	s.srv.AddTool(mcp.NewTool("agent_pause",
		mcp.WithDescription("Disables a mini-agent (no scheduled runs)."),
		mcp.WithString("name", mcp.Required()),
	), s.handleAgentPause)

	s.srv.AddTool(mcp.NewTool("agent_resume",
		mcp.WithDescription("Re-enables a previously paused mini-agent."),
		mcp.WithString("name", mcp.Required()),
	), s.handleAgentResume)

	// ---------- harness (project tier) ----------
	s.srv.AddTool(mcp.NewTool("query_project_state",
		mcp.WithDescription("Read-only snapshot of a registered project: feature_list.json, harness state files (progress/current.md, progress/history.md, CHECKPOINTS.md), and existence flags for canonical docs (CLAUDE.md, AGENTS.md, DESIGN.md, SPECS.md, RELEASE_NOTES.md). Use this from the main-agent to ask 'how is project X going?' without spawning a subprocess inside the project."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or numeric id. Numeric strings are tried as id first, then as name.")),
	), s.handleQueryProjectState)

	s.srv.AddTool(mcp.NewTool("run_project_init",
		mcp.WithDescription("Run init.sh in a project's repo (the BettaTech harness validator: tests + build + smoke). Synchronous; returns exit_code, combined output, duration, and a timeout flag. Default timeout 5 min, ceiling 30 min."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or numeric id.")),
		mcp.WithNumber("timeout_s", mcp.Description("Wall-clock cap in seconds. Defaults to 300, ceiling 1800.")),
	), s.handleRunProjectInit)
}

// ---------- handlers ----------

type activeChatContext struct {
	Channel       string
	Engine        string
	Model         string
	WAJID         string
	WAReplyJID    string
	WAStanzaID    string
	Scope         string
	ProjectID     int64
	ProjectSessID int64
	TopicID       int64
	AgentName     string
	SessionID     string
}

func currentActiveChatContext() activeChatContext {
	return activeChatContext{
		Channel:       strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_CHANNEL")),
		Engine:        strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_ENGINE")),
		Model:         strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_MODEL")),
		WAJID:         strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_WA_JID")),
		WAReplyJID:    strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_WA_REPLY_JID")),
		WAStanzaID:    strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_WA_STANZA_ID")),
		Scope:         strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_SCOPE")),
		ProjectID:     parseInt64Env("AGENTHUB_ACTIVE_PROJECT_ID"),
		ProjectSessID: parseInt64Env("AGENTHUB_ACTIVE_PROJECT_SESS_ID"),
		TopicID:       parseInt64Env("AGENTHUB_ACTIVE_TOPIC_ID"),
		AgentName:     strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_AGENT_NAME")),
		SessionID:     strings.TrimSpace(os.Getenv("AGENTHUB_ACTIVE_SESSION_ID")),
	}
}

func parseInt64Env(key string) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (c activeChatContext) targetWAJID() string {
	if c.WAReplyJID != "" {
		return c.WAReplyJID
	}
	return c.WAJID
}

func (s *Server) uploadDir() string {
	dir := strings.TrimSpace(s.cfg.UploadDir)
	if dir == "" {
		dir = "data/uploads"
	}
	return dir
}

func (s *Server) stageOutgoingMedia(kind, rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("path required")
	}
	src, err := filepath.Abs(rawPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", rawPath)
	}

	base, err := filepath.Abs(s.uploadDir())
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(base, src); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return src, nil
	}

	if err := os.MkdirAll(filepath.Join(base, "tool_out"), 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(src)
	if ext == "" {
		ext = defaultMediaExt(kind)
	}
	name := sanitizeOutgoingName(strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)))
	if name == "" {
		name = "media"
	}
	dst := filepath.Join(base, "tool_out", fmt.Sprintf("%d-%s%s", time.Now().UTC().UnixNano(), name, ext))
	if err := linkOrCopyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func sanitizeOutgoingName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_.")
}

func defaultMediaExt(kind string) string {
	switch kind {
	case "image":
		return ".jpg"
	case "video":
		return ".mp4"
	case "voice", "audio":
		return ".ogg"
	case "document":
		return ".bin"
	default:
		return ""
	}
}

func linkOrCopyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	if info, err := os.Stat(dst); err == nil && !info.IsDir() {
		return nil
	}
	_ = os.Remove(dst)
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func (s *Server) sendMedia(ctx context.Context, kind, rawPath, caption, replyTo, jid string) (*mcp.CallToolResult, error) {
	stagedPath, err := s.stageOutgoingMedia(kind, rawPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jid = strings.TrimSpace(jid)
	replyTo = strings.TrimSpace(replyTo)
	caption = strings.TrimSpace(caption)
	active := currentActiveChatContext()

	if jid == "" {
		switch active.Channel {
		case "web":
			msg := store.Message{
				Channel:      "web",
				Direction:    "out",
				Body:         sqlStr(caption),
				MediaType:    sqlStr(kind),
				MediaPath:    sqlStr(stagedPath),
				MediaCaption: sqlStr(caption),
				TS:           time.Now().Unix(),
				Engine:       sqlStr(active.Engine),
				Model:        sqlStr(active.Model),
			}
			id, err := s.repos.Messages.Insert(ctx, msg)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("ok message_id=%d kind=%s channel=web", id, kind)), nil
		case "wa":
			targetJID := active.targetWAJID()
			if targetJID == "" {
				return mcp.NewToolResultError("jid required: no live WhatsApp target available"), nil
			}
			if replyTo == "" {
				replyTo = active.WAStanzaID
			}
			item := store.WaOutboxItem{
				JID:       targetJID,
				Kind:      kind,
				MediaPath: sqlStr(stagedPath),
				ReplyTo:   sqlStr(replyTo),
			}
			if caption != "" {
				item.Caption = sqlStr(caption)
			}
			outboxID, err := s.repos.WaOutbox.Enqueue(ctx, item)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			msg := store.Message{
				Channel:      "wa",
				Direction:    "out",
				JID:          sqlStr(targetJID),
				Body:         sqlStr(caption),
				MediaType:    sqlStr(kind),
				MediaPath:    sqlStr(stagedPath),
				MediaCaption: sqlStr(caption),
				ReplyTo:      sqlStr(replyTo),
				TS:           time.Now().Unix(),
				Engine:       sqlStr(active.Engine),
				Model:        sqlStr(active.Model),
				ExternalID:   sqlStr(fmt.Sprintf("outbox:%d", outboxID)),
			}
			msgID, err := s.repos.Messages.Insert(ctx, msg)
			if err != nil {
				return mcp.NewToolResultError("queued wa ok but failed to persist preview: " + err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("ok outbox_id=%d message_id=%d kind=%s channel=wa", outboxID, msgID, kind)), nil
		case "project", "topic", "agent", "mini-agent", "main":
			return s.persistSessionMedia(ctx, active, kind, stagedPath, caption)
		default:
			return mcp.NewToolResultError("jid required outside the live web/wa chat"), nil
		}
	}

	item := store.WaOutboxItem{
		JID:       jid,
		Kind:      kind,
		MediaPath: sqlStr(stagedPath),
		ReplyTo:   sqlStr(replyTo),
	}
	if caption != "" {
		item.Caption = sqlStr(caption)
	}
	id, err := s.repos.WaOutbox.Enqueue(ctx, item)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("queued id=%d kind=%s", id, kind)), nil
}

// persistSessionMedia stores an out-of-turn media drop on the active
// project/topic/agent session. The MCP server cannot reach the WS hub
// directly (it runs as a child process spawned by the CLI), so it writes
// the row and lets the parent daemon broadcast it once the turn finishes.
//
// The spawning daemon captures the latest session_messages id BEFORE the
// turn starts and replays every row above that baseline at the end —
// see broadcastProjectMessagesSince in internal/server/projects.go.
func (s *Server) persistSessionMedia(ctx context.Context, active activeChatContext, kind, stagedPath, caption string) (*mcp.CallToolResult, error) {
	scope := strings.TrimSpace(active.Scope)
	if scope == "" {
		// Fall back to channel for legacy spawns that did not set scope.
		switch active.Channel {
		case "project":
			scope = "project"
		case "topic":
			scope = "topic"
		case "agent", "mini-agent":
			scope = "agent"
		default:
			scope = "main"
		}
	}
	if scope == "mini-agent" {
		scope = "agent"
	}

	msg := store.SessionMessage{
		Scope:        scope,
		SessionID:    active.SessionID,
		Role:         "assistant",
		Body:         sqlStr(caption),
		MediaType:    sqlStr(kind),
		MediaPath:    sqlStr(stagedPath),
		MediaCaption: sqlStr(caption),
		TS:           time.Now().Unix(),
	}
	switch scope {
	case "project":
		if active.ProjectID == 0 || active.ProjectSessID == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("project session context missing (project_id=%d session=%d) — cannot deliver %s inline", active.ProjectID, active.ProjectSessID, kind)), nil
		}
		msg.ProjectID = sql.NullInt64{Int64: active.ProjectID, Valid: true}
		msg.ProjectSessID = sql.NullInt64{Int64: active.ProjectSessID, Valid: true}
	case "topic":
		if active.TopicID == 0 {
			return mcp.NewToolResultError("topic context missing — cannot deliver media inline"), nil
		}
		msg.TopicID = sql.NullInt64{Int64: active.TopicID, Valid: true}
	case "agent":
		if strings.TrimSpace(active.AgentName) == "" && active.SessionID == "" {
			return mcp.NewToolResultError("agent context missing — cannot deliver media inline"), nil
		}
	}

	id, err := s.repos.Sessions.AppendMessage(ctx, msg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("ok session_message_id=%d kind=%s scope=%s", id, kind, scope)), nil
}

func (s *Server) handleSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	// Always persist as a web 'out' so the UI sees it.
	id, err := s.repos.Messages.Insert(ctx, store.Message{
		Channel:   "web",
		Direction: "out",
		Body:      sqlStr(text),
		TS:        time.Now().Unix(),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	// When the agent supplies a jid, also queue an outgoing WA text — this
	// is how it 'replies' through the WhatsApp channel from inside an MCP
	// turn. The outbox worker dispatches it.
	if jid != "" {
		if _, oerr := s.repos.WaOutbox.Enqueue(ctx, store.WaOutboxItem{
			JID:     jid,
			Kind:    "text",
			Body:    sqlStr(text),
			ReplyTo: sqlStr(replyTo),
		}); oerr != nil {
			return mcp.NewToolResultError("queued web ok but wa enqueue failed: " + oerr.Error()), nil
		}
	}
	return mcp.NewToolResultText(fmt.Sprintf("ok message_id=%d", id)), nil
}

func (s *Server) handleSendImage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	caption := strings.TrimSpace(req.GetString("caption", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	return s.sendMedia(ctx, "image", path, caption, replyTo, jid)
}

func (s *Server) handleSendVoice(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	return s.sendMedia(ctx, "voice", path, "", replyTo, jid)
}

func (s *Server) handleSendAudio(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	return s.sendMedia(ctx, "audio", path, "", replyTo, jid)
}

func (s *Server) handleSendDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	caption := strings.TrimSpace(req.GetString("caption", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	return s.sendMedia(ctx, "document", path, caption, replyTo, jid)
}

func (s *Server) handleSendVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	jid := strings.TrimSpace(req.GetString("jid", ""))
	caption := strings.TrimSpace(req.GetString("caption", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	return s.sendMedia(ctx, "video", path, caption, replyTo, jid)
}

func (s *Server) handleSendLocation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jid, err := req.RequireString("jid")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	lat := req.GetFloat("lat", 0)
	lng := req.GetFloat("lng", 0)
	if lat == 0 && lng == 0 {
		return mcp.NewToolResultError("lat and lng required"), nil
	}
	name := strings.TrimSpace(req.GetString("name", ""))
	replyTo := strings.TrimSpace(req.GetString("reply_to", ""))
	id, err := s.repos.WaOutbox.Enqueue(ctx, store.WaOutboxItem{
		JID:     jid,
		Kind:    "location",
		LocLat:  sql.NullFloat64{Float64: lat, Valid: true},
		LocLng:  sql.NullFloat64{Float64: lng, Valid: true},
		LocName: sqlStr(name),
		ReplyTo: sqlStr(replyTo),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("queued id=%d kind=location", id)), nil
}

func (s *Server) handleRecentMessages(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 50)
	channel := req.GetString("channel", "")
	msgs, err := s.repos.Messages.Recent(ctx, channel, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(msgs)
}

func (s *Server) handleMarkRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := int64(req.GetInt("id", 0))
	if id == 0 {
		return mcp.NewToolResultError("id required"), nil
	}
	if err := s.repos.Messages.MarkRead(ctx, id); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText("ok"), nil
}

func (s *Server) handleRecord(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	data, err := req.RequireString("data")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := s.repos.Records.Insert(ctx, store.Record{Topic: topic, Data: data})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("ok record_id=%d", id)), nil
}

func (s *Server) handleQueryRecords(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topic, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	since := int64(req.GetInt("since", 0))
	limit := req.GetInt("limit", 50)
	recs, err := s.repos.Records.Query(ctx, topic, since, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(recs)
}

func (s *Server) handleListRecordTopics(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topics, err := s.repos.Records.ListTopics(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(topics)
}

func (s *Server) handleListTopics(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topics, err := s.repos.Topics.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(topics)
}

func (s *Server) handleReadTopicState(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	t, err := s.repos.Topics.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("topic not found: " + name), nil
	}
	state, err := s.repos.Topics.GetState(ctx, t.ID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"topic": t, "state": state})
}

func (s *Server) handleUpdateTopicState(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	t, err := s.repos.Topics.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("topic not found: " + name), nil
	}
	st := store.TopicState{TopicID: t.ID}
	if v := strings.TrimSpace(req.GetString("headline", "")); v != "" {
		st.Headline = sqlStr(v)
	}
	if v := strings.TrimSpace(req.GetString("active_issues", "")); v != "" {
		st.ActiveIssues = sqlStr(v)
	}
	if v := strings.TrimSpace(req.GetString("recent_decisions", "")); v != "" {
		st.RecentDecisions = sqlStr(v)
	}
	if v := strings.TrimSpace(req.GetString("pending", "")); v != "" {
		st.Pending = sqlStr(v)
	}
	if v := strings.TrimSpace(req.GetString("next_action_hint", "")); v != "" {
		st.NextActionHint = sqlStr(v)
	}
	if err := s.repos.Topics.UpsertState(ctx, st); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	_ = s.repos.Topics.Touch(ctx, t.ID)
	return mcp.NewToolResultText("ok"), nil
}

func (s *Server) handleTopicCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return mcp.NewToolResultError("name required"), nil
	}
	if existing, _ := s.repos.Topics.GetByName(ctx, name); existing != nil {
		return mcp.NewToolResultError(fmt.Sprintf("topic %q already exists", name)), nil
	}
	engine := strings.TrimSpace(req.GetString("engine", ""))
	if engine == "" {
		engine = "claude"
	}
	t := store.Topic{
		Name:      name,
		Engine:    engine,
		CreatedAt: time.Now().Unix(),
	}
	if d := strings.TrimSpace(req.GetString("description", "")); d != "" {
		t.Description = sqlStr(d)
	}
	if k := strings.TrimSpace(req.GetString("keywords", "")); k != "" {
		t.Keywords = sqlStr(k)
	}
	id, err := s.repos.Topics.Create(ctx, t)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"id": id, "name": name, "engine": engine})
}

func (s *Server) handleRunInTopic(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.engines == nil {
		return mcp.NewToolResultError("cliengine not wired"), nil
	}
	name, err := req.RequireString("topic")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	message, err := req.RequireString("message")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	t, err := s.repos.Topics.GetByName(ctx, name)
	if err != nil || t == nil {
		return mcp.NewToolResultError("topic not found: " + name), nil
	}

	prevSID := t.SessionID
	res, err := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    message,
		SessionID: prevSID,
		Channel:   "topic",
		Cwd:       ".",
		Engine:    t.Engine,
		Scope:     "topic",
		TopicID:   t.ID,
	})
	if err != nil {
		return mcp.NewToolResultError("run_in_topic: " + err.Error()), nil
	}
	// Auto-persist the session_id when the topic was fresh OR when the engine
	// returned a different id (rare, but supported).
	if res.SessionID != "" && res.SessionID != prevSID {
		if err := s.repos.Topics.SetSessionID(ctx, t.ID, res.SessionID); err != nil {
			// non-fatal: log via the result but don't fail the whole call
			return jsonResult(map[string]any{
				"topic":      name,
				"session_id": res.SessionID,
				"reply":      res.Text,
				"warning":    "could not persist session_id: " + err.Error(),
			})
		}
	}
	_ = s.repos.Topics.Touch(ctx, t.ID)
	return jsonResult(map[string]any{
		"topic":      name,
		"session_id": res.SessionID,
		"reply":      res.Text,
		"tokens":     res.CostTokens,
	})
}

func (s *Server) handleSecretGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return mcp.NewToolResultError("key required"), nil
	}
	enc, err := s.repos.Secrets.GetValue(ctx, key)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if enc == nil {
		return mcp.NewToolResultError("not found: " + key), nil
	}
	plain, err := auth.DecryptAESGCM(s.cfg.SecretKey, enc)
	if err != nil {
		return mcp.NewToolResultError("decrypt: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(plain)), nil
}

func (s *Server) handleSecretList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rows, err := s.repos.Secrets.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, sec := range rows {
		entry := map[string]any{
			"key":        sec.Key,
			"scope":      sec.Scope,
			"updated_at": sec.UpdatedAt,
		}
		if sec.Description.Valid {
			entry["description"] = sec.Description.String
		}
		if sec.ExpiresAt.Valid {
			entry["expires_at"] = sec.ExpiresAt.Int64
		}
		out = append(out, entry)
	}
	return jsonResult(out)
}

func (s *Server) handleSecretSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return mcp.NewToolResultError("key required"), nil
	}
	value, err := req.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if value == "" {
		return mcp.NewToolResultError("value required"), nil
	}
	description := strings.TrimSpace(req.GetString("description", ""))
	scope := strings.TrimSpace(req.GetString("scope", ""))
	if scope == "" {
		scope = "global"
	}

	enc, err := auth.EncryptAESGCM(s.cfg.SecretKey, []byte(value))
	if err != nil {
		return mcp.NewToolResultError("encrypt: " + err.Error()), nil
	}

	sec := store.Secret{
		Key:      key,
		ValueEnc: enc,
		Scope:    scope,
	}
	if description != "" {
		sec.Description = sql.NullString{String: description, Valid: true}
	}
	if err := s.repos.Secrets.Upsert(ctx, sec); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"ok":  true,
		"key": key,
	})
}

func (s *Server) handleSecretDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return mcp.NewToolResultError("key required"), nil
	}
	if err := s.repos.Secrets.Delete(ctx, key); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"ok":  true,
		"key": key,
	})
}

func (s *Server) handleGetStatus(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out := map[string]any{
		"agent":      "agenthub",
		"version":    "0.1.0",
		"wa_enabled": s.cfg.WAEnabled,
		"engine":     s.cfg.DefaultEngine,
		"model":      s.cfg.DefaultModel,
		"ts":         time.Now().Unix(),
	}
	return jsonResult(out)
}

// ---------- system manager handlers ----------

func (s *Server) handleGetSystemStats(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	st, err := s.sysman.Stats(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(st)
}

func (s *Server) handleListServices(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	svcs, err := s.sysman.Services(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(svcs)
}

func (s *Server) handleServiceAction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	action, err := req.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.sysman.ServiceAction(ctx, name, action); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("ok %s %s", action, name)), nil
}

func (s *Server) handleListProcesses(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	top := req.GetInt("top", 10)
	sortBy := req.GetString("sort", "cpu")
	procs, err := s.sysman.Processes(ctx, top, sortBy)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(procs)
}

func (s *Server) handleListTunnels(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	conns, err := s.sysman.Connections(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(conns.Tunnels)
}

func (s *Server) handleListCronjobs(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.sysman == nil {
		return mcp.NewToolResultError("system manager not wired"), nil
	}
	listing, err := s.sysman.CronJobs(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(listing)
}

// ---------- mini-agent handlers ----------

func (s *Server) handleAgentCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	systemPrompt, err := req.RequireString("system_prompt")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	engine := strings.TrimSpace(req.GetString("engine", ""))
	if engine == "" {
		engine = s.cfg.DefaultEngine
	}
	desc := req.GetString("description", "")
	id, err := s.repos.Agents.Create(ctx, store.Agent{
		Name:         name,
		Description:  sqlStr(desc),
		SystemPrompt: systemPrompt,
		Engine:       engine,
		Enabled:      true,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"id": id, "name": name, "engine": engine})
}

func (s *Server) handleAgentList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agents, err := s.repos.Agents.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(agents)
}

func (s *Server) handleAgentRunNow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.engines == nil {
		return mcp.NewToolResultError("cli engines not wired"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	extraPrompt := req.GetString("prompt", "")
	agent, err := s.repos.Agents.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("agent not found: " + name), nil
	}
	prompt := strings.TrimSpace(agent.SystemPrompt)
	if t := strings.TrimSpace(extraPrompt); t != "" {
		if prompt != "" {
			prompt += "\n\n"
		}
		prompt += t
	}
	startedAt := time.Now().Unix()
	runID, err := s.repos.Agents.InsertRun(ctx, store.AgentRun{
		AgentID:   agent.ID,
		Trigger:   "manual",
		StartedAt: startedAt,
		Status:    "running",
		Prompt:    prompt,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	res, runErr := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    prompt,
		Channel:   "agent",
		Cwd:       ".",
		Engine:    agent.Engine,
		Scope:     "agent",
		AgentName: agent.Name,
	})
	status := "ok"
	result := ""
	tokens := int64(0)
	errStr := ""
	if runErr != nil {
		status = "error"
		errStr = runErr.Error()
	} else if res != nil {
		result = res.Text
		tokens = res.CostTokens
	}
	if ferr := s.repos.Agents.FinishRun(ctx, runID, status, result, tokens, errStr); ferr != nil {
		return mcp.NewToolResultError(ferr.Error()), nil
	}
	return jsonResult(map[string]any{
		"run_id": runID,
		"status": status,
		"result": result,
		"tokens": tokens,
		"error":  errStr,
	})
}

func (s *Server) handleAgentLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := req.GetInt("limit", 50)
	agent, err := s.repos.Agents.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("agent not found: " + name), nil
	}
	runs, err := s.repos.Agents.RunsForAgent(ctx, agent.ID, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(runs)
}

func (s *Server) handleAgentSchedule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	expr, err := req.RequireString("cron_expr")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	tmpl := req.GetString("prompt_template", "")
	notify := req.GetString("notify_target", "none")
	agent, err := s.repos.Agents.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("agent not found: " + name), nil
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	parsed, err := parser.Parse(expr)
	if err != nil {
		return mcp.NewToolResultError("invalid cron_expr: " + err.Error()), nil
	}
	next := parsed.Next(time.Now()).Unix()
	id, err := s.repos.Agents.AddSchedule(ctx, store.AgentSchedule{
		AgentID:        agent.ID,
		CronExpr:       expr,
		PromptTemplate: tmpl,
		NotifyTarget:   notify,
		Enabled:        true,
		NextRun:        next,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"schedule_id": id,
		"agent_id":    agent.ID,
		"cron_expr":   expr,
		"next_run":    next,
	})
}

func (s *Server) handleAgentPause(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.toggleAgent(ctx, req, false)
}

func (s *Server) handleAgentResume(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.toggleAgent(ctx, req, true)
}

func (s *Server) toggleAgent(ctx context.Context, req mcp.CallToolRequest, enabled bool) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	agent, err := s.repos.Agents.GetByName(ctx, name)
	if err != nil {
		return mcp.NewToolResultError("agent not found: " + name), nil
	}
	if err := s.repos.Agents.SetEnabled(ctx, agent.ID, enabled); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	state := "paused"
	if enabled {
		state = "resumed"
	}
	return mcp.NewToolResultText(fmt.Sprintf("agent %s %s", name, state)), nil
}

// resolveProject looks up a project by numeric id (preferred) or by name. The
// input is the user-facing string given to the MCP tool — we tolerate both
// forms because the LLM doesn't always know which one it has.
func (s *Server) resolveProject(ctx context.Context, ref string) (*store.Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("project required")
	}
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		if p, err := s.repos.Projects.GetByID(ctx, id); err == nil {
			return p, nil
		}
	}
	p, err := s.repos.Projects.GetByName(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("project not found: %s", ref)
	}
	return p, nil
}

// canonicalDocsForProject mirrors the keys served by /api/projects/{id}/docs
// so the LLM gets a consistent picture across HTTP and MCP. We only stat —
// content is left to a follow-up tool to keep the response small.
var canonicalDocsForProject = map[string]string{
	"claude":  ".claude/CLAUDE.md",
	"specs":   "SPECS.md",
	"design":  "DESIGN.md",
	"agents":  "AGENTS.md",
	"release": "RELEASE_NOTES.md",
}

func (s *Server) handleQueryProjectState(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ref, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := s.resolveProject(ctx, ref)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	out := map[string]any{
		"project": map[string]any{
			"id":   project.ID,
			"name": project.Name,
			"path": project.Path,
		},
		"state": harness.ReadAllState(project.Path, harness.FileMaxBytes),
	}

	// Feature list — tolerate missing/invalid file by surfacing the error
	// instead of failing the whole tool. The caller is the LLM; it can decide
	// what to do.
	flPath := filepath.Join(project.Path, harness.FeatureListFile)
	if raw, err := os.ReadFile(flPath); err == nil {
		fl, perr := harness.ParseFeatureList(raw)
		if perr != nil {
			out["features"] = map[string]any{"exists": true, "error": perr.Error()}
		} else {
			out["features"] = map[string]any{
				"exists":     true,
				"version":    fl.Version,
				"updated_at": fl.UpdatedAt,
				"items":      fl.Features,
			}
		}
	} else if os.IsNotExist(err) {
		out["features"] = map[string]any{"exists": false, "items": []harness.FeatureItem{}}
	} else {
		out["features"] = map[string]any{"exists": false, "error": err.Error()}
	}

	docs := make([]map[string]any, 0, len(canonicalDocsForProject))
	for slug, rel := range canonicalDocsForProject {
		_, statErr := os.Stat(filepath.Join(project.Path, rel))
		docs = append(docs, map[string]any{
			"doc":    slug,
			"path":   rel,
			"exists": statErr == nil,
		})
	}
	out["docs"] = docs

	return jsonResult(out)
}

func (s *Server) handleRunProjectInit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ref, err := req.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	project, err := s.resolveProject(ctx, ref)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	initSh := filepath.Join(project.Path, "init.sh")
	info, err := os.Stat(initSh)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultError("init.sh missing — scaffold the harness first"), nil
		}
		return mcp.NewToolResultError(err.Error()), nil
	}
	if info.Mode().Perm()&0o111 == 0 {
		return mcp.NewToolResultError("init.sh exists but is not executable"), nil
	}

	timeout := harness.InitDefaultTimeout
	if n := req.GetInt("timeout_s", 0); n > 0 {
		timeout = time.Duration(n) * time.Second
	}
	if timeout > harness.InitMaxTimeout {
		timeout = harness.InitMaxTimeout
	}

	res := harness.RunInit(ctx, project.Path, []string{
		"AGENTHUB_HARNESS=1",
		"AGENTHUB_PROJECT_ID=" + strconv.FormatInt(project.ID, 10),
		"AGENTHUB_PROJECT_NAME=" + project.Name,
	}, timeout, harness.InitOutputCap)

	return jsonResult(res)
}

// ---------- helpers ----------

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func sqlStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
