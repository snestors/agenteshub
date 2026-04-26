# AgentHub — Agente del proyecto

Sos el agente de **AgentHub**. Path: `/home/nestor/agenthub`.

## Stack

- Backend Go, daemon HTTP en `:8093`.
- Frontend React + Vite en `frontend/`.
- Persistencia SQLite en `data/agenthub.db`.
- Auth por JWT cookie httpOnly.

## Servicios

- Ver `.agenthub/services.yaml` para servicios systemd/docker/tunnel del proyecto.
- Dato crítico: AgentHub se opera como `agenthub.service`; después de build se reinicia con `systemctl restart agenthub`.
- Cuando haya URL pública, asumí que puede estar detrás de Cloudflare Tunnel y revisá el manifest antes de improvisar deploy.

## Convenciones del proyecto

- Leer `ROADMAP.md` antes de tocar fases del roadmap.
- No cambiar decisiones congeladas sin discusión y entrada en changelog.
- Commits conventional, chicos por capa, sin Co-Author ni atribución AI.
- Backend: validar con `go test ./...` y build con tags `sqlite_fts5 sqlite_json` cuando corresponda.
- Frontend: validar con `pnpm run build` desde `frontend/`.

## Decisiones recientes

- Ver `DESIGN.md` para arquitectura, `RELEASE_NOTES.md` para historial y `ROADMAP.md` para fases congeladas.
