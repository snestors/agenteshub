---
name: wa-messaging
description: Cómo mandar texto, imágenes, notas de voz, audio, documentos,
  videos y ubicaciones por WhatsApp desde AgentHub.
---

# WhatsApp messaging — guía

Las MCP tools `send_*` **encolan** el mensaje en `wa_outbox`; el daemon
principal lo procesa cada ~500ms con el `wa.Client` (whatsmeow). Por eso
respondés `"queued id=N kind=..."` sin esperar — el agente confía en que
si la cola está activa, el item llega. Si necesitás verificar, leés el
estado con `query_records` o consultás la DB.

## Tools disponibles

Todas las tools de envío aceptan un argumento opcional `reply_to`:
el `external_id` (StanzaID de WhatsApp) del mensaje que querés citar.
Cuando lo pasás, la respuesta sale citando el mensaje original como
hace WhatsApp en su UI.

| Tool | Args |
| --- | --- |
| `send_message` | `text`, `jid?`, `reply_to?` |
| `send_image` | `jid?`, `path`, `caption?`, `reply_to?` |
| `send_voice` | `jid?`, `path` (.ogg opus), `reply_to?` |
| `send_audio` | `jid?`, `path`, `reply_to?` |
| `send_document` | `jid?`, `path`, `caption?`, `reply_to?` |
| `send_video` | `jid?`, `path`, `caption?`, `reply_to?` |
| `send_location` | `jid`, `lat`, `lng`, `name?`, `reply_to?` |

`send_message` con `jid` además encola un envío a WhatsApp (no solo
persiste para la web). Con `jid + reply_to` cita un mensaje específico.

Para mostrar media en el **chat activo** (Web/WA actual), omití `jid`.
Para mandar a **otro** chat/contacto, pasá `jid`.

`jid` puede ser dígitos sueltos (`51922743968`) o el JID completo
(`51922743968@s.whatsapp.net`). El primero se asume `@s.whatsapp.net`.

## Reglas duras

1. **NO uses `send_message` para responderle al user que te escribió.** El
   daemon (converger) toma tu output de texto natural y lo entrega al canal
   correcto automáticamente: si te escribió por WA → vuelve por WA Y queda
   en el chat web. `send_message` queda **únicamente** para mandar a OTRO
   contacto distinto (notificación cruzada, alerta a un tercero, etc).
2. **Las rutas son del filesystem del daemon**, no del cliente. Si querés
   enviar una imagen que el user te subió por la web, ya está en
   `data/uploads/<id>` — usá esa ruta directamente.
3. **No mandes voz si el archivo no es .ogg opus**. WhatsApp lo descarta
   silenciosamente. Conversión típica:
   ```bash
   ffmpeg -y -i input.mp3 -c:a libopus -b:a 64k /tmp/out.ogg
   ```
4. **Documentos**: el filename mostrado es el `basename(path)`. Si querés
   un nombre lindo, copiá/renombrá antes:
   ```bash
   cp /tmp/raw.pdf /tmp/Factura_Marzo_2026.pdf
   send_document jid="..." path="/tmp/Factura_Marzo_2026.pdf"
   ```
5. **Locations**: lat/lng en grados decimales. Si tenés grados/minutos,
   convertí antes. `name` opcional — lugar del pin.

## Mensajes entrantes (recibir)

El daemon recibe automáticamente y persiste en `wa_messages` con
`channel='wa'`, `direction='in'`, `media_type` (`image|voice|audio|video|document`)
y `media_path` (ruta local descargada). Las locaciones llenan `location_lat`,
`location_lng`, `location_name`.

Cada row trae también:
- `external_id` — el StanzaID que WA usa internamente. Es lo que pasás
  como `reply_to` en una tool de envío para **citar ese mensaje**.
- `reply_to` — si el user te respondió a un mensaje tuyo, este campo
  trae el StanzaID del original. Usalo para entender el contexto del
  "el user está respondiendo sobre X de hace 5 mensajes".

Para revisar lo que llegó:
- `recent_messages(channel='wa', limit=N)` — últimos N
- Si `media_path` está set, podés leer el archivo con la tool Read.
- Para **transcribir notas de voz** entrantes, leé la skill `audio-stt`.
- Para **mandar una nota de voz** (TTS), leé la skill `voice-tts`.

## Flujo típico al recibir un mensaje WA

1. `recent_messages(channel='wa', limit=5)` para ver contexto
2. Procesar. Si querés citar un mensaje específico, anotá su `external_id`.
3. **`send_message(text=..., jid=..., reply_to=<external_id>)`** para
   responder texto citando. Sin `reply_to` el envío sale como un mensaje
   normal sin cita.
4. O `send_image/document/...` si tenés que mandar un archivo (también
   acepta `reply_to`).
5. Si la respuesta es larga, considerá `send_voice` con TTS.
6. **NUNCA** dejes un mensaje sin responder — el user te está esperando.

## Ejemplo de reply citando

```
recent_messages(channel='wa', limit=3)
→ [
    {id: 401, body: "che, anda viendo cuánto pesé hoy", external_id: "3EB0AB12..."},
    ...
  ]

# El user después manda algo no relacionado y querés volver al primer mensaje:
send_message(
  text: "según los registros de hoy: 87.3 kg (-0.2 vs ayer)",
  jid: "51922743968",
  reply_to: "3EB0AB12..."   ← el external_id del mensaje 401
)
```

El mensaje sale con la cita de "che, anda viendo cuánto pesé hoy"
arriba en color WhatsApp, queda claro a qué le respondés.

## Estado del outbox

Cada item queda en `wa_outbox` con su status. Para auditar:
```sql
SELECT id, kind, status, error, created_at, sent_at
FROM wa_outbox ORDER BY created_at DESC LIMIT 20;
```

Si ves muchos `error` con la misma razón ("not connected", "upload
failed"), avisalo al user — algo está mal con el bridge.
