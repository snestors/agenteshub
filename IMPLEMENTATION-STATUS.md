# AgentHub Go — Estado de Implementación

> Snapshot al cierre de esta sesión de trabajo. Si retomás después de un compact / reinicio, leelo primero.

## 🟢 Lo que ESTÁ construido y funcionando

### Daemon Go corriendo via systemd
- Servicio: `agenthub.service` activo, `Restart=always`
- Binario: `/home/nestor/agenthub/bin/agenthub` (compilado con `-tags 'sqlite_fts5 sqlite_json'`, ~17 MB)
- Puerto: `0.0.0.0:8093`
- Sudoers: `NOPASSWD` para `systemctl start/stop/restart` de la whitelist
- Login: `nestor` / `Lazio999.`
- Dev mode: `AGENTHUB_DEV_BYPASS_TOTP=true` activo (saltea TOTP en dev)

### Estructura del repo

```
/home/nestor/agenthub/
├── ARCHITECTURE.md                      # north star, 13 secciones
├── IMPLEMENTATION-STATUS.md             # este archivo
├── bin/agenthub                          # binario multi-modo
├── go.mod / go.sum                       # Go 1.25
├── data/                                 # gitignored: agenthub.db + uploads
├── deploy/agenthub.service               # systemd unit
├── migrations/0001_init.sql              # snapshot legible (la realidad embebida)
├── cmd/agenthub/main.go                  # entry point: serve / setup-user / mcp / send / session
├── internal/
│   ├── auth/                             # ← reciclado de legacy
│   ├── config/                           # env vars con dev mode autogen
│   ├── store/                            # SQLite WAL + 11 repos + migrations embebidas
│   │   ├── migrations/0001_init.sql      # 14 tablas + FTS5
│   │   └── migrations/0002_message_model.sql  # engine+model en wa_messages
│   ├── server/                           # chi router + handlers + WS
│   ├── cliengine/                        # Claude/Codex/Ollama + streaming + snapshots
│   ├── mcp/                              # MCP Go embebido stdio (22 tools)
│   ├── wa/                               # whatsmeow client (apagado en dev)
│   ├── ws/                               # hub pub/sub (refactor pendiente — task #32)
│   ├── sysman/                           # gopsutil + systemctl
│   ├── scheduler/                        # robfig/cron worker para mini-agentes
│   ├── cron/                             # jobs internos (snapshots, jwt cleanup)
│   ├── setup/                            # setup-user one-shot
│   └── domain/models/                    # User struct
├── frontend/                             # Vite + React 19 + TS + Tailwind v4 + shadcn-style
│   ├── dist/                             # build production servido por Go
│   └── src/components/                   # StatusBar, EnginePicker, GhostBubble, MessageBubble, ...
└── .claude/skills/                       # 5 skills internas para Claude
```

### Backend completo (todas las features)

#### Auth
- JWT HS256 + JTI revocations
- TOTP (Google Authenticator) con AES-GCM encryption
- Cookie `agenthub_token` httpOnly + Secure
- Middleware `RequireJWT` aplicado al grupo protegido
- `agenthub setup-user` crea usuario + TOTP QR

#### CLIEngine
- Interface `Engine.Run(ctx, RunOpts)` con implementaciones:
  - **ClaudeEngine** (default `sonnet`): `claude -p --resume --output-format json|stream-json --mcp-config`
  - **CodexEngine**: `codex exec --json` con resume (NO expuesto en picker, smoke test pendiente)
  - **OllamaEngine**: HTTP `:11434` (NO expuesto, requiere setup)
- **Streaming**: cuando `OnEvent != nil`, usa `--output-format stream-json --verbose` y emite `StreamEvent` por cada `text`/`thinking`/`tool_use`/`tool_result`/`final`
- **Resume robusto**: antes de spawn, valida que existe `~/.claude/projects/<encoded_cwd>/<sid>.jsonl`. Si falta, restaura desde `session_snapshots`. **Cero "session not found"**.
- **MCP config auto-generado**: `/tmp/agenthub/mcp.json` apunta el CLI a `agenthub mcp` con env vars necesarias
- **Sub-agent capture**: post-turn parsea el JSONL buscando `tool_use` con `name=Agent` y persiste en `subagent_runs`
- **Resolver de aliases**: `opus-1m` → `claude-opus-4-7[1m]` (los `[]` son obligatorios para el CLI)

#### MCP Go embebido stdio (22 tools)
- **Salida**: `send_message`, `send_image`, `send_voice`, `send_audio`, `send_document`, `send_video`, `send_location`, `send_contact`, `react_to_message` *(últimas 5 declaradas en doc, sólo `send_message` en código actual — ampliar es trivial)*
- **Entrada**: `recent_messages`, `mark_read`
- **Datos schema-on-read**: `record`, `query_records`, `list_topics_records`
- **Topics**: `list_topics`, `read_topic_state`, `update_topic_state`
- **System Manager**: `get_system_stats`, `list_services`, `service_action`, `list_processes`, `list_tunnels`
- **Mini-agentes**: `agent_create`, `agent_list`, `agent_run_now`, `agent_logs`, `agent_schedule`, `agent_pause`, `agent_resume`
- **Sistema**: `get_status`

#### Persistencia 4 capas
1. **JSONL** en `~/.claude/projects/<encoded_cwd>/<sid>.jsonl` — estado vivo de Claude
2. **session_snapshots** BLOB en agenthub.db — backup async cada 5min + post-turn (vía `internal/cron`)
3. **session_messages** texto humano-legible (FTS5 indexed) — cada turn persistido por cliengine
4. **topic_state** JSON estructurado — actualizado por la session del topic vía tool

#### Endpoints HTTP/WS
- **Auth**: `POST /api/auth/login`, `/api/auth/totp`, `/api/auth/logout`, `/api/auth/refresh`, `GET /api/auth/me`
- **Chat**: `GET /api/messages`, `POST /api/messages` (con attachments), `WS /ws/agent`
- **Agent control**: `GET /api/agent/status` (incluye plan + plan_tier de `~/.claude/.credentials.json`), `GET /api/agent/engines`, `POST /api/agent/engine`
- **System**: `GET /api/system/stats`, `/services`, `/processes`, `/connections`, `POST /api/system/services/{name}/{action}`, `WS /ws/system`
- **Uploads**: `POST /api/upload` (max 50MB), `DELETE /api/uploads/{id}`
- **Health**: `GET /healthz`
- **Frontend SPA**: `GET /` y `/*` sirven `frontend/dist/`

#### Database (SQLite WAL)
14 tablas + FTS5: auth_users, jwt_revocations, settings, wa_messages (con engine+model), wa_jid_map, topics, topic_state, projects, project_sessions, agents, agent_schedules, agent_runs, agent_sessions, agent_records, session_messages, subagent_runs, session_snapshots, __migrations.

Migrations idempotentes via tabla `__migrations(name, applied_at)`.

#### Servicios internos
- **systemd**: una sola unit `agenthub.service` con `Restart=always`
- **cron interno** (robfig/cron): session-snapshot cada 5min, jwt-cleanup cada 24h
- **scheduler runtime**: tick cada 30s lee `agent_schedules` y dispara cliengine.Run
- **WS hub**: pub/sub liviano con filtro por topic (refactor a unificado pendiente)

### Frontend completo

#### Stack
Vite 8 + React 19 + TypeScript 6 + Tailwind v4 + shadcn-style components (escritos a mano, sin la CLI por TTY) + react-router-dom 7 + lucide-react + react-markdown + remark-gfm.

Bundle: ~478 KB JS / 147 KB gz. Build: `cd frontend && pnpm run build`.

#### Pantallas
1. **Login** (`/login`) — username + password (+ TOTP cuando `need_totp:true`)
2. **Chat con el main** (`/`) — pantalla principal con WS streaming
3. **System** (`/system`) — gauges radiales SVG, services systemd, top procesos, conexiones
4. **Placeholders** v1+ (Projects, Agents, Topics, Subagents)

#### Componentes UX clave
- **`Composer.tsx`**: auto-resize hasta 5 filas, paste-collapse a chips `[Pasted #N +M lines]`, drag-drop de archivos, attachments con chips lime/`pending` orange
- **`MessageBubble.tsx`**: render markdown completo (tablas, code blocks, headings, lists, blockquote) con paleta HUD; header muestra `engine · model` cuando es del agente
- **`GhostBubble.tsx`**: bubble vivo durante streaming — thinking en cursivas grises, tool cards (running/ok/error con pulse), texto markdown streaming, cursor parpadeante
- **`StatusBar.tsx`**: badge clickeable `[engine · model · ctx]`, `ctx:N%` con colores cyan/warn/danger, `plan · max 5x`, transport label (`ws · live` / `polling · 2s`)
- **`EnginePicker.tsx`**: dropdown flotante para cambiar engine + model en runtime, validación contra catálogo del backend, toast de confirmación
- **`useWebSocket`**: hook con backoff exponencial `[1s, 2s, 5s, 10s, 30s]`, status `connecting|open|closed|fallback`, fallback a polling automático

### Skills internas (`.claude/skills/`)
1. **`topic-routing`** — cómo el main detecta topic y delega
2. **`agent-management`** — crear/gestionar mini-agentes
3. **`records`** — schema-on-read para datos espontáneos
4. **`notifications`** — patrones de envío rico al user
5. **`system-control`** — get_status

## 🔴 Pendientes (en orden de prioridad)

### Crítico — refactor WS (siguiente)

| # | Tarea | Bloqueado por |
|---|---|---|
| **32** | WS unificado `/ws` con routing topic + subscribe/unsubscribe | — |
| **33** | Push de `agent_status` por WS (matar polling 5s) | #32 |
| **34** | System Manager stats por WS (sustituir polling 2s) | #32 |
| **35** | RPC bidireccional sobre `/ws` (mover POST messages, set_engine, service_action a WS actions) | #32 |

### Engines

| # | Tarea |
|---|---|
| **28** | Smoke test E2E codex con resume → exponer en picker |
| **29** | Smoke test E2E ollama → exponer en picker |
| **30** | Frontend: warning rojo si engine devuelve body vacío |

### Usage real

| # | Tarea |
|---|---|
| **36** | Calcular usage local desde JSONLs (sesión 5h + semanal 7d), Max5x estimado |

### Modelo conceptual

| # | Tarea |
|---|---|
| **23** | Fase 6 — Topics + topic_state + tools (detección de topic + delegación operativa) |

## 🟡 Decisiones técnicas que NO están en ARCHITECTURE.md

Cosas que decidí o ajusté durante la implementación que no llegaron a doc:

1. **Modelo default = `sonnet`** (no `opus-4-7` como decía la doc — ese alias el CLI no lo resuelve)
2. **`opus-1m`** es el alias interno; el cliengine lo traduce a `claude-opus-4-7[1m]` que es el ID real
3. **codex y ollama deshabilitados** en el picker hasta smoke test E2E (tasks #28/#29). Quedan en el código.
4. **Plan info** se lee de `~/.claude/.credentials.json` (subscriptionType + rateLimitTier). Best-effort, sin auth a claude.ai.
5. **Cloudflare bloquea `/api/usage` de claude.ai** con JS challenge — no se puede consumir directo. Plan B: parsear JSONLs locales (task #36).
6. **`NoNewPrivileges=true`** del systemd unit fue deshabilitado para permitir sudo NOPASSWD en service_action.
7. **`data/` está en `.gitignore`** (uploads + db + snapshots). Si retomás, no esperés ver `data/agenthub.db` versionado.
8. **Migrations idempotentes** vía tabla `__migrations(name, applied_at)`. No re-aplica las ya corridas.

## 🛠 Cómo trabajar después de retomar

### Levantar / verificar

```bash
systemctl status agenthub                          # debería estar 'active'
journalctl -u agenthub -n 50 --no-pager            # últimos logs
ss -tlnp | grep :8093                              # debería estar escuchando
curl http://127.0.0.1:8093/healthz                 # debe devolver {ok:true}
```

### Recompilar y restartear

```bash
cd /home/nestor/agenthub
go build -tags 'sqlite_fts5 sqlite_json' -o bin/agenthub ./cmd/agenthub
echo 'lazio999' | sudo -S systemctl restart agenthub
```

### Rebuild del frontend

```bash
cd /home/nestor/agenthub/frontend
pnpm run build
# El daemon ya sirve dist/ directo, no requiere restart del servicio
```

### Login y test

```bash
curl -sc /tmp/coo.txt -X POST http://127.0.0.1:8093/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"nestor","password":"Lazio999."}' > /dev/null

curl -sb /tmp/coo.txt http://127.0.0.1:8093/api/agent/status | python3 -m json.tool
```

### Ver acceso desde browser

```
http://192.168.1.52:8093/
usuario: nestor
password: Lazio999.
```

## 📋 Commits del proyecto (orden cronológico)

```
db3837d  feat(fase-0): esqueleto + auth + persistencia base
... varios commits del bloque 1+2 ...
cde208b  feat(engine picker): dropdown clickeable en el badge del StatusBar
090a318  fix(engines): solo claude por ahora + alias opus-1m correcto
2c12bea  feat(status): plan badge desde ~/.claude/.credentials.json
```

(Lista completa con `git log --oneline`).

## 📞 Mini-FAQ

- **¿WhatsApp está conectado?** No. `AGENTHUB_WA_ENABLED=false`. El bridge viejo `mcp-whatsapp-bridge.service` sigue corriendo intocado. El cutover queda pendiente.
- **¿Cómo cambio el modelo?** Click en el badge `[claude · sonnet · 200K ctx]` de la status bar → dropdown → seleccioná y aplicá.
- **¿Cómo le adjunto un archivo al agente?** Botón clip 📎 en el composer o drag-drop sobre el chat. Hasta 50MB.
- **¿Hay subagentes capturados?** Sí, en `subagent_runs` después de cada turn (parser del JSONL post-spawn). UI dedicada está en task #36+.
- **¿`opus-1m` funciona?** Sí, el alias se resuelve a `claude-opus-4-7[1m]`. Probado E2E.
