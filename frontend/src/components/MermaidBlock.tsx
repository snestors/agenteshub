import * as React from "react";
import mermaid from "mermaid";

let initialized = false;
let seq = 0;

function ensureMermaid() {
  if (initialized) return;
  mermaid.initialize({
    startOnLoad: false,
    theme: "dark",
    securityLevel: "strict",
    fontFamily: '"JetBrains Mono", ui-monospace, monospace',
    themeVariables: {
      background: "#060814",
      primaryColor: "#0a0f24",
      primaryTextColor: "#d6f5ff",
      primaryBorderColor: "#5ef0ff",
      lineColor: "#a3ff4e",
      secondaryColor: "#111936",
      tertiaryColor: "#211236",
      fontSize: "16px",
    },
    flowchart: { htmlLabels: true, curve: "basis", padding: 20 },
    sequence: { useMaxWidth: true },
  });
  initialized = true;
}

/**
 * MermaidBlock renders a Mermaid source as inline SVG.
 *
 * `size`:
 *   - "inline" (default): used in chat bubbles. The SVG keeps its natural
 *     dimensions but max-width is the container.
 *   - "fill": used in dedicated diagram pages. The SVG stretches to fill
 *     the parent, height auto, centered. Great for /diagrams.
 */
export function MermaidBlock({
  content,
  size = "inline",
}: {
  content: string;
  size?: "inline" | "fill";
}) {
  const [svg, setSvg] = React.useState<string>("");
  const [error, setError] = React.useState<string | null>(null);
  const stableId = React.useMemo(() => `m-${++seq}`, []);

  React.useEffect(() => {
    let alive = true;
    ensureMermaid();
    setSvg("");
    setError(null);
    mermaid
      .render(stableId, content)
      .then((res) => {
        if (!alive) return;
        setSvg(res.svg);
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setError(err instanceof Error ? err.message : "mermaid render error");
      });
    return () => {
      alive = false;
    };
  }, [content, stableId]);

  if (error) {
    return (
      <div className="my-2">
        <div className="mb-1 font-mono text-[10px] text-[var(--color-danger)] tracking-hud-tight">
          ⚠ mermaid: {error}
        </div>
        <pre
          className="px-3 py-2 overflow-x-auto clip-hud-sm font-mono text-[11.5px] leading-[1.5] whitespace-pre"
          style={{
            background: "rgba(0,0,0,0.55)",
            border: "1px solid var(--color-line)",
          }}
        >
          {content}
        </pre>
      </div>
    );
  }

  if (!svg) {
    return (
      <div
        className="my-2 px-3 py-2 clip-hud-sm font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight"
        style={{
          border: "1px solid var(--color-line)",
          background: "rgba(94,240,255,0.03)",
        }}
      >
        ▸ renderizando mermaid…
      </div>
    );
  }

  if (size === "fill") {
    return (
      <div
        className="mermaid-block-fill w-full h-full overflow-auto p-6 flex items-center justify-center clip-hud-sm"
        style={{
          border: "1px solid rgba(94,240,255,0.22)",
          background: "rgba(0,0,0,0.38)",
        }}
        dangerouslySetInnerHTML={{ __html: svg }}
      />
    );
  }
  return (
    <div
      className="my-2 overflow-x-auto clip-hud-sm p-3 mermaid-block"
      style={{
        border: "1px solid rgba(94,240,255,0.22)",
        background: "rgba(0,0,0,0.38)",
      }}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
