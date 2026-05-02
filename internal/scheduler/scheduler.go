// Package scheduler runs the mini-agent scheduler loop.
//
// Every tick (default 30s), pulls schedules whose next_run <= now from
// agent_schedules and dispatches the corresponding agent via cliengine.
// On completion, the run is persisted in agent_runs and the schedule's
// next_run is recomputed from its cron expression.
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/snestors/agenteshub/internal/cliengine"
	"github.com/snestors/agenteshub/internal/store"
)

// TickInterval is how often the scheduler polls for due jobs.
const TickInterval = 30 * time.Second

// RunFinishedFn is called after every cron run with its outcome so the
// server can broadcast a notification. Optional — if nil, no-op.
type RunFinishedFn func(ctx context.Context, agentName string, agentID, runID int64, status, result, errStr string)

// Scheduler is the runtime worker that drives agent_schedules.
type Scheduler struct {
	repos      *store.Repos
	engines    *cliengine.Manager
	log        *slog.Logger
	onFinished RunFinishedFn
}

// New constructs the scheduler.
func New(repos *store.Repos, engines *cliengine.Manager, log *slog.Logger) *Scheduler {
	return &Scheduler{repos: repos, engines: engines, log: log.With("comp", "scheduler")}
}

// SetRunFinishedHook installs a callback fired after each cron run finishes.
func (s *Scheduler) SetRunFinishedHook(fn RunFinishedFn) {
	s.onFinished = fn
}

// Start launches the scheduler loop in a goroutine.
// The loop exits when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Scheduler) run(ctx context.Context) {
	t := time.NewTicker(TickInterval)
	defer t.Stop()
	s.log.Info("scheduler started", "interval", TickInterval)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopping")
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

// tick processes any due schedules.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().Unix()
	pendings, err := s.repos.Agents.PendingSchedules(ctx, now)
	if err != nil {
		s.log.Warn("pending schedules", "err", err)
		return
	}
	if len(pendings) == 0 {
		return
	}
	s.log.Info("dispatching schedules", "count", len(pendings))
	for _, sched := range pendings {
		s.dispatch(ctx, sched)
	}
}

// dispatch fires a single schedule.
//
// Steps:
//  1. Look up the agent by ID (via name fallback).
//  2. Build the prompt: system_prompt + "\n\n" + prompt_template.
//  3. Insert agent_runs row with status='running'.
//  4. Run cliengine.
//  5. FinishRun + route the result via NotifyTarget.
//  6. SetNextRun based on cron expression.
//
// Errors at any step are logged and the run is marked 'error'. We always
// reschedule next_run so a transient failure doesn't stall the loop.
func (s *Scheduler) dispatch(ctx context.Context, sched store.AgentSchedule) {
	agent, err := s.lookupAgent(ctx, sched.AgentID)
	if err != nil {
		s.log.Warn("agent lookup", "schedule_id", sched.ID, "agent_id", sched.AgentID, "err", err)
		// still reschedule to avoid stalling
		s.rescheduleNext(ctx, sched, time.Now().Unix())
		return
	}
	if !agent.Enabled {
		s.log.Info("skipping disabled agent", "agent", agent.Name, "schedule_id", sched.ID)
		s.rescheduleNext(ctx, sched, time.Now().Unix())
		return
	}

	prompt := strings.TrimSpace(agent.SystemPrompt)
	if t := strings.TrimSpace(sched.PromptTemplate); t != "" {
		if prompt != "" {
			prompt += "\n\n"
		}
		prompt += t
	}

	startedAt := time.Now().Unix()
	runID, err := s.repos.Agents.InsertRun(ctx, store.AgentRun{
		AgentID:    agent.ID,
		ScheduleID: sql.NullInt64{Int64: sched.ID, Valid: true},
		Trigger:    "cron",
		StartedAt:  startedAt,
		Status:     "running",
		Prompt:     prompt,
	})
	if err != nil {
		s.log.Warn("insert run", "agent", agent.Name, "err", err)
		s.rescheduleNext(ctx, sched, startedAt)
		return
	}

	go s.runAgent(context.Background(), agent, sched, runID, prompt, startedAt)
}

// runAgent executes the cli engine and finalises the run.
// Runs in its own goroutine — does not block the tick loop.
func (s *Scheduler) runAgent(ctx context.Context, agent *store.Agent, sched store.AgentSchedule, runID int64, prompt string, startedAt int64) {
	engineName := strings.TrimSpace(agent.Engine)
	// No deadline: cron mini-agents are fire-and-forget; if one wedges, the
	// next tick will spawn another and the user can pause from the UI. The
	// 1h notification watcher only fires for interactive scopes (main/project/
	// agent-manual) where there's a user actively waiting.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	res, err := s.engines.Run(runCtx, cliengine.RunOpts{
		Prompt:    prompt,
		Channel:   "agent",
		Cwd:       ".",
		Engine:    engineName,
		Scope:     "agent",
		AgentName: agent.Name,
	})

	status := "ok"
	result := ""
	tokens := int64(0)
	errStr := ""
	if err != nil {
		status = "error"
		errStr = err.Error()
		s.log.Warn("agent run", "agent", agent.Name, "err", err)
	} else if res != nil {
		result = res.Text
		tokens = res.CostTokens
	}
	if ferr := s.repos.Agents.FinishRun(ctx, runID, status, result, tokens, errStr); ferr != nil {
		s.log.Warn("finish run", "agent", agent.Name, "err", ferr)
	}

	if s.onFinished != nil {
		s.onFinished(ctx, agent.Name, agent.ID, runID, status, result, errStr)
	}

	// Route notification.
	s.route(ctx, agent, sched, status, result, errStr)

	// Reschedule next.
	s.rescheduleNext(ctx, sched, startedAt)
}

// route delivers the result to the configured target.
func (s *Scheduler) route(ctx context.Context, agent *store.Agent, sched store.AgentSchedule, status, result, errStr string) {
	target := strings.TrimSpace(sched.NotifyTarget)
	if target == "" || target == "none" {
		return
	}
	body := result
	if status != "ok" {
		body = fmt.Sprintf("[%s error] %s", agent.Name, errStr)
	}
	switch {
	case strings.HasPrefix(target, "wa:"):
		// TODO: wire wa client when WAEnabled. Persist as web for now so it surfaces.
		s.log.Info("scheduled run notify (wa pending)", "agent", agent.Name, "target", target)
	case target == "main-agent":
		_, err := s.repos.Messages.Insert(ctx, store.Message{
			Channel:   "web",
			Direction: "out",
			Body:      sql.NullString{String: body, Valid: body != ""},
			TS:        time.Now().Unix(),
		})
		if err != nil {
			s.log.Warn("notify main-agent", "err", err)
		}
	case strings.HasPrefix(target, "topic:"):
		topicName := strings.TrimPrefix(target, "topic:")
		t, err := s.repos.Topics.GetByName(ctx, topicName)
		if err != nil {
			s.log.Warn("notify topic — topic not found", "topic", topicName, "err", err)
			return
		}
		st := store.TopicState{
			TopicID:        t.ID,
			NextActionHint: sql.NullString{String: body, Valid: body != ""},
		}
		if err := s.repos.Topics.UpsertState(ctx, st); err != nil {
			s.log.Warn("notify topic — upsert", "topic", topicName, "err", err)
		}
		_ = s.repos.Topics.Touch(ctx, t.ID)
	default:
		s.log.Warn("unknown notify target", "target", target)
	}
}

// rescheduleNext updates next_run from the cron expression.
// If parsing fails, falls back to next_run = now + 1h to avoid hammering the loop.
func (s *Scheduler) rescheduleNext(ctx context.Context, sched store.AgentSchedule, lastRun int64) {
	next := s.computeNext(sched.CronExpr, time.Now())
	if err := s.repos.Agents.SetNextRun(ctx, sched.ID, lastRun, next); err != nil {
		s.log.Warn("set next run", "schedule_id", sched.ID, "err", err)
	}
}

// computeNext returns the unix epoch of the next firing of the cron expression.
func (s *Scheduler) computeNext(expr string, base time.Time) int64 {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		s.log.Warn("invalid cron expr — falling back to +1h", "expr", expr, "err", err)
		return base.Add(time.Hour).Unix()
	}
	return sched.Next(base).Unix()
}

// lookupAgent returns an agent by ID. AgentsRepo doesn't expose GetByID, so we
// scan through List(). Fine for v0 (handful of agents).
func (s *Scheduler) lookupAgent(ctx context.Context, id int64) (*store.Agent, error) {
	all, err := s.repos.Agents.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("agent id=%d not found", id)
}
