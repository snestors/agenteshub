# AgentHub — Release Notes

Path: `/home/nestor/agenthub`

## v0.5.0 — 2026-05-02

Frontend rebrand pass del Claude Design handoff. Tres sprints:
**A** HUD lateral derecho · **B** sistema de notificaciones · **C** BIT (mascota pixel-art notif assistant).

### Added — Sprint A · ChatHUD lateral

- **`<ChatHUD>`** (componente nuevo) montado en `/chat` y `/projects/:id/sessions/:sid`. Secciones (visibles según scope):
  - **SESSION** — id corto, scope, started, status, ws conectado.
  - **ENGINE** — engine/model/ctx window leídos de `/api/agent/status` + `/api/agent/runtime`.
  - **FEATURE_LIST** (solo project) — counters por status (`pending/in_progress/done/blocked`) + lista scrollable. Cuando el proyecto no tiene scaffold, muestra CTA `scaffold harness` que postea `/api/projects/:id/harness/scaffold`.
  - **TOKENS · session** — número grande + bar de progreso desde `usage_session_pct`.
  - **SUBS · 5H** — radial pct + countdown a reset (parsed de `/api/usage/realtime` `claude.session`).
  - **AGENTS · Runtime** — counters de main + project running.
  - **STACK** — topic activo + tools usados (de `runtime.tools`).
- Colapsable; preferencia persiste en `localStorage('agenthub.hud.collapsed')`.
- Animaciones nuevas en `index.css` (handoff parity): `anim-fade-in-up`, `anim-zip-in`, `anim-bounce`, `anim-heartbeat`, `anim-ring-pulse`, `anim-bell-shake`, `anim-drawer-in`, `anim-toast-pop`, `anim-hud-cascade`, `anim-hud-stagger`. La `.hud-scan` ahora animada (era estática). Todas honran `prefers-reduced-motion`.
- `api.ts` types `ProjectFeatures`, `FeatureItem`, `HarnessFileSnapshot`, `ProjectHarnessState` + métodos `api.getProjectFeatures`, `api.getProjectHarnessState`.
- Polling de 6s en cada caller (`ChatMain` y `Projects.tsx`) para nutrir el HUD; best-effort, una falla no bloquea el chat.

### Added — Sprint B · Sistema de notificaciones

- **DB** — migration 0021 crea `notifications(id, kind, severity, title, body, context_json, created_at, read_at)` + índices `idx_notif_created`, `idx_notif_unread`.
- **`internal/store/notifications.go`** — `NotificationsRepo` con `Insert` (idempotente en id), `List` (con filtro unread), `CountUnread`, `MarkRead`, `MarkAllRead`. `Repos.Notifications` wired en `db.go`.
- **`broadcastNotification`** ahora **persiste antes de fan-out WS** — un cliente que se conecta después igual ve la notif via REST.
- **3 endpoints nuevos**:
  - `GET /api/notifications?unread=true&limit=N` → `{items, unread_count}`
  - `POST /api/notifications/{id}/read`
  - `POST /api/notifications/read-all`
- **`<NotifProvider>`** wrap en `AppShell`. Carga inicial REST + suscripción WS topic="notifications". Cada evento prepende a items, bumpea unread, dispara bell shake + toast 5s.
- **`<NotifBell>`** en Topbar — icono campana con shake on new + badge naranja `anim-ring-pulse`. Click abre el drawer.
- **`<NotifDrawer>`** — slide-right desde la derecha (`anim-drawer-in`), backdrop blur, lista scrollable con dots por severity, "marcar leídas" + cerrar.
- **`<NotifToast>`** — top-right pop-in (`anim-toast-pop`), kind/title/body, click abre drawer; progress bar CSS-driven que vacía en 5s.
- `useNotifCenter()` expuesto para futuros consumers (BIT lo usa abajo).

### Added — Sprint C · BIT pixel-art assistant

- **`<Bit>`** componente nuevo montado en AppShell. 4 sprite PNG en `/public/bit/` (`idle`, `happy`, `excited`, `sleep`).
- **Reacciones**:
  - Notif nueva (unread) → `excited` + bounce + bubble con `kind/title/CTA "abrir drawer"`. Vuelve a `happy` a 1.5s, bubble cierra a 5.5s.
  - 35s de inactividad → `sleep` con bubble zZ.
  - Click con unread > 0 → abre drawer. Sin unread → bubble corto "estoy acá".
- **Drag & drop** con pointer events; posición persiste en `localStorage('agenthub.bit.pos')`.
- **Botón × esquinita** para ocultar; toggle persistido en `localStorage('agenthub.bit.hidden')`. Aparece un ◆ chico abajo-derecha para reabrirlo.
- Performance: un solo `<img>` absolutamente posicionado + bubble. CSS transitions para el glide. `imageRendering: pixelated` para el sprite crisp.

### What's NOT in 0.5.0

- **3b** ChatShell extracción (refactor) — los chats `ChatMain` y `ProjectChat` siguen siendo componentes paralelos. La unificación es del componente HUD + animaciones, no de la implementación del feed. Se difiere a 0.5.x si surge dolor.
- **3d** fly-pill animation (tool-call → counter del HUD). Diferida.
- **f-014** delegate_to_project MCP tool. Sigue pendiente desde 0.4.0.
- **f-015** 404 explícito en `/api/*` no registradas. Sigue pendiente.
- **f-006** Harness Fase 2b (diff/update flow del template). Sigue pendiente.

---

## v0.4.3 — 2026-05-02

Removal-only release. Closes **f-002 (rip mini-agent system completo)** del feature_list. Solo quedan los dos tiers de agentes que el user quiere: el main-agent (Jarvis personal) y los project-agents (uno por proyecto registrado, con su harness BettaTech). No más cron loop, no más `/api/agents/*`, no más Ollama mini-agentes.

### Removed

- **Backend Go (~1142 líneas):**
  - `internal/server/agents.go` (495) — 8 handlers de mini-agente
  - `internal/server/agent_templates.go` (120) — `/agents/templates`
  - `internal/store/agents.go` (265) — `AgentsRepo`
  - `internal/scheduler/` paquete entero (262) — cron loop
- **8 routes** `/api/agents/*` removidas de `server.go`. `Repos.Agents` field + `NewAgentsRepo` también.
- **`/healthz`** ya no consulta `agent_runs` (la tabla está dropped abajo).
- **`systemTick.RunningAgents`** removido — el WS payload de stats ahora cuenta solo main + project.
- **MCP tools** (7): `agent_create`, `agent_list`, `agent_run_now`, `agent_logs`, `agent_schedule`, `agent_pause`, `agent_resume` y todos sus handlers + `toggleAgent` helper. La rama `"mini-agent"` del switch de scope fallback en `sendMedia` también se fue.
- **Frontend:**
  - `frontend/src/pages/Agents.tsx` (huérfana: no estaba routeada desde `App.tsx` ni listada en `Sidebar.tsx`)
  - `frontend/src/lib/api.ts`: tipos `MiniAgent` / `AgentSchedule` / `AgentRun` + 8 métodos (`listAgents`, `createAgent`, `getAgent`, `setAgentEnabled`, `runAgentNow`, `listAgentRuns`, `addAgentSchedule`, `setAgentScheduleEnabled`, `deleteAgentSchedule`).
- **Skill `.claude/skills/agent-management/SKILL.md`** removida.

### Database

- **Migration 0020** — `DROP TABLE agents`, `agent_schedules`, `agent_runs` (+ sus índices).

Tablas que SUENAN relacionadas pero NO son mini-agentes y se quedan:
  - `agent_sessions` — engine-scoped `session_id` store para resume de main + project (lo usa `SessionsRepo`).
  - `agent_records` — schema-on-read JSON facts que el main-agent persiste (`source='agent'` es nombre legacy, sigue en uso).
  - `subagent_runs` — capturas de Task-tool sub-spawns desde JSONL post-hoc, producidas por el main runtime.

### Notas

- `scope='agent'` se conserva en el `RunTracker` y en MCP `sendMedia` porque `diagrams.go` sigue corriendo el mermaid generator bajo ese scope (`cliengine.Run` con `AgentName='diagram-architect'`). Ya no es exclusivo de mini-agentes — es scope para "one-shot agent runs no asociados a una sesión".
- Ollama deja de tener cualquier dependencia funcional. Los `cfg.OllamaURL` / `cfg.OllamaModel` siguen en `internal/config/config.go` por si en el futuro Claude llama a Ollama como tool subordinada (tipo `delegate_to_ollama`); hoy son dead code.

### Breaking

- Cualquier consumer de `/api/agents/*` recibe **404**.
- Los 7 MCP tools `agent_*` ya no se ofrecen al spawnear `agenthub mcp`. Si Claude Code estaba usando `agent_create` o similares, ahora dará "tool not found".

---

## v0.4.2 — 2026-05-02

Refuerzo de las dos invariantes que el user pidió en el plan de jerarquía: **WA solo va al main-agent** (f-003) y **Codex es siempre tool, nunca engine principal** (f-004).

### Added

- **`validPrimaryEngine` / `validPrimaryEngineModel`** en `internal/server/engines.go`. Listan los engines admitidos como engine principal de una sesión (main o project). Hoy solo `claude`; Codex queda fuera porque solo se invoca como tool desde Claude (`delegate_to_codex`). El catálogo `/api/agent/engines` sigue exponiendo Codex para que el frontend lo describa — el gate vive en los setters, no en el listing.
- **7 tests nuevos** (`engines_test.go`) pinean el contrato: claude aceptado, codex rechazado en todos los formatos, modelos no-claude rechazados, catálogo sigue listando ambos.

### Changed

- **f-004 — gates en los 4 setters de engine primario:**
  - `POST /api/agent/engine` (main-agent) — `validEngineModel` → `validPrimaryEngineModel`.
  - `POST /api/projects` (default_engine del proyecto) — `validEngine` → `validPrimaryEngine`.
  - `POST /api/projects/{id}/sessions` (engine + model de sesión nueva) — ambos checks migrados.
  - `POST /api/projects/{id}/sessions/{sid}/engine` (cambio de engine en sesión existente) — `validEngine` → `validPrimaryEngine`. Sesiones existentes con `engine=codex` pre-gate siguen corriendo (la regla de inmutabilidad del session engine las preserva).
- **f-003 — invariante documentada:** `routeConversationInput` y `StartWAConsumer` llevan ahora un comentario explícito que explica que el `scope` de cualquier mensaje WA es siempre `"main"`. La estructura `conversationInput` no tiene un campo `Scope`, así que el invariante ya valía por construcción; el comentario evita que un refactor lo rompa silenciosamente.

### Notas

- **Deuda visual (0.5.0):** el dropdown de Engine en el frontend va a seguir mostrando `Codex` aunque el backend lo rechace. UX: el user verá un error 400 al guardar. La limpieza del dropdown entra con la Fase 3 (frontend).
- `handleAgentsCreate` (mini-agentes) sigue usando `validEngine` no-restringido. Se barre con f-002 cuando ripeemos el sistema mini-agente completo.

---

## v0.4.1 — 2026-05-02

Removal-only release. Closes **f-001 (rip OpenSpec)** y **f-005 (skill + CLAUDE.md actualizados a promote.sh)** del feature_list de agenthub. La SDD-style proposal-design-tasks-apply-verify se reemplaza por la disciplina del harness BettaTech (loop leader/implementer/reviewer + feature_list.json + CHECKPOINTS.md) que llegó en 0.4.0.

### Removed

- **OpenSpec backend** — `internal/server/openspec.go` (824 líneas, 8 handlers), `internal/projects/openspec.go` (210), `internal/projects/openspec_test.go` (181), `internal/store/openspec.go` (145). Las 8 routes `/api/projects/{id}/openspec/*` salieron de `server.go`. `Repos.OpenSpec` field + `NewOpenSpecChangesRepo` también quitados.
- **OpenSpec frontend** — types `OpenSpecState/Change/ChangeDetail/Spec` y 6 métodos API en `frontend/src/lib/api.ts`. En `Projects.tsx` el tab "changes", el componente `ProjectChanges` y todos sus auxiliares (`OpenSpecCreateModal`, `GateActions`, `MarkdownDoc`, `SpecsView`, `InnerTab`, `StateBadge`, `contentForTab`, `tabForState`) — ~263 líneas. Imports inused (`GitBranch`, `FileText`, `Pencil`, `Check`, `Loader2`) podados.
- **Skill `.claude/skills/sdd-workflow/`** y **dir `openspec/`** completos (cambios in-flight `fix-codex-engine-killed-tasks` y `stream-codex-engine-events` se descartan).
- **Cleanup de comentarios** — referencias a "openspec" eliminadas de `runtracker.go`, `long_running.go`, `scheduler.go`, `canon.go` (este último ya no llamaba `EnsureOpenSpecLayout` al canonicalizar proyectos).

### Database

- **Migration 0019** — `DROP TABLE openspec_changes` + `DROP INDEX idx_openspec_state`. `conversation_runs` rebuilded sin `'openspec'` en su `CHECK(scope IN ...)` enum (SQLite no permite ALTER de CHECK in place); rows con scope='openspec' se descartan porque su productor desapareció.

### Changed

- **`.claude/skills/deploy-safe-restart/SKILL.md`** ahora apunta a `bin/promote.sh` en todas las referencias de promote (version 2.1). Anti-pattern documentado: `mv bin/agenthub.next bin/agenthub && bin/safe-restart.sh` deja `bin/agenthub.prev` igual al binario nuevo y pierde el rollback.
- **`.claude/CLAUDE.md`** receta exacta paso 6 colapsado a un solo `bin/promote.sh`. Reglas no negociables actualizadas.

### Breaking

- Cualquier consumer de `/api/projects/{id}/openspec/*` ahora recibe **404**. Reemplazo: la vista de `feature_list.json` (HUD lateral) que llega en 0.5.0 con la Fase 3 frontend.

---

## v0.4.0 — 2026-05-02

Primera fase del **harness BettaTech**: API + plumbing necesario para que cada proyecto registrado en agenthub adopte un loop de trabajo disciplinado (leader → implementer → reviewer + `feature_list.json` + `init.sh` + `CHECKPOINTS.md`). El backend ya está completo y se puede ejercer hoy desde MCP. La parte visual (chat unificado `<ChatShell>` con HUD lateral, sistema de notificaciones y BIT pixel-art) queda para 0.5.0 frontend-only.

### Added

- **Endpoints HTTP del harness** bajo `/api/projects/{id}/...`:
  - `GET /features` — lee `feature_list.json` del cwd. Schema: `{version, updated_at, features:[{id,name,status,description?,depends_on?,blocked_reason?,completed_at?}]}` con `status ∈ pending|in_progress|done|blocked`. File missing → 200 `{exists:false}`. JSON inválido → 502 con error del validador.
  - `PUT /features` — sobrescribe `feature_list.json` con escritura atómica (tmp + rename en el mismo dir). Body cap 512 KiB. `updated_at` se reescribe server-side.
  - `GET /harness/state` — un round-trip que devuelve `progress/current.md` + `progress/history.md` + `CHECKPOINTS.md` con cap por archivo de 256 KiB y flag `truncated`.
  - `POST /harness/init` — corre `init.sh` en el cwd. Síncrono. Default 5 min, ceiling 30. Devuelve `{exit_code, combined, truncated, duration_ms, timeout}`. Sentinels `-1` (spawn fail) / `-2` (timeout).
  - `POST /harness/scaffold` — materializa el template embebido en el repo del proyecto. Idempotente: respeta `owner` de cada archivo (managed sobrescribe siempre, seed solo en primer scaffold, project nunca). Crea/actualiza `.harness/manifest.json` con `template_version_applied`, `scaffolded_at`, `last_update_at` y `sha256` por archivo.

- **Tools MCP nuevas** (servidor `agenthub`):
  - `query_project_state(project)` — snapshot read-only que junta `feature_list.json` + harness state + flags de docs canónicos. Acepta numeric id o name.
  - `run_project_init(project, timeout_s?)` — espejo MCP de `POST /harness/init`.
  - `delegate_to_project` queda **diferida** hasta que cableamos WS routing project-side por MCP.

- **Template BettaTech embebido** (`template_version 0.1.0`) con 13 archivos compilados al binario via `embed.FS`:
  - `AGENTS.md` (template-seed) — punto de entrada del proyecto.
  - `init.sh` (template-managed, +x) — validador stack-detecting: detecta `go.mod`/`package.json`/`pyproject.toml`/`Cargo.toml` y corre los chequeos apropiados.
  - `feature_list.json` (project-owned) — vacío inicial, `version:1`.
  - `CHECKPOINTS.md` (template-managed) — criterios objetivos del reviewer (init.sh verde, scope respetado, tests, convenciones, docs, reversibilidad).
  - `progress/current.md` + `progress/history.md` (project-owned) — bitácora de sesión + append-only.
  - `docs/{architecture,conventions,verification}.md` (seed/seed/managed).
  - `.claude/agents/{leader,implementer,reviewer}.md` (template-managed) — los 3 prompts del loop.
  - `.claude/settings.json` (template-managed) — permisos y hook Stop.
  - Truco de embed: el template usa `dot_claude/` en disco (Go `embed` excluye paths que empiezan con `.`); el rename a `.claude/` ocurre en `Scaffold`.

- **Owner-manifest model** (`internal/harness/manifest.go`): tres categorías (`template-managed` | `template-seed` | `project`) con clasificación por path. `.harness/manifest.json` por proyecto guarda `template_version_applied` + `sha256` por archivo, lo cual habilita el flujo update/diff de la futura **Fase 2b**.

- **`internal/harness` — paquete neutral** con `ParseFeatureList`, `WriteFeatureListAtomic`, `ReadStateFile`/`ReadAllState`, `RunInit`, `Scaffold`, `LoadManifest`/`SaveManifest`, `TemplateFiles`, `TemplateVersion`. Server y MCP comparten esta capa para que la lectura/escritura del harness no haga loopback HTTP.

- **~25 tests nuevos** (`internal/harness/*_test.go`) cubren: parser válido + 6 errores; round-trip de WriteFeatureListAtomic + overwrite + ausencia de tmp huérfano; ReadStateFile missing/small/truncated + ReadAllState; RunInit success/exit-7/timeout/truncation; TemplateVersion/TemplateFiles/dot-rename/init.sh ejecutable; Scaffold fresh + project-owned-no-overwrite + template-seed-no-overwrite + template-managed-overwrite + idempotencia + manifest hashes.

### Changed

- **`internal/harness` extraído desde `internal/server`** — los handlers HTTP del harness se adelgazaron a wrappers finos; toda la lógica de archivos del harness vive en el paquete neutral.

### Notas

- Pendiente para 0.4.x: ripeo de OpenSpec (handlers + UI + skill + dir), WA solo main, Codex como tool de Claude (no engine principal directo), decisión A/B sobre mini-agentes.
- Pendiente para 0.5.0: extracción del componente `<ChatShell>` para unificar el chat de main-agent y el de proyectos, HUD lateral derecho con secciones SESSION · ENGINE · FEATURE_LIST · TOKENS · SUBS 5h · AGENTS·RUNTIME · STACK, sistema de notificaciones (campana + drawer + toast), BIT pixel-art como asistente de notificaciones, rebrand visual cyberpunk del handoff de Claude Design.
- La skill `.claude/skills/deploy-safe-restart` y la receta de `.claude/CLAUDE.md` siguen mencionando `mv bin/agenthub.next bin/agenthub` en lugar de `bin/promote.sh`. La actualización pidió permiso explícito durante 0.3.1 y aún no se aplicó.

---

## v0.3.1 — 2026-05-02

Quick-fix release on top of 0.3.0: closes two ciclos abiertos del release pipeline (annotated tags, real binary backup) y suma el alert mensual de budget de Anthropic via WhatsApp.

### Added

- **`GET /api/internal/usage`** — loopback-only endpoint (no JWT) que devuelve `today`, `month`, `lifetime` con `events` + `cost_usd` agregados de `usage_events`. El handler rechaza cualquier `RemoteAddr` que no sea `127.0.0.0/8` o `::1` como defensa en profundidad. Pensado para `bin/budget-alert.sh` y futuras integraciones de cron.
- **`POST /api/internal/notify`** — loopback-only, encola un mensaje de texto a `AGENTHUB_WA_NOTIFY_PHONE` reusando el worker de `wa_outbox`. Body: `{"body": "..."}`.
- **`bin/budget-alert.sh`** — script de cron que compara cost mensual contra `AGENTHUB_BUDGET_USD` (default $50) y dispara una alerta de WhatsApp al cruzar 50/75/90/100/125 %. Idempotente por mes via `data/budget-alert.state`; resetea automáticamente al cambiar de mes. Modo `--status` imprime totales + estado sin alertar.
- **`bin/promote.sh`** — script atómico de promote que respeta el orden correcto: 1) backup `bin/agenthub` → `bin/agenthub.prev` (binario aún corriendo), 2) `mv bin/agenthub.next bin/agenthub`, 3) delega en `bin/safe-restart.sh "$@"`. Acepta `--wait` que se pasa a safe-restart.
- **`internal/usage.SumSince(since)`** — agregado de totales planos (count + cost) sin grouping, separado de `AggregateBy` porque el alert path no necesita buckets.
- **Test `internal/server/usage_test.go`** — cubre `isLoopbackAddr` con la grilla completa (IPv4 con/sin puerto, IPv6, no-loopback, garbage).

### Fixed

- **`bin/release.sh`** ahora crea **annotated tags** (`git tag -a -m "Release vX.Y.Z"`). Los lightweight tags previos eran ignorados por `git push --follow-tags`, dejando los releases sin push automático del tag.
- **`bin/safe-restart.sh`** ya **NO** hace backup del binario. El backup era post-`mv`, así que `bin/agenthub.prev` quedaba idéntico al binario nuevo y no había rollback real. La responsabilidad se movió a `bin/promote.sh` que hace el `cp` ANTES del `mv`.

### Changed

- Skill `.claude/skills/deploy-safe-restart` y la receta exacta del `.claude/CLAUDE.md` deberían apuntar a `bin/promote.sh` en lugar de `mv bin/agenthub.next bin/agenthub && bin/safe-restart.sh` — pendiente de aplicar (el editor pidió permiso explícito durante el release).

---

## v0.3.0 — 2026-05-02

Major housekeeping release: release pipeline, commit enforcement, deploy
safety net, and CI baseline. Daemon code untouched in this version —
all the changes are in scripts, hooks, deploy config and workflows.

### Added

- **`bin/release.sh`** — two-phase atomic release helper. `bin/release.sh patch|minor|major|X.Y.Z` validates the working tree is clean and the 3 sources of truth (VERSION, frontend/package.json, RELEASE_NOTES.md) are aligned, then bumps all three and inserts a skeleton entry. `bin/release.sh continue` commits + tags. `bin/release.sh abort` rolls back. Closes the gap of 53 unsynced versions between VERSION (0.2.64) and the last git tag (v0.2.11).
- **Conventional Commits enforcement** — `.githooks/commit-msg` hook (activated by `bin/setup-githooks.sh`) rejects subject lines outside the spec and any AI attribution footers (`Co-Authored-By: <ai>`, `Generated with <ai tool>`, robot emoji). The hook is versioned under `.githooks/`, not `.git/hooks/`, so it travels with the repo.
- **`bin/safe-restart.sh --wait`** — optional flag that polls `/api/runs` until `pending_restart=false` (max 5 min), then verifies `/healthz` returns `ok:true` with the new binary. Exit codes: 2 if restart didn't complete, 3 if `/healthz` not OK after restart (with rollback hint printed).
- **Backup automático del binario** — `bin/safe-restart.sh` ahora copia `bin/agenthub` → `bin/agenthub.prev` antes de cualquier restart. Rollback de emergencia: `cp bin/agenthub.prev bin/agenthub && bin/safe-restart.sh`.
- **`.github/workflows/build.yml`** — three CI jobs run on push to main and PRs: backend (`go build` + `go test` with `sqlite_fts5/sqlite_json` tags), frontend (`pnpm run build`), and `versions-aligned` (validates the 3 sources of truth still match). Does not deploy.
- **`deploy/agenthub.sudoers`** — versioned reference of the NOPASSWD config installed at `/etc/sudoers.d/agenthub`. Adds entries for `systemctl daemon-reload`, `enable`, `disable` and `mount -o remount,rw|ro /etc` to the existing wildcard `start/stop/restart`. Daemon still runs as `nestor` (NOT root) — granular grants only.

### Changed

- **`.gitignore`** — `bin/` (which ignored everything in `bin/`) replaced by explicit `bin/agenthub`, `bin/agenthub.next`, `bin/agenthub.prev` so versioned scripts under `bin/` (release.sh, safe-restart.sh, setup-githooks.sh) can be tracked. Also added `.release-in-flight` (release script in-flight marker).

### Fixed

- **`bin/release.sh`** — initial implementation used `git status --porcelain` which counts untracked files; switched to `git diff-index --quiet HEAD` so unrelated scratch files in the working tree don't block a release prep.

---

## v0.2.65 — 2026-05-02

### Added

- **Vault `secret_set` y `secret_delete`** expuestos como MCP tools del servidor `agenthub`. Cierra el ciclo de credenciales: cualquier agente puede ahora guardar (`secret_set`) o rotar/eliminar tokens del vault encriptado (AES-GCM) sin pasos manuales en SQL. Idempotente — misma `key` sobrescribe el valor previo.
  - `internal/mcp/server.go` — registro de las dos tools nuevas + handlers `handleSecretSet` y `handleSecretDelete` siguiendo el patrón de `handleSecretGet` y `handleSecretList`.
  - Acepta parámetros `key` y `value` requeridos; opcionales `description` y `scope` (`global` por defecto).

### Changed

- `.claude/CLAUDE.md` — sección nueva **"Vault de credenciales"** con regla operativa: si el usuario comparte una credencial, guardarla YA en el vault; si se necesita una credencial para una tarea, **buscar en el vault primero** antes de pedírsela. Elimina el patrón anterior de re-pedir tokens en cada sesión.
- Convención de naming `UPPER_SNAKE_CASE` con prefijo por servicio (`CLOUDFLARE_API_TOKEN`, `BBVA_API_KEY`, etc.) y descripción obligatoria al guardar.

---

## v0.2.64 — 2026-04-30

### Added

- **AI Usage Tracking — Fase 2**: datos en tiempo real de usage contra la API de Anthropic y el RPC local de Codex CLI.
  - `internal/usage/realtime/` — paquete con `FetchAnthropicUsage` (OAuth token de `~/.claude/.credentials.json` + `GET api.anthropic.com/api/oauth/usage`) y `FetchCodexUsage` (spawn de `codex app-server`, JSON-RPC 2.0 con `initialize` + `account/read` + `account/rateLimits/read`).
  - Cache in-memory 60 s por provider; si el fetch falla devuelve el valor anterior + campo `error`.
  - `GET /api/usage/realtime` — responde ambos providers en paralelo (falla parcial no propaga 500); soporta query param `source=claude|codex`.

---

## v0.2.63 — 2026-05-01

### Added

- **AI Usage Tracking — Fase 1**: tracking offline de tokens consumidos por Claude Code y Codex CLI leyendo los JSONL que ya existen en disco. Sin auth tokens, sin red.
  - `internal/store/migrations/0018_usage_events.sql` — tabla `usage_events` con índices de dedup por `(source, message_id, request_id)` para Claude y `(source, session_id, ts, input_tokens, output_tokens)` para Codex.
  - `internal/usage/` — parsers `ParseClaudeJSONL` y `ParseCodexJSONL`, tabla de pricing hardcoded (con override via `data/pricing.json`), repo con `UpsertEvent`/`AggregateBy`, worker que escanea `~/.claude/projects` y `~/.codex/sessions` cada 5 minutos idempotentemente.
  - `GET /api/usage` — endpoint protegido con filtros `since`/`until`/`source`/`model` y `group_by` (`day`|`model`|`source`|`session`). Devuelve `totals` + `buckets`.

### Changed

- **Simplificación de docs del agente principal**: `AGENTS.md`, `CLAUDE.md`, `DESIGN.md` y `SPECS.md` reducidos a esqueleto con TODOs para que el usuario los complete según su stack real.
- **Retirados `README.md` y `ROADMAP.md`**: estaban duplicando información que debería vivir en los otros docs o en el código mismo.
- **Retirado `system-prompt.example.md` y `skills-lock.json`**: obsoletos, sin uso activo en el flujo del proyecto.

---

## v0.2.62 — 2026-04-30

### Removed

- **Ollama retirado del catálogo de engines**: el daemon ya no registra `OllamaEngine` ni intenta descubrir modelos contra `:11434/api/tags`. `GET /api/engines` devuelve únicamente `claude` y `codex`. La lógica de cache, `resolveOllamaEngine`, `fetchOllamaModels` e `isEmbeddingModel` fueron eliminadas de `internal/server/engines.go`. El archivo `internal/cliengine/ollama.go` fue borrado por completo. Los campos `OllamaURL`/`OllamaModel` del config siguen presentes para no romper `.env` existentes, pero no tienen efecto.

---

---

## v0.2.61 — 2026-04-30

### Added
- **Codex spawn registra el MCP `agenthub` per-run**: `cliengine/codex.go` inyecta `-c mcp_servers.agenthub.{command,args,env.*}` con todas las env vars de scope/project/session, replicando lo que ya hace claude.go con `--mcp-config`. Resultado: Codex pasa a usar exactamente las mismas tools `send_image/video/voice/...` que Claude para entregar media al chat actual, eliminando la práctica previa de inserts manuales en SQLite.

### Fixed
- **Receta de release con path de módulo erróneo**: `.claude/CLAUDE.md` y `CLAUDE.md` ya pintaban `-X github.com/snestors/agenthub/internal/buildinfo.Version` cuando el `go.mod` real es `github.com/snestors/agenteshub`. El ldflags caía en silencio y el daemon reportaba `version="dev"` aunque la build hubiera bumpeado VERSION (eso era lo que disparaba el banner `vdev ≠ ui v0.2.x · recargar ui`). El root `CLAUDE.md` ya estaba correcto; queda pendiente sincronizar `.claude/CLAUDE.md` (no se pudo editar en este pase por bloqueo de archivo sensible).

### Validated
- Smoke en `127.0.0.1:8094` reporta ahora `version="0.2.60"` y `git_commit="fc7deba"`, confirmando que el ldflags ya inyecta correctamente.

---

## v0.2.60 — 2026-04-30

### Fixed
- **Media delivery en project sessions**: `send_image`, `send_video`, `send_audio`, `send_voice` y `send_document` sin `jid` ahora funcionan cuando el agente corre en un canal `project`/`topic`/`agent`/`mini-agent`. Antes, el MCP server rechazaba con `"jid required outside the live web/wa chat"` y el archivo no llegaba al chat aunque el frontend ya soportaba la preview.
- **Mensajes intermedios del turno no se perdían más**: tras un turn de project, el daemon emitía sólo la última row de `session_messages`, así que cualquier media insertada mid-turn (por ejemplo un video que el agente acababa de renderizar) quedaba persistido pero invisible hasta el próximo refresh manual.

### Added
- Migración `0017_session_messages_media.sql`: agrega `media_type`, `media_path`, `media_caption` a `session_messages` para alcanzar paridad de render con `wa_messages` (el `MessageBubble` ya las consumía).
- `SessionsRepo.LatestMessageIDForProjectSession`: helper para capturar el baseline antes de un Run y replay de todas las rows nuevas al final del turn.
- Variables de entorno `AGENTHUB_ACTIVE_SCOPE`, `AGENTHUB_ACTIVE_PROJECT_ID`, `AGENTHUB_ACTIVE_PROJECT_SESS_ID`, `AGENTHUB_ACTIVE_TOPIC_ID`, `AGENTHUB_ACTIVE_AGENT_NAME`, `AGENTHUB_ACTIVE_SESSION_ID`: el spawn de Claude las pasa al MCP server para que pueda persistir `session_messages` con el contexto correcto.

### Changed
- `sendMedia` (MCP) ya no escribe en `wa_outbox` cuando el chat activo es project/topic/agent: persiste la entrega como `session_messages` (`role='assistant'`) con los campos de media, evitando que un video destinado al UI termine como mensaje de WhatsApp para Nestor.
- `broadcastLatestProjectMessage` queda como compat shim sobre `broadcastProjectMessagesSince`, que reproduce todas las filas con `id > baselineID` (incluyendo media intermedia).

### Validated
- `go build -tags 'sqlite_fts5 sqlite_json'` OK.
- Smoke `127.0.0.1:8094`: `/healthz` responde `ok=true`, `migrations.applied=17`.
- `pnpm run build` (frontend) OK.

---

## v0.2.59 — 2026-04-30

### Added
- **Skill `agenthub-media-delivery`**: guía reusable para que agentes entreguen fotos, videos, audios, documentos y renders al chat actual con `send_*` sin `jid` cuando exista la tool, o publiquen un link `/api/file` seguro bajo `data/uploads/shared/`.
- **Workflow de fallback para project/GridBot/Codex**: documenta cuándo no prometer preview inline, cómo publicar archivos generados y cómo construir links para tunnel/local.

### Changed
- **Skills de video alineadas**: `video-generator`, `agenthub-ui-video` y `wa-messaging` ahora apuntan al patrón correcto de media delivery y evitan recomendar inserts manuales en SQLite como camino normal.

### Validated
- **Skill válida**: `quick_validate.py .claude/skills/agenthub-media-delivery`.

---

## v0.2.58 — 2026-04-30

### Added
- **Skill `video-generator`**: guía general para crear videos desde brief/assets, elegir HyperFrames o Remotion, validar, renderizar MP4 y publicar links descargables.
- **Workflow de publicación de videos**: referencia con estructura de brief, comandos de render, hardlink a `data/uploads/shared/` y fallback de inserción en historial web.

### Validated
- **Skill válida**: `quick_validate.py .claude/skills/video-generator`.

---

## v0.2.57 — 2026-04-30

### Added
- **Skill `agenthub-ui-video`**: workflow reutilizable para crear videos explicativos de AgentHub con capturas reales autenticadas de la UI, diagramas HyperFrames, render MP4 y links descargables.
- **Script de captura CDP**: `.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs` captura páginas reales usando JWT temporal sin imprimir secretos.
- **Documentación del workflow**: referencia paso a paso en `.claude/skills/agenthub-ui-video/references/workflow.md` y registro en `AGENTS.md`.

### Validated
- **Skill válida y script probado**: `quick_validate.py .claude/skills/agenthub-ui-video` y captura real de `/` a `/tmp/agenthub-ui-video-test/chat.png`.

---

## v0.2.56 — 2026-04-30

### Added
- **Video visual de arquitectura AgentHub**: nuevo proyecto HyperFrames `videos/agenthub-architecture-tour/` con capturas reales de la UI, diagrama del flujo Web/WA → daemon → engines/MCP → SQLite/outbox y explicación de media clickeable.
- **Render compartido**: MP4 final publicado en `data/uploads/shared/agenthub-architecture-tour.mp4` para descarga desde `/api/file`.

### Validated
- **HyperFrames OK**: `npx hyperframes lint`, `npx hyperframes inspect --samples 18` y render final a MP4 de 42s.

---

## v0.2.55 — 2026-04-30

### Added
- **Media al chat actual sin `jid`**: `send_image`, `send_video`, `send_document`, `send_audio` y `send_voice` ahora pueden postearse en la conversación activa cuando el turn corre en Web/WhatsApp, sin obligar a pasar un JID explícito.
- **Contexto activo propagado al MCP**: cada run Claude genera un `mcp.json` temporal con el canal/JID/stanza activos para que las tools de media sepan si deben contestar en Web o en el chat WA en curso.

### Changed
- **Preview web de adjuntos del agente**: cuando el agente inserta media en el chat actual, la UI refresca el historial al cerrar el turn y el video queda clickable/reproducible desde la propia burbuja.
- **Broadcast WS más rico**: los eventos `message` ahora incluyen metadatos de media (`media_type`, `media_path`, `media_caption`) para que imágenes y videos entrantes/salientes no pierdan contexto live.
- **Outbox WA sin filas duplicadas**: si una tool crea una vista previa provisional (`outbox:<id>`), el worker la finaliza con el `external_id` real al despachar, en vez de insertar otra fila.

### Validated
- **Suite del release en verde**: `go test ./...`, `pnpm run build` y smoke backend aislado en `:8094`.

---

## v0.2.54 — 2026-04-30

### Added
- **WhatsApp media saliente con traza web**: los envíos `send_image`, `send_video`, `send_document`, `send_audio`, `send_voice` ahora persisten una fila `wa_messages` saliente con `external_id` real y, cuando hace falta, archivan una copia/hardlink bajo `data/uploads/wa_sent/` para que la UI pueda volver a servir el archivo.
- **Prompt operativo actualizado para media**: el template `system-prompt.example.md` y la instalación local del prompt ya explican cuándo usar `send_image`/`send_video`/`send_document`/`send_audio`/`send_voice` y recuerdan que el `path` debe ser absoluto.

### Changed
- **Burbuja de mensajes entiende media sin texto**: la UI deja de mostrar `[vacío]` para adjuntos y ahora usa placeholders específicos (`[imagen adjunta]`, `[video adjunto]`, etc.).
- **Preview inline de video**: `MessageBubble` ahora renderiza videos adjuntos en el historial web, además de imágenes.
- **SPECS.md documenta media saliente**: el bridge WA ya declara como capacidad verificable el envío de foto/video a otros chats con persistencia en historial web.

### Validated
- **Builds del release pasaron**: `go test ./...`, `pnpm run build` y smoke backend aislado en `:8094`.

---

## v0.2.53 — 2026-04-30

### Added
- **Stack HyperFrames instalada en el repo**: se agregaron `.agents/skills/` y symlinks en `.claude/skills/` para `hyperframes`, `hyperframes-cli`, `website-to-hyperframes`, `remotion-to-hyperframes` y `gsap`, dejando el skill de Miguel disponible dentro del proyecto.
- **Demos de video de AgentHub workflows**: nuevo brief en `videos/agenthub-workflows-brief.md`, proyecto HyperFrames en `videos/hyperframes-agenthub-workflows/` y proyecto Remotion en `videos/remotion-agenthub-workflows/`, ambos enfocados en explicar el flujo chat → tools → project workflow → operación segura.
- **Guía de rerender local**: `videos/README.md` documenta cómo volver a renderizar ambas piezas y dónde quedan los MP4 generados.

### Changed
- **AGENTS.md registra las skills de video**: el proyecto ya declara HyperFrames/GSAP como herramientas aprobadas para futuras demos y piezas generativas.

### Validated
- **Render probado en ambos stacks**: se validó HyperFrames con `doctor`, `lint`, `inspect` y render MP4 1080p; Remotion quedó instalado y renderizó su MP4 1080p correctamente.

---

## v0.2.52 — 2026-04-30

### Added
- **Skill `cron-admin`**: nueva guía de proyecto en `.claude/skills/cron-admin/` para crear, auditar, pausar y retirar cronjobs con guardrails. Prioriza `/etc/cron.d/agenthub-*`, usa inventario previo, logs absolutos, `flock` para evitar overlap y deja claro que este host soporta `crontab -n` pero no `crontab -T`.
- **Template + notas locales verificadas**: la skill incluye `assets/cron.d.template` y `references/local-cron-notes.md` con hechos chequeados en esta máquina (servicio `cron` activo, `cron -f -P`, auto-reload de `/etc/cron.d`, reglas de ownership/perms y límites de `run-parts`).

### Changed
- **AGENTS.md ahora registra `cron-admin`** como skill custom del proyecto para futuros agentes.

---

## v0.2.51 — 2026-04-30

### Added
- **Visor read-only de cronjobs del host**: el backend ahora expone `GET /api/system/cronjobs`, el System Manager lista `crontab -l`, `/etc/crontab`, `/etc/cron.d/*` y los directorios periódicos (`/etc/cron.hourly|daily|weekly|monthly`), y el MCP suma `list_cronjobs` como base para futuras automatizaciones/skills.

### Changed
- **UI más enfocada en sistema**: el sidebar oculta Mini-agentes, Topics y Sub-agentes; las rutas legacy redirigen a `/system`, y las notificaciones/push de `agent_run` apuntan a esa vista para no mandar a pantallas retiradas.
- **Cronjobs se muestran como lectura, no edición libre**: la nueva tabla deja explícito que la creación/administración vendrá por un skill dedicado en vez de editar cron desde la UI sin guardrails.

---

## v0.2.50 — 2026-04-29

### Fixed
- **Picker anclado al botón real**: el selector de engine/modelo ahora calcula la posición del chip que lo abrió, evita salirse de pantalla y queda por encima de drawers/overlays (`z-[90]`).

---

## v0.2.49 — 2026-04-29

### Fixed
- **Selector de modelos clickeable**: el picker de engine/modelo del status bar ahora es `fixed` y no queda recortado por el overflow horizontal de la barra.
- **Contexto más legible**: el indicador de contexto ahora es un chip con porcentaje y uso/window en desktop (`ctx 81% · 162K/200K ctx`) en vez de texto suelto.

---

## v0.2.48 — 2026-04-29

### Documentation
- **PWA/FCM/update watcher documentado**: se agregó `docs/PWA_PUSH_UPDATES.md` con arquitectura, flujos, headers de caché, operación y troubleshooting.
- **README/DESIGN/SPECS actualizados**: la PWA, FCM push y el watcher de updates quedan registrados como capacidad y decisión técnica.

---

## v0.2.47 — 2026-04-29

### Added
- **Watcher de actualización UI**: la PWA compara periódicamente el `index.html` actual contra el publicado y muestra una notificación con botón **recargar** cuando detecta nuevos bundles de Vite.

---

## v0.2.46 — 2026-04-29

### Fixed
- **Service worker sin caché de Cloudflare**: `/sw.js` y el manifest ahora salen con `no-store`, y el registro usa query de versión para evitar que Chrome reciba un service worker viejo desde el edge.

---

## v0.2.45 — 2026-04-29

### Fixed
- **Permiso push menos fastidioso**: AgentHub ahora intenta pedir el permiso de notificaciones automáticamente al abrir; si Chrome exige gesto del usuario, reintenta con el próximo toque/tecla sin obligar a abrir el drawer.
- **Botón push no queda muerto**: el botón de notificaciones ya no se deshabilita por estados `unsupported`/`denied`; muestra el motivo y permite reintentar.

---

## v0.2.44 — 2026-04-29

### Added
- **FCM Web Push con Firebase RelogTemperatura**: el frontend usa la Web app `1:100530365913:web:096380327c9c7151e265c8`, registra tokens FCM desde el drawer de notificaciones y los guarda en `/api/push/register`.
- **Envío server-side vía Firebase CLI login**: el backend persiste tokens en `push_tokens`, lee/renueva el access token del login de Firebase CLI y envía notificaciones por FCM HTTP v1 al proyecto `relogtemperatura`.
- **Service worker preparado para push**: `sw.js` ahora maneja `push` y `notificationclick`, abre la ruta enviada por la notificación y conserva el comportamiento PWA.

---

## v0.2.43 — 2026-04-29

### Fixed
- **Cancelar ejecución junto a sesión**: el botón de cancelar de Project Chat ahora aparece arriba, en la misma fila de tabs donde está el engranaje de sesión/modelo, y se quitó del cuerpo del chat para no generar saltos visuales.

---

## v0.2.42 — 2026-04-29

### Fixed
- **Botón de sesión sin salto visual**: el engranaje de configuración de sesión/modelo ahora vive al lado de `Changes` en la fila de tabs del proyecto. Se quitó la fila extra arriba del chat que generaba el salto/espacio raro.

---

## v0.2.41 — 2026-04-29

### Added
- **PWA instalable**: se agregó manifest web, registro de service worker, meta tags mobile y assets PNG 192/512/maskable para que Chrome pueda instalar AgentHub como app standalone y ocultar la barra del navegador al abrirlo instalado.

### Fixed
- **Tabs de Project Chat compactos en mobile**: Chat, Services y Changes ahora se ven como botones chicos con icono en pantallas chicas; las etiquetas completas quedan para desktop.

---

## v0.2.40 — 2026-04-29

### Fixed
- **Config de Project Chat compacta**: el control visible de sesión/modelo quedó reducido a un botón chico con icono de engranaje. Todos los selects y acciones siguen agrupados dentro del panel desplegable.

---

## v0.2.39 — 2026-04-29

### Fixed
- **Mobile menu vive en el topbar**: se eliminó el handle lateral que quedaba cortado/encima del contenido; ahora el drawer mobile se abre desde un botón del topbar con badge de notificaciones.
- **Project chat mueve sesión/modelo arriba**: la barra inferior de sesión/modelo desaparece para no pelear con el composer. En su lugar hay un botón **configurar** arriba del chat que abre sesión, nueva/borrar sesión, modelo y reasoning effort.

---

## v0.2.38 — 2026-04-29

### Fixed
- **Mobile nav ya no tapa el composer**: se reemplazó la bottom bar mobile por un sidebar ocultable con handle lateral. El drawer entra desde la izquierda, cierra con overlay/Escape/navegación, mantiene badges de notificaciones y libera todo el borde inferior del chat.

---

## v0.2.37 — 2026-04-29

### Changed
- **UI web-mobile organizada**: el shell ahora oculta el sidebar desktop en pantallas chicas y agrega navegación mobile inferior con badges de notificaciones, padding seguro y soporte `viewport-fit=cover`.
- **Layouts responsivos en pantallas clave**: chat, project chat, proyectos, mini-agentes, topics, vault, skills, sub-agentes, system, diagramas y releases reducen padding, apilan grids y habilitan scroll horizontal donde hay tablas/status bars anchas.
- **Composer y barras de estado ajustados para móvil**: botones compactos, chips truncados, controles con `overflow-x-auto`, `100dvh` y inputs a 16px para evitar zoom automático en Safari/iOS.

---

## v0.2.36 — 2026-04-29

### Changed
- **Archive de OpenSpec ahora mergea deltas en vez de pisar** (camino A fase 2): `applySpecDeltas` recibe `changeName` + `archivedAt` y, cuando una capability ya existe en `openspec/specs/`, appendea el delta con un separador `---` y un header `## Delta from change: <name> (archived YYYY-MM-DD)`. Si la capability es nueva, copia el delta tal cual. Esto permite que cada change archivado sume contenido a las specs vivas sin perder lo previo, dejando trazabilidad inline. Archivo: `internal/projects/openspec.go`.

### Added
- **`GET /api/projects/{id}/openspec/specs/{capability}`**: handler nuevo para leer la spec viva de una capability específica. Devuelve `{capability, path, content}` o 404 si no existe. Cabe junto al `GET /api/projects/{id}/openspec/specs` que ya listaba todas.
- **Tests Go para el flujo de archive y merge** (`internal/projects/openspec_test.go`): cubre los casos sin deltas (no-op), primer apply (copia verbatim), apply subsiguiente (append con header verificable), capability nueva conviviendo con existente, y archive completo (rename + merge atómico). Sin estos tests el merge era una caja negra.

### Why
- Fase 2 del Camino A según `ROADMAP.md`. Sin merge, las specs vivas del proyecto quedaban congeladas o se perdían cada vez que un change archivado tocaba la misma capability. Ahora cada delta queda registrado con su origen y fecha.

---

## v0.2.35 — 2026-04-29

### Added (docs)
- **`SPECS.md` vivo del proyecto**: 14 capabilities listadas (chat unificado, project sessions, mini-agents, topics, vault, openspec, records, diagrams, skills sync, system manager, sub-agents, slash commands, versionado, cancel cross-scope) con escenarios verificables `Dado/Cuando/Entonces` y sección "fuera de scope" explícita.
- **`DESIGN.md` vivo del proyecto**: arquitectura real con vista de bloques, tabla de paquetes `internal/*` con responsabilidades, persistencia (SQLite WAL + FTS5 + snapshots + vault), engines (Claude/Codex/Ollama), sub-agents. Decisiones congeladas fechadas con motivo y trade-off (DB única, engines como child processes, MCP server interno, vault AES-GCM, sin timeout duro, DeepSeek directo, openspec approval-gated, runtime persistence, deploy blue/green).
- **`ROADMAP.md` con Camino A**: plan incremental para cerrar el loop SDD canónico. Fase 1 (este release, docs vivos), fase 2 (`archive-merges-deltas`), fase 3 (`explore-phase`), fase 4 (`auto-mode`), fase 5 (`living-project-bridge`). Caminos paralelos: streaming Codex y fix Codex killed.
- **`openspec/specs/openspec-flow/spec.md`**: primera capability viva en `openspec/specs/`. Documenta la máquina de estados (8 estados, 4 acciones por estado), filesystem layout, API HTTP, escenarios verificables (5 casos) y limitaciones conocidas.

### Why
- Los `SPECS.md` y `DESIGN.md` vivían como stubs (3 `TODO` cada uno) desde el bootstrap; `ROADMAP.md` ni existía aunque `CLAUDE.md` lo referenciaba. Se hizo evidente al revisar el avance: la historia del proyecto vive en `RELEASE_NOTES.md`, pero no había un documento que dijera **qué somos** ni **a dónde vamos**.
- Decisión 2026-04-29 con el user: completar el modelo SDD interno de AgentHub (Camino A) en vez de delegar al SDD global o mantener el OpenSpec recortado.
- Esta es la fase 1 del Camino A: solo documentación, cero código backend o frontend tocado, sirve de base para que las fases siguientes puedan mergear deltas a specs vivas.

---

## v0.2.34 — 2026-04-29

### Fixed
- **Chat ya no se queda en "MAIN finalizando…" tras un restart del daemon**: `runtimeToGhost` (en `ChatMain.tsx` y `ProjectChat.tsx`) ahora retorna `null` para cualquier run cuyo `status` no sea `"running"`, sin mirar si hay contenido capturado. El caso típico era hacer `safe-restart` mientras un turn estaba activo: el chunk `final` del WebSocket que limpia el ghost se perdía con la conexión, el cliente al reentrar hidrataba el ghost desde el runtime persistido (`status=done` con text streamed) y el bubble quedaba mostrando "finalizando…" para siempre, con el composer bloqueado. El mensaje persistido en `session_messages` ya cubre todo el contenido visible — el ghost solo tiene sentido mientras el run está vivo.

### Changed
- **Skill `deploy-safe-restart` v2.0**: el flujo ahora cubre el release completo (bump `VERSION` + `frontend/package.json` + entrada en `RELEASE_NOTES.md`, `pnpm run build`, build backend, smoke, promote, restart) y termina con verificación cruzada de las 4 fuentes de verdad. Bug del `-X` con módulo Go equivocado (`agenthub` en vez de `agenteshub`) corregido — venía dropeando los ldflags en silencio.

---

## v0.2.33 — 2026-04-29

### Fixed
- **Project/main chat ya no se queda en "MAIN está pensando…" para siempre**: cuando un `conversation_runs` quedaba en estado `done` con texto/thinking/tools vacíos (p. ej. un turn que terminó sin emitir eventos), `runtimeToGhost` igual hidrataba un ghost vacío que bloqueaba el composer y mostraba "pensando…" eternamente. Ahora se ignora ese caso y el chat queda usable.
- **Botón cancelar ahora siempre desbloquea la UI**: si el backend responde 409 ("no turn running") porque el run ya terminó pero la UI tenía un ghost stale, el cancel local resetea pending/ghosts/refresh igual en lugar de dejar al usuario trabado.

---

## v0.2.32 — 2026-04-29

### Added
- **Persistencia backend del runtime live**: nueva tabla `conversation_runs` para guardar el snapshot de ejecución de `main` y `project` (texto parcial, thinking, tools, sub-agents, estado, tiempos, errores) y poder hidratar el chat al reentrar.
- **Hydration endpoints**: agregados `GET /api/agent/runtime` y `GET /api/projects/{id}/sessions/{sid}/runtime` para reconstruir el estado live desde backend después de navegar/recargar.

### Changed
- **Project chat ya no pierde el estado live**: al volver a entrar, la UI reconstruye el ghost bubble desde backend y muestra sub-agents/tools que siguen corriendo.
- **Actividad final persistida en project messages**: `session_messages` ahora guarda `activity` para respuestas del assistant en proyectos, así el historial conserva thinking/tools/sub-agents incluso cuando el run ya terminó.

---

## v0.2.31 — 2026-04-28

### Added
- **Skill `deploy-safe-restart`**: nuevo skill de proyecto en `.claude/skills/deploy-safe-restart/SKILL.md` que formaliza el flujo de build con `ldflags`, smoke aislado, promote a `bin/agenthub` y `bin/safe-restart.sh`.

### Changed
- **Docs de agentes alineadas**: `AGENTS.md`, `CLAUDE.md` y `.claude/CLAUDE.md` ahora apuntan al workflow correcto de deploy y dejan explícito que `safe-restart` no promueve `bin/agenthub.next` por sí solo.

---

## v0.2.30 — 2026-04-28

### Fixed
- **Restaurados sonarr/radarr/qbittorrent a la whitelist**: se habían eliminado por error en v0.2.29. Solo los servicios legacy del bridge (`whatsapp-bridge.service`, `whatsapp-bridge-watchdog.timer`) debían salir.

---

## v0.2.29 — 2026-04-28

### Fixed
- **Main chat separa sesiones Claude vs Claude+DeepSeek**: la sesión principal ya no reutiliza el mismo `session_id` genérico de `claude` cuando el modelo seleccionado es `deepseek-v4-*`. Ahora AgentHub guarda y busca una sesión main específica por modelo DeepSeek, evitando que el chat principal intente resumir una sesión Anthropic incompatible y dispare el error `content[].thinking must be passed back`.
- **Status y `/reset` alineados al nuevo scope**: `agent_status` y el slash command `/reset` entienden las sesiones main de DeepSeek para que el estado visible y el reset manual sigan apuntando al scope correcto.

### Changed
- **Servicios legacy del bridge removidos de la whitelist**: `whatsapp-bridge.service` y `whatsapp-bridge-watchdog.timer` ya no aparecen en `list_services`. WA está integrado directo en agenthub desde el converger.

---

## v0.2.28 — 2026-04-28

### Fixed
- **DeepSeek en Claude vuelve a usar `--resume` real**: la integración de modelos `deepseek-v4-*` ahora sigue la guía oficial de DeepSeek para Claude Code: `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL`, defaults `ANTHROPIC_DEFAULT_*`, `CLAUDE_CODE_SUBAGENT_MODEL` y `CLAUDE_CODE_EFFORT_LEVEL`. Con esto AgentHub preserva la sesión nativa de Claude/CLI en lugar de puentear el historial manualmente.
- **Mapeo doc-compliant de modelos DeepSeek**: `deepseek-v4-pro` se ejecuta como `deepseek-v4-pro[1m]` para alinearse con la recomendación oficial y dejar a `deepseek-v4-flash` para sub-agents/haiku-like.

### Changed
- Se retira el workaround de replay manual del historial introducido en v0.2.27 porque rompía el requisito de `claude --resume` real.

---

## v0.2.27 — 2026-04-28

### Fixed
- **Claude + DeepSeek multi-turn sin error 400 de thinking**: cuando el modelo `deepseek-v4-*` corre por el engine `claude`, AgentHub deja de usar `claude --resume` y reconstruye el historial desde `session_messages` antes de cada turno. Esto evita el `invalid_request_error` de DeepSeek (`The content[].thinking in the thinking mode must be passed back to the API`) que aparecía en el segundo mensaje o posteriores. La sesión visible en AgentHub se mantiene estable con un `session_id` local aunque el CLI no haga resume nativo contra DeepSeek.

### Note
- En modo DeepSeek vía Claude, la continuidad ahora depende del historial persistido por AgentHub (user/assistant) en vez del estado interno de resume del CLI. Es el trade-off explícito para priorizar estabilidad con el endpoint Anthropic-compatible de DeepSeek.

---

## v0.2.26 — 2026-04-28

### Added
- **Stats reales del sub-agent en SubAgentCard (B2-A)**: cada Task() que MAIN dispara ahora muestra info medida por claude — duración real, tokens consumidos, total de tool calls y breakdown por tipo (`Bash:8, Read:11, Grep:4`). Antes solo se veía el timer client-side y el preview del result. Ahora también:
  - Backend (`internal/cliengine/claude.go`): `runStreaming` trackea cada `tool_use` con `name === "Agent"`, parsea el JSONL del session post-`cmd.Wait` para extraer el `toolUseResult` block (agentId, agentType, totalDurationMs, totalTokens, totalToolUseCount, usage, toolStats), y emite eventos nuevos `kind: "subagent_stats"` con el payload en `Meta`. Se emiten ANTES del `final` event para que la SubAgentCard se decore antes de cerrar el ghost bubble.
  - `StreamEvent` ganó un campo genérico `Meta map[string]any`. Lo usa por ahora `subagent_stats`; queda flexible para otros kinds futuros.
  - Frontend: `ToolCall` ahora guarda `claudeToolUseID` (el id "call_..." original del LLM) para correlacionar; un nuevo case `subagent_stats` en `applyChunk` (streamsStore.tsx) y `applyStreamChunk` (ProjectChat.tsx) hidrata `subagentStats` en el ToolCall que matchea. `SubAgentCard` renderiza una fila extra con `stats: 4m08s · 12,450 tok · 23 tool calls` y otra con `tools: Bash:8, Read:11, …`.

### Limitación honesta (sigue valiendo)
- Esto es post-Task, no live profundo. Mientras el sub-agent corre solo se ve la SubAgentCard con status running + duración client-side. El detalle interior (qué bash corrió, qué archivo leyó) sigue siendo opaco — claude CLI no lo expone. Para visibility live profunda haría falta cambiar el modelo (B2-B), descartado por innecesario.

---

## v0.2.25 — 2026-04-28

### Added
- **Sub-agents visibles con tarjeta dedicada (B fase 1)**: cuando el MAIN delega vía `Task`, el evento `tool_use` con `name === "Agent"` ahora se renderiza como `SubAgentCard` con borde cian, ícono `GitBranch` y header `delegating · <subagent_type>`. Antes se mostraba como un tool_use crudo con args enormes (description + subagent_type + prompt) que era ilegible. Ahora muestra solo `description` + duración live + result preview cuando termina. Cierra el bug que reportaste con academia (no veías la actividad de los sub-agents en project chat).
- **Duración live en toda ToolCard**: `ToolCall` lleva `startedAt`/`finishedAt` (epoch ms). Mientras `status === "running"` un timer interno de 1s actualiza el elapsed; cuando llega el `tool_result` se congela. Se ve igual en main chat, project chat y mini-agent run-now. Archivos: `frontend/src/components/GhostBubble.tsx`, `frontend/src/lib/streamsStore.tsx`, `frontend/src/components/ProjectChat.tsx`, `frontend/src/pages/Agents.tsx`.

### Limitación honesta
- **No es timeline jerárquico todavía**: solo muestra cada Task() como tarjeta individual. NO ves qué hace el sub-agent por dentro (qué bash corre, qué archivos lee) — claude CLI no expone eventos de sub-agents al stream del padre. Para eso hace falta interceptar el JSONL del sub-agent en disco mientras corre, parsearlo y reemitir al WebSocket. Es un cambio mayor que queda en el roadmap.

---

## v0.2.24 — 2026-04-28

### Added
- **Toast `long_running_turn` con botones reales (G frontend)**: el toast de "el turn lleva >1h" ahora trae dos botones inline — `cancelar` (POST `/api/runs/cancel` con `{scope, id}` del context, cierra el toast cuando responde 200) y `continuar` (cierra el toast, próxima alerta a la otra hora). Cierra el ciclo de v0.2.21 → v0.2.23 → v0.2.24. Archivos: `frontend/src/lib/notifications.tsx`, `frontend/src/lib/api.ts`.

---

## v0.2.23 — 2026-04-28

### Added
- **Generic cross-scope cancel** (G backend): `RunTracker` ahora también guarda un `cancels` map keyed por `<scope>:<id>` y expone `RegisterCancel`/`UnregisterCancel`/`Cancel`. Cada handler que arranca un turn (main, project, agent-manual, openspec, openspec apply+verify) registra su `context.CancelFunc`; al terminar, deregister via `defer`. Nuevo endpoint protegido `POST /api/runs/cancel` con body `{scope, id}` que invoca el cancel sin importar de dónde se disparó el turn.
- **Toast `long_running_turn` con `actions`**: el watcher de 1h ahora incluye `id` y `actions: [{label:"Cancelar",kind:"cancel"}, {label:"Continuar",kind:"dismiss"}]` en el `Context` de la notification. El frontend tiene todo lo que necesita para mostrar dos botones y POST a `/api/runs/cancel` cuando el user clickea Cancelar. (UI con botones queda pendiente para el próximo turn.)

### Changed
- `watchLongRunning` toma un parámetro extra `id` para que el toast lleve el mismo `(scope, id)` que el handler usó al `RegisterCancel` — no hay traducción del lado del frontend.

---

## v0.2.22 — 2026-04-28

### Added
- **Slash commands paritaridad con bridge viejo (`/reset`, `/status`, `/engine`, `/help`)**: AgentHub heredó WhatsApp del bridge anterior pero perdió los slash commands en la migración. Ahora el daemon intercepta los comandos ANTES de mandar al engine — sin gastar tokens, respuesta inmediata. Disponibles tanto en chat web/WA (scope main) como en project chat. Comandos: `/reset` `/clear` `/forget` `/new` (limpia session_id activa, próximo mensaje arranca nuevo), `/status` (uptime, engine, RAM/CPU/temp/disk, runs activos, WA), `/engine` (muestra engine activo), `/engine claude` o `/engine codex` (cambia engine activo — sólo en main, project chat tiene engine inmutable), `/help` (lista). Implementación: `internal/server/slash_commands.go` con handler central; wireado en `conversation.go` y `projects.go`. `/claude <tarea>` y `/codex <tarea>` (one-shot) quedan para una próxima iteración.

### Fixed
- **Mensaje del assistant perdido cuando claude no emite `result` event**: `runStreaming` devolvía `claude stream produced no result` y descartaba el texto que el user ya había visto en pantalla por el WebSocket. Pasaba cuando la CLI salía con exit 0 pero sin la última línea JSONL del result (race con safe-restart, buffering interno, etc.). Fix defensivo: acumulamos los `text_delta` mientras streamean y los usamos como fallback si el result event no llega o llega vacío. Archivo: `internal/cliengine/claude.go`.

---

## v0.2.21 — 2026-04-28

### Changed
- **Sin timeout duro en runs del engine**: los `context.WithTimeout` de cada handler (main 30m, project 60m, mini-agent 30m, OpenSpec 30m+45m, scheduler 30m) eran SIGTERM mid-flight cuando el turn se pasaba. El user los pidió fuera; ahora todos usan `context.WithCancel` y los runs corren mientras hagan falta.
- **Watcher 1h cada hora**: `internal/server/long_running.go` arranca una goroutine por turn (main/project/agent-manual/openspec) que emite `Notification{Kind: "long_running_turn", Severity: "warn"}` cada hora con `elapsed_secs` y `cancel_hint`. El frontend muestra el toast — el user decide si esperar o cancelar manualmente. El cron del scheduler queda sin watcher (fire-and-forget).

### Removed
- **Cron del sistema `sonarr_notify`**: eliminado del `crontab -l` por orden del user (no funcionaba). Backup en `/tmp/crontab-backup-20260428_200630.bak`.

---

## v0.2.20 — 2026-04-28

### Removed
- **Path Ollama Cloud (`deepseek-v4-pro:cloud`)**: el wrapper `ollama launch claude --model X --` queda eliminado del catálogo de modelos del engine `claude`. Razón: latencia mucho peor que la API directa de DeepSeek vía Anthropic-compatible endpoint (v0.2.19). Helper `isOllamaCloudModel()` borrado, switch simplificado a un solo `if deepseekDirect`. Si en el futuro queremos otro modelo cloud-via-ollama, vuelve la rama. Limpieza, no funcional.

### Cleanup
- Mini-agente PoC `finanzas-monitor` (id=7) eliminado de la DB. Era validación del wrapper LLM sobre `bbva_monitor.py`. El cron del sistema sigue ejecutando el script normal.

---

## v0.2.19 — 2026-04-28

### Added
- **DeepSeek-V4-Pro/Flash directos vía API (mucho más rápido que ollama)**: el engine `claude` acepta dos modelos nuevos `deepseek-v4-pro` y `deepseek-v4-flash` (sin sufijo `:cloud`). Cuando se eligen, el daemon invoca el binario `claude` directamente con env vars `ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic` + `ANTHROPIC_API_KEY=<vault>`. DeepSeek expone un endpoint Anthropic-compatible nativo, así que claude habla protocolo Anthropic igual — pero el cerebro corre en DeepSeek y la cuenta va contra el plan del user (sk-...). Costo y latencia mucho mejores que el path `:cloud` (que pasaba por Ollama). La API key vive cifrada en el vault bajo la clave `DEEPSEEK_API_KEY`. Helpers nuevos en `internal/cliengine/claude.go`: `isDeepSeekDirectModel()` + `deepseekAPIKey()`. Catalog actualizado en `internal/server/engines.go`.

---

## v0.2.18 — 2026-04-28

### Added
- **DeepSeek-V4-Pro vía Ollama Cloud como modelo del engine claude**: el selector de modelos del engine `claude` incluye `deepseek-v4-pro:cloud` (158B params, FP8, contexto 200k). Cuando se elige, AgentHub envuelve el spawn con `ollama launch claude --model deepseek-v4-pro:cloud -- ...` para que el CLI de claude conserve toda su UX (tools, skills, system prompt, MCP) pero el cerebro corra en Ollama Cloud. Costo va contra el plan Cloud Pro del user, no contra Anthropic. Helper `isOllamaCloudModel()` detecta cualquier modelo `:cloud` futuro. Daemon prepende `dirname(ClaudeBinPath)` al PATH para que ollama encuentre `claude` aún corriendo bajo systemd con PATH limitado. Archivos: `internal/cliengine/claude.go`, `internal/server/engines.go`.

---

## v0.2.17 — 2026-04-28

### Fixed
- **Turns largos morían por timeout**: cualquier turn que delegara a sub-agents (Task tool) en paralelo se cortaba con `exit 143` (SIGTERM) cuando pasaba del límite. Project chats con 10 min, main chat / mini-agents / OpenSpec phases con 5 min — todos cortos para fan-out real (15+ sub-agents a 3-5 min cada uno + síntesis del MAIN). Síntoma observado: sesión 9 academia 2026-04-27 13:38 muerta a media respuesta. Subimos: project chat 10→60 min, main chat 5→30 min, mini-agent manual+cron 5→30 min, OpenSpec phase 5→30 min. NVIDIA chat completion (5 min) y OpenSpec full pipeline (45 min) sin cambios. Archivos: `internal/server/conversation.go`, `internal/server/projects.go`, `internal/server/agents.go`, `internal/scheduler/scheduler.go`, `internal/server/openspec.go`.

---

## v0.2.16 — 2026-04-28

### Fixed
- **Codex main agent sin contexto**: el engine Codex ahora prepende el system prompt global (`data/system-prompt.md`) + `.claude/CLAUDE.md` del proyecto al primer turno de cada sesión CLI, e inyecta un mensaje `system` al inicio del path NVIDIA. Antes Claude lo hacía vía `--append-system-prompt` pero Codex pasaba el prompt crudo, así que el agente no sabía quién era ni qué tools tenía. Resolver compartido entre engines en `internal/cliengine/system_prompt.go`.

---

## v0.2.15 — 2026-04-27

### Added
- **Codex glm-5.1 vía NVIDIA NIM**: el engine Codex acepta `glm-5.1` usando `NVIDIA_API_KEY` y `NVIDIA_BASE_URL` OpenAI-compatible sin guardar secretos en git.

---

## v0.2.14 — 2026-04-27

### Fixed
- **Notificaciones a ubicación exacta**: las notificaciones de proyecto ahora navegan a `/projects/{project_id}/sessions/{session_id}` cuando el backend envía ese contexto.
- **Confirmación antes de navegar**: el badge de secciones con notificaciones abre el drawer y el toast/drawer preguntan antes de ir; cancelar marca la notificación como leída sin borrarla.
- **Guardrail no visible**: se retiró el prefijo runtime de project chat que podía aparecer como mensaje del usuario.

---

## v0.2.13 — 2026-04-27

### Fixed
- **Breadcrumbs navegables**: `AgentHub` y `Projects` en la cabecera ahora navegan a sus rutas, evitando tener que salir y volver a entrar manualmente.
- **Project chat no alucina envíos WA**: las sesiones de proyecto reciben una instrucción runtime para no afirmar envíos por WhatsApp sin herramienta real y para entregar diagramas también en el chat; el historial visible conserva el mensaje original del usuario.

---

## v0.2.12 — 2026-04-27

### Changed
- **Router común Web/WhatsApp**: Web UI y WhatsApp ahora entran por `routeConversationInput`, comparten resolución de engine/model, sesión del main agent, ejecución y emisión de respuestas.
- **WhatsApp como adaptador**: el consumidor WA solo maneja read receipts/typing/reply target; no ejecuta el agente directamente.
- **Mensajes WA visibles en Web**: los mensajes entrantes de WhatsApp se persisten y se emiten al topic `agent`; remitentes no autorizados quedan auditados sin ejecutar runtime.

---

## v0.2.11 — 2026-04-27

### Changed
- **Sesiones sin sidebar**: Projects elimina el panel lateral de sesiones; la selección vive en el dropdown del chat y el botón `+` abre un modal para elegir engine, modelo y reasoning effort.
- **Acciones de sesión en chat**: crear y eliminar sesión quedan junto al selector de sesión en la barra inferior del project chat.

---

## v0.2.10 — 2026-04-27

### Added
- **Ocultar lista de sesiones**: el panel izquierdo de sesiones puede colapsarse a una pestaña vertical.
- **Dropdown de sesión en chat**: la barra inferior del project chat ahora incluye selector de sesión junto a modelo y reasoning effort.

---

## v0.2.9 — 2026-04-27

### Fixed
- **Cancelar runs Codex/proyecto**: Codex ahora corre en process group y cancelación mata el grupo completo; el endpoint emite evento final y la UI limpia pending/ghosts al cancelar.

---

## v0.2.8 — 2026-04-27

### Added
- **Eliminar sesiones de proyecto**: agregado endpoint `DELETE /api/projects/{id}/sessions/{sid}` con limpieza de mensajes/subagentes asociados y botón `X` en la lista de sesiones.

---

## v0.2.7 — 2026-04-27

### Fixed
- **Botón crear sesión siempre visible**: el panel de Projects apila los selects de engine/model/effort y fija el botón `+` en una columna derecha para evitar overflow en el sidebar.

---

## v0.2.6 — 2026-04-27

### Changed
- **Creación de sesiones automática**: se quitó el input manual de nombre en Projects; la UI envía `name: ""` y el backend genera el nombre de sesión automáticamente.

---

## v0.2.5 — 2026-04-27

### Added
- **Modelo y effort por project session**: las sesiones de proyecto ahora guardan `model` y `reasoning_effort`; la barra inferior del chat permite cambiar modelo y effort sin cambiar el engine fijo de la sesión.
- **Codex gpt-5.4**: agregado `gpt-5.4` al catálogo de modelos Codex junto a `gpt-5.5`.

---

## v0.2.4 — 2026-04-27

### Changed
- **Alerta backend/UI version mismatch**: el badge `VER` ahora muestra `api:<version> ui:<version>` cuando backend y bundle frontend no coinciden y agrega acción para recargar la UI.

---

## v0.2.3 — 2026-04-27

### Changed
- **Regla obligatoria para Claude y Codex**: `AGENTS.md` y `CLAUDE.md` ahora exigen versionar/releasear cada cambio aplicado y crear un `git commit` separado por cambio, sin mezclar archivos ajenos.

---

## v0.2.2 — 2026-04-27

### Fixed
- **Project sessions engine-scoped**: las sesiones de proyecto ahora fijan el `engine` al crearse; el `session_id` y `summary` quedan aislados por engine para evitar compartir contexto entre Claude, Codex u Ollama.
- **Engine immutable en sesiones existentes**: el endpoint de cambio de engine ahora rechaza cambios reales con `409`; para usar otro engine se debe crear otra sesión.
- **DB identity por engine**: `project_sessions` pasa a `UNIQUE(project_id, engine, name)` y agrega migración para permitir nombres iguales por proyecto en engines distintos.

---

## v0.2.1 — 2026-04-27

### Fixed
- **Engine selector en project sessions**: reemplaza el `StatusBar` del main agent por una barra de sesión propia. El selector de engine (`claude` / `codex` / `ollama`) vive en esa barra junto al botón cancelar y el label de transporte — sin duplicados.

---

## v0.2.0 — 2026-04-27

### Added
- **Canales aislados WA/web**: WA y web comparten el mismo brain pero tienen superficies separadas; el agente responde al canal correcto sin enviar `send_message` manualmente.
- **Safe-restart con drain**: `POST /api/runs/schedule-restart` espera que todos los turns activos terminen antes de reiniciar via systemctl. `GET /api/runs` incluye `pending_restart`.
- **`isShutdownError` unificado**: detecta `context.Canceled`, `exit status 143`, `signal: killed/terminated` para evitar mensajes de error espurios en shutdown.
- **`RunTracker.ScheduleRestart`**: callback disparado una sola vez cuando el total de runs llega a cero; thread-safe, sin deadlock desde un turn activo.
- **Sistema de versioning**: `internal/buildinfo` con `Version` + `GitCommit` inyectables vía `-ldflags`; `/healthz` expone `version` y `git_commit`; `VERSION` como fuente de verdad semver; git tags por release.
- **Fase A.2 — OpenSpec gated workflow**: persistencia `openspec_changes`, filesystem por proyecto, endpoints HTTP con gates (propose → specs → design → tasks → apply → archive), dry_run smoke, frontend Changes tab, skill `sdd-workflow`.
- **Fase A.1 — Canon de proyectos**: `.agenthub/services.yaml` con status live, inyección de `CLAUDE.md` por proyecto, tab Services en frontend, endpoint `/api/projects/{id}/services`.
- **Fase 0 — Diagramas y media**: upload autenticado, CRUD de diagramas, renderer Mermaid compartido, previews de imágenes en chat, UI `/diagrams` con Excalidraw.
- **Frontend WS RPC**: `POST /api/messages`, `/api/agent/engine`, `/api/system/services/{name}/{action}` migrados a `wsClient.rpc()` sobre WebSocket unificado.
- **Badge notificaciones** en sidebar de proyectos.

### Fixed
- Ghost "done" en UI al reconectar WebSocket.
- `external_id` de mensajes salientes guardado correctamente para quoted reply.
- Cloudflare Tunnel apuntando al puerto correcto (8093).

---

## v0.1.0 — 2026-04-25

- Scaffold inicial: daemon HTTP Go, SQLite, auth JWT + TOTP, WhatsApp (whatsmeow), MCP server, mini-agentes con cron, vault de credenciales, topics, records, engine claude/codex/ollama.
