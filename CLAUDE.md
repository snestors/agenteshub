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
- **Commitear inmediatamente después de cada cambio aplicado** — no acumular. Un fix = un commit. Un feature = un commit. Conventional commits, sin Co-Author ni atribución AI.
- Backend: validar con `go test ./...` y build con tags `sqlite_fts5 sqlite_json` cuando corresponda.
- Frontend: validar con `pnpm run build` desde `frontend/`.

## Deploy workflow — blue/green smoke (OBLIGATORIO)

**Cualquier cambio backend que modifique código Go DEBE pasar por este flujo antes de tocar prod.** El daemon `agenthub.service` corre en `:8093`; no lo reinicies sin smoke.

### Receta exacta

```bash
cd /home/nestor/agenthub

# 1. Compilar a binario "next" (NO sobrescribe el de prod)
go build -tags 'sqlite_fts5 sqlite_json' -o bin/agenthub.next ./cmd/agenthub

# 2. Smoke en :8094 con DB temp + WA disabled (aislado del prod)
SMOKE_DB="/tmp/agenthub-smoke-$$.db"
AGENTHUB_HTTP_ADDR="127.0.0.1:8094" \
AGENTHUB_DB_PATH="$SMOKE_DB" \
AGENTHUB_WA_ENABLED=false \
AGENTHUB_DEV=true \
./bin/agenthub.next serve > /tmp/agenthub-smoke.log 2>&1 &
SMOKE_PID=$!

# 3. Esperar /healthz OK (max 30s)
for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:8094/healthz > /dev/null 2>&1; then break; fi
  sleep 1
done

# 4. Validar respuesta
HEALTH=$(curl -s http://127.0.0.1:8094/healthz)
echo "$HEALTH" | grep -q '"ok":true' || { echo "SMOKE FAIL"; kill $SMOKE_PID; rm -f $SMOKE_DB*; exit 1; }

# 5. Matar smoke + limpiar
kill $SMOKE_PID 2>/dev/null
wait $SMOKE_PID 2>/dev/null
rm -f $SMOKE_DB*

# 6. Promover a prod con safe-restart (espera turns activos antes de reiniciar)
mv bin/agenthub.next bin/agenthub
bin/safe-restart.sh
```

### Reglas no negociables

- Si el smoke falla en CUALQUIER paso → NO tocar `bin/agenthub` ni reiniciar prod. Avisar al usuario y dejar `/tmp/agenthub-smoke.log` para inspección.
- NO arrancar dos instancias contra la misma DB — SQLite WAL no lo permite.
- NO arrancar el smoke con `AGENTHUB_WA_ENABLED=true` — mata la sesión de WhatsApp del prod.
- Frontend (cambios sólo en `frontend/`) NO requiere smoke: `pnpm run build` desde `frontend/` y listo.

## Decisiones recientes

- Ver `DESIGN.md` para arquitectura, `RELEASE_NOTES.md` para historial y `ROADMAP.md` para fases congeladas.
