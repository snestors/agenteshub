import * as React from "react";
import { Lock, Eye, EyeOff, Plus, Trash2, X, Copy, Check } from "lucide-react";
import { Topbar } from "@/components/Topbar";
import { secretsApi, type SecretMeta } from "@/lib/api";

function fmtRelative(ts?: number): string {
  if (!ts) return "—";
  const sec = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
  return `${Math.floor(sec / 86400)}d`;
}

export function Vault() {
  const [items, setItems] = React.useState<SecretMeta[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);
  const [revealing, setRevealing] = React.useState<{ key: string; value: string } | null>(null);
  const [confirmDelete, setConfirmDelete] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      setItems(await secretsApi.list());
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

  async function handleReveal(key: string) {
    try {
      const value = await secretsApi.reveal(key);
      setRevealing({ key, value });
      // refresh to update last_accessed_at
      void refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error revelando");
    }
  }

  async function handleDelete(key: string) {
    try {
      await secretsApi.delete(key);
      setConfirmDelete(null);
      void refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error borrando");
    }
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Vault" }]}
        status={
          error
            ? { label: "OFFLINE", tone: "danger" }
            : { label: `${items.length} SECRETS`, tone: "ok" }
        }
      />

      <div
        className="px-4 py-3 border-b flex items-center gap-3"
        style={{ borderColor: "var(--color-line)" }}
      >
        <Lock size={14} strokeWidth={1.6} style={{ color: "var(--color-orange)" }} />
        <span className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]">
          tokens encriptados · AES-GCM
        </span>
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="ml-auto px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors"
          style={{
            color: "var(--color-orange)",
            borderColor: "var(--color-orange)",
            background: "rgba(255,184,108,0.06)",
          }}
        >
          <Plus size={11} strokeWidth={1.8} className="inline mr-1" />
          nuevo
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-3">
        {loading ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            ▸ cargando…
          </div>
        ) : items.length === 0 ? (
          <div className="text-center font-mono text-[11px] text-[var(--color-dim)] italic py-8">
            sin tokens guardados. agregá uno con "+ nuevo"
          </div>
        ) : (
          <table className="w-full font-mono text-[11px]">
            <thead>
              <tr
                className="text-left text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] border-b"
                style={{ borderColor: "var(--color-line)" }}
              >
                <th className="px-2 py-1.5">key</th>
                <th className="px-2 py-1.5">descripción</th>
                <th className="px-2 py-1.5">scope</th>
                <th className="px-2 py-1.5">creado</th>
                <th className="px-2 py-1.5">accedido</th>
                <th className="px-2 py-1.5"></th>
              </tr>
            </thead>
            <tbody>
              {items.map((s) => (
                <tr
                  key={s.key}
                  className="border-b hover:bg-[rgba(255,184,108,0.04)]"
                  style={{ borderColor: "var(--color-line)" }}
                >
                  <td className="px-2 py-1.5 font-semibold" style={{ color: "var(--color-orange)" }}>
                    {s.key}
                  </td>
                  <td className="px-2 py-1.5 text-[var(--color-fg)] truncate max-w-[260px]">
                    {s.description || "—"}
                  </td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{s.scope}</td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">{fmtRelative(s.created_at)}</td>
                  <td className="px-2 py-1.5 text-[var(--color-dim)]">
                    {s.last_accessed_at ? fmtRelative(s.last_accessed_at) : "nunca"}
                  </td>
                  <td className="px-2 py-1.5 flex gap-1 justify-end">
                    <button
                      type="button"
                      onClick={() => void handleReveal(s.key)}
                      className="text-[var(--color-dim)] hover:text-[var(--color-cyan)] cursor-pointer p-1 transition-colors"
                      title="revelar"
                    >
                      <Eye size={12} strokeWidth={1.6} />
                    </button>
                    <button
                      type="button"
                      onClick={() => setConfirmDelete(s.key)}
                      className="text-[var(--color-dim)] hover:text-[var(--color-danger)] cursor-pointer p-1 transition-colors"
                      title="borrar"
                    >
                      <Trash2 size={12} strokeWidth={1.6} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {creating && (
        <CreateSecretModal
          onClose={() => setCreating(false)}
          onCreated={() => {
            setCreating(false);
            void refresh();
          }}
        />
      )}
      {revealing && (
        <RevealModal
          secretKey={revealing.key}
          value={revealing.value}
          onClose={() => setRevealing(null)}
        />
      )}
      {confirmDelete && (
        <ConfirmDelete
          secretKey={confirmDelete}
          onConfirm={() => void handleDelete(confirmDelete)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </div>
  );
}

function CreateSecretModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [key, setKey] = React.useState("");
  const [value, setValue] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [scope, setScope] = React.useState("global");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function handleSubmit() {
    if (!key.trim() || !value.trim()) {
      setErr("key y value son requeridos");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await secretsApi.upsert({
        key: key.trim(),
        value,
        description: description.trim() || undefined,
        scope: scope.trim() || undefined,
      });
      onCreated();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "error guardando");
    } finally {
      setBusy(false);
    }
  }

  return (
    <ModalShell onClose={onClose} title="◂ nuevo secret" accent="var(--color-orange)">
      <Field label="key (e.g. BBVA_API_KEY)" value={key} onChange={setKey} mono />
      <Field label="value" value={value} onChange={setValue} secret />
      <Field label="descripción (opcional)" value={description} onChange={setDescription} textarea />
      <Field label="scope" value={scope} onChange={setScope} />
      {err && <div className="text-[var(--color-danger)] text-[11px]">⚠ {err}</div>}
      <div className="flex gap-2 justify-end pt-1">
        <ModalBtn onClick={onClose} disabled={busy}>
          cancelar
        </ModalBtn>
        <ModalBtn onClick={() => void handleSubmit()} disabled={busy} primary>
          {busy ? "guardando…" : "guardar"}
        </ModalBtn>
      </div>
    </ModalShell>
  );
}

function RevealModal({
  secretKey,
  value,
  onClose,
}: {
  secretKey: string;
  value: string;
  onClose: () => void;
}) {
  const [shown, setShown] = React.useState(false);
  const [copied, setCopied] = React.useState(false);
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore
    }
  }

  return (
    <ModalShell onClose={onClose} title={`◂ ${secretKey}`} accent="var(--color-cyan)">
      <div className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]">
        valor (texto plano · NO compartir)
      </div>
      <div
        className="px-3 py-2 clip-tag border font-mono text-[12px] break-all relative"
        style={{
          background: "rgba(94,240,255,0.04)",
          borderColor: "rgba(94,240,255,0.30)",
          color: "var(--color-fg)",
        }}
      >
        {shown ? value : "•".repeat(Math.min(value.length, 40))}
      </div>
      <div className="flex gap-2 justify-end pt-1">
        <ModalBtn onClick={() => setShown((v) => !v)}>
          {shown ? <EyeOff size={11} strokeWidth={1.8} className="inline mr-1" /> : <Eye size={11} strokeWidth={1.8} className="inline mr-1" />}
          {shown ? "ocultar" : "mostrar"}
        </ModalBtn>
        <ModalBtn onClick={() => void copy()} primary>
          {copied ? <Check size={11} strokeWidth={1.8} className="inline mr-1" /> : <Copy size={11} strokeWidth={1.8} className="inline mr-1" />}
          {copied ? "copiado" : "copiar"}
        </ModalBtn>
        <ModalBtn onClick={onClose}>cerrar</ModalBtn>
      </div>
    </ModalShell>
  );
}

function ConfirmDelete({
  secretKey,
  onConfirm,
  onCancel,
}: {
  secretKey: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
      if (e.key === "Enter") onConfirm();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onConfirm, onCancel]);

  return (
    <ModalShell onClose={onCancel} title="◂ borrar secret" accent="var(--color-danger)">
      <div className="font-mono text-[12px] text-[var(--color-fg)]">
        ¿Borrar <span style={{ color: "var(--color-danger)" }}>{secretKey}</span>? Esta acción no se puede deshacer.
      </div>
      <div className="flex gap-2 justify-end pt-1">
        <ModalBtn onClick={onCancel}>cancelar</ModalBtn>
        <ModalBtn onClick={onConfirm} danger>
          borrar ▸
        </ModalBtn>
      </div>
    </ModalShell>
  );
}

// ─── shared modal pieces ─────────────────────────

function ModalShell({
  onClose,
  title,
  accent,
  children,
}: {
  onClose: () => void;
  title: string;
  accent: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={onClose}
    >
      <div
        className="clip-hud-sm border max-w-md w-[90%] mx-4 max-h-[85vh] flex flex-col"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: accent,
          boxShadow: `0 0 24px ${accent}50`,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="px-4 py-3 border-b flex items-center justify-between"
          style={{ borderColor: accent + "30" }}
        >
          <div className="font-display font-semibold text-[12px] uppercase tracking-hud" style={{ color: accent }}>
            {title}
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
        <div className="px-4 py-3 space-y-3 overflow-y-auto">{children}</div>
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  textarea,
  mono,
  secret,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  textarea?: boolean;
  mono?: boolean;
  secret?: boolean;
}) {
  const cls =
    "w-full font-mono text-[11.5px] px-2 py-1.5 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)]" +
    (mono ? " uppercase tracking-hud-tight" : "");
  return (
    <label className="block">
      <span className="font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] block mb-1">
        {label}
      </span>
      {textarea ? (
        <textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          rows={2}
          className={cls + " resize-y"}
          style={{ borderColor: "var(--color-line)" }}
        />
      ) : (
        <input
          type={secret ? "password" : "text"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete={secret ? "new-password" : "off"}
          className={cls}
          style={{ borderColor: "var(--color-line)" }}
        />
      )}
    </label>
  );
}

function ModalBtn({
  onClick,
  disabled,
  primary,
  danger,
  children,
}: {
  onClick: () => void;
  disabled?: boolean;
  primary?: boolean;
  danger?: boolean;
  children: React.ReactNode;
}) {
  const accent = danger ? "var(--color-danger)" : primary ? "var(--color-orange)" : "var(--color-line)";
  const isFilled = primary || danger;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={
        "px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight cursor-pointer transition-all hover:scale-[1.02] disabled:opacity-60 disabled:cursor-not-allowed " +
        (isFilled ? "font-semibold" : "border text-[var(--color-dim)] hover:text-[var(--color-fg)]")
      }
      style={
        isFilled
          ? {
              color: "var(--color-bg)",
              background: accent,
              boxShadow: `0 0 8px ${accent}80`,
            }
          : { borderColor: "var(--color-line)" }
      }
    >
      {children}
    </button>
  );
}
