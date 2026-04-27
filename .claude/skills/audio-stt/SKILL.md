---
name: audio-stt
description: Transcribir audios entrantes WhatsApp (voice notes) a texto con openai-whisper local en el venv tts-venv.
---

# Audio STT — transcribir notas de voz entrantes

Cuando el user te manda una nota de voz por WA, el daemon ya descargó el
archivo a `data/uploads/wa/<msg_id>.ogg`. Vos lo transcribís antes de procesar.

## Cuándo usar

- Mensaje WA entrante con `media_type='voice'` o `media_type='audio'` y `body` vacío/corto.
- El user adjunta un mp3/ogg pidiendo explícitamente que lo transcribas.

## Pipeline

```bash
/home/nestor/tts-venv/bin/python3 -c "
import whisper
model = whisper.load_model('tiny')   # o 'base' si tiny equivoca
result = model.transcribe(
    '/home/nestor/agenthub/data/uploads/wa/<msg_id>.ogg',
    language='es'
)
print(result['text'].strip())
"
```

Reemplazá `<msg_id>.ogg` por el `external_id` (StanzaID) que viene en la fila
de `wa_messages` para ese mensaje.

## Modelos

| Modelo | Tamaño | Velocidad mini PC | Cuándo |
| --- | --- | --- | --- |
| `tiny`  | 75 MB  | <2 s para 30 s de audio | default — frases cortas, español neutro |
| `base`  | 142 MB | 4–6 s para 30 s | acentos cerrados, palabras técnicas, nombres propios |
| `small` | 466 MB | 12 s+ para 30 s | NO usar — para WA `base` es el techo razonable |

Default: **`tiny`**. Si la transcripción sale rara (palabras inventadas, contexto perdido), reintentá con `base`.

## Reglas duras

1. **Siempre `language='es'`** — el auto-detect de Whisper es lento y a veces se confunde con voces marcadas.
2. **NO usar `whisper.cpp` que está en `/media/hdd/`** — ese binario es ARM (de la RPi vieja). El mini PC es x86_64 y no corre. Siempre Python.
3. **Después de transcribir, procesá el TEXTO** como si el user lo hubiera escrito. NO le respondas "transcribí: '...'" — eso es ruido. Hacé como si lo hubiera mandado escrito y respondé al contenido.
4. **Confirmá si entendiste mal** cuando la transcripción es ambigua: "entendí que querés X — ¿es correcto?". Mejor confirmar que actuar mal.
5. **Audios > 60 s** → tirá `base` directo, `tiny` se desbarranca con duración.
