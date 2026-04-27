# AgentesHub

A self-hosted AI agent hub that runs on your hardware. One brain, two surfaces: a web UI and WhatsApp. Chat with your agent, run AI-powered project sessions, manage mini-agents, and keep everything local.

## Features

- **Unified brain** — Web and WhatsApp share the same conversational context and AI engine
- **Project sessions** — Isolated AI coding sessions per project, with engine selection (Claude / Codex / Ollama)
- **Mini-agents** — Scheduled background agents for recurring tasks (log watching, finance categorization, etc.)
- **Local-first** — SQLite persistence, runs entirely on your machine
- **Multi-engine** — Switch between Claude, Codex, and local Ollama models per session
- **Vault** — AES-GCM encrypted credential store
- **Topics** — Persistent context threads for recurring subjects
- **OpenSpec gates** — SDD-style change workflow with proposal → spec → design → tasks → apply
- **Diagrams** — Mermaid rendering and Excalidraw whiteboard
- **Release versioning** — `/releases` page with live version comparison between backend and UI

## Requirements

- Go 1.22+
- Node.js 20+ and [pnpm](https://pnpm.io/)
- [Claude Code CLI](https://claude.ai/code) (`claude` in PATH) for the Claude engine
- [Codex CLI](https://github.com/openai/codex) (`codex` in PATH) for the Codex engine — optional
- [Ollama](https://ollama.ai/) for local models — optional
- SQLite (bundled via CGO)

## Installation

```bash
git clone https://github.com/agenteshub/agenteshub
cd agenteshub

# 1. Build the frontend
cd frontend && pnpm install && pnpm run build && cd ..

# 2. Build the backend
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X github.com/agenteshub/agenteshub/internal/buildinfo.Version=$(cat VERSION) \
            -X github.com/agenteshub/agenteshub/internal/buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenteshub ./cmd/agenthub

# 3. Run the setup wizard
./bin/agenteshub setup
# → generates .env with secure secrets
# → creates data/system-prompt.md from the template
# → creates your admin user

# 4. Personalize your agent
# Edit data/system-prompt.md — add your name, hardware, services, context

# 5. Start the daemon
./bin/agenteshub serve
```

Open `http://localhost:8093` in your browser.

## Configuration

All configuration is via environment variables (or `.env` file). See `.env.example` for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENTHUB_HTTP_ADDR` | `0.0.0.0:8093` | Listen address |
| `AGENTHUB_DB_PATH` | `data/agenthub.db` | SQLite database path |
| `AGENTHUB_SECRET_KEY` | — | 32-byte hex key for encryption (required in prod) |
| `AGENTHUB_JWT_SECRET` | — | JWT signing secret ≥32 chars (required in prod) |
| `AGENTHUB_DEV` | `false` | Dev mode — auto-generates secrets, relaxed auth |
| `AGENTHUB_WA_ENABLED` | `false` | Enable WhatsApp bridge |
| `AGENTHUB_WA_NOTIFY_PHONE` | — | Your WhatsApp number (international format) |
| `AGENTHUB_DEFAULT_ENGINE` | `claude` | Default AI engine (`claude`/`codex`/`ollama`) |
| `AGENTHUB_LOG_LEVEL` | `info` | Log level (`debug`/`info`/`warn`/`error`) |

## System Prompt

AgentHub loads `data/system-prompt.md` at startup and injects it into every AI turn. This is where you define your agent's identity, context, and behavior.

Copy `system-prompt.example.md` to `data/system-prompt.md` and customize it:
- Your name, hardware, local IP
- Services running on your machine
- Personal context the agent should know
- Workflow preferences

`data/system-prompt.md` is gitignored — your personal data stays local.

## WhatsApp Setup

1. Set `AGENTHUB_WA_ENABLED=true` and `AGENTHUB_WA_NOTIFY_PHONE=<your-number>` in `.env`
2. Start the daemon and open `http://localhost:8093/api/wa/qr` to scan the pairing QR
3. Once paired, incoming WhatsApp messages from your number are routed to the agent

## Running as a systemd service

```ini
[Unit]
Description=AgentHub daemon
After=network.target

[Service]
Type=simple
User=youruser
WorkingDirectory=/home/youruser/agenthub
ExecStart=/home/youruser/agenteshub/bin/agenteshub serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now agenthub
```

## Deploy (blue/green smoke)

AgentHub includes a safe-restart script that waits for in-flight AI turns to complete before restarting:

```bash
# Build new binary
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X .../buildinfo.Version=$(cat VERSION) -X .../buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenthub.next ./cmd/agenthub

# Smoke test on port 8094
SMOKE_DB=/tmp/smoke-$$.db \
AGENTHUB_HTTP_ADDR=127.0.0.1:8094 \
AGENTHUB_DB_PATH=$SMOKE_DB \
AGENTHUB_WA_ENABLED=false \
AGENTHUB_DEV=true \
./bin/agenthub.next serve &
# ... wait for /healthz → {"ok":true} ...

# Promote and restart gracefully
mv bin/agenthub.next bin/agenthub
bin/safe-restart.sh
```

See `CLAUDE.md` for the full recipe.

## Architecture

```
                    ┌──────────┐    ┌──────────────┐
  Web Browser ──────│  Vite    │    │  Claude Code │
                    │  React   │    │  Codex CLI   │
                    └────┬─────┘    │  Ollama      │
                         │ HTTP/WS  └──────┬───────┘
                    ┌────▼──────────────────▼───────┐
                    │         agenthub daemon        │
                    │   Go · HTTP · WebSocket · MCP  │
                    ├────────────────────────────────┤
                    │  SQLite (WAL)  │  Vault (AES)  │
                    └────────────────┬───────────────┘
                                     │ whatsmeow
                              WhatsApp Web
```

- **Backend**: Go HTTP daemon (`internal/server/`) with WebSocket hub for real-time streaming
- **Frontend**: React + Vite + Tailwind, cyberpunk HUD aesthetic
- **Persistence**: SQLite with FTS5 for message search
- **AI engines**: Claude Code CLI, Codex CLI, Ollama — all spawned as child processes, JSONL output parsed in real time
- **WhatsApp**: `whatsmeow` library, outbox worker for reliable delivery

## Versioning

Each release bumps `VERSION`, adds an entry to `RELEASE_NOTES.md`, and gets a git tag. The running version is visible in:
- `GET /healthz` → `{"version": "0.2.11", "git_commit": "abc1234"}`
- Sidebar stats badge — goes orange if UI and backend are out of sync

## License

MIT
