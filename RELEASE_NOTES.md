# AgentHub — Release Notes

Path: `/home/nestor/agenthub`

## Unreleased

_(nada pendiente)_

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
