package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/agenteshub/agenteshub/internal/sysman"
	"github.com/agenteshub/agenteshub/internal/ws"
)

// startSystemPoller pushes a stats envelope every 2s on the system topic.
// Skips work if no client is currently subscribed to "system".
//
// The payload is the sysman.Stats embedded plus a few server-side counters
// (running agents, ws clients) that depend on state outside sysman.
func (s *Server) startSystemPoller(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if s.hub.CountSubscribed("system") == 0 {
				continue
			}
			payload := s.buildSystemTick(ctx)
			if payload == nil {
				continue
			}
			raw, _ := json.Marshal(payload)
			s.hub.Broadcast(ws.Envelope{Type: "stats", Topic: "system", Payload: raw})
		}
	}
}

// systemTick wraps sysman.Stats with a couple of cross-cutting counters that
// the server has and sysman doesn't (running agents, ws clients).
type systemTick struct {
	sysman.Stats
	RunningAgents  int `json:"running_agents"`
	RunningMain    int `json:"running_main"`
	RunningProject int `json:"running_project"`
	RunningTotal   int `json:"running_total"`
	WSClients      int `json:"ws_clients"`
}

func (s *Server) buildSystemTick(ctx context.Context) *systemTick {
	stats, err := s.sysman.Stats(ctx)
	if err != nil {
		return nil
	}
	runningAgents, _ := s.repos.Agents.CountRunning(ctx)
	runningMain := 0
	runningProject := 0
	if s.runs != nil {
		runningMain = s.runs.Count("main")
		runningProject = s.runs.Count("project")
	}
	clients := 0
	if s.hub != nil {
		clients = s.hub.CountClients()
	}
	return &systemTick{
		Stats:          stats,
		RunningAgents:  runningAgents,
		RunningMain:    runningMain,
		RunningProject: runningProject,
		RunningTotal:   runningAgents + runningMain + runningProject,
		WSClients:      clients,
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
