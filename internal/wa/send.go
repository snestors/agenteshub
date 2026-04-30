package wa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// ReplyContext identifies a previous WhatsApp message to quote in a reply.
// Both fields come from a previously received message (Info.ID and
// Info.Sender.String() in whatsmeow's events.Message).
type ReplyContext struct {
	StanzaID    string // wa external message id we're quoting
	Participant string // JID of the original sender (chat partner in 1-to-1)
}

func buildContextInfo(reply *ReplyContext) *waE2E.ContextInfo {
	if reply == nil || strings.TrimSpace(reply.StanzaID) == "" {
		return nil
	}
	ci := &waE2E.ContextInfo{StanzaID: proto.String(reply.StanzaID)}
	if strings.TrimSpace(reply.Participant) != "" {
		ci.Participant = proto.String(reply.Participant)
	}
	// QuotedMessage required by some clients; an empty conversation works as a stub.
	ci.QuotedMessage = &waE2E.Message{Conversation: proto.String("")}
	return ci
}

// SendImage uploads + sends an image. Returns the WhatsApp stanza id.
// `caption` is optional. The MIME type is sniffed from the bytes; falls back
// to image/jpeg. Pass `reply` to quote a previous message; pass nil for a
// non-reply send.
func (c *Client) SendImage(ctx context.Context, jid, path, caption string, reply *ReplyContext) (string, error) {
	return c.sendMedia(ctx, jid, path, caption, mediaKindImage, reply)
}

// SendVoice sends a voice note (PTT=true). The file should be opus-encoded
// .ogg for best compatibility. WhatsApp will reject other codecs silently.
func (c *Client) SendVoice(ctx context.Context, jid, path string, reply *ReplyContext) (string, error) {
	return c.sendMedia(ctx, jid, path, "", mediaKindVoice, reply)
}

// SendAudio sends a music/audio file (PTT=false).
func (c *Client) SendAudio(ctx context.Context, jid, path string, reply *ReplyContext) (string, error) {
	return c.sendMedia(ctx, jid, path, "", mediaKindAudio, reply)
}

// SendDocument sends an arbitrary file as a document attachment. `caption`
// optional. The displayed filename is the basename of `path`.
func (c *Client) SendDocument(ctx context.Context, jid, path, caption string, reply *ReplyContext) (string, error) {
	return c.sendMedia(ctx, jid, path, caption, mediaKindDocument, reply)
}

// SendVideo sends a video file and returns the WhatsApp stanza id.
// `caption` optional.
func (c *Client) SendVideo(ctx context.Context, jid, path, caption string, reply *ReplyContext) (string, error) {
	return c.sendMedia(ctx, jid, path, caption, mediaKindVideo, reply)
}

// SendLocation sends a static location pin. `name` is the place label
// shown next to the map; pass "" for none. Returns the WhatsApp stanza id.
func (c *Client) SendLocation(ctx context.Context, jid string, lat, lng float64, name string, reply *ReplyContext) (string, error) {
	if !c.Connected() {
		return "", errors.New("wa not connected")
	}
	parsed, err := parseJID(jid)
	if err != nil {
		return "", err
	}
	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(lat),
		DegreesLongitude: proto.Float64(lng),
	}
	if strings.TrimSpace(name) != "" {
		loc.Name = proto.String(name)
	}
	if ci := buildContextInfo(reply); ci != nil {
		loc.ContextInfo = ci
	}
	resp, err := c.wmClient.SendMessage(ctx, parsed, &waE2E.Message{LocationMessage: loc})
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

type mediaKind int

const (
	mediaKindImage mediaKind = iota
	mediaKindVoice
	mediaKindAudio
	mediaKindDocument
	mediaKindVideo
)

// sendMedia is the shared upload + envelope path. Each kind picks its
// MediaType + builds the right *waE2E.Message variant. Returns the WhatsApp
// stanza id. `reply` is optional.
func (c *Client) sendMedia(ctx context.Context, jid, path, caption string, kind mediaKind, reply *ReplyContext) (string, error) {
	if !c.Connected() {
		return "", errors.New("wa not connected")
	}
	parsed, err := parseJID(jid)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if len(data) == 0 {
		return "", errors.New("empty file")
	}
	mtype := http.DetectContentType(data)

	var appInfo whatsmeow.MediaType
	switch kind {
	case mediaKindImage:
		appInfo = whatsmeow.MediaImage
	case mediaKindVoice, mediaKindAudio:
		appInfo = whatsmeow.MediaAudio
	case mediaKindDocument:
		appInfo = whatsmeow.MediaDocument
	case mediaKindVideo:
		appInfo = whatsmeow.MediaVideo
	default:
		return "", fmt.Errorf("unknown media kind %d", kind)
	}

	uploaded, err := c.wmClient.Upload(ctx, data, appInfo)
	if err != nil {
		return "", fmt.Errorf("wa upload: %w", err)
	}
	size := uint64(len(data))

	ci := buildContextInfo(reply)
	msg := &waE2E.Message{}
	switch kind {
	case mediaKindImage:
		// override mime if generic
		if !strings.HasPrefix(mtype, "image/") {
			mtype = "image/jpeg"
		}
		msg.ImageMessage = &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mtype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(size),
		}
		if strings.TrimSpace(caption) != "" {
			msg.ImageMessage.Caption = proto.String(caption)
		}
		if ci != nil {
			msg.ImageMessage.ContextInfo = ci
		}
	case mediaKindVoice, mediaKindAudio:
		if !strings.HasPrefix(mtype, "audio/") {
			mtype = "audio/ogg; codecs=opus"
		}
		msg.AudioMessage = &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mtype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(size),
			PTT:           proto.Bool(kind == mediaKindVoice),
		}
		if ci != nil {
			msg.AudioMessage.ContextInfo = ci
		}
	case mediaKindDocument:
		fname := filepath.Base(path)
		msg.DocumentMessage = &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mtype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(size),
			FileName:      proto.String(fname),
			Title:         proto.String(fname),
		}
		if strings.TrimSpace(caption) != "" {
			msg.DocumentMessage.Caption = proto.String(caption)
		}
		if ci != nil {
			msg.DocumentMessage.ContextInfo = ci
		}
	case mediaKindVideo:
		if !strings.HasPrefix(mtype, "video/") {
			mtype = "video/mp4"
		}
		msg.VideoMessage = &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mtype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(size),
		}
		if strings.TrimSpace(caption) != "" {
			msg.VideoMessage.Caption = proto.String(caption)
		}
		if ci != nil {
			msg.VideoMessage.ContextInfo = ci
		}
	}
	resp, err := c.wmClient.SendMessage(ctx, parsed, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
