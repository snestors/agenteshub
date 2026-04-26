import * as React from "react";
import {
  api,
  AGENT_STATUS_FALLBACK,
  ApiError,
  type AgentStatus,
} from "@/lib/api";

interface StatusBarProps {
  /** transport status text — e.g. "ws · live" / "polling · 2s" */
  transportLabel?: string;
}

const POLL_MS = 5_000;

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

  const ctxPctStr = fmtPct(status.ctx_pct ?? 0);
  const ctxClr = ctxColor(status.ctx_pct ?? 0);
  const ctxUsedStr = fmtTokens(status.ctx_used ?? 0);
  const engineBadge = `${status.engine} · ${status.model} · ${fmtCtxWindow(
    status.ctx_window
  )}`;

  return (
    <div
      className="relative flex items-center gap-3 px-4 py-1.5 font-mono text-[10px] tracking-hud-tight border-t border-[var(--color-line)] select-none"
      style={{
        background: "rgba(0, 0, 0, 0.55)",
        minHeight: 26,
      }}
    >
      {/* engine · model · ctx-window */}
      <span
        className="inline-flex items-center px-2 py-0.5 clip-tag"
        style={{
          background: "rgba(94, 240, 255, 0.10)",
          border: "1px solid rgba(94, 240, 255, 0.45)",
          color: "var(--color-cyan)",
        }}
        title="engine · model · context window"
      >
        [{engineBadge}]
      </span>

      <span className="text-[var(--color-dim)]">·</span>

      {/* ctx:N% — contexto consumido por el último turn (input_tokens del JSONL) */}
      <span
        style={{ color: ctxClr }}
        title={`contexto consumido del último turn: ${ctxUsedStr} / ${fmtCtxWindow(status.ctx_window)}`}
      >
        ctx:{ctxPctStr}
      </span>

      <span className="ml-auto text-[var(--color-dim)]">
        {transportLabel ?? ""}
      </span>
    </div>
  );
}
