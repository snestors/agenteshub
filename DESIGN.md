# AgentHub — Diseño

Path: `/home/nestor/agenthub`
Última actualización: 2026-04-29 (v0.2.48)

> Este documento describe **cómo** AgentHub está construido y **por qué**. Las capacidades viven en `SPECS.md`; el plan de evolución en `ROADMAP.md`.

---

## Arquitectura

### Vista de bloques

```
                    ┌──────────┐    ┌──────────────┐
  Web Browser ──────│  Vite    │    │  Claude CLI  │
                    │  React   │    │  Codex CLI   │
                    └────┬─────┘    │  Ollama      │
                         │ HTTP/WS  └──────┬───────┘
                    ┌────▼──────────────────▼───────┐
                    │       agenthub daemon          │
                    │  Go · HTTP · WebSocket · MCP   │
                    ├────────────────────────────────┤
                    │  SQLite (WAL+FTS5) · Vault AES │
                    └────────────────┬───────────────┘
                                     │ whatsmeow
                              WhatsApp Web
```

### Componentes principales

| Paquete | Responsabilidad |
|---------|-----------------|
| `cmd/agenthub/` | Entry point: subcomandos `serve`, `setup`, etc. |
| `internal/server/` | Router HTTP (chi), handlers REST, WebSocket hub, slash commands. |
| `internal/cliengine/` | Wrapper de los engines (claude, codex, ollama). Spawn como child process, parseo JSONL line-by-line, mapeo a `StreamEvent`. |
| `internal/store/` | Repos sobre SQLite. Una struct + un Repo por agregado (`projects`, `agents`, `topics`, `openspec`, etc.). |
| `internal/wa/` | Bridge WhatsApp con `whatsmeow`. Worker outbox para entrega confiable. |
| `internal/mcp/` | Servidor MCP que expone tools al CLI hijo (send_message, secret_get, run_in_topic, etc.). |
| `internal/scheduler/` | Cron interno para mini-agents y jobs periódicos. |
| `internal/orchestrator/` | Coordinación de runs concurrentes y RunTracker (cancels cross-scope). |
| `internal/notif/` | Generación de notificaciones (toasts) para la UI. |
| `internal/topics/` | Lógica de topics y delegación con `run_in_topic`. |
| `internal/projects/` | Filesystem layout y descubrimiento de proyectos. |
| `internal/projecttemplates/` | Plantillas para bootstrap de proyectos nuevos. |
| `internal/skillsync/` | Sync de skills con el filesystem global del usuario. |
| `internal/sysman/` | System manager: stats, services whitelisted, processes, tunnels. |
| `internal/ws/` | Hub WebSocket: pub/sub por scope (main/project/agent). |
| `internal/auth/` | JWT cookie httpOnly + TOTP login flow. |
| `internal/buildinfo/` | Inyectado vía `-ldflags`: versión + git commit. |
| `internal/sock/` | Unix socket optional para integraciones locales. |
| `internal/setup/` | Wizard de setup inicial (genera `.env`, primer admin). |
| `internal/config/` | Carga de env vars y validación. |
| `frontend/src/` | React + Vite + Tailwind. Estética cyberpunk HUD. |

### PWA, push y update watcher

La UI web también funciona como PWA instalable. La documentación operativa completa vive en [`docs/PWA_PUSH_UPDATES.md`](docs/PWA_PUSH_UPDATES.md).

- `frontend/public/manifest.webmanifest` define metadata de instalación.
- `frontend/public/sw.js` maneja lifecycle básico, `push` y `notificationclick`.
- `frontend/src/pwa.ts` registra el service worker con query de versión (`/sw.js?v=<VERSION>`) para esquivar caches viejos de Cloudflare/navegador.
- `frontend/src/lib/firebasePush.ts` registra tokens FCM desde el navegador usando el proyecto Firebase `relogtemperatura`.
- `internal/server/push.go` persiste tokens en `push_tokens` y manda FCM HTTP v1 usando el login cacheado del Firebase CLI.
- `frontend/src/lib/uiUpdateWatcher.ts` compara los assets hasheados publicados en `index.html` contra los cargados; si cambian, emite una notificación local con botón **recargar**.

Regla de caché:

- `index.html`, `sw.js` y `manifest.webmanifest`: `Cache-Control: no-store`.
- `/assets/*`: `public, max-age=31536000, immutable` porque Vite genera nombres con hash.

### Persistencia

- **DB única**: `data/agenthub.db` (SQLite WAL).
- **Migraciones**: numeradas en `migrations/*.sql`. Aplicadas al boot.
- **FTS5**: índices full-text sobre `wa_messages` y `session_messages`.
- **Snapshots**: `session_snapshots` guarda BLOB de las JSONL del CLI como capa 2 de respaldo.
- **Vault**: AES-GCM con `AGENTHUB_SECRET_KEY` (32 bytes hex).

### Streaming en tiempo real

- Cada engine corre como child process con stdout JSONL.
- `cliengine` parsea cada línea y emite `StreamEvent{Kind, Text, Meta}` via callback `OnEvent`.
- El handler HTTP que disparó el run reenvía cada evento al WebSocket hub, scopeado por `(scope, id)`.
- El frontend mantiene un `streamsStore` que aplica chunks en orden y reconcilia con el `runtime` persistido al rehidratar.

### Flujo de un turn (web/WA)

```
[user msg] → handler HTTP/WA bridge → store_message → runtime_create
          → engine.Run(ctx, opts{OnEvent}) → child process
              ├─ JSONL line → parse → StreamEvent → ws.Publish → frontend
              └─ exit → final result → store_message(assistant) → runtime_update(done)
```

### Engines

| Engine | Binario | Streaming nativo | Notas |
|--------|---------|------------------|-------|
| `claude` | Claude Code CLI | ✅ `--output-format stream-json` | Soporta `--resume`, sub-agents reales. Modelos Anthropic + DeepSeek vía Anthropic-compatible endpoint. |
| `codex` | Codex CLI | ⚠️ buffereado hoy (falta streaming, ver ROADMAP) | Soporta NVIDIA NIM como backend OpenAI-compatible. |
| `ollama` | binario `ollama` | ✅ | Modelos locales (`gemma4:e2b`, `nomic-embed-text`). Solo apto para tareas livianas. |

### Sub-agents

- Cuando MAIN delega con `Task()`, el evento `tool_use` con `name === "Agent"` se renderiza como `SubAgentCard` separada.
- Stats reales se extraen del `toolUseResult` block del JSONL post-`cmd.Wait` y se emiten como `subagent_stats` con `Meta` enriquecido.
- Persistencia en `subagent_runs` con `parent_session_id` + `parent_scope` + `agent_type`.

---

## Decisiones congeladas

Cada decisión fechada y con motivo. No tocar sin discusión + entrada en changelog.

### 2026-04-26 · Una sola DB SQLite WAL

**Qué**: todo el estado vive en `data/agenthub.db`. No multi-DB, no Postgres, no Redis.
**Por qué**: simplicidad operativa. El user es uno solo. WAL + FTS5 cubren el caso. La carga concurrente es modesta (un agente, múltiples runs paralelos pero pocos).
**Trade-off aceptado**: SQLite no soporta dos procesos escribiendo simultáneamente. Por eso el smoke usa DB temporal aislada.

### 2026-04-26 · Engines como procesos hijos con JSONL stream

**Qué**: AgentHub no llama HTTP a los proveedores; spawnea el CLI oficial (`claude`, `codex`, `ollama`) y parsea su output.
**Por qué**: heredamos toda la UX del CLI (tools, skills, MCP, system prompt) sin reimplementarla. Cuando salga una nueva versión del CLI, AgentHub la usa sin cambios.
**Trade-off aceptado**: dependencia binaria en PATH; latencia de spawn.

### 2026-04-27 · MCP server interno expuesto al CLI hijo

**Qué**: el daemon corre un MCP server que el CLI hijo consume vía stdio.
**Por qué**: las tools `send_message`, `secret_get`, `run_in_topic`, etc. necesitan estado del daemon. MCP es el protocolo estándar para este tipo de extension.
**Por dónde mirar**: `internal/mcp/server.go` — registro de tools en `registerTools()`.

### 2026-04-27 · Vault AES-GCM con `AGENTHUB_SECRET_KEY`

**Qué**: secretos cifrados a nivel de fila con AES-GCM. Key vive en env, no en DB.
**Por qué**: si alguien copia `agenthub.db` sin la key, no puede leer los secretos.
**Trade-off aceptado**: rotación de key invalida todos los secretos viejos. No hay re-encrypt automático.

### 2026-04-28 · Sin timeout duro en runs del engine

**Qué**: los `context.WithTimeout` fueron reemplazados por `context.WithCancel`. Los turns corren mientras hagan falta.
**Por qué**: turns con fan-out a sub-agents se cortaban en `exit 143` cuando el orchestrator delegaba 15+ Tasks paralelas. Los timeouts cortos (5-30 min) eran arbitrarios y rompían el caso real.
**Mitigación**: watcher de 1h emite `long_running_turn` toast con botones inline `Cancelar`/`Continuar`.
**Ref**: v0.2.21, v0.2.23, v0.2.24.

### 2026-04-28 · DeepSeek-V4-Pro/Flash vía endpoint Anthropic-compatible directo

**Qué**: `deepseek-v4-pro` y `deepseek-v4-flash` se ejecutan con `claude` CLI usando `ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic` + `ANTHROPIC_AUTH_TOKEN` desde vault.
**Por qué**: latencia mucho mejor que pasar por Ollama Cloud; preserva resume nativo del CLI.
**Trade-off aceptado**: dependencia de que DeepSeek mantenga compatibilidad con el protocolo Anthropic.
**Ref**: v0.2.19, v0.2.27, v0.2.28.

### 2026-04-28 · OpenSpec approval-gated, sin auto-advance

**Qué**: cada cambio de fase (proposal → design → tasks → apply → archive) requiere click manual del user.
**Por qué**: defensivo. El sistema no debe aplicar cambios estructurales sin OK humano.
**Tensión actual**: el user esperaba autonomía. La fase 4 del ROADMAP introduce modo `auto`/`interactive` para resolver sin perder el gate humano clave (antes de apply).

### 2026-04-29 · Persistencia del runtime live

**Qué**: tabla `conversation_runs` guarda snapshot de ejecución (texto parcial, thinking, tools, sub-agents) para hidratar al reentrar.
**Por qué**: el chat se trababa con ghosts stale tras `safe-restart`. Sin persistencia, el cliente al reentrar no podía reconstruir el estado live.
**Ref**: v0.2.32, v0.2.33, v0.2.34.

### 2026-04-29 · Deploy blue/green smoke obligatorio

**Qué**: cualquier cambio backend pasa por build → smoke en `:8094` con DB temp + WA disabled → promote → safe-restart.
**Por qué**: SQLite WAL no permite dos escritores; matar la sesión WA del prod por un smoke es destructivo.
**Ref**: skill `.claude/skills/deploy-safe-restart/SKILL.md` v2.0.

### 2026-04-29 · PWA updates por hash de assets, no solo por versión

**Qué**: la PWA detecta updates comparando la firma de scripts/styles `/assets/*` en el `index.html` publicado contra la UI cargada.
**Por qué**: en mobile no siempre está visible el sidebar desktop que compara backend/UI version, y un frontend-only deploy puede publicar bundles nuevos antes de que el proceso backend cambie de versión.
**Mitigación de caché**: `sw.js` se registra con query de versión y se sirve con `no-store`.
**Ref**: `docs/PWA_PUSH_UPDATES.md`, v0.2.47-v0.2.48.

---

## Deploy / Operación

### Build

```bash
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X github.com/snestors/agenthub/internal/buildinfo.Version=$(cat VERSION) \
            -X github.com/snestors/agenthub/internal/buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenthub.next ./cmd/agenthub
```

### Smoke aislado en `:8094`

DB temp + WA disabled + dev mode. Validar `GET /healthz` → `{"ok":true}` antes de promover.

### Promote + restart

```bash
mv bin/agenthub.next bin/agenthub
bin/safe-restart.sh   # espera turns activos antes de SIGHUP
```

### Servicios

- `agenthub.service` — daemon principal en `:8093`.
- Whitelisted vía `.agenthub/services.yaml` para que el agente pueda operarlos sin sudo password.

### Variables de entorno clave

Ver `.env.example`. Las críticas:
- `AGENTHUB_SECRET_KEY` (32 bytes hex)
- `AGENTHUB_JWT_SECRET` (≥32 chars)
- `AGENTHUB_HTTP_ADDR` (default `0.0.0.0:8093`)
- `AGENTHUB_WA_ENABLED` (default `false`)

---

## Archivos de referencia

- `SPECS.md` — qué hace.
- `ROADMAP.md` — qué viene.
- `RELEASE_NOTES.md` — historial fechado de cambios.
- `AGENTS.md` — reglas para agentes que trabajen el código.
- `CLAUDE.md` — instrucciones del agente principal del proyecto.
- `openspec/specs/<capability>/spec.md` — specs vivas por capability (a partir de ROADMAP fase 2).
