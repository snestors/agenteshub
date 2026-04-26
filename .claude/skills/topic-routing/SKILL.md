---
name: topic-routing
description: Cómo el agente principal detecta el topic del mensaje y delega
  cuando hace falta resolver algo en serio.
---

# Topic routing — guía del main agent

## Reglas

1. Si el mensaje del user es **casual o no tiene tema claro** → respondé directo desde tu memoria, sin tools.
2. Si detectás un **tema específico** (grid-bot, casa-media, salud, etc.):
   - Llamá `read_topic_state(topic)` primero (es barato).
   - Si con eso te alcanza para responder bien, sintetizá y respondé.
   - Si el tema **necesita trabajo en serio** (resolver un bug, decidir algo,
     ejecutar pasos) → delegá con `run_in_topic(topic, message)`. Esto reanuda
     la session persistente del topic con contexto pleno y devuelve la
     respuesta para que vos se la pases al user.
   - Si solo aprendiste algo nuevo del topic, llamá `update_topic_state` al
     final para reflejarlo.
3. **No preguntes "¿de qué tema hablamos?"** — vos detectás por contenido.

## Topics conocidos

- `general` — catch-all (default cuando no identificás otro)
- `grid-bot` — bot de trading en Bitget, PnL, errores, decisiones
- `casa-media` — Sonarr, qBit, Emby, descargas
- `salud` — calorías, ejercicio, dormir
- `finanzas` — sueldo, gastos, BBVA

Si aparece un tema que no está en la lista, podés crearlo con
`topic_create(name, description?, keywords?, engine?)`. La session_id se
asigna sola la primera vez que se llame `run_in_topic` para ese topic.

## Cuándo NO delegar con run_in_topic

- Preguntas factuales rápidas que ya están en `read_topic_state`.
- Charla casual o saludos sobre el topic.
- Dudas sobre qué hace el topic (respondé desde memoria).

Reservá `run_in_topic` para acciones reales: depurar, decidir, ejecutar.

## Indicador en la respuesta

Cuando respondas en base a un topic específico, terminá tu mensaje con una
línea breve `📍 <topic>` para que el user sepa en qué contexto estás.

Ejemplo:

> Bajaron 3 episodios de The Last of Us anoche. HDD al 78%.
>
> 📍 casa-media

## Salida según canal

- `[canal: wa]` → respondé con `send_message` (única forma).
- `[canal: web]` → tu output natural ya llega al user; no necesitás `send_message`.
