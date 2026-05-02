# Conventions

> Reglas de estilo y proceso de este proyecto. **template-seed** —
> agenthub genera este archivo con valores por defecto razonables; ajustalos
> al proyecto y los updates del template no los van a pisar.

## Commits

- Formato: Conventional Commits (`type(scope): summary`).
- `type` ∈ `feat | fix | refactor | docs | test | chore | perf | style`.
- **Sin attribution AI** (`Co-Authored-By: <ai>`, "Generated with X", emoji robot).
- Subject ≤ 70 chars. Body opcional explica el porqué, no el qué.

## Branches

_(nombrar el patrón de branches del proyecto: `main`, `feat/*`, `fix/*`, etc.)_

## Tests

- Cada feature trae al menos un test que falla sin el cambio.
- Tests describen conducta, no implementación.
- No `t.Skip` / `xit` / `pytest.mark.skip` sin issue declarado.

## Estilo

_(formatter del lenguaje: gofmt, prettier, ruff, rustfmt — anotar cuál y si
corre en pre-commit)_

## Reviews

- Reviewer agente sigue `CHECKPOINTS.md` literalmente.
- Reviewer humano puede agregar opinión, pero los checkpoints son inmutables
  durante la feature.
