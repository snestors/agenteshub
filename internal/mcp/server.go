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
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// Server wraps the MCP server bound to repos.
type Server struct {
	cfg   *config.Config
	repos *store.Repos
	srv   *server.MCPServer
}

// New constructs the MCP server with all tools registered.
func New(cfg *config.Config, repos *store.Repos) *Server {
	s := &Server{cfg: cfg, repos: repos}
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
	), s.handleSendMessage)

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

	// ---------- system ----------
	s.srv.AddTool(mcp.NewTool("get_status",
		mcp.WithDescription("Returns a quick status snapshot of AgentHub."),
	), s.handleGetStatus)
}

// ---------- handlers ----------

func (s *Server) handleSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	// In v0 we just persist as channel='web' direction='out' to surface in the UI.
	// At connect time the WA writer (when WAEnabled) will also broadcast to the
	// active jid. Here we route via DB only, which is what the chat UI consumes.
	id, err := s.repos.Messages.Insert(ctx, store.Message{
		Channel:   "web",
		Direction: "out",
		Body:      sqlStr(text),
		TS:        time.Now().Unix(),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("ok message_id=%d", id)), nil
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
