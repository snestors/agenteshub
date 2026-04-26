package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/ws"
)

type projectWire struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Description   string `json:"description,omitempty"`
	DefaultEngine string `json:"default_engine"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	SessionsCount int64  `json:"sessions_count,omitempty"`
}

type projectSessionWire struct {
	ID           int64  `json:"id"`
	ProjectID    int64  `json:"project_id"`
	Name         string `json:"name"`
	SessionID    string `json:"session_id"`
	Engine       string `json:"engine"`
	Summary      string `json:"summary,omitempty"`
	LastActiveAt int64  `json:"last_active_at,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

type sessionMessageWire struct {
	ID            int64  `json:"id"`
	Scope         string `json:"scope"`
	ProjectID     int64  `json:"project_id,omitempty"`
	ProjectSessID int64  `json:"project_sess_id,omitempty"`
	SessionID     string `json:"session_id"`
	Role          string `json:"role"`
	Direction     string `json:"direction"`
	Channel       string `json:"channel"`
	Body          string `json:"body"`
	CostTokens    int64  `json:"cost_tokens,omitempty"`
	TS            int64  `json:"ts"`
}

type createProjectReq struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Description   string `json:"description"`
	DefaultEngine string `json:"default_engine"`
}

type createProjectSessionReq struct {
	Name    string `json:"name"`
	Engine  string `json:"engine"`
	Summary string `json:"summary"`
}

type sendProjectMessageReq struct {
	Body string `json:"body"`
}

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	projects, err := s.repos.Projects.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]projectWire, 0, len(projects))
	for _, p := range projects {
		pw := projectToWire(p)
		pw.SessionsCount, _ = s.repos.Projects.SessionCount(r.Context(), p.ID)
		out = append(out, pw)
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func (s *Server) handleProjectsCreate(w http.ResponseWriter, r *http.Request) {
	var req createProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	if req.DefaultEngine == "" {
		req.DefaultEngine = s.cfg.DefaultEngine
	}
	if req.Name == "" || req.Path == "" {
		http.Error(w, "name and path required", http.StatusBadRequest)
		return
	}
	if !validEngine(req.DefaultEngine) {
		http.Error(w, "default_engine not supported", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(req.Path)
	if err != nil || !info.IsDir() {
		http.Error(w, "path must exist and be a directory", http.StatusBadRequest)
		return
	}
	id, err := s.repos.Projects.Create(r.Context(), store.Project{
		Name:          req.Name,
		Path:          req.Path,
		Description:   sql.NullString{String: strings.TrimSpace(req.Description), Valid: strings.TrimSpace(req.Description) != ""},
		DefaultEngine: req.DefaultEngine,
	})
	if err != nil {
		if isUniqueConstraint(err) {
			http.Error(w, "project name already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, err := s.repos.Projects.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": projectToWire(*p)})
}

func (s *Server) handleProjectGet(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	sessions, err := s.repos.Projects.ListSessions(r.Context(), project.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]projectSessionWire, 0, len(sessions))
	for _, ps := range sessions {
		out = append(out, projectSessionToWire(ps))
	}
	pw := projectToWire(*project)
	pw.SessionsCount = int64(len(out))
	writeJSON(w, http.StatusOK, map[string]any{"project": pw, "sessions": out})
}

func (s *Server) handleProjectSessionsList(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	sessions, err := s.repos.Projects.ListSessions(r.Context(), project.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]projectSessionWire, 0, len(sessions))
	for _, ps := range sessions {
		out = append(out, projectSessionToWire(ps))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleProjectSessionsCreate(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	var req createProjectSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "session-" + time.Now().Format("20060102-150405")
	}
	engine := strings.TrimSpace(req.Engine)
	if engine == "" {
		engine = project.DefaultEngine
	}
	if !validEngine(engine) {
		http.Error(w, "engine not supported", http.StatusBadRequest)
		return
	}
	now := time.Now().Unix()
	id, err := s.repos.Projects.CreateSession(r.Context(), store.ProjectSession{
		ProjectID:    project.ID,
		Name:         name,
		SessionID:    "",
		Engine:       engine,
		Summary:      sql.NullString{String: strings.TrimSpace(req.Summary), Valid: strings.TrimSpace(req.Summary) != ""},
		LastActiveAt: sql.NullInt64{Int64: now, Valid: true},
	})
	if err != nil {
		if isUniqueConstraint(err) {
			http.Error(w, "session name already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ps, err := s.repos.Projects.GetSession(r.Context(), project.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session": projectSessionToWire(*ps)})
}

func (s *Server) handleProjectSessionMessagesList(w http.ResponseWriter, r *http.Request) {
	project, sess, ok := s.projectSessionFromRequest(w, r)
	if !ok {
		return
	}
	msgs, err := s.repos.Sessions.MessagesForProjectSession(r.Context(), sess.ID, 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]sessionMessageWire, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, sessionMessageToWire(project.ID, sess.ID, m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": out})
}

func (s *Server) handleProjectSessionMessagesSend(w http.ResponseWriter, r *http.Request) {
	project, sess, ok := s.projectSessionFromRequest(w, r)
	if !ok {
		return
	}
	var req sendProjectMessageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	topic := projectSessionTopic(sess.ID)
	go s.runProjectSessionTurn(project, sess, body, topic)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":   true,
		"project_id": project.ID,
		"session_id": sess.ID,
		"topic":      topic,
	})
}

func (s *Server) runProjectSessionTurn(project *store.Project, sess *store.ProjectSession, body, topic string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	engineName := sess.Engine
	if engineName == "" {
		engineName = project.DefaultEngine
	}
	prev := sess.SessionID
	res, err := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:        body,
		SessionID:     prev,
		Channel:       "project",
		Cwd:           project.Path,
		Engine:        engineName,
		Scope:         "project",
		ProjectID:     project.ID,
		ProjectSessID: sess.ID,
		OnEvent:       s.streamEventBroadcasterForTopic(topic),
	})
	if err != nil {
		s.log.Warn("project run", "project", project.Name, "session", sess.Name, "err", err)
		raw, _ := json.Marshal(sessionMessageWire{
			ID:            time.Now().UnixNano() * -1,
			ProjectID:     project.ID,
			ProjectSessID: sess.ID,
			SessionID:     prev,
			Role:          "assistant",
			Direction:     "out",
			Channel:       "project",
			Body:          "⚠ engine: " + err.Error(),
			TS:            time.Now().Unix(),
		})
		s.hub.Broadcast(ws.Envelope{Type: "message", Topic: topic, Payload: raw})
		return
	}
	if res != nil && res.SessionID != "" && res.SessionID != prev {
		_ = s.repos.Projects.UpdateSessionID(context.Background(), sess.ID, res.SessionID)
		_ = s.repos.Sessions.BackfillProjectSessionID(context.Background(), sess.ID, res.SessionID)
	} else {
		_ = s.repos.Projects.TouchSession(context.Background(), sess.ID)
	}
	s.broadcastLatestProjectMessage(context.Background(), project.ID, sess.ID, topic)
}

func (s *Server) broadcastLatestProjectMessage(ctx context.Context, projectID, sessID int64, topic string) {
	msgs, err := s.repos.Sessions.MessagesForProjectSession(ctx, sessID, 500)
	if err != nil || len(msgs) == 0 {
		return
	}
	m := msgs[len(msgs)-1]
	raw, _ := json.Marshal(sessionMessageToWire(projectID, sessID, m))
	s.hub.Broadcast(ws.Envelope{Type: "message", Topic: topic, Payload: raw})
}

func (s *Server) streamEventBroadcasterForTopic(topic string) func(cliengine.StreamEvent) {
	if s.hub == nil {
		return func(cliengine.StreamEvent) {}
	}
	return func(ev cliengine.StreamEvent) {
		raw, err := json.Marshal(ev)
		if err != nil {
			return
		}
		s.hub.Broadcast(ws.Envelope{Type: "stream", Topic: topic, Payload: raw})
	}
}

func (s *Server) projectFromRequest(w http.ResponseWriter, r *http.Request) (*store.Project, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad project id", http.StatusBadRequest)
		return nil, false
	}
	project, err := s.repos.Projects.GetByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "project not found", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	return project, true
}

func (s *Server) projectSessionFromRequest(w http.ResponseWriter, r *http.Request) (*store.Project, *store.ProjectSession, bool) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return nil, nil, false
	}
	sid, err := strconv.ParseInt(chi.URLParam(r, "sid"), 10, 64)
	if err != nil || sid <= 0 {
		http.Error(w, "bad session id", http.StatusBadRequest)
		return nil, nil, false
	}
	sess, err := s.repos.Projects.GetSession(r.Context(), project.ID, sid)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "session not found", http.StatusNotFound)
		return nil, nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, nil, false
	}
	return project, sess, true
}

func projectToWire(p store.Project) projectWire {
	return projectWire{
		ID:            p.ID,
		Name:          p.Name,
		Path:          p.Path,
		Description:   nullString(p.Description),
		DefaultEngine: p.DefaultEngine,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

func projectSessionToWire(s store.ProjectSession) projectSessionWire {
	return projectSessionWire{
		ID:           s.ID,
		ProjectID:    s.ProjectID,
		Name:         s.Name,
		SessionID:    s.SessionID,
		Engine:       s.Engine,
		Summary:      nullString(s.Summary),
		LastActiveAt: nullInt(s.LastActiveAt),
		CreatedAt:    s.CreatedAt,
	}
}

func sessionMessageToWire(projectID, projectSessID int64, m store.SessionMessage) sessionMessageWire {
	direction := "out"
	if m.Role == "user" {
		direction = "in"
	}
	return sessionMessageWire{
		ID:            m.ID,
		Scope:         m.Scope,
		ProjectID:     projectID,
		ProjectSessID: projectSessID,
		SessionID:     m.SessionID,
		Role:          m.Role,
		Direction:     direction,
		Channel:       "project",
		Body:          nullString(m.Body),
		CostTokens:    m.CostTokens,
		TS:            m.TS,
	}
}

func projectSessionTopic(id int64) string {
	return "project_session:" + strconv.FormatInt(id, 10)
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func nullInt(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "constraint failed")
}
