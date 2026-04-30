# AgentHub UI video workflow

Use this reference when a user asks for a visual/explainer video about AgentHub, its architecture, workflows, or UI.

## Goal

Produce an MP4 that is more than a generic animation:

- includes real screenshots from the current AgentHub UI;
- explains a concrete runtime flow with diagrams/callouts;
- renders via HyperFrames;
- publishes the MP4 under `data/uploads/shared/` so the user can click/download it through `/api/file`.

## Recommended structure

For a 35–50s landscape explainer:

1. **Intro** — one brain, two surfaces, what problem it solves.
2. **Real UI** — screenshot of the current chat or page the user cares about.
3. **Runtime graph** — Web/WA → `agenthub` daemon → engines → MCP tools → SQLite/outbox → output.
4. **Operation screen** — system/services/cron/projects/skills as appropriate.
5. **Workflow view** — project sessions, release/deploy, skills, or SDD flow.
6. **Final output** — how the requested deliverable is exposed: video bubble, file link, WhatsApp media, etc.

## Capture real UI screenshots

Do not fake UI screenshots when the local daemon is healthy. Capture from the live app using the bundled CDP script:

```bash
cd /home/nestor/agenthub
mkdir -p videos/<slug>/assets/screens
set -a; source .env; set +a
.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs \
  videos/<slug>/assets/screens \
  http://127.0.0.1:8093 \
  chat-main=/ system-cronjobs=/system projects=/projects skills=/skills releases=/releases
```

Notes:

- The script generates a short-lived JWT in memory from `AGENTHUB_JWT_SECRET`; never print or log the secret.
- It sets the `agenthub_token` cookie through Chrome DevTools Protocol and captures authenticated screenshots.
- It requires `/usr/bin/google-chrome` and a live AgentHub daemon.
- If screenshots show stale content, first refresh the relevant UI or insert the desired media/message through the normal product flow.

## Build the HyperFrames project

```bash
cd /home/nestor/agenthub
npx hyperframes init videos/<slug> --non-interactive
```

Then create:

- `videos/<slug>/brief.md` — narrative and scenes.
- `videos/<slug>/index.html` — root HyperFrames composition.
- `videos/<slug>/assets/screens/*.png` — real UI screenshots.

Follow AgentHub visual identity from root `DESIGN.md`:

- dark navy background;
- cyan/magenta/lime/orange accents;
- HUD panels with clipped corners;
- circuit/grid lines;
- real UI screenshots in framed cards;
- animated arrows/wires for architecture flow.

Use GSAP for motion, but keep it deterministic. Register `window.__timelines["main"]` synchronously.

## Validate

Always run before rendering:

```bash
npx hyperframes lint videos/<slug>
npx hyperframes inspect videos/<slug> --samples 18
```

Fix errors. Warnings about file size/dense track are acceptable for small one-off videos, but mention them if they remain.

## Render and publish

```bash
mkdir -p data/generated-videos data/uploads/shared
npx hyperframes render videos/<slug> \
  --output /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  --fps 30 --quality standard
ln -f /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  /home/nestor/agenthub/data/uploads/shared/<slug>.mp4
ffprobe -v error -show_entries format=duration,size -of default=nw=1:nk=1 \
  /home/nestor/agenthub/data/generated-videos/<slug>.mp4
```

User-facing link format:

```text
https://agenthub.kyn3d.com/api/file?path=%2Fhome%2Fnestor%2Fagenthub%2Fdata%2Fuploads%2Fshared%2F<slug>.mp4
```

Generate the encoded path with Python when unsure:

```bash
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/<slug>.mp4'
print('https://agenthub.kyn3d.com/api/file?path=' + quote(p, safe=''))
PY
```

## Make it visible in chat

Preferred path: if the active agent has MCP `send_video`, call it without `jid`:

```text
send_video(path="/home/nestor/agenthub/data/uploads/shared/<slug>.mp4", caption="...")
```

That posts a proper playable web bubble through the product flow.

Fallback from a coding shell only: insert a `web/out` `wa_messages` row with `media_type='video'` and the `media_path`, but be explicit that a direct DB insert does **not** broadcast live WebSocket events and the user may need to refresh.

## Repo bookkeeping

For AgentHub project changes:

- bump `VERSION` and `frontend/package.json`;
- add `RELEASE_NOTES.md` entry;
- update `videos/README.md` with rerender commands;
- commit with Conventional Commit;
- if version is bumped, build/smoke/promote with `deploy-safe-restart`.
