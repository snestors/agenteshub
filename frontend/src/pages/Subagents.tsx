import * as React from "react";
import { GitBranch, Loader2, CheckCircle2, AlertCircle, X } from "lucide-react";
import { Topbar } from "@/components/Topbar";
import { subagentsApi, type Subagent } from "@/lib/api";

const STATUS_OPTS: Array<{ key: ""; label: string } | { key: Subagent["status"]; label: string }> = [
  { key: "", label: "todos" },
  { key: "running", label: "corriendo" },
  { key: "ok", label: "ok" },
  { key: "error", label: "error" },
  { key: "cancelled", label: "cancelado" },
];

function fmtTime(ts: number) {
  return new Date(ts * 1000).toLocaleString("es-PE", {
    hour: "2-digit",
    minute: "2-digit",
    day: "2-digit",
    month: "2-digit",
  });
}

function fmtDuration(start: number, end?: number): string {
  if (!end) return "—";
  const sec = Math.max(0, end - start);
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
}

function statusTone(s: Subagent["status"]): { color: string; Icon: React.ComponentType<{ size?: number; strokeWidth?: number; style?: React.CSSProperties; className?: string }> } {
  switch (s) {
    case "running":
      return { color: "var(--color-orange)", Icon: Loader2 };
    case "ok":
      return { color: "var(--color-lime)", Icon: CheckCircle2 };
    case "error":
      return { color: "var(--color-danger)", Icon: AlertCircle };
    default:
      return { color: "var(--color-dim)", Icon: AlertCircle };
  }
}

export function Subagents() {
  const [items, setItems] = React.useState<Subagent[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [status, setStatus] = React.useState<string>("");
  const [selected, setSelected] = React.useState<Subagent | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      const list = await subagentsApi.list(status || undefined);
      setItems(list);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error de red");
    } finally {
      setLoading(false);
    }
  }, [status]);

  React.useEffect(() => {
    void refresh();
    const t = window.setInterval(refresh, 5000);
    return () => window.clearInterval(t);
  }, [refresh]);

  const running = items.filter((i) => i.status === "running").length;

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Sub-agentes" }]}
        status={
          error
            ? { label: "OFFLINE", tone: "danger" }
            : running > 0
            ? { label: `${running} LIVE`, tone: "warn" }
            : { label: "IDLE", tone: "ok" }
        }
      />

      {/* filter bar */}
      <div className="px-4 py-3 border-b flex items-center gap-3 flex-wrap" style={{ borderColor: "var(--color-line)" }}>
        <GitBranch size={14} strokeWidth={1.6} style={{ color: "var(--color-cyan)" }} />
        <span className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]">
          status:
        </span>
        {STATUS_OPTS.map((opt) => (
          <button
            key={opt.key || "all"}
            type="button"
            onClick={() => setStatus(opt.key)}
            className={`px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors ${
              status === opt.key
                ? "text-[var(--color-fg)]"
                : "text-[var(--color-dim)] hover:text-[var(--color-fg)]"
            }`}
            style={{
              borderColor: status === opt.key ? "var(--color-cyan)" : "var(--color-line)",
              background: status === opt.key ? "rgba(94,240,255,0.08)" : "transparent",
            }}
          >
            {opt.label}
          </button>
        ))}
        <span className="ml-auto font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
          {items.length} {items.length === 1 ? "registro" : "registros"}
        </span>
      </div>

      {/* table */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            ▸ cargando…
          </div>
        ) : items.length === 0 ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            sin sub-agentes capturados todavía
          </div>
        ) : (
          <table className="w-full font-mono text-[11px]">
            <thead>
              <tr className="text-left text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] border-b" style={{ borderColor: "var(--color-line)" }}>
                <th className="px-2 py-1.5">id</th>
                <th className="px-2 py-1.5">type</th>
                <th className="px-2 py-1.5">descripción</th>
                <th className="px-2 py-1.5">parent</th>
                <th className="px-2 py-1.5">status</th>
                <th className="px-2 py-1.5">started</th>
                <th className="px-2 py-1.5">duración</th>
                <th className="px-2 py-1.5">tokens</th>
              </tr>
            </thead>
            <tbody>
              {items.map((s) => {
                const tone = statusTone(s.status);
                const SIcon = tone.Icon;
                return (
                  <tr
                    key={s.id}
                    onClick={() => setSelected(s)}
                    className="border-b cursor-pointer hover:bg-[rgba(94,240,255,0.04)]"
                    style={{ borderColor: "var(--color-line)" }}
                  >
                    <td className="px-2 py-1.5 text-[var(--color-dim)]">#{s.id}</td>
                    <td className="px-2 py-1.5" style={{ color: "var(--color-cyan)" }}>
                      {s.agent_type || "—"}
                    </td>
                    <td className="px-2 py-1.5 truncate max-w-[260px] text-[var(--color-fg)]">
                      {s.description || s.prompt?.slice(0, 80) || "—"}
                    </td>
                    <td className="px-2 py-1.5 text-[var(--color-dim)]">{s.parent_scope}</td>
                    <td className="px-2 py-1.5">
                      <span className="inline-flex items-center gap-1" style={{ color: tone.color }}>
                        <SIcon
                          size={11}
                          strokeWidth={1.8}
                          className={s.status === "running" ? "animate-spin" : ""}
                        />
                        {s.status}
                      </span>
                    </td>
                    <td className="px-2 py-1.5 text-[var(--color-dim)]">{fmtTime(s.started_at)}</td>
                    <td className="px-2 py-1.5 text-[var(--color-dim)]">{fmtDuration(s.started_at, s.finished_at)}</td>
                    <td className="px-2 py-1.5 text-[var(--color-dim)] tabular-nums">{s.cost_tokens}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {selected && <SubagentDetail subagent={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}

function SubagentDetail({ subagent, onClose }: { subagent: Subagent; onClose: () => void }) {
  const tone = statusTone(subagent.status);
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={onClose}
    >
      <div
        className="clip-hud-sm border max-w-2xl w-[90%] mx-4 max-h-[85vh] flex flex-col"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: tone.color,
          boxShadow: `0 0 24px ${tone.color}50`,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 flex items-center justify-between border-b" style={{ borderColor: tone.color + "30" }}>
          <div>
            <div className="font-display font-semibold text-[12px] uppercase tracking-hud" style={{ color: tone.color }}>
              ◂ sub-agent #{subagent.id} · {subagent.agent_type || "—"}
            </div>
            <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight mt-0.5 uppercase">
              parent · {subagent.parent_scope} · {subagent.parent_session_id.slice(0, 8)}…
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer p-1 transition-colors"
            aria-label="cerrar"
          >
            <X size={14} strokeWidth={1.8} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3 font-mono text-[11px]">
          <DetailRow label="status" value={subagent.status} accent={tone.color} />
          <DetailRow label="started" value={fmtTime(subagent.started_at)} />
          <DetailRow label="duration" value={fmtDuration(subagent.started_at, subagent.finished_at)} />
          <DetailRow label="cost" value={`${subagent.cost_tokens} tokens`} />
          {subagent.worktree_path && <DetailRow label="worktree" value={subagent.worktree_path} mono />}
          {subagent.tools_used && <DetailRow label="tools_used" value={subagent.tools_used} mono />}
          {subagent.description && (
            <Block label="descripción">
              <span className="text-[var(--color-fg)]">{subagent.description}</span>
            </Block>
          )}
          {subagent.prompt && (
            <Block label="prompt">
              <pre className="whitespace-pre-wrap break-words text-[var(--color-fg)] text-[11px] leading-[1.55]">
                {subagent.prompt}
              </pre>
            </Block>
          )}
          {subagent.result && (
            <Block label="resultado">
              <pre className="whitespace-pre-wrap break-words text-[var(--color-fg)] text-[11px] leading-[1.55]">
                {subagent.result}
              </pre>
            </Block>
          )}
        </div>
      </div>
    </div>
  );
}

function DetailRow({
  label,
  value,
  accent,
  mono,
}: {
  label: string;
  value: string;
  accent?: string;
  mono?: boolean;
}) {
  return (
    <div className="flex gap-3">
      <span className="font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] w-20 shrink-0 pt-0.5">
        {label}
      </span>
      <span
        className={mono ? "font-mono text-[10.5px] break-all" : "text-[var(--color-fg)]"}
        style={accent ? { color: accent } : undefined}
      >
        {value}
      </span>
    </div>
  );
}

function Block({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="font-mono text-[9px] uppercase tracking-hud-tight mb-1" style={{ color: "var(--color-cyan)" }}>
        ▸ {label}
      </div>
      <div
        className="px-3 py-2 clip-tag border"
        style={{
          background: "rgba(94, 240, 255, 0.03)",
          borderColor: "rgba(94, 240, 255, 0.15)",
        }}
      >
        {children}
      </div>
    </div>
  );
}
