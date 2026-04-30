---
name: cron-admin
description: >
  Crear, auditar, pausar, reanudar y retirar cron jobs del host de forma segura,
  priorizando `/etc/cron.d/agenthub-*` y evitando tocar crontabs ajenos sin necesidad.
  Trigger: cuando el user pida "creá un cron", "programá X", "agendá", "cada N minutos",
  "administrá cronjobs", "pausá este cron", "eliminá este cron" o "mostrame los cronjobs".
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

# Cron admin

## When to Use

- Crear un job recurrente del sistema.
- Auditar cronjobs existentes antes de agregar otro.
- Pausar, reanudar o eliminar un cron administrado por AgentHub.
- Migrar automatizaciones livianas desde ideas ad-hoc a cron del host.

## Critical Patterns

- **Inventario primero**: listar cronjobs antes de crear o modificar uno. Preferí `list_cronjobs` o `GET /api/system/cronjobs`; si no están, usá `crontab -l`, `/etc/crontab` y `/etc/cron.d/*`.
- **Por default usar `/etc/cron.d/agenthub-<slug>`**. No mezclar jobs gestionados por AgentHub con `crontab -e` del usuario salvo pedido explícito.
- **Nombres simples, sin puntos** para cron.d: `agenthub-<slug>`. Este host corre `cron -f -P`, así que conviene evitar nombres raros o sufijos tipo `.disabled`.
- **No tocar `/var/spool/cron/*` directamente**. Para crontabs de usuario se usa `crontab file`; para sistema se escribe archivo nuevo en `/etc/cron.d/`.
- **Un job = un archivo** en `/etc/cron.d/agenthub-<slug>`. Facilita diff, pausa, rollback y borrado limpio.
- **Absoluto todo**: comandos, intérpretes, paths, logs y `cwd`. En cron no dependas del entorno interactivo.
- **Sin restart de cron**: en este host Debian/Ubuntu, `cron` monitorea `/etc/crontab` y `/etc/cron.d` automáticamente cada minuto.
- **Permisos correctos**:
  - `/etc/cron.d/agenthub-*` → `root:root`, modo `0644`, **no** ejecutable.
  - scripts bajo `/etc/cron.hourly|daily|weekly|monthly` → root-owned y ejecutables.
- **Validación conservadora**:
  - este host soporta `crontab -n file` para dry-run de crontabs de usuario;
  - **no** soporta `crontab -T`, así que `/etc/cron.d` se valida por estructura antes de instalar.
- **Evitá overlaps**: si el job puede tardar más que su frecuencia, envolvelo con `flock -n /tmp/<name>.lock ...`.
- **Siempre logueá**: redirigí `stdout/stderr` a un archivo determinístico.

## Decision Table

| Caso | Dónde va | Validación | Acción recomendada |
|---|---|---|---|
| Job nuevo administrado por AgentHub | `/etc/cron.d/agenthub-<slug>` | chequeo estructural + owner/perms | crear archivo dedicado |
| Job del usuario pedido explícitamente | `crontab -e` / `crontab file` | `crontab -n file` | backup + instalar |
| Job de paquete / hook periódico | `/etc/cron.daily` etc. | `run-parts --test` si aplica | usar solo si el user lo pidió |
| Pausa temporal | mismo archivo cron.d | n/a | comentar la línea o mover backup fuera de `/etc/cron.d` |
| Baja definitiva | mismo archivo cron.d | n/a | backup + borrar |

## Safe Workflow

1. **Inventariar**
   - `list_cronjobs`
   - o `curl -s http://127.0.0.1:8093/api/system/cronjobs`
2. **Buscar colisiones**
   - mismo comando
   - misma frecuencia
   - mismo log/lock
3. **Elegir ubicación**
   - default: `/etc/cron.d/agenthub-<slug>`
   - user crontab solo si fue pedido explícitamente
4. **Preparar contenido**
   - header con `SHELL` y `PATH`
   - línea cron con usuario (`root`) si va en `/etc/cron.d`
   - comando absoluto + logs + `flock` si hace falta
5. **Validar**
   - para user crontab: `crontab -n file`
   - para cron.d: verificar 6+ campos (`m h dom mon dow user cmd`) o macro `@daily user cmd`
6. **Instalar**
   - `sudo install -o root -g root -m 0644 file /etc/cron.d/agenthub-<slug>`
7. **Verificar**
   - `ls -l /etc/cron.d/agenthub-<slug>`
   - re-listar con `list_cronjobs`
   - revisar `journalctl -u cron -n 50 --no-pager` si hay dudas

## Structural Rules for `/etc/cron.d`

- Header mínimo:
  - `SHELL=/bin/bash`
  - `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
- Formato:
  - normal: `m h dom mon dow user command`
  - macro: `@daily user command`
- El archivo debe:
  - pertenecer a `root:root`
  - no ser writable por group/others
  - no necesitar bit ejecutable

## Minimal Examples

### Crear job nuevo en cron.d

```cron
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

*/15 * * * * root flock -n /tmp/agenthub-sync.lock /usr/bin/curl -fsS http://127.0.0.1:8093/healthz >> /home/nestor/agenthub/logs/cron/agenthub-sync.log 2>&1
```

### Pausar sin borrar

```cron
# */15 * * * * root flock -n /tmp/agenthub-sync.lock /usr/bin/curl -fsS http://127.0.0.1:8093/healthz >> /home/nestor/agenthub/logs/cron/agenthub-sync.log 2>&1
```

### User crontab con dry-run

```bash
TMP="$(mktemp)"
crontab -l > "$TMP"
printf '%s\n' '0 13 * * * /home/nestor/finanzas/run_monitor.sh' >> "$TMP"
crontab -n "$TMP"
crontab "$TMP"
rm -f "$TMP"
```

## Commands

```bash
# Inventario via AgentHub
curl -s http://127.0.0.1:8093/api/system/cronjobs

# Backup de un cron administrado
sudo cp /etc/cron.d/agenthub-demo /tmp/agenthub-demo.$(date +%Y%m%d-%H%M%S).bak

# Instalar archivo cron.d
sudo install -o root -g root -m 0644 /tmp/agenthub-demo /etc/cron.d/agenthub-demo

# Verificar que cron lo ve
sudo ls -l /etc/cron.d/agenthub-demo
journalctl -u cron -n 50 --no-pager

# Dry-run para crontab de usuario
crontab -n /tmp/user.cron

# Probar qué correría run-parts
run-parts --test /etc/cron.daily
```

## Anti-Patterns

- Meter jobs nuevos en el crontab personal del usuario “porque es más rápido”.
- Usar paths relativos o depender de aliases/env de shell interactiva.
- Reusar el mismo archivo cron.d para jobs no relacionados.
- Reiniciar `cron` después de cada cambio: no hace falta en este host.
- Renombrar archivos de `/etc/cron.d` a `*.disabled` y asumir que cron siempre los ignorará.
- Crear scripts en `/etc/cron.daily` con puntos o nombres raros; `run-parts` filtra nombres.
- Modificar cron ajeno sin backup antes.

## Resources

- **Template**: ver [assets/cron.d.template](assets/cron.d.template)
- **Notas locales verificadas**: ver [references/local-cron-notes.md](references/local-cron-notes.md)
