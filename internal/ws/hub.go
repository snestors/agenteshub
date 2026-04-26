// Package ws implements a minimal pub/sub hub for browser clients.
//
// One Hub instance, N subscribers. Producers (chat handlers, system manager
// poller, mini-agent runs) call Broadcast on a topic; subscribers receive
// JSON envelopes via their personal channel and the WebSocket goroutine
// pumps them to the browser.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Envelope is what gets sent to subscribed browsers as JSON.
type Envelope struct {
	Type    string          `json:"type"`              // "message" | "stats" | "subagent" | "agent_run" | ...
	Topic   string          `json:"topic,omitempty"`   // routing key the producer used
	Payload json.RawMessage `json:"payload"`
	TS      int64           `json:"ts"`
}

// Hub fan-outs Envelopes to every subscribed channel.
type Hub struct {
	log     *slog.Logger
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// Client is a single connected browser session.
type Client struct {
	ID    string
	send  chan Envelope
	topic string // optional filter; empty = all
}

// New constructs a Hub.
func New(log *slog.Logger) *Hub {
	return &Hub{log: log, clients: map[*Client]struct{}{}}
}

// Register adds a new subscriber. Optional topic filter ("" = receive all).
// Returns the client and a cleanup func to call on disconnect.
func (h *Hub) Register(id, topic string) (*Client, func()) {
	c := &Client{
		ID:    id,
		send:  make(chan Envelope, 64),
		topic: topic,
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	cleanup := func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.send)
	}
	return c, cleanup
}

// Broadcast pushes the envelope to every client whose topic matches (or is empty).
// Non-blocking: if a client's send buffer is full, the envelope is dropped for
// that client (we prefer dropping over blocking the producer).
func (h *Hub) Broadcast(env Envelope) {
	if env.TS == 0 {
		env.TS = time.Now().UnixMilli()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.topic != "" && c.topic != env.Topic {
			continue
		}
		select {
		case c.send <- env:
		default:
			// dropped — slow client; the deadline-based reader on the WS side
			// will drop the connection if it doesn't catch up.
			h.log.Warn("ws drop", "client", c.ID, "type", env.Type)
		}
	}
}

// Pump runs the read+write loop of a websocket connection.
// Blocks until ctx is cancelled, the connection is closed, or an error occurs.
func (h *Hub) Pump(ctx context.Context, conn *websocket.Conn, c *Client) error {
	// reader: drains incoming frames (we don't act on client-sent messages yet,
	// but we must read so the lib processes ping/pong + close frames).
	go func() {
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case env, ok := <-c.send:
			if !ok {
				return errors.New("client send closed")
			}
			raw, err := json.Marshal(env)
			if err != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = conn.Write(wctx, websocket.MessageText, raw)
			cancel()
			if err != nil {
				return err
			}
		}
	}
}

// CountClients returns the number of currently connected subscribers.
func (h *Hub) CountClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
