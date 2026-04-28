---
name: deploy-safe-restart
description: >
  Flujo obligatorio de build, smoke, promote y safe-restart para cambios backend de AgentHub.
  Trigger: cuando un agente toque Go/backend, deba desplegar a prod, o necesite reiniciar agenthub sin cortar runs activos.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

# Deploy safe-restart

## When to Use

- Cambios en `cmd/agenthub` o cualquier `internal/**/*.go`
- Necesidad de publicar un binario nuevo a `agenthub.service`
- Reinicios que deben esperar a que terminen runs activos

## Critical Patterns

- **Nunca** sobrescribas prod directo al compilar: build siempre a `bin/agenthub.next`
- **Siempre** estampá el binario con `VERSION` + `git rev-parse --short HEAD` usando `-ldflags`
- **Siempre** hacé smoke aislado antes de promover
- **Nunca** corras el smoke con `AGENTHUB_WA_ENABLED=true`
- **Promote correcto**: `mv bin/agenthub.next bin/agenthub`
- **Restart correcto**: `bin/safe-restart.sh` (no `systemctl restart` directo salvo emergencia)
- Si `/api/runs` devuelve `pending_restart:true`, el restart ya quedó en cola
- Si `/healthz` sigue mostrando la versión vieja, puede ser normal mientras haya runs activos

## Commands

```bash
cd /home/nestor/agenthub

# 1) Build a binario next con versión estampada
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X github.com/snestors/agenthub/internal/buildinfo.Version=$(cat VERSION) -X github.com/snestors/agenthub/internal/buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenthub.next ./cmd/agenthub

# 2) Verificar el binario generado
./bin/agenthub.next version

# 3) Smoke aislado
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
echo "$HEALTH" | grep -q '"ok":true' || { echo "SMOKE FAIL"; kill $SMOKE_PID; rm -f "$SMOKE_DB"*; exit 1; }

kill $SMOKE_PID 2>/dev/null
wait $SMOKE_PID 2>/dev/null
rm -f "$SMOKE_DB"*

# 4) Promote + safe restart
mv bin/agenthub.next bin/agenthub
./bin/agenthub version
bin/safe-restart.sh

# 5) Verificación del estado
curl -s http://127.0.0.1:8093/api/runs
curl -s http://127.0.0.1:8093/healthz
```

## What to Check

- `./bin/agenthub version` debe mostrar la nueva versión/commit
- `/api/runs`:
  - `pending_restart:true` = restart en cola
  - `total:0` y `pending_restart:false` = ya no hay espera
- `/healthz`:
  - antes del restart puede seguir mostrando la versión vieja
  - después del restart debe reflejar la nueva

## Anti-Patterns

- `go build -o bin/agenthub ./cmd/agenthub`
- `sudo systemctl restart agenthub` sin smoke
- asumir que `safe-restart` ya promovió el binario
- asumir que `/healthz` cambia de inmediato aunque haya runs activos
