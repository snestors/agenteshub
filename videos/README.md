# Videos de demo — AgentHub

Este directorio guarda demos renderizables de flujos de AgentHub.

## Demos actuales

- `hyperframes-agenthub-workflows/` — versión HTML + GSAP + HyperFrames
- `remotion-agenthub-workflows/` — versión React + Remotion
- `agenthub-workflows-brief.md` — brief compartido para ambas piezas

## Render

### HyperFrames

```bash
cd /home/nestor/agenthub/videos/hyperframes-agenthub-workflows
npx hyperframes lint
npx hyperframes inspect . --samples 12
npx hyperframes render . --output /home/nestor/agenthub/data/generated-videos/agenthub-workflows-hyperframes.mp4 --quality standard
```

### Remotion

```bash
cd /home/nestor/agenthub/videos/remotion-agenthub-workflows
pnpm install
pnpm exec remotion render src/index.ts AgentHubWorkflows /home/nestor/agenthub/data/generated-videos/agenthub-workflows-remotion.mp4
```

## Output esperado

Los renders quedan fuera de Git en:

- `/home/nestor/agenthub/data/generated-videos/agenthub-workflows-hyperframes.mp4`
- `/home/nestor/agenthub/data/generated-videos/agenthub-workflows-remotion.mp4`


## AgentHub architecture tour

Nuevo video visual con capturas reales de la UI y diagrama de funcionamiento:

```bash
cd /home/nestor/agenthub
npx hyperframes lint videos/agenthub-architecture-tour
npx hyperframes inspect videos/agenthub-architecture-tour --samples 18
npx hyperframes render videos/agenthub-architecture-tour \
  --output /home/nestor/agenthub/data/generated-videos/agenthub-architecture-tour.mp4 \
  --fps 30 --quality standard
ln -f /home/nestor/agenthub/data/generated-videos/agenthub-architecture-tour.mp4 \
  /home/nestor/agenthub/data/uploads/shared/agenthub-architecture-tour.mp4
```

## Reusable skill

Para repetir este flujo en futuros videos, usar la skill del proyecto:

- `.claude/skills/agenthub-ui-video/SKILL.md`
- `.claude/skills/agenthub-ui-video/references/workflow.md`
- `.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs`

Ejemplo rápido:

```bash
cd /home/nestor/agenthub
set -a; source .env; set +a
.claude/skills/agenthub-ui-video/scripts/capture-agenthub-ui.mjs \
  videos/<slug>/assets/screens
```


## Generic video generator skill

Para videos no necesariamente ligados a capturas de AgentHub, usar:

- `.claude/skills/video-generator/SKILL.md`
- `.claude/skills/video-generator/references/workflow.md`

Esta skill decide HyperFrames vs Remotion, estructura el brief, valida, renderiza y publica el MP4 en `data/uploads/shared/`.

Para entregar el resultado al chat actual o como link descargable, usar:

- `.claude/skills/agenthub-media-delivery/SKILL.md`
- `.claude/skills/agenthub-media-delivery/references/workflow.md`

Regla práctica: `send_video(path=..., caption=...)` sin `jid` cuando la tool esté disponible; si no, link directo `/api/file` desde `data/uploads/shared/`.
