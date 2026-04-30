# AgentHub media delivery workflow

Use this workflow to deliver generated or existing media through AgentHub.

## 1. Locate or create the file

The file must exist on the daemon filesystem.

```bash
test -s /absolute/path/to/file
```

For generated videos, prefer final outputs under:

- `data/generated-videos/<slug>.mp4` — render artifact;
- `data/uploads/shared/<slug>.mp4` — served/downloadable artifact.

Do not place project code or configs on `/media/hdd/`. That disk is only for heavy media libraries.

## 2. Publish to upload root for links

AgentHub's `/api/file` only serves files under `data/uploads/`.

```bash
cd /home/nestor/agenthub
mkdir -p data/uploads/shared
ln -f /absolute/source/file.mp4 data/uploads/shared/file.mp4 || \
  cp /absolute/source/file.mp4 data/uploads/shared/file.mp4
```

Use a stable, human-readable slug. Avoid spaces if possible.

## 3. Build the URL

Tunnel URL:

```bash
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/file.mp4'
print('https://agenthub.kyn3d.com/api/file?path=' + quote(p, safe=''))
PY
```

Local URL:

```bash
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/file.mp4'
print('http://192.168.1.62:8093/api/file?path=' + quote(p, safe=''))
PY
```

Relative link for the Web UI:

```bash
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/file.mp4'
print('/api/file?path=' + quote(p, safe=''))
PY
```

## 4. Inline delivery when tools are available

For the current live chat, omit `jid`:

```text
send_video(path="/home/nestor/agenthub/data/uploads/shared/file.mp4", caption="Video listo")
send_image(path="/home/nestor/agenthub/data/uploads/shared/file.png", caption="Captura")
send_document(path="/home/nestor/agenthub/data/uploads/shared/file.pdf", caption="Documento")
send_audio(path="/home/nestor/agenthub/data/uploads/shared/file.mp3")
send_voice(path="/home/nestor/agenthub/data/uploads/shared/file.ogg")
```

For another WhatsApp chat, pass `jid`:

```text
send_video(jid="519XXXXXXXX@s.whatsapp.net", path="/home/nestor/agenthub/data/uploads/shared/file.mp4", caption="Video listo")
```

## 5. El link /api/file como recurso externo

El link `/api/file?path=...` queda como recurso para clientes externos al chat (tunnel, browser remoto), no como fallback de un engine que no tenía la tool.

Cualquier engine (claude o codex) corriendo dentro de agenthub tiene `send_*` per-run vía MCP; usar `send_<kind>(path=...)` es el path normal. Solo publicá el link como complemento (para descarga/apertura fuera del chat) o si en un run específico la tool genuinamente no estuviera disponible.

## 6. Video sanity checks

```bash
ffprobe -v error -show_entries format=duration,size -of default=nw=1 \
  /home/nestor/agenthub/data/uploads/shared/file.mp4
```

For HyperFrames:

```bash
cd /home/nestor/agenthub
npx hyperframes lint videos/<slug>
npx hyperframes inspect videos/<slug> --samples 18
npx hyperframes render videos/<slug> \
  --output /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  --fps 30 --quality standard
ln -f data/generated-videos/<slug>.mp4 data/uploads/shared/<slug>.mp4 || \
  cp data/generated-videos/<slug>.mp4 data/uploads/shared/<slug>.mp4
```
