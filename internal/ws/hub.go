// Package ws implements a pub/sub hub for browser clients.
//
// One Hub instance, N subscribers. Each Client holds a dynamic set of topics
// it subscribed to (subscribe/unsubscribe sent over WebSocket). Producers
// call Broadcast(envelope); only clients whose subscription set contains
// envelope.Topic (or "*") receive it.
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
	Type    string          `json:"type"`            // "message" | "stream" | "stats" | "status" | ...
	Topic   string          `json:"topic,omitempty"` // routing key the producer used
	Payload json.RawMessage `json:"payload"`
	TS      int64           `json:"ts"`
}

// ClientAction is what the browser sends back over the WS.
type ClientAction struct {
	Action      string          `json:"action"` // 'subscribe' | 'unsubscribe' | 'ping' | RPC actions
	Topic       string          `json:"topic,omitempty"`
	ID          string          `json:"id,omitempty"`          // correlation id for RPC
	Body        json.RawMessage `json:"body,omitempty"`        // send_message text, encoded as a JSON string
	Attachments json.RawMessage `json:"attachments,omitempty"` // send_message upload refs
	Engine      string          `json:"engine,omitempty"`      // set_engine
	Model       string          `json:"model,omitempty"`       // set_engine
	Name        string          `json:"name,omitempty"`        // service_action service name
	Op          string          `json:"op,omitempty"`          // service_action operation (start|stop|restart)
}

// ActionHandler is invoked for any non-meta action received from a client
// (i.e. anything that isn't subscribe/unsubscribe/ping). Allows callers to
// register handlers for RPC actions like 'send_message' or 'set_engine'.
//
// The handler may return an Envelope to be unicast back to the originating
// client (correlation via the action's ID).
type ActionHandler func(ctx context.Context, c *Client, action ClientAction) (*Envelope, error)

// Hub fan-outs Envelopes to every subscribed channel.
type Hub struct {
	log            *slog.Logger
	mu             sync.RWMutex
	clients        map[*Client]struct{}
	actionsMu      sync.RWMutex
	actionHandlers map[string]ActionHandler
}

// Client is a single connected browser session.
type Client struct {
	ID         string
	send       chan Envelope
	mu         sync.RWMutex
	subscribed map[string]struct{} // dynamic set of topics
}

// New constructs a Hub.
func New(log *slog.Logger) *Hub {
	return &Hub{
		log:            log,
		clients:        map[*Client]struct{}{},
		actionHandlers: map[string]ActionHandler{},
	}
}

// Register adds a new subscriber. The client subscribes dynamically over WS.
func (h *Hub) Register(id string) (*Client, func()) {
	c := &Client{
		ID:         id,
		send:       make(chan Envelope, 64),
		subscribed: map[string]struct{}{},
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

// HandleAction registers a handler for an RPC action coming from clients.
// Reserved actions (subscribe/unsubscribe/ping) are handled by Pump.
func (h *Hub) HandleAction(name string, handler ActionHandler) {
	h.actionsMu.Lock()
	h.actionHandlers[name] = handler
	h.actionsMu.Unlock()
}

// Subscribe adds a topic to the client's subscription set.
func (c *Client) Subscribe(topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribed[topic] = struct{}{}
}

// Unsubscribe removes a topic from the client's set.
func (c *Client) Unsubscribe(topic string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribed, topic)
}

// matches reports whether the client wants envelopes on this topic.
func (c *Client) matches(topic string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.subscribed["*"]; ok {
		return true
	}
	_, ok := c.subscribed[topic]
	return ok
}

// SendDirect pushes an envelope only to this client (for RPC responses).
// Non-blocking — drops if buffer full, same policy as Broadcast.
func (c *Client) SendDirect(env Envelope) {
	if env.TS == 0 {
		env.TS = time.Now().UnixMilli()
	}
	select {
	case c.send <- env:
	default:
	}
}

// Broadcast pushes the envelope to every client whose subscription matches.
// Non-blocking: if a client's send buffer is full, the envelope is dropped for
// that client (we prefer dropping over blocking the producer).
func (h *Hub) Broadcast(env Envelope) {
	if env.TS == 0 {
		env.TS = time.Now().UnixMilli()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.matches(env.Topic) {
			continue
		}
		select {
		case c.send <- env:
		default:
			h.log.Warn("ws drop", "client", c.ID, "type", env.Type)
		}
	}
}

// Pump runs the read+write loop of a websocket connection.
//   - Reads ClientAction frames; handles subscribe/unsubscribe/ping internally,
//     delegates other actions to registered HandleAction callbacks.
//   - Writes envelopes from the client's send channel.
//
// Blocks until ctx is cancelled, the connection is closed, or an error occurs.
func (h *Hub) Pump(ctx context.Context, conn *websocket.Conn, c *Client) error {
	// reader: process actions from the client
	go func() {
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var act ClientAction
			if err := json.Unmarshal(raw, &act); err != nil {
				h.log.Debug("ws bad action", "client", c.ID, "err", err)
				continue
			}
			switch act.Action {
			case "subscribe":
				if act.Topic != "" {
					c.Subscribe(act.Topic)
					h.log.Debug("ws subscribe", "client", c.ID, "topic", act.Topic)
				}
			case "unsubscribe":
				if act.Topic != "" {
					c.Unsubscribe(act.Topic)
					h.log.Debug("ws unsubscribe", "client", c.ID, "topic", act.Topic)
				}
			case "ping":
				c.SendDirect(Envelope{Type: "pong", Topic: "system", Payload: json.RawMessage(`{}`)})
			default:
				h.actionsMu.RLock()
				handler := h.actionHandlers[act.Action]
				h.actionsMu.RUnlock()
				if handler == nil {
					h.log.Debug("ws unhandled action", "client", c.ID, "action", act.Action)
					continue
				}
				go func(a ClientAction) {
					resp, err := handler(ctx, c, a)
					if err != nil {
						h.log.Warn("ws action error", "action", a.Action, "err", err)
						errPayload, _ := json.Marshal(map[string]any{"error": err.Error(), "action": a.Action, "id": a.ID})
						c.SendDirect(Envelope{Type: "error", Topic: "rpc", Payload: errPayload})
						return
					}
					if resp != nil {
						c.SendDirect(*resp)
					}
				}(act)
			}
		}
	}()

	// writer
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

// CountSubscribed returns how many clients have the given topic in their set.
// Used by producers (e.g. system poller) to skip work when nobody listens.
func (h *Hub) CountSubscribed(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.clients {
		if c.matches(topic) {
			n++
		}
	}
	return n
}
