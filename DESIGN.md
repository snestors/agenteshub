# AgentHub — Diseño

Path: `/home/nestor/agenthub`

## Arquitectura

- Daemon Go en `cmd/agenthub` expone HTTP/WS y sirve `frontend/dist`.
- Backend modular en `internal/`: server, store, cliengine, sysman, projects, auth, ws.
- Frontend React/Vite en `frontend/src` consume endpoints `/api/*` con cookie auth.

## Decisiones

- Ver `ROADMAP.md` para decisiones congeladas: 3-tier agent context, SDD con gates, OpenSpec, Excalidraw, per-role engine defaults.
- Fase A.1: el prompt del proyecto vive en `.claude/CLAUDE.md` y se concatena al prompt global cuando el cwd es un proyecto registrado.

## Deploy / Operación

- Build backend: `go build -tags 'sqlite_fts5 sqlite_json' -o bin/agenthub ./cmd/agenthub`.
- Build frontend: `cd frontend && pnpm run build`.
- Restart: `systemctl restart agenthub`.
- Status de servicios: `.agenthub/services.yaml` + `/api/projects/{id}/services`.
