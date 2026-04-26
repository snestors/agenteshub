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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"golang.org/x/crypto/bcrypt"

	"github.com/snestors/agenthub/internal/auth"
	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/store"
)

const cookieName = "agenthub_token"

// Server owns the HTTP server + dependencies.
type Server struct {
	cfg    *config.Config
	repos  *store.Repos
	tokens *auth.TokenService
	log    *slog.Logger
	httpSrv *http.Server
}

// New constructs a Server.
func New(cfg *config.Config, repos *store.Repos, log *slog.Logger) (*Server, error) {
	tokens := auth.NewTokenService(cfg, repos.Auth)
	s := &Server{cfg: cfg, repos: repos, tokens: tokens, log: log}
	s.httpSrv = &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

// Run blocks serving HTTP until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.httpSrv.Close()
	}()
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
	})

	// frontend (sirve dist/ si existe; si no, mensaje placeholder)
	r.Get("/", s.serveFrontend)
	r.Get("/*", s.serveFrontend)
	return r
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
	Body string `json:"body"`
}

func (s *Server) handleMessagesSend(w http.ResponseWriter, r *http.Request) {
	var req sendMsgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Body) == "" {
		http.Error(w, "empty", http.StatusBadRequest)
		return
	}
	id, err := s.repos.Messages.Insert(r.Context(), store.Message{
		Channel:   "web",
		Direction: "in",
		Body:      sqlStr(req.Body),
		TS:        time.Now().Unix(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Cliengine no integrado todavía — solo eco para validar pipe E2E.
	_, _ = s.repos.Messages.Insert(r.Context(), store.Message{
		Channel:   "web",
		Direction: "out",
		Body:      sqlStr("✓ recibido [eco · cliengine pendiente]: " + req.Body),
		TS:        time.Now().Unix(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "echoed": true})
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
