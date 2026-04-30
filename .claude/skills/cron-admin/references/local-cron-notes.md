# Cron local — notas verificadas en este host

Fecha de verificación: 2026-04-30

## Estado del sistema

- Servicio activo: `cron`
- Binario: `/usr/sbin/cron`
- Proceso observado: `/usr/sbin/cron -f -P`
- `run-parts`: `/usr/bin/run-parts`
- Paquete detectado: `cron 3.0pl1-184ubuntu2`

## Validación soportada

- `crontab -n file` → **sí** (dry-run de crontab de usuario)
- `crontab -T file` → **no** (flag inexistente en este host)

## Reglas observadas desde `man cron` / `man crontab`

- `cron` monitorea `/etc/crontab` y `/etc/cron.d` para cambios.
- No hace falta reiniciar `cron` cuando cambia un archivo cron válido.
- `/etc/crontab` y `/etc/cron.d/*` usan formato con campo `user`.
- `/etc/cron.d/*` debe pertenecer a `root` y no ser group/other writable.
- `/etc/cron.d/*` no necesita bit ejecutable.
- `/etc/cron.hourly|daily|weekly|monthly` se ejecuta vía `run-parts`.

## Reglas observadas desde `man run-parts`

- Los nombres válidos de scripts para `run-parts` usan solo:
  - letras ASCII
  - dígitos
  - `_`
  - `-`

## Ubicación recomendada para jobs administrados por AgentHub

- `/etc/cron.d/agenthub-<slug>`
- nombre de archivo sin puntos ni sufijos raros

Motivo:
- un job por archivo
- alta/baja/pausa limpias
- sin tocar crontabs personales salvo que el user lo pida explícitamente
