package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/store"
)

type conversationInput struct {
	Channel     string
	Body        string
	Attachments []msgAttachment
	Async       bool
	WA          *conversationWAInput
}

type conversationWAInput struct {
	JID        string
	ReplyJID   string
	MediaPath  string
	MediaKind  string
	LocLat     float64
	LocLng     float64
	LocName    string
	QuotedID   string
	ExternalID string
	TS         time.Time
	Authorized bool
}

type conversationTurn struct {
	Prompt     string
	PreviousID string
	Engine     string
	Model      string
	Source     string
	WAReply    *waReplyTarget
}

func (s *Server) routeConversationInput(ctx context.Context, in conversationInput) (sendMessageAccepted, error) {
	in.Channel = strings.TrimSpace(in.Channel)
	if in.Channel == "" {
		in.Channel = "web"
	}
	body := strings.TrimSpace(in.Body)
	hasMedia := in.WA != nil && (in.WA.MediaPath != "" || in.WA.MediaKind != "" || in.WA.LocLat != 0 || in.WA.LocLng != 0)
	if body == "" && !hasMedia {
		return sendMessageAccepted{}, errors.New("empty")
	}

	row := store.Message{
		Channel:   in.Channel,
		Direction: "in",
		Body:      sqlStr(in.Body),
		TS:        time.Now().Unix(),
	}
	if in.WA != nil {
		row.JID = sql.NullString{String: in.WA.JID, Valid: in.WA.JID != ""}
		row.MediaType = sql.NullString{String: in.WA.MediaKind, Valid: in.WA.MediaKind != ""}
		row.MediaPath = sql.NullString{String: in.WA.MediaPath, Valid: in.WA.MediaPath != ""}
		row.LocationLat = sql.NullFloat64{Float64: in.WA.LocLat, Valid: in.WA.LocLat != 0}
		row.LocationLng = sql.NullFloat64{Float64: in.WA.LocLng, Valid: in.WA.LocLng != 0}
		row.LocationName = sql.NullString{String: in.WA.LocName, Valid: in.WA.LocName != ""}
		row.ReplyTo = sql.NullString{String: in.WA.QuotedID, Valid: in.WA.QuotedID != ""}
		row.ExternalID = sql.NullString{String: in.WA.ExternalID, Valid: in.WA.ExternalID != ""}
		if !in.WA.TS.IsZero() {
			row.TS = in.WA.TS.Unix()
		}
	}

	id, err := s.repos.Messages.Insert(ctx, row)
	if err != nil {
		return sendMessageAccepted{}, err
	}
	s.broadcastMessage(id, in.Channel, "in", in.Body)

	// Unauthorized WA senders are stored and surfaced in Web, but they never
	// cross into the agent runtime.
	if in.WA != nil && !in.WA.Authorized {
		return sendMessageAccepted{ID: id, MessageID: id, Accepted: false}, nil
	}
	if body == "" {
		return sendMessageAccepted{ID: id, MessageID: id, Accepted: true}, nil
	}

	enginePrompt := in.Body + formatAttachments(in.Attachments)
	if in.WA != nil && in.WA.QuotedID != "" {
		if quoted, _ := s.repos.Messages.GetByExternalID(ctx, in.WA.QuotedID); quoted != nil && quoted.Body.Valid && quoted.Body.String != "" {
			enginePrompt = "[el user responde a este mensaje tuyo: \"" + quoted.Body.String + "\"]\n\n" + enginePrompt
		}
	}

	engine, model := s.currentEngineModel(ctx)

	// Slash commands bypass the engine entirely. The handler runs synchronously
	// (state changes hit the DB before we return) so the WA outbox / WS clients
	// see a clean response without spawning a CLI.
	if sr := s.tryHandleSlashCommand(ctx, body, slashScope{Kind: "main", Engine: engine}); sr.Handled {
		s.deliverSlashResponse(ctx, in, id, engine, model, sr)
		return sendMessageAccepted{ID: id, MessageID: id, Accepted: true, Engine: engine, Model: model}, nil
	}

	prev := ""
	if mainSess := s.mainAgentSession(ctx, engine, model); mainSess != nil {
		prev = mainSess.SessionID
	}
	turn := conversationTurn{Prompt: enginePrompt, PreviousID: prev, Engine: engine, Model: model, Source: in.Channel}
	if in.WA != nil {
		replyJID := in.WA.ReplyJID
		if replyJID == "" {
			replyJID = in.WA.JID
		}
		turn.WAReply = &waReplyTarget{JID: replyJID, StanzaID: in.WA.ExternalID}
	}
	if in.Async {
		go s.runConversationTurn(turn)
	} else {
		s.runConversationTurn(turn)
	}
	return sendMessageAccepted{ID: id, MessageID: id, Accepted: true, Engine: engine, Model: model}, nil
}

func (s *Server) runConversationTurn(turn conversationTurn) {
	s.runs.Inc("main")
	defer s.runs.Dec("main")
	// No deadline: long orchestrator chains were getting SIGTERMed mid-flight.
	// Instead a watcher (watchLongRunning) nags every hour while the turn is
	// alive; the user decides when to give up.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.runs.RegisterCancel("main", turn.Engine, cancel)
	defer s.runs.UnregisterCancel("main", turn.Engine)
	go s.watchLongRunning(ctx, "main", turn.Engine, "Main · "+turn.Engine, "Cancelá el turn desde el toast.")

	activity := &turnActivity{}
	res, err := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    turn.Prompt,
		SessionID: turn.PreviousID,
		Channel:   turn.Source,
		Cwd:       ".",
		Engine:    turn.Engine,
		Model:     turn.Model,
		Scope:     "main",
		AgentName: "main-agent",
		OnEvent:   s.streamEventBroadcasterWithActivity(activity),
	})
	if err != nil {
		if isShutdownError(ctx, err) {
			s.log.Info("conversation turn cancelled (shutdown)", "channel", turn.Source)
			return
		}
		s.log.Error("cliengine", "channel", turn.Source, "err", err)
		errBody := "⚠ engine: " + err.Error()
		s.emitConversationOutput(turn, errBody, "error", activity)
		s.broadcastNotification(Notification{Kind: "main_turn_failed", Severity: "error", Title: "Main · " + turn.Engine, Body: truncate(err.Error(), 280), Context: map[string]any{"engine": turn.Engine, "model": turn.Model, "channel": turn.Source}})
		return
	}
	if res.SessionID != "" && res.SessionID != turn.PreviousID {
		_ = s.repos.Sessions.UpsertAgentSession(context.Background(), store.AgentSession{AgentName: mainAgentSessionName(turn.Engine, turn.Model), Engine: turn.Engine, SessionID: res.SessionID})
	}
	s.emitConversationOutput(turn, res.Text, "ok", activity)
	go s.broadcastAgentStatus(context.Background())
	s.broadcastNotification(Notification{Kind: "main_turn_done", Severity: "info", Title: "Main · " + turn.Engine, Body: truncate(res.Text, 280), Context: map[string]any{"engine": turn.Engine, "model": turn.Model, "channel": turn.Source}})
}

// deliverSlashResponse surfaces the synthetic answer of a slash command on the
// same channel the user wrote from, reusing the normal output path so the
// message lands in DB + WS + WA outbox just like an engine reply would.
func (s *Server) deliverSlashResponse(ctx context.Context, in conversationInput, _ int64, engine, model string, sr slashResult) {
	body := sr.Response
	status := "ok"
	if sr.Error != "" {
		body = "⚠ " + sr.Error
		status = "error"
	}
	if strings.TrimSpace(body) == "" {
		return
	}
	turn := conversationTurn{Engine: engine, Model: model, Source: in.Channel}
	if in.WA != nil {
		jid := in.WA.ReplyJID
		if jid == "" {
			jid = in.WA.JID
		}
		turn.WAReply = &waReplyTarget{JID: jid, StanzaID: in.WA.ExternalID}
	}
	s.emitConversationOutput(turn, body, status, nil)
	_ = ctx
}

func (s *Server) emitConversationOutput(turn conversationTurn, body, status string, activity *turnActivity) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	if turn.Source == "wa" {
		s.enqueueWAReply(turn.WAReply, body)
		// Surface the outgoing WA reply live in Web. The WA outbox worker owns
		// durable WA persistence because it can attach the real external stanza id.
		s.broadcastMessageWithModel(0, "wa", "out", body, turn.Engine, turn.Model)
		return
	}

	activityJSON := ""
	if status == "ok" && activity != nil && (activity.Thinking != "" || len(activity.Tools) > 0) {
		if buf, err := json.Marshal(activity); err == nil {
			activityJSON = string(buf)
		}
	}
	outID, _ := s.repos.Messages.Insert(context.Background(), store.Message{
		Channel:   turn.Source,
		Direction: "out",
		Body:      sqlStr(body),
		TS:        time.Now().Unix(),
		Engine:    sqlStr(turn.Engine),
		Model:     sqlStr(turn.Model),
		Activity:  sqlStr(activityJSON),
	})
	s.broadcastMessageWithModel(outID, turn.Source, "out", body, turn.Engine, turn.Model)
}
