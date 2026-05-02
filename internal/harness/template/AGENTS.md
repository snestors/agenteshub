# Agents — BettaTech harness

> Punto de entrada para los agentes que trabajan en este proyecto.
> Si entrás como Claude Code y nunca leíste este archivo, leelo entero antes de
> hacer cambios.

## Loop de trabajo

Toda feature pasa por tres roles bien delimitados. Ningún rol salta a otro:

| Rol | Archivo | Mandato |
|---|---|---|
| **Leader** | `.claude/agents/leader.md` | Orquesta. Elige la próxima feature de `feature_list.json`. **NO edita código**. Delega al implementer. |
| **Implementer** | `.claude/agents/implementer.md` | Implementa **una sola** feature por turno. Escribe código + tests. Corre `init.sh` antes de cerrar. |
| **Reviewer** | `.claude/agents/reviewer.md` | Audita contra `CHECKPOINTS.md`. Devuelve `APPROVED` o `CHANGES_REQUESTED` con razones. NO edita. |

El loop solo avanza cuando el reviewer aprueba. Si el reviewer pide cambios, vuelve al implementer.

## Archivos clave

| Archivo | Para qué sirve |
|---|---|
| `feature_list.json` | Roadmap viviente. El leader lee de acá. |
| `CHECKPOINTS.md` | Criterios objetivos del reviewer. Inmutable durante una feature. |
| `progress/current.md` | Bitácora de la sesión activa. El implementer escribe acá mientras trabaja. |
| `progress/history.md` | Append-only. Cuando una sesión cierra, su `current.md` se concatena acá. |
| `init.sh` | Validador local. Tests + build + smoke según el stack del repo. |
| `docs/architecture.md` | Mapa de cómo está organizado el código. |
| `docs/conventions.md` | Reglas de estilo del proyecto (commits, tests, formato). |
| `docs/verification.md` | Cómo validar trabajo manualmente cuando `init.sh` no es suficiente. |

## Reglas duras

- **Nunca** editar `feature_list.json` desde el implementer. El leader es el único que cambia el estado de las features.
- **Nunca** mergear código sin que `init.sh` salga con `exit_code=0`.
- **Nunca** mergear código sin `APPROVED` del reviewer.
- **Una feature por turno.** Si el implementer descubre que tiene que tocar algo fuera del scope, lo registra en `feature_list.json` (via leader) y lo deja para otra iteración.

## Versionado del harness

Este harness fue scaffoldado desde el template embebido en `agenthub` y guarda su versión en `.harness/manifest.json`. Cuando agenthub publique una versión nueva del template, podrás ver el diff y decidir si actualizar.

Archivos `template-managed` (init.sh, agents/*.md, CHECKPOINTS.md, settings.json, docs/verification.md) se sobrescriben en updates si los aceptás. Archivos `project-owned` (feature_list.json, progress/*.md) y `template-seed` (AGENTS.md, docs/architecture.md, docs/conventions.md) NO se tocan automáticamente.
