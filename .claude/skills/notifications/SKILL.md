---
name: notifications
description: Patrones para enviar mensajes ricos al user (texto, voz, imagen,
  ubicación, documento) según el canal activo.
---

# Notifications

## Por canal

- `[canal: wa]` → tu output natural NO llega al user. **Tenés que usar `send_message`** y demás tools de salida. Una sola tool por mensaje (no spamees).
- `[canal: web]` → tu output natural ya llega via WS. **No uses `send_message`** salvo que quieras enviar algo "extra" o multimedia.

## Tools de salida

| Tool | Cuándo |
|---|---|
| `send_message(text)` | Texto plano |
| `send_image(path \| url, caption?)` | Mostrar imagen, captura, gráfico |
| `send_voice(path)` | Nota de voz (ogg/opus) — para mensajes "íntimos" o cuando el user está manejando |
| `send_audio(path)` | Audio normal (mp3/m4a) — para música o clips no-voz |
| `send_document(path, filename?, caption?)` | PDFs, zips, archivos grandes |
| `send_location(lat, lng, name?)` | Pin en mapa |
| `send_video(path, caption?)` | Video |
| `react_to_message(message_id, emoji)` | Reacción rápida sin texto |

## Reglas

1. **Preferí texto cuando alcance** — voz/imagen son ricos pero gastan más.
2. **Concatená — no spamees**: si tenés 3 cosas que decir, una sola tool con texto bien estructurado, no 3 calls.
3. **Si vas a mandar imagen, agregá caption breve**.
4. **Para listados largos**, usá texto con bullets `- `, no JSON.
