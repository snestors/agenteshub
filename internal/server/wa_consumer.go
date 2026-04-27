package server

import (
	"context"
	"strings"
	"time"

	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/wa"
)

// StartWAConsumer drains the wa.Client incoming queue and feeds each message
// into the SAME pipeline the web chat uses (acceptMessage → runMainAgentTurn).
// The web channel is the source of truth for the conversation; WA is just an
// I/O surface that mirrors the user's text on input and the agent's reply on
// output. The browser sees both directions in real time exactly like a
// regular chat turn.
//
// Per-message work:
//  1. MarkRead so the sender sees ✓✓ blue immediately
//  2. SendChatPresence "composing" so they see "typing…"
//  3. Persist the user text on messages(channel='web', direction='in') and
//     broadcast it on the WS hub, so the chat web UI shows the WA message
//  4. Run the main agent turn (inherits engine/model/session from settings)
//  5. Reply lands on wa_outbox via runMainAgentTurnTo(waReply=…)
//  6. SendChatPresence "paused" to clear typing
func (s *Server) StartWAConsumer(ctx context.Context, client *wa.Client) {
	if s == nil || client == nil {
		return
	}
	go s.waConsumeLoop(ctx, client)
	s.log.Info("wa consumer started (chat-web flow)")
}

func (s *Server) waConsumeLoop(ctx context.Context, client *wa.Client) {
	queue := client.Queue()
	for {
		select {
		case <-ctx.Done():
			return
		case in, ok := <-queue:
			if !ok {
				return
			}
			s.handleWAIncoming(ctx, client, in)
		}
	}
}

func (s *Server) handleWAIncoming(ctx context.Context, client *wa.Client, in wa.IncomingMessage) {
	// ✓✓ azul (best-effort — log and continue if it fails).
	readCtx, cancelRead := context.WithTimeout(ctx, 5*time.Second)
	if err := client.MarkRead(readCtx, in); err != nil {
		s.log.Debug("wa mark read", "id", in.ExternalID, "err", err)
	}
	cancelRead()

	body := strings.TrimSpace(in.Body)
	if body == "" {
		// Media-only or unhandled type — already persisted on wa_messages by
		// dispatchIncoming. Nothing to feed the agent.
		return
	}

	// Typing presence (best-effort).
	typingCtx, cancelTyping := context.WithTimeout(ctx, 3*time.Second)
	_ = client.StartTyping(typingCtx, in)
	cancelTyping()
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.StopTyping(stopCtx, in)
		stopCancel()
	}()

	// Mirror the user message on the unified chat (same channel='web' so the
	// browser sees it next to its own messages). The body is the raw text —
	// the agent treats it as a normal user turn, no system preamble.
	id, err := s.repos.Messages.Insert(ctx, store.Message{
		Channel:   "web",
		Direction: "in",
		Body:      sqlStr(body),
		TS:        time.Now().Unix(),
	})
	if err == nil {
		s.broadcastMessage(id, "web", "in", body)
	}

	// Engine + model from settings (same source the web chat uses).
	engine := s.cfg.DefaultEngine
	model := s.cfg.DefaultModel
	if v, _ := s.repos.Settings.Get(ctx, "engine"); v != "" {
		engine = v
	}
	if v, _ := s.repos.Settings.Get(ctx, "model"); v != "" {
		model = v
	}

	mainSess := s.mainAgentSession(ctx, engine)
	prev := ""
	if mainSess != nil {
		prev = mainSess.SessionID
	}

	// Use the resolved reply target. ReplyJID is set when sender is @lid; for
	// @s.whatsapp.net senders it equals JID.
	replyJID := in.ReplyJID
	if replyJID == "" {
		replyJID = in.JID
	}

	// If the user quoted a previous message, prepend its content as context so
	// the agent knows which message is being replied to.
	enginePrompt := body
	if in.QuotedID != "" {
		if quoted, err := s.repos.Messages.GetByExternalID(ctx, in.QuotedID); err == nil && quoted != nil {
			quotedBody := ""
			if quoted.Body.Valid {
				quotedBody = quoted.Body.String
			}
			if quotedBody != "" {
				enginePrompt = "[el user responde a este mensaje tuyo: \"" + quotedBody + "\"]\n\n" + body
			}
		}
	}

	go s.runMainAgentTurnTo(enginePrompt, prev, engine, model, &waReplyTarget{
		JID:      replyJID,
		StanzaID: in.ExternalID,
	})
}
