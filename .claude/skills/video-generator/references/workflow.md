# Video generator workflow

## Decide stack

Use HyperFrames for most new videos:

- HTML/CSS/JS source is easy for agents to edit;
- deterministic frame rendering;
- GSAP timelines;
- simple asset embedding;
- good fit for product explainers, architecture tours, captions, UI demos, and social clips.

Use Remotion when:

- the user explicitly asks for Remotion;
- an existing Remotion codebase must be edited;
- React component reuse matters;
- a side-by-side implementation comparison is requested.

## Brief template

Create `videos/<slug>/brief.md`:

```md
# <Title>

Goal: <what viewer should understand/do>
Audience: <who>
Duration: <seconds>
Format: 1920x1080 landscape unless user says otherwise
Style: AgentHub cyberpunk HUD / or user-provided style
Scenes:
1. Hook
2. Problem/context
3. Main flow
4. UI/assets proof
5. Outcome/CTA
Assets:
- path/to/file.png — use in scene N
Output:
- data/generated-videos/<slug>.mp4
- data/uploads/shared/<slug>.mp4
```

## HyperFrames source pattern

Required files:

- `videos/<slug>/index.html`
- `videos/<slug>/brief.md`
- `videos/<slug>/hyperframes.json`
- `videos/<slug>/meta.json`
- optional `videos/<slug>/assets/`

Before writing HTML, define visual identity from:

1. root `DESIGN.md`; or
2. user-provided brand/style; or
3. minimal style section in `brief.md`.

Use 1920x1080 unless user requests vertical/social.

Scenes should be actual visual sections, not just captions. Put every top-level scene on a timed `.clip` element with non-overlapping `data-start`, `data-duration`, and `data-track-index`.

Register GSAP timeline synchronously:

```js
window.__timelines = window.__timelines || {};
const tl = gsap.timeline({ paused: true });
// animations
window.__timelines["main"] = tl;
```

## Rendering and publishing

```bash
cd /home/nestor/agenthub
mkdir -p data/generated-videos data/uploads/shared
npx hyperframes lint videos/<slug>
npx hyperframes inspect videos/<slug> --samples 18
npx hyperframes render videos/<slug> \
  --output /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  --fps 30 --quality standard
ln -f /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  /home/nestor/agenthub/data/uploads/shared/<slug>.mp4
```

Generate public-in-app link:

```bash
python3 - <<'PY'
from urllib.parse import quote
p='/home/nestor/agenthub/data/uploads/shared/<slug>.mp4'
print('https://agenthub.kyn3d.com/api/file?path=' + quote(p, safe=''))
PY
```

## Chat delivery

Preferred:

```text
send_video(path="/home/nestor/agenthub/data/uploads/shared/<slug>.mp4", caption="...")
```

Fallback from a shell-only coding session:

```sql
INSERT INTO wa_messages(channel, direction, body, media_type, media_path, media_caption, ts, is_read, engine, model)
VALUES('web','out','<caption>','video','/home/nestor/agenthub/data/uploads/shared/<slug>.mp4','<caption>',unixepoch(),1,'codex','gpt-5.5');
```

Warn that the fallback does not emit a WebSocket event; the user may need to refresh.

## AgentHub repo bookkeeping

If sources/docs are added to the repo:

- bump `VERSION`;
- bump `frontend/package.json`;
- add `RELEASE_NOTES.md` entry;
- update `videos/README.md` when useful;
- commit with a Conventional Commit.

If the binary version is bumped, run deploy-safe-restart: build `bin/agenthub.next`, smoke on `127.0.0.1:8094` with temp DB and WA disabled, promote, safe-restart.
