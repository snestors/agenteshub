package wa

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/snestors/agenteshub/internal/store"
)

func nsStr(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

// StartOutboxWorker drains the wa_outbox table every ~500ms and dispatches
// each pending item via the corresponding Send* function. Items are marked
// 'sent' or 'error' (with the error message) so the agent can audit later.
//
// No-op when wa is disabled. Runs until ctx is cancelled.
func (c *Client) StartOutboxWorker(ctx context.Context) {
	if !c.cfg.WAEnabled {
		c.log.Info("wa outbox worker skipped (wa disabled)")
		return
	}
	go c.outboxLoop(ctx)
}

func (c *Client) outboxLoop(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	c.log.Info("wa outbox worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !c.Connected() {
				continue
			}
			c.drainOnce(ctx)
		}
	}
}

func (c *Client) drainOnce(ctx context.Context) {
	items, err := c.repos.WaOutbox.PullPending(ctx, 10)
	if err != nil {
		c.log.Warn("wa outbox pull", "err", err)
		return
	}
	for _, item := range items {
		sentID, err := c.dispatchOutboxItem(ctx, item)
		if err != nil {
			_ = c.repos.WaOutbox.MarkError(ctx, item.ID, err.Error())
			c.log.Warn("wa outbox dispatch", "id", item.ID, "kind", item.Kind, "err", err)
			continue
		}
		_ = c.repos.WaOutbox.MarkSent(ctx, item.ID)
		// Persist the sent message in wa_messages so quoted replies can find it.
		if sentID != "" && item.Kind == "text" && item.Body.Valid {
			_, _ = c.repos.Messages.Insert(ctx, store.Message{
				Channel:    "wa",
				Direction:  "out",
				JID:        sql.NullString{String: item.JID, Valid: true},
				Body:       sql.NullString{String: item.Body.String, Valid: true},
				ExternalID: sql.NullString{String: sentID, Valid: true},
				TS:         time.Now().Unix(),
			})
		}
	}
}

func (c *Client) dispatchOutboxItem(ctx context.Context, item store.WaOutboxItem) (string, error) {
	if item.JID == "" {
		return "", errors.New("missing jid")
	}
	var reply *ReplyContext
	if item.ReplyTo.Valid && item.ReplyTo.String != "" {
		reply = &ReplyContext{
			StanzaID:    item.ReplyTo.String,
			Participant: nsStr(item.ReplyParticipant),
		}
		if reply.Participant == "" {
			reply.Participant = item.JID // 1-to-1 fallback
		}
	}
	switch item.Kind {
	case "text":
		if !item.Body.Valid || item.Body.String == "" {
			return "", errors.New("text outbox item missing body")
		}
		id, err := c.SendText(ctx, item.JID, item.Body.String, reply)
		return id, err
	case "image":
		if !item.MediaPath.Valid {
			return "", errors.New("image outbox item missing media_path")
		}
		return "", c.SendImage(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "voice":
		if !item.MediaPath.Valid {
			return "", errors.New("voice outbox item missing media_path")
		}
		return "", c.SendVoice(ctx, item.JID, item.MediaPath.String, reply)
	case "audio":
		if !item.MediaPath.Valid {
			return "", errors.New("audio outbox item missing media_path")
		}
		return "", c.SendAudio(ctx, item.JID, item.MediaPath.String, reply)
	case "document":
		if !item.MediaPath.Valid {
			return "", errors.New("document outbox item missing media_path")
		}
		return "", c.SendDocument(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "video":
		if !item.MediaPath.Valid {
			return "", errors.New("video outbox item missing media_path")
		}
		return "", c.SendVideo(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "location":
		if !item.LocLat.Valid || !item.LocLng.Valid {
			return "", errors.New("location outbox item missing coords")
		}
		return "", c.SendLocation(ctx, item.JID, item.LocLat.Float64, item.LocLng.Float64, nsStr(item.LocName), reply)
	default:
		return "", errors.New("unknown kind: " + item.Kind)
	}
}
