import * as React from "react";
import { AGENT_STATUS_FALLBACK, api, type AgentStatus } from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { EnginePicker } from "./EnginePicker";

interface StatusBarProps {
  /** transport status text — e.g. "ws · live" / "ws · reconnect…" */
  transportLabel?: string;
}

const TOAST_MS = 2_000;

function fmtCtxWindow(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(0)}M ctx`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}K ctx`;
  return `${n} ctx`;
}

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(2)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return `${n}`;
}

function fmtPct(pct: number): string {
  // pct in [0..1]
  return `${Math.round(pct * 100)}%`;
}

function ctxColor(pct: number): string {
  if (pct >= 0.8) return "var(--color-danger)";
  if (pct >= 0.5) return "var(--color-warn)";
  return "var(--color-cyan)";
}

/**
 * StatusBar — Claude Code CLI-style status line.
 */
export function StatusBar({ transportLabel }: StatusBarProps) {
  const [status, setStatus] = React.useState<AgentStatus>(AGENT_STATUS_FALLBACK);
  const [pickerOpen, setPickerOpen] = React.useState(false);
  const [pickerAnchor, setPickerAnchor] = React.useState<DOMRect | null>(null);
  const [toast, setToast] = React.useState<string | null>(null);
  const engineButtonRef = React.useRef<HTMLButtonElement | null>(null);

  // Agent status is pushed by WS and refreshed by the server heartbeat.
  useTopic<AgentStatus>("agent_status", (payload) => {
    if (payload && typeof payload === "object") {
      setStatus(payload);
    }
  });

  // Fetch initial status via HTTP so the bar shows real data immediately,
  // without waiting for the WS connect → subscribe → broadcast round-trip.
  React.useEffect(() => {
    api.agentStatus().then(setStatus).catch(() => {});
  }, []);

  // auto-dismiss toast
  React.useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), TOAST_MS);
    return () => window.clearTimeout(t);
  }, [toast]);

  const ctxPctStr = fmtPct(status.ctx_pct ?? 0);
  const ctxClr = ctxColor(status.ctx_pct ?? 0);
  const ctxUsedStr = fmtTokens(status.ctx_used ?? 0);
  const ctxWindowStr = fmtCtxWindow(status.ctx_window);
  const engineBadge = `${status.engine} · ${status.model} · ${fmtCtxWindow(
    status.ctx_window
  )}`;
  const hasUsage = !!status.usage_calculated_at;

  function handleApplied(engine: string, model: string, ctxWindow?: number) {
    setPickerOpen(false);
    setPickerAnchor(null);
    setToast(`engine cambiado a ${engine} · ${model}`);
    setStatus((s) => ({ ...s, engine, model, ctx_window: ctxWindow ?? s.ctx_window }));
  }

  function togglePicker() {
    setPickerAnchor(engineButtonRef.current?.getBoundingClientRect() ?? null);
    setPickerOpen((v) => !v);
  }

  function closePicker() {
    setPickerOpen(false);
    setPickerAnchor(null);
  }

  React.useEffect(() => {
    if (!pickerOpen) return;
    const updateAnchor = () => {
      setPickerAnchor(engineButtonRef.current?.getBoundingClientRect() ?? null);
    };
    window.addEventListener("resize", updateAnchor);
    window.addEventListener("scroll", updateAnchor, true);
    return () => {
      window.removeEventListener("resize", updateAnchor);
      window.removeEventListener("scroll", updateAnchor, true);
    };
  }, [pickerOpen]);

  return (
    <div
      className="relative flex items-center gap-2 overflow-x-auto whitespace-nowrap px-2 py-2 font-mono text-[10px] tracking-hud-tight border-t border-[var(--color-line)] select-none sm:gap-3 sm:px-4 sm:py-1.5"
      style={{
        background: "rgba(0, 0, 0, 0.55)",
        minHeight: 26,
      }}
    >
      {/* engine · model · ctx-window — clickable badge */}
      <div className="relative">
        <button
          ref={engineButtonRef}
          type="button"
          onClick={togglePicker}
          className="inline-flex items-center px-2 py-0.5 clip-tag cursor-pointer hover:opacity-80 transition-opacity"
          style={{
            background: "rgba(94, 240, 255, 0.10)",
            border: "1px solid rgba(94, 240, 255, 0.45)",
            color: "var(--color-cyan)",
            font: "inherit",
            letterSpacing: "inherit",
          }}
          title="cambiar engine · model"
          aria-haspopup="dialog"
          aria-expanded={pickerOpen}
        >
          [{engineBadge}]
        </button>

        {pickerOpen ? (
          <EnginePicker
            currentEngine={status.engine}
            currentModel={status.model}
            anchorRect={pickerAnchor}
            onApplied={handleApplied}
            onClose={closePicker}
          />
        ) : null}
      </div>

      <span className="text-[var(--color-dim)]">·</span>

      {/* contexto consumido por el último turn (input_tokens del JSONL) */}
      <span
        className="inline-flex items-center gap-1 px-2 py-0.5 clip-tag"
        style={{
          color: ctxClr,
          background: "rgba(255,255,255,0.04)",
          border: `1px solid ${ctxClr}`,
        }}
        title={`contexto consumido del último turn: ${ctxUsedStr} / ${ctxWindowStr}`}
      >
        <span>ctx</span>
        <span>{ctxPctStr}</span>
        <span className="hidden sm:inline text-[var(--color-dim)]">
          · {ctxUsedStr}/{ctxWindowStr}
        </span>
      </span>

      {/* plan badge — leído de ~/.claude/.credentials.json */}
      {status.plan ? (
        <>
          <span className="text-[var(--color-dim)]">·</span>
          <span
            className="inline-flex items-center px-2 py-0.5 clip-tag"
            style={{
              background: "rgba(163, 255, 78, 0.10)",
              border: "1px solid rgba(163, 255, 78, 0.45)",
              color: "var(--color-lime)",
            }}
            title={`plan ${status.plan}${status.plan_tier ? ` · ${status.plan_tier}` : ""}`}
          >
            plan · {status.plan}
            {status.plan_tier?.includes("5x") ? " 5x" : ""}
          </span>
        </>
      ) : null}

      {hasUsage ? (
        <>
          <span className="text-[var(--color-dim)]">·</span>
          <span
            className="inline-flex items-center px-2 py-0.5 clip-tag"
            style={{
              background: "rgba(94, 240, 255, 0.08)",
              border: "1px solid rgba(94, 240, 255, 0.30)",
              color: "var(--color-cyan)",
            }}
            title={`uso local estimado desde JSONL · sesión ${fmtTokens(
              status.usage_session_tokens ?? 0
            )} tokens · semana ${fmtTokens(status.usage_week_tokens ?? 0)} tokens`}
          >
            sesión {fmtPct(status.usage_session_pct ?? 0)} · semana{" "}
            {fmtPct(status.usage_week_pct ?? 0)}
          </span>
        </>
      ) : null}

      <span className="shrink-0 text-[var(--color-dim)] sm:ml-auto">
        {transportLabel ?? ""}
      </span>

      {/* toast — bottom-right floating notice */}
      {toast ? (
        <div
          className="fixed bottom-10 right-4 z-50 px-3 py-1.5 clip-tag font-mono text-[11px] tracking-hud-tight"
          style={{
            background: "rgba(10, 15, 36, 0.95)",
            border: "1px solid rgba(94, 240, 255, 0.65)",
            color: "var(--color-cyan)",
            boxShadow: "0 0 14px rgba(94, 240, 255, 0.45)",
          }}
          role="status"
        >
          {toast}
        </div>
      ) : null}
    </div>
  );
}
