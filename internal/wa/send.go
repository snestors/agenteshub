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

// SendImage uploads + sends an image. `caption` is optional. The MIME type is
// sniffed from the bytes; falls back to image/jpeg.
func (c *Client) SendImage(ctx context.Context, jid, path, caption string) error {
	return c.sendMedia(ctx, jid, path, caption, mediaKindImage)
}

// SendVoice sends a voice note (PTT=true). The file should be opus-encoded
// .ogg for best compatibility. WhatsApp will reject other codecs silently.
func (c *Client) SendVoice(ctx context.Context, jid, path string) error {
	return c.sendMedia(ctx, jid, path, "", mediaKindVoice)
}

// SendAudio sends a music/audio file (PTT=false).
func (c *Client) SendAudio(ctx context.Context, jid, path string) error {
	return c.sendMedia(ctx, jid, path, "", mediaKindAudio)
}

// SendDocument sends an arbitrary file as a document attachment. `caption`
// optional. The displayed filename is the basename of `path`.
func (c *Client) SendDocument(ctx context.Context, jid, path, caption string) error {
	return c.sendMedia(ctx, jid, path, caption, mediaKindDocument)
}

// SendVideo sends a video file. `caption` optional.
func (c *Client) SendVideo(ctx context.Context, jid, path, caption string) error {
	return c.sendMedia(ctx, jid, path, caption, mediaKindVideo)
}

// SendLocation sends a static location pin. `name` is the place label
// shown next to the map; pass "" for none.
func (c *Client) SendLocation(ctx context.Context, jid string, lat, lng float64, name string) error {
	if !c.Connected() {
		return errors.New("wa not connected")
	}
	parsed, err := parseJID(jid)
	if err != nil {
		return err
	}
	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(lat),
		DegreesLongitude: proto.Float64(lng),
	}
	if strings.TrimSpace(name) != "" {
		loc.Name = proto.String(name)
	}
	_, err = c.wmClient.SendMessage(ctx, parsed, &waE2E.Message{LocationMessage: loc})
	return err
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
// MediaType + builds the right *waE2E.Message variant.
func (c *Client) sendMedia(ctx context.Context, jid, path, caption string, kind mediaKind) error {
	if !c.Connected() {
		return errors.New("wa not connected")
	}
	parsed, err := parseJID(jid)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if len(data) == 0 {
		return errors.New("empty file")
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
		return fmt.Errorf("unknown media kind %d", kind)
	}

	uploaded, err := c.wmClient.Upload(ctx, data, appInfo)
	if err != nil {
		return fmt.Errorf("wa upload: %w", err)
	}
	size := uint64(len(data))

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
	}
	_, err = c.wmClient.SendMessage(ctx, parsed, msg)
	return err
}
