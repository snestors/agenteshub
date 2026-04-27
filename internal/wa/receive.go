package wa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// dispatchIncoming is the second half of handleEvent: when the event is a
// real *events.Message we extract whichever payload it carries (text,
// image, audio, video, document, location), download media if any and push an IncomingMessage onto the queue. The
// server-side conversation router owns durable message persistence.
//
// Authorization is classified here: unauthorized senders still enter the
// queue so the router can persist/audit them, but the router will not run
// the agent for them.
func (c *Client) dispatchIncoming(ctx context.Context, evt *events.Message) {
	if evt == nil || evt.Message == nil {
		return
	}
	// Skip messages from ourselves.
	if evt.Info.IsFromMe {
		return
	}

	phone := evt.Info.Sender.User
	jid := evt.Info.Sender.String()
	authorized := c.IsAuthorized(phone)

	// Resolve the reply target. Whatsmeow rejects sends to JIDs that carry a
	// device part ("message recipient must be a user JID with no device
	// part"), so we always strip it. When the sender is on the linked-id
	// space (@lid) we additionally prefer SenderAlt (real @s.whatsapp.net
	// JID) which WhatsApp ships in the same event for direct chats.
	replyTarget := evt.Info.Sender
	if !evt.Info.SenderAlt.IsEmpty() {
		replyTarget = evt.Info.SenderAlt
	}
	replyTarget.Device = 0
	replyJID := replyTarget.String()

	in := IncomingMessage{
		JID:        jid,
		ReplyJID:   replyJID,
		Phone:      phone,
		Name:       evt.Info.PushName,
		TS:         evt.Info.Timestamp,
		ExternalID: evt.Info.ID,
		Authorized: authorized,
	}

	// Extract reply context if this message quotes another. WA exposes it
	// through *Message.ContextInfo.StanzaID — same structure across types.
	if rid := extractQuotedID(evt.Message); rid != "" {
		in.QuotedID = rid
	}

	// Pick the carrier that has actual content. WA wraps messages in
	// nested envelopes (ephemeral, view-once, etc.); we unwrap when the
	// outer one is just a wrapper.
	msg := unwrap(evt.Message)

	switch {
	case msg.GetConversation() != "" || msg.GetExtendedTextMessage() != nil:
		text := msg.GetConversation()
		if text == "" {
			text = msg.GetExtendedTextMessage().GetText()
		}
		in.Body = text
		in.IsCommand = strings.HasPrefix(strings.TrimSpace(text), "/")
	case msg.GetImageMessage() != nil:
		img := msg.GetImageMessage()
		path, err := c.downloadAndSave(ctx, img, evt.Info.ID, ".jpg")
		if err != nil {
			c.log.Warn("wa download image", "id", evt.Info.ID, "err", err)
		}
		in.MediaKind = "image"
		in.MediaPath = path
		in.Body = img.GetCaption()
	case msg.GetAudioMessage() != nil:
		au := msg.GetAudioMessage()
		ext := ".ogg"
		path, err := c.downloadAndSave(ctx, au, evt.Info.ID, ext)
		if err != nil {
			c.log.Warn("wa download audio", "id", evt.Info.ID, "err", err)
		}
		if au.GetPTT() {
			in.MediaKind = "voice"
		} else {
			in.MediaKind = "audio"
		}
		in.MediaPath = path
	case msg.GetVideoMessage() != nil:
		vid := msg.GetVideoMessage()
		path, err := c.downloadAndSave(ctx, vid, evt.Info.ID, ".mp4")
		if err != nil {
			c.log.Warn("wa download video", "id", evt.Info.ID, "err", err)
		}
		in.MediaKind = "video"
		in.MediaPath = path
		in.Body = vid.GetCaption()
	case msg.GetDocumentMessage() != nil:
		doc := msg.GetDocumentMessage()
		ext := filepath.Ext(doc.GetFileName())
		if ext == "" {
			ext = ".bin"
		}
		path, err := c.downloadAndSave(ctx, doc, evt.Info.ID, ext)
		if err != nil {
			c.log.Warn("wa download document", "id", evt.Info.ID, "err", err)
		}
		in.MediaKind = "document"
		in.MediaPath = path
		in.Body = doc.GetCaption()
	case msg.GetLocationMessage() != nil:
		loc := msg.GetLocationMessage()
		in.LocLat = loc.GetDegreesLatitude()
		in.LocLng = loc.GetDegreesLongitude()
		in.LocName = loc.GetName()
	default:
		// unhandled type (sticker, contact, etc.) — log + ignore
		c.log.Debug("wa unhandled message kind", "id", evt.Info.ID)
		return
	}

	if !authorized {
		c.log.Warn("wa unauthorized sender", "phone", phone)
	}
	select {
	case c.queue <- in:
	default:
		c.log.Warn("wa queue full, dropping message", "id", evt.Info.ID)
	}
}

// downloadAndSave decrypts the media and writes it under
// data/uploads/wa/<msg_id><ext>. Returns the absolute path.
func (c *Client) downloadAndSave(ctx context.Context, msg downloadable, msgID, ext string) (string, error) {
	if c.wmClient == nil {
		return "", fmt.Errorf("wa client nil")
	}
	data, err := c.wmClient.Download(ctx, msg)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(c.cfg.WADBPath), "uploads", "wa")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, msgID+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// downloadable is a tiny interface alias so both image/audio/video/document
// satisfy the same shape Download expects.
type downloadable interface {
	GetURL() string
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileEncSHA256() []byte
	GetFileSHA256() []byte
	GetFileLength() uint64
}

// extractQuotedID returns the StanzaID of the message being quoted, if any.
// Each media message type carries its own ContextInfo pointer so we have to
// peek at every variant. Returns "" when the message is not a reply.
func extractQuotedID(m *waE2E.Message) string {
	m = unwrap(m)
	if m == nil {
		return ""
	}
	if ext := m.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		return ext.GetContextInfo().GetStanzaID()
	}
	if img := m.GetImageMessage(); img != nil && img.GetContextInfo() != nil {
		return img.GetContextInfo().GetStanzaID()
	}
	if au := m.GetAudioMessage(); au != nil && au.GetContextInfo() != nil {
		return au.GetContextInfo().GetStanzaID()
	}
	if doc := m.GetDocumentMessage(); doc != nil && doc.GetContextInfo() != nil {
		return doc.GetContextInfo().GetStanzaID()
	}
	if vid := m.GetVideoMessage(); vid != nil && vid.GetContextInfo() != nil {
		return vid.GetContextInfo().GetStanzaID()
	}
	if loc := m.GetLocationMessage(); loc != nil && loc.GetContextInfo() != nil {
		return loc.GetContextInfo().GetStanzaID()
	}
	return ""
}

// unwrap peels off whatsmeow's container envelopes (ephemeral, view-once,
// device-sent) so the dispatcher sees the actual content.
func unwrap(m *waE2E.Message) *waE2E.Message {
	if m == nil {
		return nil
	}
	if e := m.GetEphemeralMessage(); e != nil && e.GetMessage() != nil {
		return unwrap(e.GetMessage())
	}
	if v := m.GetViewOnceMessage(); v != nil && v.GetMessage() != nil {
		return unwrap(v.GetMessage())
	}
	if v := m.GetViewOnceMessageV2(); v != nil && v.GetMessage() != nil {
		return unwrap(v.GetMessage())
	}
	if d := m.GetDeviceSentMessage(); d != nil && d.GetMessage() != nil {
		return unwrap(d.GetMessage())
	}
	return m
}
