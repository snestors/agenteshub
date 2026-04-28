package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"

	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/store"
	"github.com/snestors/agenteshub/internal/ws"
)

type agentWire struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
	Engine         string `json:"engine"`
	Enabled        bool   `json:"enabled"`
	ProjectID      int64  `json:"project_id,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	NextRun        int64  `json:"next_run,omitempty"`
	SchedulesCount int    `json:"schedules_count,omitempty"`
	Runs24h        int64  `json:"runs_24h,omitempty"`
}

type agentScheduleWire struct {
	ID             int64  `json:"id"`
	AgentID        int64  `json:"agent_id"`
	CronExpr       string `json:"cron_expr"`
	PromptTemplate string `json:"prompt_template"`
	NotifyTarget   string `json:"notify_target"`
	Enabled        bool   `json:"enabled"`
	LastRunAt      int64  `json:"last_run_at,omitempty"`
	NextRun        int64  `json:"next_run"`
}

type agentRunWire struct {
	ID         int64  `json:"id"`
	AgentID    int64  `json:"agent_id"`
	ScheduleID int64  `json:"schedule_id,omitempty"`
	Trigger    string `json:"trigger"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	Status     string `json:"status"`
	Prompt     string `json:"prompt"`
	Result     string `json:"result,omitempty"`
	ToolsUsed  string `json:"tools_used,omitempty"`
	CostTokens int64  `json:"cost_tokens"`
	Error      string `json:"error,omitempty"`
}

type createAgentReq struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Engine       string `json:"engine"`
	ProjectID    int64  `json:"project_id"`
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

type runAgentReq struct {
	Prompt string `json:"prompt"`
}

type addScheduleReq struct {
	CronExpr       string `json:"cron_expr"`
	PromptTemplate string `json:"prompt_template"`
	NotifyTarget   string `json:"notify_target"`
}

func (s *Server) handleAgentsList(w http.ResponseWriter, r *http.Request) {
	agents, err := s.repos.Agents.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]agentWire, 0, len(agents))
	since := time.Now().Add(-24 * time.Hour).Unix()
	for _, a := range agents {
		aw := agentToWire(a, false)
		if schedules, err := s.repos.Agents.SchedulesForAgent(r.Context(), a.ID); err == nil {
			aw.SchedulesCount = len(schedules)
			for _, sch := range schedules {
				if sch.Enabled && (aw.NextRun == 0 || sch.NextRun < aw.NextRun) {
					aw.NextRun = sch.NextRun
				}
			}
		}
		aw.Runs24h, _ = s.repos.Agents.RunsSinceCount(r.Context(), a.ID, since)
		out = append(out, aw)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": out})
}

func (s *Server) handleAgentsCreate(w http.ResponseWriter, r *http.Request) {
	var req createAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.SystemPrompt = strings.TrimSpace(req.SystemPrompt)
	if req.Engine == "" {
		req.Engine = s.cfg.DefaultEngine
	}
	if req.Name == "" || req.SystemPrompt == "" {
		http.Error(w, "name and system_prompt required", http.StatusBadRequest)
		return
	}
	if !validEngine(req.Engine) {
		http.Error(w, "engine not supported", http.StatusBadRequest)
		return
	}
	id, err := s.repos.Agents.Create(r.Context(), store.Agent{
		Name:         req.Name,
		Description:  sql.NullString{String: strings.TrimSpace(req.Description), Valid: strings.TrimSpace(req.Description) != ""},
		SystemPrompt: req.SystemPrompt,
		Engine:       req.Engine,
		Enabled:      true,
		CreatedBy:    sql.NullString{String: "web", Valid: true},
		ProjectID:    sql.NullInt64{Int64: req.ProjectID, Valid: req.ProjectID > 0},
	})
	if err != nil {
		if isUniqueConstraint(err) {
			http.Error(w, "agent name already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	agent, err := s.repos.Agents.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agent": agentToWire(*agent, true)})
}

func (s *Server) handleAgentGet(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	schedules, err := s.repos.Agents.SchedulesForAgent(r.Context(), agent.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	runs, err := s.repos.Agents.RunsForAgent(r.Context(), agent.ID, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent":     agentToWire(*agent, true),
		"schedules": schedulesToWire(schedules),
		"runs":      runsToWire(runs),
	})
}

func (s *Server) handleAgentSetEnabled(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	var req enabledReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.repos.Agents.SetEnabled(r.Context(), agent.ID, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": req.Enabled})
}

func (s *Server) handleAgentRunNow(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	var req runAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	prompt := buildAgentPrompt(agent.SystemPrompt, req.Prompt)
	runID, err := s.repos.Agents.InsertRun(r.Context(), store.AgentRun{
		AgentID:   agent.ID,
		Trigger:   "manual",
		StartedAt: time.Now().Unix(),
		Status:    "running",
		Prompt:    prompt,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	topic := agentRunTopic(runID)
	go s.runAgentManual(context.Background(), agent, runID, prompt, topic)
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "run_id": runID, "topic": topic})
}

func (s *Server) handleAgentRuns(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := s.repos.Agents.RunsForAgent(r.Context(), agent.ID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runsToWire(runs)})
}

func (s *Server) handleAgentSchedulesAdd(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	var req addScheduleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.CronExpr = strings.TrimSpace(req.CronExpr)
	req.PromptTemplate = strings.TrimSpace(req.PromptTemplate)
	req.NotifyTarget = strings.TrimSpace(req.NotifyTarget)
	if req.NotifyTarget == "" {
		req.NotifyTarget = "main-agent"
	}
	if req.CronExpr == "" || req.PromptTemplate == "" {
		http.Error(w, "cron_expr and prompt_template required", http.StatusBadRequest)
		return
	}
	next, err := nextCron(req.CronExpr, time.Now())
	if err != nil {
		http.Error(w, "invalid cron_expr: "+err.Error(), http.StatusBadRequest)
		return
	}
	id, err := s.repos.Agents.AddSchedule(r.Context(), store.AgentSchedule{
		AgentID:        agent.ID,
		CronExpr:       req.CronExpr,
		PromptTemplate: req.PromptTemplate,
		NotifyTarget:   req.NotifyTarget,
		Enabled:        true,
		NextRun:        next,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	schedules, _ := s.repos.Agents.SchedulesForAgent(r.Context(), agent.ID)
	for _, sch := range schedules {
		if sch.ID == id {
			writeJSON(w, http.StatusCreated, map[string]any{"schedule": scheduleToWire(sch)})
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleAgentScheduleEnabled(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	sid, ok := scheduleIDFromRequest(w, r)
	if !ok {
		return
	}
	var req enabledReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.repos.Agents.SetScheduleEnabled(r.Context(), agent.ID, sid, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": req.Enabled})
}

func (s *Server) handleAgentScheduleDelete(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.agentFromRequest(w, r)
	if !ok {
		return
	}
	sid, ok := scheduleIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.repos.Agents.DeleteSchedule(r.Context(), agent.ID, sid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) runAgentManual(ctx context.Context, agent *store.Agent, runID int64, prompt, topic string) {
	// No deadline: mini-agents that wrap long shell scripts or delegate to Task
	// can run past any sensible cutoff. watchLongRunning nags the user every
	// hour; the generic /api/runs/cancel endpoint cancels by run id.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	runIDStr := strconv.FormatInt(runID, 10)
	s.runs.RegisterCancel("agent", runIDStr, cancel)
	defer s.runs.UnregisterCancel("agent", runIDStr)
	go s.watchLongRunning(runCtx, "agent", runIDStr, "Mini-agent · "+agent.Name, "Cancelá el turn desde el toast.")
	res, err := s.engines.Run(runCtx, cliengine.RunOpts{
		Prompt:    prompt,
		Channel:   "agent",
		Cwd:       ".",
		Engine:    agent.Engine,
		Scope:     "agent",
		AgentName: agent.Name,
		OnEvent:   s.streamEventBroadcasterForTopic(topic),
	})
	status := "ok"
	result := ""
	tokens := int64(0)
	errStr := ""
	if err != nil {
		status = "error"
		errStr = err.Error()
	} else if res != nil {
		result = res.Text
		tokens = res.CostTokens
	}
	if ferr := s.repos.Agents.FinishRun(ctx, runID, status, result, tokens, errStr); ferr != nil {
		s.log.Warn("finish manual agent run", "run_id", runID, "err", ferr)
	}
	raw, _ := json.Marshal(map[string]any{
		"run_id":      runID,
		"agent_id":    agent.ID,
		"status":      status,
		"result":      result,
		"error":       errStr,
		"cost_tokens": tokens,
		"finished_at": time.Now().Unix(),
	})
	s.hub.Broadcast(ws.Envelope{Type: "run", Topic: topic, Payload: raw})

	kind := "agent_run_done"
	severity := "info"
	body := truncate(result, 280)
	if status != "ok" {
		kind = "agent_run_failed"
		severity = "error"
		body = truncate(errStr, 280)
	}
	s.broadcastNotification(Notification{
		Kind:     kind,
		Severity: severity,
		Title:    "Agent · " + agent.Name,
		Body:     body,
		Context: map[string]any{
			"agent_id": agent.ID,
			"run_id":   runID,
			"trigger":  "manual",
			"status":   status,
		},
	})
}

func (s *Server) agentFromRequest(w http.ResponseWriter, r *http.Request) (*store.Agent, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad agent id", http.StatusBadRequest)
		return nil, false
	}
	agent, err := s.repos.Agents.GetByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "agent not found", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	return agent, true
}

func scheduleIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "sid"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad schedule id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func buildAgentPrompt(systemPrompt, prompt string) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return systemPrompt
	}
	if systemPrompt == "" {
		return prompt
	}
	return systemPrompt + "\n\n" + prompt
}

func nextCron(expr string, base time.Time) (int64, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		return 0, err
	}
	return sched.Next(base).Unix(), nil
}

func agentRunTopic(id int64) string {
	return "agent_run:" + strconv.FormatInt(id, 10)
}

func agentToWire(a store.Agent, detail bool) agentWire {
	out := agentWire{
		ID:          a.ID,
		Name:        a.Name,
		Description: nullString(a.Description),
		Engine:      a.Engine,
		Enabled:     a.Enabled,
		ProjectID:   nullInt(a.ProjectID),
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
	if detail {
		out.SystemPrompt = a.SystemPrompt
	}
	return out
}

func scheduleToWire(s store.AgentSchedule) agentScheduleWire {
	return agentScheduleWire{
		ID:             s.ID,
		AgentID:        s.AgentID,
		CronExpr:       s.CronExpr,
		PromptTemplate: s.PromptTemplate,
		NotifyTarget:   s.NotifyTarget,
		Enabled:        s.Enabled,
		LastRunAt:      nullInt(s.LastRunAt),
		NextRun:        s.NextRun,
	}
}

func schedulesToWire(in []store.AgentSchedule) []agentScheduleWire {
	out := make([]agentScheduleWire, 0, len(in))
	for _, s := range in {
		out = append(out, scheduleToWire(s))
	}
	return out
}

func runToWire(r store.AgentRun) agentRunWire {
	return agentRunWire{
		ID:         r.ID,
		AgentID:    r.AgentID,
		ScheduleID: nullInt(r.ScheduleID),
		Trigger:    r.Trigger,
		StartedAt:  r.StartedAt,
		FinishedAt: nullInt(r.FinishedAt),
		Status:     r.Status,
		Prompt:     r.Prompt,
		Result:     nullString(r.Result),
		ToolsUsed:  nullString(r.ToolsUsed),
		CostTokens: r.CostTokens,
		Error:      nullString(r.Error),
	}
}

func runsToWire(in []store.AgentRun) []agentRunWire {
	out := make([]agentRunWire, 0, len(in))
	for _, r := range in {
		out = append(out, runToWire(r))
	}
	return out
}
