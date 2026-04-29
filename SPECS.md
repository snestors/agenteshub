# AgentHub — Specs

Path: `/home/nestor/agenthub`
Última actualización: 2026-04-29 (v0.2.35)

> Este documento describe **qué es AgentHub** y **qué tiene que hacer** desde la mirada de capacidades. Es la fuente de verdad para validar que el sistema cumple con sus promesas. Las decisiones técnicas viven en `DESIGN.md`; la planificación viva en `ROADMAP.md`.

---

## Producto

AgentHub es un **daemon personal de agente** self-hosted que corre en hardware del usuario. Un solo cerebro convergido sobre dos superficies:

- **Web** (UI propia en `:8093`)
- **WhatsApp** (bridge nativo con `whatsmeow`)

Stack: Go + React/Vite + SQLite. Sin dependencia de servicios cloud para operar (más allá de los engines LLM elegidos).

### Principios

- **Local-first**: persistencia en SQLite, secretos en vault AES-GCM, sin telemetría.
- **Una sola sesión por contexto**: el agente preserva memoria entre web y WhatsApp.
- **Multi-engine**: Claude CLI, Codex CLI, Ollama — intercambiables por sesión.
- **Approval-gated workflows**: cambios estructurales (OpenSpec) requieren confirmación humana antes de tocar código.

---

## Capacidades

Cada capability lista aquí debe tener al menos un escenario verificable. Los escenarios se redactan en formato `Dado / Cuando / Entonces` y deben ser ejecutables manualmente o automáticamente.

### 1. Chat unificado web + WhatsApp

Una conversación principal que comparte sesión entre los dos canales.

**Requisitos**:
- El daemon expone un chat principal en la UI web bajo `/`.
- Si `AGENTHUB_WA_ENABLED=true` y el QR fue scanneado, los mensajes entrantes desde el número configurado en `AGENTHUB_WA_NOTIFY_PHONE` van al mismo cerebro.
- La respuesta del agente se entrega al canal por el que llegó el mensaje original.
- El historial es navegable desde la UI con búsqueda full-text (FTS5).

**Escenarios verificables**:
- _Dado_ que envío "hola" desde el chat web, _cuando_ el agente responde, _entonces_ esa respuesta NO se duplica en WhatsApp.
- _Dado_ que envío "hola" desde WhatsApp, _cuando_ el agente responde, _entonces_ la respuesta llega por WhatsApp Y queda visible en el chat web sin pasos extra del agente.
- _Dado_ que mando una nota de voz por WhatsApp, _cuando_ se procesa, _entonces_ se transcribe localmente con whisper antes de pasar al engine.

### 2. Project sessions (chats aislados por proyecto)

Cada proyecto registrado tiene chats independientes con su propio engine y modelo.

**Requisitos**:
- Crear/listar/borrar proyectos vía `POST/GET/DELETE /api/projects`.
- Cada proyecto puede tener N sesiones nombradas (`main`, `feature-x`, etc).
- Cada sesión puede cambiar engine y modelo en runtime.
- El historial de la sesión persiste el `activity` (thinking, tool calls, sub-agents) además del texto.
- Al recargar el chat se reconstruye el estado live desde backend (`GET /api/projects/{id}/sessions/{sid}/runtime`).

**Escenarios verificables**:
- _Dado_ un project chat en mitad de un turn, _cuando_ refresco la página, _entonces_ el ghost bubble se reconstruye con sub-agents y tools en curso.
- _Dado_ un `safe-restart` del daemon mientras un turn está corriendo, _cuando_ vuelvo al chat, _entonces_ el composer queda usable y no se traba en "finalizando…" para siempre.

### 3. Mini-agents (cron / manual / event)

Agentes especializados con prompt propio, schedule opcional y resultado persistido.

**Requisitos**:
- Crear vía `POST /api/agents` o MCP `agent_create`.
- Schedules en cron (`POST /api/agents/{id}/schedules`).
- Ejecución manual one-shot (`POST /api/agents/{id}/run`).
- Cada run queda persistido con tokens, duración, tools usados y resultado en `agent_runs`.
- El target de notificación es configurable: `wa:<jid>`, `main-agent`, `topic:<X>`, `none`.

**Escenarios verificables**:
- _Dado_ un agente con `enabled=1` y cron `*/5 * * * *`, _cuando_ pasan 5 minutos, _entonces_ se dispara una run y aparece en `GET /api/agents/{id}/runs`.
- _Dado_ un agente con engine `ollama`, _cuando_ corre, _entonces_ no consume tokens del plan Anthropic.

### 4. Topics (contextos persistentes del agente principal)

Conversaciones del agente principal organizadas por tema con estado vivo.

**Requisitos**:
- Cada topic tiene un `session_id` propio del CLI elegido.
- El estado del topic incluye `headline`, `active_issues`, `recent_decisions`, `pending`, `next_action_hint`.
- El topic `general` existe siempre como catch-all (`is_default=1`).
- `run_in_topic(topic, message)` delega ejecución a la sesión del topic preservando el resume.

**Escenarios verificables**:
- _Dado_ que actualizo el state del topic `salud` con `update_topic_state`, _cuando_ leo `read_topic_state` después, _entonces_ los campos parchados aparecen y los no tocados quedan intactos.

### 5. Vault (credenciales encriptadas)

Almacén de secretos cifrado con AES-GCM usando `AGENTHUB_SECRET_KEY`.

**Requisitos**:
- `secret_list` devuelve solo metadatos (key + descripción), nunca el plaintext.
- `secret_get(key)` devuelve plaintext solo dentro del turno; el agente NO debe persistirlo.
- `secret_create` cifra antes de escribir a disco.
- Si `AGENTHUB_SECRET_KEY` cambia, los secrets viejos quedan ilegibles (no hay rotación automática hoy).

**Escenarios verificables**:
- _Dado_ un secret guardado, _cuando_ leo el archivo de DB con sqlite3, _entonces_ NO veo el plaintext en ninguna columna.

### 6. OpenSpec — workflow approval-gated de cambios

Mecanismo SDD interno para proponer/diseñar/aplicar cambios sobre proyectos.

**Requisitos**: ver `openspec/specs/openspec-flow/spec.md` (capability dedicada).

### 7. Records (datos espontáneos schema-on-read)

Almacén libre de JSON bajo un topic name para datos ad-hoc del agente (peso, calorías, gastos).

**Requisitos**:
- `record(topic, data)` persiste un JSON arbitrario.
- `query_records(topic, since?, limit?)` recupera filtrado por timestamp.
- `list_topics_records` enumera topics con count.

### 8. Diagrams (Mermaid + Excalidraw)

Renderizador embebido de diagramas y whiteboard.

**Requisitos**:
- CRUD de diagramas vía `/api/diagrams`.
- Generación asistida vía `POST /api/diagrams/generate` (engine LLM).

### 9. Skills sync

Sincronización del registry de skills personales con el daemon.

**Requisitos**:
- `GET /api/skills` lista las skills detectadas.
- `POST /api/skills/sync` rescanea filesystem.
- Las improvements propuestas viven en `skill_improvements`.

### 10. System manager

Operación de servicios systemd whitelisted desde el agente.

**Requisitos**:
- Solo servicios listados en `.agenthub/services.yaml` (o equivalente) son operables.
- `service_action(name, action)` requiere sudoers NOPASSWD configurado para los servicios whitelisted.
- `get_system_stats` expone CPU/RAM/disk/temp/uptime.

### 11. Sub-agent visibility

Cuando MAIN delega vía `Task()`, la UI muestra el sub-agent con stats reales.

**Requisitos**:
- Cada sub-agent run queda persistido en `subagent_runs` con prompt, result, status, tokens.
- La UI renderiza una `SubAgentCard` con duración live + stats post-finalización (tokens, tool calls breakdown).
- El listado completo está en `GET /api/subagents`.

### 12. Slash commands

Comandos interceptados antes de pasar al engine (sin gastar tokens).

**Disponibles**: `/reset` `/clear` `/forget` `/new` `/status` `/engine` `/engine claude|codex` `/help`.

**Escenarios verificables**:
- _Dado_ que escribo `/status`, _cuando_ envío, _entonces_ la respuesta llega instantánea sin pasar por el LLM.
- _Dado_ que escribo `/engine codex` en main, _cuando_ envío, _entonces_ los siguientes turns usan Codex.

### 13. Versionado y release visibility

La versión del binario y del frontend son verificables en runtime.

**Requisitos**:
- `GET /healthz` devuelve `{ok, version, git_commit}`.
- La UI muestra un badge en el sidebar; va en naranja si frontend y backend están desincronizados.
- Cada release bumpea `VERSION` + `frontend/package.json` + entrada en `RELEASE_NOTES.md`.

### 14. Cancelación cross-scope de runs

Cualquier turn activo puede cancelarse desde la UI sin importar dónde se disparó.

**Requisitos**:
- `POST /api/runs/cancel` con `{scope, id}` invoca el `context.CancelFunc` registrado.
- El watcher de turns >1h emite `Notification{kind:"long_running_turn"}` con botones `Cancelar`/`Continuar` accionables.

---

## Fuera de scope

Lo que AgentHub **no** intenta ser:

- **No es un orquestador multi-tenant**: un solo usuario, una sola identidad, sin RBAC.
- **No es un media server**: pelis/series viven en HDD bajo control de Sonarr/Radarr/Emby — AgentHub solo orquesta.
- **No reemplaza al CLI de Claude/Codex**: los engines son procesos hijos; AgentHub es un facade conversacional.
- **No es un sistema de backup**: las JSONL de Claude se snapshot, pero la responsabilidad de respaldo de DB es del usuario.
- **No es un reemplazo de Notion/Drive/Calendar**: integra vía MCP, no los reimplementa.
- **No expone públicamente sin tunnel**: el daemon escucha en LAN; la exposición pública es responsabilidad de Cloudflare Tunnel u otro reverse proxy.

---

## Convenciones de validación

- Cada capability nueva requiere al menos un escenario verificable agregado a esta sección antes de mergear.
- Los escenarios deben poder ejecutarse localmente o quedar como TODO explícito.
- Cuando un change archive de OpenSpec toque una capability, el delta del spec se mergea acá (ver `ROADMAP.md` fase 2).
