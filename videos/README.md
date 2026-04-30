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
