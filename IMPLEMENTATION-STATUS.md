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
  - **CodexEngine**: `codex exec --json` con resume validado E2E (expuesto en picker como `gpt-5.5`)
  - **OllamaEngine**: HTTP `:11434` (NO expuesto, requiere setup)
- **Streaming**: cuando `OnEvent != nil`, usa `--output-format stream-json --verbose` y emite `StreamEvent` por cada `text`/`thinking`/`tool_use`/`tool_result`/`final`
- **Resume robusto**: antes de spawn, valida que existe `~/.claude/projects/<encoded_cwd>/<sid>.jsonl`. Si falta, restaura desde `session_snapshots`. **Cero "session not found"**.
- **MCP config auto-generado**: `/tmp/agenthub/mcp.json` apunta el CLI a `agenthub mcp` con env vars necesarias
- **Sub-agent capture**: post-turn parsea el JSONL buscando `tool_use` con `name=Agent` y persiste en `subagent_runs`
- **Resolver de aliases**: `opus-1m` → `claude-opus-4-7[1m]` (los `[]` son obligatorios para el CLI)
- **Sesiones por engine**: el main usa `main-agent:<engine>` para no mezclar resume IDs de Claude y Codex; fallback legacy a `main-agent`.

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
- **Chat**: `GET /api/messages`, `POST /api/messages` (compat curl/scripts), `WS /ws` topic `agent` + action `send_message`
- **Agent control**: `GET /api/agent/status` (incluye plan + usage local), `GET /api/agent/engines`, `POST /api/agent/engine` (compat) + WS action `set_engine`
- **System**: `GET /api/system/stats`, `/services`, `/processes`, `/connections`, `POST /api/system/services/{name}/{action}` (compat) + `WS /ws` topic `system` + action `service_action` con `{name, op}`
- **Uploads**: `POST /api/upload` (max 50MB), `DELETE /api/uploads/{id}`
- **Health**: `GET /healthz`
- **Frontend SPA**: `GET /` y `/*` sirven `frontend/dist/`; `index.html`/fallback van con `Cache-Control: no-store`, `/assets/*` hasheados con `public, max-age=31536000, immutable`, otros estáticos con `public, max-age=3600`.

#### Database (SQLite WAL)
14 tablas + FTS5: auth_users, jwt_revocations, settings, wa_messages (con engine+model), wa_jid_map, topics, topic_state, projects, project_sessions, agents, agent_schedules, agent_runs, agent_sessions, agent_records, session_messages, subagent_runs, session_snapshots, __migrations.

Migrations idempotentes via tabla `__migrations(name, applied_at)`.

#### Servicios internos
- **systemd**: una sola unit `agenthub.service` con `Restart=always`
- **cron interno** (robfig/cron): session-snapshot cada 5min, jwt-cleanup cada 24h
- **scheduler runtime**: tick cada 30s lee `agent_schedules` y dispara cliengine.Run
- **WS hub**: `/ws` unificado con pub/sub por topic y RPC bidireccional con acks correlacionados. Endpoints legacy `/ws/agent` y `/ws/system` removidos (404).

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
- **`StatusBar.tsx`**: badge clickeable `[engine · model · ctx]`, `ctx:N%` con colores cyan/warn/danger, `plan · max 5x`, transport label (`ws · live` / `ws · reconnect…`); agent_status llega 100% por WS.
- **`EnginePicker.tsx`**: dropdown flotante para cambiar engine + model en runtime, validación contra catálogo del backend, toast de confirmación
- **`wsClient` + `useTopic`**: singleton `/ws` con subscribe/unsubscribe por topic, backoff exponencial y re-suscripción automática; el hook legacy `useWebSocket` fue removido.

### Skills internas (`.claude/skills/`)
1. **`topic-routing`** — cómo el main detecta topic y delega
2. **`agent-management`** — crear/gestionar mini-agentes
3. **`records`** — schema-on-read para datos espontáneos
4. **`notifications`** — patrones de envío rico al user
5. **`system-control`** — get_status

## 📋 TODO List canónico — actualizado

> Esta es la fuente de verdad de las tareas. Las TaskList del runtime de Claude Code se pierden con `/compact`; este bloque NO. Si retomás, **leé esto y volvé a crear las tasks abiertas con `TaskCreate`** para tener trazabilidad en la sesión.

### ✅ Completadas (actualizado al cierre)

**Cerradas en esta sesión post-compact**:
- #32 (#37) WS unificado `/ws` con routing por topic
- #33 (#38) Push de agent_status por WS
- #34 (#39) System Manager push (service_event + skip work cuando nadie escucha)
- #35 RPC bidireccional sobre `/ws` para `send_message`, `set_engine`, `service_action` (`op`)
- #30 Warning visible cuando el engine devuelve body vacío
- #28 Codex smoke E2E validado y expuesto como `codex · gpt-5.5`
- #36 Usage local desde JSONLs (5h + 7d) persistido en settings y mostrado en StatusBar
- Limpieza WS legacy: removidos `/ws/agent`, `/ws/system`, `legacyTopic`, hook viejo `useWebSocket` y fallbacks de polling de Chat/System stats/StatusBar.
- Cache headers frontend: HTML/fallback `no-store`, assets hasheados immutable 1 año, estáticos no hasheados 1 hora.



| # | Tarea |
|---|---|
| 16 | Fase 0 — Esqueleto + auth de AgentHub |
| 17 | Bloque 2 — Canal web del main + topics (UI base lista; routing operativo de topics queda en #23) |
| 18 | Fase 1 — Bridge WhatsApp con whatsmeow (apagado en dev hasta cutover) |
| 19 | Fase 2 — CLIEngine + cola + comandos slash |
| 20 | Fase 3 — MCP Go embebido stdio (22 tools) |
| 21 | Fase 4 — Persistencia 4 capas (snapshots + restore automático) |
| 22 | Fase 5 — Frontend React+Vite+Tailwind+shadcn |
| 24 | Deploy — systemd unit + sudoers NOPASSWD |
| 25 | WS Hub Go (versión inicial; refactor unificado en #32) |
| 26 | Streaming claude stream-json + broadcast WS por chunks |
| 27 | Backend: upload + agent/status + messages con attachments |

### 🔲 Abiertas (5) — orden de prioridad recomendado

#### Crítico — refactor de transporte WS (hacer primero, en este orden)

**#32 — WS unificado `/ws` con routing por topic + subscribe/unsubscribe**

Refactor crítico completado: `/ws/agent` + `/ws/system` fueron reemplazados por un único endpoint `/ws` con protocolo JSON pequeño:

```
CLIENT → SERVER:
  { action: "subscribe",   topic: "agent" }
  { action: "unsubscribe", topic: "agent" }
  { action: "ping" }

SERVER → CLIENT (envelope existente):
  { type: "message" | "stream" | "stats" | "status" | ..., topic, payload, ts }
```

Cambios backend:
- `internal/ws/hub.go`: `Client.subscribed map[string]bool` en vez del topic fijo
- `Register` devuelve helpers `Subscribe(topic)`/`Unsubscribe(topic)`
- `Pump` lee mensajes del cliente y procesa actions, además de write loop
- `Broadcast` filtra: si `client.subscribed[env.Topic] || subscribed["*"]`

Cambios frontend:
- Hook legacy `useWebSocket` removido → `wsClient` singleton + `useTopic`
- Hook helper `useTopic(name, onMessage)` que sube/baja la suscripción al mount/unmount
- `ChatMain`, `System`, `StatusBar` dejan de tener su propio WS — todos usan `useTopic`
- Reconnect coordinado: cuando vuelve, re-suscribe a todos los topics activos

Endpoints viejos `/ws/agent` y `/ws/system`: removidos; el catch-all SPA devuelve 404 para `/ws/*` desconocido.

---

**#33 — Push de `agent_status` por WS (eliminar polling cada 5s)** ✅ COMPLETADA

Una vez en `/ws` unificado, eliminar el polling de `/api/agent/status` cada 5s. El server emite `{ type:'status', topic:'agent_status', payload:{...} }` cuando:
1. Después de cada respuesta del agente (ctx_used cambió)
2. Cuando el user cambia engine/model via `/api/agent/engine`
3. Heartbeat cada 60s (catch-all para drift por cambios externos)
4. On-demand: cliente envía `{action:'status.refresh'}` y server emite

`GET /api/agent/status` sigue existiendo para compat/curl, pero `StatusBar` ya no lo pollea; usa `useTopic("agent_status")` y heartbeat server cada 60s.

---

**#34 — System Manager stats por WS (sustituir polling 2s)** ✅ COMPLETADA

Implementado: `startSystemPoller` pushea stats al topic `'system'` cada 2s SOLO si hay clientes suscritos (`CountSubscribed("system")`). Endpoint legacy `/ws/system` removido; en frontend se removió el polling fallback de stats.

Para cambios de servicios (start/stop/restart), también pushear `{ type:'service_event', topic:'system', payload:{name,action,result} }` al instante para refresh inmediato sin esperar el siguiente tick.

---

**#35 — RPC bidireccional sobre `/ws` para acciones del frontend** ✅ COMPLETADA

Una vez que el WS sea unificado y bidireccional, mover acciones HTTP que son cortas a WS:
- `POST /api/messages` (chat) → `{ action: 'send_message', body, attachments }`
- `POST /api/agent/engine` → `{ action: 'set_engine', engine, model }`
- `POST /api/system/services/{name}/{action}` → `{ action: 'service_action', name, op }` (`op` evita chocar con el `action` RPC)

Implementado con `type: "ack"` y `payload.id` correlacionado. `send_message` persiste y ACKea rápido con `message_id`; el engine corre async y sigue emitiendo `stream`/`message`. Handlers HTTP siguen existiendo para curl/scripts.

#### Engines

**#28 — Habilitar codex en el picker (smoke test E2E)** ✅ COMPLETADA

`codex` CLI 0.124.0 validado: spawn limpio, resume con `codex exec resume <thread_id>`, parsing de `thread.started`/`item.completed`/`turn.completed`. `gpt-5-codex` no está habilitado en esta cuenta; se expuso `codex · gpt-5.5`.

---

**#29 — Habilitar ollama en el picker**

Verificar que ollama está corriendo en `localhost:11434` con modelo descargado. `cliengine.OllamaEngine` ya existe pero no testeado E2E. Manejar timeouts y errores. Agregar a `availableEngines` cuando funcione.

---

**#30 — Frontend: mostrar warning si engine devuelve body vacío** ✅ COMPLETADA

Cuando un engine responde con `body=[vacío]` o solo espacios, `MessageBubble` muestra `⚠ El engine no devolvió respuesta. Probá cambiar de modelo o reintentar.` manteniendo header engine/model.

#### Usage

**#36 — Calcular usage local desde JSONLs (alternativo a Cloudflare)** ✅ COMPLETADA

Para mostrar 'sesión 66% · semanal 63%' como en `claude.ai/settings/usage`. NO se puede consumir el endpoint directo (Cloudflare JS challenge). Plan B local:

Implementado: walk streaming de `~/.claude/projects/**/*.jsonl`, ignora líneas inválidas, suma `usage.input_tokens` en ventanas 5h/7d, cron cada 30m + cálculo inicial al startup, persiste `usage_session_pct`, `usage_week_pct`, `usage_calculated_at`, `usage_session_tokens`, `usage_week_tokens`; `agent_status` los expone y StatusBar muestra `sesión N% · semana N%`.

Inspiración: https://github.com/ryoppippi/ccusage

Nota: los porcentajes serán aproximados — Anthropic no publica los límites exactos del plan. Lo correcto vs `claude.ai/settings/usage` puede diferir un 5-10%.

#### Modelo conceptual

**#23 — Fase 6 — Topics + topic_state + tools**

Sistema de detección automática de topic + delegación operativa por el agente principal. Hoy los topics existen como tabla y tools MCP (`list_topics`, `read_topic_state`, `update_topic_state`) pero el routing automático del main hacia topics no está activo. La skill `topic-routing` ya describe el patrón. Falta:
- En el system prompt del main, instrucción de leer `read_topic_state` antes de responder cuando detecta tema específico
- Tool `run_in_topic(topic, message)` (declarada en doc, no implementada) que delega el turno a la session de un topic con `--resume <topic.session_id>`
- Persistir `topics.session_id` automáticamente al primer uso

## 🔴 Pendientes (en orden de prioridad)

### Engines

| # | Tarea |
|---|---|
| **29** | Smoke test E2E ollama → exponer en picker |

### Modelo conceptual

| # | Tarea |
|---|---|
| **23** | Fase 6 — Topics + topic_state + tools (detección de topic + delegación operativa) |

## 🟡 Decisiones técnicas que NO están en ARCHITECTURE.md

Cosas que decidí o ajusté durante la implementación que no llegaron a doc:

1. **Modelo default = `sonnet`** (no `opus-4-7` como decía la doc — ese alias el CLI no lo resuelve)
2. **`opus-1m`** es el alias interno; el cliengine lo traduce a `claude-opus-4-7[1m]` que es el ID real
3. **codex habilitado** en el picker como `gpt-5.5` tras smoke E2E; `gpt-5-codex` fue rechazado por la cuenta ChatGPT actual. Ollama sigue deshabilitado hasta #29.
4. **Plan info** se lee de `~/.claude/.credentials.json` (subscriptionType + rateLimitTier). Best-effort, sin auth a claude.ai.
5. **Cloudflare bloquea `/api/usage` de claude.ai** con JS challenge — se implementó Plan B local: parsear JSONLs y estimar 5h/7d. Límites configurables: `AGENTHUB_USAGE_SESSION_TOKEN_LIMIT` (default 88K) y `AGENTHUB_USAGE_WEEK_TOKEN_LIMIT` (default 3M, estimación gruesa).
6. **`NoNewPrivileges=true`** del systemd unit fue deshabilitado para permitir sudo NOPASSWD en service_action.
7. **`data/` está en `.gitignore`** (uploads + db + snapshots). Si retomás, no esperés ver `data/agenthub.db` versionado.
8. **Migrations idempotentes** vía tabla `__migrations(name, applied_at)`. No re-aplica las ya corridas.
9. **WS RPC service_action usa `op`**, no `action`, para evitar colisión con el campo action del envelope RPC.
10. **Cache frontend explícito**: HTML/fallback con `no-store`; `/assets/*` de Vite con hash cacheados 1 año immutable; estáticos sin hash 1h.
11. **Catch-all SPA no captura `/ws/*` desconocido**: tras remover legacy, `/ws/agent` y `/ws/system` deben responder 404, no `index.html`.

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
