package wa

import (
	"errors"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// makeTextMessage wraps a plain string into a whatsmeow conversation message.
func makeTextMessage(text string) *waE2E.Message {
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
