# AgentHub — Agentes

Path: `/home/nestor/agenthub`

## Agente principal

- Opera como socio fijo del proyecto AgentHub.
- Debe leer `ROADMAP.md` antes de implementar fases.
- Debe aplicar las mismas reglas de release/versionado que Claude: todo cambio aplicado requiere bump en `VERSION` y entrada en `RELEASE_NOTES.md` antes de cerrar.
- Debe crear un `git commit` inmediatamente después de cada cambio aplicado. Un fix = un commit. Un feature = un commit. Usar Conventional Commits, sin Co-Author ni atribución AI.

## Especialistas del proyecto

- TODO: listar agentes custom en `.claude/agents/` cuando se materialicen las fases B/C.

## Skills del proyecto

- `go-testing`: usar cuando se escriban o modifiquen tests Go.
- `deploy-safe-restart`: flujo obligatorio para backend Go — build con ldflags, smoke aislado, promote a `bin/agenthub` y `bin/safe-restart.sh`.
- `cron-admin`: crear y administrar cronjobs del host con guardrails, priorizando `/etc/cron.d/agenthub-*`, inventario previo y validación conservadora.
- `branch-pr` / `issue-creation`: usar para flujos GitHub cuando aplique.
- `sdd-workflow`: reglas OpenSpec/SDD con gates obligatorios para proposal/design/tasks/apply/verify/archive.
- TODO: listar más skills custom en `.claude/skills/`.

## Release y commits obligatorios

- Cada cambio aplicado debe actualizar `VERSION` con semver apropiado.
- Cada cambio aplicado debe agregar entrada fechada en `RELEASE_NOTES.md`.
- Cada cambio aplicado debe terminar en un `git commit` propio antes de empezar otro cambio.
- No mezclar cambios no relacionados ni archivos modificados por el usuario en el commit.
