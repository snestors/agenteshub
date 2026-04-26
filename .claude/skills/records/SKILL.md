---
name: records
description: Persistir y consultar datos espontáneos del agente (calorías,
  ejercicio, libros leídos, lo que sea) sin necesidad de schema fijo.
---

# Schema-on-read records

Cuando el user pida registrar algún dato (calorías, peso, ejercicio, libros,
gastos, ideas, etc.), **NO crees una tabla nueva** ni propongas migraciones.
Usá la tabla genérica `agent_records` con JSON libre.

## Tools

| Tool | Args |
|---|---|
| `record(topic, data)` | data es un JSON string |
| `query_records(topic, since?, limit?)` | leer histórico |
| `list_topics_records` | inventario de topics existentes |

## Convención de topics

- `kebab-case`, descriptivo: `calories`, `workouts`, `books-read`, `weight`, `mood`
- Si el user dice "registrame X" y X no encaja en topics existentes, inventá un
  nombre razonable.
- Llamá `list_topics_records` antes si dudás si el topic ya existe.

## Patrón JSON sugerido

Mantené el JSON simple y plano para que `json_extract` funcione en queries:

```json
{"value": 2300, "unit": "kcal", "meal": "lunch", "notes": "asado"}
```

Si vas a hacer reports tipo "calorías promedio últimos 7 días", el JSON debe
tener un campo numérico explícito (`value`).

## Ejemplo

User: *"registrame que comí 2300 calorías hoy en el almuerzo"*

```
record(
  topic="calories",
  data='{"value": 2300, "unit": "kcal", "meal": "lunch"}',
)
```

User: *"cuántas calorías comí esta semana?"*

```
query_records(topic="calories", since=<unix_7d_ago>)
```
Después sumás `value` de los rows y respondés.
