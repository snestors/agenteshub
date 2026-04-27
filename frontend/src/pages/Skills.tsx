import * as React from "react";
import { Sparkles, RefreshCcw, X } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Topbar } from "@/components/Topbar";
import { skillsApi, type Skill, type SkillSyncResult } from "@/lib/api";

function fmtRelative(ts?: number): string {
  if (!ts) return "—";
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
  return `${Math.floor(sec / 86400)}d`;
}

export function Skills() {
  const [items, setItems] = React.useState<Skill[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [filter, setFilter] = React.useState<string>("");
  const [selected, setSelected] = React.useState<Skill | null>(null);
  const [syncing, setSyncing] = React.useState(false);
  const [lastSync, setLastSync] = React.useState<SkillSyncResult | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      const list = await skillsApi.list();
      setItems(list);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error de red");
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  async function handleSync() {
    setSyncing(true);
    try {
      const res = await skillsApi.sync();
      setLastSync(res);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error sincronizando");
    } finally {
      setSyncing(false);
    }
  }

  const sources = React.useMemo(() => {
    const seen = new Set<string>();
    items.forEach((i) => seen.add(i.source));
    return Array.from(seen).sort();
  }, [items]);

  const filtered = filter ? items.filter((i) => i.source === filter) : items;

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Skills" }]}
        status={
          error
            ? { label: "OFFLINE", tone: "danger" }
            : { label: `${items.length} SKILLS`, tone: "ok" }
        }
      />

      <div
        className="px-4 py-3 border-b flex items-center gap-3 flex-wrap"
        style={{ borderColor: "var(--color-line)" }}
      >
        <Sparkles size={14} strokeWidth={1.6} style={{ color: "var(--color-cyan)" }} />
        <span className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]">
          registro · {items.length} skills · {sources.length} sources
        </span>

        <div className="flex gap-1">
          <SourceChip
            label="todos"
            active={filter === ""}
            onClick={() => setFilter("")}
          />
          {sources.map((s) => (
            <SourceChip
              key={s}
              label={s}
              active={filter === s}
              onClick={() => setFilter(s)}
            />
          ))}
        </div>

        <button
          type="button"
          onClick={() => void handleSync()}
          disabled={syncing}
          className="ml-auto px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors disabled:opacity-60"
          style={{
            color: "var(--color-cyan)",
            borderColor: "var(--color-cyan)",
            background: "rgba(94,240,255,0.06)",
          }}
        >
          <RefreshCcw size={11} strokeWidth={1.8} className={`inline mr-1 ${syncing ? "animate-spin" : ""}`} />
          {syncing ? "sincronizando…" : "sync now"}
        </button>
      </div>

      {lastSync && (
        <div
          className="px-4 py-2 border-b font-mono text-[10px] text-[var(--color-dim)]"
          style={{ borderColor: "var(--color-line)" }}
        >
          ▸ último sync: {lastSync.total_upserted} upserted · {lastSync.total_removed} removed ·{" "}
          {lastSync.sources.length} sources
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            ▸ cargando…
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            sin skills aún. correlas con "sync now"
          </div>
        ) : (
          <table className="w-full font-mono text-[11px]">
            <thead>
              <tr
                className="text-left text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] border-b"
                style={{ borderColor: "var(--color-line)" }}
              >
                <th className="px-2 py-1.5">name</th>
                <th className="px-2 py-1.5">source</th>
                <th className="px-2 py-1.5">version</th>
                <th className="px-2 py-1.5">role hint</th>
                <th className="px-2 py-1.5">descripción</th>
                <th className="px-2 py-1.5">actualizado</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((s) => (
                <tr
                  key={`${s.source}/${s.name}`}
                  onClick={() => setSelected(s)}
                  className="border-b cursor-pointer hover:bg-[rgba(94,240,255,0.04)]"
                  style={{ borderColor: "var(--color-line)" }}
                >
                  <td className="px-2 py-1.5 font-semibold" style={{ color: "var(--color-cyan)" }}>
                    {s.name}
                  </td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{s.source}</td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{s.version || "—"}</td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{s.role_hint || "—"}</td>
                  <td className="px-2 py-1.5 text-[var(--color-fg)] truncate max-w-[400px]">
                    {s.description || "—"}
                  </td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{fmtRelative(s.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {selected && <SkillDetail skill={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}

function SourceChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors ${
        active ? "text-[var(--color-fg)]" : "text-[var(--color-dim)] hover:text-[var(--color-fg)]"
      }`}
      style={{
        borderColor: active ? "var(--color-cyan)" : "var(--color-line)",
        background: active ? "rgba(94,240,255,0.08)" : "transparent",
      }}
    >
      {label}
    </button>
  );
}

function SkillDetail({ skill, onClose }: { skill: Skill; onClose: () => void }) {
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
        className="clip-hud-sm border max-w-3xl w-[95%] mx-4 max-h-[85vh] flex flex-col"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: "var(--color-cyan)",
          boxShadow: "0 0 24px rgba(94,240,255,0.40)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="px-4 py-3 border-b flex items-center justify-between"
          style={{ borderColor: "rgba(94,240,255,0.30)" }}
        >
          <div>
            <div
              className="font-display font-semibold text-[13px] uppercase tracking-hud"
              style={{ color: "var(--color-cyan)" }}
            >
              ◂ {skill.name} {skill.version && `· v${skill.version}`}
            </div>
            <div className="font-mono text-[10px] text-[var(--color-dim)] mt-1">
              source · {skill.source} · {skill.path}
            </div>
            {skill.description && (
              <div className="font-mono text-[11px] text-[var(--color-fg)] mt-2">
                {skill.description}
              </div>
            )}
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

        <div className="flex-1 overflow-y-auto px-4 py-3 font-mono text-[12px] leading-[1.55] text-[var(--color-fg)]">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{skill.body}</ReactMarkdown>
        </div>
      </div>
    </div>
  );
}
