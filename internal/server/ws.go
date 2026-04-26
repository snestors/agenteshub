package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/snestors/agenthub/internal/sysman"
	"github.com/snestors/agenthub/internal/ws"
)

// startSystemPoller pushes a stats envelope every 2s on the /ws/system topic.
// Runs for the lifetime of the daemon (ctx cancelled at shutdown).
func startSystemPoller(ctx context.Context, hub *ws.Hub, sm *sysman.Manager) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if hub.CountClients() == 0 {
				continue
			}
			stats, err := sm.Stats(ctx)
			if err != nil {
				continue
			}
			raw, _ := json.Marshal(stats)
			hub.Broadcast(ws.Envelope{Type: "stats", Topic: "system", Payload: raw})
		}
	}
}

// handleWSAgent upgrades the request and pumps "message" envelopes for the chat.
func (s *Server) handleWSAgent(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.log.Warn("ws accept", "err", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	id := uuid.NewString()
	client, cleanup := s.hub.Register(id, "agent")
	defer cleanup()

	ctx := r.Context()
	if err := s.hub.Pump(ctx, conn, client); err != nil {
		s.log.Debug("ws agent pump end", "id", id, "err", err)
	}
}

// handleWSSystem upgrades the request and pumps "stats" envelopes from the poller.
func (s *Server) handleWSSystem(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.log.Warn("ws accept", "err", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	id := uuid.NewString()
	client, cleanup := s.hub.Register(id, "system")
	defer cleanup()

	ctx := r.Context()
	if err := s.hub.Pump(ctx, conn, client); err != nil {
		s.log.Debug("ws system pump end", "id", id, "err", err)
	}
}
