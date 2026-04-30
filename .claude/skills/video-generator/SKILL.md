---
name: video-generator
description: Generate polished videos from prompts, briefs, assets, screenshots, or repo context; choose HyperFrames by default and Remotion only when React-specific output is requested; validate, render, publish MP4 files, and provide downloadable/chat-ready links. Use when the user asks to create, improve, rerender, compare, package, or send any generated video, product explainer, workflow demo, architecture animation, social clip, or UI tour.
---

# Video Generator

## Default stack

- Use **HyperFrames** for new videos unless the user explicitly asks for Remotion or provides existing Remotion source.
- Use **Remotion** when the user specifically wants React components, an existing Remotion project, or a Remotion-vs-HyperFrames comparison.
- Use `agenthub-ui-video` when the video needs real AgentHub UI screenshots.
- Use `gsap` with HyperFrames for motion and transitions.

## Workflow

1. Read `references/workflow.md`.
2. Create a short brief: audience, message, scenes, duration, aspect ratio, assets, output path.
3. Gather assets:
   - screenshots via `agenthub-ui-video` for AgentHub UI tours;
   - local uploads from `data/uploads/`;
   - generated images only when the user asks for original visuals.
4. Build sources under `videos/<slug>/`.
5. Validate before render.
6. Render final MP4 to `data/generated-videos/<slug>.mp4`.
7. Hardlink/copy final MP4 to `data/uploads/shared/<slug>.mp4`.
8. Return the clickable `/api/file` URL.

## Quality bar

A generated video is not acceptable if it is only generic text cards. It should include at least two of:

- real screenshots or real assets;
- animated flow/architecture diagram;
- kinetic labels/callouts;
- before/after or workflow progression;
- clear final CTA or delivery state.

## Delivery rule

Always provide a direct download link. If the environment supports `send_video`, also post the MP4 into the current chat without `jid`; otherwise be explicit that a DB-only insertion needs refresh.

## Validation commands

HyperFrames:

```bash
npx hyperframes lint videos/<slug>
npx hyperframes inspect videos/<slug> --samples 18
npx hyperframes render videos/<slug> \
  --output /home/nestor/agenthub/data/generated-videos/<slug>.mp4 \
  --fps 30 --quality standard
```

Remotion:

```bash
cd videos/<slug>
pnpm install
pnpm exec remotion render <entry> <composition> /home/nestor/agenthub/data/generated-videos/<slug>.mp4
```
