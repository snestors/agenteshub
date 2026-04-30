---
name: agenthub-media-delivery
description: Deliver images, videos, audio, voice notes, and documents through AgentHub's current Web/WhatsApp chat or as safe downloadable links; use when a user asks to send, show, preview, attach, publish, or share generated media such as HyperFrames/Remotion MP4s, screenshots, photos, PDFs, or audio.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

# AgentHub Media Delivery

## When to use

Use this skill whenever an agent needs to deliver a non-text file to Nestor:

- photos/screenshots/images;
- HyperFrames, Remotion, ffmpeg, or other generated videos;
- audio, voice notes, documents, PDFs, CSVs, zips;
- previews/download links in the AgentHub web chat or Cloudflare tunnel;
- WhatsApp media to another contact.

## Critical rules

1. **Text replies are natural output.** Do not call `send_message` to answer the user who wrote the current turn; AgentHub delivers the final assistant text automatically.
2. **Media to the current live chat uses `send_*` without `jid`.** If the tool is available, call `send_image`, `send_video`, `send_document`, `send_audio`, or `send_voice` with an absolute local `path` and no `jid`.
3. **Media to another WhatsApp chat requires `jid`.** Only pass `jid` when the destination is a different contact/chat.
4. **Always provide a direct link for generated files.** Even when using `send_video`, include or be ready to provide the `/api/file` link so tunnel/local users can open/download it.
5. **Paths are daemon filesystem paths, not URLs.** `send_*` expects an existing absolute path on the mini PC.
6. **Do not fake a preview with raw SQLite inserts.** DB-only inserts do not broadcast WebSocket events; use `send_*` or return a link.
7. **Project/Codex sessions may not expose MCP media tools.** If `send_video` is unavailable or returns `jid required outside the live web/wa chat`, publish under `data/uploads/shared/` and return the link instead of pretending it was posted inline.

## Decision table

| Need | Action |
| --- | --- |
| Show image/video in the current main Web/WA chat | `send_image` / `send_video` with `path`, optional `caption`, **no `jid`** |
| Send media to another WhatsApp contact | `send_image` / `send_video` / etc. with `jid`, `path`, optional `caption` |
| Agent is in a project/GridBot/Codex session without `send_*` | Copy/hardlink to `data/uploads/shared/`, return `/api/file` link |
| User says they are on the tunnel | Return full `https://agenthub.kyn3d.com/api/file?path=...` URL |
| User says they are local/LAN | Return relative `/api/file?path=...` or `http://192.168.1.62:8093/api/file?path=...` |
| Voice note | Create Opus `.ogg` first, then `send_voice(path=...)` |
| Existing upload from user | Use its existing `data/uploads/...` path directly |

## Minimal examples

Current chat video:

```text
send_video(path="/home/nestor/agenthub/data/uploads/shared/demo.mp4", caption="Demo listo")
```

Current chat image:

```text
send_image(path="/home/nestor/agenthub/data/uploads/shared/screen.png", caption="Captura")
```

Other WhatsApp chat:

```text
send_video(jid="519XXXXXXXX@s.whatsapp.net", path="/home/nestor/agenthub/data/uploads/shared/demo.mp4", caption="Te paso el demo")
```

## Publish a generated file safely

Read the full workflow in [references/workflow.md](references/workflow.md). The short version:

```bash
cd /home/nestor/agenthub
mkdir -p data/uploads/shared
ln -f /tmp/demo.mp4 data/uploads/shared/demo.mp4 || cp /tmp/demo.mp4 data/uploads/shared/demo.mp4
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/demo.mp4'
print('https://agenthub.kyn3d.com/api/file?path=' + quote(p, safe=''))
PY
```

Then either call `send_video(path="/home/nestor/agenthub/data/uploads/shared/demo.mp4", caption="...")` or return the printed link.

## Verification before telling the user

- `test -s <path>` confirms the file exists and is not empty.
- `ffprobe -v error -show_entries format=duration,size -of default=nw=1 <video.mp4>` confirms a generated video is playable enough to serve.
- `curl -I 'http://127.0.0.1:8093/api/file?path=<encoded>'` confirms AgentHub can serve it from the upload root.

## Failure wording

If inline delivery is not available, say it plainly and give the link:

> No pude insertarlo como preview desde esta sesión de proyecto; te dejo el link directo para abrir/descargar: ...
