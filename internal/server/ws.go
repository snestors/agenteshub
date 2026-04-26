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

// startSystemPoller pushes a stats envelope every 2s on the system topic.
// Skips work if no client is currently subscribed to "system".
func startSystemPoller(ctx context.Context, hub *ws.Hub, sm *sysman.Manager) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if hub.CountSubscribed("system") == 0 {
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

// handleWSUnified upgrades the request and runs a Pump with dynamic
// subscriptions controlled by the client.
func (s *Server) handleWSUnified(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.log.Warn("ws accept", "err", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	id := uuid.NewString()
	client, cleanup := s.hub.Register(id)
	defer cleanup()

	ctx := r.Context()
	if err := s.hub.Pump(ctx, conn, client); err != nil {
		s.log.Debug("ws pump end", "id", id, "err", err)
	}
}
