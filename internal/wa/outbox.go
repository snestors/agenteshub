package wa

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		if sentID != "" {
			archivedPath, aerr := c.archiveOutboxMedia(item, sentID)
			if aerr != nil {
				c.log.Warn("wa outbox archive media", "id", item.ID, "kind", item.Kind, "err", aerr)
			}
			if perr := c.persistSentOutboxItem(ctx, item, sentID, archivedPath); perr != nil {
				c.log.Warn("wa outbox persist sent", "id", item.ID, "kind", item.Kind, "err", perr)
			}
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
		return c.SendImage(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "voice":
		if !item.MediaPath.Valid {
			return "", errors.New("voice outbox item missing media_path")
		}
		return c.SendVoice(ctx, item.JID, item.MediaPath.String, reply)
	case "audio":
		if !item.MediaPath.Valid {
			return "", errors.New("audio outbox item missing media_path")
		}
		return c.SendAudio(ctx, item.JID, item.MediaPath.String, reply)
	case "document":
		if !item.MediaPath.Valid {
			return "", errors.New("document outbox item missing media_path")
		}
		return c.SendDocument(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "video":
		if !item.MediaPath.Valid {
			return "", errors.New("video outbox item missing media_path")
		}
		return c.SendVideo(ctx, item.JID, item.MediaPath.String, nsStr(item.Caption), reply)
	case "location":
		if !item.LocLat.Valid || !item.LocLng.Valid {
			return "", errors.New("location outbox item missing coords")
		}
		return c.SendLocation(ctx, item.JID, item.LocLat.Float64, item.LocLng.Float64, nsStr(item.LocName), reply)
	default:
		return "", errors.New("unknown kind: " + item.Kind)
	}
}

func (c *Client) persistSentOutboxItem(ctx context.Context, item store.WaOutboxItem, sentID, archivedPath string) error {
	msg := store.Message{
		Channel:    "wa",
		Direction:  "out",
		JID:        sql.NullString{String: item.JID, Valid: item.JID != ""},
		ExternalID: sql.NullString{String: sentID, Valid: sentID != ""},
		TS:         time.Now().Unix(),
	}

	switch item.Kind {
	case "text":
		msg.Body = item.Body
	case "image", "voice", "audio", "document", "video":
		msg.MediaType = sql.NullString{String: item.Kind, Valid: true}
		if archivedPath != "" {
			msg.MediaPath = sql.NullString{String: archivedPath, Valid: true}
		}
		if item.Caption.Valid && strings.TrimSpace(item.Caption.String) != "" {
			msg.Body = item.Caption
			msg.MediaCaption = item.Caption
		}
	case "location":
		msg.LocationLat = item.LocLat
		msg.LocationLng = item.LocLng
		msg.LocationName = item.LocName
		if item.LocName.Valid && strings.TrimSpace(item.LocName.String) != "" {
			msg.Body = item.LocName
		}
	default:
		return nil
	}

	_, err := c.repos.Messages.Insert(ctx, msg)
	return err
}

func (c *Client) archiveOutboxMedia(item store.WaOutboxItem, sentID string) (string, error) {
	if !item.MediaPath.Valid || strings.TrimSpace(item.MediaPath.String) == "" {
		return "", nil
	}

	uploadDir := strings.TrimSpace(c.cfg.UploadDir)
	if uploadDir == "" {
		uploadDir = "data/uploads"
	}
	base, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", err
	}
	src, err := filepath.Abs(item.MediaPath.String)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(base, src); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return src, nil
	}

	dir := filepath.Join(base, "wa_sent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(src)
	if ext == "" {
		ext = defaultMediaExt(item.Kind)
	}
	dst := filepath.Join(dir, sanitizeMediaID(sentID)+ext)
	if err := linkOrCopyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func sanitizeMediaID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return strconvUnix()
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if strings.Trim(out, "_") == "" {
		return strconvUnix()
	}
	return out
}

func strconvUnix() string {
	return time.Now().UTC().Format("20060102T150405")
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
