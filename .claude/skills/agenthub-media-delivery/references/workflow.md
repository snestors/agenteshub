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

## 5. Project/GridBot/Codex fallback

Some project sessions run without AgentHub MCP media tools. In that case:

1. Publish the file under `data/uploads/shared/`.
2. Return the direct `/api/file` link.
3. Do not manually insert `wa_messages` rows unless the user explicitly asked you to patch AgentHub itself.
4. Do not claim “te lo dejé en el chat” unless `send_*` succeeded or the UI visibly shows the media row.

Good fallback response:

```text
No pude insertarlo como preview desde esta sesión, pero quedó publicado acá:
https://agenthub.kyn3d.com/api/file?path=...
```

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
