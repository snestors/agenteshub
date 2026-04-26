// Command agenthub is a multi-mode binary: serve | send | mcp | setup-user | session.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/config"
	intcron "github.com/snestors/agenthub/internal/cron"
	"github.com/snestors/agenthub/internal/mcp"
	"github.com/snestors/agenthub/internal/scheduler"
	"github.com/snestors/agenthub/internal/server"
	"github.com/snestors/agenthub/internal/setup"
	"github.com/snestors/agenthub/internal/store"
	"github.com/snestors/agenthub/internal/sysman"
	"github.com/snestors/agenthub/internal/wa"
)

func main() {
	_ = godotenv.Load(".env")

	if len(os.Args) < 2 {
		runServe()
		return
	}
	mode := os.Args[1]
	args := os.Args[2:]

	switch mode {
	case "serve":
		runServe()
	case "setup-user":
		runSetupUser(args)
	case "send", "send-image", "send-voice", "send-document", "status":
		runCLI(mode, args)
	case "mcp":
		runMCP()
	case "session":
		runSession(args)
	case "version", "-v", "--version":
		fmt.Println("agenthub", version)
	case "help", "-h", "--help":
		printHelp()
	default:
		// fall through to serve so plain ./agenthub works
		runServe()
	}
}

const version = "0.1.0"

func printHelp() {
	fmt.Println(`agenthub — tu asistente personal en la mini PC

USO:
  agenthub                       arranca el daemon (default)
  agenthub serve                 idem
  agenthub setup-user --username NAME --password PASS
                                 crea/actualiza el usuario único + TOTP QR
  agenthub send <jid> <text>     manda un mensaje vía Unix socket
  agenthub send-image <jid> <path> [caption]
  agenthub send-voice <jid> <path>
  agenthub send-document <jid> <path>
  agenthub status                muestra el estado del daemon
  agenthub mcp                   MCP server stdio (lo invoca Claude/Codex)
  agenthub session backup [<id>] fuerza snapshot
  agenthub session restore <id>  restaura desde snapshot
  agenthub session list          lista sessions

VARIABLES IMPORTANTES (.env):
  AGENTHUB_HTTP_ADDR         (default 0.0.0.0:8090)
  AGENTHUB_DB_PATH           (default data/agenthub.db)
  AGENTHUB_DEV               (default false; en true autogen secrets)
  AGENTHUB_DEV_BYPASS_TOTP   (default false; permite login sin TOTP)
  AGENTHUB_SECRET_KEY        (32-byte hex; obligatorio salvo dev)
  AGENTHUB_JWT_SECRET        (>=32 bytes; obligatorio salvo dev)`)
}

func runServe() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	cfg.Mode = "serve"

	logger := newLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("db open", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	repos := store.NewRepos(db)
	engines := cliengine.New(cfg, repos, logger)
	sm := sysman.New(cfg, logger)
	cronRunner := intcron.New(cfg, repos, logger)
	cronRunner.Start()
	defer cronRunner.Stop()

	sched := scheduler.New(repos, engines, logger)

	srv, err := server.New(cfg, repos, engines, sm, logger)
	if err != nil {
		logger.Error("server new", "err", err)
		os.Exit(1)
	}
	sched.SetRunFinishedHook(srv.NotifyAgentRunFinished)
	sched.Start(ctx)

	// WhatsApp client + outbox worker. Both no-op when WAEnabled=false so
	// dev mode stays untouched.
	waClient, err := wa.New(cfg, repos, logger)
	if err != nil {
		logger.Error("wa new", "err", err)
	} else if cfg.WAEnabled {
		if err := waClient.Connect(ctx); err != nil {
			logger.Error("wa connect", "err", err)
		}
		waClient.StartOutboxWorker(ctx)
		defer waClient.Disconnect()
	}

	logger.Info("agenthub starting", "addr", cfg.HTTPAddr, "dev_bypass_totp", cfg.DevBypassTOTP, "wa_enabled", cfg.WAEnabled)

	go func() {
		if err := srv.Run(ctx); err != nil {
			logger.Error("server run", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("agenthub stopping")
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("agenthub stopped")
}

func runSetupUser(args []string) {
	fs := flag.NewFlagSet("setup-user", flag.ExitOnError)
	username := fs.String("username", "", "username (default 'nestor')")
	password := fs.String("password", "", "password (plain)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *username == "" {
		*username = "nestor"
	}
	if *password == "" {
		fmt.Fprintln(os.Stderr, "--password requerido")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("db open", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	repos := store.NewRepos(db)

	if err := setup.User(ctx, cfg, repos, *username, *password); err != nil {
		logger.Error("setup-user", "err", err)
		os.Exit(1)
	}
}

func runCLI(mode string, args []string) {
	// Stub: full Unix-socket CLI is implemented in internal/sock; for now print a hint.
	fmt.Fprintf(os.Stderr, "agenthub %s — Unix-socket CLI no implementado todavía. Usá la API HTTP local.\n", mode)
	os.Exit(2)
}

func runMCP() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db open:", err)
		os.Exit(1)
	}
	defer db.Close()
	repos := store.NewRepos(db)
	engines := cliengine.New(cfg, repos, logger)
	sm := sysman.New(cfg, logger)
	srv := mcp.New(cfg, repos, sm, engines)
	if err := srv.ServeStdio(); err != nil {
		fmt.Fprintln(os.Stderr, "mcp serve:", err)
		os.Exit(1)
	}
}

func runSession(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "agenthub session backup|restore|list")
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "agenthub session — no implementado todavía.")
	os.Exit(2)
}

func newLogger(level string) *slog.Logger {
	l := slog.LevelInfo
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
