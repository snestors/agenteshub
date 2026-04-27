package server

import (
	"context"
	"strings"
	"time"

	"github.com/snestors/agenteshub/internal/wa"
)

// StartWAConsumer drains the wa.Client incoming queue and forwards each item
// into the shared conversation router. WhatsApp is now only an adapter: it
// handles channel-specific affordances (read receipt, typing presence, reply
// JID) and never runs the agent directly.
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
	readCtx, cancelRead := context.WithTimeout(ctx, 5*time.Second)
	if err := client.MarkRead(readCtx, in); err != nil {
		s.log.Debug("wa mark read", "id", in.ExternalID, "err", err)
	}
	cancelRead()

	body := strings.TrimSpace(in.Body)
	shouldType := in.Authorized && body != ""
	if shouldType {
		typingCtx, cancelTyping := context.WithTimeout(ctx, 3*time.Second)
		_ = client.StartTyping(typingCtx, in)
		cancelTyping()
		defer func() {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = client.StopTyping(stopCtx, in)
			stopCancel()
		}()
	}

	_, err := s.routeConversationInput(ctx, conversationInput{
		Channel: "wa",
		Body:    in.Body,
		Async:   false,
		WA: &conversationWAInput{
			JID:        in.JID,
			ReplyJID:   in.ReplyJID,
			MediaPath:  in.MediaPath,
			MediaKind:  in.MediaKind,
			LocLat:     in.LocLat,
			LocLng:     in.LocLng,
			LocName:    in.LocName,
			QuotedID:   in.QuotedID,
			ExternalID: in.ExternalID,
			TS:         in.TS,
			Authorized: in.Authorized,
		},
	})
	if err != nil {
		s.log.Warn("wa conversation route", "jid", in.JID, "id", in.ExternalID, "err", err)
	}
}
