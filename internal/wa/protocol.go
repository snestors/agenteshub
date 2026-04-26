package wa

import (
	"errors"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// makeTextMessage wraps a plain string into a whatsmeow conversation message.
// When `reply` is non-nil it returns an ExtendedTextMessage carrying the
// quote context — Conversation does not support ContextInfo.
func makeTextMessage(text string, reply *ReplyContext) *waE2E.Message {
	if ci := buildContextInfo(reply); ci != nil {
		return &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: ci,
			},
		}
	}
	return &waE2E.Message{Conversation: proto.String(text)}
}

// parseJID accepts "<phone>@s.whatsapp.net", "<phone>@c.us", "<phone>@lid", or
// a bare digit string and returns a valid types.JID.
func parseJID(in string) (types.JID, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return types.JID{}, errors.New("empty jid")
	}
	// digits-only → assume @s.whatsapp.net
	if !strings.Contains(in, "@") {
		return types.NewJID(in, types.DefaultUserServer), nil
	}
	jid, err := types.ParseJID(in)
	if err != nil {
		return types.JID{}, err
	}
	return jid, nil
}
