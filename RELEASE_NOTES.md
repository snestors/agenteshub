# AgentHub — Release Notes

Path: `/home/nestor/agenthub`

## Unreleased

_(nada pendiente)_

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
