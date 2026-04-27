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
	"github.com/mdp/qrterminal/v3"
	"github.com/skip2/go-qrcode"

	"github.com/snestors/agenthub/internal/buildinfo"
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
	case "migrate-bridge":
		runMigrateBridge(args)
	case "version", "-v", "--version":
		fmt.Printf("agenthub %s (%s)\n", buildinfo.Version, buildinfo.GitCommit)
	case "help", "-h", "--help":
		printHelp()
	default:
		// fall through to serve so plain ./agenthub works
		runServe()
	}
}


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
  agenthub migrate-bridge --from <path/messages.db>
                                 importa el histórico del bridge legacy a wa_messages (idempotente)

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
		srv.SetWAClient(waClient)
		// QR consumer: render every pairing code as ASCII to stdout AND as
		// a PNG at data/wa-qr.png. The PNG path is also published to the
		// server so /api/wa/qr can serve it and /healthz can advertise it.
		// Without a reader the channel drops codes silently and the cutover stalls.
		go func() {
			// data/wa-qr.png lives inside the daemon's WorkingDirectory so
			// it survives the systemd PrivateTmp sandbox.
			const qrPNGPath = "data/wa-qr.png"
			for code := range waClient.QR() {
				fmt.Println("─────────── WA PAIRING QR ───────────")
				qrterminal.GenerateHalfBlock(code, qrterminal.L, os.Stdout)
				fmt.Println("─────────────────────────────────────")
				fmt.Println("Scan with WhatsApp → Linked devices.")
				if err := qrcode.WriteFile(code, qrcode.Medium, 512, qrPNGPath); err != nil {
					logger.Warn("wa qr png write", "err", err)
				} else {
					logger.Info("wa qr png written", "path", qrPNGPath)
					srv.SetWAQRPath(qrPNGPath)
				}
			}
		}()
		if err := waClient.Connect(ctx); err != nil {
			logger.Error("wa connect", "err", err)
		}
		waClient.StartOutboxWorker(ctx)
		// WAConsumer feeds incoming WA messages into the SAME pipeline the
		// web chat uses (mirror to messages(channel='web') + broadcast WS +
		// runMainAgentTurn). Reply fans out to WA via wa_outbox. WA is just
		// another I/O surface on the unified main agent.
		srv.StartWAConsumer(ctx, waClient)
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
	username := fs.String("username", "", "username for the single admin account")
	password := fs.String("password", "", "password (plain)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *username == "" {
		fmt.Fprintln(os.Stderr, "--username requerido")
		os.Exit(2)
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

func runMigrateBridge(args []string) {
	fs := flag.NewFlagSet("migrate-bridge", flag.ExitOnError)
	from := fs.String("from", "", "path to legacy bridge messages.db (required)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *from == "" {
		fmt.Fprintln(os.Stderr, "--from requerido")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("db open", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	repos := store.NewRepos(db)

	logger.Info("legacy bridge migration starting", "from", *from, "to", cfg.DBPath)
	res, err := wa.MigrateLegacyMessages(ctx, repos.DB(), *from)
	if err != nil {
		logger.Error("migrate-bridge", "err", err)
		os.Exit(1)
	}
	fmt.Printf("legacy bridge migration done: total=%d imported=%d skipped=%d errors=%d\n",
		res.Total, res.Imported, res.Skipped, res.Errors)
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
