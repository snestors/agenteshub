import * as React from "react";
import {
  GitBranch,
  Loader2,
  CheckCircle2,
  AlertCircle,
  X,
  ChevronRight,
  ChevronDown,
  Bot,
  FolderKanban,
  MessageSquare,
  Hash,
} from "lucide-react";
import { Topbar } from "@/components/Topbar";
import { subagentsApi, type Subagent } from "@/lib/api";

const STATUS_OPTS: Array<{ key: ""; label: string } | { key: Subagent["status"]; label: string }> = [
  { key: "", label: "todos" },
  { key: "running", label: "corriendo" },
  { key: "ok", label: "ok" },
  { key: "error", label: "error" },
  { key: "cancelled", label: "cancelado" },
];

const DAY_S = 86400;

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

function fmtRelative(ts: number): string {
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
  return `${Math.floor(sec / 86400)}d`;
}

function statusTone(s: Subagent["status"]): {
  color: string;
  Icon: React.ComponentType<{ size?: number; strokeWidth?: number; style?: React.CSSProperties; className?: string }>;
} {
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

function parentLabel(s: Subagent): { label: string; Icon: typeof MessageSquare; accent: string } {
  switch (s.parent_scope) {
    case "main":
      return { label: "Main agent", Icon: MessageSquare, accent: "var(--color-magenta)" };
    case "project":
      return {
        label: s.parent_project_session_id ? `Project session #${s.parent_project_session_id}` : "Project",
        Icon: FolderKanban,
        accent: "var(--color-lime)",
      };
    case "agent":
      return { label: "Mini-agent", Icon: Bot, accent: "var(--color-orange)" };
    case "topic":
      return {
        label: s.parent_topic_id ? `Topic #${s.parent_topic_id}` : "Topic",
        Icon: Hash,
        accent: "var(--color-cyan)",
      };
    default:
      return { label: s.parent_scope, Icon: GitBranch, accent: "var(--color-dim)" };
  }
}

interface ParentGroup {
  /** key for React + dedup: scope + session_id */
  groupKey: string;
  scope: Subagent["parent_scope"];
  sessionID: string;
  label: string;
  Icon: typeof MessageSquare;
  accent: string;
  children: Subagent[];
  /** counters across the children */
  running: number;
  ok24h: number;
  error24h: number;
  lastActivity: number;
}

function groupByParent(items: Subagent[]): ParentGroup[] {
  const now = Math.floor(Date.now() / 1000);
  const byKey = new Map<string, ParentGroup>();
  for (const s of items) {
    const groupKey = `${s.parent_scope}:${s.parent_session_id}`;
    let g = byKey.get(groupKey);
    if (!g) {
      const meta = parentLabel(s);
      g = {
        groupKey,
        scope: s.parent_scope,
        sessionID: s.parent_session_id,
        label: meta.label,
        Icon: meta.Icon,
        accent: meta.accent,
        children: [],
        running: 0,
        ok24h: 0,
        error24h: 0,
        lastActivity: 0,
      };
      byKey.set(groupKey, g);
    }
    g.children.push(s);
    if (s.status === "running") g.running++;
    if (now - s.started_at < DAY_S) {
      if (s.status === "ok") g.ok24h++;
      if (s.status === "error") g.error24h++;
    }
    if (s.started_at > g.lastActivity) g.lastActivity = s.started_at;
  }
  // sort children newest first, groups by recent activity desc + running first
  const groups = Array.from(byKey.values());
  for (const g of groups) g.children.sort((a, b) => b.started_at - a.started_at);
  groups.sort((a, b) => {
    if (a.running !== b.running) return b.running - a.running;
    return b.lastActivity - a.lastActivity;
  });
  return groups;
}

export function Subagents() {
  const [items, setItems] = React.useState<Subagent[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [tab, setTab] = React.useState<"tree" | "history">("tree");
  const [status, setStatus] = React.useState<string>("");
  const [selected, setSelected] = React.useState<Subagent | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      const list = await subagentsApi.list(tab === "history" ? status || undefined : undefined, 200);
      setItems(list);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error de red");
    } finally {
      setLoading(false);
    }
  }, [tab, status]);

  React.useEffect(() => {
    void refresh();
    const t = window.setInterval(refresh, 5000);
    return () => window.clearInterval(t);
  }, [refresh]);

  const groups = React.useMemo(() => groupByParent(items), [items]);
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

      {/* tab + filter bar */}
      <div
        className="px-4 py-3 border-b flex items-center gap-3 flex-wrap"
        style={{ borderColor: "var(--color-line)" }}
      >
        <GitBranch size={14} strokeWidth={1.6} style={{ color: "var(--color-cyan)" }} />
        <div className="flex gap-1">
          <TabBtn active={tab === "tree"} onClick={() => setTab("tree")}>
            esquemático
          </TabBtn>
          <TabBtn active={tab === "history"} onClick={() => setTab("history")}>
            histórico
          </TabBtn>
        </div>
        {tab === "history" && (
          <>
            <span className="font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] ml-3">
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
          </>
        )}
        <span className="ml-auto font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
          {tab === "tree" ? `${groups.length} parents · ${items.length} subagents` : `${items.length} registros`}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <Empty msg="▸ cargando…" />
        ) : items.length === 0 ? (
          <Empty msg="sin sub-agentes capturados todavía" />
        ) : tab === "tree" ? (
          <div className="space-y-3">
            {groups.map((g) => (
              <ParentCard key={g.groupKey} group={g} onSelect={setSelected} />
            ))}
          </div>
        ) : (
          <HistoryTable items={items} onSelect={setSelected} />
        )}
      </div>

      {selected && <SubagentDetail subagent={selected} onClose={() => setSelected(null)} />}
    </div>
  );
}

function TabBtn({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`px-3 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors ${
        active ? "text-[var(--color-fg)]" : "text-[var(--color-dim)] hover:text-[var(--color-fg)]"
      }`}
      style={{
        borderColor: active ? "var(--color-cyan)" : "var(--color-line)",
        background: active ? "rgba(94,240,255,0.08)" : "transparent",
      }}
    >
      {children}
    </button>
  );
}

function Empty({ msg }: { msg: string }) {
  return (
    <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
      {msg}
    </div>
  );
}

function ParentCard({
  group,
  onSelect,
}: {
  group: ParentGroup;
  onSelect: (s: Subagent) => void;
}) {
  const [open, setOpen] = React.useState(group.running > 0); // expand if active
  const visible = open ? group.children : group.children.slice(0, 3);
  const hasMore = group.children.length > visible.length;

  return (
    <div
      className="clip-tag border"
      style={{
        borderColor: group.running > 0 ? group.accent : "var(--color-line)",
        background: group.running > 0 ? `${group.accent}06` : "rgba(255,255,255,0.01)",
      }}
    >
      {/* header */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full px-3 py-2 flex items-center gap-3 cursor-pointer transition-colors hover:bg-[rgba(94,240,255,0.04)] text-left"
      >
        {open ? (
          <ChevronDown size={12} strokeWidth={1.8} style={{ color: "var(--color-dim)" }} />
        ) : (
          <ChevronRight size={12} strokeWidth={1.8} style={{ color: "var(--color-dim)" }} />
        )}
        <group.Icon size={14} strokeWidth={1.6} style={{ color: group.accent }} />
        <div className="flex-1 min-w-0">
          <div
            className="font-display font-semibold text-[12px] uppercase tracking-hud truncate"
            style={{ color: group.accent }}
          >
            {group.label}
          </div>
          <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight">
            session · {group.sessionID.slice(0, 8)}…
          </div>
        </div>
        <div className="flex items-center gap-3 font-mono text-[10px] tabular-nums shrink-0">
          {group.running > 0 && (
            <span className="flex items-center gap-1" style={{ color: "var(--color-orange)" }}>
              <Loader2 size={10} strokeWidth={2} className="animate-spin" />
              {group.running} live
            </span>
          )}
          {group.ok24h > 0 && (
            <span style={{ color: "var(--color-lime)" }}>{group.ok24h} ok 24h</span>
          )}
          {group.error24h > 0 && (
            <span style={{ color: "var(--color-danger)" }}>{group.error24h} err 24h</span>
          )}
          <span className="text-[var(--color-dim)]">total · {group.children.length}</span>
          <span className="text-[var(--color-dim)]">{fmtRelative(group.lastActivity)}</span>
        </div>
      </button>

      {/* children */}
      {open && (
        <div className="border-t" style={{ borderColor: "var(--color-line)" }}>
          <div className="pl-7 pr-3 py-2 space-y-1.5">
            {visible.map((s, idx) => (
              <ChildRow
                key={s.id}
                subagent={s}
                isLast={idx === visible.length - 1 && !hasMore}
                onSelect={onSelect}
              />
            ))}
            {hasMore && (
              <div className="pl-4 pt-1 font-mono text-[9px] text-[var(--color-dim)] italic">
                + {group.children.length - visible.length} más en histórico
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ChildRow({
  subagent,
  isLast,
  onSelect,
}: {
  subagent: Subagent;
  isLast: boolean;
  onSelect: (s: Subagent) => void;
}) {
  const tone = statusTone(subagent.status);
  const SIcon = tone.Icon;
  const desc = subagent.description || subagent.prompt?.slice(0, 80) || "—";
  return (
    <div className="flex items-start gap-2 relative">
      {/* tree connector */}
      <div className="relative w-3 shrink-0 self-stretch">
        <span
          className="absolute left-0 top-0 w-px"
          style={{
            background: "var(--color-line)",
            height: isLast ? "10px" : "100%",
          }}
        />
        <span
          className="absolute left-0 top-[10px] h-px w-3"
          style={{ background: "var(--color-line)" }}
        />
      </div>
      <button
        type="button"
        onClick={() => onSelect(subagent)}
        className="flex-1 flex items-start gap-2 px-2 py-1 clip-tag cursor-pointer hover:bg-[rgba(94,240,255,0.04)] text-left"
      >
        <SIcon
          size={11}
          strokeWidth={1.8}
          className={subagent.status === "running" ? "animate-spin shrink-0 mt-0.5" : "shrink-0 mt-0.5"}
          style={{ color: tone.color }}
        />
        <div className="flex-1 min-w-0">
          <div className="font-mono text-[11px] flex items-center gap-2 flex-wrap">
            <span style={{ color: "var(--color-cyan)" }}>{subagent.agent_type || "—"}</span>
            <span className="text-[var(--color-fg)] truncate">{desc}</span>
          </div>
          <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight mt-0.5">
            #{subagent.id} · started {fmtRelative(subagent.started_at)} ago · dur{" "}
            {fmtDuration(subagent.started_at, subagent.finished_at)}
            {subagent.cost_tokens > 0 && ` · ${subagent.cost_tokens} tok`}
          </div>
        </div>
      </button>
    </div>
  );
}

function HistoryTable({
  items,
  onSelect,
}: {
  items: Subagent[];
  onSelect: (s: Subagent) => void;
}) {
  return (
    <table className="w-full font-mono text-[11px]">
      <thead>
        <tr
          className="text-left text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] border-b"
          style={{ borderColor: "var(--color-line)" }}
        >
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
              onClick={() => onSelect(s)}
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
                  <SIcon size={11} strokeWidth={1.8} className={s.status === "running" ? "animate-spin" : ""} />
                  {s.status}
                </span>
              </td>
              <td className="px-2 py-1.5 text-[var(--color-dim)]">{fmtTime(s.started_at)}</td>
              <td className="px-2 py-1.5 text-[var(--color-dim)]">
                {fmtDuration(s.started_at, s.finished_at)}
              </td>
              <td className="px-2 py-1.5 text-[var(--color-dim)] tabular-nums">{s.cost_tokens}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
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
        <div
          className="px-4 py-3 flex items-center justify-between border-b"
          style={{ borderColor: tone.color + "30" }}
        >
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
