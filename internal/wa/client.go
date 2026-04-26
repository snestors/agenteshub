// Package wa wraps the whatsmeow client + handler + queue for AgentHub.
//
// Disabled by default in dev (cfg.WAEnabled = false) so the daemon can run
// without touching the production WA number until cutover.
package wa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

// Client wraps whatsmeow's Client with persistence + queue glue.
type Client struct {
	cfg         *config.Config
	repos       *store.Repos
	log         *slog.Logger
	wmClient    *whatsmeow.Client
	connected   bool
	connectedMu sync.RWMutex
	qrCh        chan string // QR scan codes during pairing
	queue       chan IncomingMessage
}

// IncomingMessage is what the handler dispatches to the cola serial of the main.
type IncomingMessage struct {
	JID       string
	Phone     string
	Name      string
	Body      string
	MediaPath string
	MediaKind string // 'image'|'voice'|'audio'|'video'|'document'|''
	LocLat    float64
	LocLng    float64
	LocName   string
	QuotedID  string
	TS        time.Time
	IsCommand bool   // /reset /status /topic etc.
	Authorized bool
}

// New creates the wa client. Connect() actually pairs / reconnects.
func New(cfg *config.Config, repos *store.Repos, log *slog.Logger) (*Client, error) {
	c := &Client{
		cfg:   cfg,
		repos: repos,
		log:   log,
		qrCh:  make(chan string, 4),
		queue: make(chan IncomingMessage, 64),
	}
	if err := os.MkdirAll(filepath.Dir(cfg.WADBPath), 0o755); err != nil {
		return nil, fmt.Errorf("ensure wa db dir: %w", err)
	}
	return c, nil
}

// Queue exposes the channel where new authorized messages flow.
func (c *Client) Queue() <-chan IncomingMessage { return c.queue }

// Connected reports whether whatsmeow is currently connected.
func (c *Client) Connected() bool {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

// QR returns a channel that emits QR scan codes during initial pairing.
func (c *Client) QR() <-chan string { return c.qrCh }

// Connect wires the whatsmeow store + client and starts handling events.
// Safe to call when cfg.WAEnabled is false; returns immediately.
func (c *Client) Connect(ctx context.Context) error {
	if !c.cfg.WAEnabled {
		c.log.Info("wa disabled by config — skipping connection")
		return nil
	}
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", c.cfg.WADBPath)
	wlog := waLog.Stdout("waStore", "WARN", true)
	container, err := sqlstore.New(ctx, "sqlite3", dsn, wlog)
	if err != nil {
		return fmt.Errorf("wa sqlstore: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("wa device: %w", err)
	}
	clog := waLog.Stdout("waClient", "WARN", true)
	c.wmClient = whatsmeow.NewClient(device, clog)
	c.wmClient.AddEventHandler(c.handleEvent)

	if c.wmClient.Store.ID == nil {
		// Pairing first time — emit QR codes through QR channel.
		qrCh, err := c.wmClient.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("wa qr channel: %w", err)
		}
		go func() {
			for evt := range qrCh {
				if evt.Event == "code" {
					c.log.Info("wa qr emitted")
					select {
					case c.qrCh <- evt.Code:
					default:
					}
				} else {
					c.log.Info("wa pairing event", "event", evt.Event)
				}
			}
		}()
	}
	if err := c.wmClient.Connect(); err != nil {
		return fmt.Errorf("wa connect: %w", err)
	}
	c.setConnected(true)
	c.log.Info("wa connected", "id", deviceJID(c.wmClient))
	return nil
}

// Disconnect closes the whatsmeow client.
func (c *Client) Disconnect() {
	if c.wmClient != nil {
		c.wmClient.Disconnect()
	}
	c.setConnected(false)
}

func (c *Client) setConnected(v bool) {
	c.connectedMu.Lock()
	defer c.connectedMu.Unlock()
	c.connected = v
}

// SendText posts a plain message to a JID. Returns error if not connected.
func (c *Client) SendText(ctx context.Context, jid, text string) error {
	if !c.Connected() {
		return errors.New("wa not connected")
	}
	parsed, err := parseJID(jid)
	if err != nil {
		return err
	}
	_, err = c.wmClient.SendMessage(ctx, parsed, makeTextMessage(text))
	return err
}

func (c *Client) handleEvent(evt any) {
	switch e := evt.(type) {
	case *events.Message:
		c.dispatchIncoming(context.Background(), e)
	case *events.Connected:
		c.setConnected(true)
		c.log.Info("wa connected event")
	case *events.Disconnected:
		c.setConnected(false)
		c.log.Warn("wa disconnected event")
	case *events.LoggedOut:
		c.setConnected(false)
		c.log.Warn("wa logged out — re-pair required", "reason", e.Reason)
	default:
		c.log.Debug("wa event", "type", fmt.Sprintf("%T", evt))
	}
}

// IsAuthorized reports whether the phone is in the configured whitelist.
// Empty whitelist = allow all (useful for solo accounts).
func (c *Client) IsAuthorized(phone string) bool {
	if len(c.cfg.WAAuthorized) == 0 {
		return true
	}
	return slices.Contains(c.cfg.WAAuthorized, strings.TrimSpace(phone))
}

// deviceJID returns the device's JID string when available.
func deviceJID(cl *whatsmeow.Client) string {
	if cl == nil || cl.Store.ID == nil {
		return ""
	}
	return cl.Store.ID.String()
}
