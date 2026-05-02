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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/crypto/bcrypt"

	"github.com/snestors/agenteshub/internal/auth"
	"github.com/snestors/agenteshub/internal/buildinfo"
	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/config"
	"github.com/snestors/agenteshub/internal/store"
	"github.com/snestors/agenteshub/internal/sysman"
	"github.com/snestors/agenteshub/internal/usage"
	"github.com/snestors/agenteshub/internal/ws"
)

const cookieName = "agenthub_token"

// Server owns the HTTP server + dependencies.
type Server struct {
	cfg            *config.Config
	repos          *store.Repos
	tokens         *auth.TokenService
	engines        *cliengine.Manager
	sysman         *sysman.Manager
	hub            *ws.Hub
	runs           *RunTracker
	log            *slog.Logger
	httpSrv        *http.Server
	waState        waState
	projectCancels sync.Map // key: project_session id (int64) → context.CancelFunc
	usageRepo      *usage.UsageRepo
}

// Hub exposes the WS hub for producers (poller, message handlers).
func (s *Server) Hub() *ws.Hub { return s.hub }

// New constructs a Server.
func New(cfg *config.Config, repos *store.Repos, engines *cliengine.Manager, sm *sysman.Manager, log *slog.Logger) (*Server, error) {
	tokens := auth.NewTokenService(cfg, repos.Auth)
	hub := ws.New(log.With("comp", "ws"))
	s := &Server{cfg: cfg, repos: repos, tokens: tokens, engines: engines, sysman: sm, hub: hub, runs: NewRunTracker(), log: log, usageRepo: usage.NewUsageRepo(repos.DB())}
	if repos != nil && repos.ConversationRuns != nil {
		_ = repos.ConversationRuns.MarkInterruptedOlderThan(context.Background(), time.Now().Unix())
	}
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
	go s.startSystemPoller(ctx)
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
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	// public
	r.Get("/healthz", s.handleHealth)
	r.Get("/api/runs", s.handleRunsStatus)
	r.Post("/api/runs/schedule-restart", s.handleScheduleRestart)
	r.Get("/api/releases", s.handleReleases)
	// Loopback-only — used by bin/budget-alert.sh + cron. Both handlers enforce
	// RemoteAddr in 127.0.0.0/8 || ::1; no JWT.
	r.Get("/api/internal/usage", s.handleInternalUsage)
	r.Post("/api/internal/notify", s.handleInternalNotify)
	r.Get("/api/wa/status", s.handleWaStatus)
	r.Get("/api/wa/qr", s.handleWaQR)
	r.Post("/api/auth/login", s.handleLogin)
	r.Post("/api/auth/totp", s.handleTOTP)

	// protected
	r.Group(func(pr chi.Router) {
		pr.Use(s.tokens.RequireJWT)
		pr.Post("/api/auth/logout", s.handleLogout)
		pr.Post("/api/auth/refresh", s.handleRefresh)
		pr.Get("/api/auth/me", s.handleMe)
		pr.Post("/api/push/register", s.handlePushRegister)
		pr.Post("/api/push/test", s.handlePushTest)

		// stubs (próximas tools)
		pr.Get("/api/messages", s.handleMessagesList)
		pr.Get("/api/messages/search", s.handleMessagesSearch)
		pr.Post("/api/messages", s.handleMessagesSend)

		// Agent status (StatusBar UI)
		pr.Get("/api/agent/status", s.handleAgentStatus)
		pr.Get("/api/agent/runtime", s.handleAgentRuntime)
		pr.Get("/api/agent/engines", s.handleListEngines)
		pr.Post("/api/agent/engine", s.handleSetEngine)

		// Generic cross-scope cancel (used by the long_running_turn toast).
		pr.Post("/api/runs/cancel", s.handleRunsCancel)

		// Uploads — adjuntar archivos al prompt
		pr.Post("/api/upload", s.handleUpload)
		pr.Get("/api/uploads/{id}", s.handleGetUpload)
		pr.Delete("/api/uploads/{id}", s.handleDeleteUpload)
		pr.Get("/api/file", s.handleGetFile)

		// Projects — coding workspaces + browser sessions
		pr.Get("/api/projects", s.handleProjectsList)
		pr.Post("/api/projects", s.handleProjectsCreate)
		pr.Get("/api/projects/discover", s.handleProjectsDiscover)
		pr.Get("/api/projects/{id}", s.handleProjectGet)
		pr.Get("/api/projects/{id}/services", s.handleProjectServices)
		pr.Post("/api/projects/{id}/services/reload", s.handleProjectServicesReload)
		pr.Post("/api/projects/{id}/services/{idx}/{action}", s.handleProjectServiceAction)
		pr.Get("/api/projects/{id}/openspec/changes", s.handleOpenSpecChangesList)
		pr.Post("/api/projects/{id}/openspec/changes", s.handleOpenSpecChangesCreate)
		pr.Get("/api/projects/{id}/openspec/changes/{name}", s.handleOpenSpecChangeGet)
		pr.Post("/api/projects/{id}/openspec/changes/{name}/approve", s.handleOpenSpecApprove)
		pr.Post("/api/projects/{id}/openspec/changes/{name}/reject", s.handleOpenSpecReject)
		pr.Post("/api/projects/{id}/openspec/changes/{name}/feedback", s.handleOpenSpecFeedback)
		pr.Get("/api/projects/{id}/openspec/specs", s.handleOpenSpecSpecsList)
		pr.Get("/api/projects/{id}/openspec/specs/{capability}", s.handleOpenSpecSpecGet)
		pr.Get("/api/projects/{id}/sessions", s.handleProjectSessionsList)
		pr.Post("/api/projects/{id}/sessions", s.handleProjectSessionsCreate)
		pr.Delete("/api/projects/{id}/sessions/{sid}", s.handleProjectSessionDelete)
		pr.Get("/api/projects/{id}/sessions/{sid}/messages", s.handleProjectSessionMessagesList)
		pr.Post("/api/projects/{id}/sessions/{sid}/messages", s.handleProjectSessionMessagesSend)
		pr.Get("/api/projects/{id}/sessions/{sid}/run", s.handleProjectSessionRunStatus)
		pr.Get("/api/projects/{id}/sessions/{sid}/runtime", s.handleProjectSessionRuntime)
		pr.Delete("/api/projects/{id}/sessions/{sid}/run", s.handleProjectSessionCancel)
		pr.Post("/api/projects/{id}/sessions/{sid}/engine", s.handleProjectSessionSetEngine)
		pr.Post("/api/projects/{id}/sessions/{sid}/model", s.handleProjectSessionSetModel)

		// Diagrams — Mermaid + Excalidraw
		pr.Get("/api/diagrams", s.handleDiagramsList)
		pr.Post("/api/diagrams", s.handleDiagramsCreate)
		pr.Post("/api/diagrams/generate", s.handleDiagramsGenerate)
		pr.Get("/api/diagrams/{id}", s.handleDiagramGet)
		pr.Put("/api/diagrams/{id}", s.handleDiagramUpdate)
		pr.Delete("/api/diagrams/{id}", s.handleDiagramDelete)

		// Mini-agents — persistent scheduled/manual agents
		pr.Get("/api/agents", s.handleAgentsList)
		pr.Post("/api/agents", s.handleAgentsCreate)
		// Templates: literal route registered before /{id} so chi doesn't
		// treat "templates" as a numeric agent id.
		pr.Get("/api/agents/templates", s.handleAgentsTemplates)
		pr.Get("/api/agents/{id}", s.handleAgentGet)
		pr.Post("/api/agents/{id}/enabled", s.handleAgentSetEnabled)
		pr.Post("/api/agents/{id}/run", s.handleAgentRunNow)
		pr.Get("/api/agents/{id}/runs", s.handleAgentRuns)
		pr.Post("/api/agents/{id}/schedules", s.handleAgentSchedulesAdd)
		pr.Post("/api/agents/{id}/schedules/{sid}/enabled", s.handleAgentScheduleEnabled)
		pr.Delete("/api/agents/{id}/schedules/{sid}", s.handleAgentScheduleDelete)

		// Sub-agents — captured from JSONL post-spawn
		pr.Get("/api/subagents", s.handleSubagentsList)
		pr.Get("/api/subagents/{id}", s.handleSubagentGet)

		// Skills registry
		pr.Get("/api/skills", s.handleSkillsList)
		pr.Post("/api/skills/sync", s.handleSkillsSync)
		// Skill improvements (auto-mejora propose-and-approve)
		pr.Get("/api/skills/improvements", s.handleSkillImprovementsList)
		pr.Post("/api/skills/improvements", s.handleSkillImprovementsCreate)
		pr.Post("/api/skills/improvements/{id}/resolve", s.handleSkillImprovementResolve)

		// Project templates (clones rápidos de stacks pre-armados)
		pr.Get("/api/project-templates", s.handleProjectTemplatesList)
		pr.Get("/api/project-templates/{name}", s.handleProjectTemplateGet)
		pr.Post("/api/projects/{id}/apply-template", s.handleProjectApplyTemplate)

		// Canonical project docs (CLAUDE.md, SPECS.md, DESIGN.md, AGENTS.md, RELEASE_NOTES.md)
		pr.Get("/api/projects/{id}/docs", s.handleProjectDocsList)
		pr.Get("/api/projects/{id}/docs/{doc}", s.handleProjectDocGet)

		// BettaTech harness — feature_list.json sits at the repo root.
		pr.Get("/api/projects/{id}/features", s.handleProjectFeaturesGet)

		// Topics
		pr.Get("/api/topics", s.handleTopicsList)
		pr.Post("/api/topics", s.handleTopicsCreate)
		pr.Get("/api/topics/{id}/state", s.handleTopicGetState)
		pr.Post("/api/topics/{id}/state", s.handleTopicUpdateState)

		// Secrets vault
		pr.Get("/api/secrets", s.handleSecretsList)
		pr.Post("/api/secrets", s.handleSecretsCreate)
		pr.Get("/api/secrets/{key}/reveal", s.handleSecretReveal)
		pr.Delete("/api/secrets/{key}", s.handleSecretDelete)

		// Usage tracking
		pr.Get("/api/usage", s.handleUsageList)
		pr.Get("/api/usage/realtime", s.handleUsageRealtime)

		// System manager
		pr.Get("/api/system/stats", s.handleSystemStats)
		pr.Get("/api/system/services", s.handleSystemServices)
		pr.Post("/api/system/services/{name}/{action}", s.handleSystemServiceAction)
		pr.Get("/api/system/processes", s.handleSystemProcesses)
		pr.Get("/api/system/connections", s.handleSystemConnections)
		pr.Get("/api/system/cronjobs", s.handleSystemCronJobs)

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
	case urlPath == "/sw.js":
		setFrontendNoStore(w)
		w.Header().Set("Service-Worker-Allowed", "/")
	case urlPath == "/manifest.webmanifest":
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

// handleHealth runs a deep health check used by the blue/green smoke flow
// (see CLAUDE.md → Deploy workflow). It verifies the SQLite handle is alive,
// at least one migration has been applied, and reports WA connection state
// when the WhatsApp client is enabled. Returns 200 when every component is
// healthy and 503 otherwise — the smoke runner gates the deploy on `ok:true`.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]any{}
	allOK := true

	// DB ping
	if db := s.repos.DB(); db != nil {
		if err := db.PingContext(ctx); err != nil {
			checks["db"] = map[string]any{"ok": false, "err": err.Error()}
			allOK = false
		} else {
			checks["db"] = map[string]any{"ok": true}
		}
	} else {
		checks["db"] = map[string]any{"ok": false, "err": "no db handle"}
		allOK = false
	}

	// Migrations: at least one row in __migrations
	migrationsCount := 0
	if db := s.repos.DB(); db != nil {
		_ = db.QueryRowContext(ctx, `SELECT COUNT(1) FROM __migrations`).Scan(&migrationsCount)
	}
	if migrationsCount > 0 {
		checks["migrations"] = map[string]any{"ok": true, "applied": migrationsCount}
	} else {
		checks["migrations"] = map[string]any{"ok": false, "err": "no migrations applied"}
		allOK = false
	}

	// Scheduler: indirect — agent_runs is touched on every tick. We just expose
	// the row count so a stuck scheduler shows up as a flat number across deploys.
	var runsCount int
	if db := s.repos.DB(); db != nil {
		_ = db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_runs`).Scan(&runsCount)
	}
	checks["scheduler"] = map[string]any{"ok": true, "agent_runs": runsCount}

	// WhatsApp: only relevant when enabled. Smoke runs with WAEnabled=false so
	// the check stays informational. When enabled, ok=true requires the
	// whatsmeow socket to be connected. When disconnected we surface the QR
	// URL so the operator can pair from the same /healthz response.
	if s.cfg != nil && s.cfg.WAEnabled {
		connected := s.waConnected()
		waCheck := map[string]any{
			"ok":        connected,
			"enabled":   true,
			"connected": connected,
		}
		if !connected {
			if path := s.waQRPath(); path != "" {
				if _, err := os.Stat(path); err == nil {
					waCheck["qr_url"] = "/api/wa/qr"
				}
			}
			allOK = false
		}
		checks["wa"] = waCheck
	} else {
		checks["wa"] = map[string]any{"ok": true, "enabled": false}
	}

	status := http.StatusOK
	if !allOK {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"ok":         allOK,
		"ts":         time.Now().Unix(),
		"version":    buildinfo.Version,
		"git_commit": buildinfo.GitCommit,
		"checks":     checks,
	})
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
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	before := parseInt64Default(r.URL.Query().Get("before"), 0)
	msgs, err := s.repos.Messages.Range(r.Context(), "", before, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleMessagesSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"messages": []store.Message{}})
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	msgs, err := s.repos.Messages.Search(r.Context(), "", q, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs, "query": q})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseInt64Default(s string, def int64) int64 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return def
	}
	return n
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
	return s.routeConversationInput(ctx, conversationInput{
		Channel:     "web",
		Body:        req.Body,
		Attachments: req.Attachments,
		Async:       true,
	})
}

// waReplyTarget tells the shared conversation runtime to also emit the reply
// via WhatsApp. nil = no WA fan-out. When non-nil the agent's text reply is
// enqueued on wa_outbox with reply_to fields set so it threads correctly.
type waReplyTarget struct {
	JID      string // chat to reply to
	StanzaID string // external WA message id we are quoting
}

// enqueueWAReply pushes the agent's text reply onto wa_outbox so the existing
// outbox worker dispatches it via whatsmeow. We thread the reply by quoting
// the original incoming message when stanza id is known. Empty bodies are
// dropped — outbox would reject them anyway.
func (s *Server) enqueueWAReply(target *waReplyTarget, body string) {
	body = strings.TrimSpace(body)
	if body == "" || target == nil || target.JID == "" {
		return
	}
	item := store.WaOutboxItem{
		JID:  target.JID,
		Kind: "text",
		Body: sql.NullString{String: body, Valid: true},
	}
	if target.StanzaID != "" {
		item.ReplyTo = sql.NullString{String: target.StanzaID, Valid: true}
	}
	if _, err := s.repos.WaOutbox.Enqueue(context.Background(), item); err != nil {
		s.log.Warn("wa enqueue reply", "err", err)
	}
}

func (s *Server) mainAgentSession(ctx context.Context, engine, model string) *store.AgentSession {
	if engine != "" {
		if sess, _ := s.repos.Sessions.GetAgentSession(ctx, mainAgentSessionName(engine, model)); sess != nil && sess.Engine == engine {
			return sess
		}
	}
	// Legacy fallback: engine-scoped rows that predate model/provider-aware
	// session partitioning. Never reuse the generic claude row for DeepSeek
	// models because Anthropic-backed session_ids are incompatible there.
	if engine == "claude" && isDeepSeekMainModel(model) {
		return nil
	}
	if engine != "" {
		if sess, _ := s.repos.Sessions.GetAgentSession(ctx, mainAgentSessionName(engine, "")); sess != nil && sess.Engine == engine {
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

func mainAgentSessionName(engine, model string) string {
	if engine == "" {
		return "main-agent"
	}
	if engine == "claude" && isDeepSeekMainModel(model) {
		model = strings.ToLower(strings.TrimSpace(model))
		if model == "" {
			model = "deepseek"
		}
		return "main-agent:" + engine + ":" + model
	}
	return "main-agent:" + engine
}

func isDeepSeekMainModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "deepseek-")
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

// activityToolEntry mirrors GhostBubble.ToolCall in the persistence shape.
type activityToolEntry struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	Args          any    `json:"args,omitempty"`
	Status        string `json:"status"`
	ResultPreview string `json:"result_preview,omitempty"`
}

// turnActivity is the audit blob saved to wa_messages.activity (assistant turns).
type turnActivity struct {
	Thinking string              `json:"thinking,omitempty"`
	Tools    []activityToolEntry `json:"tools,omitempty"`
}

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

// streamEventBroadcasterWithActivity is like streamEventBroadcaster but also
// accumulates thinking + tool calls into the provided pointer so the caller
// can persist the turn's audit trail after Run returns.
func (s *Server) streamEventBroadcasterWithActivity(ref runtimeRunRef, acc *turnActivity) func(cliengine.StreamEvent) {
	broadcast := s.streamEventBroadcaster()
	snap := &runtimeSnapshot{}
	return func(ev cliengine.StreamEvent) {
		broadcast(ev)
		applyRuntimeEvent(snap, ev)
		s.persistRuntimeSnapshot(context.Background(), ref, "running", *snap, ev.SessionID, "", "", ev.Seq, false)
		switch ev.Kind {
		case "thinking":
			if ev.Text != "" {
				acc.Thinking += ev.Text
			}
		case "tool_use":
			acc.Tools = append(acc.Tools, activityToolEntry{
				ID:     ev.ToolID,
				Name:   ev.ToolName,
				Args:   ev.ToolArgs,
				Status: "running",
			})
		case "tool_result":
			// Find the tool call by ID (preferred) or fall back to last running.
			idx := -1
			if ev.ToolID != "" {
				for i := range acc.Tools {
					if acc.Tools[i].ID == ev.ToolID {
						idx = i
						break
					}
				}
			}
			if idx == -1 {
				for i := len(acc.Tools) - 1; i >= 0; i-- {
					if acc.Tools[i].Status == "running" {
						idx = i
						break
					}
				}
			}
			if idx >= 0 {
				acc.Tools[idx].Status = "ok"
				acc.Tools[idx].ResultPreview = truncate(ev.ToolResult, 1000)
			}
		}
	}
}

// broadcastMessage pushes a chat message envelope to all WS subscribers on the agent topic.
// handleRunsStatus returns the count of in-flight turns per kind.
// Used by bin/safe-restart.sh to know when it's safe to restart.
func (s *Server) handleRunsStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.runs.Snapshot()
	total := 0
	for _, v := range snap {
		total += v
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runs":            snap,
		"total":           total,
		"pending_restart": s.runs.PendingRestart(),
	})
}

// handleRunsCancel cancels an in-flight run identified by (scope, id). Used by
// the long_running_turn toast (and any other UI affordance that wants to kill
// a hanging turn) without needing a scope-specific endpoint.
//
// Scopes:
//   - "main"     id = engine ("claude" / "codex")
//   - "project"  id = project_session_id (int64 as string)
//   - "agent"    id = agent_run_id (int64 as string)
//   - "openspec" id = "<change_id>:<phase>" or "<change_id>:apply-verify"
func (s *Server) handleRunsCancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"`
		ID    string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	scope := strings.TrimSpace(req.Scope)
	id := strings.TrimSpace(req.ID)
	if scope == "" || id == "" {
		http.Error(w, "scope and id required", http.StatusBadRequest)
		return
	}
	cancelled := s.runs.Cancel(scope, id)
	if !cancelled {
		http.Error(w, "no run registered under that scope+id", http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": true, "scope": scope, "id": id})
}

// handleScheduleRestart schedules a daemon restart to occur once all in-flight
// turns complete. If no turns are active, the restart is immediate.
// This endpoint is public (no JWT) so safe-restart.sh can call it without credentials.
// It is only accessible locally since the daemon binds to 127.0.0.1 in prod.
func (s *Server) handleScheduleRestart(w http.ResponseWriter, r *http.Request) {
	snap := s.runs.Snapshot()
	total := 0
	for _, v := range snap {
		total += v
	}
	s.runs.ScheduleRestart(func() {
		s.log.Info("safe-restart: all turns done, restarting via systemctl")
		if err := exec.Command("sudo", "systemctl", "restart", "agenthub").Run(); err != nil {
			s.log.Error("safe-restart: systemctl restart failed", "err", err)
		}
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"scheduled":                true,
		"active_runs":              total,
		"will_restart_immediately": total == 0,
	})
}

// handleReleases serves RELEASE_NOTES.md content along with current build info.
// Public endpoint — no JWT required.
func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("RELEASE_NOTES.md")
	if err != nil {
		http.Error(w, "release notes not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"content":    string(data),
		"version":    buildinfo.Version,
		"git_commit": buildinfo.GitCommit,
	})
}

func (s *Server) broadcastMessage(id int64, channel, direction, body string) {
	s.broadcastMessageWithModel(id, channel, direction, body, "", "")
}

func (s *Server) broadcastStoredMessage(msg store.Message) {
	if s.hub == nil {
		return
	}
	payload := messageBroadcastPayload(msg)
	raw, _ := json.Marshal(payload)
	s.hub.Broadcast(ws.Envelope{Type: "message", Topic: "agent", Payload: raw})
}

// broadcastMessageWithModel includes engine/model so the UI can show the
// model that produced an assistant turn.
func (s *Server) broadcastMessageWithModel(id int64, channel, direction, body, engine, model string) {
	if s.hub == nil {
		return
	}
	payload, _ := json.Marshal(messageBroadcastPayload(store.Message{
		ID:        id,
		Channel:   channel,
		Direction: direction,
		Body:      sqlStr(body),
		TS:        time.Now().Unix(),
		Engine:    sqlStr(engine),
		Model:     sqlStr(model),
	}))
	s.hub.Broadcast(ws.Envelope{Type: "message", Topic: "agent", Payload: payload})
}

func messageBroadcastPayload(msg store.Message) map[string]any {
	out := map[string]any{
		"id":        msg.ID,
		"channel":   msg.Channel,
		"direction": msg.Direction,
		"body":      msg.Body.String,
		"ts":        msg.TS,
		"is_read":   msg.IsRead,
	}
	if msg.Engine.Valid {
		out["engine"] = msg.Engine.String
	}
	if msg.Model.Valid {
		out["model"] = msg.Model.String
	}
	if msg.Activity.Valid {
		out["activity"] = msg.Activity.String
	}
	if msg.MediaType.Valid {
		out["media_type"] = msg.MediaType.String
	}
	if msg.MediaPath.Valid {
		out["media_path"] = msg.MediaPath.String
	}
	if msg.MediaCaption.Valid {
		out["media_caption"] = msg.MediaCaption.String
	}
	return out
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
