// Package config loads runtime configuration from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the complete runtime configuration for AgentHub.
type Config struct {
	// HTTP / WS
	HTTPAddr     string
	CookieDomain string
	CookieSecure bool

	// Auth
	SecretKey     []byte
	JWTSecret     []byte
	JWTTTL        time.Duration
	JWTRefreshTTL time.Duration
	DevBypassTOTP bool

	// Storage
	DBPath       string
	UploadDir    string
	WAMediaDir   string
	LogDir       string
	LogLevel     string
	FrontendDist string

	// CLI engines
	DefaultEngine     string // claude | codex
	DefaultModel      string // opus-4-7 | sonnet | etc.
	ClaudeProjectsDir string
	ClaudeBinPath     string
	CodexBinPath      string
	OllamaURL         string
	OllamaModel       string
	// Path to a markdown file that gets passed as --append-system-prompt to
	// the claude CLI on every Run. Empty = no extra prompt (only the CLI's
	// default + skills loaded from cwd/.claude/skills/).
	SystemPromptPath string

	// WhatsApp
	WAEnabled     bool     // false en dev hasta cutover
	WADBPath      string   // whatsmeow store
	WAAuthorized  []string // whitelist de números
	WANotifyPhone string   // tu jid

	// Unix socket (CLI subcomandos)
	UnixSocketPath string

	// Snapshots
	SnapshotInterval time.Duration

	// Local Claude usage estimates from JSONLs. Week limit is intentionally
	// configurable because Anthropic does not publish exact Max 5x caps.
	UsageSessionTokenLimit int64
	UsageWeekTokenLimit    int64

	// System manager
	ManagedServices []string

	// Modes
	Mode string // serve | send | mcp | setup-user | session
}

// Load reads environment variables, applies defaults, and validates required secrets.
// In dev mode (AGENTHUB_DEV=true), missing secrets are auto-generated.
func Load() (*Config, error) {
	dev := boolEnv("AGENTHUB_DEV", false)
	home, _ := os.UserHomeDir()

	// Secret key — 32 bytes hex
	secretHex := env("AGENTHUB_SECRET_KEY", "")
	if secretHex == "" {
		if !dev {
			return nil, fmt.Errorf("AGENTHUB_SECRET_KEY must be a hex-encoded 32-byte key")
		}
		// auto-generate for dev
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		secretHex = hex.EncodeToString(buf)
	}
	secret, err := hex.DecodeString(strings.TrimSpace(secretHex))
	if err != nil || len(secret) != 32 {
		return nil, fmt.Errorf("AGENTHUB_SECRET_KEY must be a hex-encoded 32-byte key (got %d bytes)", len(secret))
	}

	// JWT secret — at least 32 bytes
	jwtSecret := []byte(env("AGENTHUB_JWT_SECRET", ""))
	if len(jwtSecret) < 32 {
		if !dev {
			return nil, fmt.Errorf("AGENTHUB_JWT_SECRET must be at least 32 bytes")
		}
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		jwtSecret = []byte(hex.EncodeToString(buf))
	}

	cfg := &Config{
		HTTPAddr:     env("AGENTHUB_HTTP_ADDR", "0.0.0.0:8090"),
		CookieDomain: env("AGENTHUB_COOKIE_DOMAIN", ""),
		CookieSecure: boolEnv("AGENTHUB_COOKIE_SECURE", !dev),

		SecretKey:     secret,
		JWTSecret:     jwtSecret,
		JWTTTL:        durationEnv("AGENTHUB_JWT_TTL", 4*time.Hour),
		JWTRefreshTTL: durationEnv("AGENTHUB_JWT_REFRESH_TTL", 7*24*time.Hour),
		DevBypassTOTP: boolEnv("AGENTHUB_DEV_BYPASS_TOTP", dev),

		DBPath:       env("AGENTHUB_DB_PATH", "data/agenthub.db"),
		UploadDir:    env("AGENTHUB_UPLOAD_DIR", "data/uploads"),
		WAMediaDir:   env("AGENTHUB_WA_MEDIA_DIR", "data/wa_media"),
		LogDir:       env("AGENTHUB_LOG_DIR", "logs"),
		LogLevel:     env("AGENTHUB_LOG_LEVEL", "info"),
		FrontendDist: env("AGENTHUB_FRONTEND_DIST", "frontend/dist"),

		DefaultEngine:     env("AGENTHUB_DEFAULT_ENGINE", "claude"),
		DefaultModel:      env("AGENTHUB_DEFAULT_MODEL", "opus-4-7"),
		ClaudeProjectsDir: env("CLAUDE_PROJECTS_DIR", filepath.Join(home, ".claude/projects")),
		ClaudeBinPath:     env("CLAUDE_BIN", filepath.Join(home, ".local/bin/claude")),
		CodexBinPath:      env("CODEX_BIN", filepath.Join(home, ".npm-global/bin/codex")),
		OllamaURL:         env("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:       env("OLLAMA_MODEL", "gemma:2b"),
		SystemPromptPath:  env("AGENTHUB_SYSTEM_PROMPT", "data/system-prompt.md"),

		WAEnabled:     boolEnv("AGENTHUB_WA_ENABLED", false), // off por default en dev
		WADBPath:      env("AGENTHUB_WA_DB_PATH", "data/whatsmeow.db"),
		WAAuthorized:  csvEnv("AGENTHUB_WA_AUTHORIZED", []string{}),
		WANotifyPhone: env("AGENTHUB_WA_NOTIFY_PHONE", ""),

		UnixSocketPath: env("AGENTHUB_SOCK", "/tmp/agenthub.sock"),

		SnapshotInterval: durationEnv("AGENTHUB_SNAPSHOT_INTERVAL", 5*time.Minute),

		UsageSessionTokenLimit: int64Env("AGENTHUB_USAGE_SESSION_TOKEN_LIMIT", 88_000),
		UsageWeekTokenLimit:    int64Env("AGENTHUB_USAGE_WEEK_TOKEN_LIMIT", 3_000_000),

		ManagedServices: csvEnv("AGENTHUB_MANAGED_SERVICES", []string{
			"agenthub.service",
			"whatsapp-bridge.service",
			"whatsapp-bridge-watchdog.timer",
			"workspace-mcp.service",
			"cloudflared.service",
			"sonarr.service",
			"radarr.service",
			"qbittorrent-nox.service",
			"emby-server.service",
		}),

		Mode: env("AGENTHUB_MODE", "serve"),
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return b
}

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	out := []string{}
	for _, s := range strings.Split(value, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
