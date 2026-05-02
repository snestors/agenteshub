# HyperFrames Composition Project — Grid Bot explainer

## Commands

```bash
npx hyperframes lint
npx hyperframes inspect . --samples 18
npx hyperframes render
```

## Style

Dark mode estilo Bitget. Fondo negro/grafito. Acentos verde (#22c55e / #4ade80) para compras y ganancias. Acentos rojo (#ef4444 / #f87171) para ventas. Texto grande. Voseo rioplatense. Sin TTS — todo subtítulos en pantalla.

## Key Rules

1. Every timed element needs `data-start`, `data-duration`, and `data-track-index`
2. Elements with timing MUST have `class="clip"`
3. Timelines paused, registered on `window.__timelines`
4. Deterministic only
