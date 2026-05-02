# HyperFrames Composition Project

## Skills

This project uses AI agent skills for framework-specific patterns. Install them if not already present:

```bash
npx skills add heygen-com/hyperframes
```

## Commands

```bash
npx hyperframes preview      # preview in browser (studio editor)
npx hyperframes render       # render to MP4
npx hyperframes lint         # validate compositions (errors + warnings)
npx hyperframes lint --json  # machine-readable output for CI
npx hyperframes docs <topic> # reference docs in terminal
```

## Project Structure

- `index.html` — main composition (root timeline)
- `compositions/` — sub-compositions referenced via `data-composition-src`
- `assets/` — media files (video, audio, images)
- `meta.json` — project metadata (id, name)

## Linting — Always Run After Changes

```bash
npx hyperframes lint
```

Fix all errors before considering the task complete.

## Key Rules

1. Every timed element needs `data-start`, `data-duration`, and `data-track-index`
2. Visible timed elements **must** have `class="clip"` for visibility control
3. GSAP timelines must be paused and registered on `window.__timelines`
4. Only deterministic logic — no `Date.now()`, no `Math.random()`, no network fetches
