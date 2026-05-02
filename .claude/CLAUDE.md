# AgentHub — Agente del proyecto

Eres el agente de **AgentHub**. Path: `/home/nestor/agenthub`.

## Stack

- Backend Go, daemon HTTP en `:8093`.
- Frontend React + Vite en `frontend/`.
- Persistencia SQLite en `data/agenthub.db`.
- Auth por JWT cookie httpOnly.

## Servicios

- Ver `.agenthub/services.yaml` para servicios systemd/docker/tunnel del proyecto.
- Dato crítico: AgentHub se opera como `agenthub.service`; después de build se reinicia con `systemctl restart agenthub`.
- Cuando haya URL pública, asume que puede estar detrás de Cloudflare Tunnel y revisa el manifest antes de improvisar deploy.

## Convenciones del proyecto

- Leer `ROADMAP.md` antes de tocar fases del roadmap.
- No cambiar decisiones congeladas sin discusión y entrada en changelog.
- Commits conventional, chicos por capa, sin Co-Author ni atribución AI.
- Backend: validar con `go test ./...` y build con tags `sqlite_fts5 sqlite_json` cuando corresponda.
- Frontend: validar con `pnpm run build` desde `frontend/`.
- Para cambios backend/deploy usar la skill `.claude/skills/deploy-safe-restart/SKILL.md`.

## Vault de credenciales (LEE ESTO ANTES DE PEDIR UN TOKEN)

**Regla de oro:** si el usuario te comparte una API key, token, password o cualquier credencial, **guárdala en el vault encriptado YA**. Si vas a usar una credencial, **busca en el vault primero** antes de pedírsela. El usuario no quiere repetir credenciales entre sesiones.

Tools MCP disponibles (servidor `agenthub`):

- `secret_list` — devuelve `[{key, scope, description, updated_at, expires_at?}]`. Sin plaintext. **Llamala al inicio de cualquier tarea que necesite credenciales** para descubrir qué hay disponible.
- `secret_get(key)` — devuelve plaintext. Úsala dentro del mismo turno y olvídala. Nunca la eches al chat.
- `secret_set(key, value, description?, scope?)` — guarda o rota. AES-GCM con `AGENTHUB_SECRET_KEY`. Idempotente: el mismo `key` sobrescribe.
- `secret_delete(key)` — borra permanentemente.

**Convenciones de naming:** UPPER_SNAKE_CASE, prefijo por servicio. Ejemplos:

- `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_ZONE_KYN3D`
- `BBVA_API_KEY`, `DEEPSEEK_API_KEY`
- `SUDO_PASSWORD` (scope: `system`)

**Descripción obligatoria al guardar.** Sin description, futuras sesiones no saben qué scope/uso tiene el secreto. Ejemplo bueno: `"CF token con scope DNS Edit + Tunnel para zona kyn3d.com"`.

**Cuándo NO guardar:** valores efímeros (request IDs, session tokens de un turno), datos públicos.

## Deploy workflow — blue/green smoke (OBLIGATORIO)

**Cualquier cambio backend que modifique código Go DEBE pasar por este flujo antes de tocar prod.** El daemon `agenthub.service` corre en `:8093`; no lo reinicies sin smoke.

### Receta exacta

```bash
cd /home/nestor/agenthub

# 1. Compilar a binario "next" (NO sobrescribe el de prod)
go build -tags 'sqlite_fts5 sqlite_json' \
  -ldflags "-X github.com/snestors/agenteshub/internal/buildinfo.Version=$(cat VERSION) -X github.com/snestors/agenteshub/internal/buildinfo.GitCommit=$(git rev-parse --short HEAD)" \
  -o bin/agenthub.next ./cmd/agenthub

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

# 6. Promover a prod con backup + safe-restart en un solo paso
bin/promote.sh           # backup → mv → safe-restart; --wait para polear /healthz post-restart
```

### Reglas no negociables

- Si el smoke falla en CUALQUIER paso → NO tocar `bin/agenthub` ni reiniciar prod. Avisar al usuario y dejar `/tmp/agenthub-smoke.log` para inspección.
- NO arrancar dos instancias contra la misma DB — SQLite WAL no lo permite.
- NO arrancar el smoke con `AGENTHUB_WA_ENABLED=true` — mata la sesión de WhatsApp del prod.
- **Promote SIEMPRE con `bin/promote.sh`**. Hace `cp bin/agenthub bin/agenthub.prev` ANTES del `mv bin/agenthub.next bin/agenthub`. Si hacés `mv` directo y después `bin/safe-restart.sh`, el backup queda igual al binario nuevo y perdés el rollback.
- `safe-restart.sh` NO promueve `bin/agenthub.next` y NO hace backup. Es solo restart graceful.
- Frontend (cambios sólo en `frontend/`) NO requiere smoke: `pnpm run build` desde `frontend/` y listo.

## Decisiones recientes

- Ver `DESIGN.md` para arquitectura, `RELEASE_NOTES.md` para historial y `ROADMAP.md` para fases congeladas.
