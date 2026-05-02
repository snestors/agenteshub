// ChatHUD — lateral right panel with the sections from the Claude Design
// handoff. Mounted by ChatMain (main-agent) and ProjectChat (project-agent).
// The chat caller is responsible for fetching data and passing it down; the
// HUD is presentational so we can mock + test it without spinning up the full
// conversation graph.
//
// Sections, in order (revised after first user feedback):
//   1. ENGINE · TOKENS — engine + model + ctx + the big tokens-session number
//                        and bar (combined; the user wanted the tokens row up
//                        top alongside the engine info)
//   2. AGENTS·RUNTIME  — counts for main + project running turns
//   3. FEATURE_LIST    — only when scope=project; reads feature_list.json
//   4. SUBS · 5H       — Anthropic subscription window (radial + countdown)
//   5. STACK           — tools seen + topic in context
//   6. SESSION         — id, scope, started, status, ws (kept for forensics
//                        but moved to the bottom; rarely the first thing the
//                        user wants to see)
//
// Collapsable. Persistence in localStorage(`agenthub.hud.collapsed`). Cascade
// fade-in via .anim-hud-stagger on the sections wrapper (animations defined
// in index.css).
import * as React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import type { AgentStatus, ConversationRuntime, FeatureItem, ProjectFeatures, RealtimeResponse } from "@/lib/api";

const COLLAPSE_KEY = "agenthub.hud.collapsed";

export interface ChatHUDProps {
  scope: "main" | "project";
  scopeKey?: string; // "main" or "project:<id>:<sid>"

  // Live data — caller passes whatever it has; nulls render placeholders.
  runtime?: ConversationRuntime | null;
  status?: AgentStatus | null;
  realtimeUsage?: RealtimeResponse | null;
  projectFeatures?: ProjectFeatures | null;

  // Hub-derived counts (caller subscribes to ws/system stats).
  runningMain?: number;
  runningProject?: number;
  wsConnected?: boolean;

  // Optional CTA: when scope=project and feature_list missing, the HUD shows
  // a "scaffold harness" button. Caller wires it.
  onScaffoldHarness?: () => void;
}

export function ChatHUD(props: ChatHUDProps) {
  const [collapsed, setCollapsed] = React.useState<boolean>(() => {
    try { return localStorage.getItem(COLLAPSE_KEY) === "1"; } catch { return false; }
  });
  const toggle = React.useCallback(() => {
    setCollapsed((c) => {
      const n = !c;
      try { localStorage.setItem(COLLAPSE_KEY, n ? "1" : "0"); } catch { /* ignore */ }
      return n;
    });
  }, []);

  if (collapsed) {
    return (
      <button
        type="button"
        onClick={toggle}
        title="Expand HUD"
        aria-label="Expand HUD"
        className="cursor-pointer flex items-center justify-center clip-tag"
        style={{
          // flexShrink:0 evita que el flex padre lo aplaste a 0 cuando hay
          // contenido grande; sin esto, el botón se "comía" en /projects.
          flexShrink: 0,
          width: 24,
          minHeight: "100%",
          background: "rgba(12,18,40,0.55)",
          borderLeft: "1px solid var(--color-line)",
          color: "var(--color-cyan)",
        }}
      >
        <ChevronLeft size={14} />
      </button>
    );
  }

  return (
    <aside
      className="flex flex-col min-h-0"
      style={{
        width: 320,
        flexShrink: 0,
        borderLeft: "1px solid var(--color-line)",
        background: "rgba(12,18,40,0.45)",
      }}
    >
      <div
        className="flex items-center justify-between px-3 py-2"
        style={{ borderBottom: "1px solid var(--color-line)" }}
      >
        <span className="font-display text-[11px] tracking-hud uppercase" style={{ color: "var(--color-cyan)" }}>
          HUD · {props.scope === "main" ? "Main" : "Project"}
        </span>
        <button
          type="button"
          onClick={toggle}
          title="Collapse HUD"
          aria-label="Collapse HUD"
          className="cursor-pointer"
          style={{ color: "var(--color-dim)" }}
        >
          <ChevronRight size={14} />
        </button>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto p-3 anim-hud-stagger flex flex-col gap-3">
        <EngineTokensSection status={props.status ?? null} runtime={props.runtime ?? null} />
        <AgentsRuntimeSection
          runningMain={props.runningMain ?? 0}
          runningProject={props.runningProject ?? 0}
        />
        {props.scope === "project" && (
          <FeatureListSection
            features={props.projectFeatures ?? null}
            onScaffold={props.onScaffoldHarness}
          />
        )}
        <SubsWindowSection realtime={props.realtimeUsage ?? null} />
        <StackSection runtime={props.runtime ?? null} />
        <SessionSection
          scope={props.scope}
          scopeKey={props.scopeKey}
          runtime={props.runtime ?? null}
          wsConnected={props.wsConnected ?? false}
        />
      </div>
    </aside>
  );
}

// ─── helper components ────────────────────────────────────────────────

function HudSection({ title, accent = "cyan", children }: { title: string; accent?: "cyan" | "magenta" | "lime" | "orange"; children: React.ReactNode }) {
  const accentColor = `var(--color-${accent})`;
  return (
    <div
      className="clip-hud-sm"
      style={{
        border: "1px solid var(--color-line)",
        background: "rgba(12,18,40,0.55)",
        padding: "10px 12px",
      }}
    >
      <div
        className="font-display text-[10px] tracking-hud uppercase mb-2"
        style={{ color: accentColor }}
      >
        {title}
      </div>
      {children}
    </div>
  );
}

function Row({ label, value, mono = true }: { label: string; value?: string | number; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center text-[10px] py-0.5">
      <span style={{ color: "var(--color-dim)", letterSpacing: "0.08em", textTransform: "uppercase" }}>
        {label}
      </span>
      <span
        style={{
          color: "var(--color-fg)",
          fontFamily: mono ? "var(--font-mono)" : undefined,
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {value ?? "—"}
      </span>
    </div>
  );
}

function SessionSection({
  scope,
  scopeKey,
  runtime,
  wsConnected,
}: {
  scope: string;
  scopeKey?: string;
  runtime: ConversationRuntime | null;
  wsConnected: boolean;
}) {
  const sessId = runtime?.session_id ?? "";
  const short = sessId ? sessId.slice(0, 8) + "…" : "—";
  const started = runtime?.started_at ? relTime(runtime.started_at) : "—";
  return (
    <HudSection title="Session" accent="cyan">
      <Row label="scope" value={scope} />
      <Row label="key" value={scopeKey ?? "—"} />
      <Row label="session id" value={short} />
      <Row label="started" value={started} />
      <Row label="status" value={runtime?.status ?? "idle"} />
      <Row label="ws" value={wsConnected ? "connected" : "off"} />
    </HudSection>
  );
}

// EngineTokensSection — was two separate cards (Engine + Tokens) until the
// user asked to merge them so the headline "tokens session" number sits next
// to the engine + model that produced it. Layout: top row engine/model/ctx,
// then the big number + progress bar.
function EngineTokensSection({ status, runtime }: { status: AgentStatus | null; runtime: ConversationRuntime | null }) {
  const engine = runtime?.engine ?? status?.engine ?? "—";
  const model = runtime?.model ?? status?.model ?? "—";
  const ctxWindow = status?.ctx_window ?? 0;
  const tokens = status?.usage_session_tokens ?? 0;
  const pct = Math.round(((status?.usage_session_pct ?? 0) * 100));
  return (
    <HudSection title="Engine · Tokens" accent="cyan">
      <Row label="engine" value={engine} />
      <Row label="model" value={model} />
      <Row label="ctx window" value={ctxWindow ? ctxWindow.toLocaleString() : "—"} />
      <div
        className="mt-3 pt-2"
        style={{ borderTop: "1px solid var(--color-line)" }}
      >
        <div className="text-[9px] tracking-hud uppercase mb-1" style={{ color: "var(--color-dim)" }}>
          tokens · session
        </div>
        <div
          className="font-display text-[22px] font-bold leading-none mb-1"
          style={{
            color: "var(--color-cyan)",
            textShadow: "0 0 8px rgba(94,240,255,0.45)",
          }}
        >
          {tokens.toLocaleString()}
        </div>
        <div className="bar mb-1" style={{ height: 4, background: "rgba(120,255,220,0.08)" }}>
          <span style={{ display: "block", height: "100%", width: `${pct}%`, background: "var(--color-cyan)" }} />
        </div>
        <Row label="usage" value={`${pct}%`} />
      </div>
    </HudSection>
  );
}

function FeatureListSection({
  features,
  onScaffold,
}: {
  features: ProjectFeatures | null;
  onScaffold?: () => void;
}) {
  if (!features) {
    return (
      <HudSection title="feature_list.json" accent="lime">
        <div className="text-[10px]" style={{ color: "var(--color-dim)" }}>cargando…</div>
      </HudSection>
    );
  }
  if (!features.exists) {
    return (
      <HudSection title="feature_list.json" accent="lime">
        <div className="text-[10px] mb-2" style={{ color: "var(--color-dim)" }}>
          sin harness scaffoldado
        </div>
        {onScaffold && (
          <button
            type="button"
            onClick={onScaffold}
            className="clip-tag cursor-pointer text-[9px] tracking-hud uppercase"
            style={{
              padding: "4px 10px",
              border: "1px solid var(--color-lime)",
              color: "var(--color-lime)",
              background: "rgba(163,255,78,0.08)",
            }}
          >
            scaffold harness
          </button>
        )}
      </HudSection>
    );
  }
  const counts = countByStatus(features.features);
  return (
    <HudSection title="feature_list.json" accent="lime">
      <div className="flex gap-2 mb-2 flex-wrap">
        <Counter label="pending" value={counts.pending} accent="cyan" />
        <Counter label="in progress" value={counts.in_progress} accent="orange" />
        <Counter label="done" value={counts.done} accent="lime" />
        <Counter label="blocked" value={counts.blocked} accent="danger" />
      </div>
      <div className="flex flex-col gap-1 max-h-48 overflow-y-auto">
        {features.features.slice(0, 12).map((f) => (
          <FeatureRow key={f.id} f={f} />
        ))}
        {features.features.length > 12 && (
          <div className="text-[9px] mt-1" style={{ color: "var(--color-dim)" }}>
            +{features.features.length - 12} más
          </div>
        )}
      </div>
    </HudSection>
  );
}

function Counter({ label, value, accent }: { label: string; value: number; accent: "cyan" | "magenta" | "lime" | "orange" | "danger" }) {
  const colorVar = accent === "danger" ? "var(--color-danger)" : `var(--color-${accent})`;
  return (
    <div className="flex flex-col items-start clip-tag" style={{ padding: "2px 8px", border: `1px solid ${colorVar}`, background: "rgba(255,255,255,0.02)" }}>
      <span className="font-display text-[12px] font-bold" style={{ color: colorVar }}>{value}</span>
      <span className="text-[8px] tracking-hud uppercase" style={{ color: "var(--color-dim)" }}>{label}</span>
    </div>
  );
}

function FeatureRow({ f }: { f: FeatureItem }) {
  const accent =
    f.status === "done"        ? "var(--color-lime)" :
    f.status === "in_progress" ? "var(--color-orange)" :
    f.status === "blocked"     ? "var(--color-danger)" :
                                 "var(--color-cyan)";
  return (
    <div className="flex items-center gap-2 text-[10px]">
      <span
        className="font-mono shrink-0"
        style={{ color: accent, width: 36 }}
      >
        {f.id}
      </span>
      <span className="flex-1 truncate" style={{ color: "var(--color-fg)" }} title={f.name}>
        {f.name}
      </span>
    </div>
  );
}

function SubsWindowSection({ realtime }: { realtime: RealtimeResponse | null }) {
  const claudeWindow = realtime?.claude?.session;
  if (!claudeWindow) {
    return (
      <HudSection title="Subs · 5H Window" accent="orange">
        <div className="text-[10px]" style={{ color: "var(--color-dim)" }}>sin datos</div>
      </HudSection>
    );
  }
  const pct = Math.round(claudeWindow.percent_used ?? 0);
  const reset = claudeWindow.reset_at ? relTime(claudeWindow.reset_at, /*future*/ true) : "—";
  return (
    <HudSection title="Subs · 5H Window" accent="orange">
      <div className="flex items-center gap-3 mb-1">
        <RadialPct pct={pct} />
        <div className="flex-1">
          <div className="font-display text-[16px] font-bold" style={{ color: "var(--color-orange)" }}>
            {reset}
          </div>
          <div className="text-[9px] tracking-hud uppercase" style={{ color: "var(--color-dim)" }}>hasta reset</div>
        </div>
      </div>
      <Row label="session pct" value={`${pct}%`} />
    </HudSection>
  );
}

function RadialPct({ pct }: { pct: number }) {
  const r = 18;
  const c = 2 * Math.PI * r;
  const off = c - (c * pct) / 100;
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" style={{ flexShrink: 0 }}>
      <circle cx="24" cy="24" r={r} stroke="var(--color-line)" strokeWidth="3" fill="none" />
      <circle
        cx="24" cy="24" r={r}
        stroke="var(--color-orange)" strokeWidth="3" fill="none"
        strokeDasharray={c}
        strokeDashoffset={off}
        strokeLinecap="round"
        transform="rotate(-90 24 24)"
        style={{ filter: "drop-shadow(0 0 4px rgba(255,159,67,0.6))" }}
      />
      <text x="24" y="28" textAnchor="middle" fontFamily="var(--font-display)" fontSize="11" fontWeight="700" fill="var(--color-fg)">
        {pct}%
      </text>
    </svg>
  );
}

function AgentsRuntimeSection({
  runningMain,
  runningProject,
}: {
  runningMain: number;
  runningProject: number;
}) {
  return (
    <HudSection title="Agents · Runtime" accent="magenta">
      <div className="flex gap-2">
        <div className="flex-1 clip-hud-sm" style={{ padding: "8px 10px", border: "1px solid var(--color-cyan)", background: "rgba(94,240,255,0.06)" }}>
          <div className="text-[9px] tracking-hud uppercase" style={{ color: "var(--color-dim)" }}>main</div>
          <div className="font-display text-[18px] font-bold" style={{ color: "var(--color-cyan)" }}>
            {runningMain}
          </div>
        </div>
        <div className="flex-1 clip-hud-sm" style={{ padding: "8px 10px", border: "1px solid var(--color-magenta)", background: "rgba(255,78,214,0.06)" }}>
          <div className="text-[9px] tracking-hud uppercase" style={{ color: "var(--color-dim)" }}>project</div>
          <div className="font-display text-[18px] font-bold" style={{ color: "var(--color-magenta)" }}>
            {runningProject}
          </div>
        </div>
      </div>
    </HudSection>
  );
}

function StackSection({ runtime }: { runtime: ConversationRuntime | null }) {
  // The runtime payload carries the activity_json which contains tool_use
  // entries with names. We surface the unique tool names + topic.
  const tools = pickActivityTools(runtime);
  return (
    <HudSection title="Stack" accent="lime">
      <Row label="topic" value={runtime?.topic || "—"} />
      <Row label="tools" value={tools.length} />
      {tools.length > 0 && (
        <div className="mt-2 flex gap-1 flex-wrap">
          {tools.slice(0, 8).map((t, i) => (
            <span
              key={i}
              className="text-[9px] clip-tag"
              style={{
                padding: "2px 6px",
                border: "1px solid var(--color-line)",
                color: "var(--color-dim)",
                background: "rgba(255,255,255,0.02)",
              }}
            >
              {t}
            </span>
          ))}
          {tools.length > 8 && (
            <span className="text-[9px]" style={{ color: "var(--color-dim)" }}>+{tools.length - 8}</span>
          )}
        </div>
      )}
    </HudSection>
  );
}

// ─── helpers ─────────────────────────────────────────────────────────

function countByStatus(features: FeatureItem[]) {
  const out = { pending: 0, in_progress: 0, done: 0, blocked: 0 };
  for (const f of features) {
    if (f.status in out) (out as Record<string, number>)[f.status]++;
  }
  return out;
}

function pickActivityTools(runtime: ConversationRuntime | null): string[] {
  const tools = runtime?.tools ?? [];
  const names = tools.map((t) => t.name).filter((n): n is string => !!n);
  return Array.from(new Set(names));
}

function relTime(unix: number, future = false): string {
  if (!unix) return "—";
  const now = Math.floor(Date.now() / 1000);
  const diff = future ? unix - now : now - unix;
  if (diff < 0) return "—";
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) {
    const h = Math.floor(diff / 3600);
    const m = Math.floor((diff % 3600) / 60);
    return m > 0 ? `${h}h ${m}m` : `${h}h`;
  }
  return `${Math.floor(diff / 86400)}d`;
}
