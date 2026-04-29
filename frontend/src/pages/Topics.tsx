import * as React from "react";
import { Hash, Plus, X, Pencil } from "lucide-react";
import { Topbar } from "@/components/Topbar";
import { topicsApi, type Topic, type TopicState } from "@/lib/api";

function fmtRelative(ts?: number): string {
  if (!ts) return "—";
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
  return `${Math.floor(sec / 86400)}d`;
}

export function Topics() {
  const [items, setItems] = React.useState<Topic[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [selected, setSelected] = React.useState<Topic | null>(null);
  const [creating, setCreating] = React.useState(false);

  const refresh = React.useCallback(async () => {
    try {
      const list = await topicsApi.list();
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

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Topics" }]}
        status={
          error
            ? { label: "OFFLINE", tone: "danger" }
            : { label: `${items.length} TOPICS`, tone: "ok" }
        }
      />

      <div className="px-3 py-3 border-b flex flex-wrap items-center gap-3 sm:px-4" style={{ borderColor: "var(--color-line)" }}>
        <Hash size={14} strokeWidth={1.6} style={{ color: "var(--color-magenta)" }} />
        <span className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]">
          contextos del main agent
        </span>
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="sm:ml-auto px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors"
          style={{
            color: "var(--color-magenta)",
            borderColor: "var(--color-magenta)",
            background: "rgba(255,78,214,0.06)",
          }}
        >
          <Plus size={11} strokeWidth={1.8} className="inline mr-1" />
          nuevo
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-3 sm:px-4">
        {loading ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            ▸ cargando…
          </div>
        ) : items.length === 0 ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            sin topics todavía. crea uno con el botón "+ nuevo"
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {items.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => setSelected(t)}
                className="text-left clip-tag border px-3 py-2.5 cursor-pointer transition-colors hover:bg-[rgba(255,78,214,0.06)]"
                style={{ borderColor: "var(--color-line)", background: "rgba(255,78,214,0.03)" }}
              >
                <div className="flex items-baseline justify-between gap-2 mb-1">
                  <span className="font-display font-semibold text-[12px] tracking-hud" style={{ color: "var(--color-magenta)" }}>
                    # {t.name}
                  </span>
                  {t.is_default && (
                    <span className="font-mono text-[9px] uppercase text-[var(--color-cyan)] tracking-hud-tight">
                      default
                    </span>
                  )}
                </div>
                {t.description && (
                  <div className="font-mono text-[11px] text-[var(--color-fg)] mb-2 break-words">
                    {t.description}
                  </div>
                )}
                <div className="flex items-center gap-3 font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight uppercase">
                  <span>engine · {t.engine}</span>
                  <span>·</span>
                  <span>activo · {fmtRelative(t.last_active_at)}</span>
                </div>
                {t.keywords && t.keywords.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {t.keywords.map((k) => (
                      <span
                        key={k}
                        className="font-mono text-[9px] px-1.5 py-px clip-tag"
                        style={{
                          color: "var(--color-cyan)",
                          background: "rgba(94,240,255,0.06)",
                          border: "1px solid rgba(94,240,255,0.20)",
                        }}
                      >
                        {k}
                      </span>
                    ))}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {selected && (
        <TopicDetail topic={selected} onClose={() => setSelected(null)} onUpdated={refresh} />
      )}
      {creating && (
        <CreateTopicModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            void refresh();
          }}
        />
      )}
    </div>
  );
}

function TopicDetail({
  topic,
  onClose,
  onUpdated,
}: {
  topic: Topic;
  onClose: () => void;
  onUpdated: () => void;
}) {
  const [state, setState] = React.useState<TopicState | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [editing, setEditing] = React.useState(false);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !editing) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, editing]);

  React.useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const s = await topicsApi.getState(topic.id);
        if (alive) setState(s);
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [topic.id]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center overflow-y-auto py-4"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={editing ? undefined : onClose}
    >
      <div
        className="clip-hud-sm border max-w-2xl w-[90%] mx-4 max-h-[85vh] flex flex-col"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: "var(--color-magenta)",
          boxShadow: "0 0 24px rgba(255,78,214,0.40)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 flex items-center justify-between border-b" style={{ borderColor: "rgba(255,78,214,0.30)" }}>
          <div>
            <div className="font-display font-semibold text-[13px] uppercase tracking-hud" style={{ color: "var(--color-magenta)" }}>
              # {topic.name}
            </div>
            {topic.description && (
              <div className="font-mono text-[11px] text-[var(--color-fg)] mt-1 break-words">
                {topic.description}
              </div>
            )}
          </div>
          <div className="flex items-center gap-2">
            {!editing && state && (
              <button
                type="button"
                onClick={() => setEditing(true)}
                className="text-[var(--color-dim)] hover:text-[var(--color-cyan)] cursor-pointer p-1 transition-colors"
                aria-label="editar"
              >
                <Pencil size={13} strokeWidth={1.8} />
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              className="text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer p-1 transition-colors"
              aria-label="cerrar"
            >
              <X size={14} strokeWidth={1.8} />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3 font-mono text-[11px] sm:px-4">
          {loading ? (
            <div className="text-[var(--color-dim)] italic">▸ cargando state…</div>
          ) : editing && state ? (
            <TopicStateEditor
              state={state}
              onSave={async (next) => {
                await topicsApi.updateState(topic.id, next);
                const fresh = await topicsApi.getState(topic.id);
                setState(fresh);
                setEditing(false);
                onUpdated();
              }}
              onCancel={() => setEditing(false)}
            />
          ) : state ? (
            <TopicStateView state={state} />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function TopicStateView({ state }: { state: TopicState }) {
  const empty =
    !state.headline &&
    !state.next_action_hint &&
    !(state.active_issues && state.active_issues.length) &&
    !(state.recent_decisions && state.recent_decisions.length) &&
    !(state.pending && state.pending.length);
  if (empty) {
    return (
      <div className="text-[var(--color-dim)] italic py-4 text-center">
        topic sin state. clickeá ✎ para inicializar.
      </div>
    );
  }
  return (
    <>
      {state.headline && <Block label="headline">{state.headline}</Block>}
      {state.next_action_hint && (
        <Block label="next action" accent="orange">
          {state.next_action_hint}
        </Block>
      )}
      {state.active_issues && state.active_issues.length > 0 && (
        <ListBlock label="active issues" items={state.active_issues} accent="danger" />
      )}
      {state.recent_decisions && state.recent_decisions.length > 0 && (
        <ListBlock label="recent decisions" items={state.recent_decisions} />
      )}
      {state.pending && state.pending.length > 0 && (
        <ListBlock label="pending" items={state.pending} accent="orange" />
      )}
      {state.updated_at > 0 && (
        <div className="text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] pt-1">
          updated · {new Date(state.updated_at * 1000).toLocaleString("es-PE")}
        </div>
      )}
    </>
  );
}

function TopicStateEditor({
  state,
  onSave,
  onCancel,
}: {
  state: TopicState;
  onSave: (s: Partial<TopicState>) => Promise<void>;
  onCancel: () => void;
}) {
  const [headline, setHeadline] = React.useState(state.headline ?? "");
  const [nextAction, setNextAction] = React.useState(state.next_action_hint ?? "");
  const [activeIssues, setActiveIssues] = React.useState((state.active_issues ?? []).join("\n"));
  const [decisions, setDecisions] = React.useState((state.recent_decisions ?? []).join("\n"));
  const [pending, setPending] = React.useState((state.pending ?? []).join("\n"));
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  async function handleSubmit() {
    setBusy(true);
    setErr(null);
    try {
      await onSave({
        headline: headline.trim() || undefined,
        next_action_hint: nextAction.trim() || undefined,
        active_issues: activeIssues.split("\n").map((s) => s.trim()).filter(Boolean),
        recent_decisions: decisions.split("\n").map((s) => s.trim()).filter(Boolean),
        pending: pending.split("\n").map((s) => s.trim()).filter(Boolean),
      });
    } catch (e) {
      setErr(e instanceof Error ? e.message : "error guardando");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3">
      <Field label="headline" value={headline} onChange={setHeadline} />
      <Field label="next action hint" value={nextAction} onChange={setNextAction} />
      <Field label="active issues (uno por línea)" value={activeIssues} onChange={setActiveIssues} textarea />
      <Field label="recent decisions (uno por línea)" value={decisions} onChange={setDecisions} textarea />
      <Field label="pending (uno por línea)" value={pending} onChange={setPending} textarea />
      {err && <div className="text-[var(--color-danger)] text-[11px]">⚠ {err}</div>}
      <div className="flex gap-2 justify-end pt-1">
        <button
          type="button"
          onClick={onCancel}
          disabled={busy}
          className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
          style={{ borderColor: "var(--color-line)" }}
        >
          cancelar
        </button>
        <button
          type="button"
          onClick={() => void handleSubmit()}
          disabled={busy}
          className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight font-semibold cursor-pointer transition-all hover:scale-[1.02] disabled:opacity-60"
          style={{ color: "var(--color-bg)", background: "var(--color-magenta)", boxShadow: "0 0 8px rgba(255,78,214,0.50)" }}
        >
          {busy ? "guardando…" : "guardar"}
        </button>
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  textarea,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  textarea?: boolean;
}) {
  return (
    <label className="block">
      <span className="font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] block mb-1">
        {label}
      </span>
      {textarea ? (
        <textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          rows={3}
          className="w-full font-mono text-[11.5px] px-2 py-1.5 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)] resize-y"
          style={{ borderColor: "var(--color-line)" }}
        />
      ) : (
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full font-mono text-[11.5px] px-2 py-1.5 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)]"
          style={{ borderColor: "var(--color-line)" }}
        />
      )}
    </label>
  );
}

function CreateTopicModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [keywords, setKeywords] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function handleSubmit() {
    if (!name.trim()) {
      setErr("nombre requerido");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const kws = keywords.split(",").map((s) => s.trim()).filter(Boolean);
      await topicsApi.create(name.trim(), description.trim() || undefined, kws.length > 0 ? kws : undefined);
      onCreated();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "error creando");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center overflow-y-auto py-4"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={onClose}
    >
      <div
        className="clip-hud-sm border max-w-md w-[90%] mx-4"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: "var(--color-magenta)",
          boxShadow: "0 0 24px rgba(255,78,214,0.40)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b" style={{ borderColor: "rgba(255,78,214,0.30)" }}>
          <div className="font-display font-semibold text-[12px] uppercase tracking-hud" style={{ color: "var(--color-magenta)" }}>
            ◂ nuevo topic
          </div>
        </div>
        <div className="px-4 py-3 space-y-3">
          <Field label="nombre" value={name} onChange={setName} />
          <Field label="descripción (opcional)" value={description} onChange={setDescription} textarea />
          <Field label="keywords (separadas por coma)" value={keywords} onChange={setKeywords} />
          {err && <div className="text-[var(--color-danger)] text-[11px]">⚠ {err}</div>}
          <div className="flex gap-2 justify-end pt-1">
            <button
              type="button"
              onClick={onClose}
              disabled={busy}
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)" }}
            >
              cancelar
            </button>
            <button
              type="button"
              onClick={() => void handleSubmit()}
              disabled={busy}
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight font-semibold cursor-pointer transition-all hover:scale-[1.02] disabled:opacity-60"
              style={{ color: "var(--color-bg)", background: "var(--color-magenta)", boxShadow: "0 0 8px rgba(255,78,214,0.50)" }}
            >
              {busy ? "creando…" : "crear"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function Block({
  label,
  accent,
  children,
}: {
  label: string;
  accent?: "danger" | "orange" | "cyan";
  children: React.ReactNode;
}) {
  const color =
    accent === "danger"
      ? "var(--color-danger)"
      : accent === "orange"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  return (
    <div>
      <div className="font-mono text-[9px] uppercase tracking-hud-tight mb-1" style={{ color }}>
        ▸ {label}
      </div>
      <div
        className="px-3 py-2 clip-tag border font-mono text-[11px] text-[var(--color-fg)] break-words"
        style={{ background: `${color}08`, borderColor: `${color}30` }}
      >
        {children}
      </div>
    </div>
  );
}

function ListBlock({
  label,
  items,
  accent,
}: {
  label: string;
  items: string[];
  accent?: "danger" | "orange" | "cyan";
}) {
  const color =
    accent === "danger"
      ? "var(--color-danger)"
      : accent === "orange"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  return (
    <div>
      <div className="font-mono text-[9px] uppercase tracking-hud-tight mb-1" style={{ color }}>
        ▸ {label} ({items.length})
      </div>
      <ul className="space-y-1">
        {items.map((it, i) => (
          <li
            key={i}
            className="px-3 py-1.5 clip-tag border font-mono text-[11px] text-[var(--color-fg)] break-words"
            style={{ background: `${color}06`, borderColor: `${color}25` }}
          >
            ▸ {it}
          </li>
        ))}
      </ul>
    </div>
  );
}
