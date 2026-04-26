---
name: ollama-engine
description: Cuándo y cómo usar Ollama (modelos locales) en mini-agentes
  o tareas livianas — no como main agent.
---

# Ollama engine — guía de uso

Ollama corre modelos open-source **locales** en `localhost:11434`. En este
mini PC (i5-12450H, 16 GB RAM) el modelo activo es `gemma4:e2b` (5.1 B Q4)
y `nomic-embed-text` (embeddings, no chat).

## Cuándo SÍ usarlo

- **Mini-agentes baratos** que clasifican, resumen corto o filtran. Ej:
  - `notify-ranker` rankea por urgencia mensajes recientes de WhatsApp.
  - `log-watcher` mira `journalctl` y resume en 1 párrafo si hay errores.
  - `intent-classifier` decide si un mensaje del user es chat casual o
    requiere derivar a un topic.
- **Pre-procesamiento** antes de mandar al main (que es caro). Ej: clasificar
  el topic con ollama y solo entonces invocar `read_topic_state`.
- **Tareas repetitivas** de un cron que no necesitan calidad alta y donde
  ahorrar créditos del plan max5x importa.
- **Modo offline / fallback** cuando claude está rate-limited.

## Cuándo NO usarlo

- **Main agent / chat con el user**. Va a sentirse lento (5-15 tok/s en este
  hardware) y la calidad de razonamiento de un 5 B no se acerca a Sonnet.
- **Tareas con tools (MCP)**. `gemma4:e2b` puede llamar tools pero la
  consistencia es inestable; vas a perder más tiempo debugueando que
  haciendo.
- **Código no trivial**. Para coding usá codex o claude.

## Cómo invocarlo

### Vía mini-agente (recomendado)

Cuando crees un mini-agente con `agent_create`, pasale `engine: "ollama"`:

```json
{
  "name": "notify-ranker",
  "engine": "ollama",
  "system_prompt": "Sos un clasificador. Te paso una lista de mensajes WA y devolvés JSON: {urgent: [ids], normal: [ids]}. Sin explicaciones.",
  "description": "Clasifica WA recientes por urgencia. Corre cada 5min."
}
```

El cron tickea, el scheduler invoca `cliengine.Run` con `engine=ollama`, y
el modelo configurado en `OLLAMA_MODEL` (env del daemon, default
`gemma:2b`) responde.

### Vía picker del frontend

El catálogo de engines descubre los modelos disponibles en runtime. Si
ollama está corriendo y tiene modelos chat-capable instalados, aparecen
en el dropdown del StatusBar. **No lo dejes seleccionado para el main
agent en uso normal** — es para experimentos.

## Tradeoffs concretos

| Eje              | Ollama (gemma4:e2b) | Claude Sonnet | Codex gpt-5.5 |
| ---------------- | ------------------- | ------------- | ------------- |
| Costo            | 0 (CPU/RAM local)   | Plan max5x    | Plan codex    |
| Latencia 1° tok  | 2-5 s               | 0.8-1.5 s     | 1-2 s         |
| Tok/s            | 5-15                | 50-80         | 40-70         |
| Razonamiento     | Bajo                | Alto          | Alto          |
| Tools / MCP      | Inestable           | Robusto       | Robusto       |
| Privacidad       | 100 % local         | API           | API           |
| Funciona offline | Sí                  | No            | No            |

## Cambiar el modelo default

```bash
# en deploy/agenthub.service o /etc/agenthub.env
OLLAMA_MODEL=gemma4:e2b
OLLAMA_URL=http://localhost:11434  # default
```

Después `systemctl restart agenthub`. El nuevo modelo aplica al próximo
turn de cualquier engine ollama.

## Tareas open / ideas para después

- Embeddings con `nomic-embed-text` para búsqueda semántica de mensajes
  (hoy hay solo FTS5, lexical). No es prioridad — la búsqueda lexical
  cubre 90 % de los casos.
- Auto-pull de modelos cuando se selecciona uno no instalado (UI →
  `ollama pull <model>`).
- Streaming de tokens (hoy `Stream: false` en `cliengine/ollama.go`) para
  que la UI muestre el output en vivo.
