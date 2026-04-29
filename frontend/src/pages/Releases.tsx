import * as React from "react";
import { Topbar } from "@/components/Topbar";
import { api } from "@/lib/api";

const UI_VERSION = __APP_VERSION__;

export function Releases() {
  const [data, setData] = React.useState<{
    content: string;
    version: string;
    git_commit: string;
  } | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    api.releases()
      .then(setData)
      .catch((e) => setError(String(e)));
  }, []);

  const serverVersion = data?.version ?? null;
  const mismatch = serverVersion !== null && serverVersion !== UI_VERSION;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <Topbar
        breadcrumb={[{ label: "Releases" }]}
        status={
          serverVersion
            ? mismatch
              ? { label: `server v${serverVersion} / ui v${UI_VERSION}`, tone: "warn" }
              : { label: `v${serverVersion} (${data?.git_commit})`, tone: "ok" }
            : { label: "cargando...", tone: "warn" }
        }
      />

      <div className="flex-1 overflow-y-auto px-3 py-4 sm:px-6 sm:py-6">
        {error && (
          <p className="font-mono text-sm text-[var(--color-danger)]">{error}</p>
        )}

        {/* Version comparison banner when mismatch */}
        {mismatch && (
          <div
            className="mb-6 px-4 py-3 border border-[var(--color-orange)] font-mono text-xs"
            style={{ background: "rgba(255,159,67,0.06)" }}
          >
            <span style={{ color: "var(--color-orange)" }}>VERSIÓN DESINCRONIZADA</span>
            <span className="text-[var(--color-dim)] ml-3">
              server <span style={{ color: "var(--color-fg)" }}>v{serverVersion}</span>
              {" · "}
              ui <span style={{ color: "var(--color-fg)" }}>v{UI_VERSION}</span>
              {" — recargá la página para actualizar la UI"}
            </span>
          </div>
        )}

        {data && <ReleaseNotes content={data.content} currentVersion={serverVersion} />}
      </div>
    </div>
  );
}

function ReleaseNotes({ content, currentVersion }: { content: string; currentVersion: string | null }) {
  // Split by H2 sections (## v...)
  const sections = content.split(/^(?=## )/m).filter(Boolean);

  return (
    <div className="flex max-w-2xl flex-col gap-4 sm:gap-6">
      {sections.map((section, i) => {
        const lines = section.trimEnd().split("\n");
        const heading = lines[0].replace(/^#+\s*/, "").trim();
        const body = lines.slice(1).join("\n").trim();
        const isVersionSection = /^v\d+\.\d+\.\d+/.test(heading);
        const versionMatch = heading.match(/^v(\d+\.\d+\.\d+)/);
        const isCurrent = currentVersion !== null && versionMatch && versionMatch[1] === currentVersion;

        if (!isVersionSection) {
          // Header / meta section (title, path, Unreleased)
          return (
            <div key={i} className="font-mono text-xs text-[var(--color-dim)]">
              {body && <MarkdownBody content={body} />}
            </div>
          );
        }

        return (
          <div
            key={i}
            className="border border-[var(--color-line)] relative"
            style={isCurrent ? { borderColor: "var(--color-lime)" } : {}}
          >
            {/* Section header */}
            <div
              className="flex flex-col items-start gap-2 px-3 py-2 border-b border-[var(--color-line)] sm:flex-row sm:items-center sm:justify-between sm:px-4"
              style={isCurrent ? {
                borderColor: "var(--color-lime)",
                background: "rgba(163,255,78,0.04)",
              } : {}}
            >
              <span
                className="font-mono text-sm tracking-hud-tight"
                style={{ color: isCurrent ? "var(--color-lime)" : "var(--color-fg)" }}
              >
                {heading}
              </span>
              {isCurrent && (
                <span
                  className="font-mono text-[9px] tracking-hud-tight uppercase px-2 py-0.5"
                  style={{
                    color: "var(--color-lime)",
                    border: "1px solid var(--color-lime)",
                    background: "rgba(163,255,78,0.08)",
                  }}
                >
                  RUNNING
                </span>
              )}
            </div>

            {/* Body */}
            <div className="px-3 py-3 font-mono text-xs text-[var(--color-dim)] leading-relaxed sm:px-4">
              <MarkdownBody content={body} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

function MarkdownBody({ content }: { content: string }) {
  const lines = content.split("\n");

  return (
    <div className="flex flex-col gap-1">
      {lines.map((line, i) => {
        if (/^###\s/.test(line)) {
          return (
            <p key={i} className="mt-2 text-[10px] uppercase tracking-hud-tight" style={{ color: "var(--color-cyan)" }}>
              {line.replace(/^###\s*/, "")}
            </p>
          );
        }
        if (/^##\s/.test(line)) return null;
        if (/^-\s/.test(line)) {
          // Bold the **text** segments
          const text = line.replace(/^-\s*/, "");
          return (
            <div key={i} className="flex gap-2">
              <span style={{ color: "var(--color-dim)" }}>·</span>
              <span dangerouslySetInnerHTML={{ __html: renderInline(text) }} />
            </div>
          );
        }
        if (line.trim() === "" || line.trim() === "---") return <div key={i} className="h-1" />;
        return (
          <p key={i} dangerouslySetInnerHTML={{ __html: renderInline(line) }} />
        );
      })}
    </div>
  );
}

function renderInline(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\*\*(.+?)\*\*/g, `<span style="color:var(--color-fg)">$1</span>`)
    .replace(/`(.+?)`/g, `<code style="color:var(--color-cyan);background:rgba(94,240,255,0.08);padding:0 3px">$1</code>`);
}
