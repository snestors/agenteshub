// Package server wires the HTTP/WS surface of the daemon.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/crypto/bcrypt"

	"github.com/snestors/agenthub/internal/auth"
	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/sysman"
	"github.com/snestors/agenthub/internal/ws"
)

const cookieName = "agenthub_token"

// Server owns the HTTP server + dependencies.
type Server struct {
	cfg     *config.Config
	repos   *store.Repos
	tokens  *auth.TokenService
	engines *cliengine.Manager
	sysman  *sysman.Manager
	hub     *ws.Hub
	log     *slog.Logger
	httpSrv *http.Server
}

// Hub exposes the WS hub for producers (poller, message handlers).
func (s *Server) Hub() *ws.Hub { return s.hub }

// New constructs a Server.
func New(cfg *config.Config, repos *store.Repos, engines *cliengine.Manager, sm *sysman.Manager, log *slog.Logger) (*Server, error) {
	tokens := auth.NewTokenService(cfg, repos.Auth)
	hub := ws.New(log.With("comp", "ws"))
	s := &Server{cfg: cfg, repos: repos, tokens: tokens, engines: engines, sysman: sm, hub: hub, log: log}
	s.registerWSActions()
	s.httpSrv = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

// Run blocks serving HTTP until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	// Background pollers that push to WS subscribers.
	go startSystemPoller(ctx, s.hub, s.sysman)
	go s.startStatusHeartbeat(ctx)
	go func() {
		<-ctx.Done()
		_ = s.httpSrv.Close()
	}()
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// startStatusHeartbeat emits agent_status every 60s as a catch-all for drift
// (changes that didn't go through our handlers, e.g. settings table edited
// directly). Skips work if no subscriber.
func (s *Server) startStatusHeartbeat(ctx context.Context) {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if s.hub.CountSubscribed("agent_status") == 0 {
				continue
			}
			s.broadcastAgentStatus(ctx)
		}
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, requestLogger(s.log), middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	// public
	r.Get("/healthz", s.handleHealth)
	r.Post("/api/auth/login", s.handleLogin)
	r.Post("/api/auth/totp", s.handleTOTP)

	// protected
	r.Group(func(pr chi.Router) {
		pr.Use(s.tokens.RequireJWT)
		pr.Post("/api/auth/logout", s.handleLogout)
		pr.Post("/api/auth/refresh", s.handleRefresh)
		pr.Get("/api/auth/me", s.handleMe)

		// stubs (próximas tools)
		pr.Get("/api/messages", s.handleMessagesList)
		pr.Post("/api/messages", s.handleMessagesSend)

		// Agent status (StatusBar UI)
		pr.Get("/api/agent/status", s.handleAgentStatus)
		pr.Get("/api/agent/engines", s.handleListEngines)
		pr.Post("/api/agent/engine", s.handleSetEngine)

		// Uploads — adjuntar archivos al prompt
		pr.Post("/api/upload", s.handleUpload)
		pr.Delete("/api/uploads/{id}", s.handleDeleteUpload)

		// Projects — coding workspaces + browser sessions
		pr.Get("/api/projects", s.handleProjectsList)
		pr.Post("/api/projects", s.handleProjectsCreate)
		pr.Get("/api/projects/{id}", s.handleProjectGet)
		pr.Get("/api/projects/{id}/sessions", s.handleProjectSessionsList)
		pr.Post("/api/projects/{id}/sessions", s.handleProjectSessionsCreate)
		pr.Get("/api/projects/{id}/sessions/{sid}/messages", s.handleProjectSessionMessagesList)
		pr.Post("/api/projects/{id}/sessions/{sid}/messages", s.handleProjectSessionMessagesSend)

		// Mini-agents — persistent scheduled/manual agents
		pr.Get("/api/agents", s.handleAgentsList)
		pr.Post("/api/agents", s.handleAgentsCreate)
		pr.Get("/api/agents/{id}", s.handleAgentGet)
		pr.Post("/api/agents/{id}/enabled", s.handleAgentSetEnabled)
		pr.Post("/api/agents/{id}/run", s.handleAgentRunNow)
		pr.Get("/api/agents/{id}/runs", s.handleAgentRuns)
		pr.Post("/api/agents/{id}/schedules", s.handleAgentSchedulesAdd)
		pr.Post("/api/agents/{id}/schedules/{sid}/enabled", s.handleAgentScheduleEnabled)
		pr.Delete("/api/agents/{id}/schedules/{sid}", s.handleAgentScheduleDelete)

		// System manager
		pr.Get("/api/system/stats", s.handleSystemStats)
		pr.Get("/api/system/services", s.handleSystemServices)
		pr.Post("/api/system/services/{name}/{action}", s.handleSystemServiceAction)
		pr.Get("/api/system/processes", s.handleSystemProcesses)
		pr.Get("/api/system/connections", s.handleSystemConnections)

		// WebSockets
		pr.Get("/ws", s.handleWSUnified)
	})

	// frontend SPA — sirve frontend/dist con fallback a index.html para client-side routing
	s.mountFrontend(r)
	return r
}

// mountFrontend serves the React SPA from frontend/dist with fallback to index.html.
// Falls back to a placeholder if the dist isn't built yet.
func (s *Server) mountFrontend(r chi.Router) {
	dist := s.cfg.FrontendDist
	if dist == "" {
		dist = "frontend/dist"
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			setFrontendNoStore(w)
			s.serveFrontend(w, r)
		})
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			setFrontendNoStore(w)
			s.serveFrontend(w, r)
		})
		return
	}
	fs := http.FileServer(http.Dir(dist))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// API/WS already matched; here only static + SPA fallback. Do not let
		// deleted WS routes fall through to index.html.
		if strings.HasPrefix(req.URL.Path, "/ws/") {
			http.NotFound(w, req)
			return
		}
		path := filepath.Join(dist, filepath.Clean(req.URL.Path))
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			setFrontendCacheHeaders(w, req.URL.Path)
			fs.ServeHTTP(w, req)
			return
		}
		// Fallback to index.html for SPA client-side routes.
		setFrontendNoStore(w)
		http.ServeFile(w, req, filepath.Join(dist, "index.html"))
	})
}

func setFrontendCacheHeaders(w http.ResponseWriter, urlPath string) {
	switch {
	case urlPath == "/" || urlPath == "/index.html":
		setFrontendNoStore(w)
	case strings.HasPrefix(urlPath, "/assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
}

func setFrontendNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

// ---------- handlers ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ts": time.Now().Unix()})
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type loginResp struct {
	NeedTOTP bool   `json:"need_totp"`
	Token    string `json:"token,omitempty"`
}

// handleLogin validates user/password. If DevBypassTOTP, emits the JWT immediately.
// Otherwise responds need_totp=true and the client must call /api/auth/totp.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	user, err := s.repos.Auth.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "credentials", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "credentials", http.StatusUnauthorized)
		return
	}
	if s.cfg.DevBypassTOTP {
		token, err := s.tokens.Issue(user.ID, "")
		if err != nil {
			http.Error(w, "issue token", http.StatusInternalServerError)
			return
		}
		_ = s.repos.Auth.MarkLogin(r.Context(), user.ID, time.Now().Unix())
		s.setCookie(w, r, token)
		writeJSON(w, http.StatusOK, loginResp{NeedTOTP: false, Token: token})
		return
	}
	// Stash a short-lived intermediate token (JTI marked as "totp-pending"). For
	// the v0 we just respond NeedTOTP and the next call carries username again.
	writeJSON(w, http.StatusOK, loginResp{NeedTOTP: true})
}

type totpReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (s *Server) handleTOTP(w http.ResponseWriter, r *http.Request) {
	var req totpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	user, err := s.repos.Auth.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "credentials", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "credentials", http.StatusUnauthorized)
		return
	}
	secret, err := auth.DecryptAESGCM(s.cfg.SecretKey, user.TOTPSecretEncrypted)
	if err != nil {
		http.Error(w, "totp", http.StatusUnauthorized)
		return
	}
	if !auth.ValidateTOTP(req.Code, string(secret)) {
		http.Error(w, "totp invalid", http.StatusUnauthorized)
		return
	}
	token, err := s.tokens.Issue(user.ID, "")
	if err != nil {
		http.Error(w, "issue", http.StatusInternalServerError)
		return
	}
	_ = s.repos.Auth.MarkLogin(r.Context(), user.ID, time.Now().Unix())
	s.setCookie(w, r, token)
	writeJSON(w, http.StatusOK, loginResp{Token: token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if ok {
		_ = s.tokens.Revoke(r.Context(), c)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "no claims", http.StatusUnauthorized)
		return
	}
	uid, _ := c.UserID()
	token, err := s.tokens.Issue(uid, "")
	if err != nil {
		http.Error(w, "issue", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no user", http.StatusUnauthorized)
		return
	}
	user, err := s.repos.Auth.GetUserByID(r.Context(), uid)
	if err != nil {
		http.Error(w, "user", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": user.ID, "username": user.Username, "last_login": user.LastLogin,
	})
}

// ---------- messages stubs (los completa Bloque 2) ----------

func (s *Server) handleMessagesList(w http.ResponseWriter, r *http.Request) {
	msgs, err := s.repos.Messages.Recent(r.Context(), "", 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

type sendMsgReq struct {
	Body        string          `json:"body"`
	Attachments []msgAttachment `json:"attachments,omitempty"`
}

// msgAttachment is the lightweight reference the client sends back after upload.
type msgAttachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size,omitempty"`
}

// formatAttachments expands the attachments list into a footer the agent reads.
// The agent (Claude) can use the Read tool to inspect them at the given paths.
func formatAttachments(att []msgAttachment) string {
	if len(att) == 0 {
		return ""
	}
	lines := []string{"", "[archivos adjuntos del user — leelos con la tool Read si te ayudan a responder]"}
	for _, a := range att {
		size := ""
		if a.Size > 0 {
			size = fmt.Sprintf(" (%d bytes)", a.Size)
		}
		lines = append(lines, fmt.Sprintf("- %s%s · type=%s · path=%s", a.Name, size, a.Type, a.Path))
	}
	return strings.Join(lines, "\n")
}

func (s *Server) handleMessagesSend(w http.ResponseWriter, r *http.Request) {
	var req sendMsgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	res, err := s.acceptMessage(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "empty" {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type sendMessageAccepted struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"message_id"`
	Accepted  bool   `json:"accepted"`
	Engine    string `json:"engine"`
	Model     string `json:"model"`
}

func (s *Server) acceptMessage(ctx context.Context, req sendMsgReq) (sendMessageAccepted, error) {
	if strings.TrimSpace(req.Body) == "" {
		return sendMessageAccepted{}, errors.New("empty")
	}
	// Persist the user message verbatim (without the attachments footer — that's
	// only for the agent prompt). The UI shows what the user wrote, not the path
	// list.
	id, err := s.repos.Messages.Insert(ctx, store.Message{
		Channel:   "web",
		Direction: "in",
		Body:      sqlStr(req.Body),
		TS:        time.Now().Unix(),
	})
	if err != nil {
		return sendMessageAccepted{}, err
	}
	s.broadcastMessage(id, "web", "in", req.Body)

	// Build the actual prompt sent to the engine: user text + attachments footer.
	enginePrompt := req.Body + formatAttachments(req.Attachments)

	// Read engine/model from settings (set by /api/agent/engine), with config fallback.
	engine := s.cfg.DefaultEngine
	model := s.cfg.DefaultModel
	if v, _ := s.repos.Settings.Get(ctx, "engine"); v != "" {
		engine = v
	}
	if v, _ := s.repos.Settings.Get(ctx, "model"); v != "" {
		model = v
	}

	// Resume id of the main agent's session (persisted between turns). Session
	// ids are engine-specific; never feed a Claude session id into Codex, etc.
	mainSess := s.mainAgentSession(ctx, engine)
	prev := ""
	if mainSess != nil {
		prev = mainSess.SessionID
	}

	go s.runMainAgentTurn(enginePrompt, prev, engine, model)

	return sendMessageAccepted{ID: id, MessageID: id, Accepted: true, Engine: engine, Model: model}, nil
}

func (s *Server) runMainAgentTurn(enginePrompt, prev, engine, model string) {
	// Run via cliengine. cwd=agenthub repo so .claude/skills/ is discovered.
	// Stream events to the WS hub so the browser shows the agent's reasoning + tool calls
	// while the turn is still in flight.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	res, err := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    enginePrompt,
		SessionID: prev,
		Channel:   "web",
		Cwd:       ".",
		Engine:    engine,
		Model:     model,
		Scope:     "main",
		AgentName: "main-agent",
		OnEvent:   s.streamEventBroadcaster(),
	})
	if err != nil {
		s.log.Error("cliengine", "err", err)
		outID, _ := s.repos.Messages.Insert(context.Background(), store.Message{
			Channel:   "web",
			Direction: "out",
			Body:      sqlStr("⚠ engine: " + err.Error()),
			TS:        time.Now().Unix(),
			Engine:    sqlStr(engine),
			Model:     sqlStr(model),
		})
		s.broadcastMessageWithModel(outID, "web", "out", "⚠ engine: "+err.Error(), engine, model)
		return
	}
	if res.SessionID != "" && res.SessionID != prev {
		_ = s.repos.Sessions.UpsertAgentSession(context.Background(), store.AgentSession{
			AgentName: mainAgentSessionName(engine),
			Engine:    engine,
			SessionID: res.SessionID,
		})
	}
	outID, _ := s.repos.Messages.Insert(context.Background(), store.Message{
		Channel:   "web",
		Direction: "out",
		Body:      sqlStr(res.Text),
		TS:        time.Now().Unix(),
		Engine:    sqlStr(engine),
		Model:     sqlStr(model),
	})
	s.broadcastMessageWithModel(outID, "web", "out", res.Text, engine, model)
	// Push fresh agent_status to all subscribers — ctx_used cambió
	go s.broadcastAgentStatus(context.Background())
}

func (s *Server) mainAgentSession(ctx context.Context, engine string) *store.AgentSession {
	if engine != "" {
		if sess, _ := s.repos.Sessions.GetAgentSession(ctx, mainAgentSessionName(engine)); sess != nil && sess.Engine == engine {
			return sess
		}
	}
	// Legacy fallback from the pre-#28 schema, where main-agent had a single
	// mutable row. Only reuse it when its engine matches.
	if sess, _ := s.repos.Sessions.GetAgentSession(ctx, "main-agent"); sess != nil && sess.Engine == engine {
		return sess
	}
	return nil
}

func mainAgentSessionName(engine string) string {
	if engine == "" {
		return "main-agent"
	}
	return "main-agent:" + engine
}

// ---------- frontend / static ----------

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, frontendPlaceholder, s.cfg.HTTPAddr)
}

const frontendPlaceholder = `<!doctype html><html><head><meta charset=utf-8><title>AgentHub</title>
<style>
body{background:#0b0f14;color:#cbd5e1;font-family:ui-monospace,Menlo,monospace;padding:40px;max-width:780px;margin:0 auto;line-height:1.5}
h1{color:#2dd4bf;letter-spacing:3px;border-bottom:2px solid #2dd4bf;padding-bottom:8px}
code{color:#38bdf8;background:rgba(56,189,248,.08);padding:2px 6px;border-radius:3px}
.box{background:#131c27;border:1px solid #1e2a3a;padding:16px 20px;border-radius:6px;margin:16px 0}
.ok{color:#4ade80}
.warn{color:#facc15}
</style></head><body>
<h1>AGENTHUB · v0.1</h1>
<p class=ok>✓ Daemon arriba en %s</p>
<div class=box>
<p>El frontend React+Vite+Tailwind+shadcn se sirve desde <code>frontend/dist/</code> cuando esté buildeado.</p>
<p>Mientras tanto:</p>
<ul>
<li><code>POST /api/auth/login</code> — username + password</li>
<li><code>POST /api/auth/totp</code> — código TOTP (omitido si <code>AGENTHUB_DEV_BYPASS_TOTP=true</code>)</li>
<li><code>GET /api/auth/me</code> — perfil del usuario logueado (requiere cookie JWT)</li>
<li><code>GET /api/messages</code> · <code>POST /api/messages</code> — chat</li>
<li><code>GET /healthz</code> — público</li>
</ul>
</div>
<p class=warn>⚠ Ejecutá <code>agenthub setup-user --password &lt;X&gt;</code> antes del primer login.</p>
</body></html>`

// streamEventBroadcaster returns a callback that wraps each StreamEvent into
// a "stream"-typed envelope and broadcasts it on the agent topic.
func (s *Server) streamEventBroadcaster() func(cliengine.StreamEvent) {
	if s.hub == nil {
		return func(cliengine.StreamEvent) {}
	}
	return func(ev cliengine.StreamEvent) {
		raw, err := json.Marshal(ev)
		if err != nil {
			return
		}
		s.hub.Broadcast(ws.Envelope{Type: "stream", Topic: "agent", Payload: raw})
	}
}

// broadcastMessage pushes a chat message envelope to all WS subscribers on the agent topic.
func (s *Server) broadcastMessage(id int64, channel, direction, body string) {
	s.broadcastMessageWithModel(id, channel, direction, body, "", "")
}

// broadcastMessageWithModel includes engine/model so the UI can show the
// model that produced an assistant turn.
func (s *Server) broadcastMessageWithModel(id int64, channel, direction, body, engine, model string) {
	if s.hub == nil {
		return
	}
	out := map[string]any{
		"id":        id,
		"channel":   channel,
		"direction": direction,
		"body":      body,
		"ts":        time.Now().Unix(),
	}
	if engine != "" {
		out["engine"] = engine
	}
	if model != "" {
		out["model"] = model
	}
	payload, _ := json.Marshal(out)
	s.hub.Broadcast(ws.Envelope{Type: "message", Topic: "agent", Payload: payload})
}

// ---------- helpers ----------

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.JWTTTL.Seconds()),
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func wsAck(topic, id string, result any, err error) ws.Envelope {
	payload := map[string]any{
		"id": id,
		"ok": err == nil,
	}
	if err != nil {
		payload["error"] = err.Error()
	} else {
		payload["result"] = result
	}
	raw, _ := json.Marshal(payload)
	return ws.Envelope{Type: "ack", Topic: topic, Payload: raw}
}

func sqlStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
