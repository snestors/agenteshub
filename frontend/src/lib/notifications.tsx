import * as React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Bot, AlertTriangle, Info, X } from "lucide-react";
import { wsClient } from "@/lib/wsClient";

// Web Audio API blip — synthesizes a short two-tone notification on demand.
// We don't ship an audio file because (a) one less network request, (b) no
// licensing question, and (c) it's a couple of lines.
let audioCtx: AudioContext | null = null;
function playBlip(severity: "info" | "warn" | "error") {
  try {
    if (!audioCtx) {
      const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctx) return;
      audioCtx = new Ctx();
    }
    const ctx = audioCtx;
    if (ctx.state === "suspended") void ctx.resume();
    const now = ctx.currentTime;
    const freq = severity === "error" ? 320 : severity === "warn" ? 520 : 740;
    const tones = severity === "error" ? [freq, freq * 0.7] : [freq, freq * 1.5];
    tones.forEach((f, i) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sine";
      osc.frequency.value = f;
      const start = now + i * 0.12;
      gain.gain.setValueAtTime(0, start);
      gain.gain.linearRampToValueAtTime(0.18, start + 0.01);
      gain.gain.exponentialRampToValueAtTime(0.001, start + 0.18);
      osc.connect(gain).connect(ctx.destination);
      osc.start(start);
      osc.stop(start + 0.2);
    });
  } catch {
    // ignore — autoplay policy or unavailable
  }
}

// Map a notification kind to the route we should offer to navigate to.
function routeForKind(kind: string): string | null {
  if (kind.startsWith("main_turn")) return "/";
  if (kind.startsWith("agent_run")) return "/agents";
  if (kind.startsWith("project_turn")) return "/projects";
  return null;
}

export interface Notification {
  id: string;
  kind: string;
  severity: "info" | "warn" | "error";
  title: string;
  body?: string;
  context?: Record<string, unknown>;
  ts: number;
  /** local-only flag: set when the user has marked the toast as seen */
  read?: boolean;
}

interface NotificationsState {
  items: Notification[];
  unreadByKindPrefix: (prefix: string) => number;
  markAllRead: () => void;
  dismiss: (id: string) => void;
  push: (n: Notification) => void;
}

const Ctx = React.createContext<NotificationsState | null>(null);

const MAX_KEEP = 50;
const TOAST_TTL_MS = 15000;

function parsePayload(payload: unknown): Notification | null {
  if (typeof payload === "string") {
    try {
      return JSON.parse(payload) as Notification;
    } catch {
      return null;
    }
  }
  if (payload && typeof payload === "object") return payload as Notification;
  return null;
}

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<Notification[]>([]);

  const push = React.useCallback((n: Notification) => {
    setItems((curr) => {
      const next = [n, ...curr].slice(0, MAX_KEEP);
      return next;
    });
  }, []);

  const dismiss = React.useCallback((id: string) => {
    setItems((curr) => curr.filter((i) => i.id !== id));
  }, []);

  const markAllRead = React.useCallback(() => {
    setItems((curr) => curr.map((i) => ({ ...i, read: true })));
  }, []);

  const unreadByKindPrefix = React.useCallback(
    (prefix: string) =>
      items.reduce((acc, i) => (!i.read && i.kind.startsWith(prefix) ? acc + 1 : acc), 0),
    [items]
  );

  // Subscribe globally to the 'notifications' topic.
  React.useEffect(() => {
    const off = wsClient.subscribe("notifications", (env) => {
      if (env.type !== "notification") return;
      const n = parsePayload(env.payload);
      if (!n || !n.id) return;
      push(n);
      playBlip(n.severity ?? "info");
    });
    return off;
  }, [push]);

  const value = React.useMemo<NotificationsState>(
    () => ({ items, unreadByKindPrefix, markAllRead, dismiss, push }),
    [items, unreadByKindPrefix, markAllRead, dismiss, push]
  );

  return (
    <Ctx.Provider value={value}>
      {children}
      <RoutedToastStack items={items} dismiss={dismiss} />
    </Ctx.Provider>
  );
}

// ─── Confirm modal ───────────────────────────────────────────

interface ConfirmModalProps {
  notif: Notification;
  route: string;
  onConfirm: () => void;
  onCancel: () => void;
}

function ConfirmModal({ notif, route, onConfirm, onCancel }: ConfirmModalProps) {
  const accent =
    notif.severity === "error"
      ? "var(--color-danger)"
      : notif.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";

  // Esc closes
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
      if (e.key === "Enter") onConfirm();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onConfirm, onCancel]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={onCancel}
    >
      <div
        className="clip-hud-sm border max-w-md w-[90%] mx-4"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: accent,
          boxShadow: `0 0 24px ${accent}50`,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b" style={{ borderColor: accent + "30" }}>
          <div className="font-display font-semibold text-[11px] uppercase tracking-hud" style={{ color: accent }}>
            ◂ {notif.title}
          </div>
          <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight mt-1 uppercase">
            navegar · {route}
          </div>
        </div>
        <div className="px-4 py-3">
          {notif.body && (
            <div className="font-mono text-[12px] text-[var(--color-fg)] leading-[1.55] break-words mb-3">
              {notif.body}
            </div>
          )}
          <div className="font-mono text-[11px] text-[var(--color-dim)] mb-4">
            ¿Querés ir a <span style={{ color: accent }}>{route}</span> ahora?
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onCancel}
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)", background: "rgba(255,255,255,0.02)" }}
            >
              cancelar
            </button>
            <button
              type="button"
              onClick={onConfirm}
              autoFocus
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight font-semibold cursor-pointer transition-all hover:scale-[1.02]"
              style={{
                color: "var(--color-bg)",
                background: accent,
                boxShadow: `0 0 8px ${accent}80`,
              }}
            >
              ir ahora ▸
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// shouldToast hides the toast when the user is already on the screen that
// would naturally surface that event. The notification is still kept in the
// store so the badge counter is honest.
function shouldToast(kind: string, pathname: string): boolean {
  if (kind.startsWith("main_turn") && pathname === "/") return false;
  if (kind.startsWith("agent_run") && pathname.startsWith("/agents")) return false;
  if (kind.startsWith("project_turn") && pathname.startsWith("/projects")) return false;
  return true;
}

export function useNotifications(): NotificationsState {
  const v = React.useContext(Ctx);
  if (!v) throw new Error("useNotifications must be used inside NotificationProvider");
  return v;
}

// ─── Toast stack ─────────────────────────────────────────────

function RoutedToastStack({
  items,
  dismiss,
}: {
  items: Notification[];
  dismiss: (id: string) => void;
}) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const [now, setNow] = React.useState(Date.now());
  const [confirming, setConfirming] = React.useState<{ notif: Notification; route: string } | null>(null);

  React.useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  const visible = items
    .filter((i) => !i.read && now - i.ts * 1000 < TOAST_TTL_MS && shouldToast(i.kind, pathname))
    .slice(0, 4);

  const handleClick = (n: Notification) => {
    const route = routeForKind(n.kind);
    if (!route || pathname === route || pathname.startsWith(route + "/")) {
      dismiss(n.id);
      return;
    }
    setConfirming({ notif: n, route });
  };

  return (
    <>
      {visible.length > 0 && (
        <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm pointer-events-none">
          {visible.map((n) => (
            <Toast key={n.id} n={n} onClose={() => dismiss(n.id)} onClick={() => handleClick(n)} />
          ))}
        </div>
      )}
      {confirming && (
        <ConfirmModal
          notif={confirming.notif}
          route={confirming.route}
          onConfirm={() => {
            navigate(confirming.route);
            dismiss(confirming.notif.id);
            setConfirming(null);
          }}
          onCancel={() => {
            dismiss(confirming.notif.id);
            setConfirming(null);
          }}
        />
      )}
    </>
  );
}

function Toast({
  n,
  onClose,
  onClick,
}: {
  n: Notification;
  onClose: () => void;
  onClick: () => void;
}) {
  const accent =
    n.severity === "error"
      ? "var(--color-danger)"
      : n.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  const Icon = n.severity === "error" ? AlertTriangle : n.kind.startsWith("agent_run") ? Bot : Info;
  const hasRoute = routeForKind(n.kind) !== null;
  return (
    <div
      role={hasRoute ? "button" : undefined}
      tabIndex={hasRoute ? 0 : -1}
      onClick={hasRoute ? onClick : undefined}
      onKeyDown={
        hasRoute
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
      className={`pointer-events-auto clip-tag flex gap-3 px-3 py-2 backdrop-blur-sm border ${
        hasRoute ? "cursor-pointer hover:scale-[1.01] transition-transform" : ""
      }`}
      style={{
        background: "rgba(10, 15, 36, 0.90)",
        borderColor: accent,
        boxShadow: `0 0 14px ${accent}40`,
      }}
    >
      <Icon size={16} strokeWidth={1.6} style={{ color: accent, marginTop: 2 }} />
      <div className="flex-1 min-w-0">
        <div className="font-mono text-[11px] uppercase tracking-hud-tight" style={{ color: accent }}>
          {n.title}
        </div>
        {n.body && (
          <div className="font-mono text-[11px] text-[var(--color-fg)] mt-1 break-words">
            {n.body}
          </div>
        )}
      </div>
      <button
        onClick={(e) => {
          e.stopPropagation();
          onClose();
        }}
        className="text-[var(--color-dim)] hover:text-[var(--color-fg)] transition-colors p-0.5 cursor-pointer self-start"
        aria-label="cerrar"
      >
        <X size={12} strokeWidth={1.8} />
      </button>
    </div>
  );
}
