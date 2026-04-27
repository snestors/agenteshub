package server

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/wa"
)

// StartWAConsumer drains the wa.Client incoming queue and runs each message
// through the agent synchronously. WA is an independent I/O surface — it
// shares the same Claude session (and therefore the same memory/context) as
// the web chat, but the two channels never pollute each other's UI:
//
//   - WA messages are only stored in wa_messages(channel='wa') — already done
//     by dispatchIncoming before they reach this consumer.
//   - The agent reply is sent back via wa_outbox and stored in
//     wa_messages(channel='wa', direction='out'). It never touches the web WS.
//   - The web chat has its own pipeline (acceptMessage → runMainAgentTurn)
//     that only handles messages from the browser.
//
// One brain, two surfaces, zero cross-contamination.
//
// Per-message work:
//  1. MarkRead → ✓✓ blue
//  2. StartTyping → "escribiendo…" (lasts the full turn because the call is sync)
//  3. Build prompt (with quoted-reply context if any)
//  4. Run engine with the shared main-agent session
//  5. Enqueue reply on wa_outbox
//  6. StopTyping (deferred)
func (s *Server) StartWAConsumer(ctx context.Context, client *wa.Client) {
	if s == nil || client == nil {
		return
	}
	go s.waConsumeLoop(ctx, client)
	s.log.Info("wa consumer started")
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
	// 1. ✓✓ azul — best-effort.
	readCtx, cancelRead := context.WithTimeout(ctx, 5*time.Second)
	if err := client.MarkRead(readCtx, in); err != nil {
		s.log.Debug("wa mark read", "id", in.ExternalID, "err", err)
	}
	cancelRead()

	body := strings.TrimSpace(in.Body)
	if body == "" {
		return
	}

	// 2. Typing presence — start before the turn, stop when we return.
	//    Because handleWAIncoming is called synchronously from the consumer
	//    loop goroutine, the defer runs only after the engine finishes.
	typingCtx, cancelTyping := context.WithTimeout(ctx, 3*time.Second)
	_ = client.StartTyping(typingCtx, in)
	cancelTyping()
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.StopTyping(stopCtx, in)
		stopCancel()
	}()

	// 3. Build prompt — prepend quoted context when the user replied to a message.
	enginePrompt := body
	if in.QuotedID != "" {
		if quoted, _ := s.repos.Messages.GetByExternalID(ctx, in.QuotedID); quoted != nil && quoted.Body.Valid && quoted.Body.String != "" {
			enginePrompt = "[el user responde a este mensaje tuyo: \"" + quoted.Body.String + "\"]\n\n" + body
		}
	}

	// 4. Engine + session — same source as the web chat so both channels share
	//    the same Claude context (one brain, different surfaces).
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

	replyJID := in.ReplyJID
	if replyJID == "" {
		replyJID = in.JID
	}

	s.runWATurn(ctx, enginePrompt, prev, engine, model, in, replyJID)
}

// runWATurn executes a single WA turn synchronously. It:
//   - Runs the engine (blocks until the reply is ready)
//   - Persists the reply in wa_messages(channel='wa', direction='out')
//   - Enqueues the reply on wa_outbox for physical delivery
//   - Updates the shared session ID in agent_sessions
//
// It intentionally does NOT insert into channel='web' and does NOT broadcast
// on the WS hub — the web chat is a separate surface.
func (s *Server) runWATurn(ctx context.Context, prompt, prev, engine, model string, in wa.IncomingMessage, replyJID string) {
	s.runs.Inc("main")
	defer s.runs.Dec("main")

	turnCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	res, err := s.engines.Run(turnCtx, cliengine.RunOpts{
		Prompt:    prompt,
		SessionID: prev,
		Channel:   "wa",
		Cwd:       ".",
		Engine:    engine,
		Model:     model,
		Scope:     "main",
		AgentName: "main-agent",
	})
	if err != nil {
		// Daemon shutdown or context cancelled — don't spam the user with an
		// internal error message. The turn will be retried on next interaction.
		if errors.Is(err, context.Canceled) || errors.Is(turnCtx.Err(), context.Canceled) {
			s.log.Info("wa turn cancelled (shutdown)", "jid", in.JID)
			return
		}
		s.log.Warn("wa turn error", "jid", in.JID, "err", err)
		_, _ = s.repos.WaOutbox.Enqueue(context.Background(), store.WaOutboxItem{
			JID:  replyJID,
			Kind: "text",
			Body: sql.NullString{String: "⚠ " + err.Error(), Valid: true},
			ReplyTo: sql.NullString{
				String: in.ExternalID,
				Valid:  in.ExternalID != "",
			},
		})
		return
	}

	// Update shared session so the next turn (from web or WA) resumes context.
	if res.SessionID != "" && res.SessionID != prev {
		_ = s.repos.Sessions.UpsertAgentSession(context.Background(), store.AgentSession{
			AgentName: mainAgentSessionName(engine),
			Engine:    engine,
			SessionID: res.SessionID,
		})
	}

	reply := strings.TrimSpace(res.Text)
	if reply == "" {
		s.log.Info("wa engine produced empty reply", "jid", in.JID)
		return
	}

	// Persist outgoing WA message (channel='wa') — the external_id will be
	// filled later by the outbox worker via SendText.
	_, _ = s.repos.Messages.Insert(context.Background(), store.Message{
		Channel:   "wa",
		Direction: "out",
		JID:       sql.NullString{String: replyJID, Valid: true},
		Body:      sql.NullString{String: reply, Valid: true},
		Engine:    sql.NullString{String: engine, Valid: true},
		Model:     sql.NullString{String: model, Valid: true},
		TS:        time.Now().Unix(),
	})

	_, _ = s.repos.WaOutbox.Enqueue(context.Background(), store.WaOutboxItem{
		JID:  replyJID,
		Kind: "text",
		Body: sql.NullString{String: reply, Valid: true},
		ReplyTo: sql.NullString{
			String: in.ExternalID,
			Valid:  in.ExternalID != "",
		},
	})
}
