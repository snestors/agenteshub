package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/snestors/agenteshub/internal/buildinfo"
	"github.com/snestors/agenteshub/internal/store"
)

// slashScope identifies which session a slash command should affect when it
// has side effects (reset, engine change). Project sessions can't switch
// engines mid-stream — their resume id is engine-specific — so /engine X
// returns an error in that scope.
type slashScope struct {
	Kind          string // "main" | "project"
	Engine        string // current engine (used when Kind == "main")
	ProjectSessID int64  // when Kind == "project"
}

// slashResult is what conversation.go / projects.go look at to decide whether
// the message bypasses the engine. Empty when the body wasn't a slash command.
type slashResult struct {
	Handled  bool
	Response string // assistant body to surface to the user
	Error    string // surfaced as error severity if non-empty (still Handled)
}

// tryHandleSlashCommand checks if `body` is a recognised slash command and, if
// so, executes its side effects and returns the synthetic assistant response.
// Returns Handled=false (zero value) when the body should flow to the engine
// normally. Calls into store repos via s.repos so it must run with a live
// context — the caller's request ctx is fine.
func (s *Server) tryHandleSlashCommand(ctx context.Context, body string, scope slashScope) slashResult {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "/") {
		return slashResult{}
	}
	parts := strings.Fields(body)
	if len(parts) == 0 {
		return slashResult{}
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]
	switch cmd {
	case "/reset", "/clear", "/forget", "/new":
		return s.slashReset(ctx, scope)
	case "/status":
		return s.slashStatus(ctx, scope)
	case "/engine":
		return s.slashEngine(ctx, scope, args)
	case "/help", "/?":
		return s.slashHelp()
	}
	return slashResult{}
}

func (s *Server) slashReset(ctx context.Context, scope slashScope) slashResult {
	switch scope.Kind {
	case "main":
		// Wipe the resume id for both engine-scoped names and the legacy
		// shared row. Next message starts fresh.
		for _, eng := range []string{scope.Engine, "claude", "codex"} {
			if eng == "" {
				continue
			}
			_ = s.repos.Sessions.UpsertAgentSession(ctx, store.AgentSession{
				AgentName: mainAgentSessionName(eng),
				Engine:    eng,
				SessionID: "",
			})
		}
		_ = s.repos.Sessions.UpsertAgentSession(ctx, store.AgentSession{
			AgentName: "main-agent",
			Engine:    scope.Engine,
			SessionID: "",
		})
		return slashResult{
			Handled:  true,
			Response: "Sesion reseteada. El proximo mensaje arranca conversacion nueva.",
		}
	case "project":
		if scope.ProjectSessID == 0 {
			return slashResult{Handled: true, Error: "/reset: project session no resuelta"}
		}
		if err := s.repos.Projects.UpdateSessionID(ctx, scope.ProjectSessID, ""); err != nil {
			return slashResult{Handled: true, Error: "/reset: " + err.Error()}
		}
		return slashResult{
			Handled:  true,
			Response: "Sesion del proyecto reseteada. El proximo mensaje arranca conversacion nueva.",
		}
	}
	return slashResult{Handled: true, Error: "/reset: scope desconocido"}
}

func (s *Server) slashStatus(ctx context.Context, scope slashScope) slashResult {
	stats, err := s.sysman.Stats(ctx)
	if err != nil {
		return slashResult{Handled: true, Error: "/status: " + err.Error()}
	}
	engine, model := s.currentEngineModel(ctx)

	uptime := stats.UptimeSec
	uph := uptime / 3600
	upm := (uptime % 3600) / 60

	disks := ""
	for _, d := range stats.Disks {
		disks += fmt.Sprintf("\n  - %s: %.1fGB / %.1fGB (%.0f%%)", d.Mount, d.UsedGB, d.TotalGB, d.UsedPct)
	}

	runMain := s.runs.Count("main")
	runProject := s.runs.Count("project")

	waLabel := "off"
	if s.cfg.WAEnabled {
		if s.waConnected() {
			waLabel = "connected"
		} else {
			waLabel = "disconnected"
		}
	}

	body := fmt.Sprintf(
		"AgentHub %s (%s)\n"+
			"Uptime: %dh %dm | Engine: %s · %s\n"+
			"CPU: %.0f%% (load %.2f, %d cores) | RAM: %.1f/%.1fGB | Temp: %.0fC%s\n"+
			"WA: %s | Runs: main=%d project=%d | Scope: %s",
		buildinfo.Version, buildinfo.GitCommit,
		uph, upm, engine, model,
		stats.CPUPct, stats.LoadAvg[0], stats.Cores, stats.RAMUsedGB, stats.RAMTotalGB, stats.TempC, disks,
		waLabel, runMain, runProject, scope.Kind,
	)
	return slashResult{Handled: true, Response: body}
}

func (s *Server) slashEngine(ctx context.Context, scope slashScope, args []string) slashResult {
	if scope.Kind == "project" {
		return slashResult{
			Handled: true,
			Error:   "/engine no se puede cambiar dentro de un project session — el resume id es engine-specifico. Crea una sesion nueva con el engine que quieras.",
		}
	}
	engine, model := s.currentEngineModel(ctx)
	if len(args) == 0 {
		return slashResult{
			Handled: true,
			Response: fmt.Sprintf(
				"Engine activo: %s · %s\n"+
					"Cambia con: /engine claude  o  /engine codex\n"+
					"Para preguntar one-shot sin cambiar: /claude <tarea>  o  /codex <tarea>",
				engine, model,
			),
		}
	}
	target := strings.ToLower(strings.TrimSpace(args[0]))
	if target != "claude" && target != "codex" {
		return slashResult{
			Handled: true,
			Error:   "/engine: valor desconocido. Usa: /engine claude  |  /engine codex",
		}
	}
	newModel := defaultModelForEngine(target, s.cfg.OllamaModel)
	if _, err := s.setEngine(ctx, engineSetReq{Engine: target, Model: newModel}); err != nil {
		return slashResult{Handled: true, Error: "/engine: " + err.Error()}
	}
	go s.broadcastAgentStatus(context.Background())
	return slashResult{
		Handled:  true,
		Response: fmt.Sprintf("Engine cambiado a %s · %s", target, newModel),
	}
}

func (s *Server) slashHelp() slashResult {
	body := strings.Join([]string{
		"Comandos disponibles:",
		"  /reset · /clear · /forget · /new   — limpia la sesion activa, proximo mensaje arranca de cero",
		"  /status                            — uptime, engine, RAM/CPU/temp, runs activos, WA",
		"  /engine                            — engine activo + ayuda",
		"  /engine claude | /engine codex     — cambia engine activo (solo en main chat)",
		"  /help · /?                         — esta ayuda",
	}, "\n")
	return slashResult{Handled: true, Response: body}
}
