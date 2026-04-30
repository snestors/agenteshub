---
name: agenthub-ui-video
description: Create polished HyperFrames explainer videos for AgentHub using real UI screenshots, animated architecture/flow diagrams, and downloadable MP4 delivery. Use when the user asks to make, improve, document, or rerender videos showing AgentHub workflows, UI behavior, architecture, media delivery, system pages, project sessions, skills, or product tours.
---

# AgentHub UI Video

## Core workflow

1. Read root `DESIGN.md` for AgentHub visual identity before writing video HTML.
2. Read `references/workflow.md` for the exact capture → HyperFrames → render → publish procedure.
3. Capture real UI screenshots with `scripts/capture-agenthub-ui.mjs` unless the user explicitly wants a mockup.
4. Build or update a HyperFrames project under `videos/<slug>/`.
5. Validate with `npx hyperframes lint` and `npx hyperframes inspect --samples 18`.
6. Render to `data/generated-videos/<slug>.mp4` and hardlink/copy to `data/uploads/shared/<slug>.mp4`.
7. Return a clickable `https://agenthub.kyn3d.com/api/file?path=...` link.

## Non-negotiables

- Use real UI screenshots whenever AgentHub is reachable locally.
- Never print `.env` secrets; the capture script reads `AGENTHUB_JWT_SECRET` only to mint an in-memory JWT.
- Do not claim a video is visible live in chat when you inserted it directly into SQLite; direct DB inserts require refresh and do not emit WebSocket events.
- Prefer `send_video(path, caption)` without `jid` when available, because that is the product path that creates a playable chat bubble.
- For project changes, follow AgentHub release rules: bump `VERSION`, bump `frontend/package.json`, update `RELEASE_NOTES.md`, commit, and use safe restart when the binary version changes.

## Reusable script

Capture authenticated screenshots:

```bash
cd /home/nestor/agenthub
set -a; source .env; set +a
.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs videos/<slug>/assets/screens
```

Optional route specs:

```bash
.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs \
  videos/<slug>/assets/screens \
  http://127.0.0.1:8093 \
  chat-main=/ system-cronjobs=/system projects=/projects skills=/skills releases=/releases
```

## Output standard

A good AgentHub tour video should include:

- one sentence problem framing;
- real UI screenshot cards;
- animated arrows/wires showing runtime flow;
- labels for daemon, engines, MCP tools, SQLite/outbox, Web/WA surfaces;
- a final scene showing how the output is delivered: chat bubble, media link, WhatsApp, or system action.
