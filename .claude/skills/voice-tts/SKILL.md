---
name: voice-tts
description: Generar notas de voz (.ogg opus) con edge-tts (voz Paloma) y mandarlas por WhatsApp con send_voice.
---

# Voz TTS — texto a nota de voz por WhatsApp

Sintetiza audio en español neutro y lo despacha como nota de voz por WA.
Usa el venv `/home/nestor/tts-venv/` que ya tiene `edge-tts` instalado.

## Cuándo usar

- El user te mandó una nota de voz → contestá con voz si la respuesta es corta y conversacional.
- Anuncios largos donde escuchar > leer (resumen del día, recordatorio).
- El user lo pide explícito ("decímelo en voz", "mandame una nota de voz").

**NO** usar para respuestas cortas tipo "ok"/"listo" ni para data tabular — texto es mejor.

## Pipeline

```bash
# 1. Generar mp3 con edge-tts
/home/nestor/tts-venv/bin/python3 -m edge_tts \
  --text "hola, esto es lo que querías saber" \
  --voice es-US-PalomaNeural \
  --write-media /tmp/voz.mp3

# 2. Convertir a .ogg opus (WA descarta otros codecs silenciosamente)
ffmpeg -y -i /tmp/voz.mp3 -c:a libopus -b:a 64k /tmp/voz.ogg

# 3. Enviar como nota de voz (PTT=true en WA)
send_voice jid="51922743968" path="/tmp/voz.ogg"
```

## Voces disponibles

Default: **es-US-PalomaNeural** ("Paloma") — natural, neutra.

Alternativas decentes:

| Voz | Acento |
| --- | --- |
| `es-AR-ElenaNeural` | argentina rioplatense (femenina) |
| `es-AR-TomasNeural` | argentina rioplatense (masculina) |
| `es-MX-DaliaNeural` | mexicana neutra |

Listar todas: `python3 -m edge_tts --list-voices | grep es-`

## Reglas duras

1. **El archivo final SIEMPRE debe ser .ogg opus**. WA descarta otros codecs silenciosamente.
2. **Bitrate 48–64 kbps** alcanza para voz; subir más sólo agrega peso.
3. **Texto < 1500 chars** por nota — más largo se hace pesado y poco escuchable. Si excede, partilo en varias.
4. **Limpiá `/tmp/voz.*`** si vas a generar muchas notas en una sesión.
