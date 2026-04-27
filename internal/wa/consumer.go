package wa

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/store"
)

// StartIncomingConsumer wakes the agent on every authorized message: marks the
// inbound message read (✓✓ blue), shows a typing presence, runs the configured
// engine and enqueues the reply on wa_outbox. Without this loop messages just
// pile up in the in-memory queue. Safe to call multiple times — only the first
// effective call wires the goroutine; the second is a no-op (cancellation lives
// in the parent ctx).
func (c *Client) StartIncomingConsumer(ctx context.Context, engines *cliengine.Manager) {
	if c == nil || engines == nil {
		return
	}
	go c.consumeLoop(ctx, engines)
	c.log.Info("wa incoming consumer started")
}

func (c *Client) consumeLoop(ctx context.Context, engines *cliengine.Manager) {
	// In-memory continuity: each JID gets a stable cliengine session id so the
	// model keeps context across messages. Lost on restart (no persistence
	// yet); the first message after a daemon restart starts a fresh thread.
	var (
		sessMu sync.Mutex
		sess   = map[string]string{}
	)
	getSess := func(jid string) string {
		sessMu.Lock()
		defer sessMu.Unlock()
		return sess[jid]
	}
	setSess := func(jid, sid string) {
		if sid == "" {
			return
		}
		sessMu.Lock()
		defer sessMu.Unlock()
		sess[jid] = sid
	}

	for {
		select {
		case <-ctx.Done():
			return
		case in, ok := <-c.queue:
			if !ok {
				return
			}
			c.processIncoming(ctx, engines, in, getSess, setSess)
		}
	}
}

func (c *Client) processIncoming(
	ctx context.Context,
	engines *cliengine.Manager,
	in IncomingMessage,
	getSess func(string) string,
	setSess func(string, string),
) {
	if c.wmClient == nil {
		c.log.Warn("wa consumer: wmClient nil")
		return
	}
	chat, err := types.ParseJID(in.JID)
	if err != nil {
		c.log.Warn("wa parse jid", "jid", in.JID, "err", err)
		return
	}

	// 1) Mark read so the sender sees ✓✓ blue.
	if in.ExternalID != "" {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := c.wmClient.MarkRead(readCtx, []types.MessageID{types.MessageID(in.ExternalID)}, time.Now(), chat, chat); err != nil {
			c.log.Debug("wa mark read", "id", in.ExternalID, "err", err)
		}
		cancel()
	}

	// 2) Skip media-only or empty messages — for now the agent only replies
	// to text. Persisted in wa_messages already, so nothing is lost.
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return
	}

	// 3) Typing presence (best-effort; ignore errors).
	typingCtx, cancelTyping := context.WithTimeout(ctx, 3*time.Second)
	_ = c.wmClient.SendChatPresence(typingCtx, chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	cancelTyping()
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = c.wmClient.SendChatPresence(stopCtx, chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
		stopCancel()
	}()

	// 4) Run the configured engine. The 5-minute deadline matches our usual
	// cliengine bound — long enough for tool calls but bounded so a stuck
	// turn doesn't hang the consumer goroutine.
	runCtx, runCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer runCancel()

	res, err := engines.Run(runCtx, cliengine.RunOpts{
		Prompt:    body,
		SessionID: getSess(in.JID),
		Channel:   "wa",
		Scope:     "topic",
		Engine:    c.cfg.DefaultEngine,
		Model:     c.cfg.DefaultModel,
		Cwd:       "/home/nestor",
	})
	if err != nil {
		c.log.Warn("wa engine run", "jid", in.JID, "err", err)
		_, _ = c.repos.WaOutbox.Enqueue(ctx, store.WaOutboxItem{
			JID:  in.JID,
			Kind: "text",
			Body: sql.NullString{String: "⚠ engine: " + err.Error(), Valid: true},
			ReplyTo: sql.NullString{
				String: in.ExternalID,
				Valid:  in.ExternalID != "",
			},
		})
		return
	}
	if res != nil {
		setSess(in.JID, res.SessionID)
	}
	reply := ""
	if res != nil {
		reply = strings.TrimSpace(res.Text)
	}
	if reply == "" {
		c.log.Info("wa engine produced empty reply", "jid", in.JID)
		return
	}
	if _, err := c.repos.WaOutbox.Enqueue(ctx, store.WaOutboxItem{
		JID:  in.JID,
		Kind: "text",
		Body: sql.NullString{String: reply, Valid: true},
		ReplyTo: sql.NullString{
			String: in.ExternalID,
			Valid:  in.ExternalID != "",
		},
	}); err != nil {
		c.log.Warn("wa outbox enqueue reply", "err", err)
	}
}
