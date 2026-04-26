---
name: agent-management
description: Crear, listar, modificar, pausar y eliminar mini-agentes especializados.
  Trigger cuando el user pida "creame un agente", "vigilá X cada Y", "monitoreame",
  "pausá", "qué agentes tengo".
---

# Cómo gestionar mini-agentes

Cuando el user pida vigilar / monitorear / automatizar algo recurrente:

## Patrón de creación

1. **Definir un nombre kebab-case**: `sonarr-watcher`, `email-summary`, `grid-monitor`.
2. **Redactar un `system_prompt` mínimo y específico** (3-5 líneas). Patrón:
   > Sos `<name>`. Tu única tarea es `<X>`. Reportá solo si hay novedades. No hagas suposiciones.
3. **Decidir el trigger**:
   - Recurrencia (cron) → `agent_schedule(name, cron_expr, prompt_template, notify_target?)`
   - Solo manual → omitir schedule
4. **`notify_target` por default = `main-agent`**. Si el user pidió notificación cruda, `wa:<jid>`.
5. **Confirmar al user**: nombre + próxima corrida en formato humano.

## Tools disponibles

| Tool | Para |
|---|---|
| `agent_create(name, system_prompt, engine?, description?)` | Crear |
| `agent_list` | Listar todos |
| `agent_pause(name)` / `agent_resume(name)` | Toggle |
| `agent_run_now(name, prompt?)` | Disparo manual |
| `agent_logs(name, limit?)` | Ver últimas runs |
| `agent_schedule(name, cron, prompt, target?)` | Agendar |

## Cron expressions útiles

- `@every 1h` — cada hora
- `0 * * * *` — al minuto 0 de cada hora
- `*/15 * * * *` — cada 15 min
- `0 8,18 * * *` — 8 AM y 6 PM
- `0 0 * * *` — medianoche

## Ejemplo end-to-end

User: *"creame un agente que cada hora revise sonarr y me avise solo si hay descargas nuevas"*

```
agent_create(
  name="sonarr-watcher",
  system_prompt="Sos sonarr-watcher. Revisás Sonarr y reportás solo novedades.",
)
agent_schedule(
  name="sonarr-watcher",
  cron_expr="0 * * * *",
  prompt_template="Consultá la API de Sonarr. Si hay episodios importados desde el último run, mandá un resumen breve. Si no hay, no mandes nada.",
  notify_target="topic:casa-media",
)
```

Respuesta: *"Listo, sonarr-watcher creado. Próxima corrida 09:00. Te avisará en topic casa-media solo si hay descargas nuevas."*
