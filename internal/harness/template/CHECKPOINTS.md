# Checkpoints — Reviewer

Criterios objetivos que el reviewer aplica antes de devolver `APPROVED`. No son
opiniones — son chequeos verificables. Si todos pasan, la feature está cerrada.
Si alguno falla, devolver `CHANGES_REQUESTED` con la lista exacta de fallas.

> **Inmutable durante una feature.** Si una feature requiere cambiar
> CHECKPOINTS.md, eso es una feature aparte (revisar este archivo).

## 1) Validación local

- [ ] `./init.sh` salió con `exit_code=0` en este turno (NO en uno anterior).
- [ ] Si init.sh corrió tests, todos pasan. No "skipped temporales".
- [ ] El output de init.sh no tiene `WARN`/`fail` filtrados.

## 2) Scope

- [ ] La feature tocada es exactamente la que reportó el implementer en `progress/current.md`.
- [ ] No hay cambios en archivos no relacionados al scope. Si los hay, deben estar declarados como features separadas en `feature_list.json` y aprobados antes.
- [ ] No hay TODOs nuevos del estilo "vuelvo después" sin entrada en `feature_list.json`.

## 3) Tests

- [ ] La feature trae al menos un test que falla sin el cambio (regression-anchor).
- [ ] Los tests describen la conducta, no la implementación interna.
- [ ] No hay tests `.skip` / `t.Skip` / `xit` nuevos sin issue declarado.

## 4) Convenciones

- [ ] El commit message sigue Conventional Commits.
- [ ] No hay attribution AI en commits (`Co-Authored-By: <ai>`, `Generated with`, etc.).
- [ ] Si el proyecto define `docs/conventions.md`, se respetan.

## 5) Documentación

- [ ] Si la feature cambia un contrato (API, schema, env var), la doc relevante (`docs/architecture.md` o el archivo del módulo) se actualiza en el mismo cambio.
- [ ] `progress/current.md` describe qué se hizo, no qué se planeaba hacer.

## 6) Reversibilidad

- [ ] El cambio es revertible con un `git revert` limpio.
- [ ] No hay migraciones destructivas sin un `down`.
- [ ] No hay cambios en datos de producción sin confirmación explícita.

## Procedimiento del reviewer

1. Leer `progress/current.md` para entender el scope declarado.
2. Leer el diff completo. Si toca algo fuera del scope → fallo automático.
3. Correr (mentalmente o de hecho) `./init.sh` y leer su output.
4. Aplicar cada checkpoint de arriba.
5. Si todos pasan → escribir `APPROVED` en el commit/PR review.
6. Si alguno falla → escribir `CHANGES_REQUESTED` con lista numerada de fallas. Cita archivo:línea cuando aplique.

Nunca aprobar parcialmente. Nunca aprobar "con TODOs". El loop solo avanza con todos los checkpoints en verde.
