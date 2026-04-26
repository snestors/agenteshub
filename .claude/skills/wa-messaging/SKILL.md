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

| Tool | Args |
| --- | --- |
| `send_message` | `text` |
| `send_image` | `jid`, `path`, `caption?` |
| `send_voice` | `jid`, `path` (.ogg opus) |
| `send_audio` | `jid`, `path` |
| `send_document` | `jid`, `path`, `caption?` |
| `send_video` | `jid`, `path`, `caption?` |
| `send_location` | `jid`, `lat`, `lng`, `name?` |

`jid` puede ser dígitos sueltos (`51922743968`) o el JID completo
(`51922743968@s.whatsapp.net`). El primero se asume `@s.whatsapp.net`.

## Reglas duras

1. **`send_message` ES obligatorio en canal WA**. Tu output natural NO se
   entrega al user cuando el canal es WhatsApp. Sin `send_message`, el
   user no recibe nada.
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

Para revisar lo que llegó:
- `recent_messages(channel='wa', limit=N)` — últimos N
- Si `media_path` está set, podés leer el archivo con la tool Read.
- Para voice notes, transcribir con whisper local:
  ```bash
  /home/nestor/tts-venv/bin/python3 -c "
  import whisper; m = whisper.load_model('tiny')
  print(m.transcribe('/path/audio.ogg', language='es')['text'])
  "
  ```

## TTS (texto → voz) para mandar notas

Voz preferida: **es-US-PalomaNeural** ("Paloma").

```bash
/home/nestor/tts-venv/bin/python3 -m edge_tts \
  --text "hola, esto es lo que querías saber" \
  --voice es-US-PalomaNeural \
  --write-media /tmp/voz.mp3
ffmpeg -y -i /tmp/voz.mp3 -c:a libopus -b:a 64k /tmp/voz.ogg
# después:
send_voice jid="51922743968" path="/tmp/voz.ogg"
```

## Flujo típico al recibir un mensaje WA

1. `recent_messages(channel='wa', limit=5)` para ver contexto
2. Procesar
3. **`send_message(text=...)`** para responder texto
4. O `send_image/document/...` si tenés que mandar un archivo
5. Si la respuesta es larga, considerá `send_voice` con TTS
6. **NUNCA** dejes un mensaje sin responder — el user te está esperando

## Estado del outbox

Cada item queda en `wa_outbox` con su status. Para auditar:
```sql
SELECT id, kind, status, error, created_at, sent_at
FROM wa_outbox ORDER BY created_at DESC LIMIT 20;
```

Si ves muchos `error` con la misma razón ("not connected", "upload
failed"), avisalo al user — algo está mal con el bridge.
