# AgentHub — Roadmap del target final

> **Status**: documento vivo. Última actualización: 2026-04-26.
> Decisiones congeladas en este archivo NO se cambian sin discusión + entrada en CHANGELOG al final.

## TL;DR

AgentHub se transforma en una **plataforma de proyectos donde el agente es socio fijo**, no un chat efímero. Cada proyecto tiene:

- Estructura canónica (`CLAUDE.md`, `SPECS.md`, `DESIGN.md`, `AGENTS.md`, `RELEASE_NOTES.md`)
- Sus propios agentes especializados (heredados de globales, customizados por stack)
- Sus propias skills (heredadas + custom)
- Workflow SDD con **gates de aprobación obligatorios** (no más decisiones autónomas)
- Servicios declarados (`services.yaml`) con status en vivo y URLs públicas
- Versionamiento automático (`RELEASE_NOTES.md` actualizado por workflow)
- Self-improvement: cada skill puede proponer mejoras tras uso real

El sistema es **3-tier**: registry compartido (skills.sh + repo personal) → globales (`~/.claude/`) → por proyecto (`<project>/.claude/`).

---

## El problema que resuelve

> "Empiezo una sesión, el agente olvida que puede deployer con CF tunnel, le re-explico, termina haciéndolo de otra forma cada vez."

Tres causas:

1. **Contexto contaminado**: el main agent hace de orquestador y ejecutor a la vez.
2. **Sin estructura canónica**: cada proyecto es un cwd con archivos sueltos, sin "manual de operaciones" que el agente lea siempre.
3. **Sin versionamiento de capacidades**: no hay forma de que el agente sepa "este proyecto se deploya con X" más allá de que vos se lo digas en el momento.

## Las decisiones clave (congeladas)

| Decisión | Detalle |
| --- | --- |
| **Orquestadores = Opus** | Main agent, project-main, sdd-propose, sdd-design, project-evaluator. Razonamiento crack importa. |
| **Verificadores = Sonnet** | Review, sdd-verify, security-check. Juicio sólido y más barato que Opus. |
| **Ejecutores = Codex GPT-5.5** | sdd-apply, frontend-specialist, backend-specialist, deploy-specialist. Trabajo sucio sin consumir cuota Anthropic. |
| **Watchers = Ollama gemma4:e2b** | log-classifier, intent-router, finance-categorizer. Tareas baratas repetitivas, costo cero. |
| **SDD con gates obligatorios** | propose → APPROVE → design → APPROVE → tasks → APPROVE → apply. El agente nunca ejecuta sin tu OK explícito. SDD libre quedó descartado por experiencia previa (agenthub legacy). |
| **OpenSpec como formato** | `<project>/openspec/{specs,changes,archive}/`. Portátil, repo-resident, agnostic de agente. Inspiración: <https://openspec.dev/>. |
| **Excalidraw para diagramas** | UI propia de AgentHub para esquemas (Excalidraw embed + Mermaid como formato intermedio + skill `diagram-architect`). |

---

## Arquitectura de 3 niveles

```
┌────────────────────────────────────────────────────────────────────────────┐
│  NIVEL 3 — REGISTRY COMPARTIDO  (sincronizable)                            │
│  ─────────────────────────────────────────────────────────────────────     │
│  📦 skills.sh (público)              📦 snestors/skills (tuyo, custom)     │
│      → caché en data/skill-registry/  con sync periódico                   │
└────────────────────────────────────────────────────────────────────────────┘
                              ▲
                              │ pull on-demand
                              ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  NIVEL 1 — GLOBAL  (~/.claude/)                                            │
│  ─────────────────────────────────────────────────────────────────────     │
│  agents/                             skills/                               │
│  ├─ agent-builder.md   (opus)        ├─ topic-routing/                     │
│  ├─ project-evaluator.md (opus)      ├─ vault-secrets/                     │
│  ├─ ui-designer.md (opus)            ├─ ollama-engine/                     │
│  ├─ frontend-specialist.md (codex)   ├─ wa-messaging/                      │
│  ├─ backend-specialist.md (codex)    ├─ openspec-workflow/                 │
│  ├─ deploy-specialist.md (codex)     ├─ diagram-architect/                 │
│  └─ verifier.md (sonnet)             └─ ...                                │
│                                                                            │
│  project-templates/                                                        │
│  ├─ astro-frontend.yaml                                                    │
│  ├─ go-backend-gin.yaml                                                    │
│  ├─ python-fastapi.yaml                                                    │
│  ├─ docker-compose-stack.yaml                                              │
│  └─ cf-pages-deploy.yaml                                                   │
└────────────────────────────────────────────────────────────────────────────┘
                              ▲
                              │ inherit + override
                              ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  NIVEL 2 — POR PROYECTO  (ej. /home/nestor/academia/)                      │
│  ─────────────────────────────────────────────────────────────────────     │
│  .claude/                                                                  │
│  ├─ CLAUDE.md            ← system prompt del proyecto (identidad/stack)    │
│  ├─ agents/                                                                │
│  │  ├─ astro-blog.md     ← custom: solo este proyecto                      │
│  │  └─ project-main.md   ← orquestador del proyecto (opus)                 │
│  └─ skills/                                                                │
│     ├─ academia-domain.md                                                  │
│     └─ supabase-schema.md                                                  │
│                                                                            │
│  AGENTS.md               ← qué agente hace qué en este proyecto            │
│  SPECS.md                ← qué hace el producto                            │
│  DESIGN.md               ← decisiones de arquitectura                      │
│  RELEASE_NOTES.md        ← changelog auto-versionado                       │
│                                                                            │
│  openspec/                                                                 │
│  ├─ specs/               ← capabilities formales                           │
│  ├─ changes/<name>/      ← propuestas con gates                            │
│  │  ├─ proposal.md  (gate)                                                 │
│  │  ├─ design.md    (gate)                                                 │
│  │  ├─ tasks.md     (gate)                                                 │
│  │  └─ specs/       ← deltas                                               │
│  └─ archive/             ← changes ya aplicados (historial)                │
│                                                                            │
│  .agenthub/                                                                │
│  └─ services.yaml        ← qué corre + URLs públicas                       │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Workflows clave

### Crear proyecto nuevo (3 clicks + gates)

```
USER: "Quiero crear blog Astro + MDX, deploy a CF Pages"
                ▼
project-evaluator (opus) escanea path, infiere stack, propone:
  · 4 agentes a crear (astro-blog, ui-designer, deploy-specialist, project-main)
  · 6 skills a importar (astro-5, tailwind-v4, mdx-content, cf-pages-deploy, …)
  · SPECS.md skeleton + AGENTS.md + services.yaml inicial
                ▼
GATE EN UI: vos aprobás cada agente / skill por separado
                ▼
Materialización: archivos físicos creados, registro en DB
                ▼
PROYECTO LISTO
```

### Ejecutar una task con SDD-light (gates obligatorios)

```
USER: "Agregá login con Google"
                ▼
project-main (opus, orquestador)
                ▼
[propose]   sdd-propose (opus)        → proposal.md   ➜ GATE
[design]    sdd-design (opus)         → design.md     ➜ GATE
[tasks]     sdd-tasks (sonnet)        → tasks.md      ➜ GATE
[apply]     delegado a especialistas:
              · frontend-specialist (codex)
              · backend-specialist  (codex)
            paralelo cuando aplica
                ▼
[verify]    verifier (sonnet) → corre tests, revisa diff, check de specs
                ▼
[archive]   change → openspec/archive/, RELEASE_NOTES.md updated
```

### Auto-mejora de skills (propose-and-approve)

```
agent termina tarea → detecta patrón mejorable
                ▼
genera improvement_note.md (parche markdown)
                ▼
notif en UI: "💡 1 mejora propuesta para skill <name>"
                ▼
USER revisa diff → aprueba/rechaza
                ▼
si aprueba: version bump (1.2.0 → 1.2.1) + push a snestors/skills
            otros proyectos que usan la skill ven update disponible
```

---

## Servicios por proyecto (`services.yaml`)

Cada proyecto declara qué corre a nivel sistema, en formato YAML:

```yaml
# <project_path>/.agenthub/services.yaml
services:
  - kind: systemd
    unit: academia-api.service
    description: Backend Go (Gin)
    health_url: http://localhost:8081/health
    public_url: https://api.academia.kyn3d.com   # via CF tunnel

  - kind: docker
    container: academia-postgres
    description: Postgres 16
    health_cmd: pg_isready

  - kind: cloudflare-tunnel
    hostname: academia.kyn3d.com
    target: http://localhost:3000
    description: Frontend SSR

  - kind: process
    command: pnpm run dev
    cwd: ./frontend
    description: Vite dev server (manual launch)
```

El dashboard del proyecto consulta status en vivo, muestra URLs clickeables, expone acciones (start/stop/restart) según `kind`. El `deploy-specialist` actualiza este file cuando crea servicios nuevos.

---

## UI de diagramas (Excalidraw)

**Capacidad core de AgentHub**, no parche por caso. Inspirado en investigación de Codex.

### Stack

- **Excalidraw embebido** como componente React (`@excalidraw/excalidraw`) — UI tipo pizarra hand-drawn
- **Mermaid como formato intermedio** — la IA genera Mermaid (mejor que coordenadas crudas)
- **`@excalidraw/mermaid-to-excalidraw`** convierte Mermaid → Excalidraw editable
- **Skill `diagram-architect`** — entiende intención, elige tipo (flowchart, C4, ERD, secuencia, mind-map), genera Mermaid limpio

### Pantalla `/diagrams`

```
┌────────────────────────────────────────────────────────────────┐
│  Sidebar        │  Canvas Excalidraw (grande, editable)        │
│  ─ Chat         │  ┌──────────────────────────────────────┐    │
│  ─ Proyectos    │  │                                      │    │
│  ─ Mini-agentes │  │   diagrama editable visualmente      │    │
│  ─ Diagramas ⭐ │  │                                      │    │
│  ─ ...          │  └──────────────────────────────────────┘    │
│                 │  ─ Prompt panel ───────────────────────────  │
│                 │  > "diagrama de arquitectura de AgentHub"    │
│                 │  [Generar] [Mejorar] [Aplicar al canvas]     │
│                 │  [Ver Mermaid] [Exportar PNG/SVG] [Guardar]  │
└────────────────────────────────────────────────────────────────┘
```

### Persistencia

Tabla nueva `diagrams`:

```sql
CREATE TABLE diagrams (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  prompt TEXT,
  mermaid_source TEXT,
  excalidraw_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

Endpoints:

```
GET    /api/diagrams
POST   /api/diagrams
GET    /api/diagrams/{id}
PUT    /api/diagrams/{id}
DELETE /api/diagrams/{id}
POST   /api/diagrams/generate    # llama al diagram-architect
```

Bonus: **mermaid blocks dentro del chat** se renderizan inline (mismo `mermaid` package). Capacidad "renderizar esquemas" es transversal — chat + UI dedicada usan la misma infra.

### Skill `diagram-architect`

Ubicación: `~/.claude/skills/diagram-architect/SKILL.md`.

Responsabilidades:

- Entender intención del user (es flowchart? secuencia? arquitectura? ERD? mind-map?)
- Elegir tipo apropiado
- Generar Mermaid limpio y validado
- Evitar texto excesivo (legibilidad antes que detalle)
- Producir versiones simple/detallada según el contexto
- Output: Mermaid string + tipo + título sugerido

---

## Skills externas a importar

Investigación del user (validar antes de importar; algunas son URLs incompletas):

| Skill | Origen | Para qué |
| --- | --- | --- |
| Frontend Design Skill | Anthropic (claude-...) | Patrones de UI/UX para Claude |
| Interface Design | <https://skills.sh/dammyjay93/interfac...> | Diseño de interfaces |
| Vercel React Best Practices | <https://skills.sh/vercel-labs/agent-s...> | React + Next + Vercel |
| Brainstorming | <https://github.com/obra/superpowers> | Generar ideas estructuradas |
| Systematic Debugging | <https://github.com/obra/superpowers> | Debug con método |
| Changelog Generator | <https://skills.sh/composiohq/awesome-...> | Auto-generar CHANGELOG |
| API Design Principles | <https://github.com/wshobson/agents> | Diseño de APIs REST/GraphQL |
| Error Handling Patterns | <https://github.com/wshobson/agents/...> | Patterns Go/TS para errores |
| PostgreSQL Skill | <https://github.com/wshobson/agents/...> | Schema, queries, perf |
| Prompt Engineering Patterns | <https://github.com/wshobson/agents> | Prompting técnicas |

Política: **antes de importar**, el `agent-builder` o vos mismo revisan la skill. No importamos a ciegas — algunas pueden tener prompts muy opinativos que distorsionan.

---

## Roadmap por fases

| Fase | Qué entrega | Prioridad |
| --- | --- | --- |
| **0** | **AgentHub renderiza esquemas e imágenes**: mermaid en chat, endpoint `GET /api/uploads/{id}` con auth JWT, image preview en MessageBubble cuando `media_type=image`, ruta `/diagrams` con Excalidraw embebido, tabla `diagrams`, endpoints CRUD + generate, skill `diagram-architect` skeleton | **Hacer ya** |
| **A.1** | Project canon + `services.yaml` + carga de `CLAUDE.md` como append-system-prompt cuando el cwd es del proyecto. Reader Go del manifest + endpoint live status + tab UI con cards | Después |
| **A.2** | OpenSpec adoptado: `<project>/openspec/{specs,changes,archive}/` + MCP tools `openspec_propose / openspec_apply / openspec_archive` + UI con gates de aprobación explícitos | |
| **B.1** | `agent-builder` global + 4 especialistas iniciales (`project-evaluator`, `ui-designer`, `frontend-specialist`, `backend-specialist`, `deploy-specialist`, `verifier`) en `~/.claude/agents/`. Engines según roles (opus/sonnet/codex) | |
| **B.2** | Patrón Claude/Codex/Ollama cableado como defaults por rol del agente | |
| **C** | **Skills registry sync**: pull de skills.sh + `snestors/skills` con caché local. Importar el set inicial (la lista de arriba). | |
| **D** | **Project templates galería**: pool de plantillas pre-fabricadas (`astro-frontend`, `go-backend-gin`, etc.) clonables con un click. | |
| **E** | **Dashboard del proyecto** con tabs: Resumen / Specs / Changes / Tasks / Agents / Services / Releases / Diagrams / Chat | |
| **F** | **Auto-mejora de skills** (propose-and-approve), versionamiento semver de skills, push automático al repo personal, sync notification entre proyectos que usan la misma skill | |
| **G** | **Per-phase model assignment** automático (Gentleman style). | |

---

## Tareas operativas pendientes (fuera de fases)

- **WhatsApp cutover**: encender `AGENTHUB_WA_ENABLED=true`, parear con QR, apagar `whatsapp-bridge.service` viejo.
- **Migración del bridge viejo**: importar `wa_messages` históricos del bridge legacy a agenthub. **NO ejecutar todavía** — esperar a que el cutover esté estable.
- **Auto-discovery de proyectos**: escanear `/home/nestor` y registrar proyectos automáticamente. **Ejecutar como paso final antes del cutover** de WA.

---

## Inspiraciones (no copiar todo, robar lo bueno)

- **OpenSpec.dev** — formato de specs/changes/archive, slash commands.
- **Gentleman gentle-ai** (<https://github.com/Gentleman-Programming/gentle-ai>) — ecosystem configurator, persistent memory (Engram, ya usamos), SDD workflow, per-phase model assignment, persona teaching-first, agent-builder pattern (`PRD-AGENT-BUILDER.md` como referencia).
- **superpowers** (<https://github.com/obra/superpowers>) — skills atómicas, Brainstorming + Systematic Debugging.
- **wshobson/agents** — colección de agentes especializados (API design, error handling, postgres, etc.).
- **skills.sh** — registry público de skills.

---

## Changelog del roadmap

| Fecha | Cambio |
| --- | --- |
| 2026-04-26 | Fase A.1 implementada: canon por proyecto, `.agenthub/services.yaml`, status live de servicios, tab Services y append-system-prompt con `CLAUDE.md` del proyecto por cwd. |
| 2026-04-26 | Fase 0 implementada: render Mermaid inline en chat, previews de imágenes protegidas por auth, endpoint de uploads/file, CRUD + generate de diagramas, pantalla `/diagrams` con Excalidraw, y skill `diagram-architect` skeleton. |
| 2026-04-26 | Documento creado. Decisiones congeladas: 3 niveles, engines por rol (opus/sonnet/codex/ollama), SDD con gates, OpenSpec format, Excalidraw para diagramas. Fase 0 = render de mermaid + UI Excalidraw. |
