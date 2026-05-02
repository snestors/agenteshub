---
name: deploy-safe-restart
description: >
  Flujo obligatorio de release de AgentHub: alineación de versiones,
  build de frontend y backend, smoke aislado, promote y safe-restart.
  Trigger: cuando un agente toque Go/backend, modifique frontend/, deba
  publicar un release, o necesite reiniciar agenthub sin cortar runs activos.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.1"
---

# Deploy safe-restart

## When to Use

- Cambios en `cmd/agenthub` o cualquier `internal/**/*.go`
- Cambios en `frontend/**`
- Bump de versión / release nuevo
- Reinicios que deben esperar a runs activos

## Sources of Truth — must stay in sync

Tres archivos definen "la versión activa". El flujo aborta si quedan desalineados:

| Archivo | Qué representa |
|---|---|
| `VERSION` (raíz) | Versión canónica que se estampa en el binario via `-ldflags` |
| `frontend/package.json` (`version`) | Versión del bundle React que sirve el daemon |
| `RELEASE_NOTES.md` (entrada más reciente) | Documentación del release |

Y al final: `./bin/agenthub version` debe coincidir con esas 3.

## Critical Patterns

- **Nunca** sobrescribas prod directo al compilar: build siempre a `bin/agenthub.next`
- **Siempre** estampá el binario con `VERSION` + `git rev-parse --short HEAD` usando `-ldflags`
- **Siempre** corré smoke aislado antes de promover el binario
- **Nunca** corras smoke con `AGENTHUB_WA_ENABLED=true`
- **Promote correcto**: `bin/promote.sh` (hace backup `bin/agenthub` → `bin/agenthub.prev` ANTES del `mv`, después delega en `bin/safe-restart.sh`). Acepta `--wait` que se pasa a safe-restart. NO hagas `mv` directo salvo que sepas que estás salteando el backup
- **Restart correcto**: `bin/safe-restart.sh` (no `systemctl restart` salvo emergencia). NO hace backup — esa es responsabilidad de `bin/promote.sh`
- **Versionado**: bump `VERSION` y `frontend/package.json` a la misma semver, y agregá entrada en `RELEASE_NOTES.md` en el MISMO commit

## Flow

```bash
cd /home/nestor/agenthub

# ─── 0) Pre-flight: estado de versiones ─────────────────────────────────────
NEW_VERSION="$(cat VERSION)"
FRONTEND_VERSION="$(grep -E '"version"' frontend/package.json | head -1 | sed -E 's/.*"version": "([^"]+)".*/\1/')"
BIN_VERSION="$(./bin/agenthub version 2>/dev/null | awk '{print $2}')"
LATEST_NOTE="$(grep -E '^## v[0-9]' RELEASE_NOTES.md | head -1 | sed -E 's/^## v([0-9.]+).*/\1/')"

echo "VERSION:               $NEW_VERSION"
echo "frontend/package.json: $FRONTEND_VERSION"
echo "bin/agenthub:          $BIN_VERSION"
echo "RELEASE_NOTES top:     $LATEST_NOTE"

# Si VERSION y frontend/package.json no coinciden, alinealos al valor de VERSION
# (que es la fuente canónica). Lo mismo con RELEASE_NOTES top.
# Si todo está alineado pero el binario quedó atrás, sólo hay que rebuildear.

# ─── 1) Bump version (si hay cambios sin versionar) ─────────────────────────
# bin/release.sh patch|minor|major   # bump las 3 fuentes y agrega skeleton
# <editar RELEASE_NOTES.md a mano para llenar la entrada>
# bin/release.sh continue            # commit + annotated tag

# ─── 2) Build frontend (si frontend/ cambió o se bumpeó la versión) ─────────
cd frontend && pnpm run build && cd ..

# ─── 3) Build backend a binario "next" ──────────────────────────────────────
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X github.com/snestors/agenteshub/internal/buildinfo.Version=$NEW_VERSION -X github.com/snestors/agenteshub/internal/buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenthub.next ./cmd/agenthub

./bin/agenthub.next version   # debe imprimir $NEW_VERSION

# ─── 4) Smoke aislado en :8094 ──────────────────────────────────────────────
SMOKE_DB="/tmp/agenthub-smoke-$$.db"
AGENTHUB_HTTP_ADDR="127.0.0.1:8094" \
AGENTHUB_DB_PATH="$SMOKE_DB" \
AGENTHUB_WA_ENABLED=false \
AGENTHUB_DEV=true \
./bin/agenthub.next serve > /tmp/agenthub-smoke.log 2>&1 &
SMOKE_PID=$!

for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:8094/healthz > /dev/null 2>&1; then break; fi
  sleep 1
done

HEALTH=$(curl -s http://127.0.0.1:8094/healthz)
echo "$HEALTH" | grep -q '"ok":true' || {
  echo "SMOKE FAIL — log en /tmp/agenthub-smoke.log"
  kill $SMOKE_PID 2>/dev/null
  rm -f "$SMOKE_DB"*
  exit 1
}

kill $SMOKE_PID 2>/dev/null
wait $SMOKE_PID 2>/dev/null
rm -f "$SMOKE_DB"*

# ─── 5) Promote (backup + mv + safe restart en un paso) ─────────────────────
bin/promote.sh           # promote sin esperar; --wait para polear /healthz post-restart
./bin/agenthub version

# ─── 6) Verificación post-deploy ────────────────────────────────────────────
curl -s http://127.0.0.1:8093/api/runs
curl -s http://127.0.0.1:8093/healthz

# Las 4 versiones deben coincidir
echo "VERSION:               $(cat VERSION)"
echo "frontend/package.json: $(grep -E '"version"' frontend/package.json | head -1)"
echo "bin/agenthub:          $(./bin/agenthub version)"
echo "RELEASE_NOTES top:     $(grep -E '^## v[0-9]' RELEASE_NOTES.md | head -1)"
```

## Pre-flight Decision Tree

| Estado | Acción |
|---|---|
| Las 3 versiones (VERSION, frontend/package.json, RELEASE_NOTES top) iguales y binario igual | Nada que hacer; release ya está en prod |
| Las 3 fuentes iguales pero binario atrás | Saltar al paso 2 (rebuild + smoke + promote) |
| `frontend/package.json` o `RELEASE_NOTES` desalineados respecto a `VERSION` | Alinearlos a `VERSION` ANTES del build (commit chico aparte) |
| Hay cambios sin versionar | `bin/release.sh patch\|minor\|major` → editar RELEASE_NOTES → `bin/release.sh continue`, después correr el flujo de build/smoke/promote |

## What to Check

- `./bin/agenthub version` muestra la nueva versión y commit corto correctos
- `bin/agenthub.prev` quedó con la versión ANTERIOR (no la nueva): si es igual a `bin/agenthub`, `promote.sh` se salteó o se llamó después de un `mv` manual — perdiste el rollback
- `/api/runs`:
  - `pending_restart:true` = restart en cola (esperando runs activos)
  - `total:0` y `pending_restart:false` = restart completado
- `/healthz`:
  - antes del restart puede mostrar la versión vieja
  - después del restart debe mostrar la nueva
- Al final, las 4 versiones (VERSION, frontend pkg, binario, RELEASE_NOTES top) son idénticas

## Anti-Patterns

- Bumpear `VERSION` sin tocar `frontend/package.json` (causa desfase silencioso)
- Bumpear `frontend/package.json` sin entrada en `RELEASE_NOTES.md`
- `go build -o bin/agenthub ./cmd/agenthub` (overwrite directo de prod)
- `sudo systemctl restart agenthub` sin smoke (corta runs activos)
- Hacer `mv bin/agenthub.next bin/agenthub` y después `bin/safe-restart.sh` por separado: para cuando safe-restart corre, `bin/agenthub.prev` no se actualizó y queda igual al binario nuevo. Usá `bin/promote.sh` que hace el `cp` ANTES del `mv`
- Asumir que `safe-restart.sh` promueve `bin/agenthub.next` o que hace backup
- Asumir que `/healthz` cambia inmediato cuando hay runs activos
- Saltar el smoke "porque es un cambio chico"
