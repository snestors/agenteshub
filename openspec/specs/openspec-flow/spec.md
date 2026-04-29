# Capability: openspec-flow

> Workflow de cambios estructurales (SDD-style) embebido en AgentHub.
> Aplica a cualquier proyecto registrado en la tabla `projects`.

Última actualización: 2026-04-29 (v0.2.35 · estado actual del flujo).

---

## Propósito

OpenSpec permite proponer, diseñar, aplicar y archivar cambios sobre un proyecto siguiendo un workflow approval-gated con artefactos persistidos como markdown en `openspec/changes/<name>/`.

Cada change vive en la tabla `openspec_changes` con un `state` que progresa por aprobación humana explícita. El daemon **nunca** auto-avanza el state.

---

## Estados del change

```
pending_proposal
    │
    │ [APPROVE] → genera proposal.md vía engine LLM
    ▼
awaiting_approval_proposal
    │
    │ [APPROVE] → genera design.md
    ▼
awaiting_approval_design
    │
    │ [APPROVE] → genera tasks.md
    ▼
awaiting_approval_tasks
    │
    │ [APPROVE] → state=applying + goroutine sdd-apply
    ▼
applying  (background, no clickeable)
    │
    │ apply termina → corre verify automáticamente
    ▼
awaiting_approval_verify
    │
    │ [APPROVE] → mueve carpeta a openspec/archive/, escribe entry en RELEASE_NOTES, state=archived
    ▼
archived
```

Estados terminales: `archived`, `rejected`.

### Acciones disponibles por estado

| Estado | Approve | Reject | Feedback |
|--------|---------|--------|----------|
| `pending_proposal` | ✅ genera proposal | ✅ → rejected | ❌ |
| `awaiting_approval_proposal` | ✅ genera design | ✅ → rejected | ✅ regenera proposal con feedback |
| `awaiting_approval_design` | ✅ genera tasks | ✅ → rejected | ✅ regenera design con feedback |
| `awaiting_approval_tasks` | ✅ → applying | ✅ → rejected | ✅ regenera tasks con feedback |
| `applying` | ❌ 409 Conflict | ❌ | ❌ |
| `awaiting_approval_verify` | ✅ archive | ✅ → rejected | ✅ |
| `archived` | ❌ 409 | ❌ | ❌ |
| `rejected` | ❌ 409 | ❌ | ❌ |

---

## Requisitos

### Filesystem layout

- Cada change tiene una carpeta en `openspec/changes/<name>/` con:
  - `proposal.md` — generado en fase 1
  - `design.md` — generado en fase 2
  - `tasks.md` — generado en fase 3
  - `specs/<capability>/spec.md` — delta opcional sobre una capability existente o nueva
- Al archivar, la carpeta se mueve a `openspec/archive/<name>/`.
- Las specs vivas del proyecto viven en `openspec/specs/<capability>/spec.md` (esta misma carpeta es un ejemplo).

### Persistencia

- Tabla `openspec_changes` (ver `migrations/0009_openspec_changes.sql`).
- Constraint único: `UNIQUE(project_id, name)`.
- Campos clave: `state`, `current_phase`, `feedback`, `archived_at`.

### API HTTP

| Método + Path | Acción |
|---------------|--------|
| `GET /api/projects/{id}/openspec/changes` | Lista changes del proyecto |
| `POST /api/projects/{id}/openspec/changes` | Crea un change (`name` slug, `description`) en `pending_proposal` |
| `GET /api/projects/{id}/openspec/changes/{name}` | Detalle del change con artefactos cargados |
| `POST /api/projects/{id}/openspec/changes/{name}/approve` | Avanza al siguiente estado o regenera si hay feedback |
| `POST /api/projects/{id}/openspec/changes/{name}/reject` | Marca `rejected` |
| `POST /api/projects/{id}/openspec/changes/{name}/feedback` | Guarda feedback y regenera la fase actual |
| `GET /api/projects/{id}/openspec/specs` | Lista specs vivas del proyecto |

### Generación de artefactos

- Cada fase invoca `generateOpenSpecArtifact` que:
  1. Carga el template (`proposeTpl`, `designTpl`, `tasksTpl`).
  2. Renderiza con vars del proyecto y change.
  3. Invoca el engine LLM (default `claude` con role `sdd-<phase>`).
  4. Persiste el resultado en `openspec/changes/<name>/<file>.md`.
- Si hay `feedback` no vacío, se incluye en el template para que el LLM lo incorpore antes de regenerar.

### Apply phase

- Disparada al aprobar `awaiting_approval_tasks`.
- Corre como goroutine background (`runOpenSpecApplyAndVerify`).
- Soporta `dry_run` opcional.
- El user NO puede aprobar/rechazar mientras `state='applying'`.
- Termina automáticamente en `awaiting_approval_verify` o en error.

### Archive phase

- Mueve `openspec/changes/<name>/` → `openspec/archive/<name>/` (función `projectfs.ArchiveChange`).
- Agrega entry en `RELEASE_NOTES.md` del proyecto vía `appendOpenSpecReleaseNote`.
- Marca `state='archived'` con `archived_at=now`.
- **Pendiente (ROADMAP fase 2)**: mergear `changes/<name>/specs/<capability>/spec.md` en `openspec/specs/<capability>/spec.md`.

### WebSocket broadcast

- Cada update relevante (state change, regeneración) emite `broadcastOpenSpecChange` con el wire actualizado para que la UI refresque sin polling.

---

## Escenarios verificables

### Caso 1 — Happy path completo

- _Dado_ un proyecto registrado y `POST /api/projects/{id}/openspec/changes` con `{name: "test-change", description: "..."}`,
- _Cuando_ el user clickea Approve cuatro veces en orden (proposal → design → tasks → apply),
- _Entonces_ el state final es `awaiting_approval_verify` con los 3 markdown generados en disco y el apply ejecutado.

### Caso 2 — Feedback regenera

- _Dado_ un change en `awaiting_approval_proposal`,
- _Cuando_ el user manda `POST .../feedback` con un texto de ajuste,
- _Entonces_ el `proposal.md` se regenera incorporando el feedback y el state queda en `awaiting_approval_proposal` (no avanza).

### Caso 3 — No double-write durante apply

- _Dado_ un change en `state='applying'`,
- _Cuando_ se intenta `POST .../approve` o `.../feedback`,
- _Entonces_ el daemon responde `409 Conflict`.

### Caso 4 — Archive escribe RELEASE_NOTES

- _Dado_ un change en `awaiting_approval_verify`,
- _Cuando_ el user aprueba para archivar,
- _Entonces_ la carpeta se mueve a `openspec/archive/<name>/` Y aparece una línea nueva en `RELEASE_NOTES.md` del proyecto.

### Caso 5 — Inconsistencia DB vs filesystem

- _Dado_ que existen archivos `design.md` o `tasks.md` en disco antes de que el state DB lo refleje (caso real observado en changes legacy),
- _Cuando_ el user aprueba la fase correspondiente,
- _Entonces_ el daemon **regenera el archivo desde cero**, pisando el contenido previo. (Comportamiento documentado, no recomendado pero esperado.)

---

## Limitaciones conocidas

1. **No hay fase `explore`** antes del proposal. El LLM va directo a generar la propuesta sin investigar el código. → ROADMAP fase 3.
2. **Archive no mergea deltas a las specs vivas**. La carpeta se archiva tal cual, pero `openspec/specs/<capability>/` no se actualiza. → ROADMAP fase 2.
3. **Sin modo `auto`**: cada fase requiere click manual. → ROADMAP fase 4.
4. **Inconsistencias DB↔filesystem no detectadas**: si alguien edita `proposal.md` a mano sin pasar por la API, el daemon no lo nota. La regeneración pisa.
5. **Los changes que crean specs nuevas** (no deltas sobre existentes) crean la carpeta `specs/<new-capability>/` dentro del change, pero no hay merge automático al archivar.

---

## Out of scope

- **Multi-tenant**: un solo user, una sola cuenta.
- **Branching** de changes (un change = un linaje único).
- **Rollback** automático: si un apply rompió algo, la reversión es manual via git.
- **Merge UI conflict resolution**: si dos changes tocan la misma capability, el orden de archive define el resultado.
