# AgentHub Go — Arquitectura

> **North star**: un único binario Go que centraliza mi vida digital en la mini PC. Hospeda **un agente principal** (mi asistente personal por WhatsApp + Browser), **N mini-agentes especializados** (cron jobs inteligentes), y **N proyectos de coding** con sesiones independientes accesibles vía Web. Todo controlado desde un solo proyecto, con persistencia robusta.

> **Esta doc es viva — se actualiza al cerrar cada paso del paseo. Si algo acá no coincide con lo que charlamos, gana lo charlado y se actualiza.**

---

## 1. Modelo conceptual

AgentHub no es UN agente. Es un **orquestador de mi vida digital** con tres tipos de actores y un mecanismo transversal de **topics**:

```
┌─────────────────────────────────────────────────────────────────┐
│                          AGENTHUB                                │
│                                                                  │
│   ┌──────────────────┐   ┌──────────────────┐                  │
│   │  WhatsApp        │   │  Browser         │                  │
│   │  (whatsmeow)     │   │  (React)         │                  │
│   └────────┬─────────┘   └────────┬─────────┘                  │
│            ↓                       ↓                            │
│            └───────────┬───────────┘                            │
│                        ↓                                        │
│            ┌───────────────────────────┐                       │
│            │  AGENTE PRINCIPAL         │  ← chat libre        │
│            │  (mi asistente personal)  │    NO asume topic    │
│            │  • 1 session propia       │    salvo que lo      │
│            │  • detecta topic CUANDO   │    detecte por       │
│            │    hace falta             │    contenido         │
│            │  • lee topic_state        │                       │
│            │  • DELEGA via tools       │                       │
│            └────┬──────────────┬───────┘                       │
│                 ↓ delega       ↓ orquesta                      │
│        ┌────────────────┐  ┌──────────────────────────┐       │
│        │  TOPICS        │  │  MINI-AGENTES            │       │
│        │                │  │                          │       │
│        │ session propia │  │  cron / event / manual   │       │
│        │ por topic      │  │  sonarr-watcher,         │       │
│        │ contexto vivo  │  │  grid-monitor,           │       │
│        │                │  │  email-watcher, ...      │       │
│        └───────┬────────┘  └──────────────────────────┘       │
│                ↓                                                │
│        ┌──────────────────────────────────────────┐            │
│        │  PROYECTOS (entornos de coding)          │            │
│        │  ┌──────────┐ ┌──────────┐ ┌──────────┐ │            │
│        │  │ grid-bot │ │ academia │ │ agenthub │ │            │
│        │  │ N seses  │ │ M seses  │ │ K seses  │ │            │
│        │  └──────────┘ └──────────┘ └──────────┘ │            │
│        │  Cada sesión: session_id propia,         │            │
│        │   cwd al path real, contexto limpio     │            │
│        │  Acceso: SOLO browser (programar por    │            │
│        │   WA es dolor de bolas)                  │            │
│        └──────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

### 1.1 Quién hace qué

| Actor | Responsabilidad | Acceso |
|---|---|---|
| **Agente principal** | Mi asistente personal. Chat libre, vainas del servidor, descargas, salud, finanzas, lo cotidiano. **No asume topic salvo que lo detecte por contenido.** | WA + Browser |
| **Topics** | Espacios de conversación con contexto vivo (ej. `grid-bot`, `casa-media`, `salud`). El main los conoce y delega cuando hace falta. | El main delega; el user no habla directo |
| **Mini-agentes** | Cron jobs inteligentes (ej. `sonarr-watcher` cada 10 min). Reemplazan los `.sh + cron` huérfanos. | Disparados por scheduler; reportan al user o al main |
| **Proyectos** | Entornos de coding con N sesiones independientes (ej. `grid-bot/feature-bidget-fix`). Lo que reemplaza al SSH desde la oficina. | SOLO Browser |

### 1.2 Asimetría de visibilidad del agente principal

| Origen del mensaje | Respuesta del main sale por... |
|---|---|
| **WhatsApp** | WhatsApp ✅ + Browser ✅ (espejo en vivo) |
| **Browser** | Browser ✅ (solo). WhatsApp **NO** se entera. |

> *"Todo lo que pasa por WA se ve en el browser. Lo que pasa en el browser queda en el browser."*

### 1.3 El main como dispatcher inteligente

El main **detecta topics él mismo** (no Haiku, no heurística externa). Patrón típico:

```
1. Recibe tu mensaje
2. ¿Es algo casual / cotidiano? → responde directo desde su session
3. ¿Detecta que el tema es un topic? → llama tool read_topic_state(name)
   • Si con eso le alcanza para contestarte bien → responde sintetizando
   • Si necesita resolver en serio → llama tool run_in_topic(name, msg)
     → spawnea Claude con la session del topic, contexto vivo intacto
     → recibe la respuesta, te la pasa con su voz propia
```

**El main es tu asistente. Los topics son sus expertos cuando los necesita.**

### 1.4 Sub-agentes (lanzados ad-hoc por sessions)

**No confundir con mini-agentes.** Los sub-agentes son **ephemeral**: cuando una session de Claude/Codex (de proyecto, topic, mini-agente o el main) llama al tool `Agent`, se lanza un sub-agente que vive una sola corrida y termina.

| Aspecto | Mini-agentes | Sub-agentes |
|---|---|---|
| Definición | Persistentes en `agents` table | Ephemeral, no se predefinen |
| Trigger | Cron / event / manual | Tool `Agent` dentro de una session |
| Padre | Ninguno | La session que los lanzó |
| Lifetime | Continuo con N runs | Una corrida y se acabó |

**Capturados desde el `stream-json` del CLI** y persistidos en `subagent_runs`. Visibles en la UI: drawer lateral en la sesión que los lanzó + pantalla global cross-session.

### 1.5 Salida de mini-agentes

Cuando un mini-agente termina su corrida, su output se rutea según `agent_schedules.notify_target`:

| `notify_target` | Qué pasa |
|---|---|
| `wa:51922743968` | Envía resultado directo al user por WA |
| `main-agent` | El main lo recibe como input, decide si avisarte o filtrar |
| `topic:<name>` | Inyecta el resultado al `topic_state` de ese topic |
| `none` | Solo se loggea en `agent_runs`, no notifica |

---

## 2. Componentes

### 2.1 Bridge WhatsApp — `internal/wa`
- `go.mau.fi/whatsmeow` nativo Go (sin Chromium).
- Sesión persistida en `whatsmeow.SQLStore` (misma DB).
- Whitelist por número via env `WA_AUTHORIZED`.
- Handler: filtros + descarga de media + Whisper para audio.
- Comandos slash directos: `/topic`, `/topics`, `/engine`, `/reset`, `/status`, `/agents`, `/projects`.

### 2.2 Cliente Web — `frontend/`

React + Vite + Tailwind. Pantallas:

```
v0  →  PANTALLA 1: Login (TOTP)
       PANTALLA 2: Chat con el main
v1  →  PANTALLA 3: Lista de proyectos
       PANTALLA 4: Sesión activa (chat tipo Claude Code, WS streaming)
                   + drawer lateral con SUB-AGENTES de esa sesión en vivo
v2  →  PANTALLA 5: Lista de mini-agentes con sus runs
       PANTALLA 6: Lista de topics + sus topic_state
       PANTALLA 7: SUB-AGENTES (cross-session, histórico + activos)
```

Login con TOTP → JWT en cookie `agenthub_token` (httpOnly + Secure).

**Endpoints HTTP/WS** (todos detrás de `RequireJWT`):

```
POST  /api/auth/login | /api/auth/totp | /api/auth/logout
GET   /api/messages?limit=50&before=<id>           ← chat con main
POST  /api/messages                                ← enviar al main
WS    /ws/agent                                    ← stream del main

GET   /api/projects                                ← lista proyectos (v1)
GET   /api/projects/:name                          ← detalle
GET   /api/projects/:name/sessions                 ← lista sesiones
POST  /api/projects/:name/sessions                 ← crear sesión
GET   /api/projects/:name/sessions/:sid/messages   ← historial
POST  /api/projects/:name/sessions/:sid/messages   ← enviar prompt
WS    /ws/project-session/:sid                     ← stream sesión

GET   /api/topics                                  ← lista topics (v2)
GET   /api/topics/:name                            ← state + recent
GET   /api/agents                                  ← lista mini-agentes (v2)
GET   /api/agents/:name/runs                       ← logs
GET   /api/subagents?parent_session_id=&status=    ← lista sub-agent runs (v2)
GET   /api/subagents/:id                           ← detalle
WS    /ws/subagent/:id                             ← stream live de uno corriendo
```

**El estado vive en el servidor. La web es solo UI. Full WebSocket.**

### 2.3 Colas y workers

| Cola | Para qué | Concurrencia |
|---|---|---|
| **Cola del main** | Mensajes WA + Web al agente principal | FIFO, 1 a la vez |
| **Cola de delegación** | `run_in_topic` lanzado por el main | FIFO, 1 a la vez por topic |
| **Cola de proyectos** | Prompts a sesiones de proyecto desde el browser | 1 a la vez por sesión |
| **Pool de mini-agentes** | Disparos del scheduler | N goroutines (configurable) |

### 2.4 CLIEngine — `internal/cliengine`

```go
type RunOpts struct {
    Prompt    string
    SessionID string         // para --resume
    Channel   string         // 'wa' | 'web' | 'topic' | 'project' | 'mini-agent'
    Cwd       string         // /home/nestor/agenthub | /home/nestor/grid-bot | ...
    Engine    string         // 'claude' | 'codex' | 'ollama'
    OutputFmt string         // 'json' | 'stream-json'
}

func (e *Engine) Run(ctx context.Context, opts RunOpts) (Result, error) {
    // 1. Asegurar JSONL existe (restore desde snapshot si falta)
    // 2. Spawn CLI con --resume opts.SessionID
    // 3. Stream output (al WS si web, o capturar si json)
    // 4. Persistir cada turn en session_messages
    // 5. Trigger snapshot async post-turn
}
```

**`cwd` siempre apunta donde corresponde**:
- Main agent → `/home/nestor/agenthub/`
- Proyecto → `/home/nestor/<project_path>/`
- Topic → `/home/nestor/agenthub/` (los topics no son entornos de código)
- Mini-agente → `/home/nestor/agenthub/`

Esto garantiza que las **skills se descubren nativamente** desde `<cwd>/.claude/skills/`.

### 2.5 Sessions — múltiples por agente/proyecto

| Owner | Cuántas sessions | Persistidas en |
|---|---|---|
| Agente principal | 1 (`session_id="main-agent"`) | `agent_sessions` |
| Cada topic | 1 por topic | `topics.session_id` |
| Cada mini-agente | 1 propio | `agent_sessions` |
| Cada proyecto | N (una por sesión de coding) | `project_sessions` |

### 2.6 MCP Go embebido — `internal/mcp`

Servidor MCP en stdio. Lo invoca cada agente al spawnearse via `--mcp-config`. Acceso directo a DB y `whatsmeow.Client`.

#### 2.6.1 Tools de salida (mensajería al user)

| Tool | Args |
|---|---|
| `send_message` | `text` |
| `send_image` | `path` o `url`, `caption?` |
| `send_voice` | `path` (PTT, ogg/opus) |
| `send_audio` | `path` (mp3/m4a, no PTT) |
| `send_document` | `path`, `filename?`, `caption?` |
| `send_video` | `path`, `caption?` |
| `send_location` | `lat`, `lng`, `name?`, `address?` |
| `send_contact` | `vcard` |
| `react_to_message` | `message_id`, `emoji` |

#### 2.6.2 Tools de entrada (lectura WA)

| Tool | Args | Devuelve |
|---|---|---|
| `recent_messages` | `limit?`, `channel?`, `since?` | Texto + media + location |
| `unread_messages` | — | Solo no leídos |
| `get_message` | `message_id` | Mensaje completo |
| `get_media` | `message_id` | Path local |
| `transcribe_audio` | `path` o `message_id` | Whisper |
| `mark_read` / `mark_all_read` | — | |

#### 2.6.3 Tools de TOPICS *(usables por main + mini-agentes)*

| Tool | Args | Notas |
|---|---|---|
| `list_topics` | — | Inventario rápido |
| `read_topic_state` | `topic` | JSON estructurado del state |
| `update_topic_state` | `topic`, `patches` (JSON) | Patches: `headline`, `pending_add/remove`, `decisions_add`, `next_action_hint` |
| `run_in_topic` | `topic`, `message` | **Spawnea CLI con la session del topic**, devuelve respuesta |
| `topic_recent_messages` | `topic`, `limit?` | Histórico human-readable |
| `topic_create` | `name`, `description?`, `keywords?`, `project_id?` | Crear topic nuevo |

#### 2.6.4 Tools de PROYECTOS *(usables por main)*

| Tool | Args | Notas |
|---|---|---|
| `list_projects` | — | |
| `project_get` | `name` | |
| `list_project_sessions` | `project_name` | |
| `read_project_session_summary` | `session_id` | Liviano |
| `run_in_project_session` | `project`, `session_name`, `prompt` | Delegación a sesión específica |
| `project_create_session` | `project`, `name`, `engine?` | Crea nueva |

#### 2.6.4b Tools de SUB-AGENTES (lectura — main + cualquier session)

| Tool | Args | Notas |
|---|---|---|
| `list_subagent_runs` | `parent_session_id?`, `status?`, `since?`, `limit?` | Filtros por sesión padre o globales |
| `get_subagent_run` | `id` | Full result + prompt + tools_used |
| `cancel_subagent_run` | `id` | Mata sub-agente corriendo (envía SIGTERM al proceso) |

#### 2.6.5 Tools de mini-agentes *(solo agente principal)*

| Tool | Args |
|---|---|
| `agent_create` | `name`, `system_prompt`, `engine?`, `description?` |
| `agent_list` / `agent_get` / `agent_update` / `agent_delete` | |
| `agent_pause` / `agent_resume` | `name` |
| `agent_run_now` | `name`, `prompt?` |
| `agent_logs` | `name`, `limit?` |
| `agent_schedule` / `agent_unschedule` | |

#### 2.6.6 Tools de datos (schema-on-read)

| Tool | Args |
|---|---|
| `record` | `topic`, `data` (JSON), `agent_name?` |
| `query_records` | `topic`, `since?`, `jsonpath?` |
| `list_topics_records` | — |
| `delete_topic_records` | `topic` |

#### 2.6.7 Tools de sistema

| Tool | Args |
|---|---|
| `get_status` | — |
| `set_engine` | `claude` \| `codex` |
| `reset_session` | — |
| `ask_user` | `question` (bloqueante — pausa el turno hasta respuesta o timeout) |

> **Las skills viven en `.claude/skills/` y se descubren nativamente** — no hay tools `list_skills` / `read_skill`.

### 2.7 Skills — manuales modulares

Convención nativa de Claude Code. Como `cwd = /home/nestor/agenthub/` cuando corre el main/topics/mini-agentes:

```
/home/nestor/agenthub/.claude/skills/
├── agent-management/SKILL.md     # crear/gestionar mini-agentes
├── topic-routing/SKILL.md        # cómo el main detecta y delega a topics
├── scheduling/SKILL.md           # cron, schedules, triggers
├── records/SKILL.md              # registrar/consultar datos
├── notifications/SKILL.md        # patrones de envío rico
├── system-control/SKILL.md       # status, engine, reset
├── project-orchestration/SKILL.md # cómo el main interactúa con proyectos
└── per-agent/
    ├── sonarr-watcher/SKILL.md
    ├── grid-monitor/SKILL.md
    └── email-watcher/SKILL.md
```

Para proyectos de coding, sus propias skills viven en su path:

```
/home/nestor/grid-bot/.claude/skills/         # skills específicas del grid-bot
/home/nestor/academia/.claude/skills/         # skills del proyecto academia
```

Esas no las cargan main/topics/mini-agentes — solo las sesiones de ese proyecto, porque su `cwd` es el path del proyecto.

### 2.8 Persistencia — 4 capas

```
┌─────────────────────────────────────────────────────────────┐
│  CAPA 1 — JSONL en ~/.claude/projects/<encoded_cwd>/        │
│           (estado VIVO de Claude para --resume)              │
│           ↓ snapshot async cada 5 min + post-turn debounced │
│                                                              │
│  CAPA 2 — session_snapshots (BLOB en agenthub.db)           │
│           ↑ restore automático ANTES de spawn si falta JSONL│
│           ⇒ CERO "session not found"                        │
│                                                              │
│  CAPA 3 — session_messages (texto humano-legible)           │
│           Persistido por el cliengine en cada turn          │
│           ⇒ historial buscable, FTS5, base de la UI         │
│                                                              │
│  CAPA 4 — topic_state (JSON estructurado, accionable)       │
│           Actualizado por el agente del topic vía tool      │
│           ⇒ el main lee aquí, NO en el JSONL crudo          │
└─────────────────────────────────────────────────────────────┘
```

**Sin git.** Toda la persistencia vive en SQLite WAL.

### 2.9 Storage — SQLite WAL único `agenthub.db`

| Namespace | Tablas |
|---|---|
| `auth_*` | `auth_users`, `jwt_revocations` *(reciclado)* |
| `wa_*` | `wa_messages`, `wa_jid_map`, `wa_media`, `whatsmeow.*` |
| `agents` | `agents`, `agent_schedules`, `agent_runs`, `agent_sessions`, `agent_records` |
| `topics` | `topics`, `topic_state`, `session_messages` |
| `projects` | `projects`, `project_sessions`, `session_messages` |
| `snapshots` | `session_snapshots` |
| `subagents` | `subagent_runs` *(lanzados ad-hoc por sessions)* |

#### 2.9.1 Esquemas clave

```sql
-- TOPICS: espacios de conversación con contexto vivo
CREATE TABLE topics (
  id              INTEGER PRIMARY KEY,
  name            TEXT UNIQUE NOT NULL,    -- 'grid-bot', 'casa-media', 'salud'
  description     TEXT,
  keywords        TEXT,                     -- JSON array para hints al main
  project_id      INTEGER REFERENCES projects(id), -- NULL si no atado a proyecto
  session_id      TEXT NOT NULL,            -- el resume id del CLI
  engine          TEXT DEFAULT 'claude',
  is_default      INTEGER DEFAULT 0,        -- 'general' tiene esto =1
  last_active_at  INTEGER,
  created_at      INTEGER
);

-- TOPIC_STATE: memoria estructurada (lo que el main lee)
CREATE TABLE topic_state (
  topic_id          INTEGER PRIMARY KEY REFERENCES topics(id) ON DELETE CASCADE,
  headline          TEXT,                  -- "Grid bot perdiendo 1.2% en BTCUSDT"
  active_issues     TEXT,                  -- JSON array
  recent_decisions  TEXT,                  -- JSON array
  pending           TEXT,                  -- JSON array
  next_action_hint  TEXT,
  last_event_at     INTEGER,
  updated_at        INTEGER
);

-- PROYECTOS: entornos de coding accesibles desde browser
CREATE TABLE projects (
  id              INTEGER PRIMARY KEY,
  name            TEXT UNIQUE NOT NULL,    -- 'grid-bot', 'academia', 'agenthub'
  path            TEXT NOT NULL,           -- '/home/nestor/grid-bot'
  description     TEXT,
  default_engine  TEXT DEFAULT 'claude',
  created_at      INTEGER,
  updated_at      INTEGER
);

CREATE TABLE project_sessions (
  id               INTEGER PRIMARY KEY,
  project_id       INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  name             TEXT,                   -- 'main', 'feature-bidget-fix', 'debug-X'
  session_id       TEXT NOT NULL,          -- resume id del CLI
  engine           TEXT NOT NULL,
  summary          TEXT,                   -- resumen liviano para retomar
  last_active_at   INTEGER,
  created_at       INTEGER,
  UNIQUE(project_id, name)
);

-- MENSAJES de topics y proyectos (la "tercera capa" de persistencia)
CREATE TABLE session_messages (
  id              INTEGER PRIMARY KEY,
  scope           TEXT NOT NULL CHECK(scope IN ('topic','project')),
  topic_id        INTEGER,
  project_id      INTEGER,
  project_sess_id INTEGER,
  session_id      TEXT NOT NULL,           -- el resume id del CLI
  role            TEXT NOT NULL,           -- 'user' | 'assistant' | 'tool'
  body            TEXT,
  tool_name       TEXT,
  tool_args       TEXT,                    -- JSON
  tool_result     TEXT,
  cost_tokens     INTEGER,
  ts              INTEGER NOT NULL
);
CREATE INDEX idx_msgs_session ON session_messages(session_id, ts);
CREATE INDEX idx_msgs_topic ON session_messages(topic_id, ts);
CREATE INDEX idx_msgs_project ON session_messages(project_id, project_sess_id, ts);

-- SUB-AGENT RUNS: ephemeral, lanzados por sessions via tool Agent
CREATE TABLE subagent_runs (
  id                          INTEGER PRIMARY KEY,
  parent_session_id           TEXT NOT NULL,           -- session que lo lanzó
  parent_scope                TEXT NOT NULL CHECK(parent_scope IN ('main','topic','project','agent')),
  parent_topic_id             INTEGER,
  parent_project_session_id   INTEGER,
  agent_type                  TEXT,                    -- 'Explore' | 'general-purpose' | etc
  description                 TEXT,                    -- el "description" del Agent tool
  prompt                      TEXT,
  result                      TEXT,
  status                      TEXT NOT NULL CHECK(status IN ('running','ok','error','cancelled')),
  started_at                  INTEGER NOT NULL,
  finished_at                 INTEGER,
  cost_tokens                 INTEGER,
  tools_used                  TEXT,                    -- JSON array
  worktree_path               TEXT
);
CREATE INDEX idx_subagent_runs_parent ON subagent_runs(parent_session_id, started_at);
CREATE INDEX idx_subagent_runs_status ON subagent_runs(status, started_at DESC);

-- SNAPSHOTS: backup BLOB de las JSONL de Claude
CREATE TABLE session_snapshots (
  session_id   TEXT NOT NULL,
  cwd_dir      TEXT NOT NULL,              -- '-home-nestor-grid-bot'
  jsonl_data   BLOB NOT NULL,
  jsonl_size   INTEGER,
  snapshot_at  INTEGER NOT NULL,
  PRIMARY KEY (session_id)                 -- solo el último por session
);
```

**FTS5** habilitado en `wa_messages.body`, `session_messages.body`, `agent_runs.result`.

### 2.10 Auth — `internal/auth` *(reciclado)*
JWT HS256 + JTI revocations. TOTP. AES-GCM. Cookie `agenthub_token`. `RequireJWT` middleware. **Solo el browser usa auth.** WA por whitelist. CLI por permisos del Unix socket.

### 2.10b System Manager — `internal/sysman`

Visibilidad y control del **estado de la mini PC** desde la UI, sin SSH ni `htop`. Reemplaza el `/status` simple del bridge actual con un panel completo.

#### Métricas

- **CPU**: % uso global, load average 1/5/15, cores totales (12 en el i5-12450H).
- **RAM**: usada / total, swap, top consumidores.
- **Disco**: SSD root + HDD `/media/hdd`, % uso, IO read/write.
- **Temperatura**: sensores CPU.
- **Uptime** del sistema y del servicio.
- **Network**: throughput por interface, conexiones activas.
- **WhatsApp**: estado del cliente (paired / disconnected / pairing), latencia al WA server.
- **Túneles Cloudflare**: estado de los 2 tunnels conocidos (root config + per-user).
- **WS clients**: cuántos browsers conectados al daemon.

Lib Go: `github.com/shirou/gopsutil/v3` (estándar para monitoreo). Para systemd usamos `github.com/coreos/go-systemd/v22/dbus`.

#### Servicios systemd controlables

Lista de unidades **whitelisted** en config (no podés tocar cualquier cosa, solo las del whitelist):

```yaml
managed_services:
  - agenthub.service
  - whatsapp-bridge.service        # legacy, mientras dure el cutover
  - whatsapp-bridge-watchdog.timer
  - workspace-mcp.service
  - cloudflared.service
  - sonarr.service                  # si lo queremos manejar
  - radarr.service
  - qbittorrent-nox.service
  - emby-server.service
```

Acciones: `start | stop | restart | status`. **Permisos**: el daemon corre como `nestor` y usa `polkit` o `sudoers` con regla `NOPASSWD: /bin/systemctl restart <whitelist>`.

#### Tools MCP (sistema)

| Tool | Args | Quién la usa |
|---|---|---|
| `get_system_stats` | — | main, mini-agentes |
| `list_services` | `filter?` | main |
| `service_action` | `name`, `action` (`start`/`stop`/`restart`) | main (con confirmación si destructiva) |
| `list_processes` | `top?`, `sort?` (`cpu`/`mem`) | main |
| `list_tunnels` | — | main |
| `wa_connection_state` | — | main, mini-agentes |

#### Endpoints HTTP/WS

```
GET   /api/system/stats                          ← snapshot completo
WS    /ws/system                                 ← push cada 2s (CPU/RAM/etc rolling)
GET   /api/system/services                       ← lista con status
POST  /api/system/services/:name/:action         ← start/stop/restart (whitelist)
GET   /api/system/processes?top=10&sort=cpu      ← top procesos
GET   /api/system/tunnels                        ← estado tunnels
```

### 2.11 Self-heal interno
- `/healthz` endpoint propio.
- Reconexión automática a WA si cae socket.
- `Restart=always` en systemd.
- **Sesiones se restauran automáticamente** desde `session_snapshots` si el JSONL falta.

### 2.12 Cron interno — `internal/cron`
- `robfig/cron/v3` dentro del binario.
- Jobs fijos del sistema:
  - `scheduler-tick` — dispara mini-agentes (cada 30s).
  - `session-snapshot` — backup de JSONL (cada 5 min).
  - `media-rotation` — borra `wa_media/` con > 14 días.
  - `jwt-cleanup` — DELETE expirados.

---

## 3. Modos del binario

```
agenthub serve                              → daemon (default)
agenthub send <jid> <text>                  → CLI via Unix socket
agenthub send-image <jid> <path>
agenthub send-voice <jid> <path>
agenthub status                             → CLI status
agenthub mcp                                → MCP server stdio
agenthub setup-user                         → crear user + TOTP
agenthub session backup [<id>]              → forzar snapshot
agenthub session restore <id>               → restaurar desde snapshot
agenthub session list                       → listar sessions con su estado
```

---

## 4. Flujos completos

### 4.1 Vos por WA: charla casual con el main

```
1. WA → "ya descargó el último de The Last of Us?"
2. main detecta: tema → 'casa-media' (palabra "descargó")
3. main llama read_topic_state("casa-media")
   → headline: "Sonarr al día. The Last of Us E04 esperado."
4. main responde: "No todavía, esperando E04."
5. NO delegó. Respondió desde el state.
```

### 4.2 Vos por WA: pedido sobre un topic real

```
1. WA → "el grid-bot empezó a perder, mirame"
2. main detecta: tema → 'grid-bot'
3. main llama read_topic_state("grid-bot")
   → headline: "Bot en BTCUSDT, PnL +0.5%. Sin issues."
4. main ve que vos decís "empezó a perder" y el state dice "+0.5%".
   Hay desfase → necesita resolver con la session viva.
5. main llama run_in_topic("grid-bot", "el user dice: el bot empezó a perder, mirame")
6. cliengine spawn claude --resume <session_grid_bot>
7. La session del grid-bot tiene CONTEXTO COMPLETO de los últimos turns.
   Consulta logs, query a Bitget, analiza. Llama update_topic_state con findings.
8. Retorna: "Sí, perdió 1.8% en últimas 2h. Spread > 0.5% disparó pair_scanner crash.
   Te recomiendo: ..."
9. main recibe la respuesta, te la entrega con su voz: "Mirá, el bot perdió 1.8%..."
```

### 4.3 Vos en oficina abrís el browser y entrás a un proyecto

```
1. Login en /api/auth/login + TOTP → JWT cookie
2. Click "Proyectos" → GET /api/projects
3. Click "grid-bot" → GET /api/projects/grid-bot
4. Click sesión "feature-bidget-fix" → GET sessions/<id>/messages (historial)
5. WS /ws/project-session/<id> conecta
6. Vos: "retomemos donde quedamos"
7. Backend:
   - Verifica JSONL existe; si no, restore desde session_snapshots
   - cliengine.Run con cwd=/home/nestor/grid-bot/, --resume <session_id>, stream-json
   - Cada chunk → WS al browser
   - Cada turn completo → INSERT session_messages
   - Post-turn → trigger snapshot async
8. La sesión retoma con TODO el contexto, sin reenviar nada.
```

### 4.4 Vos creás un mini-agente desde WA

```
1. WA → "creame un agente que cada hora revise Sonarr"
2. main lee skill 'agent-management' (descubrimiento nativo)
3. agent_create + agent_schedule
4. main: "Listo. Próxima 09:00."

Después en cron:
1. scheduler-tick detecta sonarr-watcher.next_run <= now
2. cliengine.Run con cwd=/home/nestor/agenthub/, --resume <sonarr_watcher_session>
3. Claude descubre .claude/skills/per-agent/sonarr-watcher/SKILL.md
4. Hace su tarea, llama send_message si hay novedad
5. INSERT agent_runs con timing, result, tools_used, tokens
6. Si notify_target=topic:casa-media → update_topic_state("casa-media", patches)
```

---

## 5. Stack

- **Backend**: Go 1.23+
- **Módulo**: `github.com/snestors/agenthub`
- **Router**: `github.com/go-chi/chi/v5` + `github.com/go-chi/cors`
- **WebSocket**: `github.com/coder/websocket`
- **DB**: `github.com/mattn/go-sqlite3` (FTS5)
- **WhatsApp**: `go.mau.fi/whatsmeow`
- **Auth**: `github.com/golang-jwt/jwt/v5` + `github.com/pquerna/otp` + `golang.org/x/crypto/bcrypt`
- **Cron**: `github.com/robfig/cron/v3`
- **MCP**: `github.com/mark3labs/mcp-go` (a confirmar)
- **Frontend**: React + Vite + Tailwind

---

## 6. Estructura de carpetas

```
agenthub/
├── cmd/
│   └── agenthub/main.go            # binario único multi-modo
├── internal/
│   ├── auth/                       # ← reciclado de legacy
│   ├── config/
│   ├── store/
│   │   ├── db.go
│   │   ├── auth.go                 # ← reciclado
│   │   ├── messages.go             # wa_messages, session_messages
│   │   ├── topics.go               # topics, topic_state
│   │   ├── projects.go             # projects, project_sessions
│   │   ├── agents.go               # agents, agent_schedules, agent_runs, agent_sessions
│   │   ├── records.go              # agent_records
│   │   └── snapshots.go            # session_snapshots
│   ├── cliengine/                  # Claude / Codex / Ollama
│   │   ├── engine.go               # interface + RunOpts
│   │   ├── claude.go
│   │   ├── codex.go
│   │   ├── ollama.go
│   │   └── snapshot.go             # backup + restore de JSONL
│   ├── wa/                         # whatsmeow + handler + cola
│   ├── ws/                         # WS hub para browser
│   ├── mcp/                        # MCP server Go embebido
│   ├── topics/                     # router de topics + state manager
│   ├── orchestrator/               # gestión de mini-agentes (CRUD + spawn)
│   ├── projects/                   # gestión de proyectos y sesiones
│   ├── scheduler/                  # worker del scheduler dinámico
│   ├── server/
│   │   ├── handlers/
│   │   │   ├── auth.go             # ← reciclado
│   │   │   ├── messages.go         # /api/messages, /ws/agent
│   │   │   ├── projects.go         # /api/projects/*
│   │   │   ├── topics.go           # /api/topics/* (v2)
│   │   │   └── agents.go           # /api/agents/* (v2)
│   │   ├── router.go
│   │   └── middleware.go
│   ├── sock/                       # Unix socket server (CLI subcomandos)
│   ├── notif/                      # alertas internas
│   └── cron/
├── .claude/
│   └── skills/
│       ├── agent-management/SKILL.md
│       ├── topic-routing/SKILL.md
│       ├── scheduling/SKILL.md
│       ├── records/SKILL.md
│       ├── notifications/SKILL.md
│       ├── system-control/SKILL.md
│       ├── project-orchestration/SKILL.md
│       └── per-agent/
├── frontend/
├── migrations/
│   └── 0001_init.sql
├── deploy/
│   └── agenthub.service
├── go.mod
├── ARCHITECTURE.md
└── agenthub-go-schema-20260426.png
```

---

## 7. Decisiones cerradas

| # | Decisión | Razón |
|---|---|---|
| 1 | `whatsmeow` en vez de puppeteer | Sin Chromium. ~50× menos RAM. |
| 2 | Un solo binario, multi-modo | KISS. |
| 3 | Una sola DB SQLite WAL, **sin git, sin backups externos** | PC en casa. Snapshots viven en la misma DB. |
| 4 | Browser y WhatsApp = canales del agente principal | Es chat, no IDE. |
| 5 | Asimetría WA→both, Web→solo Web | Evitar notificaciones spam. |
| 6 | **El main es DISPATCHER, no transformador** | Detecta topic él mismo, delega cuando hace falta, mantiene su charla propia. |
| 7 | **TOPICS como entidad de primera clase** | Sessions independientes por tema. No más contaminación de contexto. |
| 8 | **Topic state como JSON estructurado** (no string libre) | Queryable, accionable, sin drift. |
| 9 | **PROYECTOS como entidad separada de coding** | Acceso solo por browser. Sessions múltiples por proyecto. |
| 10 | Mini-agentes en DB (no scripts sueltos) | Reemplaza `notify.sh`/`sonarr_notify.py`/cron. |
| 11 | **El main detecta topics él mismo, no Haiku externo** | Es su trabajo decidir cuándo delegar. |
| 12 | **Snapshots de JSONL en `session_snapshots`** | Cero "session not found". Restore automático antes de spawn. |
| 13 | **Mensajes de sesiones de proyecto y topic en DB** | Historial buscable, base de la UI, resiliencia adicional. |
| 14 | Skills viven en `.claude/skills/` | Convención nativa Claude Code, descubrimiento automático. |
| 15 | MCP Go embebido (no Node loopback) | Acceso directo. |
| 16 | Output principal: `send_message` en WA, streaming en Web | WA no tiene streaming nativo. |
| 17 | `agent_records` JSON libre (schema-on-read) | Sin tablas tipadas para datos del agente. |
| 18 | Self-heal interno + `Restart=always` | Reemplaza `watchdog.sh`. |
| 19 | CLI subcomandos via Unix socket | Reemplaza HTTP API local con tokens. |
| 20 | HTTP/WS solo para browser, con JWT | Cero endpoints HTTP de envío. |
| 21 | `auth/` reciclado de legacy | Bien aislado, portable. |
| 22 | **Sub-agentes capturados desde stream-json y persistidos en `subagent_runs`** | Visibilidad de lo que cada session lanza vía Agent tool. |
| 23 | UI tiene drawer lateral en sesión + pantalla global cross-session | Contextual (qué lanzó esta sesión) + agregado (qué corre en todo el sistema). |
| 24 | **System Manager con `gopsutil` + `go-systemd/dbus`** | Stats + servicios desde UI sin SSH. Whitelist explícito de unidades manejables. |
| 25 | Acciones systemd via polkit/sudoers `NOPASSWD` para whitelist | Operativo sin escalar privilegios cada vez. |
| 26 | **Confirmaciones mid-ejecución = tool `ask_user(question)` bloqueante, simple** | Sin distinción destructivo vs casual. Cola del bridge sabe "este jid tiene pregunta abierta" y rutea próximo mensaje a la tool en vez de a la cola normal. Timeout configurable (default 30 min). |
| 27 | **Default `notify_target` para mini-agentes nuevos = `main-agent`** | El main filtra ruido y agrega contexto. Override explícito al crear si vos querés notificación cruda. |

---

## 8. Pendiente de decisión

| Tema | Estado |
|---|---|
| Lib MCP Go a usar (`mark3labs/mcp-go` u otra) | Por verificar al arrancar Fase 3 |
| Migración de `messages.db` viejo | Postergado al cutover (Fase 11) |

---

## 9. Reciclado vs descartado

### ✅ Reciclado de `agenthub-legacy`
- `internal/auth/` (crypto, jwt, middleware, totp).
- `internal/store/auth.go` (AuthRepo + RevocationStore).
- `internal/domain/models/user.go`.
- `migrations/0001_init.sql` — tablas `auth_users` + `jwt_revocations`.
- 5 fields del config: `SecretKey`, `JWTSecret`, `JWTTTL`, `JWTRefreshTTL`, `CookieSecure`, `DevBypassTOTP`.
- `cmd/setup-user/`.

### ❌ Descartado de `agenthub-legacy`
- `internal/broker/`, `internal/spawn/`, `internal/runner/`.
- `internal/ws/` (rehecho).
- `internal/engram/`.
- `frontend/` (rehecho con foco en orquestación).

### ❌ Descartado de `mcp-whatsapp`
- `notify.sh`, `sonarr_notify.py` (lógica → mini-agentes en DB).
- `watchdog.sh` (→ self-heal interno).
- `daily-compile.sh` + marcadores HTML en `system-prompt.md` (→ cron + topic_state).
- `index.cjs` MCP Node con HTTP loopback (→ MCP Go embebido).
- HTTP API `/send`, `/send-voice`, etc. (→ subcomandos CLI).
- Tablas `meals`, `goals`, `daily_stats` (→ `agent_records` JSON).

---

## 10. Plan por fases

| Fase | Entregable | Estimación |
|---|---|---|
| 0 | Esqueleto + auth (login + TOTP + browser sirve index estático) | ½ día |
| 1 | Bridge WhatsApp con `whatsmeow` (echo a tu jid) | 1-2 días |
| 2 | CLIEngine + cola del main + comandos slash + session "main-agent" | 1-2 días |
| 3 | MCP Go embebido con tools de salida/entrada/datos | 1-2 días |
| 4 | Persistencia 4 capas (snapshots + session_messages + restore automático) | 1 día |
| 5 | Browser canal: chat UI + WS al main + reflejo de WA | 2 días |
| 6 | Topics + topic_state + tools `read/update/run_in_topic` | 1-2 días |
| 7 | Proyectos + project_sessions + UI browser para entrar y trabajar | 2-3 días |
| 7b | Captura de sub-agentes (parser stream-json + tabla `subagent_runs` + drawer UI + pantalla global) | 1-2 días |
| 7c | System Manager (stats + servicios systemd + UI panel `/ui/system`) | 1-2 días |
| 8 | Mini-agentes: `agent_create`, scheduler, skill `agent-management` | 2 días |
| 9 | Migrar `sonarr-watcher` desde `sonarr_notify.py` (prueba de fuego) | 1 día |
| 10 | Paridad final: Whisper, Ollama fallback, daily-summary cron | 1-2 días |
| 11 | Cutover: bajar bridge viejo, levantar agenthub.service | ½ día |

**Total**: ~14-16 días de trabajo concentrado.

---

## 11. Estado actual

- [x] Análisis exhaustivo de `mcp-whatsapp` y `agenthub-legacy`
- [x] Modelo conceptual cerrado (orquestador con 3 actores + topics + 4 capas de persistencia)
- [x] `agenthub-legacy` archivado en `/home/nestor/agenthub-legacy/`
- [x] `/home/nestor/agenthub/` listo
- [x] Server local de preview en `http://192.168.1.52:8088/`
- [x] ARCHITECTURE.md refleja modelo final (este doc)
- [x] Diagrama actualizado con 3 actores + topics + persistencia
- [x] Cerrado: confirmaciones mid-ejecución = tool `ask_user` bloqueante simple
- [x] Cerrado: default `notify_target` = `main-agent`
- [x] Cerrado: skills a usar (8 elegidas + propias en `.claude/skills/`)
- [x] Mockups UI completos (chat, proyectos, sub-agentes, mini-agentes, topics, system manager)
- [x] **Fase 0** — esqueleto + auth ✓
- [x] **Fase 1** — bridge WhatsApp con whatsmeow (apagado en dev hasta cutover)
- [x] **Fase 2** — CLIEngine + comandos slash + session main
- [x] **Fase 3** — MCP Go embebido (22 tools)
- [x] **Fase 4** — persistencia 4 capas + snapshots automáticos
- [x] **Fase 5** — frontend completo (chat, system manager, login)
- [x] Bloque 2 — chat con main UI + WS streaming + ghost bubble + markdown + composer rico
- [x] Engine picker UI + opus-1m funcional + plan badge
- [ ] **Próximo: refactor WS unificado** (task #32) — luego topics (Fase 6) — luego mini-agentes UI

**Para detalle exacto del estado de implementación, ver `IMPLEMENTATION-STATUS.md`.**

---

## 12. Comandos útiles

```bash
# Compilar
cd /home/nestor/agenthub && go build -o bin/agenthub ./cmd/agenthub

# Crear usuario inicial (TOTP QR)
./bin/agenthub setup-user

# Run dev
./bin/agenthub serve

# Mensaje desde script trivial
./bin/agenthub send 51922743968@s.whatsapp.net "hola"

# Forzar snapshot de todas las sessions
./bin/agenthub session backup

# Restaurar una session específica
./bin/agenthub session restore <session-id>

# Ver topics y su state
sqlite3 agenthub.db "SELECT t.name, ts.headline, ts.updated_at FROM topics t LEFT JOIN topic_state ts ON ts.topic_id=t.id;"

# Ver proyectos y sus sesiones activas
sqlite3 agenthub.db "SELECT p.name, ps.name, ps.last_active_at FROM projects p JOIN project_sessions ps ON ps.project_id=p.id WHERE ps.last_active_at > strftime('%s','now','-7 days');"

# Ver runs recientes de mini-agentes
sqlite3 agenthub.db "SELECT a.name, ar.status, ar.started_at, ar.cost_tokens FROM agent_runs ar JOIN agents a ON a.id=ar.agent_id ORDER BY ar.started_at DESC LIMIT 20;"

# Histórico de mensajes en una sesión de proyecto
sqlite3 agenthub.db "SELECT role, body, ts FROM session_messages WHERE session_id='<sid>' ORDER BY ts;"
```

---

*Doc viva — actualizar al final de cada paso del paseo.*
