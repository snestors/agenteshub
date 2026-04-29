package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type pushRegisterReq struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

func (s *Server) handlePushRegister(w http.ResponseWriter, r *http.Request) {
	var req pushRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Provider == "" {
		req.Provider = "fcm"
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Provider != "fcm" || req.Token == "" {
		http.Error(w, "invalid push token", http.StatusBadRequest)
		return
	}
	if s.repos == nil || s.repos.Push == nil {
		http.Error(w, "push repo unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := s.repos.Push.Upsert(r.Context(), req.Provider, req.Token, r.UserAgent()); err != nil {
		s.log.Warn("push register", "err", err)
		http.Error(w, "register failed", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	n := Notification{
		Kind:     "push_test",
		Severity: "info",
		Title:    "AgentHub push activo",
		Body:     "FCM quedó conectado con RelogTemperatura.",
		Context:  map[string]any{"route": "/"},
	}
	s.broadcastNotification(n)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) sendPushNotification(ctx context.Context, n Notification) {
	if s.cfg == nil || !s.cfg.FCMEnabled || s.cfg.FCMProjectID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if s.repos == nil || s.repos.Push == nil {
		return
	}
	tokens, err := s.repos.Push.Active(ctx, "fcm")
	if err != nil || len(tokens) == 0 {
		if err != nil {
			s.log.Warn("push tokens", "err", err)
		}
		return
	}
	accessToken, err := s.firebaseAccessToken(ctx)
	if err != nil {
		s.log.Warn("fcm auth", "err", err)
		return
	}
	for _, tok := range tokens {
		if err := s.sendFCM(ctx, accessToken, tok.Token, n); err != nil {
			disable := strings.Contains(err.Error(), "UNREGISTERED") || strings.Contains(err.Error(), "INVALID_ARGUMENT")
			_ = s.repos.Push.MarkError(ctx, tok.Token, truncate(err.Error(), 500), disable)
			s.log.Warn("fcm send", "token_id", tok.ID, "disable", disable, "err", err)
		}
	}
}

type firebaseToolsConfig struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	} `json:"tokens"`
}

func (s *Server) firebaseAccessToken(ctx context.Context) (string, error) {
	read := func() (firebaseToolsConfig, error) {
		var cfg firebaseToolsConfig
		raw, err := os.ReadFile(s.cfg.FCMFirebaseToolsConfig)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	cfg, err := read()
	if err != nil {
		return "", err
	}
	if cfg.Tokens.AccessToken != "" && time.Now().Add(2*time.Minute).Before(time.UnixMilli(cfg.Tokens.ExpiresAt)) {
		return cfg.Tokens.AccessToken, nil
	}
	cli := s.cfg.FCMFirebaseCLI
	if cli == "" {
		cli = "firebase"
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, cli, "projects:list", "--json")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("firebase cli refresh: %w: %s", err, strings.TrimSpace(string(out)))
	}
	cfg, err = read()
	if err != nil {
		return "", err
	}
	if cfg.Tokens.AccessToken == "" {
		return "", fmt.Errorf("firebase cli token missing")
	}
	return cfg.Tokens.AccessToken, nil
}

func (s *Server) sendFCM(ctx context.Context, accessToken, token string, n Notification) error {
	data := map[string]string{
		"id":       n.ID,
		"kind":     n.Kind,
		"severity": n.Severity,
		"title":    n.Title,
		"body":     n.Body,
		"link":     routeForPush(n),
	}
	absLink := data["link"]
	if strings.HasPrefix(absLink, "/") && s.cfg.FCMPublicURL != "" {
		absLink = strings.TrimRight(s.cfg.FCMPublicURL, "/") + absLink
	}
	for k, v := range n.Context {
		data["ctx_"+k] = fmt.Sprint(v)
	}
	body := map[string]any{
		"message": map[string]any{
			"token": token,
			"notification": map[string]string{
				"title": n.Title,
				"body":  n.Body,
			},
			"data": data,
			"webpush": map[string]any{
				"notification": map[string]any{
					"icon":  "/pwa-192.png",
					"badge": "/pwa-192.png",
				},
				"fcm_options": map[string]string{"link": absLink},
			},
		},
	}
	raw, _ := json.Marshal(body)
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", s.cfg.FCMProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return fmt.Errorf("fcm status %d: %v", res.StatusCode, out)
	}
	return nil
}

func routeForPush(n Notification) string {
	if n.Context != nil {
		if route, ok := n.Context["route"].(string); ok && route != "" {
			return route
		}
	}
	if strings.HasPrefix(n.Kind, "main_turn") {
		return "/"
	}
	if strings.HasPrefix(n.Kind, "agent_run") {
		if id, ok := n.Context["agent_id"]; ok {
			return "/agents/" + fmt.Sprint(id)
		}
		return "/agents"
	}
	if strings.HasPrefix(n.Kind, "project_turn") {
		pid := fmt.Sprint(n.Context["project_id"])
		sid := fmt.Sprint(n.Context["session_id"])
		if pid != "<nil>" && sid != "<nil>" {
			return "/projects/" + pid + "/sessions/" + sid
		}
		if pid != "<nil>" {
			return "/projects/" + pid
		}
		return "/projects"
	}
	return "/"
}
