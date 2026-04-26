import * as React from "react";
import {
  api,
  AGENT_STATUS_FALLBACK,
  ApiError,
  type AgentStatus,
} from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { EnginePicker } from "./EnginePicker";

interface StatusBarProps {
  /** transport status text — e.g. "ws · live" / "polling · 2s" */
  transportLabel?: string;
}

const POLL_MS = 5_000;
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
 * Polls /api/agent/status every 5s, falls back to safe defaults when 404.
 */
export function StatusBar({ transportLabel }: StatusBarProps) {
  const [status, setStatus] = React.useState<AgentStatus>(AGENT_STATUS_FALLBACK);
  const [pickerOpen, setPickerOpen] = React.useState(false);
  const [toast, setToast] = React.useState<string | null>(null);

  // immediate refresh helper — used after a successful engine swap
  const refreshStatus = React.useCallback(async () => {
    try {
      const next = await api.agentStatus();
      setStatus(next);
    } catch {
      // ignore — keep current
    }
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    let timer: number | null = null;

    async function tick() {
      try {
        const next = await api.agentStatus();
        if (!cancelled) setStatus(next);
      } catch (err) {
        // 404 / network — keep fallback silently
        if (!cancelled && !(err instanceof ApiError && err.status === 404)) {
          // also keep fallback for any error — UX must not break
        }
      } finally {
        if (!cancelled) {
          timer = window.setTimeout(tick, POLL_MS);
        }
      }
    }

    void tick();
    return () => {
      cancelled = true;
      if (timer !== null) window.clearTimeout(timer);
    };
  }, []);

  // PREVIEW for backend task #33 — once /ws starts pushing `agent_status`
  // envelopes, this listener will keep the badge in sync without waiting for
  // the next 5s poll. The polling above keeps running until #33 ships; when
  // it does, polling can be removed.
  useTopic<AgentStatus>("agent_status", (payload) => {
    if (payload && typeof payload === "object") {
      setStatus(payload);
    }
  });

  // auto-dismiss toast
  React.useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), TOAST_MS);
    return () => window.clearTimeout(t);
  }, [toast]);

  const ctxPctStr = fmtPct(status.ctx_pct ?? 0);
  const ctxClr = ctxColor(status.ctx_pct ?? 0);
  const ctxUsedStr = fmtTokens(status.ctx_used ?? 0);
  const engineBadge = `${status.engine} · ${status.model} · ${fmtCtxWindow(
    status.ctx_window
  )}`;
  const hasUsage = !!status.usage_calculated_at;

  function handleApplied(engine: string, model: string) {
    setPickerOpen(false);
    setToast(`engine cambiado a ${engine} · ${model}`);
    // Optimistic update + immediate fetch to reflect ctx_window etc.
    setStatus((s) => ({ ...s, engine, model }));
    void refreshStatus();
  }

  return (
    <div
      className="relative flex items-center gap-3 px-4 py-1.5 font-mono text-[10px] tracking-hud-tight border-t border-[var(--color-line)] select-none"
      style={{
        background: "rgba(0, 0, 0, 0.55)",
        minHeight: 26,
      }}
    >
      {/* engine · model · ctx-window — clickable badge */}
      <div className="relative">
        <button
          type="button"
          onClick={() => setPickerOpen((v) => !v)}
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
            onApplied={handleApplied}
            onClose={() => setPickerOpen(false)}
          />
        ) : null}
      </div>

      <span className="text-[var(--color-dim)]">·</span>

      {/* ctx:N% — contexto consumido por el último turn (input_tokens del JSONL) */}
      <span
        style={{ color: ctxClr }}
        title={`contexto consumido del último turn: ${ctxUsedStr} / ${fmtCtxWindow(status.ctx_window)}`}
      >
        ctx:{ctxPctStr}
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

      <span className="ml-auto text-[var(--color-dim)]">
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
