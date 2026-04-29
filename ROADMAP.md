# AgentHub — Roadmap

Path: `/home/nestor/agenthub`
Última actualización: 2026-04-29 (v0.2.35)

> Este documento traza el camino de evolución del proyecto. Las capacidades vigentes están en `SPECS.md`; las decisiones técnicas en `DESIGN.md`; el historial fechado en `RELEASE_NOTES.md`.

---

## Estado actual

**Versión**: 0.2.35
**Última release**: 2026-04-29 — `bootstrap-living-specs`

Lo que está consolidado y estable:
- Chat unificado web+WA, project sessions, mini-agents, topics, vault, records, slash commands.
- Persistencia del runtime live + hidratación al reentrar.
- Sub-agent visibility con stats reales.
- Cancelación cross-scope con toast accionable.
- Deploy blue/green smoke + safe-restart formalizado como skill.
- DeepSeek-V4-Pro/Flash directo vía endpoint Anthropic-compatible.

Lo que existe pero está incompleto:
- **OpenSpec**: workflow approval-gated funciona, pero le faltan piezas para ser SDD canónico (ver Camino A abajo).
- **Codex engine**: corre, pero bufferea stdout en vez de streamear. Errores `signal: killed` mal clasificados.

---

## Camino A — Cerrar el loop SDD canónico

Decidido el 2026-04-29 con el user: en vez de mantener una versión recortada de OpenSpec o delegar al SDD global de Claude, **completar el modelo SDD interno** para que AgentHub sea un motor de workflow legítimo.

Estrategia: **incremental**. Cada fase entrega valor solo, se archiva, se valida en prod antes de pasar a la siguiente.

### Fase 1 — `bootstrap-living-specs` (en curso, 2026-04-29)

**Objetivo**: dejar los documentos del proyecto como fuentes de verdad vivas.

- ✅ Completar `SPECS.md` con las capacidades reales y escenarios verificables.
- ✅ Completar `DESIGN.md` con arquitectura y decisiones congeladas fechadas.
- ✅ Crear este `ROADMAP.md`.
- ✅ Crear `openspec/specs/openspec-flow/spec.md` documentando el flujo OpenSpec actual.

Es 100% docs. Cero código backend o frontend tocado.

### Fase 2 — `archive-merges-deltas`

**Objetivo**: que cuando un change OpenSpec se archive, sus deltas se mergeen a `openspec/specs/<capability>/spec.md` para que la spec del proyecto evolucione automáticamente.

Tareas:
- Cambiar `projectfs.ArchiveChange` para copiar/mergear `changes/<name>/specs/<capability>/spec.md` a `openspec/specs/<capability>/spec.md`.
- Agregar handler `GET /api/projects/{id}/openspec/specs/{capability}` para leer la spec viva.
- UI: botón "ver spec viva" desde cada capability detectada.
- Test de integración: archivar un change con delta y verificar el merge.

Riesgo: conflictos de merge entre changes que tocan la misma capability. Estrategia inicial: **append** secciones del delta al final con header del change name. La sofisticación viene después si hace falta.

### Fase 3 — `explore-phase`

**Objetivo**: agregar fase `explore` antes del `proposal` para que el LLM investigue código y compare approaches con tradeoffs ANTES de proponer.

Tareas:
- Nueva fila en máquina de estados: `pending_explore → awaiting_approval_explore → pending_proposal`.
- Template `exploreTpl` que pide: contexto del codebase, opciones consideradas, recomendación con tradeoff.
- Persistencia en `openspec/changes/<name>/explore.md`.
- Migration de DB para extender el CHECK del `state` enum.
- UI: vista del explore antes del proposal.
- Backwards-compat: changes existentes en `pending_proposal` o más adelante NO se ven afectados.

### Fase 4 — `auto-mode`

**Objetivo**: modo automático que avance solo entre fases de **planificación** (explore → proposal → design → tasks), pero siempre se detenga ANTES del apply para confirmación humana.

Tareas:
- Campo nuevo en `openspec_changes`: `mode TEXT NOT NULL DEFAULT 'interactive' CHECK(mode IN ('interactive','auto'))`.
- Cuando `mode='auto'`, después de generar un artefacto el daemon enqueue automáticamente la próxima generación.
- UI: toggle en la creación del change.
- El gate antes de `applying` queda **siempre** manual, sin importar el modo. Esto preserva la safety.

### Fase 5 — `living-project-bridge`

**Objetivo**: que `SPECS.md` y `DESIGN.md` del root del proyecto se generen/actualicen desde `openspec/specs/` y archivos auxiliares cuando un change se archiva.

Tareas:
- Generador que lee `openspec/specs/*/spec.md` y produce el `SPECS.md` agregado.
- Trigger en `ArchiveChange` que regenera el SPECS.md del proyecto si la capability cambió.
- UI: link "ver SPECS.md" desde el panel de openspec.

---

## Caminos paralelos (no bloqueados por A)

### Codex engine — paridad con Claude

Dos changes ya creados y planeados:
- `stream-codex-engine-events` — leer JSONL line-by-line, mapear a `StreamEvent`, soportar `OnEvent`.
- `fix-codex-engine-killed-tasks` — clasificar errores `signal: killed` con stderr tail y persistir status final.

Estos pueden avanzar en paralelo al Camino A — viven en otro paquete (`internal/cliengine/codex.go`) y no chocan con OpenSpec.

### Frontend / UX — pendientes menores

- Vista mejor de `agent_runs` con filtros por status y agente.
- Persist de toasts dismissed entre sesiones.
- Diagrams Excalidraw collaborative (multi-cursor LAN).

Sin prioridad asignada. Se irán pickeando cuando aporten valor concreto.

---

## Reglas del roadmap

- Una fase = un change OpenSpec = un commit (idealmente un PR).
- Antes de pasar a la siguiente fase, la actual tiene que estar **archivada y verificada en prod** (al menos 24h sin rollback).
- Cualquier cambio en este roadmap requiere actualización en este archivo + entrada en `RELEASE_NOTES.md`.
- Si una fase se descarta, queda registrada acá con `~~tachado~~` y motivo.
