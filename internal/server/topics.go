package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenthub/internal/store"
)

type topicWire struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Keywords     []string `json:"keywords,omitempty"`
	ProjectID    *int64   `json:"project_id,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	Engine       string   `json:"engine"`
	IsDefault    bool     `json:"is_default"`
	LastActiveAt *int64   `json:"last_active_at,omitempty"`
	CreatedAt    int64    `json:"created_at"`
}

type topicStateWire struct {
	TopicID         int64    `json:"topic_id"`
	Headline        string   `json:"headline,omitempty"`
	ActiveIssues    []string `json:"active_issues,omitempty"`
	RecentDecisions []string `json:"recent_decisions,omitempty"`
	Pending         []string `json:"pending,omitempty"`
	NextActionHint  string   `json:"next_action_hint,omitempty"`
	LastEventAt     *int64   `json:"last_event_at,omitempty"`
	UpdatedAt       int64    `json:"updated_at"`
}

func parseStringArray(s sql.NullString) []string {
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s.String), &out); err != nil {
		return nil
	}
	return out
}

func topicToWire(t store.Topic) topicWire {
	w := topicWire{
		ID:        t.ID,
		Name:      t.Name,
		SessionID: t.SessionID,
		Engine:    t.Engine,
		IsDefault: t.IsDefault,
		CreatedAt: t.CreatedAt,
	}
	if t.Description.Valid {
		w.Description = t.Description.String
	}
	if t.Keywords.Valid {
		w.Keywords = parseStringArray(t.Keywords)
	}
	if t.ProjectID.Valid {
		v := t.ProjectID.Int64
		w.ProjectID = &v
	}
	if t.LastActiveAt.Valid {
		v := t.LastActiveAt.Int64
		w.LastActiveAt = &v
	}
	return w
}

func topicStateToWire(s store.TopicState) topicStateWire {
	w := topicStateWire{TopicID: s.TopicID, UpdatedAt: s.UpdatedAt}
	if s.Headline.Valid {
		w.Headline = s.Headline.String
	}
	if s.ActiveIssues.Valid {
		w.ActiveIssues = parseStringArray(s.ActiveIssues)
	}
	if s.RecentDecisions.Valid {
		w.RecentDecisions = parseStringArray(s.RecentDecisions)
	}
	if s.Pending.Valid {
		w.Pending = parseStringArray(s.Pending)
	}
	if s.NextActionHint.Valid {
		w.NextActionHint = s.NextActionHint.String
	}
	if s.LastEventAt.Valid {
		v := s.LastEventAt.Int64
		w.LastEventAt = &v
	}
	return w
}

func (s *Server) handleTopicsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.repos.Topics.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]topicWire, 0, len(rows))
	for _, t := range rows {
		out = append(out, topicToWire(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": out})
}

type topicCreateReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Engine      string   `json:"engine,omitempty"`
}

func (s *Server) handleTopicsCreate(w http.ResponseWriter, r *http.Request) {
	var req topicCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	engine := strings.TrimSpace(req.Engine)
	if engine == "" {
		engine = "claude"
	}
	t := store.Topic{
		Name:      name,
		Engine:    engine,
		CreatedAt: time.Now().Unix(),
	}
	if d := strings.TrimSpace(req.Description); d != "" {
		t.Description = sqlStr(d)
	}
	if len(req.Keywords) > 0 {
		if buf, err := json.Marshal(req.Keywords); err == nil {
			t.Keywords = sqlStr(string(buf))
		}
	}
	id, err := s.repos.Topics.Create(r.Context(), t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": name})
}

func (s *Server) handleTopicGetState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad topic id", http.StatusBadRequest)
		return
	}
	st, err := s.repos.Topics.GetState(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st == nil {
		// no state yet — return an empty object
		writeJSON(w, http.StatusOK, topicStateWire{TopicID: id})
		return
	}
	writeJSON(w, http.StatusOK, topicStateToWire(*st))
}

type topicStateUpdateReq struct {
	Headline        string   `json:"headline,omitempty"`
	ActiveIssues    []string `json:"active_issues,omitempty"`
	RecentDecisions []string `json:"recent_decisions,omitempty"`
	Pending         []string `json:"pending,omitempty"`
	NextActionHint  string   `json:"next_action_hint,omitempty"`
}

func (s *Server) handleTopicUpdateState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad topic id", http.StatusBadRequest)
		return
	}
	var req topicStateUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	st := store.TopicState{
		TopicID:   id,
		UpdatedAt: time.Now().Unix(),
	}
	if h := strings.TrimSpace(req.Headline); h != "" {
		st.Headline = sqlStr(h)
	}
	if len(req.ActiveIssues) > 0 {
		if buf, err := json.Marshal(req.ActiveIssues); err == nil {
			st.ActiveIssues = sqlStr(string(buf))
		}
	}
	if len(req.RecentDecisions) > 0 {
		if buf, err := json.Marshal(req.RecentDecisions); err == nil {
			st.RecentDecisions = sqlStr(string(buf))
		}
	}
	if len(req.Pending) > 0 {
		if buf, err := json.Marshal(req.Pending); err == nil {
			st.Pending = sqlStr(string(buf))
		}
	}
	if h := strings.TrimSpace(req.NextActionHint); h != "" {
		st.NextActionHint = sqlStr(h)
	}
	if err := s.repos.Topics.UpsertState(r.Context(), st); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
