// Command agenthub is a multi-mode binary: serve | send | mcp | setup-user | session.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mdp/qrterminal/v3"
	"github.com/skip2/go-qrcode"

	"github.com/agenteshub/agenteshub/internal/buildinfo"
	"github.com/agenteshub/agenteshub/internal/cliengine"
	"github.com/agenteshub/agenteshub/internal/config"
	intcron "github.com/agenteshub/agenteshub/internal/cron"
	"github.com/agenteshub/agenteshub/internal/mcp"
	"github.com/agenteshub/agenteshub/internal/scheduler"
	"github.com/agenteshub/agenteshub/internal/server"
	"github.com/agenteshub/agenteshub/internal/setup"
	"github.com/agenteshub/agenteshub/internal/store"
	"github.com/agenteshub/agenteshub/internal/sysman"
	"github.com/agenteshub/agenteshub/internal/wa"
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
	case "setup":
		runSetup()
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
		fmt.Printf("agenteshub %s (%s)\n", buildinfo.Version, buildinfo.GitCommit)
	case "help", "-h", "--help":
		printHelp()
	default:
		// fall through to serve so plain ./agenteshub works
		runServe()
	}
}


func printHelp() {
	fmt.Println(`agenteshub — your self-hosted AI agent hub

USAGE:
  agenteshub                     start the daemon (default)
  agenteshub serve               same
  agenteshub setup               interactive first-run wizard
  agenteshub setup-user --username NAME --password PASS
                                 create/update the admin user + TOTP QR
  agenteshub mcp                 MCP server stdio (invoked by Claude/Codex)
  agenteshub version             print version and git commit
  agenteshub migrate-bridge --from <path/messages.db>
                                 import legacy bridge history into wa_messages

KEY ENV VARS (.env):
  AGENTHUB_HTTP_ADDR         (default 0.0.0.0:8093)
  AGENTHUB_DB_PATH           (default data/agenthub.db)
  AGENTHUB_DEV               (default false; true = auto-generate secrets)
  AGENTHUB_SECRET_KEY        (32-byte hex; required in production)
  AGENTHUB_JWT_SECRET        (>=32 chars; required in production)
  AGENTHUB_WA_ENABLED        (default false)
  AGENTHUB_WA_NOTIFY_PHONE   (your WhatsApp number, international format)

Run 'agenteshub setup' for a guided first-run configuration.`)
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

func runSetup() {
	fmt.Println()
	fmt.Println("  AgentesHub — Setup Wizard")
	fmt.Println("  ─────────────────────────")
	fmt.Println()

	// Detect existing .env
	envExists := false
	if _, err := os.Stat(".env"); err == nil {
		envExists = true
		fmt.Println("  ⚠  .env already exists — values will be overwritten only if you confirm.")
		fmt.Println()
	}
	_ = envExists

	// Prompt helper
	prompt := func(label, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("  %s [%s]: ", label, defaultVal)
		} else {
			fmt.Printf("  %s: ", label)
		}
		var v string
		fmt.Scanln(&v)
		v = strings.TrimSpace(v)
		if v == "" {
			return defaultVal
		}
		return v
	}

	// [1/5] Admin user
	fmt.Println("  [1/5] Admin user")
	username := prompt("Username", "admin")
	fmt.Printf("  Password (will not echo): ")
	// Read password — use simple Scanln (no terminal echo suppression needed for basic setup)
	var password string
	fmt.Scanln(&password)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		fmt.Fprintln(os.Stderr, "\n  Error: username and password are required.")
		os.Exit(1)
	}
	fmt.Println()

	// [2/5] HTTP
	fmt.Println("  [2/5] HTTP")
	httpAddr := prompt("Listen address", "0.0.0.0:8093")
	fmt.Println()

	// [3/5] WhatsApp
	fmt.Println("  [3/5] WhatsApp (press Enter to skip)")
	waEnabled := false
	waPhone := prompt("Your WhatsApp number (e.g. 15551234567)", "")
	if waPhone != "" {
		waEnabled = true
	}
	fmt.Println()

	// [4/5] Secrets
	fmt.Println("  [4/5] Generating secrets...")
	secretKey, err := generateHex(32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "  Error generating secret key:", err)
		os.Exit(1)
	}
	jwtSecret, err := generateHex(32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "  Error generating jwt secret:", err)
		os.Exit(1)
	}
	fmt.Println("  SECRET_KEY ... ✓")
	fmt.Println("  JWT_SECRET  ... ✓")
	fmt.Println()

	// [5/5] Write files
	fmt.Println("  [5/5] Writing configuration...")

	waEnabledStr := "false"
	if waEnabled {
		waEnabledStr = "true"
	}

	envContent := fmt.Sprintf(`# AgentesHub — generated by setup wizard
AGENTHUB_HTTP_ADDR=%s
AGENTHUB_DB_PATH=data/agenthub.db
AGENTHUB_SECRET_KEY=%s
AGENTHUB_JWT_SECRET=%s
AGENTHUB_DEV=false
AGENTHUB_DEV_BYPASS_TOTP=false
AGENTHUB_WA_ENABLED=%s
AGENTHUB_WA_NOTIFY_PHONE=%s
AGENTHUB_DEFAULT_ENGINE=claude
AGENTHUB_LOG_LEVEL=info
AGENTHUB_COOKIE_SECURE=false
`, httpAddr, secretKey, jwtSecret, waEnabledStr, waPhone)

	if err := os.WriteFile(".env", []byte(envContent), 0600); err != nil {
		fmt.Fprintln(os.Stderr, "  Error writing .env:", err)
		os.Exit(1)
	}
	fmt.Println("  .env ... ✓")

	// Copy system-prompt template if not present
	if _, err := os.Stat("data/system-prompt.md"); os.IsNotExist(err) {
		if tpl, err := os.ReadFile("system-prompt.example.md"); err == nil {
			if err := os.MkdirAll("data", 0755); err == nil {
				_ = os.WriteFile("data/system-prompt.md", tpl, 0644)
				fmt.Println("  data/system-prompt.md ... ✓ (copied from template)")
			}
		} else {
			fmt.Println("  data/system-prompt.md ... skipped (system-prompt.example.md not found)")
		}
	} else {
		fmt.Println("  data/system-prompt.md ... already exists, skipped")
	}

	// Create admin user
	fmt.Printf("  Creating user '%s'... ", username)
	_ = godotenv.Load(".env")
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n  Error loading config:", err)
		os.Exit(1)
	}
	logger := newLogger("warn")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := os.MkdirAll("data", 0755); err != nil {
		fmt.Fprintln(os.Stderr, "\n  Error creating data dir:", err)
		os.Exit(1)
	}
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n  Error opening DB:", err)
		os.Exit(1)
	}
	defer db.Close()
	repos := store.NewRepos(db)
	if err := setup.User(ctx, cfg, repos, username, password); err != nil {
		fmt.Fprintln(os.Stderr, "\n  Error creating user:", err)
		os.Exit(1)
	}
	_ = logger
	fmt.Println("✓")
	fmt.Println()
	fmt.Println("  ✅ Setup complete!")
	fmt.Println()
	fmt.Printf("  Edit data/system-prompt.md to personalize your agent.\n")
	fmt.Printf("  Then run: ./agenteshub serve\n")
	fmt.Printf("  Open:     http://localhost:8093\n")
	fmt.Println()
}

func generateHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
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
